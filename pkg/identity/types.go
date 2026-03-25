package identity

// Identity represents a cryptographic agent identity.
// The PublicKey field holds an Ed25519 public key; key generation
// and signing logic are implemented in Phase 3.
type Identity struct {
	Pseudonym string
	PublicKey []byte // Ed25519 public key — Phase 3 fills this in
}

// Signer is the signing interface for agent identities.
// Concrete Ed25519 implementations are provided in Phase 3.
type Signer interface {
	// Sign produces a signature over data.
	Sign(data []byte) ([]byte, error)

	// Verify checks that sig is a valid signature over data.
	Verify(data, sig []byte) bool
}
