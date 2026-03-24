package protocol

// baseEnvelopeFields are required on every message per the base envelope schema.
// Kept unexported to prevent external mutation; use BaseEnvelopeFields() for access.
var baseEnvelopeFields = [2]string{"type", "version"} //nolint:gochecknoglobals

// BaseEnvelopeFields returns a copy of the fields required on every message.
// The returned slice is independent — callers cannot affect the authoritative list.
func BaseEnvelopeFields() []string {
	s := baseEnvelopeFields
	return s[:]
}

// registry is the authoritative map of MessageType to TypeMeta.
// Kept unexported to prevent external mutation; use Lookup() and Registered() for access.
var registry = map[MessageType]TypeMeta{ //nolint:gochecknoglobals
	// Agent evolution (11)
	AgentGenome: {
		RequiredFields: []string{
			"agent_id", "kind", "status", "identity", "capabilities",
			"tools", "model_policy", "prompt_policy", "routing_policy",
			"thresholds", "constraints", "mutation_policy",
		},
		SchemaRef: "agentGenome",
	},
	AgentSpawnProposal: {
		RequiredFields: []string{"id", "from", "candidate_id", "parent_ids", "spawn_reason"},
		SchemaRef:      "agentSpawnProposal",
	},
	AgentSpawnApproval: {
		RequiredFields: []string{"id", "from", "in_reply_to", "candidate_id", "decision"},
		SchemaRef:      "agentSpawnApproval",
	},
	AgentSpawnRejection: {
		RequiredFields: []string{"id", "from", "in_reply_to", "candidate_id", "decision"},
		SchemaRef:      "agentSpawnRejection",
	},
	AgentEvaluationRequest: {
		RequiredFields: []string{"id", "from", "target_agent", "benchmark_id", "mode"},
		SchemaRef:      "agentEvaluationRequest",
	},
	AgentEvaluationResult: {
		RequiredFields: []string{"id", "from", "target_agent", "overall_decision", "metrics", "hard_constraints_passed"},
		SchemaRef:      "agentEvaluationResult",
	},
	AgentPromotion: {
		RequiredFields: []string{"id", "from", "target_agent", "from_status", "to_status"},
		SchemaRef:      "agentPromotion",
	},
	AgentRollback: {
		RequiredFields: []string{"id", "from", "target_agent", "from_status", "to_status"},
		SchemaRef:      "agentRollback",
	},
	AgentRetirement: {
		RequiredFields: []string{"id", "from", "target_agent", "reason"},
		SchemaRef:      "agentRetirement",
	},
	AgentQuarantine: {
		RequiredFields: []string{"id", "from", "target_agent", "reason"},
		SchemaRef:      "agentQuarantine",
	},
	AgentRecombineProposal: {
		RequiredFields: []string{"id", "from", "candidate_id", "parent_ids", "goal"},
		SchemaRef:      "agentRecombineProposal",
	},

	// Task (5)
	TaskRequest: {
		RequiredFields: []string{"id", "from", "to", "exec_id", "step"},
		SchemaRef:      "",
	},
	TaskResult: {
		RequiredFields: []string{"id", "from", "to", "exec_id", "step"},
		SchemaRef:      "",
	},
	TaskFail: {
		RequiredFields: []string{"id", "from", "to", "exec_id", "step"},
		SchemaRef:      "",
	},
	TaskProgress: {
		RequiredFields: []string{"id", "from", "to", "exec_id", "step"},
		SchemaRef:      "",
	},
	// agent.message: id required for deduplication and replay protection.
	AgentMessage: {
		RequiredFields: []string{"id", "from", "to"},
		SchemaRef:      "",
	},

	// Negotiation (8)
	// negotiation.propose starts a thread — no in_reply_to required.
	NegotiationPropose: {
		RequiredFields: []string{"id", "from", "to", "thread_id"},
		SchemaRef:      "",
	},
	// All subsequent negotiation types require in_reply_to.
	NegotiationAccept: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
		SchemaRef:      "",
	},
	NegotiationReject: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
		SchemaRef:      "",
	},
	NegotiationCounter: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
		SchemaRef:      "",
	},
	NegotiationClarify: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
		SchemaRef:      "",
	},
	NegotiationDelegate: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
		SchemaRef:      "",
	},
	NegotiationCommit: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
		SchemaRef:      "",
	},
	NegotiationAbort: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
		SchemaRef:      "",
	},
}

// Lookup returns the TypeMeta for mt and reports whether it is registered.
// TypeMeta is returned by value — callers cannot mutate the registry.
func Lookup(mt MessageType) (TypeMeta, bool) {
	meta, ok := registry[mt]
	return meta, ok
}

// Registered returns a snapshot of all registered MessageTypes.
// The returned slice is a copy — callers cannot affect the registry.
func Registered() []MessageType {
	out := make([]MessageType, 0, len(registry))
	for mt := range registry {
		out = append(out, mt)
	}
	return out
}
