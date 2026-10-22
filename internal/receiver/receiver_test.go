package receiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"otel-tui/internal/store"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func createSampleTraceRequest(traceIDHex, spanIDHex, serviceName, spanName string) *coltracepb.ExportTraceServiceRequest {
	tid, _ := hex.DecodeString(traceIDHex)
	sid, _ := hex.DecodeString(spanIDHex)
	now := uint64(time.Now().UnixNano())

	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: serviceName}},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           tid,
								SpanId:            sid,
								Name:              spanName,
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: now,
								EndTimeUnixNano:   now + 50000000, // 50ms
								Status: &tracepb.Status{
									Code: tracepb.Status_STATUS_CODE_OK,
								},
								Attributes: []*commonpb.KeyValue{
									{
										Key:   "http.method",
										Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func createSampleLogsRequest(traceIDHex, spanIDHex, msg string) *collogspb.ExportLogsServiceRequest {
	tid, _ := hex.DecodeString(traceIDHex)
	sid, _ := hex.DecodeString(spanIDHex)
	now := uint64(time.Now().UnixNano())

	return &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano:   now,
								TraceId:        tid,
								SpanId:         sid,
								SeverityText:   "INFO",
								SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
								Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: msg}},
							},
						},
					},
				},
			},
		},
	}
}

func TestReceiver_HTTP_ProtobufAndJSON(t *testing.T) {
	st := store.NewStore(100)
	rec := NewReceiver(st, Config{GRPCPort: 0, HTTPPort: 0})
	if err := rec.Start(); err != nil {
		t.Fatalf("failed to start receiver: %v", err)
	}
	defer rec.Stop()

	httpBase := fmt.Sprintf("http://localhost:%d", rec.HTTPPort())

	// 1. Post Protobuf trace
	tid1 := "11111111222222223333333344444444"
	sid1 := "5555555566666666"
	reqPb := createSampleTraceRequest(tid1, sid1, "user-service", "GetUser")
	pbBytes, err := proto.Marshal(reqPb)
	if err != nil {
		t.Fatalf("failed to marshal proto: %v", err)
	}

	resp, err := http.Post(httpBase+"/v1/traces", "application/x-protobuf", bytes.NewReader(pbBytes))
	if err != nil {
		t.Fatalf("HTTP post failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	tr, ok := st.GetTrace(tid1)
	if !ok {
		t.Fatalf("trace %s not found in store", tid1)
	}
	if tr.RootServiceName != "user-service" {
		t.Errorf("expected user-service, got %s", tr.RootServiceName)
	}

	// 2. Post JSON trace
	tid2 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sid2 := "bbbbbbbbbbbbbbbb"
	reqJson := createSampleTraceRequest(tid2, sid2, "order-service", "CreateOrder")
	jsonBytes, err := protojson.Marshal(reqJson)
	if err != nil {
		t.Fatalf("failed to marshal protojson: %v", err)
	}

	resp, err = http.Post(httpBase+"/v1/traces", "application/json", bytes.NewReader(jsonBytes))
	if err != nil {
		t.Fatalf("HTTP JSON post failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	tr2, ok2 := st.GetTrace(tid2)
	if !ok2 {
		t.Fatalf("trace %s not found in store", tid2)
	}
	if tr2.RootOperationName != "CreateOrder" {
		t.Errorf("expected CreateOrder, got %s", tr2.RootOperationName)
	}

	// 3. Post Gzipped JSON logs
	logReq := createSampleLogsRequest(tid2, sid2, "Order created successfully")
	logJsonBytes, _ := protojson.Marshal(logReq)

	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	_, _ = gzWriter.Write(logJsonBytes)
	_ = gzWriter.Close()

	httpReq, _ := http.NewRequest("POST", httpBase+"/v1/logs", &gzBuf)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Content-Encoding", "gzip")

	client := &http.Client{Timeout: 5 * time.Second}
	logResp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("failed to send gzipped logs: %v", err)
	}
	if logResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for logs, got %d", logResp.StatusCode)
	}
	logResp.Body.Close()

	// Verify log attached to trace 2
	tr2After, _ := st.GetTrace(tid2)
	if len(tr2After.Logs) != 1 {
		t.Fatalf("expected 1 log attached to trace, got %d", len(tr2After.Logs))
	}
	if tr2After.Logs[0].Body != "Order created successfully" {
		t.Errorf("unexpected log body: %s", tr2After.Logs[0].Body)
	}
}

func TestReceiver_GRPC(t *testing.T) {
	st := store.NewStore(100)
	rec := NewReceiver(st, Config{GRPCPort: 0, HTTPPort: 0})
	if err := rec.Start(); err != nil {
		t.Fatalf("failed to start receiver: %v", err)
	}
	defer rec.Stop()

	target := fmt.Sprintf("127.0.0.1:%d", rec.GRPCPort())
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial grpc: %v", err)
	}
	defer conn.Close()

	traceClient := coltracepb.NewTraceServiceClient(conn)
	logsClient := collogspb.NewLogsServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tid := "99999999888888887777777766666666"
	sid := "1234567812345678"

	// 1. Export Trace via gRPC
	traceReq := createSampleTraceRequest(tid, sid, "payment-service", "ProcessPayment")
	_, err = traceClient.Export(ctx, traceReq)
	if err != nil {
		t.Fatalf("gRPC ExportTrace failed: %v", err)
	}

	tr, ok := st.GetTrace(tid)
	if !ok {
		t.Fatalf("trace %s not found in store after gRPC export", tid)
	}
	if tr.RootServiceName != "payment-service" {
		t.Errorf("expected payment-service, got %s", tr.RootServiceName)
	}

	// 2. Export Log via gRPC
	logsReq := createSampleLogsRequest(tid, sid, "Payment token validated")
	_, err = logsClient.Export(ctx, logsReq)
	if err != nil {
		t.Fatalf("gRPC ExportLogs failed: %v", err)
	}

	trAfter, _ := st.GetTrace(tid)
	if len(trAfter.Logs) != 1 {
		t.Fatalf("expected 1 correlated log, got %d", len(trAfter.Logs))
	}
	if trAfter.Logs[0].Body != "Payment token validated" {
		t.Errorf("unexpected log body: %s", trAfter.Logs[0].Body)
	}
}

func TestReceiver_PortFallback(t *testing.T) {
	// Occupy a wildcard port
	busyLis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to bind test listener: %v", err)
	}
	defer busyLis.Close()
	busyPort := busyLis.Addr().(*net.TCPAddr).Port

	st := store.NewStore(10)
	// Start with auto-fallback enabled
	rec := NewReceiver(st, Config{
		GRPCPort:         0,
		HTTPPort:         busyPort,
		AutoPortFallback: true,
	})

	if err := rec.Start(); err != nil {
		t.Fatalf("expected receiver to start with fallback port, got: %v", err)
	}
	defer rec.Stop()

	if rec.HTTPPort() == busyPort {
		t.Errorf("expected different fallback port than busy port %d, got %d", busyPort, rec.HTTPPort())
	}
}

func TestReceiver_ProxyForwarding(t *testing.T) {
	forwarded := make(chan string, 1)
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/v1/traces" {
			forwarded <- "received_trace"
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstreamServer.Close()

	st := store.NewStore(10)
	rec := NewReceiver(st, Config{
		GRPCPort:  0,
		HTTPPort:  0,
		ForwardTo: upstreamServer.URL,
	})
	if err := rec.Start(); err != nil {
		t.Fatalf("failed to start receiver: %v", err)
	}
	defer rec.Stop()

	// Post a trace to receiver
	tid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sid := "1111111111111111"
	reqJson := createSampleTraceRequest(tid, sid, "test-svc", "TestOp")
	jsonBytes, _ := protojson.Marshal(reqJson)

	httpURL := fmt.Sprintf("http://localhost:%d/v1/traces", rec.HTTPPort())
	resp, err := http.Post(httpURL, "application/json", bytes.NewReader(jsonBytes))
	if err != nil {
		t.Fatalf("failed to post trace: %v", err)
	}
	resp.Body.Close()

	select {
	case msg := <-forwarded:
		if msg != "received_trace" {
			t.Errorf("unexpected forwarded message: %s", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for upstream trace forwarding")
	}
}
