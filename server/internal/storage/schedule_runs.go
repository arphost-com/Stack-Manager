package storage

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
)

// ListScheduleRuns returns the newest scheduled update attempts. Local jobs
// and callback-agent commands are joined at read time so running entries become
// completed or failed without duplicating potentially large command output.
func (s *Store) ListScheduleRuns(ctx context.Context, limit int) ([]core.UpdateScheduleRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT r.id, r.schedule_id, r.agent_id, r.agent_name, r.project, r.action,
			COALESCE(r.job_id, ''),
			CASE
				WHEN j.id IS NOT NULL THEN j.status
				WHEN c.id IS NOT NULL AND c.status='done' THEN 'completed'
				WHEN c.id IS NOT NULL AND c.status='error' THEN 'failed'
				WHEN c.id IS NOT NULL THEN c.status
				ELSE r.status
			END AS resolved_status,
			CASE
				WHEN j.id IS NOT NULL THEN j.success
				WHEN c.id IS NOT NULL THEN c.success
				WHEN r.status='skipped' THEN TRUE
				ELSE FALSE
			END AS resolved_success,
			LEFT(COALESCE(j.output, c.output, r.output, ''), 32768),
			COALESCE(NULLIF(j.error, ''), NULLIF(r.error, ''), ''),
			r.started_at,
			CASE
				WHEN j.id IS NOT NULL THEN j.ended_at
				WHEN c.id IS NOT NULL AND c.status IN ('done', 'error') THEN c.updated_at
				ELSE r.ended_at
			END,
			CASE WHEN j.id IS NOT NULL THEN j.duration ELSE r.duration END
		FROM update_schedule_runs r
		LEFT JOIN jobs j ON j.id=r.job_id
		LEFT JOIN agent_commands c ON r.job_id=CONCAT('agent-cmd-', c.id)
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]core.UpdateScheduleRun, 0, limit)
	for rows.Next() {
		var run core.UpdateScheduleRun
		if err := rows.Scan(
			&run.ID,
			&run.ScheduleID,
			&run.AgentID,
			&run.AgentName,
			&run.Project,
			&run.Action,
			&run.JobID,
			&run.Status,
			&run.Success,
			&run.Output,
			&run.Error,
			&run.StartedAt,
			&run.EndedAt,
			&run.Duration,
		); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// CompleteScheduleRun reconciles a dispatched local job with its final state.
// Calls for manual jobs are expected and intentionally become no-ops.
func (s *Store) CompleteScheduleRun(ctx context.Context, jobID, status, output, errText string, endedAt time.Time, duration string) error {
	if jobID == "" {
		return nil
	}
	if endedAt.IsZero() {
		endedAt = time.Now().UTC()
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `UPDATE update_schedule_runs
		SET status=?, output=?, error=?, ended_at=?, duration=?
		WHERE job_id=? AND status<>?`, status, nullableString(output), nullableString(errText), endedAt.UTC(), duration, jobID, status)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE update_schedules
		SET last_status=?, last_error=?, updated_at=?
		WHERE last_job_id=?`, status, nullableString(errText), time.Now().UTC(), jobID); err != nil {
		return err
	}
	if err := insertScheduleCompletionAuditTx(ctx, tx, jobID, status, errText, endedAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.DeleteCache(ctx, "schedules:list")
	return nil
}

// backfillLatestScheduleRuns preserves the most recent pre-migration run for
// each schedule so the new page is useful immediately after an upgrade.
func (s *Store) backfillLatestScheduleRuns(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO update_schedule_runs
			(schedule_id, agent_id, agent_name, project, action, job_id, status, error, started_at)
		SELECT s.id, s.agent_id, COALESCE(a.name, ''), s.project, s.action,
			NULLIF(s.last_job_id, ''), s.last_status, s.last_error, s.last_run_at
		FROM update_schedules s
		LEFT JOIN compose_agents a ON a.id=s.agent_id
		WHERE s.last_run_at IS NOT NULL
			AND NOT EXISTS (
				SELECT 1 FROM update_schedule_runs r
				WHERE r.schedule_id=s.id AND r.started_at=s.last_run_at
			)`)
	return err
}

func (s *Store) reconcileLatestScheduleStatuses(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `
		UPDATE update_schedules s
		JOIN jobs j ON j.id=s.last_job_id
		SET s.last_status=j.status,
			s.last_error=NULLIF(j.error, ''),
			s.updated_at=GREATEST(s.updated_at, COALESCE(j.ended_at, j.started_at))`); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `
		UPDATE update_schedules s
		JOIN agent_commands c ON s.last_job_id=CONCAT('agent-cmd-', c.id)
		SET s.last_status=CASE
				WHEN c.status='done' THEN 'completed'
				WHEN c.status='error' THEN 'failed'
				ELSE c.status
			END,
			s.last_error=CASE WHEN c.status='error' THEN 'agent command failed' ELSE NULL END,
			s.updated_at=GREATEST(s.updated_at, c.updated_at)`)
	return err
}

// insertScheduleRunTx snapshots the schedule target so history remains
// readable even if the schedule is later edited or deleted.
func insertScheduleRunTx(ctx context.Context, tx *sql.Tx, scheduleID int64, jobID, status, errText string, startedAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO update_schedule_runs
			(schedule_id, agent_id, agent_name, project, action, job_id, status, error, started_at)
		SELECT s.id, s.agent_id, COALESCE(a.name, ''), s.project, s.action, ?, ?, ?, ?
		FROM update_schedules s
		LEFT JOIN compose_agents a ON a.id=s.agent_id
		WHERE s.id=?`,
		nullableString(jobID), status, nullableString(errText), startedAt.UTC(), scheduleID)
	return err
}

func insertScheduleAuditTx(ctx context.Context, tx *sql.Tx, scheduleID int64, jobID, status, errText string, createdAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_log
			(node, actor, action, project, target, success, duration_ms, details, remote_ip, created_at)
		SELECT ?, 'scheduler', CONCAT('scheduled_', s.action, '_', ?), s.project,
			COALESCE(NULLIF(a.name, ''), 'this server'), ? NOT IN ('failed', 'unknown'), 0,
			CONCAT('schedule_id=', s.id,
				CASE WHEN ?<>'' THEN CONCAT(' job_id=', ?) ELSE '' END,
				CASE WHEN ?<>'' THEN CONCAT(' error=', ?) ELSE '' END),
			'', ?
		FROM update_schedules s
		LEFT JOIN compose_agents a ON a.id=s.agent_id
		WHERE s.id=?`,
		os.Getenv("AUDIT_NODE_NAME"), status, status, jobID, jobID, errText, errText, createdAt.UTC(), scheduleID)
	return err
}

func insertScheduleCompletionAuditTx(ctx context.Context, tx *sql.Tx, jobID, status, errText string, createdAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_log
			(node, actor, action, project, target, success, duration_ms, details, remote_ip, created_at)
		SELECT ?, 'scheduler', CONCAT('scheduled_', r.action, '_', ?), r.project,
			COALESCE(NULLIF(r.agent_name, ''), 'this server'), ? NOT IN ('failed', 'unknown'), 0,
			CONCAT('schedule_id=', r.schedule_id, ' job_id=', ?,
				CASE WHEN ?<>'' THEN CONCAT(' error=', ?) ELSE '' END),
			'', ?
		FROM update_schedule_runs r
		WHERE r.job_id=?
		ORDER BY r.id DESC
		LIMIT 1`,
		os.Getenv("AUDIT_NODE_NAME"), status, status, jobID, errText, errText, createdAt.UTC(), jobID)
	return err
}
