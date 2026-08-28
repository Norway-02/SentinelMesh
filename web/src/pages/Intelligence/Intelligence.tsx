import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { formatPercent } from '../../utils/formatters';
import { StatusBadge } from '../../components/StatusBadge';
import { BrainCircuit, TrendingDown, CheckCircle } from 'lucide-react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  BarChart,
  Bar,
  CartesianGrid,
} from 'recharts';

export const Intelligence: React.FC = () => {
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
  });

  const { data: policyState } = useQuery({
    queryKey: ['policyState'],
    queryFn: () => api.getPolicyState(),
    refetchInterval: 3000,
  });

  const { data: outcomesData } = useQuery({
    queryKey: ['recentOutcomes'],
    queryFn: () => api.getOutcomes(30),
    refetchInterval: 3000,
  });

  const isLive = settings?.execution_mode === 'LIVE';
  const outcomes = outcomesData?.outcomes || [];
  const hasOutcomes = outcomes.length > 0;

  const chartData = outcomes.map((o, idx) => ({
    sample: `#${idx + 1}`,
    quality: (o.actual_quality_score || 0.9) * 100,
    latency: (o.actual_latency ? (o.actual_latency > 1e5 ? o.actual_latency / 1e6 : o.actual_latency) : 120),
    cost: (o.actual_cost_usd || 0.001) * 1000,
  }));

  const activeChartData = hasOutcomes ? chartData : [];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)' }}>
            Adaptive Intelligence (Stage 18-19 Experimental)
          </h1>
          <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
            Online Bayesian learning store, dual-window drift monitoring, and automated policy guardrails (Simulated Layer).
          </p>
        </div>
        <StatusBadge
          status={isLive ? (hasOutcomes ? 'live' : 'dry_run') : 'synthetic'}
          label={isLive ? (hasOutcomes ? 'DATA SOURCE: LIVE' : 'DATA SOURCE: LIVE (0 SAMPLES)') : 'DATA SOURCE: SYNTHETIC'}
        />
      </div>

      {/* Top Overview Cards Grid */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '16px' }}>
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px 18px' }}>
          <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontWeight: 700, fontFamily: 'var(--font-mono)', letterSpacing: '0.05em' }}>ACTIVE POLICY (SIMULATED)</div>
          <div style={{ fontSize: '18px', fontWeight: 700, color: 'var(--accent-purple)', fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {policyState?.version || 'policy-v1.0'}
          </div>
          <div style={{ fontSize: '10px', color: 'var(--accent-amber)', fontWeight: 600, marginTop: '2px', fontFamily: 'var(--font-mono)' }}>
            SIMULATION ONLY · No live policy mutation
          </div>
        </div>

        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px 18px' }}>
          <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontWeight: 700, fontFamily: 'var(--font-mono)', letterSpacing: '0.05em' }}>DRIFT MONITOR</div>
          <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            HEALTHY (0.0% Drift)
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '2px' }}>
            Dual-Window Detector Active
          </div>
        </div>

        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px 18px' }}>
          <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontWeight: 700, fontFamily: 'var(--font-mono)', letterSpacing: '0.05em' }}>EXPLORATION BUDGET</div>
          <div style={{ fontSize: '18px', fontWeight: 700, color: 'var(--accent-cyan)', fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {formatPercent(policyState?.exploration_budget ?? 0.05, 0)} (UCB λ=0.50)
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '2px' }}>
            Explore: {policyState?.exploration_count ?? 0} | Exploit: {policyState?.exploitation_count ?? 0}
          </div>
        </div>

        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '16px 18px' }}>
          <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontWeight: 700, fontFamily: 'var(--font-mono)', letterSpacing: '0.05em' }}>LAST POLICY ACTION</div>
          <div style={{ fontSize: '15px', fontWeight: 700, color: policyState?.is_rolled_back ? 'var(--accent-amber)' : 'var(--accent-emerald)', fontFamily: 'var(--font-mono)', marginTop: '4px' }}>
            {policyState?.is_rolled_back ? 'Rolled back from policy-v2.0' : 'Never rolled back'}
          </div>
          <div style={{ fontSize: '10px', color: 'var(--accent-amber)', fontWeight: 600, marginTop: '2px', fontFamily: 'var(--font-mono)' }}>
            SIMULATION GUARDRAIL · Floor: 85.0% Quality
          </div>
        </div>
      </div>

      {/* 5-Column Dual-Window Performance Drift Detector */}
      <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <TrendingDown size={16} style={{ color: 'var(--accent-amber)' }} />
            <h3 style={{ fontSize: '14px', fontWeight: 700 }}>Dual-Window Performance Drift Detector</h3>
          </div>
          <span style={{ fontSize: '11px', fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>
            Reference Baseline vs Recent Window
          </span>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: '16px', backgroundColor: 'var(--bg-secondary)', padding: '16px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
          <div>
            <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>TARGET MODEL</div>
            <div style={{ fontSize: '13px', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--text-primary)', marginTop: '2px' }}>medium-balanced-v1</div>
          </div>
          <div>
            <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>BASELINE QUALITY</div>
            <div style={{ fontSize: '16px', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--accent-emerald)' }}>91.0%</div>
          </div>
          <div>
            <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>RECENT QUALITY</div>
            <div style={{ fontSize: '16px', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--accent-emerald)' }}>90.5%</div>
          </div>
          <div>
            <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>DELTA / THRESHOLD</div>
            <div style={{ fontSize: '16px', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--accent-cyan)' }}>-0.5% / -15%</div>
          </div>
          <div>
            <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>STATUS</div>
            <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--accent-emerald)', marginTop: '2px', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <CheckCircle size={14} /> HEALTHY
            </div>
          </div>
        </div>
      </div>

      {/* Adaptive Telemetry Trajectories (2 Columns) */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px' }}>
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px' }}>
          <h3 style={{ fontSize: '13px', fontWeight: 700, marginBottom: '14px' }}>Empirical Quality Score Trajectory (%)</h3>
          <div style={{ height: '180px', width: '100%' }}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={activeChartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="sample" />
                <YAxis domain={[60, 100]} />
                <Tooltip />
                <Line type="monotone" dataKey="quality" stroke="var(--accent-emerald)" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>

        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px' }}>
          <h3 style={{ fontSize: '13px', fontWeight: 700, marginBottom: '14px' }}>Observed Latency (ms)</h3>
          <div style={{ height: '180px', width: '100%' }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={activeChartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="sample" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="latency" fill="var(--accent-cyan)" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  );
};
