package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrVerificationFailed = errors.New("verification: one or more outcome rules failed")
	ErrAttestationNotFound = errors.New("verification: attestation record not found")
	ErrCorruptedAttestation = errors.New("verification: evidence digest mismatch / record corrupted")
)

type AttestationStatus string

const (
	StatusVerified AttestationStatus = "VERIFIED"
	StatusFailed   AttestationStatus = "FAILED"
	StatusError    AttestationStatus = "ERROR"
)

type RuleEvaluationStatus string

const (
	RulePass    RuleEvaluationStatus = "PASS"
	RuleFail    RuleEvaluationStatus = "FAIL"
	RuleError   RuleEvaluationStatus = "ERROR"
	RuleSkipped RuleEvaluationStatus = "SKIPPED"
)

// RuleEvaluation holds individual check forensic evidence.
type RuleEvaluation struct {
	RuleID         string               `json:"rule_id"`
	RuleType       string               `json:"rule_type"`
	Status         RuleEvaluationStatus `json:"status"`
	Reason         string               `json:"reason"`
	EvaluatedValue string               `json:"evaluated_value,omitempty"`
	ExpectedValue  string               `json:"expected_value,omitempty"`
	DurationNs     int64                `json:"duration_ns"`
}

// AttestationRecord is a cryptographically hashed evidence certificate of execution reality.
type AttestationRecord struct {
	ID             string            `json:"id"`
	RunID          string            `json:"run_id"`
	AgentID        string            `json:"agent_id"`
	TenantID       string            `json:"tenant_id"`
	Status         AttestationStatus `json:"status"`
	EvidenceDigest string            `json:"evidence_digest"`
	Evaluations    []RuleEvaluation  `json:"evaluations"`
	AttestedAt     time.Time         `json:"attested_at"`
	CreatedAt      time.Time         `json:"created_at"`
}

// ComputeEvidenceDigest calculates a deterministic SHA-256 hash over all evaluations.
func ComputeEvidenceDigest(evaluations []RuleEvaluation) string {
	raw, err := json.Marshal(evaluations)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

// VerifyDigest confirms the attestation record has not been tampered with.
func (r *AttestationRecord) VerifyDigest() bool {
	return r.EvidenceDigest == ComputeEvidenceDigest(r.Evaluations)
}
