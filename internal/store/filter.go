package store

import (
	"strconv"
	"strings"
	"time"
)

// MatchTrace evaluates whether a trace satisfies a search query string.
// Supports:
// - Free text substring match against TraceID, RootServiceName, RootOperationName, or any Span Name/Service
// - "error:true", "error:false", "status:error", "status:ok"
// - "service:<name>"
// - Duration filters: ">200ms", ">=1s", "<500ms", "<=2s", "> 100ms"
func MatchTrace(t *Trace, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}

	tokens := strings.Fields(query)
	for _, token := range tokens {
		if !matchToken(t, token) {
			return false
		}
	}
	return true
}

func matchToken(t *Trace, token string) bool {
	lowerToken := strings.ToLower(token)

	// Status / error filters
	if lowerToken == "error:true" || lowerToken == "status:error" || lowerToken == "is:error" || lowerToken == "error" {
		return t.HasError
	}
	if lowerToken == "error:false" || lowerToken == "status:ok" || lowerToken == "is:ok" {
		return !t.HasError
	}

	// Service filter
	if strings.HasPrefix(lowerToken, "service:") {
		svcTarget := strings.TrimPrefix(lowerToken, "service:")
		if strings.Contains(strings.ToLower(t.RootServiceName), svcTarget) {
			return true
		}
		for _, s := range t.Spans {
			if strings.Contains(strings.ToLower(s.ServiceName), svcTarget) {
				return true
			}
		}
		return false
	}

	// Duration filter: e.g. >200ms, >=100ms, <1s, <=500ms
	if len(token) > 1 && (token[0] == '>' || token[0] == '<') {
		op := ">"
		durStr := token[1:]
		if strings.HasPrefix(token, ">=") {
			op = ">="
			durStr = token[2:]
		} else if strings.HasPrefix(token, "<=") {
			op = "<="
			durStr = token[2:]
		} else if strings.HasPrefix(token, "<") {
			op = "<"
			durStr = token[1:]
		}

		dur, err := parseDurationLenient(durStr)
		if err == nil {
			switch op {
			case ">":
				return t.Duration > dur
			case ">=":
				return t.Duration >= dur
			case "<":
				return t.Duration < dur
			case "<=":
				return t.Duration <= dur
			}
		}
	}

	// Generic substring search
	if strings.Contains(strings.ToLower(t.TraceID), lowerToken) {
		return true
	}
	if strings.Contains(strings.ToLower(t.RootServiceName), lowerToken) {
		return true
	}
	if strings.Contains(strings.ToLower(t.RootOperationName), lowerToken) {
		return true
	}
	for _, s := range t.Spans {
		if strings.Contains(strings.ToLower(s.ServiceName), lowerToken) ||
			strings.Contains(strings.ToLower(s.Name), lowerToken) {
			return true
		}
	}

	return false
}

func parseDurationLenient(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	// If only a number was provided, default to milliseconds
	if num, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Duration(num) * time.Millisecond, nil
	}
	return time.ParseDuration(s)
}
