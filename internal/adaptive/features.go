package adaptive

import (
	"strings"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// ExtractFeatures analyzes a RoutingRequest and produces compact, normalized TaskFeatures.
func ExtractFeatures(req router.RoutingRequest) TaskFeatures {
	taskClass := ClassifyTaskIntent(req.Prompt, req.TaskComplexity)
	inBucket := GetTokenBucket(req.EstimatedInputTokens)
	outBucket := GetTokenBucket(req.EstimatedOutputTokens)

	return TaskFeatures{
		TaskID:             req.TaskID,
		TaskClass:          taskClass,
		Complexity:         req.TaskComplexity,
		InputTokens:        req.EstimatedInputTokens,
		OutputTokens:       req.EstimatedOutputTokens,
		InBucket:           inBucket,
		OutBucket:          outBucket,
		QualityRequirement: req.QualityRequirement,
		SecurityProfile:    req.SecurityProfile,
	}
}

// ComputeFeatureKey constructs a FeatureKey for a given model and request.
func ComputeFeatureKey(req router.RoutingRequest, modelID string) FeatureKey {
	features := ExtractFeatures(req)
	return FeatureKey{
		ModelID:           modelID,
		TaskClass:         features.TaskClass,
		Complexity:        features.Complexity,
		InputTokenBucket:  features.InBucket,
		OutputTokenBucket: features.OutBucket,
	}
}

// GetTokenBucket categorizes token counts into discrete bins.
func GetTokenBucket(tokens int) TokenBucket {
	switch {
	case tokens < 1000:
		return BucketSmall
	case tokens < 8000:
		return BucketMedium
	case tokens < 32000:
		return BucketLarge
	default:
		return BucketExtreme
	}
}

// ClassifyTaskIntent extracts intent from keywords and complexity hints.
func ClassifyTaskIntent(prompt string, complexity router.TaskComplexity) TaskClass {
	lower := strings.ToLower(prompt)

	switch {
	case strings.Contains(lower, "extract") || strings.Contains(lower, "json") || strings.Contains(lower, "parse") || strings.Contains(lower, "schema"):
		return ClassExtraction
	case strings.Contains(lower, "code") || strings.Contains(lower, "function") || strings.Contains(lower, "refactor") || strings.Contains(lower, "implement"):
		return ClassCodeGen
	case strings.Contains(lower, "summarize") || strings.Contains(lower, "summary") || strings.Contains(lower, "overview") || strings.Contains(lower, "brief"):
		return ClassSummarization
	case complexity == router.ComplexityReasoningHeavy || strings.Contains(lower, "proof") || strings.Contains(lower, "theorem") || strings.Contains(lower, "verify") || strings.Contains(lower, "consensus"):
		return ClassReasoning
	default:
		return ClassGeneral
	}
}
