// Package protocol defines the canonical message types, constants, and registry
// for the aga2aga envelope protocol.
package protocol

// ProtocolVersion is the wire protocol version. DO_NOT_TOUCH.
const ProtocolVersion = "v1"

// MessageType is a named string type for type-safe message identification.
type MessageType string

// Agent evolution message types (11).
const (
	AgentGenome            MessageType = "agent.genome"
	AgentSpawnProposal     MessageType = "agent.spawn.proposal"
	AgentSpawnApproval     MessageType = "agent.spawn.approval"
	AgentSpawnRejection    MessageType = "agent.spawn.rejection"
	AgentEvaluationRequest MessageType = "agent.evaluation.request"
	AgentEvaluationResult  MessageType = "agent.evaluation.result"
	AgentPromotion         MessageType = "agent.promotion"
	AgentRollback          MessageType = "agent.rollback"
	AgentRetirement        MessageType = "agent.retirement"
	AgentQuarantine        MessageType = "agent.quarantine"
	AgentRecombineProposal MessageType = "agent.recombine.proposal"
)

// Agent message types (1).
// General-purpose peer-to-peer communication — fire-and-forget, no outcome required.
// The base message kind: every envelope document is a message; tasks are a specialised subtype.
const (
	AgentMessage MessageType = "agent.message"
)

// Task message types (4).
// Request-response work units — the sender expects an explicit outcome delivered as a separate
// message: task.result (success + body) or task.fail (failure + reason).
const (
	TaskRequest  MessageType = "task.request"
	TaskResult   MessageType = "task.result"
	TaskFail     MessageType = "task.fail"
	TaskProgress MessageType = "task.progress"
)

// Negotiation message types (8).
const (
	NegotiationPropose  MessageType = "negotiation.propose"
	NegotiationAccept   MessageType = "negotiation.accept"
	NegotiationReject   MessageType = "negotiation.reject"
	NegotiationCounter  MessageType = "negotiation.counter"
	NegotiationClarify  MessageType = "negotiation.clarify"
	NegotiationDelegate MessageType = "negotiation.delegate"
	NegotiationCommit   MessageType = "negotiation.commit"
	NegotiationAbort    MessageType = "negotiation.abort"
)

// TypeMeta describes a message type's required fields and JSON Schema reference.
type TypeMeta struct {
	// RequiredFields lists fields required beyond the base envelope (type, version).
	RequiredFields []string
	// SchemaRef is the $defs name in the JSON Schema companion, empty if none.
	SchemaRef string
}
