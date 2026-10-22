---
title: Stacked Flamegraph View
description: Proportional multi-layer flamegraph of distributed trace executions
---

# Stacked Flamegraph View

Press **`f`** inside the trace detail view to switch between the hierarchical tree waterfall and the **Stacked Flamegraph**.

```
FLAMEGRAPH VIEW [f: switch to Waterfall] — Total Duration: 100.0ms

 L0  │                  api-gateway: POST /api/v1/checkout (100.0ms)                  
 L1  │     POST /auth/verify             payment-service: Charge (60.0ms)             
 L2  │                                     SELECT * FROM accounts                     
```

---

## How It Works

1. **Depth Levels (`L0`, `L1`, `L2`...)**:
   - `L0`: Root span layer.
   - `L1`: Direct child spans called by the root.
   - `L2`: Grandchild operations (e.g. database queries, downstream RPC calls, cache lookups).

2. **Proportional Widths**:
   - Spans are rendered as continuous colored blocks across the terminal width proportional to their start offset and execution time.
   - Child operations visually sit directly underneath the time window of their parent span.

3. **Dynamic Label Scaling**:
   - Depending on the character width available for each block, the label automatically scales:
     - Full: `service: operation (duration)`
     - Medium: `operation (duration)`
     - Compact: `operation`
     - Minimal: `op…` or solid colored slice for sub-millisecond spans.

4. **Visual Color Coding**:
   - **Service Color**: Each service is rendered with its distinct service palette background.
   - **Errors (Red)**: Any failed span is rendered with a bold red background and white text.
   - **Critical Path (Amber)**: Spans along the latency bottleneck path stand out with an amber warning badge.
   - **Active Selection (Yellow)**: The currently focused span is rendered in bright yellow with high-contrast text.

---

## Toggling Back

Press **`f`** again at any time to return to the [Trace Waterfall](waterfall.md).
