# SentinelMesh — A Distributed Agent Control Plane with Safe Adaptive Model Routing

[![Go Reference](https://pkg.go.dev/badge/github.com/sentinelmesh/sentinelmesh.svg)](https://pkg.go.dev/github.com/sentinelmesh/sentinelmesh)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/sentinelmesh/sentinelmesh)](https://goreportcard.com/report/github.com/sentinelmesh/sentinelmesh)
[![Tests](https://img.shields.io/badge/Tests-100%25%20Passing-brightgreen.svg)]()
[![Stages](https://img.shields.io/badge/Stages-20%2F20%20Complete-success.svg)]()

**SentinelMesh** is a high-performance, fault-resilient distributed control plane designed for mission-critical autonomous AI agent workflows. It unifies distributed state machines, multi-cluster scheduling, transactional event delivery, deterministic checkpointing, cryptographic execution attestation, and a three-tier intelligent model routing engine.

---

## 🏛️ Architecture & Evidence Matrix

The following matrix transparently defines what capabilities SentinelMesh guarantees and how each claim is experimentally verified:

| Architectural Capability | Concrete System Invariant / Mechanism | Validation Evidence | Evidence Type |
| :--- | :--- | :--- | :--- |
| **Deterministic Feasibility** | Selected model strictly satisfies security, SLA, and capacity gates | `TestRouter_NeverViolatesConstraints` (10k runs) | `[PROPERTY TEST]` |
| **Historical Replay** | Identical request + catalog state reproduces bitwise decision | `TestRouter_DeterministicReplay` (1,000 runs) | `[DETERMINISTIC TEST]` |
| **Predictive Adaptation** | Empirical distributions & dual-window drift detection ($\Delta Q \ge 0.15$) | Stage 18 trace evaluation | `[CONTROLLED SYNTHETIC]` |
| **Safe Online Exploration** | Contextual UCB bounded by 5% rolling exploration budget | `TestPolicyNeverViolatesStage17Constraints` (10k) | `[PROPERTY TEST]` |
| **SLA Attainment Under Drift**| 100.0% quality pass rate under silent upstream degradation | 1,000-task multi-stage comparison trace | `[CONTROLLED SYNTHETIC]` |
| **Live Protocol Compatibility**| OpenAI, Anthropic, Gemini & Ollama request/response parsing | `TestLiveProvider_*` protocol test suite | `[LIVE PROTOCOL TEST]` |
| **1M-Node Scalability** | 87.7 ms P50 placement time using bitmask filtering | `TestScheduler_1MillionNodes_Benchmark` | `[SIMULATED WORKLOAD]` |
| **Split-Brain Prevention** | Monotonic fencing token lease rejection during partition | `TestSecurity_FencingTokenEnforcement` | `[FAULT INJECTION]` |
| **Crash Recovery (RTO)** | 1.24 s automated standby lease reassignment on SIGKILL | `TestChaos_WorkerCrash` chaos test | `[FAULT INJECTION]` |

---

## 🏛️ Architecture Overview

```text
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   SENTINELMESH ARCHITECTURE                                      │
├─────────────────────────┬─────────────────────────┬──────────────────────────────────────────────┤
│ 1. Core Distributed OS  │ 2. Resilience & Trust   │ 3. Intelligent Model Control Plane           │
├─────────────────────────┼─────────────────────────┼──────────────────────────────────────────────┤
│ • Monotonic State Engine│ • Deterministic Chkpts  │ • Tier 1: Deterministic Feasible Set (St. 17)│
│ • Postgres Concurrency  │ • SHA-256 State Hashing │ • Tier 2: Uncertainty-Aware Predictor (St. 18│
│ • Transactional Outbox  │ • Merkle Attestations   │ • Tier 3: Contextual UCB Bandit (St. 19)     │
│ • NATS JetStream Spine  │ • Recovery Coordinator  │ • Multi-Provider Adapters (OpenAI/Anthropic) │
│ • K8s Operator / CRD    │ • Chaos Fault Matrix    │ • Safety Guardrails & Automated Rollback     │
│ • Fencing Token Leases  │ • Security Containment  │ • 10,000 Property Invariance Proofs          │
└─────────────────────────┴─────────────────────────┴──────────────────────────────────────────────┘
```

---

## 🚀 20-Stage Implementation Progression (20/20 Complete)

| Stage | Domain Area | Key Feature Delivered | Status |
| :---: | :--- | :--- | :---: |
| **01** | Core Domain | Agent, Run, Node, Policy & State Machine | ✅ Completed |
| **02** | Persistence | PostgreSQL Storage Engine & Migrations | ✅ Completed |
| **03** | Application | Lifecycle Orchestration Services | ✅ Completed |
| **04** | Transport | gRPC Services & Protocol Buffers | ✅ Completed |
| **05** | Transport | REST API Gateway & OpenAPI Specifications | ✅ Completed |
| **06** | Messaging | Transactional Outbox & NATS JetStream Engine | ✅ Completed |
| **07** | Kubernetes | Custom Operator & `AgentDeployment` CRD | ✅ Completed |
| **08** | Kubernetes | Reconciler State Machine & Pod Lifecycle | ✅ Completed |
| **09** | Security | Seccomp, AppArmor & Network Isolation Rules | ✅ Completed |
| **10** | Security | Multi-Tenant RBAC & Security Audit Logs | ✅ Completed |
| **11** | Resilience | Deterministic Application Checkpointing | ✅ Completed |
| **12** | Verification | Merkle Evidence Digest & Attestation Engine | ✅ Completed |
| **13** | Observability| Prometheus Metrics, Tracing & Health Probes | ✅ Completed |
| **14** | Topology | Multi-Cluster Federation & Monotonic Fencing | ✅ Completed |
| **15** | Chaos | Fault-Injection Framework & RTO Testing | ✅ Completed |
| **16** | Scalability | 1,000,000-Node Optimized Scheduler Bitmask | ✅ Completed |
| **17** | Intelligence | Multi-Objective Deterministic Model Router | ✅ Completed |
| **18** | Intelligence | Predictive Uncertainty & Statistical Drift | ✅ Completed |
| **19** | Intelligence | Safe Contextual UCB Bandit & Auto-Rollback | ✅ Completed |
| **20** | Production | Live Providers, Research Paper & Release | ✅ Completed |

---

## 🧠 The 3-Tier Intelligent Model Control Plane

```text
                         NEW AGENT TASK
                               │
                               ▼
                 ┌───────────────────────────┐
                 │  TIER 1: STAGE 17         │
                 │  "What is allowed?"       │
                 │  Deterministic Hard Gates │
                 └─────────────┬─────────────┘
                               │ Feasible Set
                               ▼
                 ┌───────────────────────────┐
                 │  TIER 2: STAGE 18         │
                 │  "What will happen?"      │
                 │  Uncertainty Predictors   │
                 └─────────────┬─────────────┘
                               │ Empirical Distributions (Q, S, L, C, σ)
                               ▼
                 ┌───────────────────────────┐
                 │  TIER 3: STAGE 19         │
                 │  "How to choose safely?"  │
                 │  Contextual UCB Bandit    │
                 │  Guardrails & Rollback    │
                 └─────────────┬─────────────┘
                               │
                    Selected Execution Arm
```

1. **Tier 1 (Stage 17)**: Filters candidates against security classes, context tokens, budget limits, latency SLAs, and circuit breaker status.
2. **Tier 2 (Stage 18)**: Computes Beta-Binomial success posteriors, quality shrinkage, latency regressions, and detects statistical drift.
3. **Tier 3 (Stage 19)**: Scores arms with Contextual UCB ($\text{UCB} = U_m + \lambda \sigma_m$), caps exploration at $5\%$, and automatically rolls back on guardrail breach.
4. **Safety Invariant**: $\forall \text{req}, \text{state} \implies \text{SelectedModel} \in \text{Stage17FeasibleSet}$ (100% verified across 10,000 randomized property tests).

---

## 📊 Key Benchmark Highlights

- **1M-Node Scheduler**: Under the simulated million-node scheduler benchmark, the optimized bitmask filter achieved an **87.7 ms P50 decision time** ($2.16\times$ speedup, $59.1\%$ memory reduction, 3 allocs/op).
- **Policy Engine**: **920 ns** evaluation latency ($1,086,950\text{ ops/sec}$).
- **Online Policy Router**: **5.69 µs** decision latency ($199,942\text{ decisions/sec}$, 38 allocs/op).
- **1,000-Task Trace**: **100.0% Quality Pass Rate** under injected provider drift, with 0 constraint violations across all evaluations.
- **Failover RTO**: **1.24 s** crash recovery, **0 ms** instantaneous split-brain partition blocking via fencing tokens.

---

## ⚡ Quickstart

### Prerequisites
- Go 1.22+
- Docker / Podman (optional for live multi-node setups)

### Build & Test
```bash
# Clone the repository
git clone https://github.com/sentinelmesh/sentinelmesh.git
cd sentinelmesh

# Run all unit and integration tests
make test

# Run 10,000-iteration safety invariant property checks
make test-property

# Run microbenchmarks and trace comparisons
make bench
```

### Run Demonstrations
```bash
# Path A: 3-Minute Hero Adaptive Routing & Live Provider Demo
make demo

# Path B: Deep Technical Distributed Control Plane Demo
make demo-deep
```

---

## 📁 Repository Structure

```text
├── api/proto/               # gRPC & Protocol Buffer definitions
├── benchmark/               # Microbenchmarks & 1M-node scalability suite
├── cmd/                     # Executable binaries & demo applications
│   ├── demo-stage17/        # Stage 17 Deterministic Router demo
│   ├── demo-stage18/        # Stage 18 Predictive Scheduler demo
│   ├── demo-stage19/        # Stage 19 Policy Learning demo
│   └── demo-stage20/        # Stage 20 Grand Finale production demo
├── docs/                    # Architectural decisions & research whitepaper
│   ├── design/              # ADRs (ADR-001 through ADR-012)
│   ├── benchmarks/          # Stage benchmark & evaluation compendiums
│   └── research/            # Formal research whitepaper
├── internal/                # Core Go packages
│   ├── checkpoint/          # Application-level deterministic checkpointing
│   ├── domain/              # Monotonic state machine & domain entities
│   ├── onlinepolicy/        # Stage 19 Contextual UCB Bandit & Guardrails
│   ├── adaptive/            # Stage 18 Empirical Predictors & Drift Detector
│   ├── router/              # Stage 17 Deterministic Router & Live Adapters
│   ├── scheduler/           # 1M-Node Bitmask Multi-Cluster Scheduler
│   ├── verification/        # Cryptographic Merkle Attestation Engine
│   └── outbox/              # PostgreSQL Outbox & JetStream Publisher
├── repro/                   # Benchmark manifests & reproduction seeds
├── scripts/                 # Automated execution and demo scripts
├── test/                    # Chaos, Security, Property & E2E test suites
├── Makefile                 # Build, test, lint & demo targets
├── LICENSE                  # Apache 2.0 License
└── README.md
```

---

## 📄 Documentation & Research

- **[Research Whitepaper](docs/research/SENTINELMESH_RESEARCH_WHITEPAPER.md)**: Formal academic specification and systems comparative analysis.
- **[Master Benchmark Compendium](docs/benchmarks/STAGE20_FINAL_EVALUATION.md)**: Synthesis of all microbenchmarks, chaos matrices, and 1,000-task traces.
- **[Architecture Decision Records](docs/design/)**: Formal specifications from ADR-001 to ADR-012.

---

## ⚖️ License

SentinelMesh is licensed under the [Apache 2.0 License](LICENSE).
