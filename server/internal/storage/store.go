package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/arphost-com/Stack-Manager/server/internal/core"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	DB       *sql.DB
	Redis    *redis.Client
	CacheTTL time.Duration
}

func (s *Store) ImportLegacyFiles(ctx context.Context, stateDir string) error {
	if stateDir == "" {
		return nil
	}
	if err := s.importLegacyUsers(ctx, filepath.Join(stateDir, "users.json")); err != nil {
		return err
	}
	if err := s.importLegacyJobs(ctx, filepath.Join(stateDir, "jobs")); err != nil {
		return err
	}
	return nil
}

func New(ctx context.Context, dsn, redisAddr, redisPassword string, redisDB int, cacheTTL time.Duration) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	var pingErr error
	for i := 0; i < 30; i++ {
		pingErr = db.PingContext(ctx)
		if pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		_ = db.Close()
		return nil, pingErr
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = db.Close()
		_ = rdb.Close()
		return nil, err
	}

	s := &Store{DB: db, Redis: rdb, CacheTTL: cacheTTL}
	if err := s.Migrate(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			username VARCHAR(64) PRIMARY KEY,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(32) NOT NULL,
			created_at DATETIME(6) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id VARCHAR(64) PRIMARY KEY,
			project VARCHAR(255) NOT NULL,
			action VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL,
			success BOOLEAN NOT NULL,
			exit_code INT NOT NULL,
			output MEDIUMTEXT NOT NULL,
			started_at DATETIME(6) NOT NULL,
			ended_at DATETIME(6) NULL,
			duration VARCHAR(64) NOT NULL DEFAULT '',
			error TEXT NULL,
			INDEX idx_jobs_started_at (started_at),
			INDEX idx_jobs_project_started_at (project, started_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS project_settings (
			project_name VARCHAR(255) PRIMARY KEY,
			update_policy VARCHAR(32) NOT NULL DEFAULT 'auto',
			source_type VARCHAR(64) NOT NULL DEFAULT '',
			source_url TEXT NULL,
			no_updates_reason TEXT NULL,
			notes TEXT NULL,
			auto_detected BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS image_update_checks (
			project_name VARCHAR(255) NOT NULL,
			service VARCHAR(255) NOT NULL,
			image VARCHAR(1024) NOT NULL,
			local_digest VARCHAR(255) NOT NULL DEFAULT '',
			remote_digest VARCHAR(255) NOT NULL DEFAULT '',
			status VARCHAR(64) NOT NULL,
			update_available BOOLEAN NOT NULL DEFAULT FALSE,
			error TEXT NULL,
			checked_at DATETIME(6) NOT NULL,
			PRIMARY KEY (project_name, service),
			INDEX idx_image_update_checks_available (update_available),
			INDEX idx_image_update_checks_checked_at (checked_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS compose_agents (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(128) NOT NULL UNIQUE,
			base_url VARCHAR(512) NOT NULL,
			token TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			last_seen DATETIME(6) NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			INDEX idx_compose_agents_enabled (enabled)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS compose_agent_project_snapshots (
			agent_id BIGINT PRIMARY KEY,
			projects_json JSON NOT NULL,
			received_at DATETIME(6) NOT NULL,
			CONSTRAINT fk_compose_agent_project_snapshots_agent FOREIGN KEY (agent_id) REFERENCES compose_agents(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS update_schedules (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			agent_id BIGINT NULL,
			project VARCHAR(255) NOT NULL,
			action VARCHAR(64) NOT NULL DEFAULT 'update',
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			interval_minutes INT NOT NULL,
			timeout_seconds INT NOT NULL DEFAULT 300,
			next_run_at DATETIME(6) NOT NULL,
			last_run_at DATETIME(6) NULL,
			last_job_id VARCHAR(128) NOT NULL DEFAULT '',
			last_status VARCHAR(64) NOT NULL DEFAULT '',
			last_error TEXT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			INDEX idx_update_schedules_due (enabled, next_run_at),
			INDEX idx_update_schedules_agent_project (agent_id, project),
			CONSTRAINT fk_update_schedules_agent FOREIGN KEY (agent_id) REFERENCES compose_agents(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS backup_destinations (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(128) NOT NULL UNIQUE,
			type VARCHAR(32) NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			config_json JSON NOT NULL,
			secret_json JSON NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			INDEX idx_backup_destinations_enabled (enabled),
			INDEX idx_backup_destinations_type (type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS container_metric_snapshots (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			project VARCHAR(255) NOT NULL,
			container VARCHAR(255) NOT NULL,
			cpu_percent DOUBLE NOT NULL DEFAULT 0,
			memory_percent DOUBLE NOT NULL DEFAULT 0,
			memory_usage_bytes BIGINT NOT NULL DEFAULT 0,
			memory_limit_bytes BIGINT NOT NULL DEFAULT 0,
			net_rx_bytes BIGINT NOT NULL DEFAULT 0,
			net_tx_bytes BIGINT NOT NULL DEFAULT 0,
			block_read_bytes BIGINT NOT NULL DEFAULT 0,
			block_write_bytes BIGINT NOT NULL DEFAULT 0,
			pids INT NOT NULL DEFAULT 0,
			sampled_at DATETIME(6) NOT NULL,
			INDEX idx_metric_sampled_at (sampled_at),
			INDEX idx_metric_project_sampled_at (project, sampled_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS backup_events (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			project VARCHAR(255) NOT NULL,
			backup_id VARCHAR(255) NOT NULL,
			event_type VARCHAR(32) NOT NULL,
			destination_id BIGINT NULL,
			destination_name VARCHAR(128) NOT NULL DEFAULT '',
			target TEXT NULL,
			size_bytes BIGINT NOT NULL DEFAULT 0,
			success BOOLEAN NOT NULL DEFAULT TRUE,
			error TEXT NULL,
			created_at DATETIME(6) NOT NULL,
			INDEX idx_backup_events_created_at (created_at),
			INDEX idx_backup_events_project_created_at (project, created_at),
			INDEX idx_backup_events_event_type (event_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS backup_schedules (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			projects_json JSON NOT NULL,
			destination_ids_json JSON NOT NULL,
			interval_minutes INT NOT NULL,
			next_run_at DATETIME(6) NOT NULL,
			last_run_at DATETIME(6) NULL,
			last_status VARCHAR(64) NOT NULL DEFAULT '',
			last_error TEXT NULL,
			last_backup_ids_json JSON NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			INDEX idx_backup_schedules_due (enabled, next_run_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			node VARCHAR(128) NOT NULL DEFAULT '',
			actor VARCHAR(128) NOT NULL DEFAULT '',
			action VARCHAR(64) NOT NULL,
			project VARCHAR(255) NOT NULL DEFAULT '',
			target VARCHAR(255) NOT NULL DEFAULT '',
			success BOOLEAN NOT NULL DEFAULT TRUE,
			duration_ms INT NOT NULL DEFAULT 0,
			details TEXT NULL,
			remote_ip VARCHAR(64) NOT NULL DEFAULT '',
			created_at DATETIME(6) NOT NULL,
			INDEX idx_audit_created_at (created_at),
			INDEX idx_audit_node_created_at (node, created_at),
			INDEX idx_audit_action_created_at (action, created_at),
			INDEX idx_audit_project_created_at (project, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, stmt := range stmts {
		if _, err := s.DB.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListBackupDestinations(ctx context.Context) ([]core.BackupDestination, error) {
	var cached []core.BackupDestination
	if s.GetJSON(ctx, "backup_destinations:list", &cached) {
		return cached, nil
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, type, enabled, config_json, COALESCE(JSON_LENGTH(secret_json), 0) > 0, created_at, updated_at FROM backup_destinations ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	destinations := make([]core.BackupDestination, 0)
	for rows.Next() {
		destination, err := scanBackupDestination(rows, false)
		if err != nil {
			return nil, err
		}
		destinations = append(destinations, *destination)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.SetJSON(ctx, "backup_destinations:list", destinations, s.CacheTTL)
	return destinations, nil
}

func (s *Store) GetBackupDestination(ctx context.Context, id int64) (*core.BackupDestination, map[string]string, error) {
	destination, secrets, err := scanBackupDestinationWithSecrets(s.DB.QueryRowContext(ctx, `SELECT id, name, type, enabled, config_json, secret_json, created_at, updated_at FROM backup_destinations WHERE id=?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	return destination, secrets, nil
}

func (s *Store) SaveBackupDestination(ctx context.Context, req core.BackupDestinationRequest) (*core.BackupDestination, error) {
	now := time.Now().UTC()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	configJSON, err := json.Marshal(cleanStringMap(req.Config))
	if err != nil {
		return nil, err
	}
	secrets := cleanStringMap(req.Secrets)
	if req.ID > 0 {
		var currentSecrets map[string]string
		if len(secrets) == 0 {
			_, current, err := s.GetBackupDestination(ctx, req.ID)
			if err != nil {
				return nil, err
			}
			currentSecrets = current
		} else {
			currentSecrets = secrets
		}
		secretJSON, err := json.Marshal(currentSecrets)
		if err != nil {
			return nil, err
		}
		res, err := s.DB.ExecContext(ctx, `UPDATE backup_destinations
			SET name=?, type=?, enabled=?, config_json=?, secret_json=?, updated_at=?
			WHERE id=?`,
			req.Name, req.Type, enabled, string(configJSON), string(secretJSON), now, req.ID)
		if err != nil {
			return nil, err
		}
		affected, err := res.RowsAffected()
		if err == nil && affected == 0 {
			return nil, ErrNotFound
		}
		s.DeleteCache(ctx, "backup_destinations:list")
		destination, _, err := s.GetBackupDestination(ctx, req.ID)
		return destination, err
	}
	secretJSON, err := json.Marshal(secrets)
	if err != nil {
		return nil, err
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO backup_destinations
		(name, type, enabled, config_json, secret_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			type=VALUES(type),
			enabled=VALUES(enabled),
			config_json=VALUES(config_json),
			secret_json=IF(JSON_LENGTH(VALUES(secret_json))=0, secret_json, VALUES(secret_json)),
			updated_at=VALUES(updated_at)`,
		req.Name, req.Type, enabled, string(configJSON), string(secretJSON), now, now)
	if err != nil {
		return nil, err
	}
	s.DeleteCache(ctx, "backup_destinations:list")
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		destination, _, err := s.GetBackupDestination(ctx, id)
		return destination, err
	}
	var destinationID int64
	if err := s.DB.QueryRowContext(ctx, `SELECT id FROM backup_destinations WHERE name=?`, req.Name).Scan(&destinationID); err != nil {
		return nil, err
	}
	destination, _, err := s.GetBackupDestination(ctx, destinationID)
	return destination, err
}

func (s *Store) DeleteBackupDestination(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM backup_destinations WHERE id=?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}
	s.DeleteCache(ctx, "backup_destinations:list")
	return nil
}

func (s *Store) SaveContainerMetricSnapshots(ctx context.Context, snapshots []core.ContainerMetricSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO container_metric_snapshots
		(project, container, cpu_percent, memory_percent, memory_usage_bytes, memory_limit_bytes, net_rx_bytes, net_tx_bytes, block_read_bytes, block_write_bytes, pids, sampled_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, snapshot := range snapshots {
		if _, err := stmt.ExecContext(ctx,
			snapshot.Project, snapshot.Container, snapshot.CPUPercent, snapshot.MemoryPercent, snapshot.MemoryUsageBytes, snapshot.MemoryLimitBytes,
			snapshot.NetRxBytes, snapshot.NetTxBytes, snapshot.BlockReadBytes, snapshot.BlockWriteBytes, snapshot.PIDs, snapshot.SampledAt.UTC(),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.DeleteCache(ctx, "metrics:summary", "metrics:history:24:", "metrics:backup_activity:24")
	return nil
}

func (s *Store) SaveBackupEvent(ctx context.Context, event core.BackupEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO backup_events
		(project, backup_id, event_type, destination_id, destination_name, target, size_bytes, success, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.Project, event.BackupID, event.EventType, nullableInt64(zeroNilInt64(event.DestinationID)), event.DestinationName,
		nullableString(event.Target), event.SizeBytes, event.Success, nullableString(event.Error), event.CreatedAt.UTC())
	if err == nil {
		s.DeleteCache(ctx, "metrics:summary", "metrics:backup_activity:24")
	}
	return err
}

func (s *Store) MetricsSummary(ctx context.Context) (core.MetricsSummary, error) {
	var cached core.MetricsSummary
	if s.GetJSON(ctx, "metrics:summary", &cached) {
		return cached, nil
	}
	var summary core.MetricsSummary
	var latest sql.NullTime
	if err := s.DB.QueryRowContext(ctx, `SELECT MAX(sampled_at) FROM container_metric_snapshots`).Scan(&latest); err != nil {
		return summary, err
	}
	if latest.Valid {
		summary.LastSampledAt = &latest.Time
		rows, err := s.DB.QueryContext(ctx, `SELECT project, COUNT(*), AVG(cpu_percent), AVG(memory_percent), SUM(memory_usage_bytes), SUM(net_rx_bytes), SUM(net_tx_bytes)
			FROM container_metric_snapshots
			WHERE sampled_at=?
			GROUP BY project
			ORDER BY project`, latest.Time)
		if err != nil {
			return summary, err
		}
		for rows.Next() {
			var point core.MetricHistoryPoint
			if err := rows.Scan(&point.Project, &point.ContainerCount, &point.CPUPercentAvg, &point.MemoryPercentAvg, &point.MemoryUsageBytes, &point.NetRxBytes, &point.NetTxBytes); err != nil {
				_ = rows.Close()
				return summary, err
			}
			summary.ContainerCount += point.ContainerCount
			summary.CPUPercentAvg += point.CPUPercentAvg * float64(point.ContainerCount)
			summary.MemoryPercentAvg += point.MemoryPercentAvg * float64(point.ContainerCount)
			summary.MemoryUsageBytes += point.MemoryUsageBytes
			summary.NetRxBytes += point.NetRxBytes
			summary.NetTxBytes += point.NetTxBytes
			point.SampledAt = latest.Time
			summary.ProjectSnapshots = append(summary.ProjectSnapshots, point)
		}
		if err := rows.Close(); err != nil {
			return summary, err
		}
		if summary.ContainerCount > 0 {
			summary.CPUPercentAvg = summary.CPUPercentAvg / float64(summary.ContainerCount)
			summary.MemoryPercentAvg = summary.MemoryPercentAvg / float64(summary.ContainerCount)
		}
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	rows, err := s.DB.QueryContext(ctx, `SELECT event_type, COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM backup_events
		WHERE created_at >= ? AND success=TRUE
		GROUP BY event_type`, cutoff)
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var eventType string
		var count int
		var bytes int64
		if err := rows.Scan(&eventType, &count, &bytes); err != nil {
			_ = rows.Close()
			return summary, err
		}
		switch eventType {
		case "backup":
			summary.BackupCount24h = count
			summary.BackupBytes24h = bytes
		case "restore":
			summary.RestoreCount24h = count
		case "upload":
			summary.UploadBytes24h = bytes
		}
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}
	activity, err := s.BackupActivity(ctx, 24)
	if err == nil {
		summary.BackupActivity24h = activity
	}
	s.SetJSON(ctx, "metrics:summary", summary, s.CacheTTL)
	return summary, nil
}

func (s *Store) MetricHistory(ctx context.Context, hours int, project string) ([]core.MetricHistoryPoint, error) {
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	key := fmt.Sprintf("metrics:history:%d:%s", hours, project)
	var cached []core.MetricHistoryPoint
	if s.GetJSON(ctx, key, &cached) {
		return cached, nil
	}
	cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	query := `SELECT sampled_at, project, COUNT(*), AVG(cpu_percent), AVG(memory_percent), SUM(memory_usage_bytes), SUM(net_rx_bytes), SUM(net_tx_bytes)
		FROM container_metric_snapshots
		WHERE sampled_at >= ?`
	args := []interface{}{cutoff}
	if project != "" {
		query += ` AND project = ?`
		args = append(args, project)
	}
	query += ` GROUP BY sampled_at, project ORDER BY sampled_at ASC, project ASC LIMIT 2000`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := make([]core.MetricHistoryPoint, 0)
	for rows.Next() {
		var point core.MetricHistoryPoint
		if err := rows.Scan(&point.SampledAt, &point.Project, &point.ContainerCount, &point.CPUPercentAvg, &point.MemoryPercentAvg, &point.MemoryUsageBytes, &point.NetRxBytes, &point.NetTxBytes); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.SetJSON(ctx, key, points, s.CacheTTL)
	return points, nil
}

func (s *Store) BackupActivity(ctx context.Context, hours int) ([]core.BackupActivityPoint, error) {
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	key := fmt.Sprintf("metrics:backup_activity:%d", hours)
	var cached []core.BackupActivityPoint
	if s.GetJSON(ctx, key, &cached) {
		return cached, nil
	}
	cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.DB.QueryContext(ctx, `SELECT created_at, event_type, size_bytes FROM backup_events WHERE created_at >= ? AND success=TRUE ORDER BY created_at ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	buckets := map[time.Time]*core.BackupActivityPoint{}
	for rows.Next() {
		var createdAt time.Time
		var eventType string
		var sizeBytes int64
		if err := rows.Scan(&createdAt, &eventType, &sizeBytes); err != nil {
			return nil, err
		}
		bucket := createdAt.UTC().Truncate(time.Hour)
		point := buckets[bucket]
		if point == nil {
			point = &core.BackupActivityPoint{BucketStart: bucket}
			buckets[bucket] = point
		}
		switch eventType {
		case "backup":
			point.Backups++
			point.BackupBytes += sizeBytes
		case "restore":
			point.Restores++
			point.RestoreBytes += sizeBytes
		case "delete":
			point.Deletes++
		case "upload":
			point.Uploads++
			point.UploadBytes += sizeBytes
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	points := make([]core.BackupActivityPoint, 0, len(buckets))
	for _, point := range buckets {
		points = append(points, *point)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].BucketStart.Before(points[j].BucketStart)
	})
	s.SetJSON(ctx, key, points, s.CacheTTL)
	return points, nil
}

func (s *Store) ListAgents(ctx context.Context) ([]core.ComposeAgent, error) {
	var cached []core.ComposeAgent
	if s.GetJSON(ctx, "agents:list", &cached) {
		return cached, nil
	}

	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, base_url, token, enabled, last_seen, created_at, updated_at FROM compose_agents ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := make([]core.ComposeAgent, 0)
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, *agent)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.SetJSON(ctx, "agents:list", agents, s.CacheTTL)
	return agents, nil
}

func (s *Store) GetAgent(ctx context.Context, id int64) (*core.ComposeAgent, error) {
	agent, err := scanAgent(s.DB.QueryRowContext(ctx, `SELECT id, name, base_url, token, enabled, last_seen, created_at, updated_at FROM compose_agents WHERE id=?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return agent, nil
}

func (s *Store) GetAgentByName(ctx context.Context, name string) (*core.ComposeAgent, error) {
	agent, err := scanAgent(s.DB.QueryRowContext(ctx, `SELECT id, name, base_url, token, enabled, last_seen, created_at, updated_at FROM compose_agents WHERE name=?`, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return agent, nil
}

func (s *Store) SaveAgent(ctx context.Context, req core.ComposeAgentRequest) (*core.ComposeAgent, error) {
	now := time.Now().UTC()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO compose_agents
		(name, base_url, token, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			base_url=VALUES(base_url),
			token=IF(VALUES(token)='', token, VALUES(token)),
			enabled=VALUES(enabled),
			updated_at=VALUES(updated_at)`,
		req.Name, req.BaseURL, req.Token, enabled, now, now)
	if err != nil {
		return nil, err
	}
	s.DeleteCache(ctx, "agents:list")
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		return s.GetAgent(ctx, id)
	}
	var agentID int64
	if err := s.DB.QueryRowContext(ctx, `SELECT id FROM compose_agents WHERE name=?`, req.Name).Scan(&agentID); err != nil {
		return nil, err
	}
	return s.GetAgent(ctx, agentID)
}

func (s *Store) SaveAgentProjectSnapshot(ctx context.Context, agentID int64, projects []core.Project) error {
	if projects == nil {
		projects = []core.Project{}
	}
	raw, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.DB.ExecContext(ctx, `INSERT INTO compose_agent_project_snapshots
		(agent_id, projects_json, received_at)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			projects_json=VALUES(projects_json),
			received_at=VALUES(received_at)`,
		agentID, string(raw), now)
	if err == nil {
		s.DeleteCache(ctx, "agents:list")
	}
	return err
}

func (s *Store) GetAgentProjectSnapshot(ctx context.Context, agentID int64) (*core.AgentProjectSnapshot, error) {
	var raw []byte
	var receivedAt time.Time
	if err := s.DB.QueryRowContext(ctx, `SELECT projects_json, received_at FROM compose_agent_project_snapshots WHERE agent_id=?`, agentID).Scan(&raw, &receivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var projects []core.Project
	if err := json.Unmarshal(raw, &projects); err != nil {
		return nil, err
	}
	return &core.AgentProjectSnapshot{AgentID: agentID, Projects: projects, ReceivedAt: receivedAt}, nil
}

func (s *Store) DeleteAgent(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM compose_agents WHERE id=?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}
	s.DeleteCache(ctx, "agents:list", "schedules:list")
	return nil
}

func (s *Store) TouchAgent(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE compose_agents SET last_seen=?, updated_at=? WHERE id=?`, time.Now().UTC(), time.Now().UTC(), id)
	if err == nil {
		s.DeleteCache(ctx, "agents:list")
	}
	return err
}

func (s *Store) ListSchedules(ctx context.Context) ([]core.UpdateSchedule, error) {
	var cached []core.UpdateSchedule
	if s.GetJSON(ctx, "schedules:list", &cached) {
		return cached, nil
	}
	schedules, err := s.querySchedules(ctx, `SELECT s.id, s.agent_id, COALESCE(a.name, ''), s.project, s.action, s.enabled, s.interval_minutes, s.timeout_seconds, s.next_run_at, s.last_run_at, s.last_job_id, s.last_status, s.last_error, s.created_at, s.updated_at
		FROM update_schedules s
		LEFT JOIN compose_agents a ON a.id=s.agent_id
		ORDER BY s.enabled DESC, s.next_run_at ASC`)
	if err != nil {
		return nil, err
	}
	s.SetJSON(ctx, "schedules:list", schedules, s.CacheTTL)
	return schedules, nil
}

func (s *Store) ListDueSchedules(ctx context.Context, now time.Time) ([]core.UpdateSchedule, error) {
	return s.querySchedules(ctx, `SELECT s.id, s.agent_id, COALESCE(a.name, ''), s.project, s.action, s.enabled, s.interval_minutes, s.timeout_seconds, s.next_run_at, s.last_run_at, s.last_job_id, s.last_status, s.last_error, s.created_at, s.updated_at
		FROM update_schedules s
		LEFT JOIN compose_agents a ON a.id=s.agent_id
		WHERE s.enabled=TRUE AND s.next_run_at <= ?
		ORDER BY s.next_run_at ASC`, now)
}

func (s *Store) GetSchedule(ctx context.Context, id int64) (*core.UpdateSchedule, error) {
	schedules, err := s.querySchedules(ctx, `SELECT s.id, s.agent_id, COALESCE(a.name, ''), s.project, s.action, s.enabled, s.interval_minutes, s.timeout_seconds, s.next_run_at, s.last_run_at, s.last_job_id, s.last_status, s.last_error, s.created_at, s.updated_at
		FROM update_schedules s
		LEFT JOIN compose_agents a ON a.id=s.agent_id
		WHERE s.id=?`, id)
	if err != nil {
		return nil, err
	}
	if len(schedules) == 0 {
		return nil, ErrNotFound
	}
	return &schedules[0], nil
}

func (s *Store) SaveSchedule(ctx context.Context, req core.UpdateScheduleRequest) (*core.UpdateSchedule, error) {
	now := time.Now().UTC()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	action := req.Action
	if action == "" {
		action = "update"
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 300
	}
	nextRun := now.Add(time.Duration(req.IntervalMinutes) * time.Minute)
	if req.NextRunAt != nil && !req.NextRunAt.IsZero() {
		nextRun = req.NextRunAt.UTC()
	}
	if req.ID > 0 {
		res, err := s.DB.ExecContext(ctx, `UPDATE update_schedules
			SET agent_id=?, project=?, action=?, enabled=?, interval_minutes=?, timeout_seconds=?, next_run_at=?, updated_at=?
			WHERE id=?`,
			nullableInt64(req.AgentID), req.Project, action, enabled, req.IntervalMinutes, timeout, nextRun, now, req.ID)
		if err != nil {
			return nil, err
		}
		affected, err := res.RowsAffected()
		if err == nil && affected == 0 {
			return nil, ErrNotFound
		}
		s.DeleteCache(ctx, "schedules:list")
		return s.GetSchedule(ctx, req.ID)
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO update_schedules
		(agent_id, project, action, enabled, interval_minutes, timeout_seconds, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullableInt64(req.AgentID), req.Project, action, enabled, req.IntervalMinutes, timeout, nextRun, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	s.DeleteCache(ctx, "schedules:list")
	return s.GetSchedule(ctx, id)
}

func (s *Store) DeleteSchedule(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM update_schedules WHERE id=?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}
	s.DeleteCache(ctx, "schedules:list")
	return nil
}

func (s *Store) MarkScheduleDispatched(ctx context.Context, id int64, nextRun time.Time, jobID, status, errText string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE update_schedules
		SET next_run_at=?, last_run_at=?, last_job_id=?, last_status=?, last_error=?, updated_at=?
		WHERE id=?`,
		nextRun.UTC(), time.Now().UTC(), jobID, status, nullableString(errText), time.Now().UTC(), id)
	if err == nil {
		s.DeleteCache(ctx, "schedules:list")
	}
	return err
}

func (s *Store) ListBackupSchedules(ctx context.Context) ([]core.BackupSchedule, error) {
	var cached []core.BackupSchedule
	if s.GetJSON(ctx, "backup_schedules:list", &cached) {
		return cached, nil
	}
	schedules, err := s.queryBackupSchedules(ctx, `SELECT id, name, enabled, projects_json, destination_ids_json, interval_minutes, next_run_at, last_run_at, last_status, last_error, last_backup_ids_json, created_at, updated_at
		FROM backup_schedules
		ORDER BY enabled DESC, next_run_at ASC`)
	if err != nil {
		return nil, err
	}
	s.SetJSON(ctx, "backup_schedules:list", schedules, s.CacheTTL)
	return schedules, nil
}

func (s *Store) ListDueBackupSchedules(ctx context.Context, now time.Time) ([]core.BackupSchedule, error) {
	return s.queryBackupSchedules(ctx, `SELECT id, name, enabled, projects_json, destination_ids_json, interval_minutes, next_run_at, last_run_at, last_status, last_error, last_backup_ids_json, created_at, updated_at
		FROM backup_schedules
		WHERE enabled=TRUE AND next_run_at <= ?
		ORDER BY next_run_at ASC`, now)
}

func (s *Store) GetBackupSchedule(ctx context.Context, id int64) (*core.BackupSchedule, error) {
	schedules, err := s.queryBackupSchedules(ctx, `SELECT id, name, enabled, projects_json, destination_ids_json, interval_minutes, next_run_at, last_run_at, last_status, last_error, last_backup_ids_json, created_at, updated_at
		FROM backup_schedules
		WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	if len(schedules) == 0 {
		return nil, ErrNotFound
	}
	return &schedules[0], nil
}

func (s *Store) SaveBackupSchedule(ctx context.Context, req core.BackupScheduleRequest) (*core.BackupSchedule, error) {
	now := time.Now().UTC()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.IntervalMinutes < 5 {
		req.IntervalMinutes = 1440
	}
	nextRun := now.Add(time.Duration(req.IntervalMinutes) * time.Minute)
	if req.NextRunAt != nil && !req.NextRunAt.IsZero() {
		nextRun = req.NextRunAt.UTC()
	}
	projectsJSON, _ := json.Marshal(req.Projects)
	destinationsJSON, _ := json.Marshal(req.DestinationIDs)
	if req.ID > 0 {
		res, err := s.DB.ExecContext(ctx, `UPDATE backup_schedules
			SET name=?, enabled=?, projects_json=?, destination_ids_json=?, interval_minutes=?, next_run_at=?, updated_at=?
			WHERE id=?`,
			req.Name, enabled, projectsJSON, destinationsJSON, req.IntervalMinutes, nextRun, now, req.ID)
		if err != nil {
			return nil, err
		}
		affected, err := res.RowsAffected()
		if err == nil && affected == 0 {
			return nil, ErrNotFound
		}
		s.DeleteCache(ctx, "backup_schedules:list")
		return s.GetBackupSchedule(ctx, req.ID)
	}
	emptyIDs, _ := json.Marshal([]string{})
	res, err := s.DB.ExecContext(ctx, `INSERT INTO backup_schedules
		(name, enabled, projects_json, destination_ids_json, interval_minutes, next_run_at, last_backup_ids_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Name, enabled, projectsJSON, destinationsJSON, req.IntervalMinutes, nextRun, emptyIDs, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	s.DeleteCache(ctx, "backup_schedules:list")
	return s.GetBackupSchedule(ctx, id)
}

func (s *Store) DeleteBackupSchedule(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM backup_schedules WHERE id=?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}
	s.DeleteCache(ctx, "backup_schedules:list")
	return nil
}

func (s *Store) MarkBackupScheduleDispatched(ctx context.Context, id int64, nextRun time.Time, backupIDs []string, status, errText string) error {
	idsJSON, _ := json.Marshal(backupIDs)
	_, err := s.DB.ExecContext(ctx, `UPDATE backup_schedules
		SET next_run_at=?, last_run_at=?, last_status=?, last_error=?, last_backup_ids_json=?, updated_at=?
		WHERE id=?`,
		nextRun.UTC(), time.Now().UTC(), status, nullableString(errText), idsJSON, time.Now().UTC(), id)
	if err == nil {
		s.DeleteCache(ctx, "backup_schedules:list")
	}
	return err
}

func (s *Store) querySchedules(ctx context.Context, query string, args ...interface{}) ([]core.UpdateSchedule, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := make([]core.UpdateSchedule, 0)
	for rows.Next() {
		schedule, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, *schedule)
	}
	return schedules, rows.Err()
}

func (s *Store) queryBackupSchedules(ctx context.Context, query string, args ...interface{}) ([]core.BackupSchedule, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := make([]core.BackupSchedule, 0)
	for rows.Next() {
		schedule, err := scanBackupSchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, *schedule)
	}
	return schedules, rows.Err()
}

type agentScanner interface {
	Scan(dest ...interface{}) error
}

func scanAgent(scanner agentScanner) (*core.ComposeAgent, error) {
	var agent core.ComposeAgent
	var lastSeen sql.NullTime
	if err := scanner.Scan(&agent.ID, &agent.Name, &agent.BaseURL, &agent.Token, &agent.Enabled, &lastSeen, &agent.CreatedAt, &agent.UpdatedAt); err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		agent.LastSeen = &lastSeen.Time
	}
	if agent.BaseURL == "" {
		agent.Mode = "callback"
	} else {
		agent.Mode = "inbound"
	}
	return &agent, nil
}

func scanSchedule(scanner jobScanner) (*core.UpdateSchedule, error) {
	var schedule core.UpdateSchedule
	var agentID sql.NullInt64
	var lastRun sql.NullTime
	var lastError sql.NullString
	if err := scanner.Scan(&schedule.ID, &agentID, &schedule.AgentName, &schedule.Project, &schedule.Action, &schedule.Enabled, &schedule.IntervalMinutes, &schedule.TimeoutSeconds, &schedule.NextRunAt, &lastRun, &schedule.LastJobID, &schedule.LastStatus, &lastError, &schedule.CreatedAt, &schedule.UpdatedAt); err != nil {
		return nil, err
	}
	if agentID.Valid {
		schedule.AgentID = &agentID.Int64
	}
	if lastRun.Valid {
		schedule.LastRunAt = &lastRun.Time
	}
	if lastError.Valid {
		schedule.LastError = lastError.String
	}
	return &schedule, nil
}

func scanBackupSchedule(scanner jobScanner) (*core.BackupSchedule, error) {
	var schedule core.BackupSchedule
	var projectsRaw []byte
	var destinationsRaw []byte
	var backupIDsRaw []byte
	var lastRun sql.NullTime
	var lastError sql.NullString
	if err := scanner.Scan(&schedule.ID, &schedule.Name, &schedule.Enabled, &projectsRaw, &destinationsRaw, &schedule.IntervalMinutes, &schedule.NextRunAt, &lastRun, &schedule.LastStatus, &lastError, &backupIDsRaw, &schedule.CreatedAt, &schedule.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(projectsRaw, &schedule.Projects)
	_ = json.Unmarshal(destinationsRaw, &schedule.DestinationIDs)
	_ = json.Unmarshal(backupIDsRaw, &schedule.LastBackupIDs)
	if lastRun.Valid {
		schedule.LastRunAt = &lastRun.Time
	}
	if lastError.Valid {
		schedule.LastError = lastError.String
	}
	return &schedule, nil
}

func scanBackupDestination(scanner jobScanner, includeSecrets bool) (*core.BackupDestination, error) {
	if includeSecrets {
		destination, _, err := scanBackupDestinationWithSecrets(scanner)
		return destination, err
	}
	var destination core.BackupDestination
	var configRaw []byte
	if err := scanner.Scan(&destination.ID, &destination.Name, &destination.Type, &destination.Enabled, &configRaw, &destination.HasSecret, &destination.CreatedAt, &destination.UpdatedAt); err != nil {
		return nil, err
	}
	destination.Config = map[string]string{}
	if len(configRaw) > 0 {
		_ = json.Unmarshal(configRaw, &destination.Config)
	}
	return &destination, nil
}

func scanBackupDestinationWithSecrets(scanner jobScanner) (*core.BackupDestination, map[string]string, error) {
	var destination core.BackupDestination
	var configRaw []byte
	var secretRaw []byte
	if err := scanner.Scan(&destination.ID, &destination.Name, &destination.Type, &destination.Enabled, &configRaw, &secretRaw, &destination.CreatedAt, &destination.UpdatedAt); err != nil {
		return nil, nil, err
	}
	destination.Config = map[string]string{}
	if len(configRaw) > 0 {
		_ = json.Unmarshal(configRaw, &destination.Config)
	}
	secrets := map[string]string{}
	if len(secretRaw) > 0 {
		_ = json.Unmarshal(secretRaw, &secrets)
	}
	destination.HasSecret = len(secrets) > 0
	return &destination, secrets, nil
}

func (s *Store) importLegacyUsers(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var payload struct {
		Users []struct {
			Username     string    `json:"username"`
			PasswordHash string    `json:"password_hash"`
			Role         string    `json:"role"`
			CreatedAt    time.Time `json:"created_at"`
		} `json:"users"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	for _, user := range payload.Users {
		if user.Username == "" || user.PasswordHash == "" {
			continue
		}
		if user.CreatedAt.IsZero() {
			user.CreatedAt = time.Now().UTC()
		}
		if user.Role == "" {
			user.Role = "operator"
		}
		if _, err := s.DB.ExecContext(ctx, `INSERT IGNORE INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`, user.Username, user.PasswordHash, user.Role, user.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) importLegacyJobs(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		var job core.ActionJob
		if err := json.Unmarshal(data, &job); err != nil {
			return err
		}
		if job.ID == "" {
			continue
		}
		if err := s.SaveJob(ctx, &job); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetJSON(ctx context.Context, key string, dest interface{}) bool {
	if s.Redis == nil {
		return false
	}
	raw, err := s.Redis.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, dest) == nil
}

func (s *Store) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	if s.Redis == nil {
		return
	}
	if ttl <= 0 {
		ttl = s.CacheTTL
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	_ = s.Redis.Set(ctx, key, raw, ttl).Err()
}

func (s *Store) DeleteCache(ctx context.Context, keys ...string) {
	if s.Redis == nil || len(keys) == 0 {
		return
	}
	_ = s.Redis.Del(ctx, keys...).Err()
}

func (s *Store) SaveJob(ctx context.Context, job *core.ActionJob) error {
	cp := core.JobSnapshot(job)
	var endedAt interface{}
	if !cp.EndedAt.IsZero() {
		endedAt = cp.EndedAt
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO jobs
		(id, project, action, status, success, exit_code, output, started_at, ended_at, duration, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			project=VALUES(project),
			action=VALUES(action),
			status=VALUES(status),
			success=VALUES(success),
			exit_code=VALUES(exit_code),
			output=VALUES(output),
			started_at=VALUES(started_at),
			ended_at=VALUES(ended_at),
			duration=VALUES(duration),
			error=VALUES(error)`,
		cp.ID, cp.Project, cp.Action, cp.Status, cp.Success, cp.ExitCode, cp.Output, cp.StartedAt, endedAt, cp.Duration, nullableString(cp.Error))
	if err == nil {
		s.SetJSON(ctx, "job:"+cp.ID, cp, time.Hour)
		s.DeleteCache(ctx, "jobs:list")
	}
	return err
}

func (s *Store) LoadJob(ctx context.Context, id string) (*core.ActionJob, error) {
	var cached core.ActionJob
	if s.GetJSON(ctx, "job:"+id, &cached) {
		return &cached, nil
	}

	job, err := scanJob(s.DB.QueryRowContext(ctx, `SELECT id, project, action, status, success, exit_code, output, started_at, ended_at, duration, error FROM jobs WHERE id=?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s.SetJSON(ctx, "job:"+id, job, time.Hour)
	return job, nil
}

func (s *Store) ListJobs(ctx context.Context) ([]core.ActionJob, error) {
	var cached []core.ActionJob
	if s.GetJSON(ctx, "jobs:list", &cached) {
		return cached, nil
	}

	rows, err := s.DB.QueryContext(ctx, `SELECT id, project, action, status, success, exit_code, output, started_at, ended_at, duration, error FROM jobs ORDER BY started_at DESC LIMIT 300`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]core.ActionJob, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.SetJSON(ctx, "jobs:list", jobs, s.CacheTTL)
	return jobs, nil
}

type jobScanner interface {
	Scan(dest ...interface{}) error
}

func scanJob(scanner jobScanner) (*core.ActionJob, error) {
	var job core.ActionJob
	var endedAt sql.NullTime
	var errText sql.NullString
	if err := scanner.Scan(&job.ID, &job.Project, &job.Action, &job.Status, &job.Success, &job.ExitCode, &job.Output, &job.StartedAt, &endedAt, &job.Duration, &errText); err != nil {
		return nil, err
	}
	if endedAt.Valid {
		job.EndedAt = endedAt.Time
	}
	if errText.Valid {
		job.Error = errText.String
	}
	return &job, nil
}

func (s *Store) SaveProjectUpdateStatus(ctx context.Context, projectName string, status core.ProjectUpdateStatus) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM image_update_checks WHERE project_name=?`, projectName); err != nil {
		_ = tx.Rollback()
		return err
	}
	if len(status.Images) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO image_update_checks
			(project_name, service, image, local_digest, remote_digest, status, update_available, error, checked_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		defer stmt.Close()
		for _, image := range status.Images {
			if _, err := stmt.ExecContext(ctx,
				projectName, image.Service, image.Image, image.LocalDigest, image.RemoteDigest, image.Status,
				image.UpdateAvailable, nullableString(image.Error), image.CheckedAt.UTC(),
			); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.DeleteCache(ctx, "project_updates:"+projectName, "projects:list")
	return nil
}

func (s *Store) ProjectUpdateStatus(ctx context.Context, projectName string) (core.ProjectUpdateStatus, error) {
	var cached core.ProjectUpdateStatus
	if s.GetJSON(ctx, "project_updates:"+projectName, &cached) {
		return cached, nil
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT project_name, service, image, local_digest, remote_digest, status, update_available, error, checked_at
		FROM image_update_checks
		WHERE project_name=?
		ORDER BY service`, projectName)
	if err != nil {
		return core.ProjectUpdateStatus{}, err
	}
	defer rows.Close()

	status := core.ProjectUpdateStatus{}
	var latest *time.Time
	for rows.Next() {
		var check core.ImageUpdateCheck
		var errText sql.NullString
		if err := rows.Scan(&check.Project, &check.Service, &check.Image, &check.LocalDigest, &check.RemoteDigest, &check.Status, &check.UpdateAvailable, &errText, &check.CheckedAt); err != nil {
			return core.ProjectUpdateStatus{}, err
		}
		if errText.Valid {
			check.Error = errText.String
		}
		status.Checked = true
		status.RegistryImages++
		status.Images = append(status.Images, check)
		if check.UpdateAvailable {
			status.Available = true
			status.Count++
		}
		if check.Error != "" && status.Error == "" {
			status.Error = check.Error
		}
		t := check.CheckedAt.UTC()
		if latest == nil || t.After(*latest) {
			cp := t
			latest = &cp
		}
	}
	if err := rows.Err(); err != nil {
		return core.ProjectUpdateStatus{}, err
	}
	if latest != nil {
		status.CheckedAt = latest
		next := latest.Add(24 * time.Hour)
		status.NextCheckAt = &next
	}
	s.SetJSON(ctx, "project_updates:"+projectName, status, s.CacheTTL)
	return status, nil
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt64(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func zeroNilInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func cleanStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func (s *Store) ResolveUpdatePolicy(project core.Project) core.ProjectUpdatePolicy {
	ctx := context.Background()
	detected := core.DetectProjectUpdatePolicy(project)
	var cached core.ProjectUpdatePolicy
	if s.GetJSON(ctx, "project_policy:"+project.Name, &cached) {
		cached.DetectedPolicy = detected.DetectedPolicy
		cached.DetectedSourceType = detected.DetectedSourceType
		cached.DetectedSourceURL = detected.DetectedSourceURL
		cached.DetectedReason = detected.DetectedReason
		if cached.Mode == "auto" {
			cached.EffectivePolicy = detected.EffectivePolicy
			cached.SourceType = detected.SourceType
			cached.SourceURL = detected.SourceURL
			cached.NoUpdatesReason = detected.NoUpdatesReason
			cached.AutoDetected = detected.AutoDetected
		}
		return cached
	}

	policy, err := s.loadProjectPolicy(ctx, project.Name, detected)
	if err != nil {
		return detected
	}
	s.SetJSON(ctx, "project_policy:"+project.Name, policy, s.CacheTTL)
	return policy
}

func (s *Store) GetProjectPolicy(ctx context.Context, project core.Project) (core.ProjectUpdatePolicy, error) {
	detected := core.DetectProjectUpdatePolicy(project)
	return s.loadProjectPolicy(ctx, project.Name, detected)
}

func (s *Store) SetProjectPolicy(ctx context.Context, project core.Project, mode, notes string) (core.ProjectUpdatePolicy, error) {
	if mode == "" {
		mode = "auto"
	}
	if !core.ValidProjectUpdatePolicyMode(mode) {
		return core.ProjectUpdatePolicy{}, fmt.Errorf("invalid update policy: %s", mode)
	}
	detected := core.DetectProjectUpdatePolicy(project)
	resolved := core.ResolveProjectUpdatePolicy(mode, detected)
	resolved.Notes = notes
	now := time.Now().UTC()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO project_settings
		(project_name, update_policy, source_type, source_url, no_updates_reason, notes, auto_detected, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			update_policy=VALUES(update_policy),
			source_type=VALUES(source_type),
			source_url=VALUES(source_url),
			no_updates_reason=VALUES(no_updates_reason),
			notes=VALUES(notes),
			auto_detected=VALUES(auto_detected),
			updated_at=VALUES(updated_at)`,
		project.Name, mode, resolved.SourceType, nullableString(resolved.SourceURL), nullableString(resolved.NoUpdatesReason), nullableString(notes), resolved.AutoDetected, now, now)
	if err != nil {
		return core.ProjectUpdatePolicy{}, err
	}
	s.DeleteCache(ctx, "project_policy:"+project.Name, "projects:list")
	s.SetJSON(ctx, "project_policy:"+project.Name, resolved, s.CacheTTL)
	return resolved, nil
}

func (s *Store) loadProjectPolicy(ctx context.Context, projectName string, detected core.ProjectUpdatePolicy) (core.ProjectUpdatePolicy, error) {
	var mode string
	var sourceType string
	var sourceURL sql.NullString
	var reason sql.NullString
	var notes sql.NullString
	var autoDetected bool
	err := s.DB.QueryRowContext(ctx, `SELECT update_policy, source_type, source_url, no_updates_reason, notes, auto_detected FROM project_settings WHERE project_name=?`, projectName).
		Scan(&mode, &sourceType, &sourceURL, &reason, &notes, &autoDetected)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return detected, nil
		}
		return detected, err
	}

	resolved := core.ResolveProjectUpdatePolicy(mode, detected)
	if sourceType != "" {
		resolved.SourceType = sourceType
	}
	if sourceURL.Valid {
		resolved.SourceURL = sourceURL.String
	}
	if reason.Valid {
		resolved.NoUpdatesReason = reason.String
	}
	if notes.Valid {
		resolved.Notes = notes.String
	}
	if mode != "auto" {
		resolved.AutoDetected = autoDetected
	}
	return resolved, nil
}
