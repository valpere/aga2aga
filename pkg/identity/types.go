package identity

import "crypto/ed25519"

// Identity represents a cryptographic agent identity.
// PublicKey uses the stdlib ed25519.PublicKey type, which is always 32 bytes.
// Key generation and signing logic are implemented in Phase 3.
//
// Zero value is not usable; construct via the Phase 3 identity package.
type Identity struct {
	Pseudonym string
	PublicKey ed25519.PublicKey // Ed25519 public key (32 bytes) — Phase 3 fills this in
}

// Signer is the signing interface for agent identities.
// Concrete Ed25519 implementations are provided in Phase 3.
type Signer interface {
	// Sign produces a signature over data.
	Sign(data []byte) ([]byte, error)

	// Verify checks that sig is a valid signature over data.
	// Returns (false, nil) for a cryptographically invalid signature,
	// and (false, error) for a configuration or implementation fault
	// (e.g., nil or zero-length key). Callers MUST treat a non-nil error
	// as an indeterminate result — not as a simple auth rejection. (CWE-252)
	Verify(data, sig []byte) (bool, error)
}
