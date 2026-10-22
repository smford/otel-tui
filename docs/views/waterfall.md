---
title: Trace Waterfall View
description: Hierarchical parent-child span tree with timing bars
---

# Trace Waterfall View

When you press `Enter` on any trace in the main list view, `otel-tui` opens the **Distributed Trace Waterfall**.

```
SPAN HIERARCHY                                      TIMELINE (100.0ms total)
❯ ◆ ● api-gateway: POST /api/v1/checkout [CP]        [█████████████████████████] 100.0ms (100%)
    ├─● auth-service: POST /auth/verify              [░██████░░░░░░░░░░░░░░░░░░]  25.0ms (25%)
    ├─✖ payment-service: Charge [CP]                 [░░░░░░░░███████████████░░]  60.0ms (60%)
      ├─● postgres-db: SELECT * FROM acc... [CP]     [░░░░░░░░░░██████████░░░░░]  40.0ms (40%)
```

---

## Anatomy of a Waterfall Row

Each row in the waterfall is divided into two sections:

### 1. Left Hierarchy Tree
- **Cursor (`❯`)**: Highlights the currently selected span.
- **Tree Branch (`◆`, `├─`, `│`)**: Visual indentation showing parent-child call depth.
- **Status Indicator**:
  - `●` (Green): Span completed successfully (`OK` or unset).
  - `✖` (Red): Span ended with an error status (`status.code == ERROR`) or recorded an exception event.
- **Service Name**: Distinct deterministic color palette per service name.
- **Operation Name**: HTTP route, RPC method, SQL statement, or function name.
- **Badges**:
  - `[CP]`: Marked when the span lies on the **Critical Path** (the bottleneck sequence determining trace latency).
  - `[N+1]`: Marked when the span is part of a detected repetitive database query loop.

### 2. Right Timing Bar
- Fixed-width ASCII/Unicode bar (`[████░░░░]`) showing:
  - **Start Offset**: Leading background spaces/dots showing when the span started relative to the root span.
  - **Span Duration**: Filled solid blocks showing span execution time.
  - **Formatted Duration**: Precise latency (`µs`, `ms`, or `s`).
  - **Relative Percentage**: Proportion of total trace time occupied by this span.

> [!NOTE]
> All timing bars are aligned horizontally down to the exact terminal column across all nesting levels, ensuring an accurate visual sense of parallel vs. sequential execution.

---

## Key Actions in Waterfall View

- **`↑` / `k`, `↓` / `j`**: Navigate between spans. The bottom Span Inspector drawer updates in real time.
- **`f`**: Switch to the [Stacked Flamegraph View](flamegraph.md).
- **`Tab`**: Move focus to the bottom [Span Inspector Drawer](inspector.md) to scroll through attributes and logs.
- **`y`**: Copy the active `SpanID` to your system clipboard.
- **`Esc` / `Backspace`**: Return to the main Trace List view.
