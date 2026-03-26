// Package admin provides the HTTP server, handlers, middleware, and storage
// backend for the aga2aga web administration interface.
package admin

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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

// migrate creates all tables if they do not already exist.
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
`)
	return err
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
		`INSERT INTO api_keys(id, org_id, name, key_hash, role, created_by, created_at, revoked_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		k.ID, k.OrgID, k.Name, k.KeyHash, string(k.Role),
		k.CreatedBy, k.CreatedAt.Format(time.RFC3339), "",
	)
	return err
}

func (s *SQLiteStore) GetAPIKeyByHash(ctx context.Context, hash string) (*admin.APIKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, key_hash, role, created_by, created_at, revoked_at
		 FROM api_keys WHERE key_hash=?`, hash)
	return scanAPIKey(row)
}

// ListAPIKeys returns all active (non-revoked) API keys for the given org.
// Revoked keys (revoked_at != '') are intentionally excluded.
// Use GetAPIKeyByHash to look up any key regardless of revocation status.
func (s *SQLiteStore) ListAPIKeys(ctx context.Context, orgID string) ([]admin.APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, org_id, name, key_hash, role, created_by, created_at, revoked_at
		 FROM api_keys WHERE org_id=? AND (revoked_at IS NULL OR revoked_at='')
		 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var keys []admin.APIKey
	for rows.Next() {
		var k admin.APIKey
		var role, createdAt, revokedAt string
		if err := rows.Scan(&k.ID, &k.OrgID, &k.Name, &k.KeyHash, &role,
			&k.CreatedBy, &createdAt, &revokedAt); err != nil {
			return nil, err
		}
		k.Role = admin.Role(role)
		k.CreatedAt = parseRFC3339(createdAt)
		if revokedAt != "" {
			k.RevokedAt = parseRFC3339(revokedAt)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

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
	var role, createdAt, revokedAt string
	if err := row.Scan(&k.ID, &k.OrgID, &k.Name, &k.KeyHash, &role,
		&k.CreatedBy, &createdAt, &revokedAt); err != nil {
		return nil, err
	}
	k.Role = admin.Role(role)
	k.CreatedAt = parseRFC3339(createdAt)
	if revokedAt != "" {
		k.RevokedAt = parseRFC3339(revokedAt)
	}
	return &k, nil
}
