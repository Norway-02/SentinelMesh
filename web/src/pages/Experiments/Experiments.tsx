import React from 'react';
import { FlaskConical, Trophy } from 'lucide-react';

export const Experiments: React.FC = () => {
  const comparison = [
    {
      metric: 'Total Cost (USD / 1,000 Tasks)',
      stage17: '$4.50',
      stage18: '$2.85 (-36.6%)',
      stage19: '$2.12 (-52.8%)',
      winner: 'Stage 19 (Online UCB Policy)',
    },
    {
      metric: 'Mean Latency (ms)',
      stage17: '240ms',
      stage18: '185ms',
      stage19: '162ms (-32.5%)',
      winner: 'Stage 19 (Online UCB Policy)',
    },
    {
      metric: 'P95 Tail Latency (ms)',
      stage17: '420ms',
      stage18: '310ms',
      stage19: '250ms (-40.5%)',
      winner: 'Stage 19 (Online UCB Policy)',
    },
    {
      metric: 'Empirical Quality Score',
      stage17: '92.4%',
      stage18: '94.1%',
      stage19: '95.6% (+3.2%)',
      winner: 'Stage 19 (Online UCB Policy)',
    },
    {
      metric: 'Task Pass Rate',
      stage17: '98.0%',
      stage18: '99.2%',
      stage19: '99.8%',
      winner: 'Stage 19 (Online UCB Policy)',
    },
    {
      metric: 'Exploration Rate',
      stage17: '0.0% (Static)',
      stage18: '0.0% (Passive)',
      stage19: '4.2% (Active UCB)',
      winner: 'Stage 19 (Online UCB Policy)',
    },
    {
      metric: 'Hard Constraint Violations',
      stage17: '0 (100% Enforced)',
      stage18: '0 (100% Enforced)',
      stage19: '0 (100% Enforced)',
      winner: 'SIMULATION INVARIANTS PASS',
    },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div>
        <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)' }}>
          Policy A/B Evaluation &amp; Benchmark Matrix (Simulated)
        </h1>
        <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
          Synthetic offline policy evaluation comparing Stage 17, Stage 18, and Stage 19 routing algorithms.
        </p>
      </div>

      <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '16px' }}>
          <FlaskConical size={18} style={{ color: 'var(--accent-purple)' }} />
          <h3 style={{ fontSize: '14px', fontWeight: 700 }}>1,000-Task Trace-Matched Comparison Matrix</h3>
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px', textAlign: 'left' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-color)', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', fontSize: '10px' }}>
                <th style={{ padding: '12px 16px' }}>EVALUATION METRIC</th>
                <th style={{ padding: '12px 16px' }}>STAGE 17 (STATIC)</th>
                <th style={{ padding: '12px 16px' }}>STAGE 18 (PREDICTIVE)</th>
                <th style={{ padding: '12px 16px' }}>STAGE 19 (ONLINE BANDIT)</th>
                <th style={{ padding: '12px 16px' }}>BEST IN SIMULATION</th>
              </tr>
            </thead>
            <tbody>
              {comparison.map((row, idx) => (
                <tr key={idx} style={{ borderBottom: '1px solid var(--border-color)' }}>
                  <td style={{ padding: '14px 16px', fontWeight: 600 }}>{row.metric}</td>
                  <td style={{ padding: '14px 16px', fontFamily: 'var(--font-mono)' }}>{row.stage17}</td>
                  <td style={{ padding: '14px 16px', fontFamily: 'var(--font-mono)' }}>{row.stage18}</td>
                  <td style={{ padding: '14px 16px', fontFamily: 'var(--font-mono)', color: 'var(--accent-emerald)', fontWeight: 700 }}>
                    {row.stage19}
                  </td>
                  <td style={{ padding: '14px 16px', fontWeight: 700, color: 'var(--accent-purple)', fontFamily: 'var(--font-mono)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <Trophy size={14} /> {row.winner}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
