import React, { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { PlayCircle, ShieldCheck, CheckCircle2, Lock, FileText, ArrowLeft, Cpu, Activity, Clock, Terminal } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import { formatCurrency, formatLatency } from '../../utils/formatters';

export const RunDetails: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const runId = id || 'run-9024-alpha';

  const mockRuns = [
    {
      id: 'run-9024-alpha',
      status: 'completed',
      modelId: 'medium-balanced-v1',
      cluster: 'us-east-1-cluster-alpha',
      node: 'worker-node-02',
      generationToken: '42',
      checkpointSeq: 'chk-100',
      checkpointHash: 'sha256-8f3a9d42c10b49e',
      attestationStatus: 'PASSED (SHA-256 Digest Verified)',
      cost: 0.0016,
      latency: 112,
      timeline: [
        { stage: 'Stage 1-4: Task Submission & Ingestion', status: 'COMPLETED', time: '16:34:01.002', detail: 'Received task payload & validated schema constraints' },
        { stage: 'Stage 5-8: Generation Fencing & Scheduling', status: 'COMPLETED', time: '16:34:01.015', detail: 'Allocated worker node with Generation Token #42' },
        { stage: 'Stage 9-12: State Checkpointing & Outbox', status: 'COMPLETED', time: '16:34:01.045', detail: 'Persisted state checkpoint chk-100 (Seq #100)' },
        { stage: 'Stage 13-16: Verification & Attestation', status: 'COMPLETED', time: '16:34:01.114', detail: 'SHA-256 evidence digest verified; invariant check passed' },
        { stage: 'Stage 17: Model Router Execution', status: 'COMPLETED', time: '16:34:01.115', detail: 'Routed to medium-balanced-v1 via deterministic safety gate' },
      ],
    },
    {
      id: 'run-9025-beta',
      status: 'completed',
      modelId: 'large-reasoning-v1',
      cluster: 'eu-west-1-cluster-beta',
      node: 'worker-node-04',
      generationToken: '18',
      checkpointSeq: 'chk-046',
      checkpointHash: 'sha256-3b1a49f012e84c',
      attestationStatus: 'PASSED (SHA-256 Digest Verified)',
      cost: 0.0150,
      latency: 420,
      timeline: [
        { stage: 'Stage 1-4: Task Submission & Ingestion', status: 'COMPLETED', time: '16:35:10.100', detail: 'Received task payload & validated schema constraints' },
        { stage: 'Stage 5-8: Generation Fencing & Scheduling', status: 'COMPLETED', time: '16:35:10.112', detail: 'Allocated worker node with Generation Token #18' },
        { stage: 'Stage 9-12: State Checkpointing & Outbox', status: 'COMPLETED', time: '16:35:10.180', detail: 'Persisted state checkpoint chk-046 (Seq #46)' },
        { stage: 'Stage 13-16: Verification & Attestation', status: 'COMPLETED', time: '16:35:10.520', detail: 'SHA-256 evidence digest verified; invariant check passed' },
        { stage: 'Stage 17: Model Router Execution', status: 'COMPLETED', time: '16:35:10.522', detail: 'Routed to large-reasoning-v1 via deterministic safety gate' },
      ],
    },
  ];

  const activeRun = mockRuns.find(r => r.id === runId) || mockRuns[0];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {/* Page Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '4px' }}>
            <Link to="/runs" style={{ color: 'var(--text-muted)', textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '4px', fontSize: '12px' }}>
              <ArrowLeft size={14} /> Back to Runs
            </Link>
          </div>
          <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
            RUN: {activeRun.id}
          </h1>
          <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
            Complete Lifecycle Reconstruction — Stage 1–16 Execution, Fencing &amp; Attestation
          </p>
        </div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          <StatusBadge status="completed" label="● COMPLETED" />
          <StatusBadge status="synthetic" label="STAGE 1-16 VERIFIED" />
        </div>
      </div>

      {/* Primary Technical Details Cards Grid */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '16px' }}>
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px' }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>MODEL ENDPOINT</div>
          <div style={{ fontSize: '15px', fontWeight: 700, color: 'var(--accent-cyan)', fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {activeRun.modelId}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '2px' }}>Stage 17 Deterministic Gate</div>
        </div>

        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px' }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>TARGET CLUSTER / NODE</div>
          <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {activeRun.node}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '2px' }}>{activeRun.cluster}</div>
        </div>

        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px' }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>FENCING GENERATION TOKEN</div>
          <div style={{ fontSize: '15px', fontWeight: 700, color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            TOKEN #{activeRun.generationToken} (Gen 3)
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '2px' }}>Stale split-brain mutation guarded</div>
        </div>

        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px' }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>EXECUTION METRICS</div>
          <div style={{ fontSize: '15px', fontWeight: 700, color: 'var(--accent-purple)', fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {formatCurrency(activeRun.cost)} | {formatLatency(activeRun.latency)}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '2px' }}>100% Invariant Compliant</div>
        </div>
      </div>

      {/* Attestation & Checkpoint Box */}
      <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
        <h3 style={{ fontSize: '14px', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '8px' }}>
          <ShieldCheck size={16} color="var(--accent-emerald)" /> Cryptographic Attestation &amp; State Checkpoint
        </h3>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}>
          <div style={{ backgroundColor: 'var(--bg-secondary)', padding: '12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
            <span style={{ color: 'var(--text-muted)', fontSize: '10px', display: 'block' }}>CHECKPOINT DIGEST</span>
            <span style={{ color: 'var(--accent-cyan)', fontWeight: 700 }}>{activeRun.checkpointSeq} — {activeRun.checkpointHash}</span>
          </div>
          <div style={{ backgroundColor: 'var(--bg-secondary)', padding: '12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
            <span style={{ color: 'var(--text-muted)', fontSize: '10px', display: 'block' }}>ATTESTATION VERIFIER</span>
            <span style={{ color: 'var(--accent-emerald)', fontWeight: 700 }}>✓ {activeRun.attestationStatus}</span>
          </div>
        </div>
      </div>

      {/* Lifecycle Timeline Visualizer */}
      <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <h3 style={{ fontSize: '14px', fontWeight: 700, fontFamily: 'var(--font-mono)', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Activity size={16} color="var(--accent-blue)" /> Stage 1–17 Execution Lifecycle Timeline
        </h3>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          {activeRun.timeline.map((item, idx) => (
            <div key={idx} style={{ display: 'flex', alignItems: 'flex-start', gap: '14px', padding: '10px 14px', backgroundColor: 'var(--bg-secondary)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ color: 'var(--accent-emerald)', marginTop: '2px' }}>
                <CheckCircle2 size={16} />
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <span style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>{item.stage}</span>
                  <span style={{ fontSize: '11px', color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>{item.time}</span>
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px', fontFamily: 'var(--font-mono)' }}>
                  {item.detail}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export const RunsList: React.FC = () => {
  const runs = [
    { id: 'run-9024-alpha', modelId: 'medium-balanced-v1', cluster: 'us-east-1-cluster-alpha', node: 'worker-node-02', token: '42', status: 'completed', cost: '$0.0016', latency: '112ms', time: '16:34:01' },
    { id: 'run-9025-beta', modelId: 'large-reasoning-v1', cluster: 'eu-west-1-cluster-beta', node: 'worker-node-04', token: '18', status: 'completed', cost: '$0.0150', latency: '420ms', time: '16:35:10' },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)' }}>
            Execution Runs Log
          </h1>
          <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
            Historical record of execution runs, fencing tokens, checkpoints, and attestation status.
          </p>
        </div>
        <StatusBadge status="completed" label="● 2 RUNS RECORDED" />
      </div>

      <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
          {runs.map((r) => (
            <Link
              key={r.id}
              to={`/runs/${r.id}`}
              style={{
                textDecoration: 'none',
                backgroundColor: 'var(--bg-secondary)',
                border: '1px solid var(--border-color)',
                borderRadius: 'var(--radius-sm)',
                padding: '14px 16px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                color: 'var(--text-primary)',
              }}
            >
              <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                <span style={{ fontSize: '14px', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--accent-blue)' }}>
                  {r.id}
                </span>
                <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                  Cluster: {r.cluster} | Node: {r.node} | Fencing: #{r.token}
                </span>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '16px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}>
                <span style={{ color: 'var(--accent-cyan)', fontWeight: 600 }}>{r.modelId}</span>
                <span style={{ color: 'var(--accent-emerald)' }}>{r.cost} / {r.latency}</span>
                <StatusBadge status="completed" size="sm" />
              </div>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
};
