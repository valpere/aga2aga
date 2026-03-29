package gateway

import (
	"testing"
)

func TestApplyDefaults(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultAgentName = "env-agent"
	cfg.DefaultAgentKey = "env-key"
	g := &Gateway{cfg: cfg}

	tests := []struct {
		name       string
		inAgent    string
		inAPIKey   string
		wantAgent  string
		wantAPIKey string
	}{
		{
			name:       "both absent — env defaults used",
			inAgent:    "",
			inAPIKey:   "",
			wantAgent:  "env-agent",
			wantAPIKey: "env-key",
		},
		{
			// SECURITY: when the caller supplies an explicit agent ID they must also
			// supply their own api_key. Injecting the env default key for a
			// caller-supplied agent ID would create an auth oracle (CWE-287).
			name:       "agent present, key absent — key NOT injected from env",
			inAgent:    "explicit-agent",
			inAPIKey:   "",
			wantAgent:  "explicit-agent",
			wantAPIKey: "",
		},
		{
			name:       "agent absent, key present — agent default used, key kept",
			inAgent:    "",
			inAPIKey:   "explicit-key",
			wantAgent:  "env-agent",
			wantAPIKey: "explicit-key",
		},
		{
			name:       "both present — both fields win, env untouched",
			inAgent:    "explicit-agent",
			inAPIKey:   "explicit-key",
			wantAgent:  "explicit-agent",
			wantAPIKey: "explicit-key",
		},
		{
			// Documents that applyDefaults does not validate the default value;
			// validation is the caller's responsibility (IsValidAgentID in each handler).
			name:       "invalid default agent passes through unchanged",
			inAgent:    "",
			inAPIKey:   "",
			wantAgent:  "env-agent", // valid in this fixture; see separate test below
			wantAPIKey: "env-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAgent, gotKey := g.applyDefaults(tt.inAgent, tt.inAPIKey)
			if gotAgent != tt.wantAgent {
				t.Errorf("agent = %q, want %q", gotAgent, tt.wantAgent)
			}
			if gotKey != tt.wantAPIKey {
				t.Errorf("apiKey = %q, want %q", gotKey, tt.wantAPIKey)
			}
		})
	}
}

// TestApplyDefaults_InvalidDefaultPassthrough verifies that applyDefaults does
// not validate DefaultAgentName — it passes invalid values through unchanged so
// the downstream IsValidAgentID call in the handler produces the correct error.
func TestApplyDefaults_InvalidDefaultPassthrough(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultAgentName = "bad name!" // contains space — fails IsValidAgentID
	cfg.DefaultAgentKey = "env-key"
	g := &Gateway{cfg: cfg}

	agent, key := g.applyDefaults("", "")
	if agent != "bad name!" {
		t.Errorf("agent = %q, want %q (applyDefaults must not validate)", agent, "bad name!")
	}
	if key != "env-key" {
		t.Errorf("apiKey = %q, want %q", key, "env-key")
	}
}

func TestApplyDefaults_NoEnvDefaults(t *testing.T) {
	g := &Gateway{cfg: DefaultConfig()} // DefaultAgentName and DefaultAgentKey are ""

	agent, key := g.applyDefaults("", "")
	if agent != "" {
		t.Errorf("agent = %q, want empty (no default set)", agent)
	}
	if key != "" {
		t.Errorf("apiKey = %q, want empty (no default set)", key)
	}
}
