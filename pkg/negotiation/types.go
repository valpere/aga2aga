package negotiation

import "github.com/valpere/aga2aga/pkg/protocol"

// NegotiationState is the current phase of a negotiation session.
// Values are derived from pkg/protocol MessageType constants to prevent
// silent divergence if the protocol registry is ever updated.
type NegotiationState string

const (
	StatePropose  NegotiationState = NegotiationState(protocol.NegotiationPropose)
	StateAccept   NegotiationState = NegotiationState(protocol.NegotiationAccept)
	StateReject   NegotiationState = NegotiationState(protocol.NegotiationReject)
	StateCounter  NegotiationState = NegotiationState(protocol.NegotiationCounter)
	StateClarify  NegotiationState = NegotiationState(protocol.NegotiationClarify)
	StateDelegate NegotiationState = NegotiationState(protocol.NegotiationDelegate)
	StateCommit   NegotiationState = NegotiationState(protocol.NegotiationCommit)
	StateAbort    NegotiationState = NegotiationState(protocol.NegotiationAbort)
)

// NegotiationTransition reports whether transitioning from → to is valid.
// Full state-machine logic is implemented in Phase 4.
//
// STUB: always returns false for every input, including identity transitions
// (from == to). Do NOT use this function as a gate or guard decision before
// Phase 4 is implemented — every call will silently reject valid transitions.
func NegotiationTransition(from, to NegotiationState) bool {
	return false
}
