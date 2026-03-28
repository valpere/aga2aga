package admin_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/admin"
)

func TestRoleAgent_Constant(t *testing.T) {
	if admin.RoleAgent != admin.Role("agent") {
		t.Fatalf("RoleAgent = %q, want %q", admin.RoleAgent, "agent")
	}
}

func TestAPIKey_AgentIDField(t *testing.T) {
	k := admin.APIKey{AgentID: "my-agent"}
	if k.AgentID != "my-agent" {
		t.Fatalf("AgentID = %q, want %q", k.AgentID, "my-agent")
	}
}
