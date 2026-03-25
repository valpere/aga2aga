package document

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/valpere/aga2aga/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// Layer name constants for ValidationError.Layer.
const (
	LayerStructural = "structural"
	LayerSchema     = "schema"
	LayerSemantic   = "semantic"
)

// ValidationError records a single validation failure, carrying enough context
// for callers to filter by layer or field.
type ValidationError struct {
	Layer   string // "structural", "schema", or "semantic"
	Field   string // YAML field name, or "" if not field-specific
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Layer, e.Field, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Layer, e.Message)
}

// Validator performs 3-layer validation of Skills Documents.
// Construct with NewValidator — zero value is not usable.
type Validator struct {
	compiler *jsonschema.Compiler
}

// NewValidator constructs a Validator from JSON Schema 2020-12 bytes (YAML format).
// The schemaBytes are converted from YAML to JSON at load time — no runtime YAML parsing
// on the hot Validate path.
func NewValidator(schemaBytes []byte) (*Validator, error) {
	// Convert YAML schema → JSON for the jsonschema library.
	var schemaMap any
	if err := yaml.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, fmt.Errorf("NewValidator: parse schema YAML: %w", err)
	}
	jsonBytes, err := json.Marshal(schemaMap)
	if err != nil {
		return nil, fmt.Errorf("NewValidator: convert schema to JSON: %w", err)
	}

	// Unmarshal JSON to a Go value — AddResource expects a JSON-decoded value, not a reader.
	var jsonValue any
	if err := json.Unmarshal(jsonBytes, &jsonValue); err != nil {
		return nil, fmt.Errorf("NewValidator: unmarshal JSON schema: %w", err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("aga2aga://schema", jsonValue); err != nil {
		return nil, fmt.Errorf("NewValidator: add schema resource: %w", err)
	}
	// Pre-compile the root schema to catch structural errors at load time.
	if _, err := c.Compile("aga2aga://schema"); err != nil {
		return nil, fmt.Errorf("NewValidator: compile schema: %w", err)
	}

	return &Validator{compiler: c}, nil
}

// ValidateStructural performs Layer 1: required-field checks using protocol.Registry.
// Fast — no JSON Schema needed. Returns all structural errors (not fail-fast).
func (v *Validator) ValidateStructural(doc *Document) []ValidationError {
	if doc == nil {
		return []ValidationError{{Layer: LayerStructural, Message: "doc is nil"}}
	}

	var errs []ValidationError

	// Base envelope: type must be present before we can look up the registry.
	if doc.Type == "" {
		return []ValidationError{{Layer: LayerStructural, Field: "type", Message: "required field missing"}}
	}

	if doc.Version == "" {
		errs = append(errs, ValidationError{Layer: LayerStructural, Field: "version", Message: "required field missing"})
	}

	// Validate the type is registered.
	meta, ok := protocol.Lookup(doc.Type)
	if !ok {
		return append(errs, ValidationError{
			Layer:   LayerStructural,
			Field:   "type",
			Message: fmt.Sprintf("unknown message type %q", doc.Type),
		})
	}

	// Check type-specific required fields — they may live in Envelope or Extra.
	for _, field := range meta.RequiredFields {
		if !docHasField(doc, field) {
			errs = append(errs, ValidationError{
				Layer:   LayerStructural,
				Field:   field,
				Message: "required field missing",
			})
		}
	}

	return errs
}

// docHasField reports whether a field is non-empty in the Document's Envelope or Extra.
func docHasField(doc *Document, field string) bool {
	// Check envelope fields first via their YAML tag names.
	switch field {
	case "type":
		return doc.Type != ""
	case "version":
		return doc.Version != ""
	case "id":
		return doc.ID != ""
	case "from":
		return doc.From != ""
	case "to":
		return len(doc.To) > 0
	case "created_at":
		return doc.CreatedAt != ""
	case "in_reply_to":
		return doc.InReplyTo != ""
	case "thread_id":
		return doc.ThreadID != ""
	case "exec_id":
		return doc.ExecID != ""
	case "status":
		return doc.Status != ""
	}
	// Fall through to Extra for type-specific fields.
	v, exists := doc.Extra[field]
	if !exists {
		return false
	}
	// Reject zero-value strings and nil values.
	switch val := v.(type) {
	case string:
		return val != ""
	case nil:
		return false
	default:
		return true
	}
}

// ValidateSchema performs Layer 2: JSON Schema 2020-12 validation.
// Skipped silently for message types with no SchemaRef in the registry.
func (v *Validator) ValidateSchema(doc *Document) []ValidationError {
	if doc == nil {
		return nil
	}

	meta, ok := protocol.Lookup(doc.Type)
	if !ok || meta.SchemaRef == "" {
		return nil
	}

	// Compile the type-specific $def.
	// NOTE: the internal refURL is not surfaced in errors to avoid leaking schema internals.
	refURL := "aga2aga://schema#/$defs/" + meta.SchemaRef
	defSchema, err := v.compiler.Compile(refURL)
	if err != nil {
		return []ValidationError{{
			Layer:   LayerSchema,
			Message: fmt.Sprintf("schema unavailable for type %q", string(doc.Type)),
		}}
	}

	// Convert the full document to a JSON-typed value for validation.
	// NOTE: doc.Extra (attacker-controlled) is included in the validation object because
	// type-specific required fields legitimately live in Extra. The schema $defs do NOT
	// use additionalProperties:false by design (forward-compat). The validation surface
	// is intentionally the merged wire document, not the As[T] execution surface. Callers
	// that require stricter field isolation should use As[T] after validation.
	docVal, err := docToJSONValue(doc)
	if err != nil {
		return []ValidationError{{Layer: LayerSchema, Message: fmt.Sprintf("convert doc: %v", err)}}
	}

	if err := defSchema.Validate(docVal); err != nil {
		return schemaErrToValidationErrors(err)
	}

	return nil
}

// maxIntermediateBytes caps the intermediate YAML/JSON representations produced
// during schema validation to prevent CWE-400 via nested-map expansion.
// 4× MaxDocumentBytes provides a generous expansion budget while bounding allocations.
const maxIntermediateBytes = 4 * MaxDocumentBytes

// docToJSONValue converts a Document to a map[string]any with JSON-native types
// (float64 for numbers, string, bool, []any, map[string]any) suitable for the
// jsonschema library. The YAML→JSON round-trip normalises integer types.
func docToJSONValue(doc *Document) (any, error) {
	yamlBytes, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal doc: %w", err)
	}
	if len(yamlBytes) > maxIntermediateBytes {
		return nil, fmt.Errorf("docToJSONValue: intermediate YAML exceeds size limit (%d bytes)", maxIntermediateBytes)
	}
	var m any
	if err := yaml.Unmarshal(yamlBytes, &m); err != nil {
		return nil, fmt.Errorf("unmarshal doc: %w", err)
	}
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	var v any
	if err := json.Unmarshal(jsonBytes, &v); err != nil {
		return nil, fmt.Errorf("unmarshal JSON: %w", err)
	}
	return v, nil
}

// schemaErrToValidationErrors converts a jsonschema validation error tree into
// a flat slice of ValidationError values.
func schemaErrToValidationErrors(err error) []ValidationError {
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []ValidationError{{Layer: LayerSchema, Message: err.Error()}}
	}

	var out []ValidationError
	collectSchemaErrors(ve, &out)
	if len(out) == 0 {
		out = append(out, ValidationError{Layer: LayerSchema, Message: ve.Error()})
	}
	return out
}

func collectSchemaErrors(ve *jsonschema.ValidationError, out *[]ValidationError) {
	if len(ve.Causes) == 0 {
		field := ""
		if len(ve.InstanceLocation) > 0 {
			field = ve.InstanceLocation[len(ve.InstanceLocation)-1]
		}
		*out = append(*out, ValidationError{
			Layer:   LayerSchema,
			Field:   field,
			Message: ve.Error(),
		})
		return
	}
	for _, cause := range ve.Causes {
		collectSchemaErrors(cause, out)
	}
}

// ValidateSemantic performs Layer 3: semantic protocol rule checks.
// Validates lifecycle transition legality for promotion, rollback, quarantine,
// and retirement messages. Validates self-promotion denial for agent.promotion.
func (v *Validator) ValidateSemantic(doc *Document) []ValidationError {
	if doc == nil {
		return nil
	}

	switch doc.Type {
	case protocol.AgentPromotion, protocol.AgentRollback:
		return validateLifecycleTransition(doc)
	case protocol.AgentQuarantine:
		return validateTerminalTransition(doc, StateQuarantined)
	case protocol.AgentRetirement:
		return validateTerminalTransition(doc, StateRetired)
	}

	return nil
}

// validateTerminalTransition validates from_status → toState when from_status is
// present on the wire. When absent (omitempty), no error is returned — the
// orchestrator MUST perform a state-store lookup before applying the transition.
func validateTerminalTransition(doc *Document, toState LifecycleState) []ValidationError {
	fromStr, _ := doc.Extra["from_status"].(string)
	if fromStr == "" {
		// from_status is optional for quarantine/retirement; orchestrator handles lookup.
		return nil
	}
	from := LifecycleState(fromStr)
	if !ValidTransition(from, toState) {
		return []ValidationError{{
			Layer:   LayerSemantic,
			Field:   "from_status",
			Message: fmt.Sprintf("transition %q → %q is not permitted by spec §16", from, toState),
		}}
	}
	return nil
}

func validateLifecycleTransition(doc *Document) []ValidationError {
	var errs []ValidationError

	fromStr, _ := doc.Extra["from_status"].(string)
	toStr, _ := doc.Extra["to_status"].(string)

	// Explicit guard: distinguish "fields missing" from "illegal transition" in
	// governance logs. The structural layer should already have caught absence, but
	// this provides defence-in-depth and unambiguous error messages.
	if fromStr == "" || toStr == "" {
		return []ValidationError{{
			Layer:   LayerSemantic,
			Field:   "from_status/to_status",
			Message: "from_status and to_status are required for lifecycle transition",
		}}
	}

	from := LifecycleState(fromStr)
	to := LifecycleState(toStr)

	if !ValidTransition(from, to) {
		errs = append(errs, ValidationError{
			Layer:   LayerSemantic,
			Field:   "from_status/to_status",
			Message: fmt.Sprintf("transition %q → %q is not permitted by spec §16", from, to),
		})
	}

	// Self-promotion check.
	// SECURITY: doc.From is unverified until Phase 3 Ed25519 signing — this is
	// defence-in-depth today, becomes a security boundary in Phase 3 (issue #43).
	if doc.Type == protocol.AgentPromotion {
		targetAgent, ok := doc.Extra["target_agent"].(string)
		if !ok || targetAgent == "" {
			errs = append(errs, ValidationError{
				Layer:   LayerSemantic,
				Field:   "target_agent",
				Message: "target_agent is required for self-promotion check",
			})
		} else if doc.From != "" && doc.From == targetAgent {
			errs = append(errs, ValidationError{
				Layer:   LayerSemantic,
				Field:   "from/target_agent",
				Message: fmt.Sprintf("self-promotion denied: agent %q cannot promote itself", targetAgent),
			})
		}
	}

	return errs
}

// Validate runs all three layers and returns all errors (not fail-fast).
// Callers can filter by ValidationError.Layer to handle each layer independently.
// If structural validation fails (including unknown message type), schema and semantic
// layers are skipped — they have no valid basis to run on a structurally broken document.
func (v *Validator) Validate(doc *Document) []ValidationError {
	if structural := v.ValidateStructural(doc); len(structural) > 0 {
		return structural
	}
	var errs []ValidationError
	errs = append(errs, v.ValidateSchema(doc)...)
	errs = append(errs, v.ValidateSemantic(doc)...)
	return errs
}
