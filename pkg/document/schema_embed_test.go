package document_test

import (
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
