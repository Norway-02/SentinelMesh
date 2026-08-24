package verification

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// EvaluateHTTPHealthRule sends an HTTP probe to test service endpoints.
func EvaluateHTTPHealthRule(ctx context.Context, httpClient *http.Client, rule types.HTTPHealthRule) RuleEvaluation {
	start := time.Now()
	eval := RuleEvaluation{
		RuleID:        rule.ID,
		RuleType:      "http_health",
		ExpectedValue: fmt.Sprintf("status=%d, substring=%s", rule.ExpectedStatus, rule.ExpectedBodySubstring),
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	method := rule.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, rule.URL, nil)
	if err != nil {
		eval.Status = RuleError
		eval.Reason = fmt.Sprintf("failed to create http request: %v", err)
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		eval.Status = RuleFail
		eval.Reason = fmt.Sprintf("http probe failed to connect: %v", err)
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}
	defer resp.Body.Close()

	eval.EvaluatedValue = fmt.Sprintf("status=%d", resp.StatusCode)

	if rule.ExpectedStatus > 0 && resp.StatusCode != rule.ExpectedStatus {
		eval.Status = RuleFail
		eval.Reason = fmt.Sprintf("http probe status %d did not match expected %d", resp.StatusCode, rule.ExpectedStatus)
		eval.DurationNs = time.Since(start).Nanoseconds()
		return eval
	}

	if rule.ExpectedBodySubstring != "" {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		if !strings.Contains(bodyStr, rule.ExpectedBodySubstring) {
			eval.Status = RuleFail
			eval.Reason = fmt.Sprintf("http probe response body missing required substring: %s", rule.ExpectedBodySubstring)
			eval.DurationNs = time.Since(start).Nanoseconds()
			return eval
		}
	}

	eval.Status = RulePass
	eval.Reason = fmt.Sprintf("http probe %s returned expected status %d", rule.URL, resp.StatusCode)
	eval.DurationNs = time.Since(start).Nanoseconds()
	return eval
}
