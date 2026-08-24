import React from 'react';
import { ArrowRight, ShieldCheck, BrainCircuit, Target, Cpu, CheckCircle } from 'lucide-react';

interface RoutingPipelineProps {
  selectedModel?: string;
  stage17Passed?: boolean;
  stage18Passed?: boolean;
  stage19Passed?: boolean;
  isExecuting?: boolean;
}

export const RoutingPipeline: React.FC<RoutingPipelineProps> = ({
  selectedModel = 'medium-balanced-v1',
  stage17Passed = true,
  stage18Passed = true,
  stage19Passed = true,
  isExecuting = false,
}) => {
  const stages = [
    {
      step: 'STAGE 17',
      name: 'SAFETY FILTER',
      sub: '3 Feasible / 1 Rejected',
      icon: ShieldCheck,
      color: 'var(--accent-blue)',
      active: true,
    },
    {
      step: 'STAGE 18',
      name: 'PREDICTION',
      sub: 'Bayesian Quality 95%',
      icon: BrainCircuit,
      color: 'var(--accent-cyan)',
      active: stage17Passed,
    },
    {
      step: 'STAGE 19',
      name: 'ONLINE POLICY',
      sub: 'Contextual UCB λ=0.5',
      icon: Target,
      color: 'var(--accent-purple)',
      active: stage18Passed,
    },
    {
      step: 'MODEL PROVIDER',
      name: selectedModel,
      sub: 'Invocation Completed',
      icon: Cpu,
      color: 'var(--accent-emerald)',
      active: stage19Passed,
    },
  ];

  return (
    <div
      style={{
        backgroundColor: 'var(--bg-panel)',
        border: '1px solid var(--border-color)',
        borderRadius: 'var(--radius-md)',
        padding: '16px 20px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {stages.map((st, idx) => {
        const Icon = st.icon;
        const isLast = idx === stages.length - 1;

        return (
          <React.Fragment key={st.step}>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                padding: '10px 14px',
                borderRadius: 'var(--radius-sm)',
                backgroundColor: 'var(--bg-secondary)',
                border: '1px solid var(--border-color)',
                flex: 1,
                maxWidth: '240px',
              }}
            >
              <div
                style={{
                  width: '32px',
                  height: '32px',
                  borderRadius: 'var(--radius-sm)',
                  backgroundColor: 'rgba(255, 255, 255, 0.03)',
                  border: '1px solid var(--border-color)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: st.color,
                }}
              >
                <Icon size={16} />
              </div>
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', letterSpacing: '0.05em' }}>
                  {st.step}
                </div>
                <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
                  {st.name}
                </div>
                <div style={{ fontSize: '10px', color: 'var(--text-dim)', marginTop: '1px' }}>
                  {st.sub}
                </div>
              </div>
            </div>

            {!isLast && (
              <div style={{ padding: '0 8px', display: 'flex', alignItems: 'center', color: 'var(--text-muted)' }}>
                <ArrowRight size={14} className={isExecuting ? 'animate-pulse' : ''} />
              </div>
            )}
          </React.Fragment>
        );
      })}
    </div>
  );
};
