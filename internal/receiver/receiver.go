package receiver

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"otel-tui/internal/store"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Config holds receiver server configurations.
type Config struct {
	GRPCPort         int
	HTTPPort         int
	ForwardTo        string // Upstream OTLP destination URL (e.g. "http://staging:4318")
	AutoPortFallback bool   // Try alternative ports if default port is occupied
}

// Receiver manages both gRPC and HTTP OTLP intake servers.
type Receiver struct {
	store            *store.Store
	grpcPort         int
	httpPort         int
	forwardTo        string
	autoPortFallback bool
	grpcServer       *grpc.Server
	httpServer       *http.Server
	grpcLis          net.Listener
	httpLis          net.Listener
	mu               sync.Mutex
	running          bool
}

type grpcTraceServer struct {
	coltracepb.UnimplementedTraceServiceServer
	receiver *Receiver
}

func (s *grpcTraceServer) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	if req != nil && len(req.ResourceSpans) > 0 {
		spans := store.SpansFromResourceSpans(req.ResourceSpans)
		s.receiver.store.AddSpans(spans)

		if s.receiver.forwardTo != "" {
			if pbBytes, err := proto.Marshal(req); err == nil {
				s.receiver.forwardPayload("/v1/traces", "application/x-protobuf", pbBytes)
			}
		}
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type grpcLogsServer struct {
	collogspb.UnimplementedLogsServiceServer
	receiver *Receiver
}

func (s *grpcLogsServer) Export(ctx context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	if req != nil && len(req.ResourceLogs) > 0 {
		logs := store.LogsFromResourceLogs(req.ResourceLogs)
		s.receiver.store.AddLogs(logs)

		if s.receiver.forwardTo != "" {
			if pbBytes, err := proto.Marshal(req); err == nil {
				s.receiver.forwardPayload("/v1/logs", "application/x-protobuf", pbBytes)
			}
		}
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}

// NewReceiver creates a new Receiver instance.
func NewReceiver(s *store.Store, cfg Config) *Receiver {
	if cfg.GRPCPort == 0 {
		cfg.GRPCPort = 4317
	}
	if cfg.HTTPPort == 0 {
		cfg.HTTPPort = 4318
	}
	return &Receiver{
		store:            s,
		grpcPort:         cfg.GRPCPort,
		httpPort:         cfg.HTTPPort,
		forwardTo:        cfg.ForwardTo,
		autoPortFallback: cfg.AutoPortFallback,
	}
}

func listenWithFallback(port int, autoFallback bool) (net.Listener, int, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err == nil {
		return lis, lis.Addr().(*net.TCPAddr).Port, nil
	}
	if !autoFallback && port != 0 {
		return nil, 0, err
	}
	fallbackPorts := []int{port + 10000, port + 1, 0}
	for _, fb := range fallbackPorts {
		fbLis, fbErr := net.Listen("tcp", fmt.Sprintf(":%d", fb))
		if fbErr == nil {
			return fbLis, fbLis.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return nil, 0, err
}

// Start binds and starts both the gRPC and HTTP OTLP listeners.
func (r *Receiver) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return nil
	}

	// 1. Start gRPC listener
	lis, actualGRPCPort, err := listenWithFallback(r.grpcPort, r.autoPortFallback)
	if err != nil {
		return fmt.Errorf("failed to bind gRPC port %d: %w", r.grpcPort, err)
	}
	r.grpcLis = lis
	r.grpcPort = actualGRPCPort

	r.grpcServer = grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(r.grpcServer, &grpcTraceServer{receiver: r})
	collogspb.RegisterLogsServiceServer(r.grpcServer, &grpcLogsServer{receiver: r})

	go func() {
		_ = r.grpcServer.Serve(r.grpcLis)
	}()

	// 2. Start HTTP listener
	httpLis, actualHTTPPort, err := listenWithFallback(r.httpPort, r.autoPortFallback)
	if err != nil {
		r.grpcServer.Stop()
		r.grpcLis.Close()
		return fmt.Errorf("failed to bind HTTP port %d: %w", r.httpPort, err)
	}
	r.httpLis = httpLis
	r.httpPort = actualHTTPPort

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", r.handleHTTPTraces)
	mux.HandleFunc("/v1/logs", r.handleHTTPLogs)

	r.httpServer = &http.Server{
		Handler: r.corsMiddleware(mux),
	}

	go func() {
		_ = r.httpServer.Serve(r.httpLis)
	}()

	r.running = true
	return nil
}

// GRPCPort returns the active gRPC listening port.
func (r *Receiver) GRPCPort() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.grpcPort
}

// HTTPPort returns the active HTTP listening port.
func (r *Receiver) HTTPPort() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.httpPort
}

// Stop gracefully shuts down both the gRPC and HTTP servers.
func (r *Receiver) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running {
		return
	}
	if r.grpcServer != nil {
		r.grpcServer.GracefulStop()
	}
	if r.httpServer != nil {
		_ = r.httpServer.Shutdown(context.Background())
	}
	r.running = false
}



// HTTP middleware for CORS handling
func (r *Receiver) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Encoding, x-otlp-version")

		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func readRequestBody(req *http.Request) ([]byte, error) {
	var reader io.Reader = req.Body
	defer req.Body.Close()

	if strings.EqualFold(req.Header.Get("Content-Encoding"), "gzip") {
		gzReader, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader error: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	return io.ReadAll(reader)
}

func (r *Receiver) handleHTTPTraces(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := readRequestBody(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusBadRequest)
		return
	}

	var traceReq coltracepb.ExportTraceServiceRequest
	contentType := req.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, &traceReq); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse JSON trace payload: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		// Default to protobuf
		if err := proto.Unmarshal(body, &traceReq); err != nil {
			// If protobuf parsing fails, attempt protojson as fallback
			if jsonErr := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, &traceReq); jsonErr != nil {
				http.Error(w, fmt.Sprintf("Failed to parse protobuf trace payload: %v", err), http.StatusBadRequest)
				return
			}
		}
	}

	spans := store.SpansFromResourceSpans(traceReq.ResourceSpans)
	r.store.AddSpans(spans)
	r.forwardPayload("/v1/traces", contentType, body)

	resp := &coltracepb.ExportTraceServiceResponse{}
	if strings.Contains(contentType, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		respBytes, _ := protojson.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBytes)
	} else {
		w.Header().Set("Content-Type", "application/x-protobuf")
		respBytes, _ := proto.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBytes)
	}
}

func (r *Receiver) handleHTTPLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := readRequestBody(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusBadRequest)
		return
	}

	var logsReq collogspb.ExportLogsServiceRequest
	contentType := req.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, &logsReq); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse JSON logs payload: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		if err := proto.Unmarshal(body, &logsReq); err != nil {
			if jsonErr := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, &logsReq); jsonErr != nil {
				http.Error(w, fmt.Sprintf("Failed to parse protobuf logs payload: %v", err), http.StatusBadRequest)
				return
			}
		}
	}

	logs := store.LogsFromResourceLogs(logsReq.ResourceLogs)
	r.store.AddLogs(logs)
	r.forwardPayload("/v1/logs", contentType, body)

	resp := &collogspb.ExportLogsServiceResponse{}
	if strings.Contains(contentType, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		respBytes, _ := protojson.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBytes)
	} else {
		w.Header().Set("Content-Type", "application/x-protobuf")
		respBytes, _ := proto.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(respBytes)
	}
}

func (r *Receiver) forwardPayload(path, contentType string, body []byte) {
	if r.forwardTo == "" || len(body) == 0 {
		return
	}
	targetURL := strings.TrimRight(r.forwardTo, "/") + path
	bodyCopy := make([]byte, len(body))
	copy(bodyCopy, body)

	go func() {
		req, err := http.NewRequest("POST", targetURL, bytes.NewReader(bodyCopy))
		if err != nil {
			return
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			_ = resp.Body.Close()
		}
	}()
}
