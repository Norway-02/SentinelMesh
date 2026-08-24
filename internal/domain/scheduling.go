package domain

import "time"

// NodeResources represents the capacity and available resources on a compute node.
type NodeResources struct {
	CPUCapacity      float64 `json:"cpu_capacity"`
	CPUAvailable     float64 `json:"cpu_available"`
	MemoryCapacity   float64 `json:"memory_capacity"` // in MB or Bytes, we'll assume GiB for simplicity or match agent string, but floats are easiest to math. Let's assume standard units.
	MemoryAvailable  float64 `json:"memory_available"`
	GPUCapacity      int     `json:"gpu_capacity"`
	GPUAvailable     int     `json:"gpu_available"`
}

type NodeHealth string

const (
	NodeHealthHealthy   NodeHealth = "HEALTHY"
	NodeHealthDegraded  NodeHealth = "DEGRADED"
	NodeHealthUnhealthy NodeHealth = "UNHEALTHY"
)

// Node represents a compute node in a cluster.
type Node struct {
	ID             string            `json:"id"`
	ClusterID      string            `json:"cluster_id"`
	Resources      NodeResources     `json:"resources"`
	Health         NodeHealth        `json:"health"`
	SecurityClass  string            `json:"security_class"`
	Labels         map[string]string `json:"labels"`
	Taints         []string          `json:"taints"`
	NetworkLatency float64           `json:"network_latency"` // normalized 0-1 or actual ms
	CostPerHour    float64           `json:"cost_per_hour"`
}

// SchedulingDecision holds the detailed scoring logic for transparency.
type SchedulingDecision struct {
	ResourceFit          float64 `json:"resource_fit"`
	Latency              float64 `json:"latency"`
	Security             float64 `json:"security"`
	Priority             float64 `json:"priority"`
	Locality             float64 `json:"locality"`
	Cost                 float64 `json:"cost"`
	CandidatesConsidered int     `json:"candidates_considered"`
	CandidatesRejected   int     `json:"candidates_rejected"`
}

// SchedulingAssignment is the persisted result of a scheduling decision.
type SchedulingAssignment struct {
	RunID               string             `json:"run_id"`
	ClusterID           string             `json:"cluster_id"`
	NodeID              string             `json:"node_id"`
	AlgorithmVersion    string             `json:"algorithm_version"`
	ExecutionGeneration int                `json:"execution_generation"`
	FencingToken        string             `json:"fencing_token"`
	Score               float64            `json:"score"`
	Decision            SchedulingDecision `json:"decision"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
	Version             int                `json:"version"`
}
