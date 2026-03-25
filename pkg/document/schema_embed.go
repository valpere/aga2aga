package document

import (
	_ "embed"
	"sync"
)

//go:embed schema.yaml
var embeddedSchema []byte

var (
	defaultValidatorOnce sync.Once
	defaultValidatorVal  *Validator
	defaultValidatorErr  error
)

func init() {
	// Eagerly construct the singleton so startup fails loudly for a corrupt embed
	// rather than silently returning a permanent error on every subsequent call.
	if _, err := DefaultValidator(); err != nil {
		panic("aga2aga: failed to compile embedded schema: " + err.Error())
	}
}

// DefaultValidator returns a Validator pre-loaded with the canonical embedded schema.
// The Validator is constructed once at package init and cached — repeated calls
// return the same instance. Safe for concurrent use after init completes.
// This is the correct constructor for production use — the schema is baked in at
// compile time and cannot be substituted at runtime.
// NewValidator is reserved for tests that need to inject a custom or minimal schema.
func DefaultValidator() (*Validator, error) {
	defaultValidatorOnce.Do(func() {
		defaultValidatorVal, defaultValidatorErr = NewValidator(embeddedSchema)
	})
	return defaultValidatorVal, defaultValidatorErr
}
