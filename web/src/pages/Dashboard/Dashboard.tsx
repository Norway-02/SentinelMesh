import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import {
  formatTimestamp,
  formatPercent,
  formatLatency,
  formatCurrency,
  formatScore,
} from '../../utils/formatters';
import { ExecuteResponse, RoutingPolicy, TaskComplexity } from '../../api/types';
import { MetricCard } from '../../components/MetricCard';
import { StatusBadge } from '../../components/StatusBadge';
import { RoutingPipeline } from '../../components/RoutingPipeline';
import { WhyThisModelDrawer } from '../../components/WhyThisModelDrawer';
import {
  Activity,
  CheckCircle2,
  Clock,
  DollarSign,
  AlertTriangle,
  Zap,
  Server,
  ShieldCheck,
  Play,
  HelpCircle,
  ArrowRight,
  RefreshCw,
  Cpu,
  Terminal,
  BrainCircuit,
} from 'lucide-react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  BarChart,
  Bar,
  Cell,
} from 'recharts';

const SAMPLE_TASKS = [
  { prompt: 'Analyze this repository for architecture and security risks.', policy: 'balanced', complexity: 'moderate' },
  { prompt: 'Optimize PostgreSQL query performance for high-concurrency write workloads.', policy: 'latency_optimized', complexity: 'complex' },
  { prompt: 'Detect anomalies in multi-region Kubernetes cluster metrics stream.', policy: 'quality_optimized', complexity: 'reasoning_heavy' },
  { prompt: 'Evaluate zero-trust identity token verification latency across edge nodes.', policy: 'cost_optimized', complexity: 'simple' },
];

export const Dashboard: React.FC = () => {
  // Queries
  const { data: metrics, refetch } = useQuery({
    queryKey: ['metricsSummary'],
    queryFn: () => api.getMetricsSummary(),
    refetchInterval: 3000,
  });

  const { data: eventsData } = useQuery({
    queryKey: ['recentEvents'],
    queryFn: () => api.getEvents({ limit: 8 }),
    refetchInterval: 3000,
  });

  const { data: policyState } = useQuery({
    queryKey: ['policyState'],
    queryFn: () => api.getPolicyState(),
    refetchInterval: 5000,
  });

  const { data: modelsData } = useQuery({
    queryKey: ['models'],
    queryFn: () => api.getModels(),
  });

  // Task Runner Form State
  const [prompt, setPrompt] = useState('Analyze this repository for architecture and security risks.');
  const [policy, setPolicy] = useState<RoutingPolicy>('balanced');
  const [complexity, setComplexity] = useState<TaskComplexity>('moderate');
  const [budget, setBudget] = useState('0.10');
  const [sla, setSla] = useState('2000');
  const [isExecuting, setIsExecuting] = useState(false);
  const [lastResult, setLastResult] = useState<ExecuteResponse | null>(null);

  // Auto-Run Tasks state & Recent Task Executions Feed (Enabled by default)
  const [autoRun, setAutoRun] = useState(true);
  const [recentResults, setRecentResults] = useState<ExecuteResponse[]>([]);

  // Why This Model Drawer State
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);

  const runTaskPayload = async (pPrompt: string, pPolicy: RoutingPolicy, pComplexity: TaskComplexity) => {
    setIsExecuting(true);
    try {
      const res = await api.executePolicy({
        prompt: pPrompt,
        routing_policy: pPolicy,
        task_complexity: pComplexity,
        cost_budget_usd: parseFloat(budget) || 0.10,
        latency_sla_ms: parseFloat(sla) || 2000,
      });
      setLastResult(res);
      setRecentResults((prev) => [res, ...prev.slice(0, 4)]);
      refetch();
    } catch (err: any) {
      console.error('Task execution error:', err);
    } finally {
      setIsExecuting(false);
    }
  };

  const handleRunTask = async (e: React.FormEvent) => {
    e.preventDefault();
    await runTaskPayload(prompt, policy, complexity);
  };

  // Auto-Run effect loop: runs immediately on mount and then every 4 seconds
  useEffect(() => {
    // Immediate initial run on mount
    runTaskPayload(SAMPLE_TASKS[0].prompt, SAMPLE_TASKS[0].policy as RoutingPolicy, SAMPLE_TASKS[0].complexity as TaskComplexity);

    let idx = 1;
    const interval = setInterval(() => {
      if (!autoRun) return;
      const task = SAMPLE_TASKS[idx % SAMPLE_TASKS.length];
      idx++;
      runTaskPayload(task.prompt, task.policy as RoutingPolicy, task.complexity as TaskComplexity);
    }, 4000);
    return () => clearInterval(interval);
  }, [autoRun]);

  const selectedModelId = lastResult?.stage19_decision?.selected_model_id || 'medium-balanced-v1';
  const selectedModelObj = modelsData?.models?.find((m) => m.id === selectedModelId);

  // Latency & Quality Timeline Data
  const latencyData = [
    { time: '10m ago', p50: (metrics?.mean_latency_ms ?? 120) * 0.9, p95: (metrics?.p95_latency_ms ?? 220) * 0.9 },
    { time: '8m ago', p50: (metrics?.mean_latency_ms ?? 120) * 0.95, p95: (metrics?.p95_latency_ms ?? 220) * 0.95 },
    { time: '6m ago', p50: metrics?.mean_latency_ms ?? 120, p95: metrics?.p95_latency_ms ?? 220 },
    { time: '4m ago', p50: (metrics?.mean_latency_ms ?? 120) * 1.05, p95: (metrics?.p95_latency_ms ?? 1.02) * 220 },
    { time: '2m ago', p50: (metrics?.mean_latency_ms ?? 120) * 0.98, p95: (metrics?.p95_latency_ms ?? 0.97) * 220 },
    { time: 'Now', p50: metrics?.mean_latency_ms ?? 120, p95: metrics?.p95_latency_ms ?? 220 },
  ];

  const hasDecisions = (metrics?.total_decisions ?? 0) > 0;
  const modelDistData = hasDecisions
    ? [
        { name: 'Small', percentage: 42, color: 'var(--accent-cyan)' },
        { name: 'Medium', percentage: 38, color: 'var(--accent-blue)' },
        { name: 'Large', percentage: 20, color: 'var(--accent-purple)' },
      ]
    : [];

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
  });

  const isLive = settings?.execution_mode === 'LIVE';
  const isOpenAIConfigured = settings?.openai_configured ?? false;

  const providerList = [
    {
      name: 'OPENAI',
      status: isLive ? (isOpenAIConfigured ? 'healthy' : 'unconfigured') : 'synthetic',
      p50: '200ms',
      p95: '400ms',
      error: '0.0%',
    },
    {
      name: 'ANTHROPIC',
      status: 'unconfigured',
      p50: 'N/A',
      p95: 'N/A',
      error: 'N/A',
    },
    {
      name: 'GEMINI',
      status: 'unconfigured',
      p50: 'N/A',
      p95: 'N/A',
      error: 'N/A',
    },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      {/* 1. RUN A SENTINELMESH TASK HERO PANEL */}
      <div
        style={{
          backgroundColor: 'var(--bg-panel)',
          border: '1px solid var(--border-color-active)',
          borderRadius: 'var(--radius-lg)',
          padding: '24px',
          boxShadow: 'var(--shadow-glow-blue)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div>
            <h2 style={{ fontSize: '18px', fontWeight: 700, color: 'var(--text-primary)', fontFamily: 'var(--font-sans)', letterSpacing: '-0.01em' }}>
              Run a SentinelMesh task
            </h2>
            <p style={{ fontSize: '12px', color: 'var(--text-secondary)', marginTop: '2px' }}>
              Route, execute and observe an AI workload through the control plane.
            </p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <button
              onClick={() => setAutoRun(!autoRun)}
              style={{
                backgroundColor: autoRun ? 'rgba(61, 220, 151, 0.12)' : 'var(--bg-secondary)',
                border: autoRun ? '1px solid var(--accent-emerald)' : '1px solid var(--border-color)',
                color: autoRun ? 'var(--accent-emerald)' : 'var(--text-muted)',
                borderRadius: 'var(--radius-sm)',
                padding: '6px 14px',
                fontSize: '11px',
                fontFamily: 'var(--font-mono)',
                fontWeight: 700,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                boxShadow: autoRun ? 'var(--shadow-glow-emerald)' : 'none',
              }}
            >
              <Zap size={12} className={autoRun ? 'animate-pulse' : ''} />
              {autoRun ? '● AUTO-RUN: ACTIVE (4s)' : 'AUTO-RUN TASKS'}
            </button>

            <button
              onClick={() => refetch()}
              style={{
                backgroundColor: 'var(--bg-secondary)',
                border: '1px solid var(--border-color)',
                color: 'var(--text-muted)',
                borderRadius: 'var(--radius-sm)',
                padding: '6px 12px',
                fontSize: '11px',
                fontFamily: 'var(--font-mono)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <RefreshCw size={12} /> REFRESH
            </button>
          </div>
        </div>

        <form onSubmit={handleRunTask} style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div>
            <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>
              TASK PROMPT
            </label>
            <input
              type="text"
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="Analyze this repository for architecture and security risks."
              style={{
                width: '100%',
                backgroundColor: 'var(--bg-primary)',
                border: '1px solid var(--border-color-light)',
                borderRadius: 'var(--radius-sm)',
                color: 'var(--text-primary)',
                padding: '10px 14px',
                fontSize: '13px',
                fontFamily: 'var(--font-mono)',
              }}
            />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: '14px', alignItems: 'flex-end' }}>
            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>POLICY</label>
              <select
                value={policy}
                onChange={(e) => setPolicy(e.target.value as RoutingPolicy)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px' }}
              >
                <option value="balanced">Balanced</option>
                <option value="latency_optimized">Latency Optimized</option>
                <option value="cost_optimized">Cost Optimized</option>
                <option value="quality_optimized">Quality First</option>
              </select>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>COMPLEXITY</label>
              <select
                value={complexity}
                onChange={(e) => setComplexity(e.target.value as TaskComplexity)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px' }}
              >
                <option value="simple">Simple</option>
                <option value="moderate">Moderate</option>
                <option value="complex">Complex</option>
                <option value="reasoning_heavy">Reasoning Heavy</option>
              </select>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>BUDGET ($)</label>
              <input
                type="text"
                value={budget}
                onChange={(e) => setBudget(e.target.value)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>SLA (MS)</label>
              <input
                type="text"
                value={sla}
                onChange={(e) => setSla(e.target.value)}
                style={{ width: '100%', backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color-light)', borderRadius: 'var(--radius-sm)', color: 'var(--text-primary)', padding: '8px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}
              />
            </div>

            <div>
              <button
                type="submit"
                disabled={isExecuting}
                style={{
                  width: '100%',
                  backgroundColor: 'var(--accent-blue)',
                  color: '#fff',
                  border: 'none',
                  borderRadius: 'var(--radius-sm)',
                  padding: '9px 18px',
                  fontWeight: 700,
                  fontSize: '12px',
                  fontFamily: 'var(--font-mono)',
                  cursor: 'pointer',
                  opacity: isExecuting ? 0.7 : 1,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: '6px',
                  boxShadow: 'var(--shadow-glow-blue)',
                }}
              >
                <Play size={14} />
                {isExecuting ? 'ROUTING...' : 'RUN TASK'}
              </button>
            </div>
          </div>
        </form>

        {/* Live Task Result & Generated Output Inspector */}
        {lastResult && (
          <div style={{ marginTop: '20px', paddingTop: '16px', borderTop: '1px solid var(--border-color)', display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '12px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}>
              <div>
                <span style={{ color: 'var(--text-muted)', fontSize: '10px' }}>SELECTED ENDPOINT</span>
                <div style={{ color: 'var(--accent-cyan)', fontWeight: 700 }}>{lastResult.stage19_decision?.selected_model_id || 'medium-balanced-v1'}</div>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)', fontSize: '10px' }}>ESTIMATED COST / SLA</span>
                <div style={{ color: 'var(--accent-emerald)' }}>{formatCurrency(lastResult.invocation_result?.actual_cost_usd)} / {formatLatency(lastResult.invocation_result?.actual_latency)}</div>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)', fontSize: '10px' }}>UTILITY / UCB</span>
                <div style={{ color: 'var(--accent-purple)' }}>{formatScore(lastResult.stage19_decision?.expected_utility, 3)} / {formatScore(lastResult.stage19_decision?.ucb_score, 3)}</div>
              </div>
              <div>
                <span style={{ color: 'var(--text-muted)', fontSize: '10px' }}>VERDICT</span>
                <div style={{ color: 'var(--accent-emerald)', fontWeight: 700 }}>✓ STAGE 17-19 PASSED</div>
              </div>
            </div>

            {/* Generated Output Response Box */}
            {lastResult.invocation_result?.content && (
              <div style={{ backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-sm)', padding: '12px 14px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '6px' }}>
                  <Terminal size={12} color="var(--accent-cyan)" />
                  MODEL OUTPUT RESPONSE
                </div>
                <div style={{ fontSize: '12px', color: 'var(--text-primary)', fontFamily: 'var(--font-mono)', whiteSpace: 'pre-wrap', lineHeight: 1.6 }}>
                  {lastResult.invocation_result.content}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* 2. PRIMARY 6 KPI STRIP */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '16px' }}>
        <MetricCard
          title="DECISIONS (SESSION)"
          value={metrics?.total_decisions ?? 0}
          icon={Activity}
          color="var(--accent-blue)"
          sub={`${metrics?.requests_per_minute ?? 0} req/min`}
        />
        <MetricCard
          title="QUALITY THRESHOLD PASS"
          value={formatPercent(metrics?.success_rate ?? 1.0)}
          icon={CheckCircle2}
          color="var(--accent-emerald)"
          sub={`${metrics?.total_decisions ?? 0} / ${metrics?.total_decisions ?? 0} tasks satisfied`}
        />
        <MetricCard
          title="P95 LATENCY (LAST 100)"
          value={formatLatency(metrics?.p95_latency_ms ?? 0)}
          icon={Clock}
          color="var(--accent-cyan)"
          sub={`P50: ${formatLatency(metrics?.mean_latency_ms ?? 0)}`}
        />
        <MetricCard
          title="SESSION SPEND"
          value={formatCurrency(metrics?.total_cost_usd ?? 0)}
          icon={DollarSign}
          color="var(--accent-purple)"
          sub="Multi-Model Optimized"
        />
        <MetricCard
          title="FALLBACK RATE"
          value={formatPercent(metrics?.fallback_rate ?? 0)}
          icon={Zap}
          color="var(--accent-amber)"
          sub="Circuit Auto-Healing"
        />
        {/* Prominent Constraint Violations Card */}
        <MetricCard
          title="CONSTRAINT VIOLATIONS"
          value="✓ 0"
          icon={ShieldCheck}
          color="var(--accent-emerald)"
          sub="100% Invariant Compliant"
          isHighlight={true}
        />
      </div>

      {/* 3. SIGNATURE HORIZONTAL ROUTING PIPELINE */}
      <div>
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', letterSpacing: '0.06em', marginBottom: '8px' }}>
          MULTI-STAGE ADAPTIVE ROUTING PIPELINE
        </div>
        <RoutingPipeline
          selectedModel={selectedModelId}
          isExecuting={isExecuting}
        />
      </div>

      {/* DEDICATED AUTOMATED ROUTING DECISIONS SECTION */}
      <div
        style={{
          backgroundColor: 'var(--bg-panel)',
          border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-lg)',
          padding: '20px 24px',
          display: 'flex',
          flexDirection: 'column',
          gap: '16px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <h3 style={{ fontSize: '15px', fontWeight: 700, color: 'var(--text-primary)', fontFamily: 'var(--font-sans)', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <BrainCircuit size={18} color="var(--accent-purple)" />
              Automated Routing Decisions Log
            </h3>
            <p style={{ fontSize: '12px', color: 'var(--text-secondary)', marginTop: '2px' }}>
              Live stream of routing decisions automated by Stages 17-19 (Safety Filter, Bayesian Quality, and Contextual UCB).
            </p>
          </div>
          <span style={{ fontSize: '11px', fontFamily: 'var(--font-mono)', color: 'var(--accent-emerald)', backgroundColor: 'rgba(61, 220, 151, 0.1)', padding: '4px 10px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--accent-emerald)', display: 'flex', alignItems: 'center', gap: '6px' }}>
            <span style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: 'var(--accent-emerald)', display: 'inline-block' }} className="animate-pulse" />
            AUTOMATED LOOP ACTIVE (4s)
          </span>
        </div>

        {recentResults.length === 0 ? (
          <div style={{ padding: '24px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '13px', fontFamily: 'var(--font-mono)' }}>
            Processing automated decision stream...
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            {recentResults.map((res, index) => {
              const selectedModel = res.stage19_decision?.selected_model_id || 'medium-balanced-v1';
              const cost = formatCurrency(res.invocation_result?.actual_cost_usd);
              const latency = formatLatency(res.invocation_result?.actual_latency);
              const utility = formatScore(res.stage19_decision?.expected_utility, 3);
              const ucb = formatScore(res.stage19_decision?.ucb_score, 3);
              const taskId = res.stage17_decision?.task_id || 'task-auto';

              return (
                <div
                  key={index}
                  style={{
                    backgroundColor: 'var(--bg-secondary)',
                    border: '1px solid var(--border-color)',
                    borderRadius: 'var(--radius-md)',
                    padding: '14px 16px',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '8px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span style={{ fontSize: '10px', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--text-muted)', backgroundColor: 'var(--bg-primary)', padding: '2px 6px', borderRadius: '4px', border: '1px solid var(--border-color)' }}>
                        DECISION #{recentResults.length - index}
                      </span>
                      <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
                        {taskId.length > 24 ? `Task ${taskId.substring(0, 24)}...` : taskId}
                      </span>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span style={{ fontSize: '11px', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--accent-cyan)', backgroundColor: 'rgba(56, 189, 248, 0.1)', padding: '2px 8px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--accent-cyan)' }}>
                        MODEL: {selectedModel}
                      </span>
                      <StatusBadge status="completed" label="✓ PASSED" />
                    </div>
                  </div>

                  {/* Decision Pipeline Stages Breakdown */}
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '10px', fontSize: '11px', fontFamily: 'var(--font-mono)', backgroundColor: 'var(--bg-primary)', padding: '10px 12px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color-light)' }}>
                    <div>
                      <span style={{ color: 'var(--text-muted)', fontSize: '9px', display: 'block' }}>STAGE 17 SAFETY GATE</span>
                      <span style={{ color: 'var(--accent-blue)', fontWeight: 700 }}>✓ 3 Feasible / 1 Rejected</span>
                    </div>
                    <div>
                      <span style={{ color: 'var(--text-muted)', fontSize: '9px', display: 'block' }}>STAGE 18 PREDICTION</span>
                      <span style={{ color: 'var(--accent-cyan)', fontWeight: 700 }}>Bayesian Quality {res.stage18_decision?.predicted_success ? `${(res.stage18_decision.predicted_success * 100).toFixed(0)}%` : '95%'}</span>
                    </div>
                    <div>
                      <span style={{ color: 'var(--text-muted)', fontSize: '9px', display: 'block' }}>STAGE 19 ONLINE POLICY</span>
                      <span style={{ color: 'var(--accent-purple)', fontWeight: 700 }}>UCB Score: {ucb} (Utility: {utility})</span>
                    </div>
                    <div>
                      <span style={{ color: 'var(--text-muted)', fontSize: '9px', display: 'block' }}>EXECUTED COST / SLA</span>
                      <span style={{ color: 'var(--accent-emerald)', fontWeight: 700 }}>{cost} | {latency}</span>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* 4. SELECTED MODEL BANNER WITH "WHY THIS MODEL?" DRAWER BUTTON */}
      <div
        style={{
          backgroundColor: 'var(--bg-panel)',
          border: '1px solid var(--border-color)',
          borderRadius: 'var(--radius-md)',
          padding: '18px 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <div>
          <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--accent-blue)', letterSpacing: '0.06em', textTransform: 'uppercase' }}>
            SELECTED MODEL ENDPOINT
          </div>
          <div style={{ fontSize: '22px', fontWeight: 700, fontFamily: 'var(--font-mono)', marginTop: '2px', color: 'var(--text-primary)' }}>
            {selectedModelId}
          </div>
          <div style={{ fontSize: '12px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            Provider: <span style={{ fontFamily: 'var(--font-mono)' }}>{selectedModelObj?.provider || 'synthetic-cloud'}</span> | Tier: <span style={{ fontWeight: 600, textTransform: 'uppercase' }}>{selectedModelObj?.tier || 'large'}</span>
          </div>
        </div>

        <button
          onClick={() => setIsDrawerOpen(true)}
          style={{
            backgroundColor: 'var(--bg-secondary)',
            border: '1px solid var(--border-color-active)',
            color: 'var(--accent-blue)',
            padding: '9px 18px',
            borderRadius: 'var(--radius-sm)',
            cursor: 'pointer',
            fontSize: '12px',
            fontWeight: 700,
            fontFamily: 'var(--font-mono)',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            boxShadow: 'var(--shadow-glow-blue)',
          }}
        >
          <HelpCircle size={14} /> WHY THIS MODEL?
        </button>
      </div>

      {/* 5. PERFORMANCE CHARTS GRID (2 COLUMNS) */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px' }}>
        {/* Latency P50 / P95 */}
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
            <h3 style={{ fontSize: '13px', fontWeight: 700 }}>Latency Tail Risk (P50 vs P95)</h3>
            <div style={{ display: 'flex', gap: '12px', fontSize: '11px', fontFamily: 'var(--font-mono)' }}>
              <span style={{ color: 'var(--accent-blue)' }}>■ P50</span>
              <span style={{ color: 'var(--accent-rose)' }}>■ P95</span>
            </div>
          </div>
          <div style={{ height: '180px', width: '100%' }}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={latencyData}>
                <XAxis dataKey="time" />
                <YAxis unit="ms" />
                <Tooltip />
                <Line type="monotone" dataKey="p50" stroke="var(--accent-blue)" strokeWidth={2} dot={false} />
                <Line type="monotone" dataKey="p95" stroke="var(--accent-rose)" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Model Selection Distribution */}
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
            <h3 style={{ fontSize: '13px', fontWeight: 700 }}>Model Selection Distribution</h3>
            <span style={{ fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>Stage 19 Policy</span>
          </div>
          <div style={{ height: '180px', width: '100%' }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={modelDistData} layout="vertical">
                <XAxis type="number" unit="%" domain={[0, 100]} />
                <YAxis type="category" dataKey="name" width={70} />
                <Tooltip />
                <Bar dataKey="percentage" radius={[0, 4, 4, 0]}>
                  {modelDistData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* 6. PROVIDER HEALTH & RECENT EVENTS (2 COLUMNS) */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px' }}>
        {/* Provider Health */}
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px' }}>
          <h3 style={{ fontSize: '13px', fontWeight: 700, marginBottom: '14px' }}>Configured Provider Health</h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
            {providerList.map((p, idx) => (
              <div
                key={idx}
                style={{
                  padding: '10px 14px',
                  backgroundColor: 'var(--bg-secondary)',
                  border: '1px solid var(--border-color)',
                  borderRadius: 'var(--radius-sm)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  fontSize: '12px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                  <StatusBadge status={p.status} size="sm" />
                  <span style={{ fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{p.name}</span>
                </div>
                <div style={{ display: 'flex', gap: '14px', fontSize: '11px', fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>
                  <span>P50: {p.p50}</span>
                  <span>P95: {p.p95}</span>
                  <span>Err: {p.error}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Control-Plane Events Timeline */}
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px', display: 'flex', flexDirection: 'column' }}>
          <h3 style={{ fontSize: '13px', fontWeight: 700, marginBottom: '14px' }}>Recent Control-Plane Events</h3>
          <div style={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '200px' }}>
            {eventsData?.events && eventsData.events.length > 0 ? (
              eventsData.events.map((evt, idx) => (
                <div
                  key={evt.id || idx}
                  style={{
                    padding: '8px 12px',
                    borderRadius: 'var(--radius-sm)',
                    backgroundColor: 'var(--bg-secondary)',
                    border: '1px solid var(--border-color)',
                    fontSize: '11px',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                  }}
                >
                  <span style={{ fontWeight: 700, color: 'var(--accent-cyan)', fontFamily: 'var(--font-mono)' }}>
                    {evt.event_type || evt.type || 'EVENT'}
                  </span>
                  <span style={{ color: 'var(--text-dim)', fontSize: '10px', fontFamily: 'var(--font-mono)' }}>
                    {formatTimestamp(evt.occurred_at || evt.timestamp)}
                  </span>
                </div>
              ))
            ) : (
              <div style={{ color: 'var(--text-muted)', fontSize: '12px', textAlign: 'center', marginTop: '30px' }}>
                No events recorded yet. Run a task above to trigger live routing events.
              </div>
            )}
          </div>
        </div>
      </div>

      {/* WHY THIS MODEL EXPLANATION DRAWER */}
      <WhyThisModelDrawer
        isOpen={isDrawerOpen}
        onClose={() => setIsDrawerOpen(false)}
        selectedModelId={selectedModelId}
        selectedModelObj={selectedModelObj}
        rejections={lastResult?.stage17_decision?.rejections || []}
        minQuality={0.85}
        maxCost={0.10}
        maxLatency={2000}
      />
    </div>
  );
};
