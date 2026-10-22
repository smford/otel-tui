package store

import (
	"fmt"
	"time"
)

// SpanDiffItem represents the comparison of a specific span between two traces.
type SpanDiffItem struct {
	ServiceName    string        `json:"service_name"`
	OperationName  string        `json:"operation_name"`
	ChangeType     string        `json:"change_type"` // "MATCHED", "ADDED", "REMOVED"
	BaseDuration   time.Duration `json:"base_duration"`
	TargetDuration time.Duration `json:"target_duration"`
	DeltaDuration  time.Duration `json:"delta_duration"`
	DeltaPercent   float64       `json:"delta_percent"`
	BaseStatus     string        `json:"base_status"`
	TargetStatus   string        `json:"target_status"`
	StatusChanged  bool          `json:"status_changed"`
}

// TraceDiff represents a full comparative analysis between two traces.
type TraceDiff struct {
	BaseTraceID     string         `json:"base_trace_id"`
	TargetTraceID   string         `json:"target_trace_id"`
	BaseDuration    time.Duration  `json:"base_duration"`
	TargetDuration  time.Duration  `json:"target_duration"`
	DeltaDuration   time.Duration  `json:"delta_duration"`
	DeltaPercent    float64        `json:"delta_percent"`
	BaseSpanCount   int            `json:"base_span_count"`
	TargetSpanCount int            `json:"target_span_count"`
	StatusChanged   bool           `json:"status_changed"`
	SpanDiffs       []SpanDiffItem `json:"span_diffs"`
}

// DiffTraces computes the performance and structural difference between two traces.
func DiffTraces(base, target *Trace) *TraceDiff {
	if base == nil || target == nil {
		return nil
	}

	deltaDur := target.Duration - base.Duration
	var deltaPct float64
	if base.Duration > 0 {
		deltaPct = (float64(deltaDur) / float64(base.Duration)) * 100.0
	}

	diff := &TraceDiff{
		BaseTraceID:     base.TraceID,
		TargetTraceID:   target.TraceID,
		BaseDuration:    base.Duration,
		TargetDuration:  target.Duration,
		DeltaDuration:   deltaDur,
		DeltaPercent:    deltaPct,
		BaseSpanCount:   base.SpanCount,
		TargetSpanCount: target.SpanCount,
		StatusChanged:   base.HasError != target.HasError,
	}

	// Match spans by ServiceName + OperationName
	baseSpans := make(map[string][]*Span)
	for _, s := range base.Spans {
		key := fmt.Sprintf("%s::%s", s.ServiceName, s.Name)
		baseSpans[key] = append(baseSpans[key], s)
	}

	targetSpans := make(map[string][]*Span)
	for _, s := range target.Spans {
		key := fmt.Sprintf("%s::%s", s.ServiceName, s.Name)
		targetSpans[key] = append(targetSpans[key], s)
	}

	// Track consumed spans to handle duplicates properly
	consumedBase := make(map[string]int)

	// Compare from target's perspective (ordered by target tree)
	for _, ts := range target.SpanTree {
		key := fmt.Sprintf("%s::%s", ts.ServiceName, ts.Name)
		bList := baseSpans[key]
		idx := consumedBase[key]

		if idx < len(bList) {
			bs := bList[idx]
			consumedBase[key] = idx + 1

			dSpan := ts.Duration - bs.Duration
			var dSpanPct float64
			if bs.Duration > 0 {
				dSpanPct = (float64(dSpan) / float64(bs.Duration)) * 100.0
			}

			diff.SpanDiffs = append(diff.SpanDiffs, SpanDiffItem{
				ServiceName:    ts.ServiceName,
				OperationName:  ts.Name,
				ChangeType:     "MATCHED",
				BaseDuration:   bs.Duration,
				TargetDuration: ts.Duration,
				DeltaDuration:  dSpan,
				DeltaPercent:   dSpanPct,
				BaseStatus:     bs.StatusCode,
				TargetStatus:   ts.StatusCode,
				StatusChanged:  bs.StatusCode != ts.StatusCode,
			})
		} else {
			// Added span in target
			diff.SpanDiffs = append(diff.SpanDiffs, SpanDiffItem{
				ServiceName:    ts.ServiceName,
				OperationName:  ts.Name,
				ChangeType:     "ADDED",
				TargetDuration: ts.Duration,
				DeltaDuration:  ts.Duration,
				TargetStatus:   ts.StatusCode,
				StatusChanged:  ts.StatusCode == "ERROR",
			})
		}
	}

	// Look for spans in base that are missing in target
	for key, bList := range baseSpans {
		consumed := consumedBase[key]
		for i := consumed; i < len(bList); i++ {
			bs := bList[i]
			diff.SpanDiffs = append(diff.SpanDiffs, SpanDiffItem{
				ServiceName:   bs.ServiceName,
				OperationName: bs.Name,
				ChangeType:    "REMOVED",
				BaseDuration:  bs.Duration,
				DeltaDuration: -bs.Duration,
				BaseStatus:    bs.StatusCode,
			})
		}
	}

	return diff
}
