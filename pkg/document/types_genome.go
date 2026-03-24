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
	Profile string         `yaml:"profile"`
	Style   map[string]any `yaml:"style,omitempty"`
}

// EscalationRule defines a condition that triggers routing to another agent.
type EscalationRule struct {
	Condition string `yaml:"condition"`
	Target    string `yaml:"target"`
}

// RoutingPolicy defines what messages an agent accepts and how it delegates.
type RoutingPolicy struct {
	Accepts         []string         `yaml:"accepts"`
	DelegatesTo     []string         `yaml:"delegates_to,omitempty"`
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
type RetirementPolicy struct {
	IfBelowFitness float64 `yaml:"if_below_fitness,omitempty"`
	MinEvaluations int     `yaml:"min_evaluations,omitempty"`
}

// SandboxPolicy controls constraints for agents in sandbox status.
type SandboxPolicy struct {
	MaxTasks           int  `yaml:"max_tasks,omitempty"`
	CanTouchProduction bool `yaml:"can_touch_production,omitempty"`
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
