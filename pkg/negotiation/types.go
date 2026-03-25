package negotiation

// NegotiationState is the current phase of a negotiation session.
// Values correspond to the negotiation.* message types in pkg/protocol.
type NegotiationState string

const (
	StatePropose  NegotiationState = "negotiation.propose"
	StateAccept   NegotiationState = "negotiation.accept"
	StateReject   NegotiationState = "negotiation.reject"
	StateCounter  NegotiationState = "negotiation.counter"
	StateClarify  NegotiationState = "negotiation.clarify"
	StateDelegate NegotiationState = "negotiation.delegate"
	StateCommit   NegotiationState = "negotiation.commit"
	StateAbort    NegotiationState = "negotiation.abort"
)

// NegotiationTransition reports whether transitioning from → to is valid.
// Full state-machine logic is implemented in Phase 4; this stub always
// returns false to satisfy interface consumers during Phase 1.
func NegotiationTransition(from, to NegotiationState) bool {
	return false
}
