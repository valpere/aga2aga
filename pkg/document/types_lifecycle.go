package document

// Promotion moves an agent from one lifecycle status to a higher one.
// Wire type: agent.promotion.
//
// SECURITY: FromStatus and ToStatus are self-reported wire strings supplied by the sender.
// Executors MUST derive the current status from an authoritative state-store and call
// lifecycle.ValidTransition() — never trust the wire values for transition decisions (CWE-20).
// Reason is an opaque logging label; MUST NOT influence transition logic.
type Promotion struct {
	TargetAgent string         `yaml:"target_agent"`
	FromStatus  LifecycleState `yaml:"from_status"` // self-reported; use only for logging — validate via lifecycle.ValidTransition()
	ToStatus    LifecycleState `yaml:"to_status"`   // self-reported; use only for logging — validate via lifecycle.ValidTransition()
	Reason      string         `yaml:"reason,omitempty"` // opaque logging label — MUST NOT influence transition logic
}

// Rollback moves an agent back to a lower lifecycle status.
// Wire type: agent.rollback.
//
// SECURITY: FromStatus and ToStatus are self-reported wire strings supplied by the sender.
// Executors MUST derive the current status from an authoritative state-store and call
// lifecycle.ValidTransition() — never trust the wire values for transition decisions (CWE-20).
// Reason is an opaque logging label; MUST NOT influence transition logic.
type Rollback struct {
	TargetAgent string         `yaml:"target_agent"`
	FromStatus  LifecycleState `yaml:"from_status"` // self-reported; use only for logging — validate via lifecycle.ValidTransition()
	ToStatus    LifecycleState `yaml:"to_status"`   // self-reported; use only for logging — validate via lifecycle.ValidTransition()
	Reason      string         `yaml:"reason,omitempty"` // opaque logging label — MUST NOT influence transition logic
}

// Quarantine immediately isolates an agent pending investigation.
// Wire type: agent.quarantine.
//
// FromStatus is optional on the wire. When absent, the orchestrator MUST
// perform a state-store lookup before calling ValidTransition.
//
// SECURITY: FromStatus is a self-reported wire string. Executors MUST derive the
// current status from an authoritative state-store and call lifecycle.ValidTransition()
// — never trust the wire value for transition decisions (CWE-20).
// Reason is an opaque logging label; MUST NOT influence transition logic.
type Quarantine struct {
	TargetAgent           string         `yaml:"target_agent"`
	Reason                string         `yaml:"reason"`                          // opaque logging label — MUST NOT influence transition logic
	FromStatus            LifecycleState `yaml:"from_status,omitempty"`           // self-reported; use only for logging — validate via lifecycle.ValidTransition()
	InvestigationRequired bool           `yaml:"investigation_required,omitempty"`
}

// Retirement permanently decommissions an agent.
// Wire type: agent.retirement.
//
// FromStatus is optional on the wire. When absent, the orchestrator MUST
// perform a state-store lookup before calling ValidTransition.
//
// SECURITY: FromStatus is a self-reported wire string. Executors MUST derive the
// current status from an authoritative state-store and call lifecycle.ValidTransition()
// — never trust the wire value for transition decisions (CWE-20).
// Reason is an opaque logging label; MUST NOT influence transition logic.
type Retirement struct {
	TargetAgent    string         `yaml:"target_agent"`
	Reason         string         `yaml:"reason"`                    // opaque logging label — MUST NOT influence transition logic
	FromStatus     LifecycleState `yaml:"from_status,omitempty"`     // self-reported; use only for logging — validate via lifecycle.ValidTransition()
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
