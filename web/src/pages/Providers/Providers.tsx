import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { formatLatency, formatCurrency } from '../../utils/formatters';
import { StatusBadge } from '../../components/StatusBadge';
import { Server, RefreshCw } from 'lucide-react';

export const Providers: React.FC = () => {
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
  });

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['modelsProviderList'],
    queryFn: () => api.getModels(),
    refetchInterval: 5000,
  });

  const activeMode = (settings?.execution_mode || 'SYNTHETIC').toLowerCase();
  const models = data?.models || [];

  const providersMap = models.reduce((acc, m) => {
    if (!acc[m.provider]) {
      acc[m.provider] = [];
    }
    acc[m.provider].push(m);
    return acc;
  }, {} as Record<string, typeof models>);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)' }}>
            Model Providers &amp; Endpoints
          </h1>
          <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
            Real-time status, circuit breakers, and performance telemetry for upstream LLM providers.
          </p>
        </div>
        <button
          onClick={() => refetch()}
          style={{
            backgroundColor: 'var(--bg-secondary)',
            border: '1px solid var(--border-color)',
            color: 'var(--text-primary)',
            padding: '8px 14px',
            borderRadius: 'var(--radius-sm)',
            cursor: 'pointer',
            fontSize: '12px',
            fontWeight: 600,
            fontFamily: 'var(--font-mono)',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
          }}
        >
          <RefreshCw size={12} /> REFRESH HEALTH
        </button>
      </div>

      {isLoading ? (
        <div style={{ padding: '40px 0', textAlign: 'center', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
          Loading provider catalog...
        </div>
      ) : models.length === 0 ? (
        <div style={{ padding: '40px', backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', textAlign: 'center', color: 'var(--text-muted)' }}>
          No providers registered in SentinelMesh backend.
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(360px, 1fr))', gap: '20px' }}>
          {Object.entries(providersMap).map(([providerName, providerModels]) => {
            const isHealthy = providerModels.every((m) => m.health_status === 'HEALTHY');
            const isSyntheticProvider = providerName.toLowerCase().includes('synthetic') || activeMode === 'synthetic';
            const modeStatus = isSyntheticProvider ? 'synthetic' : 'live';

            return (
              <div
                key={providerName}
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
                {/* Header */}
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '12px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <Server size={18} style={{ color: 'var(--accent-blue)' }} />
                    <div>
                      <h3 style={{ fontSize: '15px', fontWeight: 700, textTransform: 'uppercase', fontFamily: 'var(--font-mono)' }}>
                        {providerName} {isSyntheticProvider ? '— SIMULATED' : '— LIVE'}
                      </h3>
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                        {providerModels.length} Registered Model{providerModels.length > 1 ? 's' : ''} ({isSyntheticProvider ? 'SyntheticModelProvider' : 'Live OpenAI Adapter'})
                      </div>
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <StatusBadge status={modeStatus} size="sm" />
                    <StatusBadge status={isHealthy ? 'healthy' : 'degraded'} size="sm" />
                  </div>
                </div>

                {/* Models List */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                  {providerModels.map((m) => (
                    <div
                      key={m.id}
                      style={{
                        backgroundColor: 'var(--bg-secondary)',
                        border: '1px solid var(--border-color)',
                        borderRadius: 'var(--radius-sm)',
                        padding: '12px',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: '8px',
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <div style={{ fontWeight: 700, fontSize: '13px', fontFamily: 'var(--font-mono)', color: 'var(--text-primary)' }}>
                          {m.name} <span style={{ color: 'var(--text-dim)', fontWeight: 400 }}>({m.id})</span>
                        </div>
                        <span
                          style={{
                            fontSize: '10px',
                            fontWeight: 700,
                            fontFamily: 'var(--font-mono)',
                            padding: '2px 6px',
                            borderRadius: 'var(--radius-sm)',
                            backgroundColor: 'var(--bg-primary)',
                            color: m.tier === 'large' ? 'var(--accent-purple)' : m.tier === 'medium' ? 'var(--accent-blue)' : 'var(--accent-cyan)',
                            border: '1px solid var(--border-color)',
                          }}
                        >
                          TIER: {m.tier.toUpperCase()}
                        </span>
                      </div>

                      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '8px', fontSize: '11px', fontFamily: 'var(--font-mono)' }}>
                        <div>
                          <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>P50 LATENCY</div>
                          <div style={{ color: 'var(--accent-cyan)', fontWeight: 600 }}>{formatLatency(m.nominal_p50_latency_ms)}</div>
                        </div>
                        <div>
                          <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>P95 TAIL</div>
                          <div style={{ color: 'var(--accent-amber)', fontWeight: 600 }}>{formatLatency(m.nominal_p95_latency_ms)}</div>
                        </div>
                        <div>
                          <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>INPUT COST</div>
                          <div style={{ color: 'var(--text-secondary)', fontWeight: 600 }}>{formatCurrency(m.cost_per_1k_input_tokens)}/1k</div>
                        </div>
                        <div>
                          <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>CIRCUIT</div>
                          <div style={{ color: m.health_status === 'HEALTHY' ? 'var(--accent-emerald)' : 'var(--accent-rose)', fontWeight: 700 }}>
                            {m.health_status === 'HEALTHY' ? 'CLOSED' : 'OPEN'}
                          </div>
                        </div>
                      </div>

                      <div style={{ fontSize: '10px', color: 'var(--text-dim)', borderTop: '1px solid var(--border-color)', paddingTop: '6px', marginTop: '2px', fontFamily: 'var(--font-mono)' }}>
                        Context: {m.context_window.toLocaleString()} tokens | Security: {m.security_classes.join(', ')}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
