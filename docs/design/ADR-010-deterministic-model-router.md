# ADR-010: Deterministic Multi-Objective AI Model Router

## Status
Accepted

## Context
In distributed AI agent architectures, workloads exhibit extreme variance in computational reasoning requirements. Simple tasks (e.g., entity extraction, JSON structure reformatting, metadata tagging) do not require state-of-the-art reasoning models (e.g., GPT-4o, Claude 3.5 Sonnet), which introduce unnecessary financial cost ($15\times - 100\times$ higher) and latency penalties ($5\times - 20\times$ higher). Conversely, underpowered small models (e.g., 8B parameters) suffer high failure rates on complex multi-step reasoning.

Statically assigning a single global model leads to suboptimal Pareto outcomes across cost, latency, and quality. However, utilizing a non-deterministic "LLM-as-a-router" introduces circular latency overhead, unpredictable nondeterminism, and control-plane opacity.

## Decision
We implement a **Deterministic, Policy-Based Model Router** in `internal/router/` with the following architectural guarantees:

1. **Separation of Router vs. Provider**:
   - `Router.Route(ctx, req)` answers *which* model to select based on deterministic constraint filtering and utility scoring.
   - `ModelProvider.Invoke(ctx, modelID, req)` handles actual network dispatch and inference execution.

2. **Registry-Driven Capabilities & Task Quality Matrix**:
   - Model definitions in `Registry` maintain empirical capability scores mapped explicitly per complexity class:
     $$\text{Quality}(m, \text{complexity}) \in [0.0, 1.0]$$
   - Explicit task complexity classes: `ComplexitySimple`, `ComplexityModerate`, `ComplexityComplex`, and `ComplexityReasoningHeavy`.

3. **Strict Hard Constraint Ordering & Explainability**:
   Candidates must satisfy hard requirements in the following sequence before scoring:
   1. *Security Profile Compatibility* (e.g., airgapped tenants restricted to on-prem local models)
   2. *Context Window Capacity* ($\text{ContextWindow} \ge \text{InputTokens} + \text{OutputTokens}$)
   3. *Availability & Circuit Breaker Health* ($\text{HealthStatus} \neq \text{UNAVAILABLE}$)
   4. *Quality Threshold* ($\text{Quality} \ge Q_{\min}$)
   5. *Cost Budget & Latency SLA Constraints*
   Every eliminated model produces an explicit `ModelRejection` audit record with reason and details.

4. **Pareto Frontier Optimization & Distinct Policy Objectives**:
   - Feasible candidates are analyzed for Pareto dominance across $(\text{Quality}, \text{Cost}, \text{Latency})$.
   - Safe min-max normalization guarantees non-zero divisors.
   - Distinct optimization weights across policies:
     - `PolicyCostOptimized`: $w_c = 0.75, w_q = 0.15, w_l = 0.05, w_r = 0.05$
     - `PolicyLatencyOptimized`: $w_l = 0.75, w_q = 0.15, w_c = 0.05, w_r = 0.05$
     - `PolicyQualityOptimized`: $w_q = 0.90, w_c = 0.00, w_l = 0.05, w_r = 0.05$
     - `PolicyBalanced`: $w_q = 0.50, w_c = 0.25, w_l = 0.20, w_r = 0.05$
   - Deterministic tie-breaking: $\text{Score} \downarrow \to \text{Quality} \downarrow \to \text{Latency} \uparrow \to \text{ModelID} \uparrow$.

5. **Circuit Breaker Error Classification & Cascading Fallback**:
   - Infrastructure errors (`timeout`, `429`, `5xx`, `conn_refused`) increment the breaker and trip after 3 consecutive failures.
   - Client/validation errors (`400 Bad Request`, policy denial) do not trip the breaker.
   - Atomic half-open probe lock ensures only a single test probe is in flight during recovery.
   - Multi-step failover automatically cascades across pre-computed fallback chains.

6. **Deterministic Replay & Snapshot Versioning**:
   - `Replay(req, models)` guarantees bit-for-bit reproducibility of historical decisions.
   - Every decision records `AlgorithmVersion` (`router-v1.0`), `RegistryVersion` (`registry-v1.0`), and `PolicyVersion` (`policy-v1.0`).

7. **Decision & Outcome Persistence (Stage 18 Foundation)**:
   - Complete decision records and actual execution outcome telemetry are persisted in `DecisionRepository` to serve as the ground truth training set for Stage 18 adaptive models.

## What Stage 17 Guarantees vs. Does Not Guarantee

### Guarantees
- **Deterministic Selection**: Identical inputs and catalog snapshots yield bit-for-bit identical routing decisions.
- **Strict Constraint Enforcement**: Security, context window, availability, budget, and SLA constraints are inviolable hard filters.
- **Reproducible Historical Decisions**: Historical decisions can be replayed and audited with stored point-in-time catalog snapshots.
- **Provider Health-Aware Fallback**: Automated failover to pre-computed fallback chains on infrastructure failures.
- **Explainable Rejections**: Every non-selected candidate is accompanied by an explicit rejection reason and supporting metrics.
- **Policy-Specific Multi-Objective Optimization**: Proven mathematical divergence across Cost, Latency, Quality, and Balanced policies.

### Does Not Guarantee
- **Real-World Model Quality**: Empirical quality scores reflect synthetic benchmarks and nominal registry metadata, not universal LLM capabilities.
- **Real-World Cost Savings**: Reported cost savings depend on specific workload distributions and do not represent guaranteed production financial savings.
- **Optimal Future Routing**: Router does not dynamically extrapolate to unseen task domains without historical outcome telemetry (addressed in Stage 18).
- **Generalization Beyond Observed Workloads**: Nominal parameters do not automatically adjust to unannounced third-party provider performance regressions or drift.

## Consequences
- **Positive**: Decision-engine throughput is **$245,000 - 432,000\text{ decisions/sec}$** ($3.3 - 4.1\ \mu\text{s}$ per decision), introducing negligible latency overhead.
- **Positive**: Controlled 1,000-task synthetic workload demonstrates clear policy differentiation: Cost-Optimized ($78.2\%$ cost savings), Quality-Optimized ($0.96$ average quality), and Balanced Pareto ($75.4\%$ cost savings, $74.3\%$ P95 latency reduction, $0.90$ average quality).
- **Positive**: Full explainability logs clarify every model selection and rejection for regulatory compliance and audit trails.
