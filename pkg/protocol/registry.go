package protocol

// BaseEnvelopeFields are required on every message per the base envelope schema.
var BaseEnvelopeFields = []string{"type", "version"}

// Registry maps each MessageType to its TypeMeta.
// It is the single source of truth consumed by parser, validator, and builder.
var Registry = map[MessageType]TypeMeta{
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
	},
	TaskResult: {
		RequiredFields: []string{"id", "from", "to", "exec_id", "step"},
	},
	TaskFail: {
		RequiredFields: []string{"id", "from", "to", "exec_id", "step"},
	},
	TaskProgress: {
		RequiredFields: []string{"id", "from", "to", "exec_id", "step"},
	},
	AgentMessage: {
		RequiredFields: []string{"from", "to"},
	},

	// Negotiation (8)
	// negotiation.propose starts a thread — no in_reply_to required.
	NegotiationPropose: {
		RequiredFields: []string{"id", "from", "to", "thread_id"},
	},
	// All subsequent negotiation types require in_reply_to.
	NegotiationAccept: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
	},
	NegotiationReject: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
	},
	NegotiationCounter: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
	},
	NegotiationClarify: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
	},
	NegotiationDelegate: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
	},
	NegotiationCommit: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
	},
	NegotiationAbort: {
		RequiredFields: []string{"id", "from", "to", "thread_id", "in_reply_to"},
	},
}
