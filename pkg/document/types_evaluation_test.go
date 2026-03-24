package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestEvaluationRequest_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: agent.evaluation.request
version: v1
id: msg-eval-1
from: benchmark-curator
target_agent: agent-candidate-42
benchmark_id: bench-go-review-v1
mode: sandbox
num_tasks: 20
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	req, err := document.As[document.EvaluationRequest](&doc)
	if err != nil {
		t.Fatalf("As[EvaluationRequest]() error = %v", err)
	}

	if req.TargetAgent != "agent-candidate-42" {
		t.Errorf("TargetAgent = %q, want %q", req.TargetAgent, "agent-candidate-42")
	}

	if req.BenchmarkID != "bench-go-review-v1" {
		t.Errorf("BenchmarkID = %q, want %q", req.BenchmarkID, "bench-go-review-v1")
	}

	if req.Mode != "sandbox" {
		t.Errorf("Mode = %q, want %q", req.Mode, "sandbox")
	}
}

func TestEvaluationResult_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: agent.evaluation.result
version: v1
id: msg-eval-result-1
from: evaluator
target_agent: agent-candidate-42
overall_decision: pass
metrics:
  quality: 0.85
  safety: 1.0
  weighted_total: 0.88
hard_constraints_passed: true
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	res, err := document.As[document.EvaluationResult](&doc)
	if err != nil {
		t.Fatalf("As[EvaluationResult]() error = %v", err)
	}

	if res.OverallDecision != "pass" {
		t.Errorf("OverallDecision = %q, want %q", res.OverallDecision, "pass")
	}

	if !res.HardConstraintsPassed {
		t.Errorf("HardConstraintsPassed = false, want true")
	}

	if res.Metrics.Quality != 0.85 {
		t.Errorf("Metrics.Quality = %f, want 0.85", res.Metrics.Quality)
	}
}

func TestEvaluationRequest_WithSuccessCriteria(t *testing.T) {
	t.Parallel()

	raw := `type: agent.evaluation.request
version: v1
id: msg-eval-2
from: benchmark-curator
target_agent: agent-candidate-42
benchmark_id: bench-go-review-v1
mode: candidate
success_criteria:
  quality_min: 0.80
  safety_violations_max: 0
  cost_delta_max: 0.10
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	req, err := document.As[document.EvaluationRequest](&doc)
	if err != nil {
		t.Fatalf("As[EvaluationRequest]() error = %v", err)
	}

	if req.SuccessCriteria == nil {
		t.Fatalf("SuccessCriteria is nil, want non-nil")
	}

	if req.SuccessCriteria.QualityMin != 0.80 {
		t.Errorf("SuccessCriteria.QualityMin = %f, want 0.80", req.SuccessCriteria.QualityMin)
	}

	if req.SuccessCriteria.SafetyViolationsMax != 0 {
		t.Errorf("SuccessCriteria.SafetyViolationsMax = %d, want 0", req.SuccessCriteria.SafetyViolationsMax)
	}
}

func TestEvaluationResult_WithComparison(t *testing.T) {
	t.Parallel()

	raw := `type: agent.evaluation.result
version: v1
id: msg-eval-result-2
from: evaluator
target_agent: agent-candidate-42
overall_decision: pass
metrics:
  quality: 0.88
  safety: 1.0
  weighted_total: 0.90
hard_constraints_passed: true
comparison:
  parent_quality_delta: 0.05
  parent_latency_delta: -0.02
  baseline_quality_delta: 0.12
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	res, err := document.As[document.EvaluationResult](&doc)
	if err != nil {
		t.Fatalf("As[EvaluationResult]() error = %v", err)
	}

	if res.Comparison == nil {
		t.Fatalf("Comparison is nil, want non-nil")
	}

	if res.Comparison.ParentQualityDelta != 0.05 {
		t.Errorf("Comparison.ParentQualityDelta = %f, want 0.05", res.Comparison.ParentQualityDelta)
	}

	if res.Comparison.BaselineQualityDelta != 0.12 {
		t.Errorf("Comparison.BaselineQualityDelta = %f, want 0.12", res.Comparison.BaselineQualityDelta)
	}
}
