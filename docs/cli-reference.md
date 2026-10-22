# CLI Reference

`otel-tui` provides a rich set of command-line flags and subcommands for both interactive terminal sessions and automated headless streaming.

---

## Global Synopsis

```bash
otel-tui [COMMAND] [OPTIONS]
```

---

## Interactive TUI Flags

When launched without a subcommand, `otel-tui` starts the live interactive terminal UI.

| Flag | Type | Default | Description |
|:---|:---:|:---:|:---|
| `--demo` | Flag | `false` | Run with background synthetic trace generator producing live traffic. |
| `--file <PATH>` | Path | `none` | Load and visualize an offline OTLP JSON / NDJSON file. |
| `--grpc-port <PORT>` | Integer | `4317` | Port for the OTLP gRPC receiver. Automatically falls back if occupied. |
| `--http-port <PORT>` | Integer | `4318` | Port for the OTLP HTTP receiver (Protobuf & JSON). Falls back if occupied. |
| `--forward-to <URL>` | String | `none` | Forward received traces upstream to another OTLP collector or APM backend. |
| `--max-traces <NUM>` | Integer | `1000` | Maximum number of traces retained in the circular memory buffer. |
| `--export <PATH>` | Path | `none` | Automatically export buffered traces to JSON file when exiting `otel-tui`. |
| `--no-color` | Flag | `false` | Disable ANSI color codes and color depth styling. |
| `-v`, `--version` | Flag | `none` | Print version information and exit. |
| `-h`, `--help` | Flag | `none` | Display command-line help and exit. |

### Examples

```bash
# Start standard receiver on default ports
otel-tui

# Launch with instant synthetic traffic
otel-tui --demo

# Inspect an offline export from CI or production
otel-tui --file ./traces/incident-2026-10-22.json

# Relay traces to local Jaeger while inspecting locally
otel-tui --forward-to http://localhost:14318

# Bind custom non-standard ports
otel-tui --grpc-port 9090 --http-port 9091
```

---

## Subcommand: `demo`

The `demo` subcommand provides headless synthetic trace generation without opening a terminal UI.

```bash
otel-tui demo [OPTIONS]
```

### Options

| Flag | Type | Default | Description |
|:---|:---:|:---:|:---|
| `--stream` | Flag | `false` | Keep running and stream synthetic traces continuously. |
| `--endpoint <URL>` | String | `http://localhost:4318` | Target OTLP endpoint to emit traces to. |
| `--protocol <PROTO>` | Enum | `http` | Transport protocol: `http` (Protobuf / JSON) or `grpc`. |
| `--rate <NUM>` | Float | `2.0` | Number of traces generated per second during `--stream`. |
| `--count <NUM>` | Integer | `1` | Total number of traces to emit before exiting. |
| `--error-rate <PCT>` | Float | `0.1` | Percentage (0.0 to 1.0) of traces configured with error spans. |

### Examples

```bash
# Emit 10 synthetic traces to localhost:4318 and exit
otel-tui demo --count 10

# Continuously pump 5 traces/sec to a remote collector over gRPC
otel-tui demo --stream --endpoint collector.myorg.internal:4317 --protocol grpc --rate 5
```

---

## Environment Variables

Every command-line flag can also be configured via environment variables:

| Environment Variable | Equivalent Flag | Description |
|:---|:---|:---|
| `OTEL_TUI_HTTP_PORT` | `--http-port` | Port for the HTTP receiver |
| `OTEL_TUI_GRPC_PORT` | `--grpc-port` | Port for the gRPC receiver |
| `OTEL_TUI_FORWARD_TO` | `--forward-to` | Forwarding target endpoint |
| `OTEL_TUI_MAX_TRACES` | `--max-traces` | In-memory buffer size |
| `NO_COLOR` | `--no-color` | Standard Unix no-color enforcement |
