import React from 'react';
import { Network, ShieldCheck, CheckCircle2, Lock, FileText } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';

export const Distributed: React.FC = () => {
  const clusters = [
    {
      name: 'Cluster Alpha (us-east-1)',
      status: 'healthy',
      agents: [
        { id: 'agent-node-01', role: 'Scheduler Leader', state: 'RUNNING', fencingToken: '42', checkpoint: 'chk-100', attestation: 'VALID' },
        { id: 'agent-node-02', role: 'Worker Node', state: 'RUNNING', fencingToken: '42', checkpoint: 'chk-099', attestation: 'VALID' },
        { id: 'agent-node-03', role: 'Verifier Node', state: 'VERIFYING', fencingToken: '42', checkpoint: 'chk-100', attestation: 'VALID' },
      ],
    },
    {
      name: 'Cluster Beta (eu-west-1)',
      status: 'healthy',
      agents: [
        { id: 'agent-node-04', role: 'Worker Node', state: 'RUNNING', fencingToken: '18', checkpoint: 'chk-045', attestation: 'VALID' },
        { id: 'agent-node-05', role: 'Worker Node', state: 'CHECKPOINTING', fencingToken: '18', checkpoint: 'chk-046', attestation: 'VALID' },
      ],
    },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)' }}>
            Distributed Control Plane &amp; Consensus
          </h1>
          <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
            Multi-cluster topology, fencing generation tokens, state checkpoints, and zero-trust attestation.
          </p>
        </div>
        <StatusBadge status="synthetic" label="DATA SOURCE: SIMULATED TOPOLOGY" />
      </div>

      {/* Summary Bar */}
      <div
        style={{
          backgroundColor: 'var(--bg-panel)',
          border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-md)',
          padding: '14px 20px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          fontSize: '12px',
          fontFamily: 'var(--font-mono)',
        }}
      >
        <div><strong style={{ color: 'var(--text-muted)' }}>CLUSTERS:</strong> <span style={{ color: 'var(--accent-blue)', fontWeight: 700 }}>2</span></div>
        <div><strong style={{ color: 'var(--text-muted)' }}>NODES:</strong> <span style={{ color: 'var(--text-primary)', fontWeight: 700 }}>5</span></div>
        <div><strong style={{ color: 'var(--text-muted)' }}>RUNNING:</strong> <span style={{ color: 'var(--accent-emerald)', fontWeight: 700 }}>3</span></div>
        <div><strong style={{ color: 'var(--text-muted)' }}>VERIFYING:</strong> <span style={{ color: 'var(--accent-cyan)', fontWeight: 700 }}>1</span></div>
        <div><strong style={{ color: 'var(--text-muted)' }}>CHECKPOINTING:</strong> <span style={{ color: 'var(--accent-purple)', fontWeight: 700 }}>1</span></div>
        <div><strong style={{ color: 'var(--text-muted)' }}>FAILED:</strong> <span style={{ color: 'var(--accent-emerald)', fontWeight: 700 }}>0</span></div>
      </div>

      {/* Security & Checkpoint Cards Grid */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '16px' }}>
        {/* Fencing Token Card */}
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '18px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', letterSpacing: '0.05em' }}>
              FENCING GENERATION TOKEN
            </span>
            <Lock size={16} style={{ color: 'var(--accent-emerald)' }} />
          </div>
          <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)', marginTop: '6px' }}>
            TOKEN #42 (GEN 3)
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
            ✓ Stale split-brain mutation prevented
          </div>
        </div>

        {/* Checkpoint Hash Card */}
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '18px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', letterSpacing: '0.05em' }}>
              LATEST CHECKPOINT
            </span>
            <FileText size={16} style={{ color: 'var(--accent-cyan)' }} />
          </div>
          <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--accent-cyan)', fontFamily: 'var(--font-mono)', marginTop: '6px' }}>
            chk-100 (Seq #100)
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
            SHA-256: 8f3a...b49e (Merkle Root Verified)
          </div>
        </div>

        {/* Attestation Card */}
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '18px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', letterSpacing: '0.05em' }}>
              ATTESTATION VERIFIER
            </span>
            <ShieldCheck size={16} style={{ color: 'var(--accent-emerald)' }} />
          </div>
          <div style={{ fontSize: '20px', fontWeight: 700, color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)', marginTop: '6px' }}>
            100% PASSED (Signature Valid)
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
            Zero-trust state cryptographic proof
          </div>
        </div>
      </div>

      {/* Cluster Nodes Topology View */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
        {clusters.map((c) => (
          <div
            key={c.name}
            style={{
              backgroundColor: 'var(--bg-panel)',
              border: '1px solid var(--border-color)',
              borderRadius: 'var(--radius-md)',
              padding: '20px',
              display: 'flex',
              flexDirection: 'column',
              gap: '14px',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Network size={18} style={{ color: 'var(--accent-blue)' }} />
                <h3 style={{ fontSize: '14px', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{c.name}</h3>
              </div>
              <StatusBadge status={c.status} size="sm" />
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '12px' }}>
              {c.agents.map((a) => (
                <div
                  key={a.id}
                  style={{
                    backgroundColor: 'var(--bg-secondary)',
                    border: '1px solid var(--border-color)',
                    borderRadius: 'var(--radius-sm)',
                    padding: '12px',
                    fontSize: '11px',
                    fontFamily: 'var(--font-mono)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '6px',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span style={{ fontWeight: 700, color: 'var(--text-primary)' }}>{a.id}</span>
                    <span style={{ fontWeight: 700, color: a.state === 'RUNNING' ? 'var(--accent-emerald)' : 'var(--accent-cyan)' }}>● {a.state}</span>
                  </div>
                  <div style={{ color: 'var(--text-muted)' }}>Role: {a.role}</div>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '4px', marginTop: '4px', borderTop: '1px solid var(--border-color)', paddingTop: '6px', color: 'var(--text-dim)' }}>
                    <div>Fencing: #{a.fencingToken}</div>
                    <div>Checkpoint: {a.checkpoint}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
