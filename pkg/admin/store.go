package admin

import "context"

// UserStore persists and retrieves User records.
type UserStore interface {
	CreateUser(ctx context.Context, u *User) error
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	// UpdateUserPassword replaces the stored bcrypt hash for the given user ID.
	// Returns an error if the user is not found or the update fails.
	UpdateUserPassword(ctx context.Context, id, hashedPassword string) error
}

// AgentStore persists and retrieves RegisteredAgent records.
type AgentStore interface {
	RegisterAgent(ctx context.Context, a *RegisteredAgent) error
	GetAgent(ctx context.Context, orgID, agentID string) (*RegisteredAgent, error)
	ListAgents(ctx context.Context, orgID string) ([]RegisteredAgent, error)
	UpdateAgentStatus(ctx context.Context, orgID, agentID string, status AgentStatus) error
}

// PolicyStore persists and retrieves CommunicationPolicy records.
type PolicyStore interface {
	CreatePolicy(ctx context.Context, p *CommunicationPolicy) error
	GetPolicy(ctx context.Context, id string) (*CommunicationPolicy, error)
	ListPolicies(ctx context.Context, orgID string) ([]CommunicationPolicy, error)
	UpdatePolicy(ctx context.Context, p *CommunicationPolicy) error
	DeletePolicy(ctx context.Context, id string) error
}

// OrgStore persists and retrieves Organization records.
type OrgStore interface {
	CreateOrg(ctx context.Context, o *Organization) error
	GetOrgByID(ctx context.Context, id string) (*Organization, error)
}

// AuditStore appends and retrieves AuditEvent records. Events are never modified.
type AuditStore interface {
	AppendAuditEvent(ctx context.Context, e *AuditEvent) error
	ListAuditEvents(ctx context.Context, orgID string, limit int) ([]AuditEvent, error)
}

// APIKeyStore persists and retrieves APIKey records.
type APIKeyStore interface {
	CreateAPIKey(ctx context.Context, k *APIKey) error
	GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
	ListAPIKeys(ctx context.Context, orgID string) ([]APIKey, error)
	// RevokeAPIKey marks the key as revoked. orgID is required so that a caller
	// cannot revoke keys belonging to a different organization (CWE-639).
	RevokeAPIKey(ctx context.Context, orgID, id string) error
}

// Store is the full persistence interface required by the admin server.
type Store interface {
	OrgStore
	UserStore
	AgentStore
	PolicyStore
	AuditStore
	APIKeyStore
}
