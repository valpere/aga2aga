package admin_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/admin"
)

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name     string
		policies []admin.CommunicationPolicy
		source   string
		target   string
		want     admin.PolicyAction
	}{
		{
			name:     "no policies: deny by default",
			policies: nil,
			source:   "agent-alpha",
			target:   "agent-beta",
			want:     admin.PolicyActionDeny,
		},
		{
			name: "explicit allow",
			policies: []admin.CommunicationPolicy{
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow, Priority: 10},
			},
			source: "agent-alpha",
			target: "agent-beta",
			want:   admin.PolicyActionAllow,
		},
		{
			name: "explicit deny",
			policies: []admin.CommunicationPolicy{
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionDeny, Priority: 10},
			},
			source: "agent-alpha",
			target: "agent-beta",
			want:   admin.PolicyActionDeny,
		},
		{
			name: "higher priority deny overrides lower priority allow",
			policies: []admin.CommunicationPolicy{
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow, Priority: 5},
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionDeny, Priority: 20},
			},
			source: "agent-alpha",
			target: "agent-beta",
			want:   admin.PolicyActionDeny,
		},
		{
			name: "higher priority allow overrides lower priority deny",
			policies: []admin.CommunicationPolicy{
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionDeny, Priority: 5},
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow, Priority: 20},
			},
			source: "agent-alpha",
			target: "agent-beta",
			want:   admin.PolicyActionAllow,
		},
		{
			name: "wildcard source matches any agent",
			policies: []admin.CommunicationPolicy{
				{SourceID: admin.Wildcard, TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow, Priority: 10},
			},
			source: "agent-unknown",
			target: "agent-beta",
			want:   admin.PolicyActionAllow,
		},
		{
			name: "wildcard target matches any agent",
			policies: []admin.CommunicationPolicy{
				{SourceID: "agent-alpha", TargetID: admin.Wildcard, Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow, Priority: 10},
			},
			source: "agent-alpha",
			target: "agent-unknown",
			want:   admin.PolicyActionAllow,
		},
		{
			name: "bidirectional policy applies to reverse direction",
			policies: []admin.CommunicationPolicy{
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionBidirectional, Action: admin.PolicyActionAllow, Priority: 10},
			},
			source: "agent-beta",
			target: "agent-alpha",
			want:   admin.PolicyActionAllow,
		},
		{
			name: "unidirectional policy does not apply to reverse direction",
			policies: []admin.CommunicationPolicy{
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow, Priority: 10},
			},
			source: "agent-beta",
			target: "agent-alpha",
			want:   admin.PolicyActionDeny, // reverse not covered → default deny
		},
		{
			name: "specific rule overrides wildcard at same priority — higher priority wins",
			policies: []admin.CommunicationPolicy{
				{SourceID: admin.Wildcard, TargetID: admin.Wildcard, Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionDeny, Priority: 5},
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow, Priority: 20},
			},
			source: "agent-alpha",
			target: "agent-beta",
			want:   admin.PolicyActionAllow,
		},
		{
			name: "global deny-all wildcard at highest priority blocks specific allow",
			policies: []admin.CommunicationPolicy{
				{SourceID: "agent-alpha", TargetID: "agent-beta", Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionAllow, Priority: 5},
				{SourceID: admin.Wildcard, TargetID: admin.Wildcard, Direction: admin.DirectionUnidirectional, Action: admin.PolicyActionDeny, Priority: 100},
			},
			source: "agent-alpha",
			target: "agent-beta",
			want:   admin.PolicyActionDeny,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := admin.Evaluate(tc.policies, tc.source, tc.target)
			if got != tc.want {
				t.Errorf("Evaluate(%q → %q) = %q, want %q", tc.source, tc.target, got, tc.want)
			}
		})
	}
}
