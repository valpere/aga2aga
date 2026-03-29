// Package admin provides the HTTP server, handlers, middleware, and storage
// backend for the aga2aga web administration interface.
package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/valpere/aga2aga/pkg/admin"
	_ "modernc.org/sqlite" // register sqlite driver
)

// SQLiteStore implements admin.Store backed by a single SQLite file.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) an SQLite database at path and runs the
// schema migration. Use ":memory:" for tests.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite allows only one writer at a time
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Close releases the underlying database connection.
func (s *SQLiteStore) Close() error { return s.db.Close() }

// migrate creates all tables if they do not already exist and applies
// incremental schema changes for existing databases.
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS organizations (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL UNIQUE,
  created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id         TEXT PRIMARY KEY,
  org_id     TEXT NOT NULL REFERENCES organizations(id),
  username   TEXT NOT NULL UNIQUE,
  password   TEXT NOT NULL,
  role       TEXT NOT NULL,
  created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS registered_agents (
  id            TEXT PRIMARY KEY,
  org_id        TEXT NOT NULL REFERENCES organizations(id),
  agent_id      TEXT NOT NULL,
  display_name  TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL,
  registered_by TEXT NOT NULL REFERENCES users(id),
  registered_at DATETIME NOT NULL,
  UNIQUE(org_id, agent_id)
);

CREATE TABLE IF NOT EXISTS audit_events (
  id          TEXT PRIMARY KEY,
  org_id      TEXT NOT NULL REFERENCES organizations(id),
  user_id     TEXT NOT NULL,
  username    TEXT NOT NULL,
  action      TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id   TEXT NOT NULL,
  detail      TEXT NOT NULL DEFAULT '',
  created_at  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS api_keys (
  id         TEXT PRIMARY KEY,
  org_id     TEXT NOT NULL REFERENCES organizations(id),
  name       TEXT NOT NULL,
  key_hash   TEXT NOT NULL UNIQUE,
  role       TEXT NOT NULL,
  agent_id   TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at DATETIME NOT NULL,
  revoked_at DATETIME NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS communication_policies (
  id         TEXT PRIMARY KEY,
  org_id     TEXT NOT NULL REFERENCES organizations(id),
  source_id  TEXT NOT NULL,
  target_id  TEXT NOT NULL,
  direction  TEXT NOT NULL,
  action     TEXT NOT NULL,
  priority   INTEGER NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL REFERENCES users(id),
  created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS message_logs (
  id          TEXT PRIMARY KEY,
  org_id      TEXT NOT NULL REFERENCES organizations(id),
  envelope_id TEXT NOT NULL DEFAULT '',
  thread_id   TEXT NOT NULL DEFAULT '',
  from_agent  TEXT NOT NULL,
  to_agent    TEXT NOT NULL,
  msg_type    TEXT NOT NULL,
  direction   TEXT NOT NULL,
  tool_name   TEXT NOT NULL,
  body_size   INTEGER NOT NULL DEFAULT 0,
  body        TEXT NOT NULL DEFAULT '',
  created_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_message_logs_org_time  ON message_logs(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_message_logs_agent     ON message_logs(org_id, from_agent, to_agent);

CREATE TABLE IF NOT EXISTS agent_limits (
  id                TEXT PRIMARY KEY,
  org_id            TEXT NOT NULL REFERENCES organizations(id),
  agent_id          TEXT NOT NULL,
  max_body_bytes    INTEGER NOT NULL DEFAULT 0,
  max_send_per_min  INTEGER NOT NULL DEFAULT 0,
  max_pending_tasks INTEGER NOT NULL DEFAULT 0,
  max_stream_len    INTEGER NOT NULL DEFAULT 0,
  updated_at        DATETIME NOT NULL,
  updated_by        TEXT NOT NULL REFERENCES users(id),
  UNIQUE(org_id, agent_id)
);
`)
	if err != nil {
		return err
	}
	// Migration: add agent_id column if it was not present in an older schema.
	// SQLite ignores "IF NOT EXISTS" for columns, but this statement is idempotent
	// for new databases because the CREATE TABLE above already defines the column.
	_, err = db.Exec(`ALTER TABLE api_keys ADD COLUMN agent_id TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		// SQLite returns "duplicate column name" when the column already exists.
		// Treat that as a no-op so existing databases migrate cleanly.
		if !isDuplicateColumnErr(err) {
			return fmt.Errorf("migrate api_keys.agent_id: %w", err)
		}
	}
	return nil
}

// isDuplicateColumnErr reports whether the error is a SQLite "duplicate column
// name" error, which occurs when ALTER TABLE ADD COLUMN is run on a table that
// already has the column — a normal outcome for idempotent migrations.
func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate column name")
}

// --- OrgStore ---

func (s *SQLiteStore) CreateOrg(ctx context.Context, o *admin.Organization) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO organizations(id, name, created_at) VALUES(?,?,?)`,
		o.ID, o.Name, o.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) GetOrgByID(ctx context.Context, id string) (*admin.Organization, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, created_at FROM organizations WHERE id=?`, id)
	var o admin.Organization
	var createdAt string
	if err := row.Scan(&o.ID, &o.Name, &createdAt); err != nil {
		return nil, err
	}
	o.CreatedAt = parseRFC3339(createdAt)
	return &o, nil
}

// --- UserStore ---

func (s *SQLiteStore) CreateUser(ctx context.Context, u *admin.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users(id, org_id, username, password, role, created_at) VALUES(?,?,?,?,?,?)`,
		u.ID, u.OrgID, u.Username, u.Password, string(u.Role), u.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) GetUserByUsername(ctx context.Context, username string) (*admin.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, username, password, role, created_at FROM users WHERE username=?`, username)
	return scanUser(row)
}

func (s *SQLiteStore) GetUserByID(ctx context.Context, id string) (*admin.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, username, password, role, created_at FROM users WHERE id=?`, id)
	return scanUser(row)
}

func (s *SQLiteStore) UpdateUserPassword(ctx context.Context, id, hashedPassword string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password=? WHERE id=?`, hashedPassword, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("user %q not found", id)
	}
	return nil
}

func scanUser(row *sql.Row) (*admin.User, error) {
	var u admin.User
	var role, createdAt string
	if err := row.Scan(&u.ID, &u.OrgID, &u.Username, &u.Password, &role, &createdAt); err != nil {
		return nil, err
	}
	u.Role = admin.Role(role)
	u.CreatedAt = parseRFC3339(createdAt)
	return &u, nil
}

// --- AgentStore ---

func (s *SQLiteStore) RegisterAgent(ctx context.Context, a *admin.RegisteredAgent) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO registered_agents(id, org_id, agent_id, display_name, status, registered_by, registered_at)
		 VALUES(?,?,?,?,?,?,?)`,
		a.ID, a.OrgID, a.AgentID, a.DisplayName, string(a.Status),
		a.RegisteredBy, a.RegisteredAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) GetAgent(ctx context.Context, orgID, agentID string) (*admin.RegisteredAgent, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, agent_id, display_name, status, registered_by, registered_at
		 FROM registered_agents WHERE org_id=? AND agent_id=?`, orgID, agentID)
	return scanAgent(row)
}

func (s *SQLiteStore) ListAgents(ctx context.Context, orgID string) ([]admin.RegisteredAgent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, org_id, agent_id, display_name, status, registered_by, registered_at
		 FROM registered_agents WHERE org_id=? ORDER BY registered_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var agents []admin.RegisteredAgent
	for rows.Next() {
		var a admin.RegisteredAgent
		var status, registeredAt string
		if err := rows.Scan(&a.ID, &a.OrgID, &a.AgentID, &a.DisplayName, &status, &a.RegisteredBy, &registeredAt); err != nil {
			return nil, err
		}
		a.Status = admin.AgentStatus(status)
		a.RegisteredAt = parseRFC3339(registeredAt)
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *SQLiteStore) UpdateAgentStatus(ctx context.Context, orgID, agentID string, status admin.AgentStatus) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE registered_agents SET status=? WHERE org_id=? AND agent_id=?`,
		string(status), orgID, agentID)
	return err
}

func scanAgent(row *sql.Row) (*admin.RegisteredAgent, error) {
	var a admin.RegisteredAgent
	var status, registeredAt string
	if err := row.Scan(&a.ID, &a.OrgID, &a.AgentID, &a.DisplayName, &status, &a.RegisteredBy, &registeredAt); err != nil {
		return nil, err
	}
	a.Status = admin.AgentStatus(status)
	a.RegisteredAt = parseRFC3339(registeredAt)
	return &a, nil
}

// --- PolicyStore ---

func (s *SQLiteStore) CreatePolicy(ctx context.Context, p *admin.CommunicationPolicy) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO communication_policies(id, org_id, source_id, target_id, direction, action, priority, created_by, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		p.ID, p.OrgID, p.SourceID, p.TargetID, string(p.Direction), string(p.Action),
		p.Priority, p.CreatedBy, p.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) GetPolicy(ctx context.Context, id string) (*admin.CommunicationPolicy, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, source_id, target_id, direction, action, priority, created_by, created_at
		 FROM communication_policies WHERE id=?`, id)
	return scanPolicy(row)
}

func (s *SQLiteStore) ListPolicies(ctx context.Context, orgID string) ([]admin.CommunicationPolicy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, org_id, source_id, target_id, direction, action, priority, created_by, created_at
		 FROM communication_policies WHERE org_id=? ORDER BY priority DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var policies []admin.CommunicationPolicy
	for rows.Next() {
		var p admin.CommunicationPolicy
		var direction, action, createdAt string
		if err := rows.Scan(&p.ID, &p.OrgID, &p.SourceID, &p.TargetID, &direction, &action,
			&p.Priority, &p.CreatedBy, &createdAt); err != nil {
			return nil, err
		}
		p.Direction = admin.PolicyDirection(direction)
		p.Action = admin.PolicyAction(action)
		p.CreatedAt = parseRFC3339(createdAt)
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (s *SQLiteStore) UpdatePolicy(ctx context.Context, p *admin.CommunicationPolicy) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE communication_policies
		 SET source_id=?, target_id=?, direction=?, action=?, priority=?
		 WHERE id=?`,
		p.SourceID, p.TargetID, string(p.Direction), string(p.Action), p.Priority, p.ID,
	)
	return err
}

func (s *SQLiteStore) DeletePolicy(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM communication_policies WHERE id=?`, id)
	return err
}

// parseRFC3339 parses an RFC3339 timestamp stored in the database. On parse
// failure it logs the bad value and returns the zero time so callers always
// receive a valid time.Time.
func parseRFC3339(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		log.Printf("sqlite: failed to parse RFC3339 timestamp %q: %v", s, err)
	}
	return t
}

func scanPolicy(row *sql.Row) (*admin.CommunicationPolicy, error) {
	var p admin.CommunicationPolicy
	var direction, action, createdAt string
	if err := row.Scan(&p.ID, &p.OrgID, &p.SourceID, &p.TargetID, &direction, &action,
		&p.Priority, &p.CreatedBy, &createdAt); err != nil {
		return nil, err
	}
	p.Direction = admin.PolicyDirection(direction)
	p.Action = admin.PolicyAction(action)
	p.CreatedAt = parseRFC3339(createdAt)
	return &p, nil
}

// --- AuditStore ---

func (s *SQLiteStore) AppendAuditEvent(ctx context.Context, e *admin.AuditEvent) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_events(id, org_id, user_id, username, action, target_type, target_id, detail, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		e.ID, e.OrgID, e.UserID, e.Username, e.Action,
		e.TargetType, e.TargetID, e.Detail, e.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) ListAuditEvents(ctx context.Context, orgID string, limit int) ([]admin.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, org_id, user_id, username, action, target_type, target_id, detail, created_at
		 FROM audit_events WHERE org_id=? ORDER BY created_at DESC LIMIT ?`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var events []admin.AuditEvent
	for rows.Next() {
		var e admin.AuditEvent
		var createdAt string
		if err := rows.Scan(&e.ID, &e.OrgID, &e.UserID, &e.Username,
			&e.Action, &e.TargetType, &e.TargetID, &e.Detail, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt = parseRFC3339(createdAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

// --- APIKeyStore ---

func (s *SQLiteStore) CreateAPIKey(ctx context.Context, k *admin.APIKey) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_keys(id, org_id, name, key_hash, role, agent_id, created_by, created_at, revoked_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		k.ID, k.OrgID, k.Name, k.KeyHash, string(k.Role), k.AgentID,
		k.CreatedBy, k.CreatedAt.Format(time.RFC3339), "",
	)
	return err
}

// GetAPIKeyByHash returns the key row regardless of revocation status.
// SECURITY: callers MUST check RevokedAt.IsZero() before trusting the key.
// The gateway auth path (handlers_api.go) performs this check; any new caller must too.
func (s *SQLiteStore) GetAPIKeyByHash(ctx context.Context, hash string) (*admin.APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, key_hash, role, agent_id, created_by, created_at, revoked_at
		 FROM api_keys WHERE key_hash=?`, hash)
	return scanAPIKey(row)
}

// GetAPIKeyByAgentID returns the active API key bound to agentID within orgID.
// Returns an error if no matching active (non-revoked) key is found.
func (s *SQLiteStore) GetAPIKeyByAgentID(ctx context.Context, orgID, agentID string) (*admin.APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, key_hash, role, agent_id, created_by, created_at, revoked_at
		 FROM api_keys WHERE org_id=? AND agent_id=? AND (revoked_at IS NULL OR revoked_at='')
		 ORDER BY created_at DESC LIMIT 1`, orgID, agentID)
	return scanAPIKey(row)
}

// ListAPIKeys returns all active (non-revoked) API keys for the given org.
// Revoked keys (revoked_at != '') are intentionally excluded.
// Use GetAPIKeyByHash to look up any key regardless of revocation status.
func (s *SQLiteStore) ListAPIKeys(ctx context.Context, orgID string) ([]admin.APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, org_id, name, key_hash, role, agent_id, created_by, created_at, revoked_at
		 FROM api_keys WHERE org_id=? AND (revoked_at IS NULL OR revoked_at='')
		 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var keys []admin.APIKey
	for rows.Next() {
		var k admin.APIKey
		var role, agentID, createdAt, revokedAt string
		if err := rows.Scan(&k.ID, &k.OrgID, &k.Name, &k.KeyHash, &role, &agentID,
			&k.CreatedBy, &createdAt, &revokedAt); err != nil {
			return nil, err
		}
		k.Role = admin.Role(role)
		k.AgentID = agentID
		k.CreatedAt = parseRFC3339(createdAt)
		if revokedAt != "" {
			k.RevokedAt = parseRFC3339(revokedAt)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// RevokeAPIKey marks the key as revoked. orgID is required so that a caller
// cannot revoke keys belonging to a different organization (CWE-639).
// Returns an error if no matching active key is found for the (id, orgID) pair.
func (s *SQLiteStore) RevokeAPIKey(ctx context.Context, orgID, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at=? WHERE id=? AND org_id=?`,
		time.Now().UTC().Format(time.RFC3339), id, orgID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("api key %q not found in org %q", id, orgID)
	}
	return nil
}

func scanAPIKey(row *sql.Row) (*admin.APIKey, error) {
	var k admin.APIKey
	var role, agentID, createdAt, revokedAt string
	if err := row.Scan(&k.ID, &k.OrgID, &k.Name, &k.KeyHash, &role, &agentID,
		&k.CreatedBy, &createdAt, &revokedAt); err != nil {
		return nil, err
	}
	k.Role = admin.Role(role)
	k.AgentID = agentID
	k.CreatedAt = parseRFC3339(createdAt)
	if revokedAt != "" {
		k.RevokedAt = parseRFC3339(revokedAt)
	}
	return &k, nil
}

// --- MessageLogStore ---

const defaultMessageLogLimit = 200

func (s *SQLiteStore) AppendMessageLog(ctx context.Context, m *admin.MessageLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO message_logs
		 (id, org_id, envelope_id, thread_id, from_agent, to_agent, msg_type, direction, tool_name, body_size, body, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.OrgID, m.EnvelopeID, m.ThreadID,
		m.FromAgent, m.ToAgent, m.MsgType, m.Direction, m.ToolName,
		m.BodySize, m.Body, m.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (s *SQLiteStore) ListMessageLogs(ctx context.Context, orgID string, filter admin.MessageLogFilter) ([]admin.MessageLog, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultMessageLogLimit
	}

	query := `SELECT id, org_id, envelope_id, thread_id, from_agent, to_agent,
	                 msg_type, direction, tool_name, body_size, body, created_at
	          FROM message_logs WHERE org_id=?`
	args := []any{orgID}

	if filter.AgentID != "" {
		query += ` AND (from_agent=? OR to_agent=?)`
		args = append(args, filter.AgentID, filter.AgentID)
	}
	if filter.ToolName != "" {
		query += ` AND tool_name=?`
		args = append(args, filter.ToolName)
	}
	if !filter.Since.IsZero() {
		query += ` AND created_at >= ?`
		args = append(args, filter.Since.Format(time.RFC3339))
	}
	if !filter.Until.IsZero() {
		query += ` AND created_at <= ?`
		args = append(args, filter.Until.Format(time.RFC3339))
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var logs []admin.MessageLog
	for rows.Next() {
		var m admin.MessageLog
		var createdAt string
		if err := rows.Scan(&m.ID, &m.OrgID, &m.EnvelopeID, &m.ThreadID,
			&m.FromAgent, &m.ToAgent, &m.MsgType, &m.Direction, &m.ToolName,
			&m.BodySize, &m.Body, &createdAt); err != nil {
			return nil, err
		}
		m.CreatedAt = parseRFC3339(createdAt)
		logs = append(logs, m)
	}
	return logs, rows.Err()
}

func (s *SQLiteStore) DeleteMessageLogsBefore(ctx context.Context, orgID string, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM message_logs WHERE org_id=? AND created_at < ?`,
		orgID, before.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- LimitStore ---

func (s *SQLiteStore) UpsertAgentLimits(ctx context.Context, l *admin.AgentLimits) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_limits
		 (id, org_id, agent_id, max_body_bytes, max_send_per_min, max_pending_tasks, max_stream_len, updated_at, updated_by)
		 VALUES(?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(org_id, agent_id) DO UPDATE SET
		   id=excluded.id,
		   max_body_bytes=excluded.max_body_bytes,
		   max_send_per_min=excluded.max_send_per_min,
		   max_pending_tasks=excluded.max_pending_tasks,
		   max_stream_len=excluded.max_stream_len,
		   updated_at=excluded.updated_at,
		   updated_by=excluded.updated_by`,
		l.ID, l.OrgID, l.AgentID,
		l.MaxBodyBytes, l.MaxSendPerMin, l.MaxPendingTasks, l.MaxStreamLen,
		l.UpdatedAt.Format(time.RFC3339), l.UpdatedBy,
	)
	return err
}

// GetEffectiveLimits returns the agent-specific row if present, or the global
// default row (agent_id="*") if not. Returns nil, nil when neither exists.
func (s *SQLiteStore) GetEffectiveLimits(ctx context.Context, orgID, agentID string) (*admin.AgentLimits, error) {
	// Try agent-specific first.
	row := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, agent_id, max_body_bytes, max_send_per_min, max_pending_tasks, max_stream_len, updated_at, updated_by
		 FROM agent_limits WHERE org_id=? AND agent_id=?`,
		orgID, agentID,
	)
	l, err := scanAgentLimits(row)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	// Fall back to global default.
	row = s.db.QueryRowContext(ctx,
		`SELECT id, org_id, agent_id, max_body_bytes, max_send_per_min, max_pending_tasks, max_stream_len, updated_at, updated_by
		 FROM agent_limits WHERE org_id=? AND agent_id='*'`,
		orgID,
	)
	l, err = scanAgentLimits(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return l, err
}

func (s *SQLiteStore) ListAgentLimits(ctx context.Context, orgID string) ([]admin.AgentLimits, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, org_id, agent_id, max_body_bytes, max_send_per_min, max_pending_tasks, max_stream_len, updated_at, updated_by
		 FROM agent_limits WHERE org_id=? ORDER BY agent_id`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []admin.AgentLimits
	for rows.Next() {
		var l admin.AgentLimits
		var updatedAt string
		if err := rows.Scan(&l.ID, &l.OrgID, &l.AgentID,
			&l.MaxBodyBytes, &l.MaxSendPerMin, &l.MaxPendingTasks, &l.MaxStreamLen,
			&updatedAt, &l.UpdatedBy); err != nil {
			return nil, err
		}
		l.UpdatedAt = parseRFC3339(updatedAt)
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DeleteAgentLimits(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM agent_limits WHERE id=?`, id)
	return err
}

func scanAgentLimits(row *sql.Row) (*admin.AgentLimits, error) {
	var l admin.AgentLimits
	var updatedAt string
	if err := row.Scan(&l.ID, &l.OrgID, &l.AgentID,
		&l.MaxBodyBytes, &l.MaxSendPerMin, &l.MaxPendingTasks, &l.MaxStreamLen,
		&updatedAt, &l.UpdatedBy); err != nil {
		return nil, err
	}
	l.UpdatedAt = parseRFC3339(updatedAt)
	return &l, nil
}
