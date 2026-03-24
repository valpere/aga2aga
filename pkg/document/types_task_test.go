package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestTaskRequest_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: task.request
version: v1
id: msg-task-1
from: orchestrator
to: agent-reviewer
exec_id: exec-workflow-1
step: review_pr
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	req, err := document.As[document.TaskRequest](&doc)
	if err != nil {
		t.Fatalf("As[TaskRequest]() error = %v", err)
	}

	if req.Step != "review_pr" {
		t.Errorf("Step = %q, want %q", req.Step, "review_pr")
	}
}

func TestTaskResult_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: task.result
version: v1
id: msg-result-1
from: agent-reviewer
to: orchestrator
exec_id: exec-workflow-1
step: review_pr
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	res, err := document.As[document.TaskResult](&doc)
	if err != nil {
		t.Fatalf("As[TaskResult]() error = %v", err)
	}

	if res.Step != "review_pr" {
		t.Errorf("Step = %q, want %q", res.Step, "review_pr")
	}
}

func TestTaskFail_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: task.fail
version: v1
id: msg-fail-1
from: agent-reviewer
to: orchestrator
exec_id: exec-workflow-1
step: review_pr
error: timeout after 30s
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	fail, err := document.As[document.TaskFail](&doc)
	if err != nil {
		t.Fatalf("As[TaskFail]() error = %v", err)
	}

	if fail.Step != "review_pr" {
		t.Errorf("Step = %q, want review_pr", fail.Step)
	}

	if fail.Error != "timeout after 30s" {
		t.Errorf("Error = %q, want %q", fail.Error, "timeout after 30s")
	}
}

func TestTaskProgress_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	raw := `type: task.progress
version: v1
id: msg-progress-1
from: agent-reviewer
to: orchestrator
exec_id: exec-workflow-1
step: review_pr
percent: 50.0
`

	var doc document.Document

	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}

	progress, err := document.As[document.TaskProgress](&doc)
	if err != nil {
		t.Fatalf("As[TaskProgress]() error = %v", err)
	}

	if progress.Step != "review_pr" {
		t.Errorf("Step = %q, want review_pr", progress.Step)
	}

	if progress.Percent != 50.0 {
		t.Errorf("Percent = %f, want 50.0", progress.Percent)
	}
}
