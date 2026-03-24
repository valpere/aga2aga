package document

// SpawnProposal is a request to spawn a new agent from one or more parents.
// Wire type: agent.spawn.proposal
type SpawnProposal struct {
	CandidateID    string         `yaml:"candidate_id"`
	ParentIDs      []string       `yaml:"parent_ids"`
	Generation     int            `yaml:"generation,omitempty"`
	SpawnReason    string         `yaml:"spawn_reason"`
	RiskLevel      string         `yaml:"risk_level,omitempty"`
	EvaluationPlan string         `yaml:"evaluation_plan,omitempty"`
	GenomePatch    map[string]any `yaml:"genome_patch,omitempty"`
}

// SpawnApproval grants permission to move a candidate agent to sandbox.
// Wire type: agent.spawn.approval
type SpawnApproval struct {
	InReplyTo   string `yaml:"in_reply_to"`
	CandidateID string `yaml:"candidate_id"`
	Decision    string `yaml:"decision"`
}

// SpawnRejection refuses a spawn proposal.
// Wire type: agent.spawn.rejection
type SpawnRejection struct {
	InReplyTo   string `yaml:"in_reply_to"`
	CandidateID string `yaml:"candidate_id"`
	Decision    string `yaml:"decision"`
}
