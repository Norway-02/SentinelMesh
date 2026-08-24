package events

// NATS Subject Definitions
const (
	SubjectAgentCreated = "sentinel.agent.v1.created"
	SubjectAgentDeleted = "sentinel.agent.v1.deleted"

	SubjectRunCreated          = "sentinel.run.v1.created"
	SubjectRunStateChanged     = "sentinel.run.v1.state_changed"
	SubjectRunScheduled        = "sentinel.run.v1.scheduled"
	SubjectRunSchedulingFailed = "sentinel.run.v1.scheduling_failed"
	SubjectRunExecutionFenced  = "sentinel.run.v1.execution_fenced"

	SubjectSecurityPolicyViolation  = "sentinel.security.v1.policy_violation"
	SubjectSecuritySandboxViolation = "sentinel.security.v1.sandbox_violation"

	SubjectCheckpointSaved         = "sentinel.checkpoint.v1.saved"
	SubjectClusterNodeFailed       = "sentinel.cluster.v1.node_failed"
	SubjectClusterUnreachable      = "sentinel.cluster.v1.unreachable"
	SubjectClusterHeartbeat        = "sentinel.cluster.v1.heartbeat"
	SubjectRunRecoveryRequested    = "sentinel.run.v1.recovery_requested"
	SubjectRunRecovered            = "sentinel.run.v1.recovered"
	SubjectRunRecoveryFailed       = "sentinel.run.v1.recovery_failed"

	SubjectRunVerificationRequested = "sentinel.run.v1.verification_requested"
	SubjectRunVerified              = "sentinel.run.v1.verified"
	SubjectRunVerificationFailed    = "sentinel.run.v1.verification_failed"

	SubjectModelRoutingDecided       = "sentinel.router.v1.decided"
	SubjectModelInvocationCompleted = "sentinel.router.v1.invocation_completed"
	SubjectModelInvocationFailed    = "sentinel.router.v1.invocation_failed"
	SubjectModelFallbackTriggered   = "sentinel.router.v1.fallback_triggered"

	SubjectAdaptiveRoutingDecided          = "sentinel.adaptive.v1.decided"
	SubjectModelPerformanceDriftDetected = "sentinel.adaptive.v1.drift_detected"

	SubjectOnlinePolicyDecided      = "sentinel.policy.v2.decided"
	SubjectPolicyRollbackTriggered = "sentinel.policy.v2.rollback_triggered"
	SubjectShadowPolicyEvaluated   = "sentinel.policy.v2.shadow_evaluated"
)

// SubjectRunScheduledForCluster returns the cluster-targeted NATS subject for workload dispatch.
func SubjectRunScheduledForCluster(clusterID string) string {
	if clusterID == "" {
		return SubjectRunScheduled
	}
	return SubjectRunScheduled + "." + clusterID
}
