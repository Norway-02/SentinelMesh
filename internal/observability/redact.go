package observability

import (
	"regexp"
	"strings"
)

var (
	bearerPattern = regexp.MustCompile(`(?i)Bearer\s+sk-[A-Za-z0-9\-_]+`)
	keyPattern    = regexp.MustCompile(`(?i)sk-[A-Za-z0-9\-_]{20,}`)
)

// RedactSecrets replaces any OpenAI API key patterns in input strings with [REDACTED_API_KEY]
func RedactSecrets(input string) string {
	if input == "" {
		return ""
	}
	out := bearerPattern.ReplaceAllString(input, "Bearer [REDACTED_API_KEY]")
	out = keyPattern.ReplaceAllString(out, "[REDACTED_API_KEY]")
	return out
}

// RedactHeader returns a safe copy of HTTP headers with Authorization/API keys masked
func RedactHeaderKey(key, value string) string {
	keyLower := strings.ToLower(key)
	if strings.Contains(keyLower, "authorization") || strings.Contains(keyLower, "key") || strings.Contains(keyLower, "secret") || strings.Contains(keyLower, "token") {
		return "[REDACTED]"
	}
	return RedactSecrets(value)
}
