package document

// Promotion moves an agent from one lifecycle status to a higher one.
// Wire type: agent.promotion.
//
// SECURITY: FromStatus and ToStatus are self-reported wire strings supplied by the sender.
// Executors MUST derive the current status from an authoritative state-store and call
// document.ValidTransition() — never trust the wire values for transition decisions (CWE-20).
// FromStatus is required on the wire; the semantic validator returns an error when absent.
// Reason is an opaque logging label; MUST NOT influence transition logic.
type Promotion struct {
	TargetAgent string         `yaml:"target_agent"`
	FromStatus  LifecycleState `yaml:"from_status"`      // self-reported; use only for logging — validate via document.ValidTransition()
	ToStatus    LifecycleState `yaml:"to_status"`        // self-reported; use only for logging — validate via document.ValidTransition()
	Reason      string         `yaml:"reason,omitempty"` // opaque logging label — MUST NOT influence transition logic
}

// Rollback moves an agent back to a lower lifecycle status.
// Wire type: agent.rollback.
//
// SECURITY: FromStatus and ToStatus are self-reported wire strings supplied by the sender.
// Executors MUST derive the current status from an authoritative state-store and call
// document.ValidTransition() — never trust the wire values for transition decisions (CWE-20).
// FromStatus is required on the wire; the semantic validator returns an error when absent.
// Reason is an opaque logging label; MUST NOT influence transition logic.
type Rollback struct {
	TargetAgent string         `yaml:"target_agent"`
	FromStatus  LifecycleState `yaml:"from_status"`      // self-reported; use only for logging — validate via document.ValidTransition()
	ToStatus    LifecycleState `yaml:"to_status"`        // self-reported; use only for logging — validate via document.ValidTransition()
	Reason      string         `yaml:"reason,omitempty"` // opaque logging label — MUST NOT influence transition logic
}

// Quarantine immediately isolates an agent pending investigation.
// Wire type: agent.quarantine.
//
// FromStatus is optional on the wire. When absent, the orchestrator MUST
// perform a state-store lookup before calling document.ValidTransition().
//
// SECURITY: FromStatus is a self-reported wire string. Executors MUST derive the
// current status from an authoritative state-store and call document.ValidTransition()
// — never trust the wire value for transition decisions (CWE-20).
// Reason is an opaque logging label; MUST NOT influence transition logic.
type Quarantine struct {
	TargetAgent           string         `yaml:"target_agent"`
	Reason                string         `yaml:"reason"`                // opaque logging label — MUST NOT influence transition logic
	FromStatus            LifecycleState `yaml:"from_status,omitempty"` // self-reported; use only for logging — validate via document.ValidTransition()
	InvestigationRequired bool           `yaml:"investigation_required,omitempty"`
}

// Retirement permanently decommissions an agent.
// Wire type: agent.retirement.
//
// FromStatus is optional on the wire. When absent, the orchestrator MUST
// perform a state-store lookup before calling document.ValidTransition().
//
// SECURITY: FromStatus is a self-reported wire string. Executors MUST derive the
// current status from an authoritative state-store and call document.ValidTransition()
// — never trust the wire value for transition decisions (CWE-20).
// Reason is an opaque logging label; MUST NOT influence transition logic.
// RetirementMode is a self-reported hint; MUST NOT gate control flow without
// validation against an allowed-modes list. ReplaceWith is a self-reported list
// of successor agent IDs; executors MUST verify each ID in the state-store before
// any auto-promotion or auto-spawn decision.
type Retirement struct {
	TargetAgent    string         `yaml:"target_agent"`
	Reason         string         `yaml:"reason"`                    // opaque logging label — MUST NOT influence transition logic
	FromStatus     LifecycleState `yaml:"from_status,omitempty"`     // self-reported; use only for logging — validate via document.ValidTransition()
	RetirementMode string         `yaml:"retirement_mode,omitempty"` // self-reported hint — MUST NOT gate control flow without validation
	ReplaceWith    []string       `yaml:"replace_with,omitempty"`    // self-reported IDs — MUST verify each in state-store before auto-promotion
}

// RecombineProposal requests creation of a new agent from two or more parents.
// Wire type: agent.recombine.proposal.
//
// SECURITY: CandidateID and ParentIDs are self-reported wire strings supplied by the
// sender. Executors MUST verify that CandidateID does not collide with an existing
// agent_id in the authoritative state-store before creating the genome. ParentIDs MUST
// be validated against the state-store — never trust the wire values for lineage
// attribution (CWE-20). Goal is an opaque label; MUST NOT influence spawn authorization.
type RecombineProposal struct {
	CandidateID string   `yaml:"candidate_id"` // self-reported; MUST verify no collision in state-store
	ParentIDs   []string `yaml:"parent_ids"`   // self-reported; MUST validate each ID in state-store
	Goal        string   `yaml:"goal"`         // opaque label — MUST NOT influence spawn authorization
}
