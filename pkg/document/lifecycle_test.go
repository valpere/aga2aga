package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
)

// TestLifecycleState_Constants verifies all 11 state constants are defined.
func TestLifecycleState_Constants(t *testing.T) {
	states := []document.LifecycleState{
		document.StateProposed,
		document.StateApprovedForSandbox,
		document.StateSandbox,
		document.StateCandidate,
		document.StateActive,
		document.StateInactive,
		document.StateRetired,
		document.StateRejected,
		document.StateFailedSandbox,
		document.StateRolledBack,
		document.StateQuarantined,
	}
	if len(states) != 11 {
		t.Fatalf("expected 11 lifecycle states, got %d", len(states))
	}
}

// TestValidTransition_ValidPairs verifies every spec §16 valid transition returns true.
func TestValidTransition_ValidPairs(t *testing.T) {
	tests := []struct {
		from document.LifecycleState
		to   document.LifecycleState
	}{
		{document.StateProposed, document.StateApprovedForSandbox},
		{document.StateProposed, document.StateRejected},
		{document.StateApprovedForSandbox, document.StateSandbox},
		{document.StateSandbox, document.StateCandidate},
		{document.StateSandbox, document.StateFailedSandbox},
		{document.StateCandidate, document.StateActive},
		{document.StateCandidate, document.StateRolledBack},
		{document.StateActive, document.StateInactive},
		{document.StateActive, document.StateQuarantined},
		{document.StateActive, document.StateRetired},
		{document.StateInactive, document.StateActive},
		{document.StateInactive, document.StateRetired},
		{document.StateQuarantined, document.StateRetired},
	}
	for _, tc := range tests {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			if !document.ValidTransition(tc.from, tc.to) {
				t.Errorf("ValidTransition(%q, %q) = false, want true", tc.from, tc.to)
			}
		})
	}
}

// TestValidTransition_InvalidPairs verifies representative invalid transitions return false.
func TestValidTransition_InvalidPairs(t *testing.T) {
	tests := []struct {
		name string
		from document.LifecycleState
		to   document.LifecycleState
	}{
		{"proposed cannot go to active", document.StateProposed, document.StateActive},
		{"proposed cannot go to sandbox", document.StateProposed, document.StateSandbox},
		{"active cannot go to proposed", document.StateActive, document.StateProposed},
		{"active cannot go to candidate", document.StateActive, document.StateCandidate},
		{"retired is terminal", document.StateRetired, document.StateActive},
		{"rejected is terminal", document.StateRejected, document.StateProposed},
		{"failed_sandbox is terminal", document.StateFailedSandbox, document.StateSandbox},
		{"rolled_back is terminal", document.StateRolledBack, document.StateCandidate},
		{"sandbox cannot skip to active", document.StateSandbox, document.StateActive},
		{"quarantined cannot go to active", document.StateQuarantined, document.StateActive},
		{"self-transition proposed", document.StateProposed, document.StateProposed},
		{"self-transition active", document.StateActive, document.StateActive},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if document.ValidTransition(tc.from, tc.to) {
				t.Errorf("ValidTransition(%q, %q) = true, want false", tc.from, tc.to)
			}
		})
	}
}

// TestAllowedTransitions verifies each state returns the correct next states.
func TestAllowedTransitions(t *testing.T) {
	tests := []struct {
		from document.LifecycleState
		want []document.LifecycleState
	}{
		{
			document.StateProposed,
			[]document.LifecycleState{document.StateApprovedForSandbox, document.StateRejected},
		},
		{
			document.StateApprovedForSandbox,
			[]document.LifecycleState{document.StateSandbox},
		},
		{
			document.StateSandbox,
			[]document.LifecycleState{document.StateCandidate, document.StateFailedSandbox},
		},
		{
			document.StateCandidate,
			[]document.LifecycleState{document.StateActive, document.StateRolledBack},
		},
		{
			document.StateActive,
			[]document.LifecycleState{document.StateInactive, document.StateQuarantined, document.StateRetired},
		},
		{
			document.StateInactive,
			[]document.LifecycleState{document.StateActive, document.StateRetired},
		},
		{
			document.StateQuarantined,
			[]document.LifecycleState{document.StateRetired},
		},
		{document.StateRetired, nil},
		{document.StateRejected, nil},
		{document.StateFailedSandbox, nil},
		{document.StateRolledBack, nil},
	}
	for _, tc := range tests {
		t.Run(string(tc.from), func(t *testing.T) {
			got := document.AllowedTransitions(tc.from)
			if len(got) != len(tc.want) {
				t.Fatalf("AllowedTransitions(%q) = %v (len %d), want %v (len %d)",
					tc.from, got, len(got), tc.want, len(tc.want))
			}
			// Build a set of got for order-independent comparison.
			gotSet := make(map[document.LifecycleState]bool, len(got))
			for _, s := range got {
				gotSet[s] = true
			}
			for _, w := range tc.want {
				if !gotSet[w] {
					t.Errorf("AllowedTransitions(%q): missing %q in result %v", tc.from, w, got)
				}
			}
		})
	}
}
