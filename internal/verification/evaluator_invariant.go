package verification

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// EvaluateInvariantRule asserts metric thresholds and values.
func EvaluateInvariantRule(metrics map[string]string, rule types.InvariantRule) RuleEvaluation {
	start := time.Now()
	eval := RuleEvaluation{
		RuleID:        rule.ID,
		RuleType:      "invariant",
		ExpectedValue: fmt.Sprintf("%s %s %s", rule.MetricName, rule.Operator, rule.ExpectedValue),
	}

	val, exists := metrics[rule.MetricName]
	if !exists {
		eval.Status = RuleFail
		eval.Reason = fmt.Sprintf("metric '%s' not reported by workload", rule.MetricName)
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	eval.EvaluatedValue = val

	switch rule.Operator {
	case "eq", "==":
		if val != rule.ExpectedValue {
			eval.Status = RuleFail
			eval.Reason = fmt.Sprintf("metric %s (%s) != expected %s", rule.MetricName, val, rule.ExpectedValue)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}
	case "neq", "!=":
		if val == rule.ExpectedValue {
			eval.Status = RuleFail
			eval.Reason = fmt.Sprintf("metric %s (%s) == forbidden %s", rule.MetricName, val, rule.ExpectedValue)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}
	case "gt", "gte", "lt", "lte":
		fVal, err1 := strconv.ParseFloat(val, 64)
		fExp, err2 := strconv.ParseFloat(rule.ExpectedValue, 64)
		if err1 != nil || err2 != nil {
			eval.Status = RuleError
			eval.Reason = fmt.Sprintf("numeric comparison failed: %v, %v", err1, err2)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}
		passed := false
		if rule.Operator == "gt" && fVal > fExp { passed = true }
		if rule.Operator == "gte" && fVal >= fExp { passed = true }
		if rule.Operator == "lt" && fVal < fExp { passed = true }
		if rule.Operator == "lte" && fVal <= fExp { passed = true }
		if !passed {
			eval.Status = RuleFail
			eval.Reason = fmt.Sprintf("metric %s (%f) failed %s %f", rule.MetricName, fVal, rule.Operator, fExp)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}
	case "matches":
		matched, err := regexp.MatchString(rule.ExpectedValue, val)
		if err != nil || !matched {
			eval.Status = RuleFail
			eval.Reason = fmt.Sprintf("metric %s (%s) does not match regex %s", rule.MetricName, val, rule.ExpectedValue)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}
	default:
		eval.Status = RuleError
		eval.Reason = fmt.Sprintf("unknown invariant operator: %s", rule.Operator)
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	eval.Status = RulePass
	eval.Reason = fmt.Sprintf("metric %s satisfied invariant constraint", rule.MetricName)
	eval.DurationNs = time.Since(start).Nanoseconds()
	return eval
}
