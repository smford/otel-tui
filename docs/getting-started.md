---
title: Quick Start Guide
description: Installing, building, and running otel-tui in seconds
---

# Quick Start Guide

Get `otel-tui` up and running in less than a minute.

---

## 1. Installation & Building

### Requirements
- **Go 1.23+** (only required if building from source)
- Any modern terminal (macOS Terminal, iTerm2, Kitty, Alacritty, WezTerm, Windows Terminal)

### Clone & Build

```bash
git clone https://github.com/smford/otel-tui.git
cd otel-tui
make build
```

This compiles a zero-CGO static executable into `bin/otel-tui`.

---

## 2. Running otel-tui

You have three easy ways to run `otel-tui`:

### Option A: Zero Emitter Needed (Live Demo Mode)

If you don't have an instrumented app running yet, start `otel-tui` with the `--demo` flag. It automatically feeds continuous realistic multi-service traces in the background:

```bash
./bin/otel-tui --demo
```

*(You can also press `D` inside the interactive TUI at any time to inject a trace on demand!)*

### Option B: Standalone Continuous Emitter

Start the collector in one terminal pane and stream realistic multi-scenario traffic from another:

```bash
# Terminal Pane 1: Collector & TUI
./bin/otel-tui

# Terminal Pane 2: Stream realistic synthetic traces continuously
./bin/otel-tui demo --stream --interval 1s
```

### Option C: Connect Your Application

Start `otel-tui` and point your application's OpenTelemetry SDK to `localhost:4318`:

```bash
./bin/otel-tui
```

```bash
# In your application's environment:
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
```

---

## 3. Keyboard Cheat Sheet

| Key | Action |
|---|---|
| `↑` / `k`, `↓` / `j` | Move selection up / down |
| `Enter` | Drill down into Trace Waterfall / Flamegraph |
| `Esc` / `Backspace` | Return to Trace List view |
| `D` | **Inject synthetic trace on-demand** |
| `f` | Toggle between Waterfall and stacked Flamegraph |
| `d` | **Diff selected trace** against marked baseline or previous trace |
| `m` | **Mark / unmark baseline trace** (`[BASE]`) |
| `S` | Open **Service Dependency Architecture Topology** map |
| `Tab` | Switch focus between Waterfall and Span Inspector drawer |
| `J` / `K` | Scroll Span Inspector drawer |
| `y` | Copy selected `TraceID` or `SpanID` to clipboard |
| `Y` | Copy full trace JSON to clipboard |
| `/` | Live filter traces (`service:api`, `error:true`, `>200ms`) |
| `p` | Pause / resume live ingestion stream |
| `c` | Clear in-memory buffer |
| `x` | Export trace to `trace_<id>.json` |
| `?` | Toggle in-app help modal |
| `q` / `Ctrl+C` | Quit |

---

## Next Steps

* [Connect your Python, Go, Node.js, or Java application](ingestion/connecting-apps.md)
* [Explore the Trace Waterfall view](views/waterfall.md)
* [Learn about Trace Comparison & Diffing](views/diff.md)
