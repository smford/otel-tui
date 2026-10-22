package store

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// RebuildTrace updates the derived properties of a Trace, including its duration,
// parent-child hierarchy, critical path, timing percentages, and correlated logs.
func (t *Trace) RebuildTrace() {
	if len(t.Spans) == 0 {
		return
	}

	t.SpansByID = make(map[string]*Span, len(t.Spans))
	for _, s := range t.Spans {
		t.SpansByID[s.SpanID] = s
		s.Logs = nil // Reset correlated logs to avoid duplicates on re-assembly
	}

	// Correlate logs to spans
	for i := range t.Logs {
		log := t.Logs[i]
		if span, ok := t.SpansByID[log.SpanID]; ok {
			span.Logs = append(span.Logs, log)
		}
	}

	// Determine overall trace start time and end time
	minStart := t.Spans[0].StartTime
	maxEnd := t.Spans[0].EndTime
	hasError := false

	for _, s := range t.Spans {
		if s.StartTime.Before(minStart) {
			minStart = s.StartTime
		}
		if s.EndTime.After(maxEnd) {
			maxEnd = s.EndTime
		}
		if s.HasError || s.StatusCode == "ERROR" {
			hasError = true
		}
	}

	t.StartTime = minStart
	t.EndTime = maxEnd
	if maxEnd.Before(minStart) {
		t.EndTime = minStart
	}
	t.Duration = t.EndTime.Sub(t.StartTime)
	t.SpanCount = len(t.Spans)
	t.HasError = hasError

	traceDurationSec := t.Duration.Seconds()
	if traceDurationSec <= 0 {
		traceDurationSec = 0.000001 // Prevent division by zero
	}

	// Build adjacency list for children
	children := make(map[string][]*Span)
	var roots []*Span

	for _, s := range t.Spans {
		// Reset critical path flag
		s.IsCriticalPath = false

		// Calculate relative offset and width
		offsetSec := s.StartTime.Sub(t.StartTime).Seconds()
		if offsetSec < 0 {
			offsetSec = 0
		}
		s.RelativeOffset = offsetSec / traceDurationSec
		if s.RelativeOffset > 1.0 {
			s.RelativeOffset = 1.0
		}

		widthSec := s.Duration.Seconds()
		if widthSec < 0 {
			widthSec = 0
		}
		s.RelativeWidth = widthSec / traceDurationSec
		if s.RelativeOffset+s.RelativeWidth > 1.0 {
			s.RelativeWidth = 1.0 - s.RelativeOffset
		}
		if s.RelativeWidth < 0 {
			s.RelativeWidth = 0
		}

		// Root detection: no parent ID, or parent ID is not in this trace
		if s.ParentSpanID == "" || t.SpansByID[s.ParentSpanID] == nil {
			roots = append(roots, s)
		} else {
			children[s.ParentSpanID] = append(children[s.ParentSpanID], s)
		}
	}

	// Sort roots and children by StartTime
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].StartTime.Before(roots[j].StartTime)
	})
	for parentID := range children {
		sort.Slice(children[parentID], func(i, j int) bool {
			return children[parentID][i].StartTime.Before(children[parentID][j].StartTime)
		})
	}

	// Mark Root Operation and Service Name
	if len(roots) > 0 {
		t.RootServiceName = roots[0].ServiceName
		t.RootOperationName = roots[0].Name
	} else if len(t.Spans) > 0 {
		t.RootServiceName = t.Spans[0].ServiceName
		t.RootOperationName = t.Spans[0].Name
	}

	// Calculate Critical Path
	// Find the root that has the longest duration or contributes most to trace duration
	var criticalRoot *Span
	var maxDuration time.Duration = -1
	for _, r := range roots {
		if r.Duration > maxDuration {
			maxDuration = r.Duration
			criticalRoot = r
		}
	}

	curr := criticalRoot
	for curr != nil {
		curr.IsCriticalPath = true
		currChildren := children[curr.SpanID]
		if len(currChildren) == 0 {
			break
		}
		// Pick child with the longest duration (the bottleneck child)
		var bestChild *Span
		var bestDur time.Duration = -1
		for _, c := range currChildren {
			if c.Duration > bestDur {
				bestDur = c.Duration
				bestChild = c
			}
		}
		curr = bestChild
	}

	// Perform DFS to create ordered SpanTree
	var tree []*Span
	var dfs func(s *Span, depth int)
	dfs = func(s *Span, depth int) {
		s.Depth = depth
		tree = append(tree, s)
		for _, child := range children[s.SpanID] {
			dfs(child, depth+1)
		}
	}

	for _, r := range roots {
		dfs(r, 0)
	}

	t.SpanTree = tree

	// Detect N+1 Database Queries
	t.detectNPlusOne()
}

func (t *Trace) detectNPlusOne() {
	t.HasNPlusOne = false
	t.NPlusOneSummary = ""

	queryGroups := make(map[string][]*Span)
	for _, s := range t.Spans {
		s.IsNPlusOne = false
		stmt := s.Attributes["db.statement"]
		if stmt == "" {
			upperName := strings.ToUpper(strings.TrimSpace(s.Name))
			if strings.HasPrefix(upperName, "SELECT ") ||
				strings.HasPrefix(upperName, "INSERT ") ||
				strings.HasPrefix(upperName, "UPDATE ") ||
				strings.HasPrefix(upperName, "DELETE ") {
				stmt = s.Name
			}
		}
		if stmt != "" {
			norm := normalizeStatement(stmt)
			key := s.ServiceName + "::" + norm
			queryGroups[key] = append(queryGroups[key], s)
		}
	}

	maxRepetitions := 0
	var worstSummary string

	for key, spans := range queryGroups {
		if len(spans) >= 3 {
			t.HasNPlusOne = true
			for _, s := range spans {
				s.IsNPlusOne = true
			}
			if len(spans) > maxRepetitions {
				maxRepetitions = len(spans)
				parts := strings.SplitN(key, "::", 2)
				querySnippet := parts[1]
				if len(querySnippet) > 40 {
					querySnippet = querySnippet[:37] + "..."
				}
				worstSummary = fmt.Sprintf("%d repeated DB queries on %s: %s", len(spans), parts[0], querySnippet)
			}
		}
	}

	t.NPlusOneSummary = worstSummary
}

func normalizeStatement(stmt string) string {
	stmt = strings.TrimSpace(stmt)
	var b strings.Builder
	inQuotes := false
	inNumber := false
	for _, r := range stmt {
		if r == '\'' || r == '"' {
			if !inQuotes {
				b.WriteString("?")
			}
			inQuotes = !inQuotes
			continue
		}
		if inQuotes {
			continue
		}
		if r >= '0' && r <= '9' {
			if !inNumber {
				b.WriteString("?")
				inNumber = true
			}
			continue
		}
		inNumber = false
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
