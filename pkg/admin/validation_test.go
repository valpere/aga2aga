package admin_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/admin"
)

func TestIsValidAgentID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		// Valid: letters and digits
		{"a1", true},
		{"agent1", true},
		{"AGENT1", true},
		{"Agent-Alpha", true},
		{"agent.beta", true},
		{"agent_gamma", true},

		// Valid: mixed separators in middle
		{"my-agent.v2_final", true},
		{"a-b", true},
		{"x.y", true},
		{"a_b", true},

		// Valid: exactly 2 chars (both must be alphanumeric)
		{"ab", true},

		// Invalid: 2-char strings ending with separator
		{"a-", false},
		{"a.", false},
		{"a_", false},

		// Valid: exactly 64 chars (max)
		{"a" + repeat62("b") + "c", true},

		// Invalid: too short (1 char)
		{"a", false},

		// Invalid: too long (65 chars)
		{"a" + repeat63("b") + "c", false},

		// Invalid: starts with separator
		{"-agent", false},
		{".agent", false},
		{"_agent", false},

		// Invalid: ends with separator
		{"agent-", false},
		{"agent.", false},
		{"agent_", false},

		// Invalid: empty
		{"", false},

		// Invalid: contains illegal characters
		{"agent name", false},  // space
		{"agent/path", false},  // slash
		{"agent\nid", false},   // newline
		{"agent\x00id", false}, // null byte
		{"agent:id", false},    // colon
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := admin.IsValidAgentID(tt.id)
			if got != tt.want {
				t.Errorf("IsValidAgentID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func repeat62(s string) string { return repeatN(s, 62) }
func repeat63(s string) string { return repeatN(s, 63) }

func repeatN(s string, n int) string {
	out := make([]byte, n*len(s))
	for i := range n {
		copy(out[i*len(s):], s)
	}
	return string(out)
}
