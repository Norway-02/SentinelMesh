package domain

import "time"

// ClusterHealth represents the observed health and availability of a cluster.
type ClusterHealth string

const (
	ClusterHealthHealthy     ClusterHealth = "HEALTHY"
	ClusterHealthDegraded    ClusterHealth = "DEGRADED"
	ClusterHealthUnreachable ClusterHealth = "UNREACHABLE"
	ClusterHealthDrained     ClusterHealth = "DRAINED"
)

// IsAvailable returns true if the cluster can accept new workload placements.
func (h ClusterHealth) IsAvailable() bool {
	return h == ClusterHealthHealthy || h == ClusterHealthDegraded
}

// ClusterProviderType represents the underlying infrastructure orchestrator type.
type ClusterProviderType string

const (
	ProviderKubernetes ClusterProviderType = "kubernetes"
	ProviderK3s        ClusterProviderType = "k3s"
	ProviderEdge       ClusterProviderType = "edge"
	ProviderServerless ClusterProviderType = "serverless"
)

// Cluster represents a discrete compute cluster managed within SentinelMesh.
type Cluster struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Region          string              `json:"region"`
	ProviderType    ClusterProviderType `json:"provider_type"`
	SecurityClasses []string            `json:"security_classes"`
	NetworkCost     float64             `json:"network_cost"`    // Relative egress/bandwidth cost weight (e.g. 1.0 = baseline, 2.0 = expensive cross-region)
	BaseLatencyMs   float64             `json:"base_latency_ms"` // Baseline inter-cluster/WAN latency in ms
	Labels          map[string]string   `json:"labels"`
	Status          ClusterStatus       `json:"status"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// HasSecurityClass checks if the cluster supports a required security profile.
func (c *Cluster) HasSecurityClass(requiredClass string) bool {
	if requiredClass == "" || requiredClass == "standard" || requiredClass == "public" {
		return true
	}
	for _, sc := range c.SecurityClasses {
		if sc == requiredClass {
			return true
		}
	}
	return false
}

// ClusterStatus represents dynamic, rapidly-changing telemetry and capacity for a cluster.
type ClusterStatus struct {
	Health          ClusterHealth `json:"health"`
	TotalCPU        float64       `json:"total_cpu"`
	TotalMemory     float64       `json:"total_memory"` // In MB
	AvailableCPU    float64       `json:"available_cpu"`
	AvailableMemory float64       `json:"available_memory"` // In MB
	LastHeartbeatAt time.Time     `json:"last_heartbeat_at"`
}

// ClusterSchedulingDecision holds the Tier 1 cluster scoring details for explainability.
type ClusterSchedulingDecision struct {
	ClusterID     string  `json:"cluster_id"`
	HealthScore   float64 `json:"health_score"`
	LocalityScore float64 `json:"locality_score"`
	CostScore     float64 `json:"cost_score"`
	HeadroomScore float64 `json:"headroom_score"`
	SecurityScore float64 `json:"security_score"`
	FinalScore    float64 `json:"final_score"`
	Reason        string  `json:"reason,omitempty"`
}

// MultiClusterSchedulingDecision encapsulates both Tier 1 (Cluster) and Tier 2 (Node) placement decisions.
type MultiClusterSchedulingDecision struct {
	SelectedCluster      Cluster                   `json:"selected_cluster"`
	ClusterDecision      ClusterSchedulingDecision `json:"cluster_decision"`
	SelectedNode         Node                      `json:"selected_node"`
	NodeDecision         SchedulingDecision        `json:"node_decision"`
	CandidatesConsidered int                       `json:"candidates_considered"`
	CandidatesRejected   int                       `json:"candidates_rejected"`
}
