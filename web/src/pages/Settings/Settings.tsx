import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/client';
import { StatusBadge } from '../../components/StatusBadge';
import { Server, ShieldCheck, Database, Radio } from 'lucide-react';

export const Settings: React.FC = () => {
  const { data: settings, isLoading } = useQuery({
    queryKey: ['settingsInfo'],
    queryFn: () => api.getSettings(),
  });

  const activeMode = (settings?.execution_mode || 'SYNTHETIC').toLowerCase();

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
      <div>
        <h1 style={{ fontSize: '20px', fontWeight: 700, letterSpacing: '-0.01em', color: 'var(--text-primary)' }}>
          System Configuration &amp; Metadata
        </h1>
        <p style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
          Operational configuration matrix, engine version tags, and connection status (secrets redacted).
        </p>
      </div>

      {isLoading ? (
        <div style={{ padding: '40px 0', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)' }}>Loading configuration...</div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '24px' }}>
          {/* Environment & Runtime */}
          <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
              <Server size={18} style={{ color: 'var(--accent-blue)' }} />
              <h3 style={{ fontSize: '14px', fontWeight: 700 }}>Environment &amp; Runtime</h3>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
                <span style={{ color: 'var(--text-muted)' }}>ENVIRONMENT</span>
                <span style={{ fontWeight: 600, color: 'var(--accent-emerald)' }}>
                  {settings?.environment?.toUpperCase() || 'DEVELOPMENT'}
                </span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
                <span style={{ color: 'var(--text-muted)' }}>LISTEN ADDRESS</span>
                <span style={{ fontWeight: 600 }}>
                  {settings?.http_addr || '127.0.0.1:8787'}
                </span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
                <span style={{ color: 'var(--text-muted)' }}>LOG LEVEL</span>
                <span style={{ fontWeight: 600 }}>
                  {settings?.log_level?.toUpperCase() || 'INFO'}
                </span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span style={{ color: 'var(--text-muted)' }}>EXECUTION MODE</span>
                <StatusBadge status={activeMode} size="sm" />
              </div>
            </div>
          </div>

          {/* Engine Versioning Tags */}
          <div style={{ backgroundColor: 'var(--bg-panel)', border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '20px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', borderBottom: '1px solid var(--border-color)', paddingBottom: '10px' }}>
              <ShieldCheck size={18} style={{ color: 'var(--accent-purple)' }} />
              <h3 style={{ fontSize: '14px', fontWeight: 700 }}>Engine Versioning Tags</h3>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', fontSize: '12px', fontFamily: 'var(--font-mono)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
                <span style={{ color: 'var(--text-muted)' }}>STAGE 17 ROUTER</span>
                <span style={{ fontWeight: 600 }}>
                  {settings?.router_version || 'router-v1.0'}
                </span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
                <span style={{ color: 'var(--text-muted)' }}>STAGE 18 ADAPTIVE</span>
                <span style={{ fontWeight: 600 }}>
                  {settings?.adaptive_version || 'adaptive-v1.0'}
                </span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', borderBottom: '1px solid var(--border-color)', paddingBottom: '8px' }}>
                <span style={{ color: 'var(--text-muted)' }}>STAGE 19 ONLINE POLICY</span>
                <span style={{ fontWeight: 600, color: 'var(--accent-purple)' }}>
                  {settings?.policy_version || 'policy-v2.0'}
                </span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ color: 'var(--text-muted)' }}>DRIFT DETECTOR</span>
                <span style={{ fontWeight: 600 }}>
                  {settings?.drift_detector || 'drift-v1.0'}
                </span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
