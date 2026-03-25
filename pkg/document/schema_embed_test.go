package document_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/valpere/aga2aga/pkg/document"
)

// TestDefaultValidator_ReturnsSameInstance verifies that repeated calls to
// DefaultValidator return the same *Validator pointer (singleton behaviour).
func TestDefaultValidator_ReturnsSameInstance(t *testing.T) {
	v1, err := document.DefaultValidator()
	if err != nil {
		t.Fatalf("DefaultValidator() first call error = %v", err)
	}
	v2, err := document.DefaultValidator()
	if err != nil {
		t.Fatalf("DefaultValidator() second call error = %v", err)
	}
	if v1 != v2 {
		t.Errorf("DefaultValidator() returned different pointers: %p vs %p — expected singleton", v1, v2)
	}
}

// TestDefaultValidator_ValidatesKnownGoodDocument verifies that DefaultValidator
// produces a working validator by validating a known-good genome document.
func TestDefaultValidator_ValidatesKnownGoodDocument(t *testing.T) {
	v, err := document.DefaultValidator()
	if err != nil {
		t.Fatalf("DefaultValidator() error = %v", err)
	}

	raw := mustReadFile("../../tests/testdata/valid_genome.md")
	doc := mustParse(t, raw)

	if errs := v.Validate(doc); len(errs) != 0 {
		t.Errorf("DefaultValidator().Validate(valid_genome) = %v, want no errors", errs)
	}
}

// TestDefaultValidator_EmbeddedSchemaMatchesDisk verifies that the embedded schema
// (used by DefaultValidator) and the schema.yaml on disk produce equivalent validation
// results on the same document. This guards against the embedded schema diverging from
// the source without being caught by tests.
func TestDefaultValidator_EmbeddedSchemaMatchesDisk(t *testing.T) {
	embedded, err := document.DefaultValidator()
	if err != nil {
		t.Fatalf("DefaultValidator() error = %v", err)
	}

	schemaBytes, err := os.ReadFile("schema.yaml")
	if err != nil {
		t.Fatalf("read schema.yaml: %v", err)
	}
	disk, err := document.NewValidator(schemaBytes)
	if err != nil {
		t.Fatalf("NewValidator(disk): %v", err)
	}

	for _, fixture := range []string{"valid_genome.md", "valid_promotion.md", "valid_task_request.md"} {
		raw := mustReadFile("../../tests/testdata/" + fixture)
		doc := mustParse(t, raw)

		embeddedErrs := embedded.Validate(doc)
		diskErrs := disk.Validate(doc)

		if len(embeddedErrs) != len(diskErrs) {
			t.Errorf("%s: embedded Validate() = %d errors, disk = %d errors — schemas diverged",
				fixture, len(embeddedErrs), len(diskErrs))
		}
	}
}

// TestDefaultValidator_ConcurrentValidation verifies that the shared singleton Validator
// is safe for concurrent use — no data races in ValidateSchema across goroutines.
func TestDefaultValidator_ConcurrentValidation(t *testing.T) {
	v, err := document.DefaultValidator()
	if err != nil {
		t.Fatalf("DefaultValidator() error = %v", err)
	}

	raw := mustReadFile("../../tests/testdata/valid_genome.md")
	doc := mustParse(t, raw)

	const concurrency = 20
	errc := make(chan error, concurrency)
	for range concurrency {
		go func() {
			errs := v.ValidateSchema(doc)
			if len(errs) != 0 {
				errc <- fmt.Errorf("ValidateSchema() unexpected errors: %v", errs)
				return
			}
			errc <- nil
		}()
	}
	for range concurrency {
		if err := <-errc; err != nil {
			t.Error(err)
		}
	}
}
