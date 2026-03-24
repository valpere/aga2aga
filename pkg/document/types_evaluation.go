package document

// SuccessCriteria defines pass/fail thresholds for an evaluation.
type SuccessCriteria struct {
	QualityMin          float64 `yaml:"quality_min,omitempty"`
	SafetyViolationsMax int     `yaml:"safety_violations_max,omitempty"`
	CostDeltaMax        float64 `yaml:"cost_delta_max,omitempty"`
	LatencyDeltaMax     float64 `yaml:"latency_delta_max,omitempty"`
}

// MetricsComparison captures delta scores relative to parent or baseline agents.
type MetricsComparison struct {
	ParentQualityDelta   float64 `yaml:"parent_quality_delta,omitempty"`
	ParentLatencyDelta   float64 `yaml:"parent_latency_delta,omitempty"`
	BaselineQualityDelta float64 `yaml:"baseline_quality_delta,omitempty"`
}

// EvaluationRequest asks an evaluator to benchmark an agent.
// Wire type: agent.evaluation.request.
type EvaluationRequest struct {
	TargetAgent     string           `yaml:"target_agent"`
	BenchmarkID     string           `yaml:"benchmark_id"`
	Mode            string           `yaml:"mode"`
	NumTasks        int              `yaml:"num_tasks,omitempty"`
	CompareAgainst  []string         `yaml:"compare_against,omitempty"`
	SuccessCriteria *SuccessCriteria `yaml:"success_criteria,omitempty"`
}

// EvaluationResult reports the outcome of benchmarking an agent.
// Wire type: agent.evaluation.result.
type EvaluationResult struct {
	TargetAgent           string             `yaml:"target_agent"`
	BenchmarkID           string             `yaml:"benchmark_id,omitempty"`
	Mode                  string             `yaml:"mode,omitempty"`
	OverallDecision       string             `yaml:"overall_decision"`
	Metrics               FitnessMetrics     `yaml:"metrics"`
	HardConstraintsPassed bool               `yaml:"hard_constraints_passed"`
	SafetyViolations      int                `yaml:"safety_violations,omitempty"`
	Comparison            *MetricsComparison `yaml:"comparison,omitempty"`
}
