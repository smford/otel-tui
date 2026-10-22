package demo

import (
	"fmt"
	"testing"
	"time"

	"otel-tui/internal/receiver"
	"otel-tui/internal/store"
)

func TestEndToEnd_DemoTracesIngestion(t *testing.T) {
	st := store.NewStore(100)
	rec := receiver.NewReceiver(st, receiver.Config{GRPCPort: 0, HTTPPort: 0})
	if err := rec.Start(); err != nil {
		t.Fatalf("failed to start receiver: %v", err)
	}
	defer rec.Stop()

	httpURL := fmt.Sprintf("http://localhost:%d", rec.HTTPPort())

	// Fire demo traces
	traceID, err := SendDemoTraces(httpURL)
	if err != nil {
		t.Fatalf("failed to send demo trace: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	tr, exists := st.GetTrace(traceID)
	if !exists {
		t.Fatalf("trace %s not found in store", traceID)
	}

	if tr.RootServiceName != "api-gateway" {
		t.Errorf("expected root service 'api-gateway', got '%s'", tr.RootServiceName)
	}
	if tr.RootOperationName != "POST /api/v1/checkout" {
		t.Errorf("expected root operation 'POST /api/v1/checkout', got '%s'", tr.RootOperationName)
	}
	if tr.SpanCount != 8 {
		t.Errorf("expected 8 spans in trace, got %d", tr.SpanCount)
	}
	if !tr.HasError {
		t.Errorf("expected trace to have HasError=true")
	}
	if !tr.HasNPlusOne {
		t.Errorf("expected trace to have HasNPlusOne=true")
	}

	if len(tr.SpanTree) != 8 {
		t.Fatalf("expected SpanTree to have 8 spans, got %d", len(tr.SpanTree))
	}
	if tr.SpanTree[0].Depth != 0 {
		t.Errorf("root span depth should be 0, got %d", tr.SpanTree[0].Depth)
	}

	hasCriticalPath := false
	for _, s := range tr.Spans {
		if s.IsCriticalPath {
			hasCriticalPath = true
			break
		}
	}
	if !hasCriticalPath {
		t.Errorf("expected at least one span to be marked as critical path")
	}

	if len(tr.Logs) != 2 {
		t.Fatalf("expected 2 correlated logs on trace, got %d", len(tr.Logs))
	}
}

func TestScenarios_DirectInjection(t *testing.T) {
	st := store.NewStore(50)

	for _, sc := range AllScenarios {
		tid := InjectSyntheticTrace(st, sc)
		if tid == "" {
			t.Errorf("failed to inject scenario: %s", sc)
			continue
		}
		tr, found := st.GetTrace(tid)
		if !found {
			t.Errorf("injected trace not found for scenario: %s", sc)
			continue
		}
		if tr.SpanCount == 0 {
			t.Errorf("expected spans for scenario %s", sc)
		}
	}

	if st.Count() != len(AllScenarios) {
		t.Errorf("expected %d traces stored, got %d", len(AllScenarios), st.Count())
	}
}
