package demo

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func randomID(bytesCount int) []byte {
	b := make([]byte, bytesCount)
	_, _ = rand.Read(b)
	return b
}

func kvString(key, val string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
	}
}

func kvInt(key string, val int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: val}},
	}
}

// GenerateDemoTrace constructs a rich, realistic multi-service trace and correlated logs.
func GenerateDemoTrace() (*coltracepb.ExportTraceServiceRequest, *collogspb.ExportLogsServiceRequest) {
	traceID := randomID(16)
	rootSpanID := randomID(8)
	authSpanID := randomID(8)
	paymentSpanID := randomID(8)
	dbSpanID1 := randomID(8)
	dbSpanID2 := randomID(8)
	dbSpanID3 := randomID(8)
	fraudSpanID := randomID(8)
	stripeSpanID := randomID(8)

	baseTime := time.Now().Add(-200 * time.Millisecond)
	toNano := func(offsetMs int64) uint64 {
		return uint64(baseTime.Add(time.Duration(offsetMs) * time.Millisecond).UnixNano())
	}

	// 1. API Gateway (Root)
	apiGatewayResource := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				kvString("service.name", "api-gateway"),
				kvString("host.name", "gw-us-east-1a"),
				kvString("telemetry.sdk.language", "go"),
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           traceID,
						SpanId:            rootSpanID,
						Name:              "POST /api/v1/checkout",
						Kind:              tracepb.Span_SPAN_KIND_SERVER,
						StartTimeUnixNano: toNano(0),
						EndTimeUnixNano:   toNano(190),
						Status: &tracepb.Status{
							Code:    tracepb.Status_STATUS_CODE_ERROR,
							Message: "Internal Server Error in Payment Subsystem",
						},
						Attributes: []*commonpb.KeyValue{
							kvString("http.method", "POST"),
							kvString("http.route", "/api/v1/checkout"),
							kvInt("http.status_code", 500),
							kvString("client.ip", "192.168.1.105"),
						},
					},
				},
			},
		},
	}

	// 2. Auth Service (Child of Root)
	authResource := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				kvString("service.name", "auth-service"),
				kvString("host.name", "auth-worker-03"),
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           traceID,
						SpanId:            authSpanID,
						ParentSpanId:      rootSpanID,
						Name:              "POST /auth/verify-token",
						Kind:              tracepb.Span_SPAN_KIND_SERVER,
						StartTimeUnixNano: toNano(5),
						EndTimeUnixNano:   toNano(28),
						Status: &tracepb.Status{
							Code: tracepb.Status_STATUS_CODE_OK,
						},
						Attributes: []*commonpb.KeyValue{
							kvString("rpc.service", "auth.v1.TokenVerifier"),
							kvString("user.id", "usr_998412"),
							kvString("auth.role", "premium_customer"),
						},
					},
				},
			},
		},
	}

	// 3. Payment Service (Child of Root)
	paymentResource := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				kvString("service.name", "payment-service"),
				kvString("host.name", "pay-core-01"),
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           traceID,
						SpanId:            paymentSpanID,
						ParentSpanId:      rootSpanID,
						Name:              "ProcessCheckoutPayment",
						Kind:              tracepb.Span_SPAN_KIND_SERVER,
						StartTimeUnixNano: toNano(32),
						EndTimeUnixNano:   toNano(188),
						Status: &tracepb.Status{
							Code:    tracepb.Status_STATUS_CODE_ERROR,
							Message: "Payment processing failed",
						},
						Attributes: []*commonpb.KeyValue{
							kvString("payment.currency", "USD"),
							kvString("payment.amount", "149.99"),
							kvString("payment.method", "credit_card"),
						},
					},
				},
			},
		},
	}

	// 4. Postgres DB (Child of Payment Service) - Repetitive N+1 queries
	dbResource := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				kvString("service.name", "postgres-db"),
				kvString("db.system", "postgresql"),
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           traceID,
						SpanId:            dbSpanID1,
						ParentSpanId:      paymentSpanID,
						Name:              "SELECT * FROM payment_methods WHERE customer_id = $1",
						Kind:              tracepb.Span_SPAN_KIND_CLIENT,
						StartTimeUnixNano: toNano(36),
						EndTimeUnixNano:   toNano(76),
						Status: &tracepb.Status{
							Code: tracepb.Status_STATUS_CODE_OK,
						},
						Attributes: []*commonpb.KeyValue{
							kvString("db.name", "payments_master"),
							kvString("db.statement", "SELECT * FROM payment_methods WHERE customer_id = $1 FOR UPDATE"),
							kvInt("db.rows_returned", 1),
						},
					},
					{
						TraceId:           traceID,
						SpanId:            dbSpanID2,
						ParentSpanId:      paymentSpanID,
						Name:              "SELECT * FROM payment_methods WHERE customer_id = $2",
						Kind:              tracepb.Span_SPAN_KIND_CLIENT,
						StartTimeUnixNano: toNano(78),
						EndTimeUnixNano:   toNano(112),
						Status: &tracepb.Status{
							Code: tracepb.Status_STATUS_CODE_OK,
						},
						Attributes: []*commonpb.KeyValue{
							kvString("db.name", "payments_master"),
							kvString("db.statement", "SELECT * FROM payment_methods WHERE customer_id = $2 FOR UPDATE"),
							kvInt("db.rows_returned", 1),
						},
					},
					{
						TraceId:           traceID,
						SpanId:            dbSpanID3,
						ParentSpanId:      paymentSpanID,
						Name:              "SELECT * FROM payment_methods WHERE customer_id = $3",
						Kind:              tracepb.Span_SPAN_KIND_CLIENT,
						StartTimeUnixNano: toNano(114),
						EndTimeUnixNano:   toNano(148),
						Status: &tracepb.Status{
							Code: tracepb.Status_STATUS_CODE_OK,
						},
						Attributes: []*commonpb.KeyValue{
							kvString("db.name", "payments_master"),
							kvString("db.statement", "SELECT * FROM payment_methods WHERE customer_id = $3 FOR UPDATE"),
							kvInt("db.rows_returned", 1),
						},
					},
				},
			},
		},
	}

	// 5. Fraud Detection Service (Child of Payment Service)
	fraudResource := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				kvString("service.name", "fraud-detection"),
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           traceID,
						SpanId:            fraudSpanID,
						ParentSpanId:      paymentSpanID,
						Name:              "EvaluateTransactionRisk",
						Kind:              tracepb.Span_SPAN_KIND_CLIENT,
						StartTimeUnixNano: toNano(150),
						EndTimeUnixNano:   toNano(168),
						Status: &tracepb.Status{
							Code: tracepb.Status_STATUS_CODE_OK,
						},
						Attributes: []*commonpb.KeyValue{
							kvString("fraud.risk_tier", "LOW"),
							kvString("fraud.score", "0.04"),
						},
					},
				},
			},
		},
	}

	// 6. External Stripe Gateway Call (Child of Payment Service) - Fails with exception
	stripeResource := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				kvString("service.name", "stripe-client"),
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: []*tracepb.Span{
					{
						TraceId:           traceID,
						SpanId:            stripeSpanID,
						ParentSpanId:      paymentSpanID,
						Name:              "POST https://api.stripe.com/v1/charges",
						Kind:              tracepb.Span_SPAN_KIND_CLIENT,
						StartTimeUnixNano: toNano(170),
						EndTimeUnixNano:   toNano(186),
						Status: &tracepb.Status{
							Code:    tracepb.Status_STATUS_CODE_ERROR,
							Message: "Upstream payment provider gateway timeout",
						},
						Attributes: []*commonpb.KeyValue{
							kvString("http.url", "https://api.stripe.com/v1/charges"),
							kvInt("http.status_code", 504),
						},
						Events: []*tracepb.Span_Event{
							{
								TimeUnixNano: toNano(185),
								Name:         "exception",
								Attributes: []*commonpb.KeyValue{
									kvString("exception.type", "StripeConnectionTimeoutException"),
									kvString("exception.message", "Connection timeout after 3 retries to api.stripe.com:443"),
									kvString("exception.stacktrace", "stripe.go:142\nclient.go:88\npayment.go:56"),
								},
							},
						},
					},
				},
			},
		},
	}

	traceReq := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			apiGatewayResource,
			authResource,
			paymentResource,
			dbResource,
			fraudResource,
			stripeResource,
		},
	}

	// Correlated Logs
	logsReq := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						kvString("service.name", "postgres-db"),
					},
				},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano:   toNano(145),
								TraceId:        traceID,
								SpanId:         dbSpanID1,
								SeverityText:   "WARN",
								SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_WARN,
								Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Slow query detected (112ms > 50ms SLA) on customer_id lock"}},
							},
						},
					},
				},
			},
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						kvString("service.name", "stripe-client"),
					},
				},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano:   toNano(185),
								TraceId:        traceID,
								SpanId:         stripeSpanID,
								SeverityText:   "ERROR",
								SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
								Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Fatal upstream HTTP 504 Gateway Timeout from Stripe API"}},
							},
						},
					},
				},
			},
		},
	}

	return traceReq, logsReq
}

// SendDemoTraces sends synthetic sample traces to the specified HTTP OTLP endpoint.
func SendDemoTraces(httpBaseURL string) (string, error) {
	traceReq, logsReq := GenerateDemoTrace()
	traceIDHex := hex.EncodeToString(traceReq.ResourceSpans[0].ScopeSpans[0].Spans[0].TraceId)

	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Send Traces
	traceBytes, err := proto.Marshal(traceReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal trace request: %w", err)
	}

	traceURL := fmt.Sprintf("%s/v1/traces", httpBaseURL)
	resp, err := client.Post(traceURL, "application/x-protobuf", bytes.NewReader(traceBytes))
	if err != nil {
		return "", fmt.Errorf("failed to POST trace to %s: %w", traceURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("trace POST returned status %d", resp.StatusCode)
	}

	// 2. Send Correlated Logs
	logBytes, err := proto.Marshal(logsReq)
	if err != nil {
		return traceIDHex, fmt.Errorf("failed to marshal logs request: %w", err)
	}

	logsURL := fmt.Sprintf("%s/v1/logs", httpBaseURL)
	logResp, err := client.Post(logsURL, "application/x-protobuf", bytes.NewReader(logBytes))
	if err != nil {
		return traceIDHex, fmt.Errorf("failed to POST logs to %s: %w", logsURL, err)
	}
	logResp.Body.Close()
	if logResp.StatusCode != http.StatusOK {
		return traceIDHex, fmt.Errorf("logs POST returned status %d", logResp.StatusCode)
	}

	return traceIDHex, nil
}
