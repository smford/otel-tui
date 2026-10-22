package store

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStore_RingBufferCapacity(t *testing.T) {
	capacity := 5
	s := NewStore(capacity)

	baseTime := time.Now()
	for i := 0; i < 10; i++ {
		traceID := fmt.Sprintf("trace-%02d", i)
		s.AddSpans([]*Span{
			{
				TraceID:     traceID,
				SpanID:      fmt.Sprintf("span-%02d", i),
				ServiceName: "api-gw",
				Name:        "GET /test",
				StartTime:   baseTime.Add(time.Duration(i) * time.Second),
				EndTime:     baseTime.Add(time.Duration(i)*time.Second + 50*time.Millisecond),
			},
		})
	}

	if count := s.Count(); count != capacity {
		t.Fatalf("expected store count %d, got %d", capacity, count)
	}

	// Traces 0 to 4 should have been evicted; traces 5 to 9 should exist
	for i := 0; i < 5; i++ {
		traceID := fmt.Sprintf("trace-%02d", i)
		if _, exists := s.GetTrace(traceID); exists {
			t.Errorf("trace %s should have been evicted", traceID)
		}
	}
	for i := 5; i < 10; i++ {
		traceID := fmt.Sprintf("trace-%02d", i)
		if _, exists := s.GetTrace(traceID); !exists {
			t.Errorf("trace %s should exist in store", traceID)
		}
	}
}

func TestStore_OutOfOrderSpanAssembly(t *testing.T) {
	s := NewStore(10)
	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	t0 := time.Now()

	// Child span arrives FIRST
	childSpan := &Span{
		TraceID:      traceID,
		SpanID:       "child-span-01",
		ParentSpanID: "root-span-01",
		ServiceName:  "auth-service",
		Name:         "Authenticate",
		StartTime:    t0.Add(10 * time.Millisecond),
		EndTime:      t0.Add(30 * time.Millisecond),
		Duration:     20 * time.Millisecond,
		StatusCode:   "OK",
	}
	s.AddSpans([]*Span{childSpan})

	tr, found := s.GetTrace(traceID)
	if !found {
		t.Fatal("trace not found after adding child span")
	}
	if tr.SpanCount != 1 {
		t.Fatalf("expected 1 span, got %d", tr.SpanCount)
	}

	// Root span arrives SECOND
	rootSpan := &Span{
		TraceID:      traceID,
		SpanID:       "root-span-01",
		ParentSpanID: "",
		ServiceName:  "api-gateway",
		Name:         "POST /login",
		StartTime:    t0,
		EndTime:      t0.Add(50 * time.Millisecond),
		Duration:     50 * time.Millisecond,
		StatusCode:   "OK",
	}
	s.AddSpans([]*Span{rootSpan})

	tr, _ = s.GetTrace(traceID)
	if tr.SpanCount != 2 {
		t.Fatalf("expected 2 spans, got %d", tr.SpanCount)
	}
	if tr.RootServiceName != "api-gateway" {
		t.Errorf("expected root service api-gateway, got %s", tr.RootServiceName)
	}
	if tr.RootOperationName != "POST /login" {
		t.Errorf("expected root operation POST /login, got %s", tr.RootOperationName)
	}
	if tr.Duration != 50*time.Millisecond {
		t.Errorf("expected trace duration 50ms, got %v", tr.Duration)
	}

	// Verify SpanTree ordering: root first (depth 0), child second (depth 1)
	if len(tr.SpanTree) != 2 {
		t.Fatalf("expected SpanTree length 2, got %d", len(tr.SpanTree))
	}
	if tr.SpanTree[0].SpanID != "root-span-01" || tr.SpanTree[0].Depth != 0 {
		t.Errorf("expected root span at depth 0, got id=%s depth=%d", tr.SpanTree[0].SpanID, tr.SpanTree[0].Depth)
	}
	if tr.SpanTree[1].SpanID != "child-span-01" || tr.SpanTree[1].Depth != 1 {
		t.Errorf("expected child span at depth 1, got id=%s depth=%d", tr.SpanTree[1].SpanID, tr.SpanTree[1].Depth)
	}
}

func TestStore_CriticalPathAndErrors(t *testing.T) {
	s := NewStore(10)
	traceID := "critical-trace-01"
	t0 := time.Now()

	// Root span: 0 to 100ms
	root := &Span{
		TraceID:     traceID,
		SpanID:      "root",
		ServiceName: "gateway",
		Name:        "HTTP GET",
		StartTime:   t0,
		EndTime:     t0.Add(100 * time.Millisecond),
		Duration:    100 * time.Millisecond,
	}

	// Child 1: 5ms to 20ms (15ms duration)
	child1 := &Span{
		TraceID:      traceID,
		SpanID:       "child1",
		ParentSpanID: "root",
		ServiceName:  "cache",
		Name:         "cache-lookup",
		StartTime:    t0.Add(5 * time.Millisecond),
		EndTime:      t0.Add(20 * time.Millisecond),
		Duration:     15 * time.Millisecond,
	}

	// Child 2: 25ms to 95ms (70ms duration) -> Should be critical path
	child2 := &Span{
		TraceID:      traceID,
		SpanID:       "child2",
		ParentSpanID: "root",
		ServiceName:  "db",
		Name:         "sql-query",
		StartTime:    t0.Add(25 * time.Millisecond),
		EndTime:      t0.Add(95 * time.Millisecond),
		Duration:     70 * time.Millisecond,
		StatusCode:   "ERROR",
		HasError:     true,
	}

	s.AddSpans([]*Span{root, child1, child2})

	tr, ok := s.GetTrace(traceID)
	if !ok {
		t.Fatal("trace not found")
	}

	if !tr.HasError {
		t.Error("expected trace to have HasError=true")
	}

	rootSpan := tr.SpansByID["root"]
	c1Span := tr.SpansByID["child1"]
	c2Span := tr.SpansByID["child2"]

	if !rootSpan.IsCriticalPath {
		t.Error("root should be on critical path")
	}
	if c1Span.IsCriticalPath {
		t.Error("child1 should not be on critical path")
	}
	if !c2Span.IsCriticalPath {
		t.Error("child2 should be on critical path")
	}
}

func TestStore_CorrelatedLogs(t *testing.T) {
	s := NewStore(10)
	traceID := "log-trace-01"
	t0 := time.Now()

	span := &Span{
		TraceID:     traceID,
		SpanID:      "span-01",
		ServiceName: "worker",
		Name:        "ProcessJob",
		StartTime:   t0,
		EndTime:     t0.Add(100 * time.Millisecond),
	}
	s.AddSpans([]*Span{span})

	s.AddLogs([]LogRecord{
		{
			TraceID:   traceID,
			SpanID:    "span-01",
			Timestamp: t0.Add(10 * time.Millisecond),
			Severity:  "INFO",
			Body:      "Job started",
		},
		{
			TraceID:   traceID,
			SpanID:    "span-01",
			Timestamp: t0.Add(50 * time.Millisecond),
			Severity:  "ERROR",
			Body:      "Connection timeout to queue",
		},
	})

	tr, _ := s.GetTrace(traceID)
	if len(tr.Logs) != 2 {
		t.Fatalf("expected 2 trace logs, got %d", len(tr.Logs))
	}

	updatedSpan := tr.SpansByID["span-01"]
	if len(updatedSpan.Logs) != 2 {
		t.Fatalf("expected 2 span logs, got %d", len(updatedSpan.Logs))
	}
	if updatedSpan.Logs[0].Body != "Job started" {
		t.Errorf("unexpected log 0 body: %s", updatedSpan.Logs[0].Body)
	}
}

func TestStore_Filtering(t *testing.T) {
	s := NewStore(10)
	t0 := time.Now()

	s.AddSpans([]*Span{
		{
			TraceID:     "t1",
			SpanID:      "s1",
			ServiceName: "frontend",
			Name:        "GET /index",
			StartTime:   t0,
			EndTime:     t0.Add(50 * time.Millisecond),
			Duration:    50 * time.Millisecond,
			StatusCode:  "OK",
		},
		{
			TraceID:     "t2",
			SpanID:      "s2",
			ServiceName: "backend",
			Name:        "POST /checkout",
			StartTime:   t0,
			EndTime:     t0.Add(350 * time.Millisecond),
			Duration:    350 * time.Millisecond,
			StatusCode:  "ERROR",
			HasError:    true,
		},
	})

	// Match all
	if list := s.ListTraces(""); len(list) != 2 {
		t.Errorf("expected 2 traces, got %d", len(list))
	}

	// Match error
	errList := s.ListTraces("error:true")
	if len(errList) != 1 || errList[0].TraceID != "t2" {
		t.Errorf("expected only t2 in error filter, got %v", errList)
	}

	// Match service
	feList := s.ListTraces("service:frontend")
	if len(feList) != 1 || feList[0].TraceID != "t1" {
		t.Errorf("expected only t1 in service:frontend, got %v", feList)
	}

	// Match duration >200ms
	durList := s.ListTraces(">200ms")
	if len(durList) != 1 || durList[0].TraceID != "t2" {
		t.Errorf("expected only t2 in >200ms filter, got %v", durList)
	}
}

func TestStore_ExportAndImport(t *testing.T) {
	s := NewStore(10)
	t0 := time.Now()

	s.AddSpans([]*Span{
		{
			TraceID:     "export-t1",
			SpanID:      "s1",
			ServiceName: "payment",
			Name:        "Authorize",
			StartTime:   t0,
			EndTime:     t0.Add(80 * time.Millisecond),
			Duration:    80 * time.Millisecond,
		},
	})

	data, err := s.ExportTraceJSON("export-t1")
	if err != nil {
		t.Fatalf("failed to export trace: %v", err)
	}

	newStore := NewStore(10)
	if err := newStore.ImportTraceJSON(data); err != nil {
		t.Fatalf("failed to import trace: %v", err)
	}

	tr, ok := newStore.GetTrace("export-t1")
	if !ok {
		t.Fatal("imported trace not found")
	}
	if tr.RootServiceName != "payment" {
		t.Errorf("expected service payment, got %s", tr.RootServiceName)
	}
}

func TestStore_Concurrency(t *testing.T) {
	s := NewStore(500)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			base := time.Now()
			for j := 0; j < 50; j++ {
				traceID := fmt.Sprintf("trace-%d-%d", workerID, j)
				s.AddSpans([]*Span{
					{
						TraceID:     traceID,
						SpanID:      "span-0",
						ServiceName: fmt.Sprintf("svc-%d", workerID),
						Name:        "op",
						StartTime:   base,
						EndTime:     base.Add(10 * time.Millisecond),
					},
				})
				_ = s.ListTraces("op")
			}
		}(i)
	}

	wg.Wait()
	if s.Count() == 0 {
		t.Error("expected non-zero traces stored")
	}
}

func TestStore_NPlusOneDetector(t *testing.T) {
	s := NewStore(10)
	t0 := time.Now()
	traceID := "nplusone-trace"

	spans := []*Span{
		{
			TraceID:     traceID,
			SpanID:      "root",
			ServiceName: "web",
			Name:        "GET /users",
			StartTime:   t0,
			EndTime:     t0.Add(100 * time.Millisecond),
		},
	}

	// Add 4 repetitive queries matching parameterized SQL
	for i := 0; i < 4; i++ {
		spans = append(spans, &Span{
			TraceID:      traceID,
			SpanID:       fmt.Sprintf("db-%d", i),
			ParentSpanID: "root",
			ServiceName:  "postgres",
			Name:         "SELECT",
			StartTime:    t0.Add(time.Duration(10+i*10) * time.Millisecond),
			EndTime:      t0.Add(time.Duration(15+i*10) * time.Millisecond),
			Duration:     5 * time.Millisecond,
			Attributes: map[string]string{
				"db.statement": fmt.Sprintf("SELECT * FROM profiles WHERE user_id = %d", i+1),
			},
		})
	}

	s.AddSpans(spans)
	tr, ok := s.GetTrace(traceID)
	if !ok {
		t.Fatal("trace not found")
	}

	if !tr.HasNPlusOne {
		t.Errorf("expected trace to be flagged with HasNPlusOne=true")
	}
	if tr.NPlusOneSummary == "" {
		t.Errorf("expected NPlusOneSummary to be populated")
	}

	for i := 1; i <= 4; i++ {
		if !tr.SpanTree[i].IsNPlusOne {
			t.Errorf("expected span %s to be flagged as NPlusOne", tr.SpanTree[i].SpanID)
		}
	}
}

func TestStore_DiffTraces(t *testing.T) {
	t0 := time.Now()
	base := &Trace{
		TraceID:  "base-1",
		Duration: 100 * time.Millisecond,
		Spans: []*Span{
			{ServiceName: "api", Name: "GET /item", StartTime: t0, EndTime: t0.Add(100 * time.Millisecond), Duration: 100 * time.Millisecond, StatusCode: "OK"},
			{ServiceName: "db", Name: "query", StartTime: t0.Add(10 * time.Millisecond), EndTime: t0.Add(40 * time.Millisecond), Duration: 30 * time.Millisecond, StatusCode: "OK"},
		},
	}
	base.RebuildTrace()

	target := &Trace{
		TraceID:  "target-1",
		Duration: 250 * time.Millisecond,
		Spans: []*Span{
			{ServiceName: "api", Name: "GET /item", StartTime: t0, EndTime: t0.Add(250 * time.Millisecond), Duration: 250 * time.Millisecond, StatusCode: "ERROR"},
			{ServiceName: "db", Name: "query", StartTime: t0.Add(10 * time.Millisecond), EndTime: t0.Add(190 * time.Millisecond), Duration: 180 * time.Millisecond, StatusCode: "ERROR"},
			{ServiceName: "cache", Name: "miss", StartTime: t0.Add(200 * time.Millisecond), EndTime: t0.Add(210 * time.Millisecond), Duration: 10 * time.Millisecond, StatusCode: "OK"},
		},
	}
	target.RebuildTrace()

	diff := DiffTraces(base, target)
	if diff == nil {
		t.Fatal("expected non-nil diff")
	}
	if diff.DeltaDuration != 150*time.Millisecond {
		t.Errorf("expected delta duration 150ms, got %v", diff.DeltaDuration)
	}
	if !diff.StatusChanged {
		t.Errorf("expected status changed to be true")
	}
	if len(diff.SpanDiffs) != 3 {
		t.Fatalf("expected 3 span diffs, got %d", len(diff.SpanDiffs))
	}
}

func TestStore_ServiceGraph(t *testing.T) {
	s := NewStore(10)
	t0 := time.Now()

	s.AddSpans([]*Span{
		{
			TraceID:     "graph-t1",
			SpanID:      "root",
			ServiceName: "frontend",
			Name:        "HTTP GET /home",
			StartTime:   t0,
			EndTime:     t0.Add(50 * time.Millisecond),
		},
		{
			TraceID:      "graph-t1",
			SpanID:       "auth",
			ParentSpanID: "root",
			ServiceName:  "auth-svc",
			Name:         "VerifySession",
			StartTime:    t0.Add(5 * time.Millisecond),
			EndTime:      t0.Add(25 * time.Millisecond),
			Duration:     20 * time.Millisecond,
		},
		{
			TraceID:      "graph-t1",
			SpanID:       "db",
			ParentSpanID: "auth",
			ServiceName:  "user-db",
			Name:         "SELECT session",
			StartTime:    t0.Add(10 * time.Millisecond),
			EndTime:      t0.Add(20 * time.Millisecond),
			Duration:     10 * time.Millisecond,
		},
	})

	graph := s.BuildServiceGraph()
	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 service nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("expected 2 service edges, got %d", len(graph.Edges))
	}

	// Verify frontend -> auth-svc
	foundEdge := false
	for _, e := range graph.Edges {
		if e.FromService == "frontend" && e.ToService == "auth-svc" {
			foundEdge = true
			if e.CallCount != 1 {
				t.Errorf("expected 1 call count, got %d", e.CallCount)
			}
		}
	}
	if !foundEdge {
		t.Errorf("expected edge frontend -> auth-svc")
	}
}
