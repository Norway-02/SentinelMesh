package verification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// EvaluateArtifactRule inspects filesystem or blob artifacts.
func EvaluateArtifactRule(ctx context.Context, rule types.ArtifactRule) RuleEvaluation {
	start := time.Now()
	eval := RuleEvaluation{
		RuleID:        rule.ID,
		RuleType:      "artifact",
		ExpectedValue: fmt.Sprintf("path=%s, min_size=%d", rule.Path, rule.MinSizeBytes),
	}

	info, err := os.Stat(rule.Path)
	if err != nil {
		if rule.Required || os.IsNotExist(err) {
			eval.Status = RuleFail
			eval.Reason = fmt.Sprintf("artifact not found at path: %s", rule.Path)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}
	}

	eval.EvaluatedValue = fmt.Sprintf("size=%d", info.Size())

	if rule.MinSizeBytes > 0 && info.Size() < rule.MinSizeBytes {
		eval.Status = RuleFail
		eval.Reason = fmt.Sprintf("artifact size %d bytes is below required %d bytes", info.Size(), rule.MinSizeBytes)
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	if rule.ExpectedChecksum != "" || rule.SchemaJSON != "" {
		data, err := os.ReadFile(rule.Path)
		if err != nil {
			eval.Status = RuleError
			eval.Reason = fmt.Sprintf("failed to read artifact data: %v", err)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}

		if rule.ExpectedChecksum != "" {
			hash := sha256.Sum256(data)
			computedChecksum := hex.EncodeToString(hash[:])
			expected := strings.TrimPrefix(rule.ExpectedChecksum, "sha256:")
			if computedChecksum != expected {
				eval.Status = RuleFail
				eval.Reason = fmt.Sprintf("checksum mismatch: expected %s, computed %s", expected, computedChecksum)
				eval.DurationNs = time.Since(start).Nanoseconds()
				return eval
			}
		}

		if rule.SchemaJSON != "" {
			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err != nil {
				eval.Status = RuleFail
				eval.Reason = fmt.Sprintf("artifact JSON schema validation failed: invalid JSON: %v", err)
				eval.DurationNs = time.Since(start).Nanoseconds()
				return eval
			}

			var expectedKeys []string
			if err := json.Unmarshal([]byte(rule.SchemaJSON), &expectedKeys); err == nil {
				for _, key := range expectedKeys {
					if _, exists := parsed[key]; !exists {
						eval.Status = RuleFail
						eval.Reason = fmt.Sprintf("artifact missing required schema key: %s", key)
						eval.DurationNs = time.Since(start).Nanoseconds()
						return eval
					}
				}
			}
		}
	}

	eval.Status = RulePass
	eval.Reason = "artifact exists and satisfies all size, checksum, and schema constraints"
	eval.DurationNs = time.Since(start).Nanoseconds()
	return eval
}
