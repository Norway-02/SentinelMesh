package policy

import (
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/observability"
)

// Engine is the pure deterministic policy evaluator for SentinelMesh.
// It evaluates intent requests against configured SecurityProfiles without
// any side-effects, external dependencies, or runtime couplings.
type Engine struct {
	mu          sync.RWMutex
	profiles    map[ProfileName]SecurityProfile
	latencies   []time.Duration
	totalCount  int64
	allowCount  int64
	denyCount   int64
	apprvCount  int64
	maxSamples  int
}

// NewEngine constructs a new Policy Engine with standard security profiles.
func NewEngine() *Engine {
	return &Engine{
		profiles:   DefaultProfiles(),
		latencies:  make([]time.Duration, 0, 10000),
		maxSamples: 10000,
	}
}

// RegisterProfile adds or updates a SecurityProfile.
func (e *Engine) RegisterProfile(p SecurityProfile) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.profiles[p.Name] = p
}

// GetProfile retrieves a security profile by name.
func (e *Engine) GetProfile(name ProfileName) (SecurityProfile, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	p, ok := e.profiles[name]
	return p, ok
}

// Evaluate performs deterministic evaluation of an EvaluationRequest.
func (e *Engine) Evaluate(req EvaluationRequest) EvaluationResult {
	start := time.Now()

	e.mu.RLock()
	profile, exists := e.profiles[req.Profile]
	if !exists {
		profile = e.profiles[ProfileStandard]
	}
	e.mu.RUnlock()

	var res EvaluationResult
	res.Timestamp = start

	parts := strings.Split(req.Operation, ":")
	domain := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch domain {
	case "file":
		res = e.evaluateFilesystem(profile, action, req.Resource)
	case "net":
		res = e.evaluateNetwork(profile, action, req.Resource)
	case "syscall":
		res = e.evaluateSyscall(profile, req.Resource)
	case "tool":
		res = e.evaluateTool(profile, req.Resource)
	default:
		res = EvaluationResult{
			Decision:      DecisionDeny,
			RuleID:        "unknown-domain-deny",
			Reason:        fmt.Sprintf("unknown operation domain %q", domain),
			Severity:      SeverityHigh,
			AuditRequired: true,
		}
	}

	dur := time.Since(start)
	res.Duration = dur

	// Record performance metrics thread-safely
	e.recordMetrics(res.Decision, dur)

	// Record global Prometheus metrics with low cardinality
	m := observability.GetMetrics()
	profileStr := string(req.Profile)
	if profileStr == "" {
		profileStr = "standard"
	}
	m.PolicyEvaluationsTotal.WithLabelValues(profileStr, domain).Inc()
	m.PolicyEvaluationDurationSec.WithLabelValues(profileStr).Observe(dur.Seconds())

	if res.Decision == DecisionDeny {
		m.PolicyDenialsTotal.WithLabelValues(profileStr, req.Operation, res.RuleID).Inc()
		m.SecurityViolationsTotal.WithLabelValues(profileStr, string(res.Severity)).Inc()
	}

	return res
}

func (e *Engine) evaluateFilesystem(p SecurityProfile, action, rawPath string) EvaluationResult {
	if rawPath == "" {
		return EvaluationResult{
			Decision:      DecisionDeny,
			RuleID:        "fs-empty-path",
			Reason:        "empty filesystem path provided",
			Severity:      SeverityMedium,
			AuditRequired: true,
		}
	}

	// 1. Path Normalization & Canonicalization (Traversal Prevention)
	cleanPath := filepath.Clean(rawPath)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = "/" + cleanPath
		cleanPath = filepath.Clean(cleanPath)
	}

	fs := p.Filesystem

	// 2. Check explicitly Denied paths (glob/prefix match)
	for _, denied := range fs.DeniedPaths {
		if pathMatches(denied, cleanPath) {
			return EvaluationResult{
				Decision:      DecisionDeny,
				RuleID:        "fs-denied-path",
				Reason:        fmt.Sprintf("access to path %q matches explicit deny rule %q", cleanPath, denied),
				Severity:      SeverityHigh,
				AuditRequired: true,
			}
		}
	}

	// 3. If action is write/modify, check ReadOnlyPaths
	if action == "write" || action == "delete" || action == "create" {
		for _, ro := range fs.ReadOnlyPaths {
			if pathMatches(ro, cleanPath) {
				return EvaluationResult{
					Decision:      DecisionDeny,
					RuleID:        "fs-readonly-violation",
					Reason:        fmt.Sprintf("path %q is in read-only location %q", cleanPath, ro),
					Severity:      SeverityHigh,
					AuditRequired: true,
				}
			}
		}
	}

	// 4. Check Allowed paths (whitelist)
	for _, allowed := range fs.AllowedPaths {
		if pathMatches(allowed, cleanPath) {
			return EvaluationResult{
				Decision:      DecisionAllow,
				RuleID:        "fs-allowed-path",
				Reason:        fmt.Sprintf("path %q is within allowed boundary %q", cleanPath, allowed),
				Severity:      SeverityLow,
				AuditRequired: false,
			}
		}
	}

	// 5. Default Deny if not within any allowed path
	return EvaluationResult{
		Decision:      DecisionDeny,
		RuleID:        "fs-default-deny",
		Reason:        fmt.Sprintf("path %q is outside all allowed paths for profile %q", cleanPath, p.Name),
		Severity:      SeverityMedium,
		AuditRequired: true,
	}
}

func (e *Engine) evaluateNetwork(p SecurityProfile, action, target string) EvaluationResult {
	if p.Network.AllowAll {
		return EvaluationResult{
			Decision: DecisionAllow,
			RuleID:   "net-allow-all",
			Reason:   "network allow_all is enabled",
			Severity: SeverityLow,
		}
	}

	host, portStr, err := net.SplitHostPort(target)
	var port int
	if err != nil {
		host = target
	} else {
		port, _ = strconv.Atoi(portStr)
	}

	ip := net.ParseIP(host)

	// 1. Check Denied CIDRs
	if ip != nil {
		for _, deniedCIDR := range p.Network.DeniedCIDRs {
			_, subnet, err := net.ParseCIDR(deniedCIDR)
			if err == nil && subnet.Contains(ip) {
				return EvaluationResult{
					Decision:      DecisionDeny,
					RuleID:        "net-denied-cidr",
					Reason:        fmt.Sprintf("target IP %s is within denied CIDR %s", host, deniedCIDR),
					Severity:      SeverityHigh,
					AuditRequired: true,
				}
			}
		}
	}

	// 2. Check Denied Ports
	if port > 0 {
		for _, dp := range p.Network.DeniedPorts {
			if dp == port {
				return EvaluationResult{
					Decision:      DecisionDeny,
					RuleID:        "net-denied-port",
					Reason:        fmt.Sprintf("egress to port %d is denied", port),
					Severity:      SeverityHigh,
					AuditRequired: true,
				}
			}
		}
	}

	// 3. Check Allowed CIDRs
	allowed := false
	if ip != nil {
		for _, allowedCIDR := range p.Network.AllowedCIDRs {
			_, subnet, err := net.ParseCIDR(allowedCIDR)
			if err == nil && subnet.Contains(ip) {
				allowed = true
				break
			}
		}
	}

	// 4. Check Allowed Ports if configured
	if allowed && len(p.Network.AllowedPorts) > 0 && port > 0 {
		portAllowed := false
		for _, ap := range p.Network.AllowedPorts {
			if ap == port {
				portAllowed = true
				break
			}
		}
		if !portAllowed {
			return EvaluationResult{
				Decision:      DecisionDeny,
				RuleID:        "net-port-not-whitelisted",
				Reason:        fmt.Sprintf("port %d is not in allowed ports list", port),
				Severity:      SeverityMedium,
				AuditRequired: true,
			}
		}
	}

	if allowed {
		return EvaluationResult{
			Decision: DecisionAllow,
			RuleID:   "net-allowed-cidr",
			Reason:   fmt.Sprintf("target %s is allowed under profile %s", target, p.Name),
			Severity: SeverityLow,
		}
	}

	return EvaluationResult{
		Decision:      DecisionDeny,
		RuleID:        "net-default-deny",
		Reason:        fmt.Sprintf("egress to target %s is not permitted under profile %s", target, p.Name),
		Severity:      SeverityHigh,
		AuditRequired: true,
	}
}

func (e *Engine) evaluateSyscall(p SecurityProfile, syscallName string) EvaluationResult {
	for _, denied := range p.Syscalls.DeniedSyscalls {
		if strings.EqualFold(denied, syscallName) {
			return EvaluationResult{
				Decision:      DecisionDeny,
				RuleID:        "syscall-denied",
				Reason:        fmt.Sprintf("syscall %q is blocked by profile %s", syscallName, p.Name),
				Severity:      SeverityCritical,
				AuditRequired: true,
			}
		}
	}

	return EvaluationResult{
		Decision: DecisionAllow,
		RuleID:   "syscall-allowed",
		Reason:   fmt.Sprintf("syscall %q permitted by %s seccomp profile", syscallName, p.Syscalls.SeccompType),
		Severity: SeverityLow,
	}
}

func (e *Engine) evaluateTool(p SecurityProfile, toolName string) EvaluationResult {
	for _, denied := range p.Tools.DeniedTools {
		if denied == "*" || strings.EqualFold(denied, toolName) {
			return EvaluationResult{
				Decision:      DecisionDeny,
				RuleID:        "tool-denied",
				Reason:        fmt.Sprintf("tool %q is denied for profile %s", toolName, p.Name),
				Severity:      SeverityHigh,
				AuditRequired: true,
			}
		}
	}

	for _, allowed := range p.Tools.AllowedTools {
		if allowed == "*" || strings.EqualFold(allowed, toolName) {
			return EvaluationResult{
				Decision: DecisionAllow,
				RuleID:   "tool-allowed",
				Reason:   fmt.Sprintf("tool %q is permitted", toolName),
				Severity: SeverityLow,
			}
		}
	}

	return EvaluationResult{
		Decision:      DecisionRequireApproval,
		RuleID:        "tool-approval-required",
		Reason:        fmt.Sprintf("tool %q requires human supervisor approval", toolName),
		Severity:      SeverityMedium,
		AuditRequired: true,
	}
}

// pathMatches checks if candidate path matches pattern (supporting prefix and glob **).
func pathMatches(pattern, candidate string) bool {
	candidate = filepath.Clean(candidate)

	if pattern == "/**" || pattern == "*" {
		return true
	}

	// Prefix directory matching (e.g. /etc/** matches /etc/shadow or /etc/pam.d/other)
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		prefix = filepath.Clean(prefix)
		if prefix == "" || prefix == "/" {
			return true
		}
		return candidate == prefix || strings.HasPrefix(candidate, prefix+"/")
	}

	cleanPattern := filepath.Clean(pattern)
	if cleanPattern == candidate {
		return true
	}

	// Strict prefix boundary check (e.g. /workspace matches /workspace/data.csv)
	if strings.HasPrefix(candidate, cleanPattern+"/") {
		return true
	}

	// Standard glob match
	matched, err := filepath.Match(pattern, candidate)
	return err == nil && matched
}

func (e *Engine) recordMetrics(decision Decision, d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.totalCount++
	switch decision {
	case DecisionAllow:
		e.allowCount++
	case DecisionDeny:
		e.denyCount++
	case DecisionRequireApproval:
		e.apprvCount++
	}

	if len(e.latencies) < e.maxSamples {
		e.latencies = append(e.latencies, d)
	} else {
		// Replace random sample to maintain rolling window
		idx := e.totalCount % int64(e.maxSamples)
		e.latencies[idx] = d
	}
}

// GetMetrics returns p50, p95, p99 evaluation latency and decision counts.
func (e *Engine) GetMetrics() PolicyMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	metrics := PolicyMetrics{
		TotalEvaluations: e.totalCount,
		AllowedCount:     e.allowCount,
		DeniedCount:      e.denyCount,
		ApprovalCount:    e.apprvCount,
	}

	if len(e.latencies) == 0 {
		return metrics
	}

	sorted := make([]time.Duration, len(e.latencies))
	copy(sorted, e.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	n := len(sorted)
	metrics.P50Latency = sorted[int(float64(n)*0.50)]
	metrics.P95Latency = sorted[int(float64(n)*0.95)]
	metrics.P99Latency = sorted[int(float64(n)*0.99)]

	return metrics
}
