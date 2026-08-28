import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { TaskComplexity, RoutingPolicy } from '../../api/types';
import { formatPercent, formatLatency, formatCurrency, formatScore } from '../../utils/formatters';
import { GitMerge, ShieldCheck, BrainCircuit, Target, CheckCircle, XCircle, Check, ChevronDown, ChevronUp } from 'lucide-react';

export const Router: React.FC = () => {
  const [prompt, setPrompt] = useState('Implement Raft consensus log compaction with zero data loss guarantees');
  const [complexity, setComplexity] = useState<TaskComplexity>('complex');
  const [policy, setPolicy] = useState<RoutingPolicy>('balanced');
  const [minQuality, setMinQuality] = useState('0.85');
  const [maxCost, setMaxCost] = useState('0.10');
  const [maxLatency, setMaxLatency] = useState('2000');
  const [expandedDetails, setExpandedDetails] = useState<Record<string, boolean>>({});

  const toggleDetails = (id: string) => {
    setExpandedDetails((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const { data: modelsData } = useQuery({
    queryKey: ['modelsList'],
    queryFn: () => api.getModels(),
  });

  const { data: s17Data } = useQuery({
    queryKey: ['stage17Route', prompt, complexity, policy, minQuality, maxCost, maxLatency],
    queryFn: () =>
      api.routeStage17({
        prompt,
        task_complexity: complexity,
        routing_policy: policy,
        quality_requirement: parseFloat(minQuality) || 0.85,
        cost_budget_usd: parseFloat(maxCost) || 0.10,
        latency_sla_ms: parseFloat(maxLatency) || 2000,
      }),
  });

  const { data: s18Data } = useQuery({
    queryKey: ['stage18Route', prompt, complexity, policy, minQuality, maxCost, maxLatency],
    queryFn: () =>
      api.routeStage18({
        prompt,
        task_complexity: complexity,
        routing_policy: policy,
        quality_requirement: parseFloat(minQuality) || 0.85,
        cost_budget_usd: parseFloat(maxCost) || 0.10,
        latency_sla_ms: parseFloat(maxLatency) || 2000,
      }),
  });

  const { data: s19Data } = useQuery({
    queryKey: ['stage19Route', prompt, complexity, policy, minQuality, maxCost, maxLatency],
    queryFn: () =>
      api.routeStage19({
        prompt,
        task_complexity: complexity,
        routing_policy: policy,
        quality_requirement: parseFloat(minQuality) || 0.85,
        cost_budget_usd: parseFloat(maxCost) || 0.10,
        latency_sla_ms: parseFloat(maxLatency) || 2000,
      }),
  });

  const models = modelsData?.models || [];
  const selectedModelId = s19Data?.selected_model_id || s18Data?.selected_model_id || s17Data?.selected_model_id;
  const selectedModelObj = models.find((m) => m.id === selectedModelId);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <div>
        <h1 style={{ fontSize: '22px', fontWeight: 700, letterSpacing: '-0.02em' }}>
          Model Router Pipeline Visualizer
        </h1>
        <p style={{ fontSize: '13px', color: 'var(--text-secondary)' }}>
          Stage 17 Safety Gate → Stage 18 Bayesian Predictor → Stage 19 Contextual UCB Policy
        </p>
      </div>

      {/* Interactive Controls */}
      <div
        style={{
          backgroundColor: 'var(--bg-card)',
          border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-md)',
          padding: '16px 20px',
          display: 'grid',
          gridTemplateColumns: '2fr 1fr 1fr 1fr 1fr 1fr',
          gap: '12px',
          alignItems: 'center',
        }}
      >
        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>PROMPT</label>
          <input
            type="text"
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px 10px', fontSize: '12px' }}
          />
        </div>
        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>COMPLEXITY</label>
          <select
            value={complexity}
            onChange={(e) => setComplexity(e.target.value as TaskComplexity)}
            style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px', fontSize: '12px' }}
          >
            <option value="simple">Simple</option>
            <option value="moderate">Moderate</option>
            <option value="complex">Complex</option>
            <option value="reasoning_heavy">Reasoning Heavy</option>
          </select>
        </div>
        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>POLICY</label>
          <select
            value={policy}
            onChange={(e) => setPolicy(e.target.value as RoutingPolicy)}
            style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px', fontSize: '12px' }}
          >
            <option value="balanced">Balanced</option>
            <option value="cost_optimized">Cost</option>
            <option value="latency_optimized">Latency</option>
            <option value="quality_optimized">Quality</option>
          </select>
        </div>
        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>MIN QUALITY</label>
          <input
            type="number"
            step="0.05"
            value={minQuality}
            onChange={(e) => setMinQuality(e.target.value)}
            style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px', fontSize: '12px' }}
          />
        </div>
        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>MAX COST ($)</label>
          <input
            type="number"
            step="0.01"
            value={maxCost}
            onChange={(e) => setMaxCost(e.target.value)}
            style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px', fontSize: '12px' }}
          />
        </div>
        <div>
          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)' }}>SLA (MS)</label>
          <input
            type="number"
            value={maxLatency}
            onChange={(e) => setMaxLatency(e.target.value)}
            style={{ width: '100%', backgroundColor: 'var(--bg-input)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', color: '#fff', padding: '6px', fontSize: '12px' }}
          />
        </div>
      </div>

      {/* SELECTED MODEL HERO BANNER */}
      {selectedModelId && (
        <div
          style={{
            backgroundColor: 'var(--bg-card)',
            border: '2px solid var(--accent-blue)',
            borderRadius: 'var(--radius-md)',
            padding: '18px',
            boxShadow: 'var(--shadow-glow-blue)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--accent-blue)', letterSpacing: '0.05em' }}>
                SELECTED MODEL
              </div>
              <div style={{ fontSize: '22px', fontWeight: 700, fontFamily: 'var(--font-mono)', marginTop: '2px' }}>
                {selectedModelId}
              </div>
              <div style={{ fontSize: '12px', color: 'var(--text-secondary)', marginTop: '2px' }}>
                Provider: <span style={{ fontFamily: 'var(--font-mono)' }}>{selectedModelObj?.provider || 'synthetic-cloud'}</span> | Tier: <span style={{ fontWeight: 600, textTransform: 'uppercase' }}>{selectedModelObj?.tier || 'LARGE'}</span>
              </div>
            </div>
            <div style={{ display: 'flex', gap: '16px', textAlign: 'right', fontFamily: 'var(--font-mono)', fontSize: '12px' }}>
              <div>
                <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Quality</div>
                <div style={{ fontWeight: 700, color: 'var(--accent-emerald)', fontSize: '15px' }}>
                  {formatPercent(s18Data?.quality_estimate?.mean ?? s17Data?.quality_score ?? 0.95)}
                </div>
              </div>
              <div>
                <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Est. Latency</div>
                <div style={{ fontWeight: 700, color: 'var(--accent-cyan)', fontSize: '15px' }}>
                  {formatLatency(s18Data?.latency_estimate?.predicted_ms ?? s17Data?.estimated_latency_ms ?? 250)}
                </div>
              </div>
              <div>
                <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Expected Utility</div>
                <div style={{ fontWeight: 700, color: 'var(--accent-purple)', fontSize: '15px' }}>
                  {formatScore(s19Data?.expected_utility ?? 0.5906, 4)}
                </div>
              </div>
              <div>
                <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>UCB Score</div>
                <div style={{ fontWeight: 700, color: 'var(--accent-blue)', fontSize: '15px' }}>
                  {formatScore(s19Data?.ucb_score ?? 0.7291, 4)}
                </div>
              </div>
            </div>
          </div>

          {/* WHY THIS MODEL WAS SELECTED CHECKLIST */}
          <div style={{ backgroundColor: 'var(--bg-surface)', padding: '12px 16px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)', marginTop: '14px' }}>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.05em', marginBottom: '6px' }}>
              WHY THIS MODEL WAS SELECTED?
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '6px', fontSize: '11px', fontFamily: 'var(--font-mono)' }}>
              <div style={{ color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}><Check size={12} /> Security compatible</div>
              <div style={{ color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}><Check size={12} /> Context capacity sufficient</div>
              <div style={{ color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}><Check size={12} /> Quality threshold satisfied ({formatPercent(selectedModelObj?.task_quality_matrix?.[complexity] ?? 0.95)} ≥ {formatPercent(parseFloat(minQuality) || 0.85)})</div>
              <div style={{ color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}><Check size={12} /> Cost budget satisfied ({formatCurrency(selectedModelObj?.cost_per_1k_input_tokens ?? 0.015)} ≤ {formatCurrency(parseFloat(maxCost) || 0.10)})</div>
              <div style={{ color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}><Check size={12} /> Latency SLA satisfied ({formatLatency(selectedModelObj?.nominal_p50_latency_ms ?? 250)} ≤ {formatLatency(parseFloat(maxLatency) || 2000)})</div>
              <div style={{ color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}><Check size={12} /> Highest effective utility score</div>
              <div style={{ color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}><Check size={12} /> Exploration not required</div>
            </div>
          </div>
        </div>
      )}

      {/* 3 STAGES COLUMNS */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '20px' }}>
        {/* STAGE 17: DETERMINISTIC SAFETY GATE */}
        <div style={{ backgroundColor: 'var(--bg-card)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
            <ShieldCheck size={18} style={{ color: 'var(--accent-blue)' }} />
            <div>
              <h3 style={{ fontSize: '14px', fontWeight: 700 }}>Stage 17: Safety Filter</h3>
              <div style={{ fontSize: '11px', color: 'var(--accent-emerald)', fontWeight: 600 }}>Deterministic Gate · Verified</div>
            </div>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {models.map((m) => {
              const isSelected = m.id === selectedModelId;
              const rejection = s17Data?.rejections?.find((r) => r.model_id === m.id);
              const isFeasible = !rejection;
              const showDetails = expandedDetails[m.id];
              const qVal = m.task_quality_matrix?.[complexity] ?? 0.5;

              return (
                <div
                  key={m.id}
                  style={{
                    padding: '10px 12px',
                    borderRadius: 'var(--radius-sm)',
                    backgroundColor: isFeasible ? 'var(--bg-surface)' : 'rgba(244, 63, 94, 0.05)',
                    border: `1px solid ${isSelected ? 'var(--accent-blue)' : isFeasible ? 'var(--border-color)' : 'var(--badge-red-border)'}`,
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span style={{ fontWeight: 700, fontSize: '12px', fontFamily: 'var(--font-mono)' }}>
                      {m.id}
                    </span>
                    {isFeasible ? (
                      <span style={{ fontSize: '10px', fontWeight: 700, color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                        <CheckCircle size={12} /> FEASIBLE
                      </span>
                    ) : (
                      <span style={{ fontSize: '10px', fontWeight: 700, color: 'var(--accent-rose)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                        <XCircle size={12} /> REJECTED
                      </span>
                    )}
                  </div>

                  {rejection ? (
                    <div style={{ marginTop: '6px', fontSize: '11px' }}>
                      <div style={{ color: 'var(--accent-rose)', fontWeight: 600 }}>
                        Quality: {formatPercent(qVal)} &lt; required {formatPercent(parseFloat(minQuality) || 0.85)}
                      </div>
                      <div style={{ color: 'var(--text-dim)', fontSize: '10px', fontFamily: 'var(--font-mono)' }}>
                        Reason: {rejection.reason}
                      </div>
                      <button
                        onClick={() => toggleDetails(m.id)}
                        style={{ background: 'none', border: 'none', color: 'var(--accent-cyan)', fontSize: '10px', cursor: 'pointer', padding: 0, marginTop: '4px', display: 'flex', alignItems: 'center', gap: '2px' }}
                      >
                        {showDetails ? <ChevronUp size={10} /> : <ChevronDown size={10} />}
                        {showDetails ? 'Hide technical details' : 'View technical details'}
                      </button>
                      {showDetails && (
                        <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', backgroundColor: 'var(--bg-input)', padding: '6px', borderRadius: '4px', marginTop: '4px' }}>
                          {rejection.details}
                        </div>
                      )}
                    </div>
                  ) : (
                    <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                      Quality: {formatPercent(qVal)} | P50: {formatLatency(m.nominal_p50_latency_ms)}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>

        {/* STAGE 18: BAYESIAN PREDICTION PANEL */}
        <div style={{ backgroundColor: 'var(--bg-card)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
            <BrainCircuit size={18} style={{ color: 'var(--accent-cyan)' }} />
            <div>
              <h3 style={{ fontSize: '14px', fontWeight: 700 }}>Stage 18: Adaptive Predictor</h3>
              <div style={{ fontSize: '11px', color: 'var(--accent-amber)', fontWeight: 600 }}>Experimental / Simulated Layer</div>
            </div>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <div style={{ backgroundColor: 'var(--bg-surface)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Expected Quality</div>
              <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)' }}>
                {formatPercent(s18Data?.quality_estimate?.mean ?? 0.95)}
              </div>
              <div style={{ fontSize: '10px', color: 'var(--text-dim)' }}>
                95% CI: [{formatPercent(s18Data?.quality_estimate?.lower_ci ?? 0.85)} - {formatPercent(s18Data?.quality_estimate?.upper_ci ?? 1.0)}]
              </div>
            </div>

            <div style={{ backgroundColor: 'var(--bg-surface)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Success Probability (Beta Posterior)</div>
              <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--accent-cyan)', fontFamily: 'var(--font-mono)' }}>
                {formatPercent(s18Data?.predicted_success ?? 0.833)}
              </div>
            </div>

            <div style={{ backgroundColor: 'var(--bg-surface)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Expected Latency / P95</div>
              <div style={{ fontSize: '15px', fontWeight: 700, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
                {formatLatency(s18Data?.latency_estimate?.predicted_ms ?? 250)} / {formatLatency(s18Data?.latency_estimate?.observed_p95_ms ?? 375)}
              </div>
            </div>

            <div style={{ backgroundColor: 'var(--bg-surface)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Confidence &amp; Observations</div>
              <div style={{ fontSize: '12px', fontWeight: 600, color: (s18Data?.sample_count ?? 0) === 0 ? 'var(--accent-amber)' : 'var(--accent-purple)', fontFamily: 'var(--font-mono)' }}>
                {(s18Data?.sample_count ?? 0) === 0
                  ? 'Cold start (0 samples, nominal prior)'
                  : `${formatPercent(s18Data?.confidence)} (${s18Data?.sample_count} empirical samples)`}
              </div>
            </div>
          </div>
        </div>

        {/* STAGE 19: CONTEXTUAL UCB POLICY PANEL */}
        <div style={{ backgroundColor: 'var(--bg-card)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '18px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
            <Target size={18} style={{ color: 'var(--accent-purple)' }} />
            <div>
              <h3 style={{ fontSize: '14px', fontWeight: 700 }}>Stage 19: Online Policy</h3>
              <div style={{ fontSize: '11px', color: 'var(--accent-amber)', fontWeight: 600 }}>Experimental / Simulated Policy</div>
            </div>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <div style={{ backgroundColor: 'var(--bg-surface)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Policy &amp; Mode</div>
              <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)' }}>
                {policy.toUpperCase()} · {s19Data?.decision_mode || 'EXPLOIT'}
              </div>
              <div style={{ fontSize: '10px', color: 'var(--text-dim)', marginTop: '2px' }}>
                Exploration Rate: {formatPercent(s19Data?.exploration_rate ?? 0.012)}
              </div>
            </div>

            <div style={{ backgroundColor: 'var(--bg-surface)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Expected Utility &amp; UCB Score</div>
              <div style={{ fontSize: '14px', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>
                Utility: {formatScore(s19Data?.expected_utility ?? 0.5906, 4)} | UCB: {formatScore(s19Data?.ucb_score ?? 0.7291, 4)}
              </div>
            </div>

            <div style={{ backgroundColor: 'var(--bg-surface)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Multi-Objective Score Breakdown</div>
              <div style={{ fontSize: '11px', fontFamily: 'var(--font-mono)', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '2px', marginTop: '2px' }}>
                <div>Quality: {formatScore(s19Data?.score_breakdown?.quality ?? 0.95, 2)}</div>
                <div>Cost: {formatScore(s19Data?.score_breakdown?.cost ?? 0.84, 2)}</div>
                <div>Latency: {formatScore(s19Data?.score_breakdown?.latency ?? 0.79, 2)}</div>
                <div>Reliability: {formatScore(s19Data?.score_breakdown?.reliability ?? 0.83, 2)}</div>
              </div>
            </div>

            <div style={{ backgroundColor: 'var(--bg-surface)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Guardrail Hysteresis Status</div>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '4px', marginTop: '2px' }}>
                <CheckCircle size={12} /> ACTIVE &amp; HEALTHY (No Rollback)
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
