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
		name        string
		inAgent     string
		inAPIKey    string
		wantAgent   string
		wantAPIKey  string
	}{
		{
			name:       "both absent — env defaults used",
			inAgent:    "",
			inAPIKey:   "",
			wantAgent:  "env-agent",
			wantAPIKey: "env-key",
		},
		{
			name:       "agent present — field wins over env",
			inAgent:    "explicit-agent",
			inAPIKey:   "",
			wantAgent:  "explicit-agent",
			wantAPIKey: "env-key",
		},
		{
			name:       "api_key present — field wins over env",
			inAgent:    "",
			inAPIKey:   "explicit-key",
			wantAgent:  "env-agent",
			wantAPIKey: "explicit-key",
		},
		{
			name:       "both present — both fields win",
			inAgent:    "explicit-agent",
			inAPIKey:   "explicit-key",
			wantAgent:  "explicit-agent",
			wantAPIKey: "explicit-key",
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
