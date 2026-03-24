package document_test

import (
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
	"gopkg.in/yaml.v3"
)

func TestTaskRequest_UnmarshalYAML(t *testing.T) {
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
