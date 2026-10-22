package tui

import (
	"strings"
	"testing"
	"time"

	"otel-tui/internal/demo"
	"otel-tui/internal/store"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func stripAnsi(s string) string {
	return ansi.Strip(s)
}

func createTestTrace() *store.Trace {
	t0 := time.Now()
	tr := &store.Trace{
		TraceID: "0123456789abcdef0123456789abcdef",
		Spans: []*store.Span{
			{
				TraceID:     "0123456789abcdef0123456789abcdef",
				SpanID:      "root-span",
				ServiceName: "api-gateway",
				Name:        "POST /api/v1/checkout",
				StartTime:   t0,
				EndTime:     t0.Add(100 * time.Millisecond),
				Duration:    100 * time.Millisecond,
				StatusCode:  "OK",
			},
			{
				TraceID:      "0123456789abcdef0123456789abcdef",
				SpanID:       "child-span-1",
				ParentSpanID: "root-span",
				ServiceName:  "auth-service",
				Name:         "POST /auth/verify",
				StartTime:    t0.Add(5 * time.Millisecond),
				EndTime:      t0.Add(30 * time.Millisecond),
				Duration:     25 * time.Millisecond,
				StatusCode:   "OK",
			},
			{
				TraceID:      "0123456789abcdef0123456789abcdef",
				SpanID:       "child-span-2",
				ParentSpanID: "root-span",
				ServiceName:  "payment-service",
				Name:         "Charge",
				StartTime:    t0.Add(35 * time.Millisecond),
				EndTime:      t0.Add(95 * time.Millisecond),
				Duration:     60 * time.Millisecond,
				StatusCode:   "ERROR",
				HasError:     true,
			},
			{
				TraceID:      "0123456789abcdef0123456789abcdef",
				SpanID:       "grandchild-span",
				ParentSpanID: "child-span-2",
				ServiceName:  "postgres-db",
				Name:         "SELECT * FROM accounts",
				StartTime:    t0.Add(40 * time.Millisecond),
				EndTime:      t0.Add(80 * time.Millisecond),
				Duration:     40 * time.Millisecond,
				StatusCode:   "OK",
			},
		},
	}
	tr.RebuildTrace()
	return tr
}

func TestWaterfall_BarHorizontalAlignment(t *testing.T) {
	tr := createTestTrace()
	width := 100
	rendered := RenderWaterfall(tr, 0, width, 20)

	lines := strings.Split(rendered, "\n")
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 lines, got %d", len(lines))
	}

	// Line 0 is header. Lines 1 to 4 are spans.
	var barStartCols []int
	for idx, line := range lines[1:] {
		plain := stripAnsi(line)
		t.Logf("row %d: %q", idx, plain)
		col := strings.LastIndex(plain, "[")
		if col == -1 {
			t.Errorf("row %d missing '[' timing bar start: %q", idx, plain)
			continue
		}
		barStartCols = append(barStartCols, col)
	}

	if len(barStartCols) < 4 {
		t.Fatalf("not enough span rows found")
	}

	// The first horizontal bar (root span) MUST be at the exact same column as all child bars!
	rootCol := barStartCols[0]
	for idx, col := range barStartCols[1:] {
		if col != rootCol {
			t.Errorf("span row %d bar start col %d does not match root bar start col %d", idx+1, col, rootCol)
		}
	}
}

func TestFlamegraph_NoExtraneousGlyphsAndProperWidth(t *testing.T) {
	tr := createTestTrace()
	width := 90
	rendered := RenderFlamegraph(tr, 0, width, 15)

	t.Logf("Rendered flamegraph:\n%s", rendered)

	lines := strings.Split(rendered, "\n")
	for idx, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > width+2 {
			t.Errorf("flamegraph line %d width %d exceeds target width %d: %q", idx, lineWidth, width, line)
		}

		if strings.Contains(line, "█auth") || strings.Contains(line, "█rate") {
			t.Errorf("flamegraph line %d has stray block glyphs touching text: %q", idx, line)
		}
	}
}

func TestFlamegraph_AuthFailureScenario(t *testing.T) {
	st := store.NewStore(10)
	tid := demo.InjectSyntheticTrace(st, demo.ScenarioAuthFailure)
	tr, _ := st.GetTrace(tid)
	rendered := RenderFlamegraph(tr, 0, 120, 15)
	t.Logf("Auth failure flamegraph:\n%s", rendered)
}

func TestServiceGraph_ColumnsAligned(t *testing.T) {
	st := store.NewStore(10)
	t0 := time.Now()
	st.AddSpans([]*store.Span{
		{
			TraceID:     "t1",
			SpanID:      "root",
			ServiceName: "short-svc",
			Name:        "GET /",
			StartTime:   t0,
			EndTime:     t0.Add(50 * time.Millisecond),
		},
		{
			TraceID:      "t1",
			SpanID:       "c1",
			ParentSpanID: "root",
			ServiceName:  "very-long-service-name-subsystem",
			Name:         "POST /action",
			StartTime:    t0.Add(10 * time.Millisecond),
			EndTime:      t0.Add(40 * time.Millisecond),
			Duration:     30 * time.Millisecond,
			StatusCode:   "ERROR",
		},
	})

	graph := st.BuildServiceGraph()
	rendered := RenderServiceGraphView(graph, 100, 20)
	lines := strings.Split(rendered, "\n")

	// Find the rows in SERVICES SUMMARY
	inTable := false
	var inboundCols []int
	for _, line := range lines {
		if strings.Contains(line, "SERVICE NAME") {
			inTable = true
			continue
		}
		if inTable {
			if strings.TrimSpace(line) == "" || strings.Contains(line, "DEPENDENCY CALL") {
				break
			}
			plain := stripAnsi(line)
			// Column 1 is 24 width + 2 indent = starts at col 26
			fields := strings.Fields(plain)
			if len(fields) >= 5 {
				// Index of the second field (INBOUND CALLS)
				colIdx := strings.Index(plain, fields[1])
				inboundCols = append(inboundCols, colIdx)
			}
		}
	}

	if len(inboundCols) >= 2 {
		if inboundCols[0] != inboundCols[1] {
			t.Errorf("inbound calls column misaligned: row 0 col %d vs row 1 col %d", inboundCols[0], inboundCols[1])
		}
	}
}
