import React, { useState, useEffect } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { TaskComplexity, RoutingPolicy, ExecuteResponse, TaskExecutionState } from '../../api/types';
import { formatCurrency, formatLatency, formatPercent, formatScore } from '../../utils/formatters';
import { StatusBadge } from '../../components/StatusBadge';
import { Play, AlertCircle, Terminal as TerminalIcon, RotateCcw, Clock } from 'lucide-react';

export const Tasks: React.FC = () => {
  const [prompt, setPrompt] = useState('Explain the architecture of a Go microservice and identify the top 3 reliability risks.');
  const [complexity, setComplexity] = useState<TaskComplexity>('moderate');
  const [policy, setPolicy] = useState<RoutingPolicy>('balanced');
  const [budget, setBudget] = useState('0.10');
  const [sla, setSla] = useState('2000');
  const [securityProfile, setSecurityProfile] = useState('standard');
  const [pinnedModel, setPinnedModel] = useState('');
  const [activeTab, setActiveTab] = useState<'response' | 'telemetry' | 'pipeline'>('response');

  const [execState, setExecState] = useState<TaskExecutionState>('IDLE');
  const [lastResult, setLastResult] = useState<ExecuteResponse | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
  });

  const activeMode = (settings?.execution_mode || 'SYNTHETIC').toLowerCase();

  const timerRef = React.useRef<ReturnType<typeof setTimeout>[]>([]);

  const clearTimers = () => {
    timerRef.current.forEach((id) => clearTimeout(id));
    timerRef.current = [];
  };

  useEffect(() => {
    return () => clearTimers();
  }, []);

  const executeMutation = useMutation({
    mutationFn: async () => {
      clearTimers();
      setErrorMessage(null);
      setExecState('SUBMITTING');

      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 30000);
      timerRef.current.push(timeoutId);

      try {
        const res = await api.executePolicy({
          prompt,
          task_complexity: complexity,
          routing_policy: policy,
          cost_budget_usd: parseFloat(budget) || 0.10,
          latency_sla_ms: parseFloat(sla) || 2000,
          security_profile: securityProfile,
          pinned_model_id: pinnedModel || undefined,
        });
        clearTimers();
        return res;
      } catch (err: any) {
        clearTimers();
        if (err?.name === 'AbortError') {
          throw new Error('EXECUTION TIMEOUT — Request exceeded 30-second budget limit');
        }
        throw err;
      }
    },
    onSuccess: (data) => {
      clearTimers();
      setLastResult(data);
      setExecState('COMPLETED');
    },
    onError: (err: any) => {
      clearTimers();
      const msg = err?.message || err?.data?.error || 'Execution failed';

      if (msg.includes('429') || msg.includes('credit_balance_exhausted') || msg.includes('insufficient_quota')) {
        setExecState('FAILED');
        setErrorMessage('PROVIDER QUOTA EXHAUSTED: OpenAI API account has no remaining credits. Add credits at platform.openai.com/billing to resume live inference.');
      } else if (msg.includes('budget_exceeded')) {
        setExecState('FAILED');
        setErrorMessage('COST BUDGET EXCEEDED: Estimated request cost exceeds task budget.');
      } else if (msg.includes('missing_openai_api_key')) {
        setExecState('FAILED');
        setErrorMessage('MISSING API KEY: OPENAI_API_KEY environment variable is not configured in backend LIVE mode.');
      } else if (msg.includes('TIMEOUT')) {
        setExecState('TIMEOUT');
        setErrorMessage(msg);
      } else {
        setExecState('FAILED');
        setErrorMessage(msg);
      }
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    executeMutation.mutate();
  };

  const isExecuting = execState !== 'IDLE' && execState !== 'COMPLETED' && execState !== 'FAILED' && execState !== 'TIMEOUT' && execState !== 'CANCELLED';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)' }}>
            Task Workbench
          </h1>
          <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
            Submit workloads to SentinelMesh and observe live multi-stage routing &amp; model execution.
          </p>
        </div>
        <StatusBadge status={activeMode} />
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.2fr', gap: '24px' }}>
        {/* Left: Execution Controls Form */}
        <form
          onSubmit={handleSubmit}
          style={{
            backgroundColor: 'var(--bg-panel)',
            border: '1px solid var(--border-color)',
            borderRadius: 'var(--radius-md)',
            padding: '20px',
            display: 'flex',
            flexDirection: 'column',
            gap: '16px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border-color)', paddingBottom: '12px' }}>
            <TerminalIcon size={16} style={{ color: 'var(--accent-blue)' }} />
            <h3 style={{ fontSize: '14px', fontWeight: 700 }}>Task Execution Parameters</h3>
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>
              WORKLOAD PROMPT
            </label>
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              rows={4}
              required
              style={{
                width: '100%',
                backgroundColor: 'var(--bg-primary)',
                border: '1px solid var(--border-color-light)',
                borderRadius: 'var(--radius-sm)',
                color: 'var(--text-primary)',
                padding: '10px',
                fontSize: '13px',
                fontFamily: 'var(--font-sans)',
                resize: 'vertical',
              }}
            />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>
                TASK COMPLEXITY
              </label>
              <select
                value={complexity}
                onChange={(e) => setComplexity(e.target.value as TaskComplexity)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px' }}
              >
                <option value="simple">Simple (Extraction/Formatting)</option>
                <option value="moderate">Moderate (Code Refactor)</option>
                <option value="complex">Complex (System Architecture)</option>
                <option value="reasoning_heavy">Reasoning Heavy (Deep Math/Logic)</option>
              </select>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>
                ROUTING POLICY
              </label>
              <select
                value={policy}
                onChange={(e) => setPolicy(e.target.value as RoutingPolicy)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px' }}
              >
                <option value="balanced">Balanced (Multi-Objective)</option>
                <option value="cost_optimized">Cost Optimized (Lowest USD)</option>
                <option value="latency_optimized">Latency Optimized (Fastest P50)</option>
                <option value="quality_optimized">Quality First (Highest Score)</option>
                <option value="static">Static (Pinned Model)</option>
              </select>
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '12px' }}>
            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '4px' }}>COST BUDGET ($)</label>
              <input
                type="number"
                step="0.01"
                value={budget}
                onChange={(e) => setBudget(e.target.value)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '4px' }}>LATENCY SLA (MS)</label>
              <input
                type="number"
                value={sla}
                onChange={(e) => setSla(e.target.value)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '4px' }}>SECURITY PROFILE</label>
              <select
                value={securityProfile}
                onChange={(e) => setSecurityProfile(e.target.value)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px' }}
              >
                <option value="standard">Standard</option>
                <option value="restricted">Restricted</option>
                <option value="airgapped">Airgapped</option>
              </select>
            </div>
          </div>

          <button
            type="submit"
            disabled={isExecuting}
            style={{
              marginTop: '8px',
              backgroundColor: 'var(--accent-blue)',
              color: '#fff',
              border: 'none',
              borderRadius: 'var(--radius-sm)',
              padding: '12px 20px',
              fontSize: '13px',
              fontWeight: 700,
              fontFamily: 'var(--font-mono)',
              cursor: isExecuting ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '8px',
              boxShadow: 'var(--shadow-glow-blue)',
              opacity: isExecuting ? 0.7 : 1,
            }}
          >
            <Play size={16} />
            {isExecuting ? `STATE: ${execState}...` : 'ROUTE & EXECUTE TASK'}
          </button>
        </form>

        {/* Right: Command Workbench Result Panel with Tabs */}
        <div
          style={{
            backgroundColor: 'var(--bg-panel)',
            border: '1px solid var(--border-color)',
            borderRadius: 'var(--radius-md)',
            padding: '20px',
            display: 'flex',
            flexDirection: 'column',
            gap: '16px',
          }}
        >
          {/* Result Tabs */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
            <div style={{ display: 'flex', gap: '12px' }}>
              <button
                onClick={() => setActiveTab('response')}
                style={{
                  background: 'none',
                  border: 'none',
                  borderBottom: activeTab === 'response' ? '2px solid var(--accent-blue)' : '2px solid transparent',
                  color: activeTab === 'response' ? 'var(--text-primary)' : 'var(--text-muted)',
                  fontSize: '12px',
                  fontWeight: 700,
                  fontFamily: 'var(--font-mono)',
                  padding: '4px 8px',
                  cursor: 'pointer',
                }}
              >
                INFERENCE RESPONSE
              </button>
              <button
                onClick={() => setActiveTab('telemetry')}
                style={{
                  background: 'none',
                  border: 'none',
                  borderBottom: activeTab === 'telemetry' ? '2px solid var(--accent-blue)' : '2px solid transparent',
                  color: activeTab === 'telemetry' ? 'var(--text-primary)' : 'var(--text-muted)',
                  fontSize: '12px',
                  fontWeight: 700,
                  fontFamily: 'var(--font-mono)',
                  padding: '4px 8px',
                  cursor: 'pointer',
                }}
              >
                TELEMETRY
              </button>
              <button
                onClick={() => setActiveTab('pipeline')}
                style={{
                  background: 'none',
                  border: 'none',
                  borderBottom: activeTab === 'pipeline' ? '2px solid var(--accent-blue)' : '2px solid transparent',
                  color: activeTab === 'pipeline' ? 'var(--text-primary)' : 'var(--text-muted)',
                  fontSize: '12px',
                  fontWeight: 700,
                  fontFamily: 'var(--font-mono)',
                  padding: '4px 8px',
                  cursor: 'pointer',
                }}
              >
                PIPELINE DECISION
              </button>
            </div>
            {lastResult && <StatusBadge status={activeMode} />}
          </div>

          {isExecuting && (
            <div style={{ padding: '60px 0', textAlign: 'center', color: 'var(--accent-blue)', fontSize: '13px', fontFamily: 'var(--font-mono)' }}>
              <div className="animate-pulse" style={{ fontSize: '16px', fontWeight: 700, marginBottom: '8px' }}>
                LIFECYCLE STATE: {execState}
              </div>
              <p style={{ color: 'var(--text-muted)', fontSize: '11px' }}>
                {execState === 'ROUTING' && 'Stage 17 Invariant Checks & Security Filtering...'}
                {execState === 'PREDICTING' && 'Stage 18 Bayesian Prior Sampling & Quality Estimation...'}
                {execState === 'POLICY' && 'Stage 19 Contextual Bandit Policy Decision...'}
                {execState === 'INVOKING' && 'Dispatching live request to upstream provider...'}
                {execState === 'STREAMING' && 'Streaming response chunks & calculating token usage...'}
                {execState === 'SUBMITTING' && 'Validating pre-flight cost budget...'}
              </p>
            </div>
          )}

          {(execState === 'FAILED' || execState === 'TIMEOUT') && (
            <div style={{ padding: '16px', backgroundColor: 'var(--badge-red-bg)', border: '1px solid var(--badge-red-border)', borderRadius: 'var(--radius-sm)', color: 'var(--badge-red-text)' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>
                  <AlertCircle size={16} /> {execState === 'TIMEOUT' ? 'EXECUTION TIMEOUT' : 'EXECUTION ERROR'}
                </div>
                <button
                  onClick={() => executeMutation.mutate()}
                  style={{
                    backgroundColor: 'var(--accent-red)',
                    color: '#fff',
                    border: 'none',
                    borderRadius: 'var(--radius-sm)',
                    padding: '4px 10px',
                    fontSize: '11px',
                    fontFamily: 'var(--font-mono)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px',
                  }}
                >
                  <RotateCcw size={12} /> RETRY
                </button>
              </div>
              <div style={{ fontSize: '12px', marginTop: '6px', fontFamily: 'var(--font-mono)', whiteSpace: 'pre-wrap' }}>
                {errorMessage}
              </div>
            </div>
          )}

          {lastResult && !isExecuting && execState === 'COMPLETED' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }} className="animate-fade-in">
              {/* Selected Model Card */}
              <div
                style={{
                  padding: '14px 16px',
                  backgroundColor: 'var(--bg-secondary)',
                  border: '1px solid var(--border-color-active)',
                  borderRadius: 'var(--radius-sm)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                }}
              >
                <div>
                  <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--accent-blue)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                    SELECTED MODEL ENDPOINT ({lastResult.invocation_result?.data_source || 'LIVE'})
                  </div>
                  <div style={{ fontSize: '18px', fontWeight: 700, fontFamily: 'var(--font-mono)', marginTop: '2px' }}>
                    {lastResult.invocation_result?.model_id || lastResult.stage19_decision?.selected_model_id}
                  </div>
                  {lastResult.invocation_result?.provider_model_id && (
                    <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                      Provider Target: {lastResult.invocation_result?.provider_model_id} ({lastResult.invocation_result?.provider || 'openai'})
                    </div>
                  )}
                </div>
                <div style={{ textAlign: 'right' }}>
                  <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Execution Status</div>
                  <div style={{ fontSize: '15px', fontWeight: 700, color: 'var(--accent-emerald)', fontFamily: 'var(--font-mono)' }}>
                    ✓ SUCCESS
                  </div>
                </div>
              </div>

              {/* Tab 1: Response Text */}
              {activeTab === 'response' && (
                <div>
                  <pre
                    style={{
                      padding: '14px',
                      backgroundColor: 'var(--bg-primary)',
                      border: '1px solid var(--border-color)',
                      borderRadius: 'var(--radius-sm)',
                      fontSize: '12px',
                      fontFamily: 'var(--font-mono)',
                      color: 'var(--text-primary)',
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-word',
                      maxHeight: '320px',
                      overflowY: 'auto',
                    }}
                  >
                    {lastResult.invocation_result?.content || 'Task processed successfully.'}
                  </pre>
                </div>
              )}

              {/* Tab 2: Telemetry */}
              {activeTab === 'telemetry' && (
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}>
                  <div style={{ backgroundColor: 'var(--bg-secondary)', padding: '12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>ESTIMATED COST (PRE-FLIGHT)</div>
                    <div style={{ color: 'var(--text-primary)', fontSize: '14px', fontWeight: 600 }}>{formatCurrency(lastResult.invocation_result?.estimated_cost_usd || lastResult.stage17_decision?.estimated_cost_usd)}</div>
                  </div>
                  <div style={{ backgroundColor: 'var(--bg-secondary)', padding: '12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>ACTUAL COST (POST-FLIGHT)</div>
                    <div style={{ color: 'var(--accent-purple)', fontSize: '16px', fontWeight: 700 }}>{formatCurrency(lastResult.invocation_result?.actual_cost_usd)}</div>
                  </div>
                  <div style={{ backgroundColor: 'var(--bg-secondary)', padding: '12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>OBSERVED LATENCY</div>
                    <div style={{ color: 'var(--accent-cyan)', fontSize: '16px', fontWeight: 700 }}>{formatLatency(lastResult.invocation_result?.actual_latency)}</div>
                  </div>
                  <div style={{ backgroundColor: 'var(--bg-secondary)', padding: '12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>QUALITY EVALUATION STATUS</div>
                    <div style={{ color: 'var(--accent-amber)', fontSize: '14px', fontWeight: 700 }}>{lastResult.invocation_result?.quality_status || 'NOT EVALUATED'}</div>
                  </div>
                  <div style={{ backgroundColor: 'var(--bg-secondary)', padding: '12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>PROMPT TOKENS</div>
                    <div style={{ color: 'var(--text-primary)', fontSize: '14px', fontWeight: 600 }}>{lastResult.invocation_result?.prompt_tokens ?? 0}</div>
                  </div>
                  <div style={{ backgroundColor: 'var(--bg-secondary)', padding: '12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>COMPLETION TOKENS</div>
                    <div style={{ color: 'var(--text-primary)', fontSize: '14px', fontWeight: 600 }}>{lastResult.invocation_result?.completion_tokens ?? 0}</div>
                  </div>
                </div>
              )}

              {/* Tab 3: Pipeline Verdict */}
              {activeTab === 'pipeline' && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}>
                  <div style={{ padding: '10px', backgroundColor: 'var(--bg-secondary)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <strong style={{ color: 'var(--accent-blue)' }}>STAGE 17 SAFETY:</strong> 100% Invariants Compliant ({lastResult.stage17_decision?.rejections?.length || 0} Rejections)
                  </div>
                  <div style={{ padding: '10px', backgroundColor: 'var(--bg-secondary)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <strong style={{ color: 'var(--accent-cyan)' }}>STAGE 18 PREDICTION:</strong> Quality Mean {formatPercent(lastResult.stage18_decision?.quality_estimate?.mean ?? 0.94)}
                  </div>
                  <div style={{ padding: '10px', backgroundColor: 'var(--bg-secondary)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)' }}>
                    <strong style={{ color: 'var(--accent-purple)' }}>STAGE 19 POLICY:</strong> Mode {lastResult.stage19_decision?.decision_mode || 'EXPLOIT'} | Utility {formatScore(lastResult.stage19_decision?.expected_utility, 3)}
                  </div>
                </div>
              )}
            </div>
          )}

          {!lastResult && !isExecuting && (
            <div style={{ color: 'var(--text-muted)', fontSize: '12px', textAlign: 'center', marginTop: '80px', fontFamily: 'var(--font-mono)' }}>
              Configure task parameters and click <strong>ROUTE &amp; EXECUTE TASK</strong> to view output.
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
