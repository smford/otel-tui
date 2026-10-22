package store

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// SpanEvent represents an event within a span (e.g. exception).
type SpanEvent struct {
	Name       string            `json:"name"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// LogRecord represents an OTLP log record correlated with a trace/span.
type LogRecord struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Timestamp  time.Time         `json:"timestamp"`
	Severity   string            `json:"severity"`
	Body       string            `json:"body"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Span represents a single span in a distributed trace.
type Span struct {
	TraceID            string            `json:"trace_id"`
	SpanID             string            `json:"span_id"`
	ParentSpanID       string            `json:"parent_span_id,omitempty"`
	Name               string            `json:"name"`
	ServiceName        string            `json:"service_name"`
	Kind               string            `json:"kind"`
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	Duration           time.Duration     `json:"duration"`
	StatusCode         string            `json:"status_code"` // "UNSET", "OK", "ERROR"
	StatusMessage      string            `json:"status_message,omitempty"`
	Attributes         map[string]string `json:"attributes,omitempty"`
	ResourceAttributes map[string]string `json:"resource_attributes,omitempty"`
	Events             []SpanEvent       `json:"events,omitempty"`
	Logs               []LogRecord       `json:"logs,omitempty"`

	// Derived hierarchical and display fields
	Depth          int     `json:"depth"`
	IsCriticalPath bool    `json:"is_critical_path"`
	HasError       bool    `json:"has_error"`
	IsNPlusOne     bool    `json:"is_n_plus_one,omitempty"`
	RelativeOffset float64 `json:"relative_offset"` // 0.0 to 1.0 within trace
	RelativeWidth  float64 `json:"relative_width"`  // 0.0 to 1.0 within trace
}

// Trace represents an assembled distributed trace.
type Trace struct {
	TraceID           string        `json:"trace_id"`
	RootServiceName   string        `json:"root_service_name"`
	RootOperationName string        `json:"root_operation_name"`
	StartTime         time.Time     `json:"start_time"`
	EndTime           time.Time     `json:"end_time"`
	Duration          time.Duration `json:"duration"`
	SpanCount         int           `json:"span_count"`
	HasError          bool          `json:"has_error"`
	HasNPlusOne       bool          `json:"has_n_plus_one"`
	NPlusOneSummary   string        `json:"n_plus_one_summary,omitempty"`
	Spans             []*Span       `json:"spans"`
	SpanTree          []*Span       `json:"span_tree"`
	Logs              []LogRecord   `json:"logs,omitempty"`

	SpansByID map[string]*Span `json:"-"`
}

// ConvertSpanKind translates protobuf SpanKind to a string representation.
func ConvertSpanKind(kind tracepb.Span_SpanKind) string {
	switch kind {
	case tracepb.Span_SPAN_KIND_INTERNAL:
		return "INTERNAL"
	case tracepb.Span_SPAN_KIND_SERVER:
		return "SERVER"
	case tracepb.Span_SPAN_KIND_CLIENT:
		return "CLIENT"
	case tracepb.Span_SPAN_KIND_PRODUCER:
		return "PRODUCER"
	case tracepb.Span_SPAN_KIND_CONSUMER:
		return "CONSUMER"
	default:
		return "UNSPECIFIED"
	}
}

// ConvertStatusCode translates protobuf Status_StatusCode to a string representation.
func ConvertStatusCode(code tracepb.Status_StatusCode) string {
	switch code {
	case tracepb.Status_STATUS_CODE_OK:
		return "OK"
	case tracepb.Status_STATUS_CODE_ERROR:
		return "ERROR"
	default:
		return "UNSET"
	}
}

// AnyValueToString converts an OTLP AnyValue to its string representation.
func AnyValueToString(val *commonpb.AnyValue) string {
	if val == nil {
		return ""
	}
	switch v := val.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return v.StringValue
	case *commonpb.AnyValue_BoolValue:
		return fmt.Sprintf("%t", v.BoolValue)
	case *commonpb.AnyValue_IntValue:
		return fmt.Sprintf("%d", v.IntValue)
	case *commonpb.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", v.DoubleValue)
	case *commonpb.AnyValue_ArrayValue:
		if v.ArrayValue == nil {
			return "[]"
		}
		var items []string
		for _, elem := range v.ArrayValue.Values {
			items = append(items, AnyValueToString(elem))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case *commonpb.AnyValue_KvlistValue:
		if v.KvlistValue == nil {
			return "{}"
		}
		var parts []string
		for _, kv := range v.KvlistValue.Values {
			parts = append(parts, fmt.Sprintf("%s: %s", kv.Key, AnyValueToString(kv.Value)))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *commonpb.AnyValue_BytesValue:
		return hex.EncodeToString(v.BytesValue)
	default:
		return ""
	}
}

// KeyValuesToMap converts OTLP KeyValue slices into a map.
func KeyValuesToMap(kvs []*commonpb.KeyValue) map[string]string {
	if len(kvs) == 0 {
		return nil
	}
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv != nil {
			m[kv.Key] = AnyValueToString(kv.Value)
		}
	}
	return m
}

// SpansFromResourceSpans extracts our internal Span slice from OTLP ResourceSpans.
func SpansFromResourceSpans(resourceSpans []*tracepb.ResourceSpans) []*Span {
	var spans []*Span

	for _, rs := range resourceSpans {
		if rs == nil {
			continue
		}
		resAttrs := make(map[string]string)
		serviceName := "unknown-service"
		if rs.Resource != nil {
			resAttrs = KeyValuesToMap(rs.Resource.Attributes)
			if name, ok := resAttrs["service.name"]; ok && name != "" {
				serviceName = name
			}
		}

		for _, scopeSpans := range rs.ScopeSpans {
			if scopeSpans == nil {
				continue
			}
			for _, s := range scopeSpans.Spans {
				if s == nil {
					continue
				}

				traceID := hex.EncodeToString(s.TraceId)
				spanID := hex.EncodeToString(s.SpanId)
				var parentSpanID string
				if len(s.ParentSpanId) > 0 {
					parentSpanID = hex.EncodeToString(s.ParentSpanId)
				}

				start := time.Unix(0, int64(s.StartTimeUnixNano))
				end := time.Unix(0, int64(s.EndTimeUnixNano))
				if end.Before(start) {
					end = start
				}
				duration := end.Sub(start)

				statusCode := "UNSET"
				statusMsg := ""
				if s.Status != nil {
					statusCode = ConvertStatusCode(s.Status.Code)
					statusMsg = s.Status.Message
				}

				attrs := KeyValuesToMap(s.Attributes)

				var events []SpanEvent
				for _, ev := range s.Events {
					if ev == nil {
						continue
					}
					events = append(events, SpanEvent{
						Name:       ev.Name,
						Timestamp:  time.Unix(0, int64(ev.TimeUnixNano)),
						Attributes: KeyValuesToMap(ev.Attributes),
					})
				}

				hasError := statusCode == "ERROR"
				for _, ev := range events {
					if strings.EqualFold(ev.Name, "exception") || strings.Contains(strings.ToLower(ev.Name), "error") {
						hasError = true
						break
					}
				}

				span := &Span{
					TraceID:            traceID,
					SpanID:             spanID,
					ParentSpanID:       parentSpanID,
					Name:               s.Name,
					ServiceName:        serviceName,
					Kind:               ConvertSpanKind(s.Kind),
					StartTime:          start,
					EndTime:            end,
					Duration:           duration,
					StatusCode:         statusCode,
					StatusMessage:      statusMsg,
					Attributes:         attrs,
					ResourceAttributes: resAttrs,
					Events:             events,
					HasError:           hasError,
				}
				spans = append(spans, span)
			}
		}
	}

	return spans
}

// LogsFromResourceLogs extracts internal LogRecord slice from OTLP ResourceLogs.
func LogsFromResourceLogs(resourceLogs []*logspb.ResourceLogs) []LogRecord {
	var logs []LogRecord

	for _, rl := range resourceLogs {
		if rl == nil {
			continue
		}
		resAttrs := make(map[string]string)
		if rl.Resource != nil {
			resAttrs = KeyValuesToMap(rl.Resource.Attributes)
		}

		for _, scopeLogs := range rl.ScopeLogs {
			if scopeLogs == nil {
				continue
			}
			for _, lr := range scopeLogs.LogRecords {
				if lr == nil {
					continue
				}

				traceID := hex.EncodeToString(lr.TraceId)
				spanID := hex.EncodeToString(lr.SpanId)
				timestamp := time.Unix(0, int64(lr.TimeUnixNano))

				attrs := KeyValuesToMap(lr.Attributes)
				// Merge resource attributes into log attributes if not already present
				for k, v := range resAttrs {
					if _, exists := attrs[k]; !exists {
						if attrs == nil {
							attrs = make(map[string]string)
						}
						attrs[k] = v
					}
				}

				body := ""
				if lr.Body != nil {
					body = AnyValueToString(lr.Body)
				}

				severity := lr.SeverityText
				if severity == "" {
					severity = lr.SeverityNumber.String()
				}

				logs = append(logs, LogRecord{
					TraceID:    traceID,
					SpanID:     spanID,
					Timestamp:  timestamp,
					Severity:   severity,
					Body:       body,
					Attributes: attrs,
				})
			}
		}
	}

	// Sort logs chronologically
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.Before(logs[j].Timestamp)
	})

	return logs
}
