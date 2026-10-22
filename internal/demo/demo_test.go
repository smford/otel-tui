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

	// Give a small moment for processing
	time.Sleep(20 * time.Millisecond)

	// Verify trace in store
	tr, exists := st.GetTrace(traceID)
	if !exists {
		t.Fatalf("trace %s not found in store", traceID)
	}

	// Check Root Span details
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

	// Check tree assembly
	if len(tr.SpanTree) != 8 {
		t.Fatalf("expected SpanTree to have 8 spans, got %d", len(tr.SpanTree))
	}
	// Root span must be depth 0
	if tr.SpanTree[0].Depth != 0 {
		t.Errorf("root span depth should be 0, got %d", tr.SpanTree[0].Depth)
	}

	// Check critical path detection
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

	// Check correlated logs
	if len(tr.Logs) != 2 {
		t.Fatalf("expected 2 correlated logs on trace, got %d", len(tr.Logs))
	}

	// Check error span events
	foundException := false
	for _, s := range tr.Spans {
		for _, ev := range s.Events {
			if ev.Name == "exception" {
				foundException = true
				if ev.Attributes["exception.type"] != "StripeConnectionTimeoutException" {
					t.Errorf("unexpected exception type: %s", ev.Attributes["exception.type"])
				}
			}
		}
	}
	if !foundException {
		t.Errorf("expected to find exception event on stripe span")
	}
}
