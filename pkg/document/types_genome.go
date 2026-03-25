package document

// Identity holds cryptographic identity fields for an agent.
type Identity struct {
	// DO_NOT_TOUCH: spec §5.4 — public_key is immutable once set.
	PublicKey string `yaml:"public_key"`
	// DO_NOT_TOUCH: spec §5.4 — pseudonym is immutable once set.
	Pseudonym string `yaml:"pseudonym,omitempty"`
}

// Lineage tracks an agent's ancestry.
// DO_NOT_TOUCH: spec §5.4 — lineage fields are immutable once set.
type Lineage struct {
	Parents    []string `yaml:"parents,omitempty"`
	Generation int      `yaml:"generation,omitempty"`
}

// Capabilities describes what an agent can do.
type Capabilities struct {
	Skills  []string `yaml:"skills"`
	Domains []string `yaml:"domains,omitempty"`
}

// Tools describes what tools an agent may invoke.
type Tools struct {
	Allowed        []string `yaml:"allowed"`
	PreferredOrder []string `yaml:"preferred_order,omitempty"`
}

// ModelPolicy specifies which AI model the agent uses.
type ModelPolicy struct {
	Provider         string  `yaml:"provider"`
	ModelClass       string  `yaml:"model_class,omitempty"`
	Model            string  `yaml:"model,omitempty"`
	Temperature      float64 `yaml:"temperature,omitempty"`
	MaxContextTokens int     `yaml:"max_context_tokens,omitempty"`
}

// PromptPolicy controls how the agent constructs prompts.
type PromptPolicy struct {
	Profile string `yaml:"profile"`
	// SECURITY: Style is map[string]any — all keys and values are attacker-controlled
	// (open vocabulary per spec §4.3). Callers MUST NOT use Style values for auth,
	// signing, or lifecycle decisions without explicit sanitization. See issue #35.
	Style map[string]any `yaml:"style,omitempty"`
}

// EscalationRule defines a condition that triggers routing to another agent.
//
// SECURITY: Condition and Target are self-reported wire strings supplied by the sender.
// Dispatchers MUST NOT execute or interpret Condition as code or query language — it is an
// opaque label. Target MUST be validated against a known agent registry in the authoritative
// state-store before any dispatch decision — never trust the wire value (CWE-20, CWE-601).
type EscalationRule struct {
	Condition string `yaml:"condition"` // opaque label — MUST NOT be executed or interpreted
	Target    string `yaml:"target"`    // self-reported agent ID — MUST validate in registry before dispatch
}

// RoutingPolicy defines what messages an agent accepts and how it delegates.
//
// SECURITY: Accepts, DelegatesTo, and EscalationRules are all wire-supplied and attacker-
// controlled. Dispatchers MUST sanitize Accepts entries against the known protocol message
// type registry before routing. DelegatesTo agent IDs MUST be validated in the authoritative
// state-store before any delegation decision — never trust the wire values (CWE-20, CWE-601).
type RoutingPolicy struct {
	Accepts         []string         `yaml:"accepts"`                   // self-reported; MUST validate against protocol registry before routing
	DelegatesTo     []string         `yaml:"delegates_to,omitempty"`    // self-reported agent IDs — MUST validate in registry before delegation
	EscalationRules []EscalationRule `yaml:"escalation_rules,omitempty"`
}

// MemoryPolicy configures the agent's memory window.
type MemoryPolicy struct {
	ShortTermWindow int      `yaml:"short_term_window,omitempty"`
	LongTermEnabled bool     `yaml:"long_term_enabled,omitempty"`
	LongTermScope   []string `yaml:"long_term_scope,omitempty"`
}

// Thresholds defines confidence thresholds for agent decisions.
type Thresholds struct {
	ConfidenceMin float64 `yaml:"confidence_min,omitempty"`
	CommitMin     float64 `yaml:"commit_min,omitempty"`
	RejectMin     float64 `yaml:"reject_min,omitempty"`
}

// Economics configures cost/latency/quality trade-off weights.
type Economics struct {
	CostWeight    float64 `yaml:"cost_weight,omitempty"`
	LatencyWeight float64 `yaml:"latency_weight,omitempty"`
	QualityWeight float64 `yaml:"quality_weight,omitempty"`
}

// FitnessMetrics records weighted fitness scores across all dimensions.
// Safety is a hard gate — agents with safety_violations > 0 fail promotion.
type FitnessMetrics struct {
	Quality        float64 `yaml:"quality,omitempty"`
	Reliability    float64 `yaml:"reliability,omitempty"`
	Latency        float64 `yaml:"latency,omitempty"`
	CostEfficiency float64 `yaml:"cost_efficiency,omitempty"`
	Collaboration  float64 `yaml:"collaboration,omitempty"`
	Safety         float64 `yaml:"safety,omitempty"`
	WeightedTotal  float64 `yaml:"weighted_total,omitempty"`
}

// Fitness holds the agent's current fitness objectives and scores.
// DO_NOT_TOUCH via spawn proposal: intentionally absent from GenomePatch — spec §5.4.
type Fitness struct {
	Objectives FitnessMetrics `yaml:"objectives,omitempty"`
}

// Constraints specifies hard and soft behavioural limits for an agent.
type Constraints struct {
	// DO_NOT_TOUCH: spec §5.4 — hard constraints are immutable; proposals that
	// weaken hard constraints MUST be rejected.
	Hard []string `yaml:"hard"`
	Soft []string `yaml:"soft,omitempty"`
}

// MutationPolicy controls which fields of a genome can be evolved.
type MutationPolicy struct {
	Allowed []string `yaml:"allowed"`
	// DO_NOT_TOUCH: spec §5.4 — forbidden list is immutable once set.
	Forbidden []string `yaml:"forbidden,omitempty"`
}

// RetirementPolicy defines when an agent should be automatically retired.
// DO_NOT_TOUCH via spawn proposal: intentionally absent from GenomePatch — spec §5.4.
type RetirementPolicy struct {
	IfBelowFitness float64 `yaml:"if_below_fitness,omitempty"`
	MinEvaluations int     `yaml:"min_evaluations,omitempty"`
}

// SandboxPolicy controls constraints for agents in sandbox status.
// DO_NOT_TOUCH via spawn proposal: intentionally absent from GenomePatch — spec §5.4.
type SandboxPolicy struct {
	MaxTasks           int  `yaml:"max_tasks,omitempty"`
	CanTouchProduction bool `yaml:"can_touch_production,omitempty"`
}

// GenomePatch carries only the fields an evolver is permitted to propose changing.
// DO_NOT_TOUCH fields (agent_id, kind, lineage, identity, constraints.hard,
// mutation_policy.forbidden) are intentionally absent — see spec §5.4.
// A patch-apply function MUST reject any attempt to modify fields not present here.
type GenomePatch struct {
	Capabilities *Capabilities `yaml:"capabilities,omitempty"`
	Tools        *Tools        `yaml:"tools,omitempty"`
	ModelPolicy  *ModelPolicy  `yaml:"model_policy,omitempty"`
	// SECURITY: PromptPolicy.Style is attacker-controlled — see PromptPolicy.Style annotation (issue #35).
	PromptPolicy  *PromptPolicy  `yaml:"prompt_policy,omitempty"`
	RoutingPolicy *RoutingPolicy `yaml:"routing_policy,omitempty"`
	MemoryPolicy  *MemoryPolicy  `yaml:"memory_policy,omitempty"`
	Thresholds    *Thresholds    `yaml:"thresholds,omitempty"`
	Economics     *Economics     `yaml:"economics,omitempty"`
	// Patch-apply MUST append these to the live Constraints.Soft slice — never replace it.
	// Removal of existing soft constraints via a spawn proposal is not permitted.
	SoftConstraints []string `yaml:"soft_constraints,omitempty"`
	Tags            []string `yaml:"tags,omitempty"`
}

// AgentGenome is the complete blueprint for an agent, describing its
// identity, capabilities, policies, and behavioural constraints.
//
// Immutable fields (DO_NOT_TOUCH per spec §5.4):
//   - AgentID
//   - Kind
//   - Lineage
//   - CreatedAt (via Envelope)
//   - Identity.PublicKey, Identity.Pseudonym
//   - Constraints.Hard
//   - MutationPolicy.Forbidden
type AgentGenome struct {
	// DO_NOT_TOUCH: spec §5.4 — agent_id is immutable once set.
	AgentID string `yaml:"agent_id"`
	// DO_NOT_TOUCH: spec §5.4 — kind is immutable once set.
	Kind      string `yaml:"kind"`
	Status    string `yaml:"status"`
	CreatedBy string `yaml:"created_by,omitempty"`
	// DO_NOT_TOUCH: spec §5.4 — lineage is immutable once set.
	Lineage          *Lineage          `yaml:"lineage,omitempty"`
	Identity         Identity          `yaml:"identity"`
	Capabilities     Capabilities      `yaml:"capabilities"`
	Tools            Tools             `yaml:"tools"`
	ModelPolicy      ModelPolicy       `yaml:"model_policy"`
	PromptPolicy     PromptPolicy      `yaml:"prompt_policy"`
	RoutingPolicy    RoutingPolicy     `yaml:"routing_policy"`
	MemoryPolicy     *MemoryPolicy     `yaml:"memory_policy,omitempty"`
	Thresholds       Thresholds        `yaml:"thresholds"`
	Economics        *Economics        `yaml:"economics,omitempty"`
	Fitness          *Fitness          `yaml:"fitness,omitempty"`
	Constraints      Constraints       `yaml:"constraints"`
	MutationPolicy   MutationPolicy    `yaml:"mutation_policy"`
	RetirementPolicy *RetirementPolicy `yaml:"retirement_policy,omitempty"`
	SandboxPolicy    *SandboxPolicy    `yaml:"sandbox_policy,omitempty"`
	Tags             []string          `yaml:"tags,omitempty"`
}
