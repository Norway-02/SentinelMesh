import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { StatusBadge } from '../../components/StatusBadge';
import { Bot, RefreshCw } from 'lucide-react';

export const Agents: React.FC = () => {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['agentsList'],
    queryFn: () => api.getAgents(),
    refetchInterval: 5000,
  });

  const agents = data?.agents || [];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)' }}>
            Agent Registry
          </h1>
          <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
            Registered AI workload definitions, resource allocation limits, and execution state.
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
          <RefreshCw size={12} /> REFRESH AGENTS
        </button>
      </div>

      {isLoading ? (
        <div style={{ padding: '40px 0', textAlign: 'center', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
          Loading registered agents...
        </div>
      ) : agents.length === 0 ? (
        <div style={{ padding: '40px', backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', textAlign: 'center', color: 'var(--text-muted)' }}>
          No agents registered.
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '20px' }}>
          {agents.map((agent: any) => {
            const id = agent.id || agent.ID;
            const name = agent.name || agent.Name;
            const version = agent.version || agent.Version;
            const tenantId = agent.tenant_id || agent.TenantID || 'default';
            const priority = agent.priority || agent.Priority || 'normal';
            const state = agent.state || agent.State || 'ACTIVE';
            const image = agent.image || agent.Image;
            const cpu = agent.resources?.cpu || agent.Resources?.cpu || agent.Resources?.CPU || '500m';
            const memory = agent.resources?.memory || agent.Resources?.memory || agent.Resources?.Memory || '512Mi';
            const gpu = agent.resources?.gpu ?? agent.Resources?.gpu ?? agent.Resources?.GPU ?? 0;

            return (
              <div
                key={id}
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
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '12px' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                    <Bot size={18} style={{ color: 'var(--accent-blue)' }} />
                    <div>
                      <h3 style={{ fontSize: '14px', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>{name}</h3>
                      <div style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>
                        v{version} | {id}
                      </div>
                    </div>
                  </div>
                  <StatusBadge status={state} size="sm" />
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px', fontSize: '11px', fontFamily: 'var(--font-mono)' }}>
                  <div>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>TENANT ID</div>
                    <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{tenantId}</div>
                  </div>
                  <div>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>PRIORITY</div>
                    <div style={{ fontWeight: 600, textTransform: 'uppercase', color: 'var(--accent-cyan)' }}>{priority}</div>
                  </div>
                  <div>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>CPU / MEMORY</div>
                    <div>{cpu} / {memory}</div>
                  </div>
                  <div>
                    <div style={{ color: 'var(--text-muted)', fontSize: '10px' }}>GPU ACCELERATION</div>
                    <div>{gpu} GPU</div>
                  </div>
                </div>

                {image && (
                  <div style={{ fontSize: '10px', fontFamily: 'var(--font-mono)', color: 'var(--text-dim)', backgroundColor: 'var(--bg-secondary)', padding: '6px 8px', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-color)', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                    Container Image: {image}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
