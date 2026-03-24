// Package document — lifecycle state machine.
//
// DO_NOT_TOUCH: lifecycle transition table — spec §16 — modifying breaks wire compatibility.
package document

// LifecycleState is a named string type for agent lifecycle states.
type LifecycleState string

// Agent lifecycle states from spec §16.
const (
	StateProposed          LifecycleState = "proposed"
	StateApprovedForSandbox LifecycleState = "approved_for_sandbox"
	StateSandbox           LifecycleState = "sandbox"
	StateCandidate         LifecycleState = "candidate"
	StateActive            LifecycleState = "active"
	StateInactive          LifecycleState = "inactive"
	StateRetired           LifecycleState = "retired"
	StateRejected          LifecycleState = "rejected"
	StateFailedSandbox     LifecycleState = "failed_sandbox"
	StateRolledBack        LifecycleState = "rolled_back"
	StateQuarantined       LifecycleState = "quarantined"
)

// DO_NOT_TOUCH: transitionTable encodes all valid lifecycle transitions from spec §16.
// Modifying this table breaks wire compatibility between agents and orchestrators.
// Terminal states (retired, rejected, failed_sandbox, rolled_back) have no entries.
var transitionTable = map[LifecycleState][]LifecycleState{
	StateProposed:           {StateApprovedForSandbox, StateRejected},
	StateApprovedForSandbox: {StateSandbox},
	StateSandbox:            {StateCandidate, StateFailedSandbox},
	StateCandidate:          {StateActive, StateRolledBack},
	StateActive:             {StateInactive, StateQuarantined, StateRetired},
	StateInactive:           {StateActive, StateRetired},
	StateQuarantined:        {StateRetired},
}

// ValidTransition reports whether transitioning from → to is permitted by spec §16.
// DO_NOT_TOUCH: signature and table-lookup implementation — required for auditability.
func ValidTransition(from, to LifecycleState) bool {
	allowed, ok := transitionTable[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// AllowedTransitions returns all valid next states from the given state.
// Returns nil for terminal states (no outgoing transitions).
func AllowedTransitions(from LifecycleState) []LifecycleState {
	return transitionTable[from]
}
