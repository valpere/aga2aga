// Package admin contains domain types and interfaces for the aga2aga web
// administration interface. It is importable by pkg/gateway in Phase 2 to
// enforce agent registration and communication policies at the transport layer.
package admin

import "time"

// Role is a user permission level within an Organization.
type Role string

const (
	RoleAdmin    Role = "admin"    // full access: users, agents, policies, org settings
	RoleOperator Role = "operator" // register/suspend agents; create/edit policies
	RoleViewer   Role = "viewer"   // read-only
	RoleAgent    Role = "agent"    // bound to a specific agent ID; used for MCP tool auth
)

// AgentStatus is the lifecycle state of a RegisteredAgent.
type AgentStatus string

const (
	AgentStatusActive    AgentStatus = "active"
	AgentStatusSuspended AgentStatus = "suspended"
	AgentStatusRevoked   AgentStatus = "revoked"
)

// PolicyAction is the effect of a CommunicationPolicy.
type PolicyAction string

const (
	PolicyActionAllow PolicyAction = "allow"
	PolicyActionDeny  PolicyAction = "deny"
)

// PolicyDirection controls which side of a communication pair a policy applies to.
type PolicyDirection string

const (
	DirectionUnidirectional PolicyDirection = "unidirectional" // source → target only
	DirectionBidirectional  PolicyDirection = "bidirectional"  // source ↔ target
)

// Wildcard matches any agent or user within the same organization.
const Wildcard = "*"

// Organization is the top-level tenant that owns users and agents.
type Organization struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

// User is a human operator who can manage agents and policies.
type User struct {
	ID        string    `db:"id"`
	OrgID     string    `db:"org_id"`
	Username  string    `db:"username"`
	Password  string    `db:"password"` // bcrypt hash
	Role      Role      `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}

// RegisteredAgent is an agent instance authorized to operate within an org.
// AgentID corresponds to Envelope.From in the aga2aga envelope protocol.
type RegisteredAgent struct {
	ID           string      `db:"id"`
	OrgID        string      `db:"org_id"`
	AgentID      string      `db:"agent_id"` // matches Envelope.From
	DisplayName  string      `db:"display_name"`
	Status       AgentStatus `db:"status"`
	RegisteredBy string      `db:"registered_by"` // User.ID
	RegisteredAt time.Time   `db:"registered_at"`
}

// CommunicationPolicy controls whether messages may flow between two principals.
// Source and Target may be agent IDs, user IDs, or Wildcard ("*").
// Policies are evaluated by priority (highest first); first match wins.
// Default when no policy matches: deny.
type CommunicationPolicy struct {
	ID        string          `db:"id"`
	OrgID     string          `db:"org_id"`
	SourceID  string          `db:"source_id"` // agent_id, user_id, or "*"
	TargetID  string          `db:"target_id"` // agent_id, user_id, or "*"
	Direction PolicyDirection `db:"direction"`
	Action    PolicyAction    `db:"action"`
	Priority  int             `db:"priority"`   // higher value = evaluated first
	CreatedBy string          `db:"created_by"` // User.ID
	CreatedAt time.Time       `db:"created_at"`
}

// AuditEvent records a change made by a user within an organization.
// It is append-only: events are never updated or deleted.
type AuditEvent struct {
	ID         string    `db:"id"`
	OrgID      string    `db:"org_id"`
	UserID     string    `db:"user_id"`     // who performed the action
	Username   string    `db:"username"`    // denormalised for display without joins
	Action     string    `db:"action"`      // e.g. "agent.register", "policy.create"
	TargetType string    `db:"target_type"` // "agent" | "policy" | "session"
	TargetID   string    `db:"target_id"`   // ID of the affected entity
	Detail     string    `db:"detail"`      // human-readable summary
	CreatedAt  time.Time `db:"created_at"`
}

// MessageLog records a single inter-agent message exchanged through the gateway.
// Full body is stored for offline analysis; entries are append-only.
type MessageLog struct {
	ID         string    `db:"id"`
	OrgID      string    `db:"org_id"`
	EnvelopeID string    `db:"envelope_id"` // Document.ID; may be empty
	ThreadID   string    `db:"thread_id"`   // Document.ThreadID; may be empty
	FromAgent  string    `db:"from_agent"`
	ToAgent    string    `db:"to_agent"`
	MsgType    string    `db:"msg_type"`   // e.g. "agent.message", "task.request"
	Direction  string    `db:"direction"`  // "send" | "receive"
	ToolName   string    `db:"tool_name"`  // MCP tool that triggered the log
	BodySize   int       `db:"body_size"`  // len(body) in bytes
	Body       string    `db:"body"`       // full message body
	CreatedAt  time.Time `db:"created_at"`
}

// MessageLogFilter constrains which MessageLog rows are returned by ListMessageLogs.
// Zero values mean "no filter" for that field.
type MessageLogFilter struct {
	AgentID  string    // match from_agent OR to_agent
	ToolName string    // exact match on tool_name
	Since    time.Time // created_at >= Since (zero = no lower bound)
	Until    time.Time // created_at <= Until (zero = no upper bound)
	Limit    int       // max rows; 0 → default 200
}

// AgentLimits holds per-agent (or global-default) resource constraints enforced
// at the gateway. AgentID="*" means the global default; agent-specific rows take
// precedence. All numeric fields use 0 to mean "unlimited / use global default".
type AgentLimits struct {
	ID              string    `db:"id"`
	OrgID           string    `db:"org_id"`
	AgentID         string    `db:"agent_id"`          // specific agent or "*" for global default
	MaxBodyBytes    int       `db:"max_body_bytes"`    // 0 = unlimited
	MaxSendPerMin   int       `db:"max_send_per_min"`  // 0 = unlimited
	MaxPendingTasks int       `db:"max_pending_tasks"` // 0 = unlimited
	MaxStreamLen    int64     `db:"max_stream_len"`    // 0 = unlimited (Redis MAXLEN)
	UpdatedAt       time.Time `db:"updated_at"`
	UpdatedBy       string    `db:"updated_by"` // User.ID
}

// APIKey grants programmatic access to the admin API (e.g. the gateway).
// The raw key is shown once at creation; only its SHA-256 hash is stored.
type APIKey struct {
	ID        string    `db:"id"`
	OrgID     string    `db:"org_id"`
	Name      string    `db:"name"`       // human label
	KeyHash   string    `db:"key_hash"`   // SHA-256 hex of the raw key
	Role      Role      `db:"role"`       // operator | viewer | agent (never admin)
	AgentID   string    `db:"agent_id"`   // non-empty when role=agent; the bound agent identity
	CreatedBy string    `db:"created_by"` // User.ID
	CreatedAt time.Time `db:"created_at"`
	RevokedAt time.Time `db:"revoked_at"` // zero = active
}
