import React from 'react';
import { LucideIcon } from 'lucide-react';

interface MetricCardProps {
  title: string;
  value: string | number;
  icon?: LucideIcon;
  color?: string;
  bgColor?: string;
  borderColor?: string;
  sub?: string;
  trend?: string;
  isPositiveTrend?: boolean;
  isHighlight?: boolean;
}

export const MetricCard: React.FC<MetricCardProps> = ({
  title,
  value,
  icon: Icon,
  color = 'var(--accent-blue)',
  bgColor = 'var(--bg-panel)',
  borderColor = 'var(--border-color)',
  sub,
  trend,
  isPositiveTrend,
  isHighlight,
}) => {
  return (
    <div
      style={{
        backgroundColor: isHighlight ? 'rgba(61, 220, 151, 0.04)' : bgColor,
        border: `1px solid ${isHighlight ? 'rgba(61, 220, 151, 0.25)' : borderColor}`,
        borderRadius: 'var(--radius-md)',
        padding: '16px 18px',
        display: 'flex',
        flexDirection: 'column',
        gap: '6px',
        position: 'relative',
        overflow: 'hidden',
        transition: 'all 0.15s ease',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span style={{ fontSize: '11px', fontWeight: 700, color: isHighlight ? 'var(--accent-emerald)' : 'var(--text-muted)', letterSpacing: '0.06em', textTransform: 'uppercase' }}>
          {title}
        </span>
        {Icon && <Icon size={16} style={{ color }} />}
      </div>

      <div style={{ fontSize: '28px', fontWeight: 700, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)', letterSpacing: '-0.02em', marginTop: '2px' }}>
        {value}
      </div>

      {(sub || trend) && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '11px', color: 'var(--text-dim)', marginTop: '2px' }}>
          {sub && <span>{sub}</span>}
          {trend && (
            <span style={{ color: isPositiveTrend ? 'var(--accent-emerald)' : 'var(--accent-rose)', fontWeight: 600, fontFamily: 'var(--font-mono)' }}>
              {trend}
            </span>
          )}
        </div>
      )}
    </div>
  );
};
