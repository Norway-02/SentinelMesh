# SentinelMesh Control Plane GUI — API Contract

## Overview

The SentinelMesh Control Plane GUI is a thin client operating against the authoritative SentinelMesh backend API running at `http://127.0.0.1:8787` (configurable via `SENTINEL_HTTP_ADDR`).

All responses use Standard JSON format (`Content-Type: application/json`). No secret keys, credentials, or private tokens are ever exposed over the API.

---

## REST Endpoints

### 1. Health & Readiness

#### `GET /healthz`
- **Description**: Liveness probe. Returns HTTP 200 `OK`.

#### `GET /readyz`
- **Description**: Readiness probe. Checks database connectivity if configured.
- **Response**: `200 OK` or `503 Service Unavailable`.

---

### 2. Model Catalog

#### `GET /v1/models`
- **Description**: Returns safe public catalog metadata for all registered models.
- **Response**:
```json
{
  "models": [
    {
      "id": "small-fast-v1",
      "name": "Small Fast Model",
      "tier": "small",
      "provider": "synthetic-local",
      "context_window": 8192,
      "security_classes": ["public", "standard", "restricted", "airgapped"],
      "cost_per_1k_input_tokens": 0.00015,
      "cost_per_1k_output_tokens": 0.00060,
      "nominal_p50_latency_ms": 45.0,
      "nominal_p95_latency_ms": 80.0,
      "health_status": "HEALTHY",
      "observed_metrics": {
        "observed_p50_latency_ms": 45.0,
        "observed_error_rate": 0.0,
        "consecutive_failures": 0,
        "total_invocations": 120
      }
    }
  ],
  "count": 1,
  "registry_version": "registry-v1.0"
}
```

---

### 3. Stage 17 Model Router

#### `POST /v1/router/route`
- **Description**: Evaluates deterministic multi-constraint model selection without network execution.
- **Request**:
```json
{
  "prompt": "Analyze repository structure",
  "task_complexity": "moderate",
  "routing_policy": "balanced",
  "quality_requirement": 0.75,
  "latency_sla_ms": 2000.0,
  "cost_budget_usd": 0.10,
  "estimated_input_tokens": 500,
  "estimated_output_tokens": 200,
  "security_profile": "standard"
}
```
- **Response**:
```json
{
  "task_id": "task-abc-123",
  "selected_model_id": "medium-balanced-v1",
  "selected_tier": "medium",
  "policy": "balanced",
  "algorithm_version": "router-v1.0",
  "estimated_cost_usd": 0.00195,
  "estimated_latency_ms": 119.0,
  "quality_score": 0.91,
  "final_score": 0.892,
  "fallback_candidates": ["large-reasoning-v1"],
  "rejections": [
    {
      "model_id": "small-fast-v1",
      "reason": "quality_below_threshold",
      "details": "Model quality on moderate (0.74) is below required threshold (0.75)"
    }
  ],
  "is_pareto_optimal": true
}
```

#### `POST /v1/router/execute`
- **Description**: Evaluates Stage 17 routing and dispatches inference with automatic circuit breaker fallback.

---

### 4. Stage 18 Adaptive Intelligence

#### `POST /v1/adaptive/route`
- **Description**: Evaluates empirical Bayes quality posterior, posterior quantiles, and least-squares online latency regression.
- **Response**:
```json
{
  "task_id": "task-abc-123",
  "selected_model_id": "medium-balanced-v1",
  "effective_utility": 0.892,
  "confidence": 0.81,
  "sample_count": 24,
  "predicted_success": 0.94,
  "quality_estimate": {
    "mean": 0.87,
    "variance": 0.002,
    "samples": 24,
    "lower_ci": 0.82,
    "upper_ci": 0.92
  },
  "latency_estimate": {
    "predicted_ms": 170.0,
    "observed_p50_ms": 165.0,
    "observed_p95_ms": 220.0
  }
}
```

---

### 5. Stage 19 Safe Online Policy

#### `POST /v1/policy/route`
- **Description**: Contextual UCB arm selection respecting the Stage 17 safe feasible set.

#### `POST /v1/policy/execute`
- **Description**: **Flagship endpoint**. Runs complete pipeline: Stage 17 Safety → Stage 18 Prediction → Stage 19 Policy → Model Invocation → Telemetry.
- **Response**: Bundles Stage 17 decision, Stage 18 decision, Stage 19 decision, and invocation response.

#### `GET /v1/policy/state`
- **Description**: Active policy version, exploration budget, UCB lambda, and guardrail rollback status.

---

### 6. Metrics & Telemetry

#### `GET /v1/metrics/summary`
- **Description**: Aggregated real-time metrics for executive dashboard.
- **Response**:
```json
{
  "active_tasks": 0,
  "total_decisions": 124,
  "success_rate": 0.984,
  "mean_latency_ms": 162.0,
  "p95_latency_ms": 220.0,
  "total_cost_usd": 4.21,
  "fallback_rate": 0.024,
  "drift_alerts": 0,
  "policy_rollbacks": 0,
  "active_providers": 3,
  "requests_per_minute": 12.0
}
```

---

### 7. Real-Time Events (Server-Sent Events)

#### `GET /v1/events/stream`
- **Description**: Long-lived SSE stream emitting domain events in real-time.
- **Event format**:
```text
event: ROUTING_DECIDED
data: {"id":"evt-123","type":"ROUTING_DECIDED","timestamp":"2026-08-24T...","stage":"stage17","task_id":"task-123","payload":{}}
```

#### `GET /v1/events`
- **Description**: Paginated query endpoint for historical events (`limit`, `stage`, `type`, `task_id`).

---

### 8. System Settings

#### `GET /v1/settings`
- **Description**: Environment configuration and active engine tags. Redacts all sensitive credentials.
