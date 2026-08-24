import React from 'react';
import { CheckCircle2, AlertTriangle, XCircle, Radio } from 'lucide-react';

export type StatusVariant = 'healthy' | 'degraded' | 'offline' | 'synthetic' | 'live' | 'dry_run';

interface StatusBadgeProps {
  status: StatusVariant | string;
  label?: string;
  size?: 'sm' | 'md';
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status, label, size = 'md' }) => {
  const norm = String(status).toLowerCase();

  let bg = 'var(--badge-blue-bg)';
  let color = 'var(--badge-blue-text)';
  let border = 'var(--badge-blue-border)';
  let text = label || norm.toUpperCase();
  let Icon = CheckCircle2;

  if (norm === 'healthy' || norm === 'live' || norm === 'passed') {
    bg = 'var(--badge-green-bg)';
    color = 'var(--badge-green-text)';
    border = 'var(--badge-green-border)';
    Icon = CheckCircle2;
  } else if (norm === 'degraded' || norm === 'warning' || norm === 'dry_run' || norm === 'dry run') {
    bg = 'var(--badge-yellow-bg)';
    color = 'var(--badge-yellow-text)';
    border = 'var(--badge-yellow-border)';
    Icon = AlertTriangle;
  } else if (norm === 'offline' || norm === 'failed' || norm === 'rejected' || norm === 'critical' || norm === 'quota_exhausted') {
    bg = 'var(--badge-red-bg)';
    color = 'var(--badge-red-text)';
    border = 'var(--badge-red-border)';
    text = label || (norm === 'quota_exhausted' ? 'QUOTA EXHAUSTED' : norm.toUpperCase());
    Icon = XCircle;
  } else if (norm === 'synthetic') {
    bg = 'var(--badge-purple-bg)';
    color = 'var(--badge-purple-text)';
    border = 'var(--badge-purple-border)';
    Icon = Radio;
  } else if (norm === 'unconfigured' || norm === 'not_configured') {
    bg = 'rgba(255, 255, 255, 0.05)';
    color = 'var(--text-muted)';
    border = 'rgba(255, 255, 255, 0.10)';
    text = label || 'NOT CONFIGURED';
    Icon = AlertTriangle;
  }

  const padding = size === 'sm' ? '2px 6px' : '3px 9px';
  const fontSize = size === 'sm' ? '10px' : '11px';

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '5px',
        fontSize,
        fontWeight: 700,
        fontFamily: 'var(--font-mono)',
        backgroundColor: bg,
        color,
        border: `1px solid ${border}`,
        borderRadius: 'var(--radius-full)',
        padding,
        letterSpacing: '0.04em',
      }}
    >
      <Icon size={size === 'sm' ? 10 : 12} />
      <span>{text}</span>
    </span>
  );
};
