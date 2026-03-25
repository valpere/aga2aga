package document

// Promotion moves an agent from one lifecycle status to a higher one.
// Wire type: agent.promotion.
type Promotion struct {
	TargetAgent string         `yaml:"target_agent"`
	FromStatus  LifecycleState `yaml:"from_status"` // self-reported wire string; validate with ValidTransition before applying
	ToStatus    LifecycleState `yaml:"to_status"`   // self-reported wire string; validate with ValidTransition before applying
	Reason      string         `yaml:"reason,omitempty"`
}

// Rollback moves an agent back to a lower lifecycle status.
// Wire type: agent.rollback.
type Rollback struct {
	TargetAgent string         `yaml:"target_agent"`
	FromStatus  LifecycleState `yaml:"from_status"` // self-reported wire string; validate with ValidTransition before applying
	ToStatus    LifecycleState `yaml:"to_status"`   // self-reported wire string; validate with ValidTransition before applying
	Reason      string         `yaml:"reason,omitempty"`
}

// Quarantine immediately isolates an agent pending investigation.
// Wire type: agent.quarantine.
//
// FromStatus is optional on the wire. When absent, the orchestrator MUST
// perform a state-store lookup before calling ValidTransition.
type Quarantine struct {
	TargetAgent           string         `yaml:"target_agent"`
	Reason                string         `yaml:"reason"`
	FromStatus            LifecycleState `yaml:"from_status,omitempty"` // self-reported wire string; validate with ValidTransition before applying
	InvestigationRequired bool           `yaml:"investigation_required,omitempty"`
}

// Retirement permanently decommissions an agent.
// Wire type: agent.retirement.
//
// FromStatus is optional on the wire. When absent, the orchestrator MUST
// perform a state-store lookup before calling ValidTransition.
type Retirement struct {
	TargetAgent    string         `yaml:"target_agent"`
	Reason         string         `yaml:"reason"`
	FromStatus     LifecycleState `yaml:"from_status,omitempty"` // self-reported wire string; validate with ValidTransition before applying
	RetirementMode string         `yaml:"retirement_mode,omitempty"`
	ReplaceWith    []string       `yaml:"replace_with,omitempty"`
}

// RecombineProposal requests creation of a new agent from two or more parents.
// Wire type: agent.recombine.proposal.
type RecombineProposal struct {
	CandidateID string   `yaml:"candidate_id"`
	ParentIDs   []string `yaml:"parent_ids"`
	Goal        string   `yaml:"goal"`
}
