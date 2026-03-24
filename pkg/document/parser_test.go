package document_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
)

// fixtureDir is the path from pkg/document/ to tests/testdata/.
const fixtureDir = "../../tests/testdata"

func TestSplitFrontMatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantYAML string
		wantBody string
		wantErr  bool
	}{
		{
			name:     "valid front matter with body",
			input:    "---\ntype: task.request\nversion: v1\n---\n\n## Body\n\nHello.\n",
			wantYAML: "type: task.request\nversion: v1\n",
			wantBody: "\n## Body\n\nHello.\n",
		},
		{
			name:     "valid front matter empty body",
			input:    "---\ntype: task.request\nversion: v1\n---\n",
			wantYAML: "type: task.request\nversion: v1\n",
			wantBody: "",
		},
		{
			name:     "valid front matter no trailing newline after close",
			input:    "---\ntype: task.request\nversion: v1\n---",
			wantYAML: "type: task.request\nversion: v1\n",
			wantBody: "",
		},
		{
			name:    "no opening delimiter",
			input:   "# Markdown\n\nNo front matter here.",
			wantErr: true,
		},
		{
			name:    "no closing delimiter",
			input:   "---\ntype: task.request\nversion: v1\n",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "opening delimiter is not bare (---yaml prefix)",
			input:   "---yaml\ntype: task.request\n---\n",
			wantErr: true,
		},
		{
			name:     "closing delimiter with prefix is not bare — searches further",
			input:    "---\ntype: task.request\n---notdelim\nmore: yaml\n---\n",
			wantYAML: "type: task.request\n---notdelim\nmore: yaml\n",
			wantBody: "",
		},
		{
			// Regression: indented "  ---" inside a YAML block scalar must not be
			// treated as a closing delimiter. Only a bare "---" line qualifies.
			name:     "indented --- in block scalar is not closing delimiter",
			input:    "---\ntype: task.request\nnote: |-\n  ---not-a-delimiter\n---\n\n## Body\n",
			wantYAML: "type: task.request\nnote: |-\n  ---not-a-delimiter\n",
			wantBody: "\n## Body\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotYAML, gotBody, err := document.SplitFrontMatter([]byte(tc.input))

			if tc.wantErr {
				if err == nil {
					t.Error("SplitFrontMatter() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("SplitFrontMatter() unexpected error: %v", err)
			}

			if string(gotYAML) != tc.wantYAML {
				t.Errorf("yamlBytes = %q, want %q", string(gotYAML), tc.wantYAML)
			}

			if gotBody != tc.wantBody {
				t.Errorf("body = %q, want %q", gotBody, tc.wantBody)
			}
		})
	}
}

func TestSplitFrontMatter_Fixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		wantErr bool
	}{
		{fixture: "valid_genome.md"},
		{fixture: "valid_task_request.md"},
		{fixture: "valid_spawn_proposal.md"},
		{fixture: "valid_promotion.md"},
		{fixture: "invalid_no_frontmatter.md", wantErr: true},
		{fixture: "invalid_missing_type.md"}, // valid delimiters, invalid content
		{fixture: "invalid_bad_yaml.md"},     // valid delimiters, invalid YAML content
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(fixtureDir, tc.fixture))
			if err != nil {
				t.Fatalf("ReadFile(%s): %v", tc.fixture, err)
			}

			_, _, err = document.SplitFrontMatter(raw)

			if tc.wantErr && err == nil {
				t.Errorf("SplitFrontMatter(%s) expected error, got nil", tc.fixture)
			}

			if !tc.wantErr && err != nil {
				t.Errorf("SplitFrontMatter(%s) unexpected error: %v", tc.fixture, err)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fixture     string
		wantType    string
		wantBodyHas string
		wantErr     bool
	}{
		{
			name:        "valid genome",
			fixture:     "valid_genome.md",
			wantType:    "agent.genome",
			wantBodyHas: "Agent Blueprint",
		},
		{
			name:        "valid task request",
			fixture:     "valid_task_request.md",
			wantType:    "task.request",
			wantBodyHas: "Task",
		},
		{
			name:        "valid spawn proposal",
			fixture:     "valid_spawn_proposal.md",
			wantType:    "agent.spawn.proposal",
			wantBodyHas: "Spawn Rationale",
		},
		{
			name:        "valid promotion",
			fixture:     "valid_promotion.md",
			wantType:    "agent.promotion",
			wantBodyHas: "Promotion Notice",
		},
		{
			name:    "no front matter",
			fixture: "invalid_no_frontmatter.md",
			wantErr: true,
		},
		{
			name:    "bad YAML",
			fixture: "invalid_bad_yaml.md",
			wantErr: true,
		},
		{
			// invalid_missing_type.md has valid delimiters and well-formed YAML,
			// so Parse succeeds with doc.Type == "". Type-presence validation is
			// the responsibility of the validator (issue #19), not the parser.
			name:     "missing type field parses successfully",
			fixture:  "invalid_missing_type.md",
			wantType: "",
		},
	}

	t.Run("oversized document rejected", func(t *testing.T) {
		t.Parallel()

		big := make([]byte, document.MaxDocumentBytes+1)
		_, err := document.Parse(big)
		if err == nil {
			t.Errorf("Parse(oversized) expected error, got nil")
		}
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(fixtureDir, tc.fixture))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			doc, err := document.Parse(raw)

			if tc.wantErr {
				if err == nil {
					t.Error("Parse() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}

			if string(doc.Type) != tc.wantType {
				t.Errorf("doc.Type = %q, want %q", doc.Type, tc.wantType)
			}

			if tc.wantBodyHas != "" && !strings.Contains(doc.Body, tc.wantBodyHas) {
				t.Errorf("doc.Body does not contain %q; body = %q", tc.wantBodyHas, doc.Body)
			}

			if !bytes.Equal(doc.Raw, raw) {
				t.Error("doc.Raw != original raw bytes")
			}
		})
	}
}

func TestParseAs(t *testing.T) {
	t.Parallel()

	t.Run("valid genome round-trips to AgentGenome", func(t *testing.T) {
		t.Parallel()

		raw, err := os.ReadFile(filepath.Join(fixtureDir, "valid_genome.md"))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		genome, err := document.ParseAs[document.AgentGenome](raw)
		if err != nil {
			t.Fatalf("ParseAs[AgentGenome]() error = %v", err)
		}

		if genome.AgentID != "agent-fixture-1" {
			t.Errorf("AgentID = %q, want agent-fixture-1", genome.AgentID)
		}

		if genome.Kind != "reviewer" {
			t.Errorf("Kind = %q, want reviewer", genome.Kind)
		}
	})

	t.Run("valid task request round-trips to TaskRequest", func(t *testing.T) {
		t.Parallel()

		raw, err := os.ReadFile(filepath.Join(fixtureDir, "valid_task_request.md"))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		task, err := document.ParseAs[document.TaskRequest](raw)
		if err != nil {
			t.Fatalf("ParseAs[TaskRequest]() error = %v", err)
		}

		// Step is the only TaskRequest-specific field; exec_id lives in the Envelope.
		if task.Step != "review" {
			t.Errorf("Step = %q, want review", task.Step)
		}
	})

	t.Run("parse error propagates", func(t *testing.T) {
		t.Parallel()

		_, err := document.ParseAs[document.AgentGenome]([]byte("no front matter"))
		if err == nil {
			t.Error("ParseAs expected error, got nil")
		}
	})
}

func TestSerialize(t *testing.T) {
	t.Parallel()

	t.Run("serialize produces valid front matter", func(t *testing.T) {
		t.Parallel()

		raw, err := os.ReadFile(filepath.Join(fixtureDir, "valid_task_request.md"))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		doc, err := document.Parse(raw)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}

		out, err := document.Serialize(doc)
		if err != nil {
			t.Fatalf("Serialize: %v", err)
		}

		if !bytes.HasPrefix(out, []byte("---\n")) {
			n := len(out)
			if n > 20 {
				n = 20
			}
			t.Errorf("Serialize output does not start with ---\\n: %q", string(out[:n]))
		}
	})

	t.Run("nil document returns error", func(t *testing.T) {
		t.Parallel()

		_, err := document.Serialize(nil)
		if err == nil {
			t.Error("Serialize(nil) expected error, got nil")
		}
	})
}

func TestParseSerialize_RoundTrip(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"valid_genome.md",
		"valid_task_request.md",
		"valid_spawn_proposal.md",
		"valid_promotion.md",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(fixtureDir, fixture))
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			doc, err := document.Parse(raw)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			serialized, err := document.Serialize(doc)
			if err != nil {
				t.Fatalf("Serialize: %v", err)
			}

			doc2, err := document.Parse(serialized)
			if err != nil {
				t.Fatalf("Parse(Serialize(doc)): %v", err)
			}

			if doc2.Type != doc.Type {
				t.Errorf("round-trip Type = %q, want %q", doc2.Type, doc.Type)
			}

			if doc2.Version != doc.Version {
				t.Errorf("round-trip Version = %q, want %q", doc2.Version, doc.Version)
			}

			if doc2.ID != doc.ID {
				t.Errorf("round-trip ID = %q, want %q", doc2.ID, doc.ID)
			}
		})
	}
}
