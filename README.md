# otel-tui — Zero-Setup Terminal OpenTelemetry Viewer

`otel-tui` is a single-binary, zero-CGO local OpenTelemetry collector and interactive terminal UI. It receives distributed traces and logs over standard OTLP HTTP (`:4318`) and gRPC (`:4317`) protocols directly in memory, rendering live distributed trace waterfalls, critical-path flamegraphs, service dependency graphs, trace diffing, and error diagnostics right in your terminal pane.

No Docker containers, no Jaeger, Tempo, or Grafana setup required. Just run `otel-tui`, point your application's OpenTelemetry SDK to `localhost:4318`, and debug your traces immediately.

---

## Features

- **Zero-Setup & Zero-CGO**: Single portable static binary compiled with `CGO_ENABLED=0`.
- **Automatic Port Fallback**: Automatically binds to alternate ports (e.g. `:14318`) if default ports are already occupied by another service or Docker container.
- **OTLP Relay / Proxy Forwarding (`--forward-to <url>`)**: Asynchronously tee and forward incoming traces/logs to an upstream collector (e.g. Datadog, Jaeger, Tempo, Honeycomb) while debugging locally.
- **Dual OTLP Ingestion**:
  - **OTLP/HTTP** (`:4318`): `POST /v1/traces` and `POST /v1/logs` supporting both `application/x-protobuf` and `application/json` payloads, with transparent gzip decompression and CORS support.
  - **OTLP/gRPC** (`:4317`): Native high-performance gRPC export endpoint.
- **Interactive Terminal UI** (built with Bubbletea & Lipgloss):
  - **Live Trace List View**: Real-time overview of incoming traces with service names, root operations, durations, span counts, and status badges (`OK` / `ERR`).
  - **Trace Comparison & Diffing (`m` to mark, `d` to diff)**: Compare two traces side-by-side to instantly pinpoint latency regressions, missing/added spans, or status changes.
  - **N+1 Database Query Detection (`[N+1]`)**: Automatically flags traces executing repetitive parameterized database queries and highlights the offending spans.
  - **Service Dependency Architecture Map (`S` key)**: Inferred live directed topology graph showing inter-service call volumes, avg/p95 latencies, and error counts.
  - **Distributed Trace Waterfall & Stacked Flamegraph (`f` key)**: Toggle between hierarchical indented span tree and proportional stacked flamegraph views (`████░░░░`).
  - **Critical-Path Highlighting**: Automatically identifies and highlights the bottleneck execution path (`[CP]`) and error spans in red.
  - **Span Inspector Drawer**: Full inspection of span attributes, resource attributes, exception events with stack traces, and correlated OTLP logs.
  - **Clipboard Copy (`y` / `Y`)**: Copy selected `TraceID`, `SpanID`, or full trace JSON to clipboard via OSC 52 / native clipboard tools.
  - **Live Search & Filtering**: Instant querying by service name, error status (`error:true`), duration (`>200ms`, `<50ms`), or free-text keywords.
  - **Ring Buffer Memory Bounds**: Thread-safe bounded buffer (default 1,000 traces) guarantees flat memory usage under load.
  - **Trace Export & Offline Replay**: Save traces to JSON (`x`) and replay them offline (`--replay <file.json>`).
- **Headless CLI Dump Mode**: Stream colorized trace summaries or structured JSON lines (`--headless --json`) to stdout for piping or CI integration.
- **Built-in Demo Generator**: Simulate realistic multi-service traces (`api-gateway` -> `auth-service` -> `payment-service` -> `postgres-db`, `stripe-client`) with slow queries, N+1 query loops, and exceptions via `otel-tui demo`.

---

## Installation & Quick Start

### Build from Source

Requirements: Go 1.23+

```bash
git clone https://github.com/smford/otel-tui.git
cd otel-tui
make build
```

This generates `bin/otel-tui`.

### Quick Start

1. Start `otel-tui` in a terminal pane:
   ```bash
   ./bin/otel-tui
   ```

2. In another pane, generate a synthetic multi-service demo trace:
   ```bash
   ./bin/otel-tui demo
   ```

---

## Pointing Your Application to `otel-tui`

Configure your application using standard OpenTelemetry environment variables:

### Global Environment Variables

```bash
# HTTP Exporter (default :4318)
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"

# Or gRPC Exporter (default :4317)
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4317"
export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
```

### Language SDK Configuration One-Liners

#### Python
```bash
OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318" opentelemetry-instrument python app.py
```

#### Node.js
```bash
NODE_OPTIONS="--require @opentelemetry/auto-instrumentations-node/register" \
OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318" \
node app.js
```

#### Go
```go
import (
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

exporter, err := otlptracehttp.New(ctx,
    otlptracehttp.WithEndpoint("localhost:4318"),
    otlptracehttp.WithInsecure(),
)
```

#### Java
```bash
java -javaagent:opentelemetry-javaagent.jar \
     -Dotel.exporter.otlp.endpoint=http://localhost:4318 \
     -jar app.jar
```

---

## CLI Usage

```
otel-tui — Zero-Setup Terminal OpenTelemetry Viewer

USAGE:
  otel-tui [flags]
  otel-tui demo [flags]

COMMANDS:
  demo                      Send synthetic multi-service sample traces to test the UI

FLAGS:
  --grpc-port <int>         gRPC ingestion port (default: 4317)
  --http-port <int>         HTTP ingestion port (default: 4318)
  --max-traces <int>        Max traces held in memory ring buffer (default: 1000)
  --headless                Disable interactive TUI and stream trace logs to stdout
  --json                    In headless mode, output structured JSON lines instead of colorized text
  --replay <path.json>      Load an exported JSON trace file for offline inspection
  --forward-to <url>        Proxy/relay incoming traces to an upstream collector (e.g. http://localhost:4319)
  --auto-fallback           Try fallback ports if default ports are busy (default: true)
  -h, --help                Show help information
```

### Headless CLI Mode

Ideal for automated test suites or CI runners:

```bash
# Colorized human-readable streaming summary
./bin/otel-tui --headless

# Structured JSON lines (for piping into jq or file storage)
./bin/otel-tui --headless --json | jq .
```

### Offline Replay

Export any trace from the TUI by pressing `x` (saves as `trace_<id>.json`), then reload it anytime:

```bash
./bin/otel-tui --replay trace_2e95d9b6.json
```

---

## TUI Keyboard Navigation

| Key | Description |
|---|---|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `PageUp` / `PageDown` | Jump 10 rows |
| `Home` / `End` | Jump to top / bottom |
| `Enter` | Open trace waterfall / flamegraph view |
| `Esc` / `Backspace` | Return to trace list view |
| `d` | Compare / Diff trace with marked baseline or adjacent trace |
| `m` | Mark / unmark trace as comparison baseline (`[BASE]`) |
| `S` | Open live Service Dependency Architecture Topology map |
| `f` | Toggle between Waterfall and stacked Flamegraph view (in detail mode) |
| `Tab` | Toggle focus between Waterfall and Span Inspector drawer |
| `J` / `K` | Scroll Span Inspector drawer |
| `y` | Copy selected TraceID / SpanID to clipboard (OSC 52 / pbcopy) |
| `Y` | Copy full trace JSON to clipboard |
| `/` | Filter traces (`service:auth`, `error:true`, `>200ms`, `status:error`) |
| `p` | Pause / resume live trace ingestion stream |
| `c` | Clear memory buffer |
| `x` | Export selected trace to JSON file |
| `?` | Toggle help popup |
| `q` / `Ctrl+C` | Quit |

---

## Query Filtering Syntax

In the trace list view, press `/` to filter:

- **By Error Status**: `error:true`, `status:error`, or `error:false`
- **By Service**: `service:api-gateway` or `service:payment`
- **By Latency / Duration**: `>200ms`, `>=1s`, `<50ms`
- **By Substring**: search trace IDs, operation names, or service names (e.g. `checkout`)
- **Combined**: `service:payment error:true >100ms`

---

## Testing

Run the full test suite with Go's race detector:

```bash
make test
```

Unit tests cover:
- Thread-safe bounded ring buffer and eviction
- Out-of-order span batch assembly into complete traces
- N+1 database query detection
- Trace diffing and latency comparison
- Service dependency graph aggregation
- Proportional timing bar math across arbitrary column widths
- OTLP HTTP (JSON & Protobuf) and gRPC trace & log receivers
- Automatic port conflict fallback
- Upstream proxy relay forwarding
- Multi-service end-to-end synthetic demo ingestion

---

## License

MIT
