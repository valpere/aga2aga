package document_test

import (
	"os"
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
)

func mustReadFile(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

func mustNewValidator(t *testing.T) *document.Validator {
	t.Helper()
	schemaBytes, err := os.ReadFile("schema.yaml")
	if err != nil {
		t.Fatalf("read schema.yaml: %v", err)
	}
	v, err := document.NewValidator(schemaBytes)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func mustParse(t *testing.T, raw []byte) *document.Document {
	t.Helper()
	doc, err := document.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return doc
}

// TestNewValidator verifies that NewValidator loads a valid schema without error.
func TestNewValidator(t *testing.T) {
	schemaBytes, err := os.ReadFile("schema.yaml")
	if err != nil {
		t.Fatalf("read schema.yaml: %v", err)
	}
	_, err = document.NewValidator(schemaBytes)
	if err != nil {
		t.Errorf("NewValidator() error = %v, want nil", err)
	}
}

// TestNewValidator_InvalidSchema verifies that NewValidator rejects malformed YAML.
func TestNewValidator_InvalidSchema(t *testing.T) {
	_, err := document.NewValidator([]byte("not: [valid: yaml: schema"))
	if err == nil {
		t.Error("NewValidator(invalid) = nil error, want error")
	}
}

// TestValidateStructural covers Layer 1: required-field checks via protocol.Registry.
func TestValidateStructural(t *testing.T) {
	v := mustNewValidator(t)

	tests := []struct {
		name       string
		raw        []byte
		wantErrors int
		wantField  string // first error field if wantErrors > 0
	}{
		{
			name:       "valid task.request has no structural errors",
			raw:        mustReadFile("../../tests/testdata/valid_task_request.md"),
			wantErrors: 0,
		},
		{
			name:       "valid genome has no structural errors",
			raw:        mustReadFile("../../tests/testdata/valid_genome.md"),
			wantErrors: 0,
		},
		{
			name:       "valid spawn proposal has no structural errors",
			raw:        mustReadFile("../../tests/testdata/valid_spawn_proposal.md"),
			wantErrors: 0,
		},
		{
			name:       "valid promotion has no structural errors",
			raw:        mustReadFile("../../tests/testdata/valid_promotion.md"),
			wantErrors: 0,
		},
		{
			name:       "missing type returns error on type field",
			raw:        mustReadFile("../../tests/testdata/invalid_missing_type.md"),
			wantErrors: 1,
			wantField:  "type",
		},
		{
			name:       "unknown type returns error",
			raw:        []byte("---\ntype: unknown.type\nversion: v1\n---\n"),
			wantErrors: 1,
			wantField:  "type",
		},
		{
			name:       "missing version returns error",
			raw:        []byte("---\ntype: task.request\nid: msg-1\nfrom: orchestrator\nto: reviewer\nexec_id: exec-1\nstep: review\n---\n"),
			wantErrors: 1,
			wantField:  "version",
		},
		{
			name:       "task.request missing id returns error",
			raw:        []byte("---\ntype: task.request\nversion: v1\nfrom: orchestrator\nto: reviewer\nexec_id: exec-1\nstep: review\n---\n"),
			wantErrors: 1,
			wantField:  "id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.raw)
			errs := v.ValidateStructural(doc)
			if len(errs) != tc.wantErrors {
				t.Errorf("ValidateStructural() = %d errors %v, want %d", len(errs), errs, tc.wantErrors)
				return
			}
			if tc.wantErrors > 0 && errs[0].Field != tc.wantField {
				t.Errorf("first error field = %q, want %q", errs[0].Field, tc.wantField)
			}
			if tc.wantErrors > 0 && errs[0].Layer != document.LayerStructural {
				t.Errorf("first error layer = %q, want %q", errs[0].Layer, document.LayerStructural)
			}
		})
	}
}

// TestValidateSchema covers Layer 2: JSON Schema 2020-12 validation.
func TestValidateSchema(t *testing.T) {
	v := mustNewValidator(t)

	tests := []struct {
		name       string
		raw        []byte
		wantErrors int
	}{
		{
			name:       "valid genome passes schema",
			raw:        mustReadFile("../../tests/testdata/valid_genome.md"),
			wantErrors: 0,
		},
		{
			name:       "valid spawn proposal passes schema",
			raw:        mustReadFile("../../tests/testdata/valid_spawn_proposal.md"),
			wantErrors: 0,
		},
		{
			name:       "valid promotion passes schema",
			raw:        mustReadFile("../../tests/testdata/valid_promotion.md"),
			wantErrors: 0,
		},
		{
			name:       "task.request skips schema (no SchemaRef)",
			raw:        mustReadFile("../../tests/testdata/valid_task_request.md"),
			wantErrors: 0,
		},
		{
			name: "genome missing identity fails schema",
			raw: []byte("---\ntype: agent.genome\nversion: v1\nagent_id: fixture\nkind: reviewer\n" +
				"status: proposed\ncapabilities:\n  skills:\n    - code-review\n" +
				"tools:\n  allowed:\n    - read_file\n" +
				"model_policy:\n  provider: anthropic\n" +
				"prompt_policy:\n  profile: balanced\n" +
				"routing_policy:\n  accepts:\n    - task.request\n" +
				"thresholds:\n  confidence_min: 0.7\n" +
				"constraints:\n  hard:\n    - no_production_writes\n" +
				"mutation_policy:\n  allowed:\n    - prompt_policy\n---\n"),
			wantErrors: 1,
		},
		{
			name: "promotion with invalid from_status fails schema",
			raw: []byte("---\ntype: agent.promotion\nversion: v1\nid: msg-1\nfrom: pop-manager\n" +
				"target_agent: agent-1\nfrom_status: not-a-state\nto_status: active\n---\n"),
			wantErrors: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.raw)
			errs := v.ValidateSchema(doc)
			if len(errs) != tc.wantErrors {
				t.Errorf("ValidateSchema() = %d errors %v, want %d", len(errs), errs, tc.wantErrors)
			}
			for _, e := range errs {
				if e.Layer != document.LayerSchema {
					t.Errorf("error layer = %q, want %q", e.Layer, document.LayerSchema)
				}
			}
		})
	}
}

// TestValidateSemantic covers Layer 3: semantic protocol rules.
func TestValidateSemantic(t *testing.T) {
	v := mustNewValidator(t)

	tests := []struct {
		name       string
		raw        []byte
		wantErrors int
		wantMsg    string // substring in first error message
	}{
		{
			name:       "valid promotion candidate→active passes",
			raw:        mustReadFile("../../tests/testdata/valid_promotion.md"),
			wantErrors: 0,
		},
		{
			name: "invalid transition proposed→active returns error",
			raw: []byte("---\ntype: agent.promotion\nversion: v1\nid: msg-1\nfrom: pop-manager\n" +
				"target_agent: agent-1\nfrom_status: proposed\nto_status: active\n---\n"),
			wantErrors: 1,
			wantMsg:    "transition",
		},
		{
			name: "self-promotion rejected",
			raw: []byte("---\ntype: agent.promotion\nversion: v1\nid: msg-1\nfrom: agent-1\n" +
				"target_agent: agent-1\nfrom_status: candidate\nto_status: active\n---\n"),
			wantErrors: 1,
			wantMsg:    "self-promotion",
		},
		{
			name: "valid rollback active→inactive passes",
			raw: []byte("---\ntype: agent.rollback\nversion: v1\nid: msg-1\nfrom: pop-manager\n" +
				"target_agent: agent-1\nfrom_status: active\nto_status: inactive\n---\n"),
			wantErrors: 0,
		},
		{
			name: "invalid rollback transition returns error",
			raw: []byte("---\ntype: agent.rollback\nversion: v1\nid: msg-1\nfrom: pop-manager\n" +
				"target_agent: agent-1\nfrom_status: proposed\nto_status: active\n---\n"),
			wantErrors: 1,
			wantMsg:    "transition",
		},
		{
			name:       "task.request skips semantic check",
			raw:        mustReadFile("../../tests/testdata/valid_task_request.md"),
			wantErrors: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.raw)
			errs := v.ValidateSemantic(doc)
			if len(errs) != tc.wantErrors {
				t.Errorf("ValidateSemantic() = %d errors %v, want %d", len(errs), errs, tc.wantErrors)
				return
			}
			if tc.wantErrors > 0 {
				if errs[0].Layer != document.LayerSemantic {
					t.Errorf("error layer = %q, want %q", errs[0].Layer, document.LayerSemantic)
				}
				if tc.wantMsg != "" {
					found := false
					for _, e := range errs {
						if contains(e.Message, tc.wantMsg) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("no error contains %q in %v", tc.wantMsg, errs)
					}
				}
			}
		})
	}
}

// TestValidate_Composite verifies the composite Validate runs all three layers.
func TestValidate_Composite(t *testing.T) {
	v := mustNewValidator(t)

	tests := []struct {
		name       string
		raw        []byte
		wantErrors int
	}{
		{
			name:       "valid genome — all layers pass",
			raw:        mustReadFile("../../tests/testdata/valid_genome.md"),
			wantErrors: 0,
		},
		{
			name:       "valid task.request — all layers pass",
			raw:        mustReadFile("../../tests/testdata/valid_task_request.md"),
			wantErrors: 0,
		},
		{
			name:       "invalid_missing_type — layer 1 fires",
			raw:        mustReadFile("../../tests/testdata/invalid_missing_type.md"),
			wantErrors: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.raw)
			errs := v.Validate(doc)
			if len(errs) != tc.wantErrors {
				t.Errorf("Validate() = %d errors %v, want %d", len(errs), errs, tc.wantErrors)
			}
		})
	}
}

// TestValidationError_Error verifies the Error() string format.
func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		err  document.ValidationError
		want string
	}{
		{
			err:  document.ValidationError{Layer: "structural", Field: "type", Message: "required field missing"},
			want: "[structural] type: required field missing",
		},
		{
			err:  document.ValidationError{Layer: "semantic", Field: "", Message: "self-promotion denied"},
			want: "[semantic] self-promotion denied",
		},
	}
	for _, tc := range tests {
		t.Run(tc.err.Layer+"/"+tc.err.Field, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
