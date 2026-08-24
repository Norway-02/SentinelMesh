export type TaskComplexity = 'simple' | 'moderate' | 'complex' | 'reasoning_heavy';
export type ModelTier = 'small' | 'medium' | 'large';
export type RoutingPolicy = 'static' | 'cost_optimized' | 'latency_optimized' | 'quality_optimized' | 'balanced';
export type ModelHealthStatus = 'HEALTHY' | 'DEGRADED' | 'UNAVAILABLE';
export type DecisionMode = 'EXPLOIT' | 'EXPLORE' | 'CANARY' | 'SHADOW';

export interface ObservedModelMetrics {
  observed_p50_latency_ms: number;
  observed_error_rate: number;
  consecutive_failures: number;
  last_failure_at?: string;
  last_success_at?: string;
  total_invocations: number;
}

export interface ModelResponse {
  id: string;
  name: string;
  tier: ModelTier;
  provider: string;
  context_window: number;
  security_classes: string[];
  cost_per_1k_input_tokens: number;
  cost_per_1k_output_tokens: number;
  nominal_p50_latency_ms: number;
  nominal_p95_latency_ms: number;
  task_quality_matrix: Record<TaskComplexity, number>;
  health_status: ModelHealthStatus;
  observed_metrics: ObservedModelMetrics;
}

export interface ModelRejection {
  model_id: string;
  reason: string;
  details: string;
}

export interface ScoreBreakdown {
  quality: number;
  cost: number;
  latency: number;
  reliability: number;
}

export interface RoutingDecision {
  task_id: string;
  run_id: string;
  selected_model_id: string;
  selected_tier: ModelTier;
  policy: RoutingPolicy;
  algorithm_version: string;
  registry_version: string;
  policy_version: string;
  estimated_cost_usd: number;
  estimated_latency_ms: number;
  quality_score: number;
  final_score: number;
  score_breakdown: ScoreBreakdown;
  fallback_candidates: string[];
  rejections: ModelRejection[];
  is_pareto_optimal: boolean;
  decided_at: string;
}

export interface BetaQuantiles {
  q025: number;
  q50: number;
  q975: number;
}

export interface QualityEstimate {
  mean: number;
  variance: number;
  samples: number;
  lower_ci: number;
  upper_ci: number;
}

export interface LatencyEstimate {
  predicted_ms: number;
  observed_p50_ms: number;
  observed_p95_ms: number;
  samples: number;
}

export interface CostEstimate {
  predicted_usd: number;
  nominal_usd: number;
  correction_factor: number;
  samples: number;
}

export interface AdaptiveRoutingDecision {
  task_id: string;
  run_id: string;
  selected_model_id: string;
  selected_tier: ModelTier;
  policy: RoutingPolicy;
  effective_utility: number;
  confidence: number;
  sample_count: number;
  predicted_success: number;
  success_quantiles: BetaQuantiles;
  quality_estimate: QualityEstimate;
  latency_estimate: LatencyEstimate;
  cost_estimate: CostEstimate;
  nominal_score: number;
  adaptive_score: number;
  score_breakdown: ScoreBreakdown;
  fallback_candidates: string[];
  rejections: ModelRejection[];
  learning_model_version: string;
  feature_schema_version: string;
  prior_version: string;
  drift_detector_version: string;
  decided_at: string;
}

export interface PolicyDecision {
  task_id: string;
  run_id: string;
  selected_model_id: string;
  selected_tier: ModelTier;
  decision_mode: DecisionMode;
  policy_version: string;
  reward_version: string;
  exploration_version: string;
  expected_utility: number;
  ucb_score: number;
  uncertainty: number;
  exploration_rate: number;
  fallback_candidates: string[];
  rejections: ModelRejection[];
  score_breakdown: ScoreBreakdown;
  decided_at: string;
}

export type TaskExecutionState = 'IDLE' | 'SUBMITTING' | 'ROUTING' | 'PREDICTING' | 'POLICY' | 'INVOKING' | 'STREAMING' | 'COMPLETED' | 'FAILED' | 'TIMEOUT' | 'CANCELLED';

export interface ModelInvocationResponse {
  task_id: string;
  run_id: string;
  model_id: string;
  provider?: string;
  provider_model_id?: string;
  data_source?: string;
  content: string;
  finish_reason: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  estimated_cost_usd?: number;
  actual_cost_usd: number;
  actual_latency: number; // nanoseconds or milliseconds
  quality_score: number;
  quality_status?: string;
  fallback_used: boolean;
  attempt_number: number;
}

export interface ExecuteResponse {
  task_id: string;
  run_id: string;
  executed_at: string;
  stage17_decision?: RoutingDecision;
  stage18_decision?: AdaptiveRoutingDecision;
  stage19_decision?: PolicyDecision;
  invocation_result?: ModelInvocationResponse;
  execution_mode: string;
  error?: string;
}

export interface MetricsSummary {
  active_tasks: number;
  total_decisions: number;
  success_rate: number;
  mean_latency_ms: number;
  p95_latency_ms: number;
  total_cost_usd: number;
  fallback_rate: number;
  drift_alerts: number;
  policy_rollbacks: number;
  active_providers: number;
  requests_per_minute: number;
  calculated_at: string;
}

export interface SSEEvent {
  id: string;
  type: string;
  timestamp: string;
  stage?: string;
  task_id?: string;
  run_id?: string;
  agent_id?: string;
  tenant_id?: string;
  status?: string;
  payload?: any;
  event_type?: string;
  occurred_at?: string;
  aggregate_id?: string;
}

export interface AgentResources {
  cpu: string;
  memory: string;
  gpu?: number;
}

export interface Agent {
  ID: string;
  Name: string;
  Version: string;
  TenantID: string;
  Image: string;
  Resources: AgentResources;
  Priority: string;
  State: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface AgentRun {
  ID: string;
  AgentID: string;
  TenantID: string;
  State: string;
  Attempt: number;
  Node: string;
  Cluster: string;
  StartedAt?: string;
  FinishedAt?: string;
  LastCheckpointID?: string;
  FencingToken?: string;
  RetryCount: number;
  FailureReason?: string;
  VerificationState?: string;
  AttestationID?: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface PolicyState {
  version: string;
  parent_version: string;
  mode: string;
  exploration_lambda: number;
  exploration_budget: number;
  global_exploration_limit: number;
  per_model_exploration_limit: number;
  window_size: number;
  total_decisions: number;
  exploration_count: number;
  exploitation_count: number;
  reward_weights: {
    weight_quality: number;
    weight_success: number;
    weight_cost: number;
    weight_latency: number;
    weight_fallback: number;
  };
  is_rolled_back: boolean;
  last_rollback?: string;
  created_at: string;
  exploration_rate?: number;
}

export interface Settings {
  environment: string;
  http_addr: string;
  log_level: string;
  execution_mode: string;
  provider_count: number;
  registry_version: string;
  router_version: string;
  policy_version: string;
  policy_parent: string;
  policy_mode: string;
  adaptive_version: string;
  drift_detector: string;
  nats_configured: boolean;
  database_configured: boolean;
  openai_configured?: boolean;
  openai_model?: string;
}

export interface RouteTaskPayload {
  prompt: string;
  task_complexity: TaskComplexity;
  quality_requirement?: number;
  latency_sla_ms?: number;
  cost_budget_usd?: number;
  estimated_input_tokens?: number;
  estimated_output_tokens?: number;
  security_profile?: string;
  routing_policy?: RoutingPolicy;
  pinned_model_id?: string;
  task_id?: string;
  agent_id?: string;
  tenant_id?: string;
}
