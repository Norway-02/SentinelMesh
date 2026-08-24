package scheduler

import (
	"fmt"
	"math"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
)

// ClusterScoringPolicy defines the configurable weight distribution for Tier 1 cluster selection.
type ClusterScoringPolicy struct {
	WeightHealth   float64 `json:"weight_health"`
	WeightLocality float64 `json:"weight_locality"`
	WeightCost     float64 `json:"weight_cost"`
	WeightHeadroom float64 `json:"weight_headroom"`
	WeightSecurity float64 `json:"weight_security"`
}

// DefaultClusterScoringPolicy provides the production baseline weights for Tier 1 scheduling.
func DefaultClusterScoringPolicy() ClusterScoringPolicy {
	return ClusterScoringPolicy{
		WeightHealth:   0.25,
		WeightLocality: 0.25,
		WeightCost:     0.20,
		WeightHeadroom: 0.20,
		WeightSecurity: 0.10,
	}
}

// ScoreCluster evaluates a candidate cluster against agent requirements and scoring policy.
func (p ClusterScoringPolicy) ScoreCluster(agent *domain.Agent, cluster domain.Cluster) (domain.ClusterSchedulingDecision, bool) {
	// 1. Feasibility Filter: Health
	if !cluster.Status.Health.IsAvailable() {
		return domain.ClusterSchedulingDecision{
			ClusterID: cluster.ID,
			Reason:    fmt.Sprintf("cluster health %s is not available", cluster.Status.Health),
		}, false
	}

	// 2. Feasibility Filter: Security Class
	reqSecurity := agent.SecurityPolicy.Profile
	if reqSecurity == "" {
		reqSecurity = "standard"
	}
	if !cluster.HasSecurityClass(reqSecurity) {
		return domain.ClusterSchedulingDecision{
			ClusterID: cluster.ID,
			Reason:    fmt.Sprintf("cluster does not support required security class %s", reqSecurity),
		}, false
	}

	// 3. Feasibility Filter: Aggregate Capacity
	reqCPU := parseCPU(agent.Resources.CPU)
	reqMem := parseMemory(agent.Resources.Memory)
	if cluster.Status.AvailableCPU > 0 && cluster.Status.AvailableCPU < reqCPU {
		return domain.ClusterSchedulingDecision{
			ClusterID: cluster.ID,
			Reason:    fmt.Sprintf("insufficient aggregate CPU: requested %f, available %f", reqCPU, cluster.Status.AvailableCPU),
		}, false
	}
	if cluster.Status.AvailableMemory > 0 && cluster.Status.AvailableMemory < reqMem {
		return domain.ClusterSchedulingDecision{
			ClusterID: cluster.ID,
			Reason:    fmt.Sprintf("insufficient aggregate memory: requested %f, available %f", reqMem, cluster.Status.AvailableMemory),
		}, false
	}

	// Compute Individual Dimension Scores
	healthScore := 1.0
	if cluster.Status.Health == domain.ClusterHealthDegraded {
		healthScore = 0.60
	}

	// Locality score based on WAN/BaseLatency (normalize 0-200ms)
	maxLatency := 200.0
	localityScore := 1.0 - (cluster.BaseLatencyMs / maxLatency)
	if localityScore < 0 {
		localityScore = 0
	}

	// Cost score (lower cost factor is better, baseline max 5.0)
	maxCost := 5.0
	costScore := 1.0 - (cluster.NetworkCost / maxCost)
	if costScore < 0 {
		costScore = 0
	}

	// Headroom score (fraction of CPU and Memory remaining)
	var cpuHeadroom, memHeadroom float64 = 1.0, 1.0
	if cluster.Status.TotalCPU > 0 {
		cpuHeadroom = cluster.Status.AvailableCPU / cluster.Status.TotalCPU
	}
	if cluster.Status.TotalMemory > 0 {
		memHeadroom = cluster.Status.AvailableMemory / cluster.Status.TotalMemory
	}
	headroomScore := math.Max(0, math.Min(1.0, 0.5*cpuHeadroom+0.5*memHeadroom))

	// Security score (1.0 since passed requirement)
	securityScore := 1.0

	finalScore := p.WeightHealth*healthScore +
		p.WeightLocality*localityScore +
		p.WeightCost*costScore +
		p.WeightHeadroom*headroomScore +
		p.WeightSecurity*securityScore

	decision := domain.ClusterSchedulingDecision{
		ClusterID:     cluster.ID,
		HealthScore:   healthScore,
		LocalityScore: localityScore,
		CostScore:     costScore,
		HeadroomScore: headroomScore,
		SecurityScore: securityScore,
		FinalScore:    finalScore,
	}

	return decision, true
}

// SelectBestCluster evaluates all available clusters and selects the highest scoring candidate.
func SelectBestCluster(agent *domain.Agent, clusters []domain.Cluster, excludeClusterIDs []string, policy ClusterScoringPolicy) (domain.Cluster, domain.ClusterSchedulingDecision, error) {
	excludeMap := make(map[string]bool)
	for _, id := range excludeClusterIDs {
		excludeMap[id] = true
	}

	var bestCluster domain.Cluster
	var bestDecision domain.ClusterSchedulingDecision
	bestScore := -1.0

	var consideredCount, validCount int

	for _, c := range clusters {
		if excludeMap[c.ID] {
			continue
		}
		consideredCount++

		decision, valid := policy.ScoreCluster(agent, c)
		if !valid {
			continue
		}
		validCount++

		if decision.FinalScore > bestScore {
			bestScore = decision.FinalScore
			bestCluster = c
			bestDecision = decision
		}
	}

	if validCount == 0 {
		return domain.Cluster{}, domain.ClusterSchedulingDecision{}, fmt.Errorf("no feasible cluster candidate found among %d considered clusters", consideredCount)
	}

	return bestCluster, bestDecision, nil
}
