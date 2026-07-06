package storage

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// AuditEntry is a single row in the command audit log. Each mutating action
// (project up/down/restart/pull/update, backup create/restore, prune, registry
// login, docker daemon changes, SSL regen, etc.) writes one entry so operators
// can see a per-node history of who did what.
type AuditEntry struct {
	ID         int64     `json:"id"`
	Node       string    `json:"node"`
	Actor      string    `json:"actor"`
	Action     string    `json:"action"`
	Project    string    `json:"project"`
	Target     string    `json:"target"`
	Success    bool      `json:"success"`
	DurationMs int       `json:"duration_ms"`
	Details    string    `json:"details"`
	RemoteIP   string    `json:"remote_ip"`
	CreatedAt  time.Time `json:"created_at"`
}

// AuditListParams narrows an audit_log query. Empty fields skip the filter.
type AuditListParams struct {
	Node    string
	Actor   string
	Action  string
	Project string
	Since   time.Time
	Until   time.Time
	Success *bool
	Limit   int
	Offset  int
}

// InsertAuditEntry writes a single audit row. Callers should treat failures as
// non-fatal — logging must never block the actual action.
func (s *Store) InsertAuditEntry(ctx context.Context, e AuditEntry) (int64, error) {
	if strings.TrimSpace(e.Action) == "" {
		return 0, nil
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO audit_log (node, actor, action, project, target, success, duration_ms, details, remote_ip, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trimShort(e.Node, 128),
		trimShort(e.Actor, 128),
		trimShort(e.Action, 64),
		trimShort(e.Project, 255),
		trimShort(e.Target, 255),
		e.Success,
		e.DurationMs,
		trimLong(e.Details, 8192),
		trimShort(e.RemoteIP, 64),
		e.CreatedAt,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListAuditEntries returns rows matching the filter, newest first.
func (s *Store) ListAuditEntries(ctx context.Context, p AuditListParams) ([]AuditEntry, error) {
	if p.Limit <= 0 || p.Limit > 500 {
		p.Limit = 100
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	q := "SELECT id, node, actor, action, project, target, success, duration_ms, COALESCE(details, ''), remote_ip, created_at FROM audit_log WHERE 1=1"
	args := []interface{}{}
	if p.Node != "" {
		q += " AND node=?"
		args = append(args, p.Node)
	}
	if p.Actor != "" {
		q += " AND actor=?"
		args = append(args, p.Actor)
	}
	if p.Action != "" {
		q += " AND action=?"
		args = append(args, p.Action)
	}
	if p.Project != "" {
		q += " AND project=?"
		args = append(args, p.Project)
	}
	if !p.Since.IsZero() {
		q += " AND created_at >= ?"
		args = append(args, p.Since)
	}
	if !p.Until.IsZero() {
		q += " AND created_at <= ?"
		args = append(args, p.Until)
	}
	if p.Success != nil {
		q += " AND success=?"
		args = append(args, *p.Success)
	}
	q += " ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, p.Limit, p.Offset)

	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]AuditEntry, 0, p.Limit)
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Node, &e.Actor, &e.Action, &e.Project, &e.Target, &e.Success, &e.DurationMs, &e.Details, &e.RemoteIP, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// DistinctAuditNodes returns the set of nodes that have written audit rows,
// used to populate the per-node filter dropdown.
func (s *Store) DistinctAuditNodes(ctx context.Context) ([]string, error) {
	return s.distinctAuditColumn(ctx, "node")
}

// DistinctAuditActions returns the set of action names, used for filtering.
func (s *Store) DistinctAuditActions(ctx context.Context) ([]string, error) {
	return s.distinctAuditColumn(ctx, "action")
}

func (s *Store) distinctAuditColumn(ctx context.Context, col string) ([]string, error) {
	// col is a hardcoded literal from the caller, never user input, so
	// concatenating it into the query is safe (no injection surface).
	var q string
	switch col {
	case "node":
		q = "SELECT DISTINCT node FROM audit_log WHERE node <> '' ORDER BY node"
	case "action":
		q = "SELECT DISTINCT action FROM audit_log WHERE action <> '' ORDER BY action"
	default:
		return nil, sql.ErrNoRows
	}
	rows, err := s.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func trimShort(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max]
	}
	return s
}

func trimLong(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
