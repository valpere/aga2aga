package admin

import "regexp"

// agentIDPattern is the canonical agent identifier format — see IsValidAgentID.
var agentIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}[a-zA-Z0-9]$`) //nolint:gochecknoglobals

// IsValidAgentID reports whether s satisfies the agent ID naming rules:
//   - 2–64 characters
//   - starts and ends with a letter or digit
//   - middle characters: letters, digits, '.', '_', '-'
//
// This is the canonical implementation; both the admin server and the gateway
// import and call this function to avoid pattern drift.
//
// SECURITY: the pattern prevents Redis stream-name injection via newlines, null
// bytes, or path separators in agent IDs (CWE-20 / CWE-74).
func IsValidAgentID(s string) bool {
	return agentIDPattern.MatchString(s)
}
