package api

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	pactmigrate "pactmigrate.local/packages/core"
)

const runHistoryTableName = "pactmigrate_run_history"

const (
	runHistoryColRequestID  = "request_id"
	runHistoryColMode       = "mode"
	runHistoryColPlanID     = "plan_id"
	runHistoryColActorID    = "actor_id"
	runHistoryColActorRoles = "actor_roles"
)

func (s *API) findEnvironment(name string) (EnvironmentConfig, bool) {
	for _, env := range s.cfg.Environments {
		if env.Name == name {
			return env, true
		}
	}
	return EnvironmentConfig{}, false
}

func (s *API) loadMigrations() ([]pactmigrate.Migration, error) {
	fsys, fsDir, err := s.resolveFS()
	if err != nil {
		return nil, err
	}
	migs, err := pactmigrate.Load(fsys, fsDir)
	if err != nil {
		return nil, err
	}
	return migs, nil
}

func (s *API) resolveFS() (fs.FS, string, error) {
	switch s.cfg.Migrations.Source {
	case "dir":
		abs, err := filepath.Abs(s.cfg.Migrations.Dir)
		if err != nil {
			return nil, "", fmt.Errorf("migrations.dir: %w", err)
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, "", fmt.Errorf("migrations.dir: %w", err)
		}
		if !st.IsDir() {
			return nil, "", fmt.Errorf("migrations.dir is not a directory: %s", abs)
		}
		return os.DirFS(abs), s.cfg.Migrations.FSDir, nil
	default:
		return nil, "", fmt.Errorf("unsupported migrations.source %q", s.cfg.Migrations.Source)
	}
}

func (s *API) fetchApplied(ctx context.Context, env EnvironmentConfig) (map[string]pactmigrate.AppliedRecord, error) {
	db, err := sql.Open(env.Driver, env.DSN)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	defer db.Close()

	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("conn: %w", err)
	}
	defer conn.Close()

	dialect, err := parseDialect(env.Dialect)
	if err != nil {
		return nil, err
	}
	store, err := pactmigrate.NewSQLStore(dialect, env.TableName)
	if err != nil {
		return nil, err
	}
	if err := store.Init(ctx, conn); err != nil {
		return nil, err
	}
	applied, err := store.ListApplied(ctx, conn)
	if err != nil {
		return nil, err
	}
	return applied, nil
}

func parseDialect(s string) (pactmigrate.Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "postgres", "postgresql":
		return pactmigrate.DialectPostgres, nil
	case "mysql", "mariadb":
		return pactmigrate.DialectMySQL, nil
	default:
		return 0, fmt.Errorf("unknown dialect %q", s)
	}
}

func (s *API) hasFailure(env, key string) bool {
	s.failureMu.RLock()
	defer s.failureMu.RUnlock()
	m := s.failures[env]
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

func (s *API) markFailure(env, key, msg string) {
	s.failureMu.Lock()
	defer s.failureMu.Unlock()
	if s.failures[env] == nil {
		s.failures[env] = make(map[string]string)
	}
	s.failures[env][key] = msg
}

func (s *API) clearFailure(env, key string) {
	s.failureMu.Lock()
	defer s.failureMu.Unlock()
	if s.failures[env] == nil {
		return
	}
	delete(s.failures[env], key)
}

func openEnvironmentDB(ctx context.Context, env EnvironmentConfig) (*sql.DB, error) {
	db, err := sql.Open(env.Driver, env.DSN)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

func ensureRunHistoryTable(ctx context.Context, db *sql.DB, dialect pactmigrate.Dialect) error {
	switch dialect {
	case pactmigrate.DialectPostgres:
		_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS pactmigrate_run_history (
  run_id TEXT PRIMARY KEY,
  request_id TEXT,
  environment TEXT NOT NULL,
  status TEXT NOT NULL,
  mode TEXT,
  plan_id TEXT,
  actor_id TEXT,
  actor_roles TEXT,
  started_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ NOT NULL,
  duration_ms BIGINT NOT NULL,
  planned_count INTEGER NOT NULL,
  applied_count INTEGER NOT NULL,
  remaining_count INTEGER NOT NULL,
  error_text TEXT
)`)
		if err != nil {
			return fmt.Errorf("init run history table: %w", err)
		}
		if err := ensureRunHistoryColumns(ctx, db, dialect); err != nil {
			return err
		}
	case pactmigrate.DialectMySQL:
		_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS pactmigrate_run_history (
  run_id VARCHAR(255) PRIMARY KEY,
  request_id VARCHAR(255) NULL,
  environment VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL,
  mode VARCHAR(16) NULL,
  plan_id VARCHAR(255) NULL,
  actor_id VARCHAR(255) NULL,
  actor_roles TEXT NULL,
  started_at DATETIME(6) NOT NULL,
  finished_at DATETIME(6) NOT NULL,
  duration_ms BIGINT NOT NULL,
  planned_count INT NOT NULL,
  applied_count INT NOT NULL,
  remaining_count INT NOT NULL,
  error_text TEXT NULL
)`)
		if err != nil {
			return fmt.Errorf("init run history table: %w", err)
		}
		if err := ensureRunHistoryColumns(ctx, db, dialect); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported dialect %q", dialect)
	}
	return nil
}

func ensureRunHistoryColumns(ctx context.Context, db *sql.DB, dialect pactmigrate.Dialect) error {
	type col struct {
		name string
		pg   string
		my   string
	}
	cols := []col{
		{name: runHistoryColRequestID, pg: "TEXT", my: "VARCHAR(255) NULL"},
		{name: runHistoryColMode, pg: "TEXT", my: "VARCHAR(16) NULL"},
		{name: runHistoryColPlanID, pg: "TEXT", my: "VARCHAR(255) NULL"},
		{name: runHistoryColActorID, pg: "TEXT", my: "VARCHAR(255) NULL"},
		{name: runHistoryColActorRoles, pg: "TEXT", my: "TEXT NULL"},
	}
	for _, c := range cols {
		exists, err := runHistoryColumnExists(ctx, db, dialect, c.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		switch dialect {
		case pactmigrate.DialectPostgres:
			if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, runHistoryTableName, c.name, c.pg)); err != nil {
				return fmt.Errorf("alter run history add column %s: %w", c.name, err)
			}
		case pactmigrate.DialectMySQL:
			if _, err := db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", runHistoryTableName, c.name, c.my)); err != nil {
				return fmt.Errorf("alter run history add column %s: %w", c.name, err)
			}
		}
	}
	return nil
}

func runHistoryColumnExists(ctx context.Context, db *sql.DB, dialect pactmigrate.Dialect, colName string) (bool, error) {
	switch dialect {
	case pactmigrate.DialectPostgres:
		var exists bool
		err := db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1
  FROM information_schema.columns
  WHERE table_name = $1
    AND column_name = $2
    AND table_schema = ANY(current_schemas(true))
)`, runHistoryTableName, colName).Scan(&exists)
		return exists, err
	case pactmigrate.DialectMySQL:
		var exists int
		err := db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = ?
  AND column_name = ?
`, runHistoryTableName, colName).Scan(&exists)
		return exists > 0, err
	default:
		return false, fmt.Errorf("unsupported dialect %q", dialect)
	}
}

func persistRunRecord(ctx context.Context, env EnvironmentConfig, record runRecord) error {
	dialect, err := parseDialect(env.Dialect)
	if err != nil {
		return err
	}
	db, err := openEnvironmentDB(ctx, env)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := ensureRunHistoryTable(ctx, db, dialect); err != nil {
		return err
	}

	switch dialect {
	case pactmigrate.DialectPostgres:
		_, err = db.ExecContext(ctx, `
INSERT INTO pactmigrate_run_history
  (run_id, request_id, environment, status, mode, plan_id, actor_id, actor_roles, started_at, finished_at, duration_ms, planned_count, applied_count, remaining_count, error_text)
VALUES
  ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (run_id) DO UPDATE SET
  request_id = EXCLUDED.request_id,
  environment = EXCLUDED.environment,
  status = EXCLUDED.status,
  mode = EXCLUDED.mode,
  plan_id = EXCLUDED.plan_id,
  actor_id = EXCLUDED.actor_id,
  actor_roles = EXCLUDED.actor_roles,
  started_at = EXCLUDED.started_at,
  finished_at = EXCLUDED.finished_at,
  duration_ms = EXCLUDED.duration_ms,
  planned_count = EXCLUDED.planned_count,
  applied_count = EXCLUDED.applied_count,
  remaining_count = EXCLUDED.remaining_count,
  error_text = EXCLUDED.error_text
`, record.ID, nullIfEmpty(record.RequestID), record.Environment, record.Status, nullIfEmpty(record.Mode), nullIfEmpty(record.PlanID), nullIfEmpty(record.ActorID), nullIfEmpty(record.ActorRoles), record.StartedAt, record.FinishedAt, record.DurationMillis, record.PlannedCount, record.AppliedCount, record.RemainingCount, nullIfEmpty(record.Error))
	case pactmigrate.DialectMySQL:
		_, err = db.ExecContext(ctx, `
INSERT INTO pactmigrate_run_history
  (run_id, request_id, environment, status, mode, plan_id, actor_id, actor_roles, started_at, finished_at, duration_ms, planned_count, applied_count, remaining_count, error_text)
VALUES
  (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE
  request_id = VALUES(request_id),
  environment = VALUES(environment),
  status = VALUES(status),
  mode = VALUES(mode),
  plan_id = VALUES(plan_id),
  actor_id = VALUES(actor_id),
  actor_roles = VALUES(actor_roles),
  started_at = VALUES(started_at),
  finished_at = VALUES(finished_at),
  duration_ms = VALUES(duration_ms),
  planned_count = VALUES(planned_count),
  applied_count = VALUES(applied_count),
  remaining_count = VALUES(remaining_count),
  error_text = VALUES(error_text)
`, record.ID, nullIfEmpty(record.RequestID), record.Environment, record.Status, nullIfEmpty(record.Mode), nullIfEmpty(record.PlanID), nullIfEmpty(record.ActorID), nullIfEmpty(record.ActorRoles), record.StartedAt, record.FinishedAt, record.DurationMillis, record.PlannedCount, record.AppliedCount, record.RemainingCount, nullIfEmpty(record.Error))
	default:
		return fmt.Errorf("unsupported dialect %q", dialect)
	}
	if err != nil {
		return fmt.Errorf("insert run history: %w", err)
	}
	return nil
}

func (s *API) listRecentRuns(ctx context.Context, limit int) ([]runRecord, error) {
	all := make([]runRecord, 0, limit*len(s.cfg.Environments))
	for _, env := range s.cfg.Environments {
		runs, err := listRecentRunsForEnvironment(ctx, env, limit)
		if err != nil {
			return nil, fmt.Errorf("environment %s: %w", env.Name, err)
		}
		all = append(all, runs...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].FinishedAt.After(all[j].FinishedAt)
	})
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func listRecentRunsForEnvironment(ctx context.Context, env EnvironmentConfig, limit int) ([]runRecord, error) {
	dialect, err := parseDialect(env.Dialect)
	if err != nil {
		return nil, err
	}
	db, err := openEnvironmentDB(ctx, env)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := ensureRunHistoryTable(ctx, db, dialect); err != nil {
		return nil, err
	}

	query := `
SELECT run_id, request_id, environment, status, mode, plan_id, actor_id, actor_roles, started_at, finished_at, duration_ms, planned_count, applied_count, remaining_count, error_text
FROM pactmigrate_run_history
ORDER BY finished_at DESC
`
	var rows *sql.Rows
	switch dialect {
	case pactmigrate.DialectPostgres:
		rows, err = db.QueryContext(ctx, query+"LIMIT $1", limit)
	case pactmigrate.DialectMySQL:
		rows, err = db.QueryContext(ctx, query+"LIMIT ?", limit)
	default:
		return nil, fmt.Errorf("unsupported dialect %q", dialect)
	}
	if err != nil {
		return nil, fmt.Errorf("query run history: %w", err)
	}
	defer rows.Close()

	out := make([]runRecord, 0)
	for rows.Next() {
		var rr runRecord
		var errText sql.NullString
		var requestID sql.NullString
		var mode sql.NullString
		var planID sql.NullString
		var actorID sql.NullString
		var actorRoles sql.NullString
		if err := rows.Scan(
			&rr.ID,
			&requestID,
			&rr.Environment,
			&rr.Status,
			&mode,
			&planID,
			&actorID,
			&actorRoles,
			&rr.StartedAt,
			&rr.FinishedAt,
			&rr.DurationMillis,
			&rr.PlannedCount,
			&rr.AppliedCount,
			&rr.RemainingCount,
			&errText,
		); err != nil {
			return nil, fmt.Errorf("scan run history: %w", err)
		}
		if requestID.Valid {
			rr.RequestID = requestID.String
		}
		if mode.Valid {
			rr.Mode = mode.String
		}
		if planID.Valid {
			rr.PlanID = planID.String
		}
		if actorID.Valid {
			rr.ActorID = actorID.String
		}
		if actorRoles.Valid {
			rr.ActorRoles = actorRoles.String
		}
		if errText.Valid {
			rr.Error = errText.String
		}
		out = append(out, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run history: %w", err)
	}
	return out, nil
}

func (s *API) latestRunFinishedAt(ctx context.Context) (string, error) {
	runs, err := s.listRecentRuns(ctx, 1)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", nil
	}
	return runs[0].FinishedAt.Format(time.RFC3339), nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func outOfOrderPolicyForEnv(env EnvironmentConfig) pactmigrate.OutOfOrderPolicy {
	// Core has two states today:
	// - AllowLate (default)
	// - Strict (reject any late-merged migration)
	//
	// We treat "allow_out_of_order" or "allow_late_migrations" as AllowLate,
	// otherwise Strict.
	if env.Policies.AllowOutOfOrder || env.Policies.AllowLateMigrations {
		return pactmigrate.OutOfOrderAllowLate
	}
	return pactmigrate.OutOfOrderStrict
}

func (s *API) rememberPlan(envName string) string {
	s.planMu.Lock()
	defer s.planMu.Unlock()
	id := fmt.Sprintf("%s-%d", envName, time.Now().UnixNano())
	s.latestPlan[envName] = planCacheEntry{
		PlanID:    id,
		CreatedAt: time.Now().UTC(),
	}
	return id
}

func (s *API) isPlanAccepted(envName, planID string) bool {
	if strings.TrimSpace(planID) == "" {
		return false
	}
	s.planMu.Lock()
	defer s.planMu.Unlock()
	entry, ok := s.latestPlan[envName]
	if !ok {
		return false
	}
	// Keep it short to avoid stale plans being used after schema drift.
	if time.Since(entry.CreatedAt) > 5*time.Minute {
		return false
	}
	return entry.PlanID == planID
}
