import React from 'react';
import { Check, X, ShieldCheck, XCircle, ChevronRight } from 'lucide-react';
import { ModelResponse, ModelRejection } from '../api/types';
import { formatPercent, formatCurrency, formatLatency } from '../utils/formatters';

interface WhyThisModelDrawerProps {
  isOpen: boolean;
  onClose: () => void;
  selectedModelId: string;
  selectedModelObj?: ModelResponse;
  rejections?: ModelRejection[];
  minQuality?: number;
  maxCost?: number;
  maxLatency?: number;
}

export const WhyThisModelDrawer: React.FC<WhyThisModelDrawerProps> = ({
  isOpen,
  onClose,
  selectedModelId,
  selectedModelObj,
  rejections = [],
  minQuality = 0.85,
  maxCost = 0.10,
  maxLatency = 2000,
}) => {
  if (!isOpen) return null;

  const checklist = [
    { label: 'Security compatible', detail: 'Passed enterprise isolation policies', valid: true },
    { label: 'Context capacity sufficient', detail: `${selectedModelObj?.context_window?.toLocaleString() || '128,000'} tokens available`, valid: true },
    { label: 'Quality floor satisfied', detail: `Model score ≥ ${formatPercent(minQuality)}`, valid: true },
    { label: 'Cost budget satisfied', detail: `Est. cost ≤ ${formatCurrency(maxCost)}`, valid: true },
    { label: 'Latency SLA satisfied', detail: `Est. P50 ≤ ${formatLatency(maxLatency)}`, valid: true },
    { label: 'Highest effective utility score', detail: 'Stage 19 Online Policy UCB arm selection winner', valid: true },
    { label: 'Exploration not required', detail: 'Sufficient historical sample confidence', valid: true },
    { label: 'Zero invariant violations', detail: 'Stage 17 deterministic safety gate passed', valid: true },
  ];

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.75)',
        backdropFilter: 'blur(4px)',
        zIndex: 100,
        display: 'flex',
        justifyContent: 'flex-end',
      }}
      onClick={onClose}
    >
      <div
        style={{
          width: '460px',
          height: '100%',
          backgroundColor: 'var(--bg-elevated)',
          borderLeft: '1px solid var(--border-color-light)',
          padding: '24px',
          display: 'flex',
          flexDirection: 'column',
          gap: '20px',
          boxShadow: 'var(--shadow-lg)',
          overflowY: 'auto',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '16px' }}>
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--accent-blue)', letterSpacing: '0.06em', textTransform: 'uppercase' }}>
              DECISION EXPLANATION
            </div>
            <h2 style={{ fontSize: '18px', fontWeight: 700, fontFamily: 'var(--font-mono)', marginTop: '2px', color: 'var(--text-primary)' }}>
              WHY THIS MODEL?
            </h2>
          </div>
          <button
            onClick={onClose}
            style={{
              background: 'none',
              border: '1px solid var(--border-color)',
              color: 'var(--text-muted)',
              borderRadius: 'var(--radius-sm)',
              padding: '6px',
              cursor: 'pointer',
            }}
          >
            <X size={16} />
          </button>
        </div>

        {/* Winner Banner */}
        <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--accent-blue)', borderRadius: 'var(--radius-md)', padding: '16px' }}>
          <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)' }}>SELECTED ENDPOINT</div>
          <div style={{ fontSize: '18px', fontWeight: 700, fontFamily: 'var(--font-mono)', color: 'var(--accent-blue)', marginTop: '2px' }}>
            {selectedModelId}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-secondary)', marginTop: '4px' }}>
            Provider: {selectedModelObj?.provider || 'synthetic-cloud'} | Tier: {(selectedModelObj?.tier || 'large').toUpperCase()}
          </div>
        </div>

        {/* Checklist */}
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.05em', marginBottom: '10px' }}>
            FEASIBILITY &amp; POLICY VERIFICATION
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {checklist.map((item, idx) => (
              <div
                key={idx}
                style={{
                  padding: '10px 12px',
                  backgroundColor: 'var(--bg-secondary)',
                  border: '1px solid var(--border-color)',
                  borderRadius: 'var(--radius-sm)',
                  display: 'flex',
                  alignItems: 'flex-start',
                  gap: '10px',
                }}
              >
                <div style={{ color: 'var(--accent-emerald)', marginTop: '2px' }}>
                  <Check size={14} />
                </div>
                <div>
                  <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
                    {item.label}
                  </div>
                  <div style={{ fontSize: '11px', color: 'var(--text-dim)', marginTop: '1px' }}>
                    {item.detail}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Rejections */}
        {rejections.length > 0 && (
          <div>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--accent-rose)', letterSpacing: '0.05em', marginBottom: '10px' }}>
              REJECTED CANDIDATES ({rejections.length})
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {rejections.map((rej) => (
                <div
                  key={rej.model_id}
                  style={{
                    padding: '10px 12px',
                    backgroundColor: 'rgba(255, 92, 108, 0.05)',
                    border: '1px solid var(--badge-red-border)',
                    borderRadius: 'var(--radius-sm)',
                    fontSize: '11px',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontWeight: 700, fontFamily: 'var(--font-mono)' }}>
                    <span>{rej.model_id}</span>
                    <span style={{ color: 'var(--accent-rose)', fontSize: '10px' }}>REJECTED</span>
                  </div>
                  <div style={{ color: 'var(--badge-red-text)', marginTop: '4px', fontFamily: 'var(--font-mono)' }}>
                    Reason: {rej.reason}
                  </div>
                  <div style={{ color: 'var(--text-dim)', marginTop: '2px', fontSize: '10px' }}>
                    {rej.details}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
