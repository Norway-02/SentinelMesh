import React from 'react';
import { Wifi, WifiOff, RefreshCw } from 'lucide-react';

interface StatusIndicatorProps {
  isOnline: boolean;
  isReady: boolean;
  lastChecked?: string | null;
  onRetry?: () => void;
}

export const StatusIndicator: React.FC<StatusIndicatorProps> = ({
  isOnline,
  isReady,
  lastChecked,
  onRetry,
}) => {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
      <div
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: '6px',
          padding: '4px 10px',
          borderRadius: 'var(--radius-full)',
          fontSize: '12px',
          fontWeight: 600,
          backgroundColor: isOnline
            ? isReady
              ? 'var(--badge-green-bg)'
              : 'var(--badge-yellow-bg)'
            : 'var(--badge-red-bg)',
          color: isOnline
            ? isReady
              ? 'var(--badge-green-text)'
              : 'var(--badge-yellow-text)'
            : 'var(--badge-red-text)',
          border: `1px solid ${
            isOnline
              ? isReady
                ? 'var(--badge-green-border)'
                : 'var(--badge-yellow-border)'
              : 'var(--badge-red-border)'
          }`,
        }}
      >
        <span
          style={{
            width: '7px',
            height: '7px',
            borderRadius: '50%',
            backgroundColor: isOnline
              ? isReady
                ? '#10b981'
                : '#f59e0b'
              : '#f43f5e',
            animation: isOnline ? 'pulseGlow 2s infinite' : 'none',
          }}
        />
        {isOnline ? (
          <>
            <Wifi size={13} />
            <span>{isReady ? 'SYSTEM HEALTHY' : 'DEGRADED'}</span>
          </>
        ) : (
          <>
            <WifiOff size={13} />
            <span>BACKEND OFFLINE</span>
          </>
        )}
      </div>

      {!isOnline && onRetry && (
        <button
          onClick={onRetry}
          title={`Last checked: ${lastChecked || 'Never'}`}
          style={{
            background: 'none',
            border: '1px solid var(--border-color)',
            color: 'var(--text-secondary)',
            borderRadius: 'var(--radius-sm)',
            padding: '4px 8px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '4px',
            fontSize: '11px',
          }}
        >
          <RefreshCw size={12} />
          Retry
        </button>
      )}
    </div>
  );
};
