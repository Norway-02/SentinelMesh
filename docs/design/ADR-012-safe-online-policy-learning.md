# ADR-012: Safe Online Policy Learning & Exploration with Automated Rollback

## Context
Stage 17 answered **"What is allowed?"** via deterministic hard security/capacity gates.  
Stage 18 answered **"What is likely to happen?"** via empirical predictive distributions, uncertainty estimation, and statistical drift detection.

However, static or purely exploitative routing cannot discover whether under-sampled models have improved, nor can it optimize a customizable multi-objective reward function over time. We need an online policy learning mechanism that balances exploration against exploitation, optimizes scalar rewards across Quality, Reliability, Cost, and Latency, enforces strict exploration budgets, and automatically rolls back if experimental policies breach production guardrails.

## Decision
We implement **Safe Online Policy Learning & Exploration** based on a **Contextual Upper Confidence Bound (UCB) Bandit** with **Hysteresis Safety Guardrails and Automated Policy Rollback**.

### Core Architectural Laws:
1. **Inviolable Stage 17 Safety Floor**:
   - The contextual bandit samples and selects candidates **strictly within the Stage 17 safe feasible set**.
   - Under no circumstances can exploration or reward maximization bypass security classifications, context window capacity, endpoint health status, or mandatory quality floors.
   - Property invariant: $\forall \text{req}, \text{state} \implies \text{SelectedArm} \in \text{Stage17FeasibleSet}$.

2. **Normalized Multi-Objective Expected Utility & UCB Scoring**:
   - Normalized Expected Utility:
     $$U_m = w_q \hat{E}(Q_m) + w_s \hat{P}(\text{success}_m) - w_c C^{\text{norm}}_m - w_l L^{\text{norm}}_m - w_f F_m$$
   - Total Dimensionless Uncertainty $\sigma_m = \sqrt{\text{Var}(Q_m) + \text{Var}(P_m) + \text{TailRisk}_m^2}$.
   - Contextual UCB Score:
     $$\text{UCB}_m = U_m + \lambda \cdot \sigma_m \quad (\lambda = 0.50)$$

3. **Controlled Exploration Budgets**:
   - Rolling window $W_{\text{explore}} = 200$ decisions.
   - Global exploration ceiling: $\le 5\%$ ($\le 10$ decisions in 200).
   - Per-model exploration ceiling: $\le 2\%$ ($\le 4$ decisions in 200).
   - Exploration occurs only if $\text{Confidence}_m < 0.80$ and budgets permit. Otherwise, the engine executes pure exploitation ($\text{argmax}_m U_m$).

4. **Multi-Objective Reward Model**:
   - Scalar empirical reward:
     $$R = w_q Q + w_s S - w_c C_{\text{norm}} - w_l L_{\text{norm}} - w_f F \in [-1.0, 1.0]$$
   - $S = 1.0$ only if execution succeeds AND output quality satisfies the SLA requirement.

5. **Guardrails with Hysteresis & Automated Rollback**:
   - Monitored window $W=50$: Mean Quality $\ge 0.85$, Cost increase $\le 20\%$, Latency increase $\le 25\%$, Fallbacks $\le 5\%$.
   - Breach Action: Immediate automated rollback to `ParentVersion`, locking exploration rate, and publishing `SubjectPolicyRollbackTriggered`.
   - Hysteresis Recovery: Requires window mean quality $\ge 0.88$ for $N=30$ consecutive decisions before new policy promotions are permitted.

6. **Shadow & Canary Deployment Modes**:
   - `ModeShadow`: Live executions stay on baseline; policy runs in shadow to compute recommendations and shadow reward without user impact.
   - `ModeCanary`: Configurable traffic fraction (e.g. 10%) routed through experimental policy.
   - `ModeActive`: 100% policy-controlled execution.

## What Stage 19 Guarantees vs. Does Not Guarantee

### Guarantees
- **Safety Invariance**: $100\%$ guarantee that policy decisions (including exploratory ones) never violate Stage 17 feasible sets (verified over 10,000 randomized property tests).
- **Bounded Exploration Overhead**: Global exploration is mathematically capped at $5\%$ over rolling 200-task windows.
- **Automated Failure Containment**: Degraded experimental policies trigger automated rollback within 15 observations.
- **Normalized Utility Coherence**: Cost and latency are normalized before utility calculation, preventing dimensional scale skew.
- **Auditability & Trace Replay**: Decisions stamp `PolicyVersion`, `RewardVersion`, and `ExplorationVersion`.

### Does Not Guarantee
- **Unconditional Sublinear Regret in Non-Stationary Environments**: Regret bounds apply to stationary periods; during active provider drift, regret temporarily increases before adaptation.
- **Global Network Capacity Optimization**: Multi-cluster global portfolio capacity optimization is deferred to Stage 20.

## Consequences
- **Positive**: Decision-engine throughput is **$199,000\text{ decisions/sec}$** ($5.69\ \mu\text{s}$ per decision), adding virtually no overhead.
- **Positive**: 1,000-task trace evaluation achieves a **$100.0\%$ Quality Pass Rate** under injected provider drift, with **0 constraint violations** across 10,000 randomized property tests.
- **Trade-off / Observation**: Stage 19 prioritizes quality SLA certainty and exploration safety, resulting in higher operational cost ($8.4545 vs $7.1205) and slightly higher cumulative regret (+0.71%) compared to Stage 18 on stationary traces. This authentic trade-off protects critical production workloads from silent provider regressions.
- **Positive**: Automated rollback recovers immediately from toxic policy candidates without operator intervention or catalog modifications.
