package document

// SpawnProposal is a request to spawn a new agent from one or more parents.
// Wire type: agent.spawn.proposal.
type SpawnProposal struct {
	CandidateID    string         `yaml:"candidate_id"`
	ParentIDs      []string       `yaml:"parent_ids"`
	Generation     int            `yaml:"generation,omitempty"`
	SpawnReason    string         `yaml:"spawn_reason"`
	RiskLevel      string         `yaml:"risk_level,omitempty"`
	EvaluationPlan string         `yaml:"evaluation_plan,omitempty"`
	// GenomePatch describes the mutable subset of AgentGenome a proposer may change.
	// DO_NOT_TOUCH fields are structurally absent. See GenomePatch in types_genome.go.
	GenomePatch    *GenomePatch   `yaml:"genome_patch,omitempty"`
}

// SpawnApproval grants permission to move a candidate agent to sandbox.
// Wire type: agent.spawn.approval.
// Note: in_reply_to is an Envelope field — read it from doc.Envelope.InReplyTo.
type SpawnApproval struct {
	CandidateID string `yaml:"candidate_id"`
	Decision    string `yaml:"decision"`
}

// SpawnRejection refuses a spawn proposal.
// Wire type: agent.spawn.rejection.
// Note: in_reply_to is an Envelope field — read it from doc.Envelope.InReplyTo.
type SpawnRejection struct {
	CandidateID string `yaml:"candidate_id"`
	Decision    string `yaml:"decision"`
}
