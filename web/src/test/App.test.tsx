import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import App from '../App';

// Mock global fetch for API calls
beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn((url: string) => {
    if (url.includes('/healthz') || url.includes('/readyz')) {
      return Promise.resolve({
        ok: true,
        status: 200,
        text: () => Promise.resolve('OK'),
      });
    }
    if (url.includes('/v1/metrics/summary')) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          active_tasks: 0,
          total_decisions: 12,
          success_rate: 0.984,
          mean_latency_ms: 162,
          p95_latency_ms: 220,
          total_cost_usd: 4.21,
          fallback_rate: 0.03,
          drift_alerts: 1,
          policy_rollbacks: 0,
          active_providers: 3,
          requests_per_minute: 5.0,
          calculated_at: new Date().toISOString(),
        }),
      });
    }
    if (url.includes('/v1/events')) {
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ events: [], count: 0 }),
      });
    }
    return Promise.resolve({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    });
  }) as any);
});

describe('SentinelMesh Control Plane GUI', () => {
  it('renders application header and sidebar', async () => {
    render(<App />);
    expect(screen.getByText('SENTINELMESH')).toBeInTheDocument();
    expect(screen.getAllByText('Dashboard')[0]).toBeInTheDocument();
    expect(screen.getByText('Tasks')).toBeInTheDocument();
  });

  it('displays real metric values from API client', async () => {
    render(<App />);
    await waitFor(() => {
      expect(screen.getByText('98.4%')).toBeInTheDocument();
      expect(screen.getByText('$4.21')).toBeInTheDocument();
    });
  });
});
