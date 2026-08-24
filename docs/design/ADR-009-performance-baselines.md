# ADR-009: Empirical Performance Baselines and Scalability SLAs

## Status
Accepted

## Context
Following distributed invariant verification in Stage 15, SentinelMesh requires quantitative, empirical performance baselines to establish throughput and latency Service Level Agreements (SLAs). 

Furthermore, as the project prepares to incorporate machine-learning routing (Stage 17) and predictive scheduling (Stage 18), establishing rigorous deterministic baselines is mandatory to scientifically prove whether adaptive and AI-driven optimizations produce genuine efficiency gains over the deterministic baseline.

## Decision
We establish standard empirical performance SLAs across five architectural layers, verified under native benchmark suites (`benchmark/`):

1. **Scheduler Scalability**:
   - **First-Fit**: Sub-microsecond decision latency ($\approx 35\text{ ns/op}$, $0\text{ B/op}$) across all synthetic node scales up to 1,000,000 nodes.
   - **Deterministic v1 (Single-Cluster)**: Exhaustive multi-dimensional candidate scoring scales linearly $O(N)$ with node count. After profile-guided optimization (hoisting CPU/memory parsing and pre-allocating slice capacities), latency for 1,000,000 nodes was reduced from $189.8\text{ ms}$ to $87.7\text{ ms}$ ($-53.8\%$) and heap allocations reduced from $746\text{ MB}$ to $305\text{ MB}$ ($-59.1\%$).
   - **Two-Tier Multi-Cluster Placement**: By decoupling global cluster selection ($O(C)$) from local intra-cluster scoring ($O(N_{\text{selected}})$), scheduling across 1,000,000 nodes partitioned across 100 clusters completes in **$<1\text{ ms}$ ($689\ \mu\text{s}$ P50)**, achieving a **$275\times$ speedup** over flat single-cluster evaluation.

2. **Transactional Outbox & Event Pipeline**:
   - Outbox event insertion maintains $>400,000\text{ events/s}$ ($<2.5\ \mu\text{s}$ P50) in memory.
   - Batch claiming scales sub-linearly with batch sizes from 10 to 500 events ($>500,000\text{ claimed events/s}$).
   - End-to-end placement latency (`RunCreated` event $\to$ Scheduler evaluation $\to$ `RunScheduled` outbox record) operates with **$15.3\ \mu\text{s}$ P50** latency.

3. **Checkpoint Storage & Checksum Bandwidth**:
   - SHA-256 canonical hashing processes state payloads at $>400\text{ MB/s}$ on CPU hardware.
   - State payloads $\le 100\text{ KB}$ (Inline Tier) save and verify in $<300\ \mu\text{s}$.
   - State payloads $>1\text{ MB}$ (URI / Object Store Tier) stream external blobs with verification digest overhead $<2\text{ ms}$.

4. **Concurrent Self-Healing Recovery**:
   - Single-run recovery executes in $<60\ \mu\text{s}$ P50 ($>16,000\text{ recoveries/s}$).
   - Under concurrent failure bursts of 100 simultaneous node/run failovers, total recovery across all affected runs completes in $<12\text{ ms}$ aggregate burst latency.

5. **Outcome Verification & Attestation Overhead**:
   - Invariant and health rule evaluation + SHA-256 evidence certificate digest generation adds only **$16.2\ \mu\text{s}$ P50** to the run lifecycle.
   - For standard 10-second agent tasks, outcome attestation represents **$<0.001\%$ platform overhead**.

## Consequences
- **Positive**: Hard empirical SLAs documented for control-plane capacity planning.
- **Positive**: Profile-guided optimization proved $2.16\times$ scheduler speedup and $59.1\%$ memory reduction.
- **Positive**: Machine-readable benchmark outputs (`benchmark/results/benchmark_data.json` and `.csv`) enable automated regression detection in CI/CD.
- **Positive**: Establishes the exact baseline against which Stages 17-18 model routing and predictive schedulers will be evaluated.
