package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	pactmigrate "pactmigrate.local/packages/core"
)

type schemaSnapshot struct {
	Tables map[string]tableSnapshot
}

type tableSnapshot struct {
	Columns map[string]columnSnapshot
}

type columnSnapshot struct {
	Type     string
	Nullable bool
	Default  string
}

type schemaDiff struct {
	MissingTables []string `json:"missing_tables"`
	ExtraTables   []string `json:"extra_tables"`
	Tables        []struct {
		Table          string   `json:"table"`
		MissingColumns []string `json:"missing_columns"`
		ExtraColumns   []string `json:"extra_columns"`
		ChangedColumns []struct {
			Column string `json:"column"`
			From   string `json:"from"`
			To     string `json:"to"`
		} `json:"changed_columns"`
	} `json:"tables"`
}

func (s *API) handleSchemaDrift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.cfg.Auth.Enabled {
		a, ok := actorFromContext(r.Context())
		if !ok || !hasAnyRole(a, "viewer", "operator", "admin") {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}
	envName := strings.TrimSpace(r.URL.Query().Get("env"))
	if envName == "" {
		writeError(w, http.StatusBadRequest, "missing env query param")
		return
	}
	env, ok := s.findEnvironment(envName)
	if !ok {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}
	if strings.TrimSpace(env.ScratchDSN) == "" {
		writeError(w, http.StatusBadRequest, "environment has empty scratch_dsn")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	dialect, err := parseDialect(env.Dialect)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Build expected snapshot from scratch DB by applying migrations there.
	expected, buildInfo, err := s.buildExpectedSchemaSnapshot(ctx, env, dialect)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("expected schema: %v", err))
		return
	}

	// Get actual snapshot from live DB.
	actual, err := snapshotSchema(ctx, env, dialect, env.DSN, ignoreTablesForEnv(env, buildInfo))
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("actual schema: %v", err))
		return
	}

	diff := diffSchema(expected, actual)
	writeJSON(w, http.StatusOK, map[string]any{
		"environment": env.Name,
		"build":       buildInfo,
		"diff":        diff,
	})
}

func (s *API) buildExpectedSchemaSnapshot(ctx context.Context, env EnvironmentConfig, dialect pactmigrate.Dialect) (*schemaSnapshot, map[string]any, error) {
	fsys, fsDir, err := s.resolveFS()
	if err != nil {
		return nil, nil, err
	}

	// Apply migrations to scratch DB.
	db, err := sql.Open(env.Driver, env.ScratchDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("scratch sql open: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("scratch ping: %w", err)
	}

	// Use a dedicated migrations table for scratch to avoid clobbering live state conventions.
	scratchTable := env.TableName
	if scratchTable == "" {
		scratchTable = "schema_migrations"
	}
	scratchTable = scratchTable + "_scratch"

	m, err := pactmigrate.New(
		db,
		fsys,
		pactmigrate.WithFSDir(fsDir),
		pactmigrate.WithDialect(dialect),
		pactmigrate.WithTableName(scratchTable),
		pactmigrate.WithAllowChecksumMismatch(false),
		pactmigrate.WithOutOfOrderPolicy(pactmigrate.OutOfOrderAllowLate),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("scratch migrator: %w", err)
	}

	plan, err := m.Plan(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("scratch plan: %w", err)
	}
	start := time.Now()
	if err := m.Up(ctx); err != nil {
		return nil, nil, fmt.Errorf("scratch up: %w", err)
	}
	buildInfo := map[string]any{
		"scratch_table":  scratchTable,
		"planned_count":  len(plan.Pending),
		"duration_ms":    time.Since(start).Milliseconds(),
		"applied_in_run": len(plan.Pending), // scratch is expected to start empty for drift runs
	}

	snap, err := snapshotSchema(ctx, env, dialect, env.ScratchDSN, ignoreTablesForEnv(env, buildInfo))
	if err != nil {
		return nil, nil, fmt.Errorf("scratch snapshot: %w", err)
	}
	return snap, buildInfo, nil
}

func snapshotSchema(ctx context.Context, env EnvironmentConfig, dialect pactmigrate.Dialect, dsn string, ignore map[string]struct{}) (*schemaSnapshot, error) {
	db, err := sql.Open(env.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	switch dialect {
	case pactmigrate.DialectPostgres:
		return snapshotPostgres(ctx, db, ignore)
	case pactmigrate.DialectMySQL:
		return snapshotMySQL(ctx, db, ignore)
	default:
		return nil, fmt.Errorf("unsupported dialect %q", dialect)
	}
}

func snapshotPostgres(ctx context.Context, db *sql.DB, ignore map[string]struct{}) (*schemaSnapshot, error) {
	// Limit to public schema for now.
	rows, err := db.QueryContext(ctx, `
SELECT table_name, column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = 'public'
ORDER BY table_name, ordinal_position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &schemaSnapshot{Tables: map[string]tableSnapshot{}}
	for rows.Next() {
		var table, col, typ, nullable string
		var def sql.NullString
		if err := rows.Scan(&table, &col, &typ, &nullable, &def); err != nil {
			return nil, err
		}
		if _, skip := ignore[table]; skip {
			continue
		}
		t := out.Tables[table]
		if t.Columns == nil {
			t.Columns = map[string]columnSnapshot{}
		}
		t.Columns[col] = columnSnapshot{
			Type:     normalizeType(typ),
			Nullable: strings.EqualFold(nullable, "YES"),
			Default:  normalizeDefault(def.String),
		}
		out.Tables[table] = t
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func snapshotMySQL(ctx context.Context, db *sql.DB, ignore map[string]struct{}) (*schemaSnapshot, error) {
	rows, err := db.QueryContext(ctx, `
SELECT table_name, column_name, column_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = DATABASE()
ORDER BY table_name, ordinal_position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &schemaSnapshot{Tables: map[string]tableSnapshot{}}
	for rows.Next() {
		var table, col, typ, nullable string
		var def sql.NullString
		if err := rows.Scan(&table, &col, &typ, &nullable, &def); err != nil {
			return nil, err
		}
		if _, skip := ignore[table]; skip {
			continue
		}
		t := out.Tables[table]
		if t.Columns == nil {
			t.Columns = map[string]columnSnapshot{}
		}
		t.Columns[col] = columnSnapshot{
			Type:     normalizeType(typ),
			Nullable: strings.EqualFold(nullable, "YES"),
			Default:  normalizeDefault(def.String),
		}
		out.Tables[table] = t
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ignoreTablesForEnv(env EnvironmentConfig, buildInfo map[string]any) map[string]struct{} {
	ignore := map[string]struct{}{}

	// Migration metadata tables should not count as "drift".
	tableName := strings.TrimSpace(env.TableName)
	if tableName == "" {
		tableName = "schema_migrations"
	}
	ignore[tableName] = struct{}{}

	// Scratch migration metadata table (only exists in scratch DB).
	if v, ok := buildInfo["scratch_table"].(string); ok && strings.TrimSpace(v) != "" {
		ignore[v] = struct{}{}
	} else {
		ignore[tableName+"_scratch"] = struct{}{}
	}

	// Run audit table (created on demand).
	ignore[runHistoryTableName] = struct{}{}

	return ignore
}

func diffSchema(expected, actual *schemaSnapshot) schemaDiff {
	diff := schemaDiff{}

	expTables := make([]string, 0, len(expected.Tables))
	actTables := make([]string, 0, len(actual.Tables))
	for t := range expected.Tables {
		expTables = append(expTables, t)
	}
	for t := range actual.Tables {
		actTables = append(actTables, t)
	}
	sort.Strings(expTables)
	sort.Strings(actTables)

	expSet := map[string]struct{}{}
	actSet := map[string]struct{}{}
	for _, t := range expTables {
		expSet[t] = struct{}{}
	}
	for _, t := range actTables {
		actSet[t] = struct{}{}
	}

	for _, t := range expTables {
		if _, ok := actSet[t]; !ok {
			diff.MissingTables = append(diff.MissingTables, t)
		}
	}
	for _, t := range actTables {
		if _, ok := expSet[t]; !ok {
			diff.ExtraTables = append(diff.ExtraTables, t)
		}
	}

	// Column-level diffs for intersecting tables.
	for _, table := range expTables {
		actT, ok := actual.Tables[table]
		if !ok {
			continue
		}
		expT := expected.Tables[table]

		var td struct {
			Table          string   `json:"table"`
			MissingColumns []string `json:"missing_columns"`
			ExtraColumns   []string `json:"extra_columns"`
			ChangedColumns []struct {
				Column string `json:"column"`
				From   string `json:"from"`
				To     string `json:"to"`
			} `json:"changed_columns"`
		}
		td.Table = table

		expCols := map[string]columnSnapshot{}
		actCols := map[string]columnSnapshot{}
		for c, v := range expT.Columns {
			expCols[c] = v
		}
		for c, v := range actT.Columns {
			actCols[c] = v
		}

		for c := range expCols {
			if _, ok := actCols[c]; !ok {
				td.MissingColumns = append(td.MissingColumns, c)
			}
		}
		for c := range actCols {
			if _, ok := expCols[c]; !ok {
				td.ExtraColumns = append(td.ExtraColumns, c)
			}
		}
		for c, expC := range expCols {
			actC, ok := actCols[c]
			if !ok {
				continue
			}
			from := columnSig(expC)
			to := columnSig(actC)
			if from != to {
				td.ChangedColumns = append(td.ChangedColumns, struct {
					Column string `json:"column"`
					From   string `json:"from"`
					To     string `json:"to"`
				}{Column: c, From: from, To: to})
			}
		}

		if len(td.MissingColumns) > 0 || len(td.ExtraColumns) > 0 || len(td.ChangedColumns) > 0 {
			sort.Strings(td.MissingColumns)
			sort.Strings(td.ExtraColumns)
			sort.Slice(td.ChangedColumns, func(i, j int) bool {
				return td.ChangedColumns[i].Column < td.ChangedColumns[j].Column
			})
			diff.Tables = append(diff.Tables, td)
		}
	}

	sort.Strings(diff.MissingTables)
	sort.Strings(diff.ExtraTables)
	sort.Slice(diff.Tables, func(i, j int) bool {
		return diff.Tables[i].Table < diff.Tables[j].Table
	})
	return diff
}

func columnSig(c columnSnapshot) string {
	nullable := "not null"
	if c.Nullable {
		nullable = "null"
	}
	def := ""
	if strings.TrimSpace(c.Default) != "" {
		def = " default " + c.Default
	}
	return fmt.Sprintf("%s %s%s", c.Type, nullable, def)
}

func normalizeType(t string) string {
	return strings.ToLower(strings.TrimSpace(t))
}

func normalizeDefault(d string) string {
	return strings.TrimSpace(d)
}
