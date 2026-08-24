# Contributing to SentinelMesh

Thank you for your interest in contributing to **SentinelMesh**, the distributed control plane for autonomous multi-agent systems!

---

## 1. Code Quality & Architecture Standards

1. **Safety Floor Invariance**:
   - Any modifications to the scheduling or model routing layers MUST preserve the Stage 17 safety invariant:
     $$\forall \text{req}, \text{state} \implies \text{SelectedModel} \in \text{Stage17FeasibleSet}$$
   - Run `make test-property` to ensure 10,000 randomized property tests pass with 0 violations.
2. **File Size & Separation of Concerns**:
   - Keep files concise (typically 200–300 lines max).
   - Separate concerns cleanly:
     - `internal/router/` — Deterministic feasibility & circuit breaker.
     - `internal/adaptive/` — Empirical predictors & drift detection.
     - `internal/onlinepolicy/` — Reward model, UCB exploration & guardrail rollback.
3. **Deterministic Testing**:
   - All tests and benchmarks must be repeatable with explicit random seeds.
   - Use `make test` and `make bench` before submitting contributions.
4. **Zero Secret Leakage**:
   - Never log raw prompts, full responses, or authorization tokens.
   - Run `make test-security` to verify containment and secret redaction.

---

## 2. Development Commands

```bash
# Run unit & integration tests
make test

# Run property invariance tests (10,000 randomized iterations)
make test-property

# Run security & chaos containment tests
make test-security
make test-chaos

# Run microbenchmarks & trace evaluations
make bench

# Run interactive demonstrations
make demo        # Path A: 3-Minute Hero Adaptive Routing Demo
make demo-deep   # Path B: Deep Distributed Control Plane Demo
```

---

## 3. Commit Convention

We follow the **Conventional Commits** specification:
- `feat:` New feature or capability
- `fix:` Bug fix or security patch
- `test:` New or updated unit/property/e2e tests
- `bench:` Performance benchmark or profiling update
- `docs:` Documentation, ADR, or whitepaper changes
- `refactor:` Code restructuring without behavior changes
