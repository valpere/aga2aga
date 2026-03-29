package admin_test

import (
	"strings"
	"testing"

	"github.com/valpere/aga2aga/pkg/admin"
)

func TestIsValidAgentID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		// Valid: letters and digits
		{"letter+digit", "a1", true},
		{"all lowercase", "agent1", true},
		{"all uppercase", "AGENT1", true},
		{"mixed case with hyphen", "Agent-Alpha", true},
		{"dot separator", "agent.beta", true},
		{"underscore separator", "agent_gamma", true},

		// Valid: mixed separators in middle
		{"mixed separators", "my-agent.v2_final", true},
		{"hyphen only", "a-b", true},
		{"dot only", "x.y", true},
		{"underscore only", "a_b", true},

		// Valid: length boundaries
		{"exactly 2 chars letters", "ab", true},
		{"exactly 2 chars digit-first", "1a", true},
		{"exactly 63 chars", "a" + strings.Repeat("b", 61) + "c", true},
		{"exactly 64 chars (max)", "a" + strings.Repeat("b", 62) + "c", true},

		// Invalid: 2-char strings ending with separator
		{"2 chars ending hyphen", "a-", false},
		{"2 chars ending dot", "a.", false},
		{"2 chars ending underscore", "a_", false},

		// Invalid: too short
		{"1 char", "a", false},

		// Invalid: too long
		{"65 chars", "a" + strings.Repeat("b", 63) + "c", false},

		// Invalid: starts with separator
		{"starts with hyphen", "-agent", false},
		{"starts with dot", ".agent", false},
		{"starts with underscore", "_agent", false},

		// Invalid: ends with separator
		{"ends with hyphen", "agent-", false},
		{"ends with dot", "agent.", false},
		{"ends with underscore", "agent_", false},

		// Invalid: empty
		{"empty", "", false},

		// Invalid: illegal characters (CWE-20 / CWE-74 injection chars)
		{"contains space", "agent name", false},
		{"contains slash", "agent/path", false},
		{"contains newline", "agent\nid", false},
		{"contains null byte", "agent\x00id", false},
		{"contains colon", "agent:id", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := admin.IsValidAgentID(tt.id)
			if got != tt.want {
				t.Errorf("IsValidAgentID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
