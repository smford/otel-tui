package demo

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"otel-tui/internal/store"

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

// Scenario represents a synthetic trace template.
type Scenario string

const (
	ScenarioCheckout      Scenario = "checkout"
	ScenarioProductSearch Scenario = "search"
	ScenarioAuthFailure   Scenario = "auth-failure"
	ScenarioBackgroundJob Scenario = "background-job"
)

var AllScenarios = []Scenario{
	ScenarioCheckout,
	ScenarioProductSearch,
	ScenarioAuthFailure,
	ScenarioBackgroundJob,
}

// GenerateTraceByScenario creates a trace and correlated logs based on scenario.
func GenerateTraceByScenario(scenario Scenario) (*coltracepb.ExportTraceServiceRequest, *collogspb.ExportLogsServiceRequest) {
	switch scenario {
	case ScenarioProductSearch:
		return generateSearchTrace()
	case ScenarioAuthFailure:
		return generateAuthFailureTrace()
	case ScenarioBackgroundJob:
		return generateBackgroundJobTrace()
	case ScenarioCheckout:
		fallthrough
	default:
		return generateCheckoutTrace()
	}
}

// GenerateDemoTrace returns the default checkout trace.
func GenerateDemoTrace() (*coltracepb.ExportTraceServiceRequest, *collogspb.ExportLogsServiceRequest) {
	return generateCheckoutTrace()
}

// 1. Checkout Scenario: Gateway -> Auth -> Payment -> Postgres (N+1) -> Stripe (Error)
func generateCheckoutTrace() (*coltracepb.ExportTraceServiceRequest, *collogspb.ExportLogsServiceRequest) {
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

	apiGatewayResource := &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				kvString("service.name", "api-gateway"),
				kvString("host.name", "gw-us-east-1a"),
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
						Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
						Attributes: []*commonpb.KeyValue{
							kvString("rpc.service", "auth.v1.TokenVerifier"),
							kvString("user.id", "usr_998412"),
						},
					},
				},
			},
		},
	}

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
						},
					},
				},
			},
		},
	}

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
						Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
						Attributes: []*commonpb.KeyValue{
							kvString("db.statement", "SELECT * FROM payment_methods WHERE customer_id = $1 FOR UPDATE"),
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
						Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
						Attributes: []*commonpb.KeyValue{
							kvString("db.statement", "SELECT * FROM payment_methods WHERE customer_id = $2 FOR UPDATE"),
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
						Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
						Attributes: []*commonpb.KeyValue{
							kvString("db.statement", "SELECT * FROM payment_methods WHERE customer_id = $3 FOR UPDATE"),
						},
					},
				},
			},
		},
	}

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
						Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
						Attributes: []*commonpb.KeyValue{
							kvString("fraud.risk_tier", "LOW"),
						},
					},
				},
			},
		},
	}

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
							kvInt("http.status_code", 504),
						},
						Events: []*tracepb.Span_Event{
							{
								TimeUnixNano: toNano(185),
								Name:         "exception",
								Attributes: []*commonpb.KeyValue{
									kvString("exception.type", "StripeConnectionTimeoutException"),
									kvString("exception.message", "Connection timeout after 3 retries to api.stripe.com:443"),
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

	logsReq := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "postgres-db")}},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano: toNano(145),
								TraceId:      traceID,
								SpanId:       dbSpanID1,
								SeverityText: "WARN",
								Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "N+1 query pattern detected: 3 sequential queries on payment_methods"}},
							},
						},
					},
				},
			},
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "stripe-client")}},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano: toNano(185),
								TraceId:      traceID,
								SpanId:       stripeSpanID,
								SeverityText: "ERROR",
								Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Fatal upstream HTTP 504 Gateway Timeout from Stripe API"}},
							},
						},
					},
				},
			},
		},
	}

	return traceReq, logsReq
}

// 2. Search Scenario: Fast Product Search with Redis cache miss and Elasticsearch query (38ms, OK)
func generateSearchTrace() (*coltracepb.ExportTraceServiceRequest, *collogspb.ExportLogsServiceRequest) {
	traceID := randomID(16)
	rootSpanID := randomID(8)
	cacheSpanID := randomID(8)
	esSpanID := randomID(8)
	inventorySpanID := randomID(8)

	baseTime := time.Now().Add(-40 * time.Millisecond)
	toNano := func(offsetMs int64) uint64 {
		return uint64(baseTime.Add(time.Duration(offsetMs) * time.Millisecond).UnixNano())
	}

	traceReq := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "search-service")}},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            rootSpanID,
								Name:              "GET /api/v2/products/search?q=wireless+headphones",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: toNano(0),
								EndTimeUnixNano:   toNano(38),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvString("http.method", "GET"),
									kvInt("http.status_code", 200),
									kvInt("search.results_count", 42),
								},
							},
						},
					},
				},
			},
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "redis-cache")}},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            cacheSpanID,
								ParentSpanId:      rootSpanID,
								Name:              "GET search:cache:wireless+headphones",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: toNano(2),
								EndTimeUnixNano:   toNano(5),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvString("db.system", "redis"),
									kvString("db.operation", "GET"),
									kvString("cache.status", "MISS"),
								},
							},
						},
					},
				},
			},
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "elasticsearch")}},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            esSpanID,
								ParentSpanId:      rootSpanID,
								Name:              "POST /products/_search",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: toNano(6),
								EndTimeUnixNano:   toNano(28),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvString("db.system", "elasticsearch"),
									kvString("db.statement", "{\"query\": {\"match\": {\"title\": \"wireless headphones\"}}}"),
									kvInt("db.hits", 42),
								},
							},
						},
					},
				},
			},
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "inventory-service")}},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            inventorySpanID,
								ParentSpanId:      rootSpanID,
								Name:              "BatchCheckStock",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: toNano(29),
								EndTimeUnixNano:   toNano(36),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvInt("items.count", 42),
									kvString("inventory.warehouse", "us-east-central"),
								},
							},
						},
					},
				},
			},
		},
	}

	logsReq := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "search-service")}},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano: toNano(5),
								TraceId:      traceID,
								SpanId:       cacheSpanID,
								SeverityText: "DEBUG",
								Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Cache miss on query key 'search:cache:wireless+headphones'. Falling back to Elasticsearch."}},
							},
							{
								TimeUnixNano: toNano(37),
								TraceId:      traceID,
								SpanId:       rootSpanID,
								SeverityText: "INFO",
								Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Search completed in 38ms, returned 42 products"}},
							},
						},
					},
				},
			},
		},
	}

	return traceReq, logsReq
}

// 3. Auth Failure Scenario: Brute-force login attempt with 401 and Rate Limiter Warning
func generateAuthFailureTrace() (*coltracepb.ExportTraceServiceRequest, *collogspb.ExportLogsServiceRequest) {
	traceID := randomID(16)
	rootSpanID := randomID(8)
	bcryptSpanID := randomID(8)
	rateLimitSpanID := randomID(8)

	baseTime := time.Now().Add(-65 * time.Millisecond)
	toNano := func(offsetMs int64) uint64 {
		return uint64(baseTime.Add(time.Duration(offsetMs) * time.Millisecond).UnixNano())
	}

	traceReq := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "auth-service")}},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            rootSpanID,
								Name:              "POST /api/v1/auth/login",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: toNano(0),
								EndTimeUnixNano:   toNano(65),
								Status: &tracepb.Status{
									Code:    tracepb.Status_STATUS_CODE_ERROR,
									Message: "Invalid credentials (attempt 5/5)",
								},
								Attributes: []*commonpb.KeyValue{
									kvString("http.method", "POST"),
									kvInt("http.status_code", 401),
									kvString("user.email", "alex.smith@example.com"),
									kvString("client.ip", "203.0.113.42"),
								},
							},
							{
								TraceId:           traceID,
								SpanId:            bcryptSpanID,
								ParentSpanId:      rootSpanID,
								Name:              "bcrypt.CompareHashAndPassword",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: toNano(4),
								EndTimeUnixNano:   toNano(58),
								Status: &tracepb.Status{
									Code:    tracepb.Status_STATUS_CODE_ERROR,
									Message: "mismatched hash and password",
								},
								Attributes: []*commonpb.KeyValue{
									kvInt("bcrypt.cost", 12),
								},
							},
							{
								TraceId:           traceID,
								SpanId:            rateLimitSpanID,
								ParentSpanId:      rootSpanID,
								Name:              "rate_limiter.IncrementFailureCounter",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: toNano(59),
								EndTimeUnixNano:   toNano(64),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvInt("rate_limit.failed_attempts", 5),
									kvString("rate_limit.action", "TEMPORARY_ACCOUNT_LOCK_15M"),
								},
							},
						},
					},
				},
			},
		},
	}

	logsReq := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "auth-service")}},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano: toNano(58),
								TraceId:      traceID,
								SpanId:       bcryptSpanID,
								SeverityText: "WARN",
								Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Failed authentication attempt for alex.smith@example.com from 203.0.113.42"}},
							},
							{
								TimeUnixNano: toNano(64),
								TraceId:      traceID,
								SpanId:       rateLimitSpanID,
								SeverityText: "ERROR",
								Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Security Alert: Threshold exceeded (5 failed logins). Temporary lockout triggered for 15 minutes."}},
							},
						},
					},
				},
			},
		},
	}

	return traceReq, logsReq
}

// 4. Background Worker Scenario: Async document processing, S3 upload, Kafka event
func generateBackgroundJobTrace() (*coltracepb.ExportTraceServiceRequest, *collogspb.ExportLogsServiceRequest) {
	traceID := randomID(16)
	rootSpanID := randomID(8)
	downloadSpanID := randomID(8)
	resizeSpanID := randomID(8)
	s3SpanID := randomID(8)
	kafkaSpanID := randomID(8)

	baseTime := time.Now().Add(-120 * time.Millisecond)
	toNano := func(offsetMs int64) uint64 {
		return uint64(baseTime.Add(time.Duration(offsetMs) * time.Millisecond).UnixNano())
	}

	traceReq := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "media-worker")}},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            rootSpanID,
								Name:              "job:process_user_avatar",
								Kind:              tracepb.Span_SPAN_KIND_CONSUMER,
								StartTimeUnixNano: toNano(0),
								EndTimeUnixNano:   toNano(115),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvString("job.id", "job_884912"),
									kvString("user.id", "usr_10294"),
								},
							},
							{
								TraceId:           traceID,
								SpanId:            downloadSpanID,
								ParentSpanId:      rootSpanID,
								Name:              "DownloadSourceImage",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: toNano(5),
								EndTimeUnixNano:   toNano(25),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvInt("file.size_bytes", 4194304),
								},
							},
							{
								TraceId:           traceID,
								SpanId:            resizeSpanID,
								ParentSpanId:      rootSpanID,
								Name:              "GenerateThumbnails [webp 256x256]",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: toNano(26),
								EndTimeUnixNano:   toNano(72),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvString("image.format", "webp"),
									kvString("image.dimensions", "256x256"),
								},
							},
							{
								TraceId:           traceID,
								SpanId:            s3SpanID,
								ParentSpanId:      rootSpanID,
								Name:              "s3:PutObject avatars/usr_10294_thumb.webp",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: toNano(73),
								EndTimeUnixNano:   toNano(102),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvString("rpc.system", "aws_s3"),
									kvString("aws.bucket", "prod-media-assets"),
								},
							},
							{
								TraceId:           traceID,
								SpanId:            kafkaSpanID,
								ParentSpanId:      rootSpanID,
								Name:              "kafka:Produce media.avatar.processed",
								Kind:              tracepb.Span_SPAN_KIND_PRODUCER,
								StartTimeUnixNano: toNano(103),
								EndTimeUnixNano:   toNano(114),
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
								Attributes: []*commonpb.KeyValue{
									kvString("messaging.system", "kafka"),
									kvString("messaging.destination", "media.avatar.processed"),
								},
							},
						},
					},
				},
			},
		},
	}

	logsReq := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvString("service.name", "media-worker")}},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano: toNano(114),
								TraceId:      traceID,
								SpanId:       rootSpanID,
								SeverityText: "INFO",
								Body:         &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Avatar thumbnail generated and dispatched to Kafka topic 'media.avatar.processed'"}},
							},
						},
					},
				},
			},
		},
	}

	return traceReq, logsReq
}

// InjectSyntheticTrace directly injects a synthetic scenario into a store.Store without network calls.
func InjectSyntheticTrace(s *store.Store, scenario Scenario) string {
	traceReq, logsReq := GenerateTraceByScenario(scenario)
	spans := store.SpansFromResourceSpans(traceReq.ResourceSpans)
	logs := store.LogsFromResourceLogs(logsReq.ResourceLogs)

	s.AddSpans(spans)
	s.AddLogs(logs)

	if len(spans) > 0 {
		return spans[0].TraceID
	}
	return ""
}

// RandomScenario returns a random scenario.
func RandomScenario() Scenario {
	idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(AllScenarios))))
	return AllScenarios[idx.Int64()]
}

// SendDemoTraces sends synthetic sample traces to the specified HTTP OTLP endpoint.
func SendDemoTraces(httpBaseURL string) (string, error) {
	return SendDemoScenario(httpBaseURL, ScenarioCheckout)
}

// SendDemoScenario sends a specific synthetic scenario over HTTP OTLP.
func SendDemoScenario(httpBaseURL string, scenario Scenario) (string, error) {
	traceReq, logsReq := GenerateTraceByScenario(scenario)
	traceIDHex := hex.EncodeToString(traceReq.ResourceSpans[0].ScopeSpans[0].Spans[0].TraceId)

	client := &http.Client{Timeout: 5 * time.Second}

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
