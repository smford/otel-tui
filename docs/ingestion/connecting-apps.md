# Connecting Your Application

`otel-tui` operates as a standard OpenTelemetry receiver listening on the official default OTLP ports:
- **gRPC Receiver**: `http://localhost:4317`
- **HTTP (Protobuf / JSON) Receiver**: `http://localhost:4318`

Connecting your application requires zero changes to application logic—just point standard OpenTelemetry environment variables to `otel-tui`.

---

## Standard Environment Variables

Most modern OpenTelemetry language SDKs support standard environment variables:

```bash
# Set your application service name
export OTEL_SERVICE_NAME="my-awesome-service"

# Target otel-tui's HTTP endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"

# Alternatively, target otel-tui's gRPC endpoint
# export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4317"
# export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
```

---

## Language Examples

### Python

Install OpenTelemetry packages:
```bash
pip install opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp-proto-http
```

Run with automatic instrumentation:
```bash
opentelemetry-instrument \
  --service_name my-python-app \
  --traces_exporter otlp \
  --exporter_otlp_endpoint http://localhost:4318 \
  --exporter_otlp_protocol http/protobuf \
  python main.py
```

Or configure manually in code:
```python
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource

resource = Resource.create({"service.name": "order-service"})
provider = TracerProvider(resource=resource)
processor = BatchSpanProcessor(
    OTLPSpanExporter(endpoint="http://localhost:4318/v1/traces")
)
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)

tracer = trace.get_tracer("order-service")
with tracer.start_as_current_span("process_payment"):
    print("Processing payment...")
```

---

### Go

```bash
go get go.opentelemetry.io/otel \
       go.opentelemetry.io/otel/sdk \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
```

```go
package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("localhost:4317"),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("checkout-service"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}
```

---

### Node.js / TypeScript

```bash
npm install --save @opentelemetry/api \
  @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-proto
```

Create `tracing.js`:
```javascript
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-proto');

const sdk = new NodeSDK({
  serviceName: 'backend-api',
  traceExporter: new OTLPTraceExporter({
    url: 'http://localhost:4318/v1/traces',
  }),
  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();
```

Run your app with preloading:
```bash
node --require ./tracing.js app.js
```

---

### Java (Zero-Code Agent)

Download the official OpenTelemetry Java Agent:
```bash
curl -LO https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
```

Launch your application:
```bash
java -javaagent:opentelemetry-javaagent.jar \
     -Dotel.service.name=billing-service \
     -Dotel.exporter.otlp.endpoint=http://localhost:4318 \
     -jar app.jar
```

---

## Testing Connectivity with cURL

You can test that `otel-tui` is actively receiving traces over HTTP JSON:

```bash
curl -i -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "resource": {
        "attributes": [
          {"key": "service.name", "value": {"stringValue": "curl-test-service"}}
        ]
      },
      "scopeSpans": [{
        "spans": [{
          "traceId": "5b8aa5a2d2c872e8321cf97b40b94424",
          "spanId": "5fb397be34d23b0f",
          "name": "manual-ping",
          "kind": 1,
          "startTimeUnixNano": "1672531199000000000",
          "endTimeUnixNano": "1672531200000000000",
          "status": {"code": 1}
        }]
      }]
    }]
  }'
```

As soon as the request completes, the `curl-test-service` trace will appear instantly in the `otel-tui` live stream!
