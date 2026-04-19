package debuglog

import (
	"regexp"
	"strings"
)

var secretPattern = regexp.MustCompile(`(?i)"?(session_ingress_token|environment_secret|access_token|authorization|secret|token)"?\s*:\s*"([^"]*)"`)

func RedactSecrets(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	return secretPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := secretPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		field := parts[1]
		secret := parts[2]
		if len(secret) <= 12 {
			return `"` + field + `":"[REDACTED]"`
		}
		return `"` + field + `":"` + secret[:6] + `...` + secret[len(secret)-4:] + `"`
	})
}

func Truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}
