---
title: Service Dependency Topology Graph
description: Real-time architecture call map with latencies and error rates
---

# Service Dependency Topology Graph

Press **`S`** (Shift+S) in the trace list view to view the live **Service Dependency Topology Graph**.

```
◆ SERVICE TOPOLOGY GRAPH (4 services, 3 dependency links)

SERVICES SUMMARY:
  SERVICE NAME             INBOUND CALLS   OUTBOUND CALLS  ERROR COUNT   ERROR RATE
  api-gateway              0               12              2             16.7%
  auth-service             5               0               1             20.0%
  payment-service          7               14              1             4.8%
  postgres-db              14              0               0             0.0%

DEPENDENCY CALL FLOWS:
  api-gateway            ──[ 12 calls │ avg 42.1ms │ p95 110.0ms ]──▶ payment-service
  payment-service        ──[ 14 calls │ avg 12.3ms │ p95 35.0ms ]───▶ postgres-db
  api-gateway            ──[ 5 calls │ avg 18.2ms │ p95 24.0ms ]────▶ auth-service
```

---

## What It Shows

1. **Services Summary Table**:
   - Aggregates call counts across all traces currently stored in the in-memory ring buffer.
   - Computes **Inbound Calls**, **Outbound Calls**, **Total Error Count**, and **Error Rate Percentage**.

2. **Directed Dependency Call Flows**:
   - Inferred automatically from `CLIENT` ➔ `SERVER` spans or cross-service parent-child relationships.
   - For each service-to-service connection, displays:
     - Total invocation count.
     - Average latency (`avg`).
     - 95th-percentile latency (`p95`).
     - Error count badge (highlighted in red if $> 0$).

---

## When to Use It

- **Architectural Discovery**: Confirm that new microservices are communicating along expected paths.
- **Root-Cause Analysis**: Quickly identify which downstream dependency is driving up p95 latency or throwing errors.
- **Impact Assessment**: See which services act as central hubs or single points of failure.

Press **`Esc`**, **`Backspace`**, or **`q`** to return to the Trace List.
