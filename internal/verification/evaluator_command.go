package verification

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// EvaluateCommandRule executes isolated test commands and checks exit code.
func EvaluateCommandRule(ctx context.Context, rule types.CommandRule) RuleEvaluation {
	start := time.Now()
	eval := RuleEvaluation{
		RuleID:        rule.ID,
		RuleType:      "command",
		ExpectedValue: fmt.Sprintf("cmd=%s, exit_code=%d", rule.Command, rule.ExpectedExitCode),
	}

	timeout := 10 * time.Second
	if rule.TimeoutSeconds > 0 {
		timeout = time.Duration(rule.TimeoutSeconds) * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, rule.Command, rule.Args...)
	if rule.WorkingDir != "" {
		cmd.Dir = rule.WorkingDir
	}

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			eval.Status = RuleError
			eval.Reason = fmt.Sprintf("failed to execute verification command: %v", err)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}
	}

	eval.EvaluatedValue = fmt.Sprintf("exit_code=%d, output_len=%d", exitCode, len(output))

	if exitCode != rule.ExpectedExitCode {
		eval.Status = RuleFail
		eval.Reason = fmt.Sprintf("command exit code %d did not match expected %d: %s", exitCode, rule.ExpectedExitCode, string(output))
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	eval.Status = RulePass
	eval.Reason = "verification command executed successfully with expected exit code"
	eval.DurationNs = time.Since(start).Nanoseconds()
	return eval
}
