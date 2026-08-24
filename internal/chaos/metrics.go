package chaos

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ExperimentMetrics captures exact quantitative telemetry and invariant evaluations for a chaos run.
type ExperimentMetrics struct {
	ScenarioID                        string        `json:"scenario_id"`
	Seed                              int64         `json:"seed"`
	FaultType                         FaultType     `json:"fault_type"`
	FaultInjectedAt                   time.Time     `json:"fault_injected_at"`
	FaultObservedAt                   time.Time     `json:"fault_observed_at"`
	RecoveryStartedAt                 time.Time     `json:"recovery_started_at"`
	ReplacementActiveAt               time.Time     `json:"replacement_active_at"`
	RecoveryCompletedAt               time.Time     `json:"recovery_completed_at"`
	DetectionLatency                  time.Duration `json:"detection_latency"`
	RecoveryLatency                   time.Duration `json:"recovery_latency"`
	LostWorkSteps                     int64         `json:"lost_work_steps"`
	DuplicateExecutions               int           `json:"duplicate_executions"` // Multiple concurrent pods
	AuthorityViolations               int           `json:"authority_violations"` // Multiple concurrent authoritative tokens
	RestoredCheckpoint                bool          `json:"restored_checkpoint"`
	RestoredSequence                  int64         `json:"restored_sequence"`
	FinalGeneration                   int           `json:"final_generation"`
	ExpectedFinalState                string        `json:"expected_final_state"`
	ActualFinalState                  string        `json:"actual_final_state"`
	ExpectedAuthoritativeGenerations  int           `json:"expected_authoritative_generations"`
	ActualAuthoritativeGenerations    int           `json:"actual_authoritative_generations"`
	Outcome                           string        `json:"outcome"` // "PASS" | "FAIL"
	Reason                            string        `json:"reason,omitempty"`
}

// ComputeLatencies calculates detection and recovery latency from timestamps if not already set.
func (m *ExperimentMetrics) ComputeLatencies() {
	if m.DetectionLatency == 0 && !m.FaultInjectedAt.IsZero() && !m.FaultObservedAt.IsZero() {
		m.DetectionLatency = m.FaultObservedAt.Sub(m.FaultInjectedAt)
	}
	if m.RecoveryLatency == 0 && !m.FaultObservedAt.IsZero() && !m.ReplacementActiveAt.IsZero() {
		m.RecoveryLatency = m.ReplacementActiveAt.Sub(m.FaultObservedAt)
	} else if m.RecoveryLatency == 0 && !m.FaultObservedAt.IsZero() && !m.RecoveryCompletedAt.IsZero() {
		m.RecoveryLatency = m.RecoveryCompletedAt.Sub(m.FaultObservedAt)
	}
}

// AggregateMetrics calculates statistical percentiles (p50, p95, p99, min, max, avg) across repetitions.
type AggregateMetrics struct {
	ScenarioID            string        `json:"scenario_id"`
	Iterations            int           `json:"iterations"`
	PassCount             int           `json:"pass_count"`
	FailCount             int           `json:"fail_count"`
	DetectionP50          time.Duration `json:"detection_p50"`
	DetectionP95          time.Duration `json:"detection_p95"`
	DetectionP99          time.Duration `json:"detection_p99"`
	RecoveryP50           time.Duration `json:"recovery_p50"`
	RecoveryP95           time.Duration `json:"recovery_p95"`
	RecoveryP99           time.Duration `json:"recovery_p99"`
	MaxLostWorkSteps      int64         `json:"max_lost_work_steps"`
	TotalAuthorityViolations int        `json:"total_authority_violations"`
	TotalDuplicateExecutions int        `json:"total_duplicate_executions"`
}

// ComputeAggregateMetrics aggregates N experiment repetitions.
func ComputeAggregateMetrics(scenarioID string, runs []ExperimentMetrics) AggregateMetrics {
	if len(runs) == 0 {
		return AggregateMetrics{ScenarioID: scenarioID}
	}

	detList := make([]time.Duration, 0, len(runs))
	recList := make([]time.Duration, 0, len(runs))
	var passCount, failCount int
	var maxLost int64
	var totalAuthViolations, totalDupExecs int

	for _, r := range runs {
		if r.Outcome == "PASS" {
			passCount++
		} else {
			failCount++
		}
		if r.DetectionLatency > 0 {
			detList = append(detList, r.DetectionLatency)
		}
		if r.RecoveryLatency > 0 {
			recList = append(recList, r.RecoveryLatency)
		}
		if r.LostWorkSteps > maxLost {
			maxLost = r.LostWorkSteps
		}
		totalAuthViolations += r.AuthorityViolations
		totalDupExecs += r.DuplicateExecutions
	}

	sort.Slice(detList, func(i, j int) bool { return detList[i] < detList[j] })
	sort.Slice(recList, func(i, j int) bool { return recList[i] < recList[j] })

	return AggregateMetrics{
		ScenarioID:               scenarioID,
		Iterations:               len(runs),
		PassCount:                passCount,
		FailCount:                failCount,
		DetectionP50:             percentileDuration(detList, 0.50),
		DetectionP95:             percentileDuration(detList, 0.95),
		DetectionP99:             percentileDuration(detList, 0.99),
		RecoveryP50:              percentileDuration(recList, 0.50),
		RecoveryP95:              percentileDuration(recList, 0.95),
		RecoveryP99:              percentileDuration(recList, 0.99),
		MaxLostWorkSteps:         maxLost,
		TotalAuthorityViolations: totalAuthViolations,
		TotalDuplicateExecutions: totalDupExecs,
	}
}

func percentileDuration(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// GenerateChaosValidationReport produces a human-readable table report of all chaos experiments.
func GenerateChaosValidationReport(results []ExperimentMetrics, aggregates []AggregateMetrics) string {
	var sb strings.Builder
	sb.WriteString("========================================================================================\n")
	sb.WriteString("                   SENTINELMESH CHAOS EXPERIMENT VALIDATION REPORT                      \n")
	sb.WriteString("========================================================================================\n\n")

	sb.WriteString(fmt.Sprintf("%-6s | %-24s | %-10s | %-12s | %-12s | %-8s | %-6s\n",
		"SCEN", "FAULT CLASS", "OUTCOME", "DETECT (P50)", "RECOV (P50)", "LOST WK", "AUTH OK"))
	sb.WriteString(strings.Repeat("-", 90) + "\n")

	for _, a := range aggregates {
		authOK := "YES"
		if a.TotalAuthorityViolations > 0 {
			authOK = "NO"
		}
		outcome := "PASS"
		if a.FailCount > 0 {
			outcome = "FAIL"
		}

		sb.WriteString(fmt.Sprintf("%-6s | %-24s | %-10s | %-12s | %-12s | %-8d | %-6s\n",
			a.ScenarioID,
			lookupScenarioClass(a.ScenarioID),
			fmt.Sprintf("%s (%d/%d)", outcome, a.PassCount, a.Iterations),
			formatDur(a.DetectionP50),
			formatDur(a.RecoveryP50),
			a.MaxLostWorkSteps,
			authOK,
		))
	}

	sb.WriteString(strings.Repeat("-", 90) + "\n\n")
	sb.WriteString("AGGREGATE INVARIANT SUMMARY:\n")
	var totalRuns, totalPass, totalAuth, totalDups int
	for _, a := range aggregates {
		totalRuns += a.Iterations
		totalPass += a.PassCount
		totalAuth += a.TotalAuthorityViolations
		totalDups += a.TotalDuplicateExecutions
	}
	sb.WriteString(fmt.Sprintf(" - Total Experiments Executed:  %d\n", totalRuns))
	sb.WriteString(fmt.Sprintf(" - Total Scenarios Passing:    %d/%d (%.1f%%)\n", totalPass, totalRuns, float64(totalPass)/float64(totalRuns)*100))
	sb.WriteString(fmt.Sprintf(" - Authority Violations:       %d (Must be 0)\n", totalAuth))
	sb.WriteString(fmt.Sprintf(" - Stale Duplicate Placements: %d (Must be 0)\n", totalDups))
	sb.WriteString("========================================================================================\n")

	return sb.String()
}

func lookupScenarioClass(id string) string {
	switch id {
	case "F01":
		return "Pod failure"
	case "F02":
		return "Container crash"
	case "F03":
		return "Node failure"
	case "F04":
		return "Cluster unreachable"
	case "F05":
		return "NATS pub transient loss"
	case "F06":
		return "NATS message delay"
	case "F07":
		return "Duplicate NATS message"
	case "F08":
		return "PostgreSQL write error"
	case "F09":
		return "Corrupted checksum"
	case "F10":
		return "Truncated state JSON"
	case "F11":
		return "Scheduler crash"
	case "F12":
		return "Stale-gen reconnect fence"
	case "F13":
		return "Cascading compound fault"
	default:
		return "Custom fault"
	}
}

func formatDur(d time.Duration) string {
	if d == 0 {
		return "N/A"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
