import React from 'react';
import { Sidebar } from './Sidebar';
import { Header } from './Header';
import { useBackendStatus } from '../../hooks/useBackendStatus';
import { useSSE } from '../../hooks/useSSE';

interface LayoutProps {
  children: React.ReactNode;
}

export const Layout: React.FC<LayoutProps> = ({ children }) => {
  const { isOnline, isReady, checkStatus, error } = useBackendStatus();
  const { isConnected: sseConnected, lastEventTime } = useSSE();

  return (
    <div style={{ display: 'flex', minHeight: '100vh', backgroundColor: 'var(--bg-primary)' }}>
      <Sidebar />
      <div style={{ flex: 1, marginLeft: '230px', display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        <Header
          isOnline={isOnline}
          isReady={isReady}
          sseConnected={sseConnected}
          lastEventTime={lastEventTime}
          onRetry={checkStatus}
        />
        
        {!isOnline && (
          <div
            style={{
              backgroundColor: 'rgba(255, 92, 108, 0.08)',
              borderBottom: '1px solid var(--badge-red-border)',
              color: 'var(--badge-red-text)',
              padding: '10px 28px',
              fontSize: '12px',
              fontFamily: 'var(--font-mono)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <div>
              <strong>SentinelMesh Backend Offline:</strong> Unable to reach API server at <code>http://127.0.0.1:8787</code>. {error}
            </div>
            <button
              onClick={checkStatus}
              style={{
                backgroundColor: 'var(--accent-rose)',
                color: '#fff',
                border: 'none',
                padding: '4px 12px',
                borderRadius: 'var(--radius-sm)',
                cursor: 'pointer',
                fontSize: '11px',
                fontWeight: 700,
                fontFamily: 'var(--font-mono)',
              }}
            >
              RECONNECT
            </button>
          </div>
        )}

        <main style={{ flex: 1, padding: '28px', maxWidth: '1680px', width: '100%' }}>
          {children}
        </main>
      </div>
    </div>
  );
};
