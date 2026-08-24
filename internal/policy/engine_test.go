package policy

import (
	"testing"
	"time"
)

func TestPolicyEngine_Filesystem_TraversalDefense(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name     string
		profile  ProfileName
		op       string
		path     string
		wantDec  Decision
		severity Severity
	}{
		{
			name:    "allowed path in workspace",
			profile: ProfileStandard,
			op:      "file:read",
			path:    "/workspace/dataset.parquet",
			wantDec: DecisionAllow,
		},
		{
			name:     "direct access to /etc/shadow",
			profile:  ProfileStandard,
			op:       "file:read",
			path:     "/etc/shadow",
			wantDec:  DecisionDeny,
			severity: SeverityHigh,
		},
		{
			name:     "path traversal attack via ../etc/shadow",
			profile:  ProfileStandard,
			op:       "file:read",
			path:     "/workspace/../etc/shadow",
			wantDec:  DecisionDeny,
			severity: SeverityHigh,
		},
		{
			name:     "complex path traversal attack via ./../../root/.ssh/id_rsa",
			profile:  ProfileStandard,
			op:       "file:read",
			path:     "/workspace/./../../root/.ssh/id_rsa",
			wantDec:  DecisionDeny,
			severity: SeverityHigh,
		},
		{
			name:     "read-only violation writing to /bin/bash",
			profile:  ProfileStandard,
			op:       "file:write",
			path:     "/bin/bash",
			wantDec:  DecisionDeny,
			severity: SeverityHigh,
		},
		{
			name:    "allowed writing to /workspace/output.txt",
			profile: ProfileStandard,
			op:      "file:write",
			path:    "/workspace/output.txt",
			wantDec: DecisionAllow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := engine.Evaluate(EvaluationRequest{
				Profile:   tt.profile,
				Operation: tt.op,
				Resource:  tt.path,
			})

			if res.Decision != tt.wantDec {
				t.Errorf("expected decision %s for path %q, got %s (reason: %s)",
					tt.wantDec, tt.path, res.Decision, res.Reason)
			}
			if tt.severity != "" && res.Severity != tt.severity {
				t.Errorf("expected severity %s, got %s", tt.severity, res.Severity)
			}
		})
	}
}

func TestPolicyEngine_Network_CIDREvaluation(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name    string
		profile ProfileName
		target  string
		wantDec Decision
	}{
		{
			name:    "Standard profile: outbound internet allowed",
			profile: ProfileStandard,
			target:  "142.250.190.46:443", // Google IP
			wantDec: DecisionAllow,
		},
		{
			name:    "Standard profile: cloud metadata service blocked",
			profile: ProfileStandard,
			target:  "169.254.169.254:80",
			wantDec: DecisionDeny,
		},
		{
			name:    "Restricted profile: internal VPC database allowed",
			profile: ProfileRestricted,
			target:  "10.0.1.50:5432",
			wantDec: DecisionAllow,
		},
		{
			name:    "Restricted profile: external internet egress denied",
			profile: ProfileRestricted,
			target:  "142.250.190.46:443",
			wantDec: DecisionDeny,
		},
		{
			name:    "Confidential profile: zero egress by default",
			profile: ProfileConfidential,
			target:  "10.0.1.50:5432",
			wantDec: DecisionDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := engine.Evaluate(EvaluationRequest{
				Profile:   tt.profile,
				Operation: "net:egress",
				Resource:  tt.target,
			})

			if res.Decision != tt.wantDec {
				t.Errorf("expected decision %s for target %q, got %s (reason: %s)",
					tt.wantDec, tt.target, res.Decision, res.Reason)
			}
		})
	}
}

func TestPolicyEngine_SyscallAndTools(t *testing.T) {
	engine := NewEngine()

	// 1. Syscalls
	resSys := engine.Evaluate(EvaluationRequest{
		Profile:   ProfileStandard,
		Operation: "syscall:exec",
		Resource:  "ptrace",
	})
	if resSys.Decision != DecisionDeny {
		t.Errorf("expected ptrace to be denied, got %s", resSys.Decision)
	}

	resSysSafe := engine.Evaluate(EvaluationRequest{
		Profile:   ProfileStandard,
		Operation: "syscall:exec",
		Resource:  "read",
	})
	if resSysSafe.Decision != DecisionAllow {
		t.Errorf("expected read syscall to be allowed, got %s", resSysSafe.Decision)
	}

	// 2. Tools
	resTool := engine.Evaluate(EvaluationRequest{
		Profile:   ProfileRestricted,
		Operation: "tool:call",
		Resource:  "system_reboot",
	})
	if resTool.Decision != DecisionDeny {
		t.Errorf("expected system_reboot tool to be denied, got %s", resTool.Decision)
	}

	resToolAllowed := engine.Evaluate(EvaluationRequest{
		Profile:   ProfileRestricted,
		Operation: "tool:call",
		Resource:  "read_file",
	})
	if resToolAllowed.Decision != DecisionAllow {
		t.Errorf("expected read_file tool to be allowed, got %s", resToolAllowed.Decision)
	}
}

func TestPolicyEngine_LatencyBenchmarks(t *testing.T) {
	engine := NewEngine()

	req := EvaluationRequest{
		Profile:   ProfileStandard,
		Operation: "file:read",
		Resource:  "/workspace/project/file.go",
	}

	for i := 0; i < 1000; i++ {
		_ = engine.Evaluate(req)
	}

	metrics := engine.GetMetrics()
	if metrics.TotalEvaluations != 1000 {
		t.Errorf("expected 1000 evaluations, got %d", metrics.TotalEvaluations)
	}
	if metrics.AllowedCount != 1000 {
		t.Errorf("expected 1000 allowed, got %d", metrics.AllowedCount)
	}

	// P99 should be well within sub-millisecond range (< 1ms)
	if metrics.P99Latency > 2*time.Millisecond {
		t.Errorf("expected P99 latency < 2ms, got %v", metrics.P99Latency)
	}
}
