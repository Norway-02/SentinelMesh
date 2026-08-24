package policy

import (
	"time"
)

// Decision represents the policy verdict on an operation.
type Decision string

const (
	DecisionAllow           Decision = "ALLOW"
	DecisionDeny            Decision = "DENY"
	DecisionRequireApproval Decision = "REQUIRE_APPROVAL"
)

// Severity indicates the risk level of a policy violation.
type Severity string

const (
	SeverityLow      Severity = "LOW"
	SeverityMedium   Severity = "MEDIUM"
	SeverityHigh     Severity = "HIGH"
	SeverityCritical Severity = "CRITICAL"
)

// ProfileName identifies standard composable security profiles.
type ProfileName string

const (
	ProfileStandard     ProfileName = "standard"
	ProfileRestricted   ProfileName = "restricted"
	ProfileConfidential ProfileName = "confidential"
)

// FilesystemPolicy configures path-based read/write rules.
type FilesystemPolicy struct {
	AllowedPaths  []string `json:"allowed_paths"`
	DeniedPaths   []string `json:"denied_paths"`
	ReadOnlyPaths []string `json:"read_only_paths"`
	AllowRootFS   bool     `json:"allow_root_fs"`
}

// NetworkPolicy configures IP, CIDR, and port egress restrictions.
type NetworkPolicy struct {
	AllowedCIDRs []string `json:"allowed_cidrs"`
	DeniedCIDRs  []string `json:"denied_cidrs"`
	AllowedPorts []int    `json:"allowed_ports"`
	DeniedPorts  []int    `json:"denied_ports"`
	AllowAll     bool     `json:"allow_all"`
}

// SyscallPolicy defines system call containment and seccomp profile mode.
type SyscallPolicy struct {
	SeccompType    string   `json:"seccomp_type"` // e.g. "RuntimeDefault", "Custom"
	AllowedSyscalls []string `json:"allowed_syscalls"`
	DeniedSyscalls  []string `json:"denied_syscalls"`
}

// ResourcePolicy specifies resource quotas and containment boundaries.
type ResourcePolicy struct {
	MaxCPUCores   float64 `json:"max_cpu_cores"`
	MaxMemoryMB   int64   `json:"max_memory_mb"`
	MaxPIDs       int     `json:"max_pids"`
	AllowGPUAccess bool   `json:"allow_gpu_access"`
}

// ToolPolicy restricts authorized tools and operations.
type ToolPolicy struct {
	AllowedTools []string `json:"allowed_tools"`
	DeniedTools  []string `json:"denied_tools"`
}

// SecurityProfile is a composable policy bundle encapsulating all security domains.
type SecurityProfile struct {
	Name       ProfileName      `json:"name"`
	Filesystem FilesystemPolicy `json:"filesystem"`
	Network    NetworkPolicy    `json:"network"`
	Syscalls   SyscallPolicy    `json:"syscalls"`
	Resources  ResourcePolicy   `json:"resources"`
	Tools      ToolPolicy       `json:"tools"`
}

// DefaultProfiles returns standard composable security profiles.
func DefaultProfiles() map[ProfileName]SecurityProfile {
	return map[ProfileName]SecurityProfile{
		ProfileStandard: {
			Name: ProfileStandard,
			Filesystem: FilesystemPolicy{
				AllowedPaths:  []string{"/workspace", "/tmp", "/var/tmp"},
				DeniedPaths:   []string{"/etc/shadow", "/etc/sudoers", "/root", "/var/run/docker.sock"},
				ReadOnlyPaths: []string{"/bin", "/usr", "/lib", "/etc"},
				AllowRootFS:   false,
			},
			Network: NetworkPolicy{
				AllowedCIDRs: []string{"0.0.0.0/0"}, // Outbound allowed to internet
				DeniedCIDRs:  []string{"169.254.169.254/32", "10.0.0.0/8"}, // Block cloud metadata & internal VPC
				AllowedPorts: []int{80, 443, 8080},
				AllowAll:     false,
			},
			Syscalls: SyscallPolicy{
				SeccompType:    "RuntimeDefault",
				DeniedSyscalls:  []string{"ptrace", "bpf", "reboot", "kexec_load", "mount"},
			},
			Resources: ResourcePolicy{
				MaxCPUCores:   2.0,
				MaxMemoryMB:   2048,
				MaxPIDs:       1024,
				AllowGPUAccess: false,
			},
			Tools: ToolPolicy{
				AllowedTools: []string{"*"},
			},
		},
		ProfileRestricted: {
			Name: ProfileRestricted,
			Filesystem: FilesystemPolicy{
				AllowedPaths:  []string{"/workspace"},
				DeniedPaths:   []string{"/etc/**", "/root/**", "/proc/kcore", "/var/run/**"},
				ReadOnlyPaths: []string{"/bin", "/usr", "/lib"},
				AllowRootFS:   false,
			},
			Network: NetworkPolicy{
				AllowedCIDRs: []string{"10.0.0.0/16"}, // Internal VPC only (default deny for others)
				DeniedCIDRs:  []string{"169.254.169.254/32", "10.0.0.0/24"}, // Block metadata & sensitive subnet
				AllowedPorts: []int{443, 5432, 4222},
				AllowAll:     false,
			},
			Syscalls: SyscallPolicy{
				SeccompType:    "RuntimeDefault",
				DeniedSyscalls:  []string{"ptrace", "bpf", "reboot", "mount", "chroot", "init_module"},
			},
			Resources: ResourcePolicy{
				MaxCPUCores:   1.0,
				MaxMemoryMB:   1024,
				MaxPIDs:       256,
				AllowGPUAccess: false,
			},
			Tools: ToolPolicy{
				AllowedTools: []string{"read_file", "write_file", "code_exec"},
				DeniedTools:  []string{"raw_socket", "system_reboot"},
			},
		},
		ProfileConfidential: {
			Name: ProfileConfidential,
			Filesystem: FilesystemPolicy{
				AllowedPaths:  []string{"/workspace/secure"},
				DeniedPaths:   []string{"/**"}, // Everything outside /workspace/secure is denied
				ReadOnlyPaths: []string{"/bin", "/lib"},
				AllowRootFS:   false,
			},
			Network: NetworkPolicy{
				AllowedCIDRs: []string{}, // Zero egress allowed
				DeniedCIDRs:  []string{},
				AllowedPorts: []int{},
				AllowAll:     false,
			},
			Syscalls: SyscallPolicy{
				SeccompType:    "RuntimeDefault",
				DeniedSyscalls:  []string{"ptrace", "bpf", "reboot", "mount", "setuid", "setgid", "socket"},
			},
			Resources: ResourcePolicy{
				MaxCPUCores:   0.5,
				MaxMemoryMB:   512,
				MaxPIDs:       64,
				AllowGPUAccess: false,
			},
			Tools: ToolPolicy{
				AllowedTools: []string{"secure_compute"},
				DeniedTools:  []string{"*"},
			},
		},
	}
}

// EvaluationRequest specifies the intent tuple to be evaluated by the PolicyEngine.
// Notice: pure domain/intent definition with NO infrastructure types.
type EvaluationRequest struct {
	RunID         string      `json:"run_id"`
	AgentID       string      `json:"agent_id"`
	TenantID      string      `json:"tenant_id"`
	CorrelationID string      `json:"correlation_id"`
	Profile       ProfileName `json:"profile"`
	Operation     string      `json:"operation"` // e.g. "file:read", "file:write", "net:egress", "syscall:exec", "tool:call"
	Resource      string      `json:"resource"`  // e.g. "/etc/shadow", "10.0.0.1:443", "ptrace", "bash"
}

// EvaluationResult contains the deterministic policy verdict and audit details.
type EvaluationResult struct {
	Decision      Decision      `json:"decision"`
	RuleID        string        `json:"rule_id"`
	Reason        string        `json:"reason"`
	Severity      Severity      `json:"severity"`
	AuditRequired bool          `json:"audit_required"`
	Duration      time.Duration `json:"duration"`
	Timestamp     time.Time     `json:"timestamp"`
}

// PolicyMetrics summarizes evaluation performance and counts.
type PolicyMetrics struct {
	TotalEvaluations int64         `json:"total_evaluations"`
	AllowedCount     int64         `json:"allowed_count"`
	DeniedCount      int64         `json:"denied_count"`
	ApprovalCount    int64         `json:"approval_count"`
	P50Latency       time.Duration `json:"p50_latency"`
	P95Latency       time.Duration `json:"p95_latency"`
	P99Latency       time.Duration `json:"p99_latency"`
}
