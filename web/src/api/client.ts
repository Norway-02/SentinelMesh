import {
  ModelResponse,
  RoutingDecision,
  AdaptiveRoutingDecision,
  PolicyDecision,
  ExecuteResponse,
  MetricsSummary,
  Agent,
  AgentRun,
  PolicyState,
  Settings,
  RouteTaskPayload,
  SSEEvent,
} from './types';

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://127.0.0.1:8787';

export class APIClientError extends Error {
  status: number;
  statusText: string;
  data: any;

  constructor(status: number, statusText: string, message: string, data?: any) {
    super(message);
    this.name = 'APIClientError';
    this.status = status;
    this.statusText = statusText;
    this.data = data;
  }
}

async function request<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const url = endpoint.startsWith('http') ? endpoint : `${API_BASE}${endpoint}`;
  
  try {
    const res = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    });

    if (!res.ok) {
      let errorData: any = null;
      let errorMsg = `HTTP ${res.status} ${res.statusText}`;
      try {
        errorData = await res.json();
        if (errorData?.message) {
          errorMsg = errorData.message;
        } else if (errorData?.error) {
          errorMsg = typeof errorData.error === 'string' ? errorData.error : JSON.stringify(errorData.error);
        }
      } catch {
        // ignore json parse error
      }
      throw new APIClientError(res.status, res.statusText, errorMsg, errorData);
    }

    if (res.status === 24) return {} as T;
    return await res.json();
  } catch (err: any) {
    if (err instanceof APIClientError) {
      throw err;
    }
    throw new APIClientError(0, 'NETWORK_ERROR', `Unable to reach SentinelMesh API at ${API_BASE}. ${err.message}`);
  }
}

export const api = {
  // Health
  async getHealthz(): Promise<string> {
    const url = `${API_BASE}/healthz`;
    const res = await fetch(url);
    if (!res.ok) throw new Error(`Healthz failed: ${res.status}`);
    return res.text();
  },

  async getReadyz(): Promise<string> {
    const url = `${API_BASE}/readyz`;
    const res = await fetch(url);
    if (!res.ok) throw new Error(`Readyz failed: ${res.status}`);
    return res.text();
  },

  // Models
  async getModels(): Promise<{ models: ModelResponse[]; count: number; registry_version: string }> {
    return request<{ models: ModelResponse[]; count: number; registry_version: string }>('/v1/models');
  },

  async getModel(id: string): Promise<ModelResponse> {
    return request<ModelResponse>(`/v1/models/${encodeURIComponent(id)}`);
  },

  // Router (Stage 17)
  async routeStage17(payload: RouteTaskPayload): Promise<RoutingDecision> {
    return request<RoutingDecision>('/v1/router/route', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async executeStage17(payload: RouteTaskPayload): Promise<any> {
    return request<any>('/v1/router/execute', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async getDecisions(limit = 50): Promise<{ decisions: any[]; count: number }> {
    return request<{ decisions: any[]; count: number }>(`/v1/router/decisions?limit=${limit}`);
  },

  async getOutcomes(limit = 50): Promise<{ outcomes: any[]; count: number }> {
    return request<{ outcomes: any[]; count: number }>(`/v1/router/outcomes?limit=${limit}`);
  },

  // Adaptive (Stage 18)
  async routeStage18(payload: RouteTaskPayload): Promise<AdaptiveRoutingDecision> {
    return request<AdaptiveRoutingDecision>('/v1/adaptive/route', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  // Policy (Stage 19)
  async routeStage19(payload: RouteTaskPayload): Promise<PolicyDecision> {
    return request<PolicyDecision>('/v1/policy/route', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  // Flagship Execute: Full Stage 17 -> 18 -> 19 -> Provider pipeline
  async executePolicy(payload: RouteTaskPayload): Promise<ExecuteResponse> {
    return request<ExecuteResponse>('/v1/policy/execute', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  },

  async getPolicyState(): Promise<PolicyState> {
    return request<PolicyState>('/v1/policy/state');
  },

  // Metrics
  async getMetricsSummary(): Promise<MetricsSummary> {
    return request<MetricsSummary>('/v1/metrics/summary');
  },

  // Events
  async getEvents(params?: { limit?: number; type?: string; stage?: string; task_id?: string }): Promise<{ events: SSEEvent[]; count: number }> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.type) query.set('type', params.type);
    if (params?.stage) query.set('stage', params.stage);
    if (params?.task_id) query.set('task_id', params.task_id);
    return request<{ events: SSEEvent[]; count: number }>(`/v1/events?${query.toString()}`);
  },

  // Agents & Runs
  async getAgents(): Promise<{ agents: Agent[]; next_page_token: string }> {
    return request<{ agents: Agent[]; next_page_token: string }>('/v1/agents');
  },

  async getAgent(id: string): Promise<Agent> {
    return request<Agent>(`/v1/agents/${encodeURIComponent(id)}`);
  },

  async getRun(id: string): Promise<AgentRun> {
    return request<AgentRun>(`/v1/runs/${encodeURIComponent(id)}`);
  },

  // Settings
  async getSettings(): Promise<Settings> {
    return request<Settings>('/v1/settings');
  },
};
