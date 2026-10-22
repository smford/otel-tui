---
title: N+1 Database Query Detection
description: Automated detection of repetitive SQL queries and ORM loops
---

# N+1 Database Query Detection

The **N+1 query problem** is the single most common cause of latency regressions in web applications and microservices using ORMs (such as Hibernate, GORM, ActiveRecord, or Prisma).

Instead of executing a single batch query (e.g., `SELECT * FROM items WHERE id IN (...)`), an application accidentally queries the database $N$ times inside a loop for each parent record.

---

## How otel-tui Detects N+1 Queries

1. **Normalization & Pattern Matching**:
   - `otel-tui` inspects all database spans (`db.statement` attribute or client spans with SQL syntax).
   - Query strings are normalized by abstracting dynamic literals and IDs:
     - `SELECT * FROM users WHERE id = 101` ➔ `SELECT * FROM users WHERE id = ?`
     - `SELECT * FROM users WHERE id = 102` ➔ `SELECT * FROM users WHERE id = ?`
2. **Threshold Triggering**:
   - If 3 or more structurally identical queries execute within the same trace, `otel-tui` automatically flags the trace.

---

## Where It Appears

### 1. In the Trace List View
Traces with detected query loops are marked with an amber **`[N+1]`** badge in the FLAGS column:
```
  OK     15:04:05  api-gateway   POST /api/v1/checkout   190.0ms   8      5e1009355b6eb866  [N+1]
```

### 2. In the Trace Waterfall
Offending child spans are marked with a bright **`[N+1]`** badge:
```
    ├─● postgres-db: SELECT * FROM pay... [N+1]      [░░░░░░░░░░████████░░░░░░░]  40.0ms (21%)
    ├─● postgres-db: SELECT * FROM pay... [N+1]      [░░░░░░░░░░░░░░████████░░░]  34.0ms (18%)
    ├─● postgres-db: SELECT * FROM pay... [N+1]      [░░░░░░░░░░░░░░░░░░███████]  34.0ms (18%)
```

### 3. In the Span Inspector Drawer
When inspecting any span involved in the repetition, a warning banner explains the issue:
```
▶ SPAN OVERVIEW
  ...
  ⚠ Suspected N+1 Query: Repetitive DB statement detected in trace
```

---

## Testing N+1 Detection

You can see this live immediately by running:
```bash
./bin/otel-tui demo --scenario checkout
```
The checkout scenario executes 3 consecutive `SELECT * FROM payment_methods WHERE customer_id = $X` queries to demonstrate the detector in action.
