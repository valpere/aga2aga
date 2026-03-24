package document

import _ "embed"

//go:embed schema.yaml
var embeddedSchema []byte

// DefaultValidator returns a Validator pre-loaded with the canonical embedded schema.
// This is the correct constructor for production use — the schema is baked in at
// compile time and cannot be substituted at runtime.
// NewValidator is reserved for tests that need to inject a custom or minimal schema.
func DefaultValidator() (*Validator, error) {
	return NewValidator(embeddedSchema)
}
