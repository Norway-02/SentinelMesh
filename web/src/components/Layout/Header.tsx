import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { useLocation } from 'react-router-dom';
import { api } from '../../api/client';
import { StatusBadge } from '../StatusBadge';
import { Radio, RefreshCw, AlertTriangle } from 'lucide-react';

interface HeaderProps {
  isOnline: boolean;
  isReady: boolean;
  sseConnected: boolean;
  lastEventTime: string | null;
  onRetry: () => void;
}

const pageSubtitles: Record<string, { title: string; subtitle: string }> = {
  '/dashboard': { title: 'Dashboard', subtitle: 'Real-time AI control tower & operational metrics' },
  '/tasks': { title: 'Task Workbench', subtitle: 'Route, execute and observe AI workloads' },
  '/router': { title: 'Model Router', subtitle: 'Stage 17 Safety → Stage 18 Prediction → Stage 19 Policy' },
  '/providers': { title: 'Model Providers', subtitle: 'Upstream LLM provider status & circuit telemetry' },
  '/intelligence': { title: 'Adaptive Intelligence', subtitle: 'Bayesian online learning, drift monitoring & guardrails' },
  '/agents': { title: 'Agent Registry', subtitle: 'Registered AI workload agents & resource allocation' },
  '/distributed': { title: 'Distributed Control Plane', subtitle: 'Multi-cluster consensus, fencing tokens & checkpoints' },
  '/experiments': { title: 'Experiments', subtitle: 'Offline A/B testing & shadow routing comparison' },
  '/events': { title: 'Event Explorer', subtitle: 'Real-time domain event log & payload inspector' },
  '/settings': { title: 'System Settings', subtitle: 'Runtime configuration, addresses & engine versions' },
};

export const Header: React.FC<HeaderProps> = ({
  isOnline,
  isReady,
  sseConnected,
  lastEventTime,
  onRetry,
}) => {
  const location = useLocation();
  const meta = pageSubtitles[location.pathname] || { title: 'Control Plane', subtitle: 'SentinelMesh AI Infrastructure' };

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
    staleTime: 5000,
  });

  const { data: openAIHealth } = useQuery({
    queryKey: ['openAIHealth'],
    queryFn: async () => {
      try {
        const res = await fetch(`${import.meta.env.VITE_API_BASE_URL || 'http://127.0.0.1:8787'}/v1/providers/openai/health`);
        if (!res.ok) return null;
        return res.json();
      } catch {
        return null;
      }
    },
    refetchInterval: 5000,
  });

  const activeMode = (settings?.execution_mode || 'SYNTHETIC').toUpperCase();
  const openAIStatus = openAIHealth?.status || (settings?.openai_configured ? 'READY' : 'NOT_CONFIGURED');

  let bannerBg = 'rgba(110, 168, 254, 0.08)';
  let bannerColor = 'var(--accent-blue)';
  let bannerText = '— Synthetic in-memory simulation active';

  if (activeMode === 'LIVE') {
    if (openAIStatus === 'QUOTA_EXHAUSTED') {
      bannerBg = 'rgba(255, 92, 108, 0.12)';
      bannerColor = 'var(--accent-rose)';
      bannerText = '— OpenAI QUOTA EXHAUSTED: Account credits exhausted (add credits at platform.openai.com/billing)';
    } else if (openAIStatus === 'NOT_CONFIGURED') {
      bannerBg = 'rgba(243, 181, 68, 0.12)';
      bannerColor = 'var(--accent-amber)';
      bannerText = '— OpenAI NOT CONFIGURED: OPENAI_API_KEY environment variable missing';
    } else {
      bannerBg = 'rgba(61, 220, 151, 0.12)';
      bannerColor = 'var(--accent-emerald)';
      bannerText = '— OpenAI READY: Real billable inference available';
    }
  } else if (activeMode === 'DRY_RUN') {
    bannerBg = 'rgba(243, 181, 68, 0.12)';
    bannerColor = 'var(--accent-amber)';
    bannerText = '— Provider execution disabled (DRY RUN)';
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column' }}>
      {/* Persistent Execution Mode Banner */}
      <div
        style={{
          padding: '6px 28px',
          fontSize: '11px',
          fontWeight: 700,
          fontFamily: 'var(--font-mono)',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          letterSpacing: '0.04em',
          backgroundColor: bannerBg,
          borderBottom: '1px solid var(--border-color)',
          color: bannerColor,
        }}
      >
        <span>● {activeMode} PROVIDER MODE</span>
        <span style={{ color: 'var(--text-muted)', fontWeight: 400 }}>
          {bannerText}
        </span>
      </div>

      <header
        style={{
          height: '60px',
          backgroundColor: 'var(--bg-secondary)',
          borderBottom: '1px solid var(--border-color)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 28px',
          position: 'sticky',
          top: 0,
          zIndex: 30,
        }}
      >
        {/* Left: Page Title & Contextual Subtitle */}
        <div>
          <h2 style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text-primary)', letterSpacing: '-0.01em' }}>
            {meta.title}
          </h2>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
            {meta.subtitle}
          </div>
        </div>

        {/* Right: Technical Operational Pills */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
          {/* Environment Tag */}
          <span
            style={{
              fontSize: '10px',
              fontWeight: 700,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-muted)',
              backgroundColor: 'var(--bg-panel)',
              border: '1px solid var(--border-color)',
              padding: '3px 8px',
              borderRadius: 'var(--radius-sm)',
              letterSpacing: '0.05em',
            }}
          >
            {settings?.environment?.toUpperCase() || 'DEV'}
          </span>

          {/* Authoritative Single Execution Mode */}
          <StatusBadge status={activeMode.toLowerCase()} />

          {/* Backend Health Status */}
          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '5px',
              fontSize: '11px',
              fontFamily: 'var(--font-mono)',
              fontWeight: 600,
              color: isOnline ? 'var(--accent-emerald)' : 'var(--accent-rose)',
            }}
          >
            ● {isOnline ? 'Backend healthy' : 'Backend offline'}
          </span>

          {/* SSE Stream Status */}
          <div
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '5px',
              fontSize: '11px',
              fontFamily: 'var(--font-mono)',
              fontWeight: 600,
              color: sseConnected ? 'var(--accent-cyan)' : 'var(--text-muted)',
            }}
          >
            <Radio size={11} className={sseConnected ? 'animate-pulse' : ''} />
            <span>{sseConnected ? 'Events connected' : 'Events disconnected'}</span>
          </div>

          {!isOnline && (
            <button
              onClick={onRetry}
              style={{
                backgroundColor: 'var(--badge-red-bg)',
                border: '1px solid var(--badge-red-border)',
                color: 'var(--badge-red-text)',
                fontSize: '11px',
                fontFamily: 'var(--font-mono)',
                fontWeight: 700,
                padding: '3px 8px',
                borderRadius: 'var(--radius-sm)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
              }}
            >
              <RefreshCw size={10} /> RETRY
            </button>
          )}
        </div>
      </header>
    </div>
  );
};
