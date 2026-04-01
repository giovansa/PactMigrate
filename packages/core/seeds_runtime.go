package pactmigrate

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

type SeedColumnSchema struct {
	Name       string
	Nullable   bool
	HasDefault bool
}

type SeedTableSchema struct {
	Table             string
	Columns           map[string]SeedColumnSchema
	UniqueConstraints [][]string
}

type SeedRowAction struct {
	Action   string         `json:"action"`
	Identity map[string]any `json:"identity"`
}

type SeedPlanEntry struct {
	SeedID            string                `json:"seed_id"`
	Filename          string                `json:"filename"`
	Table             string                `json:"table"`
	RowCount          int                   `json:"row_count"`
	InsertCount       int                   `json:"insert_count"`
	UpdateCount       int                   `json:"update_count"`
	ValidationIssues  []SeedValidationIssue `json:"validation_issues"`
	Actions           []SeedRowAction       `json:"actions"`
}

type SeedPlan struct {
	SeedCount         int             `json:"seed_count"`
	InsertCount       int             `json:"insert_count"`
	UpdateCount       int             `json:"update_count"`
	ValidationIssues  int             `json:"validation_issue_count"`
	Entries           []SeedPlanEntry `json:"entries"`
}

type SeedApplyResult struct {
	SeedCount    int             `json:"seed_count"`
	AppliedCount int             `json:"applied_count"`
	InsertCount  int             `json:"insert_count"`
	UpdateCount  int             `json:"update_count"`
	Entries      []SeedPlanEntry `json:"entries"`
}

func PlanRequiredSeeds(ctx context.Context, db *sql.DB, fsys fs.FS, root string, dialect Dialect) (*SeedPlan, error) {
	if db == nil {
		return nil, fmt.Errorf("pactmigrate: database is nil")
	}
	seeds, err := loadRequiredSeedFiles(fsys, root)
	if err != nil {
		return nil, err
	}
	plan := &SeedPlan{
		Entries: make([]SeedPlanEntry, 0, len(seeds)),
	}
	for _, seed := range seeds {
		entry, err := planSeedEntry(ctx, db, dialect, seed)
		if err != nil {
			return nil, err
		}
		plan.SeedCount++
		plan.InsertCount += entry.InsertCount
		plan.UpdateCount += entry.UpdateCount
		plan.ValidationIssues += len(entry.ValidationIssues)
		plan.Entries = append(plan.Entries, entry)
	}
	return plan, nil
}

func ApplyRequiredSeeds(ctx context.Context, db *sql.DB, fsys fs.FS, root string, dialect Dialect) (*SeedApplyResult, error) {
	plan, err := PlanRequiredSeeds(ctx, db, fsys, root, dialect)
	if err != nil {
		return nil, err
	}
	for _, entry := range plan.Entries {
		if len(entry.ValidationIssues) > 0 {
			return nil, fmt.Errorf("pactmigrate: seed %s has validation issues", entry.SeedID)
		}
	}

	seeds, err := loadRequiredSeedFiles(fsys, root)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]seedFileOnDisk, len(seeds))
	for _, seed := range seeds {
		byID[seed.Seed.ID] = seed
	}

	result := &SeedApplyResult{
		SeedCount:   plan.SeedCount,
		InsertCount: plan.InsertCount,
		UpdateCount: plan.UpdateCount,
		Entries:     plan.Entries,
	}
	for _, entry := range plan.Entries {
		src := byID[entry.SeedID]
		for _, action := range entry.Actions {
			if action.Action != "insert" && action.Action != "update" {
				continue
			}
			row, ok := findSeedRow(src.Seed, action.Identity)
			if !ok {
				return nil, fmt.Errorf("pactmigrate: seed %s: could not resolve row for action", entry.SeedID)
			}
			if err := upsertSeedRow(ctx, db, dialect, src.Seed.Table, src.Seed.Identity.Columns, row); err != nil {
				return nil, fmt.Errorf("pactmigrate: apply seed %s: %w", entry.SeedID, err)
			}
			result.AppliedCount++
		}
	}
	return result, nil
}

type seedFileOnDisk struct {
	Filename string
	Seed     StaticSeedFile
}

func loadRequiredSeedFiles(fsys fs.FS, root string) ([]seedFileOnDisk, error) {
	root = strings.TrimPrefix(root, "./")
	if root == "" {
		root = "."
	}
	out := make([]seedFileOnDisk, 0)
	err := fs.WalkDir(fsys, root, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".seed.json") {
			return nil
		}
		seed, err := readStaticSeedFile(fsys, fpath)
		if err != nil {
			return err
		}
		if seed.Kind != SeedKindRequired {
			return nil
		}
		out = append(out, seedFileOnDisk{Filename: fpath, Seed: seed})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pactmigrate: walk seed root %q: %w", root, err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Filename < out[j].Filename })
	return out, nil
}

func readStaticSeedFile(fsys fs.FS, fpath string) (StaticSeedFile, error) {
	b, err := fs.ReadFile(fsys, fpath)
	if err != nil {
		return StaticSeedFile{}, fmt.Errorf("pactmigrate: read seed %q: %w", fpath, err)
	}
	var seed StaticSeedFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&seed); err != nil {
		return StaticSeedFile{}, fmt.Errorf("pactmigrate: parse seed %q: %w", fpath, err)
	}
	if seed.DeletePolicy == "" {
		seed.DeletePolicy = SeedDeletePolicyIgnore
	}
	return seed, nil
}

func planSeedEntry(ctx context.Context, db *sql.DB, dialect Dialect, src seedFileOnDisk) (SeedPlanEntry, error) {
	entry := SeedPlanEntry{
		SeedID:           src.Seed.ID,
		Filename:         src.Filename,
		Table:            src.Seed.Table,
		RowCount:         len(src.Seed.Rows),
		ValidationIssues: validateStaticSeed(src.Seed),
		Actions:          []SeedRowAction{},
	}
	schema, err := inspectSeedTableSchema(ctx, db, dialect, src.Seed.Table)
	if err != nil {
		return entry, err
	}
	entry.ValidationIssues = append(entry.ValidationIssues, validateSeedAgainstSchema(src.Seed, schema)...)
	if len(entry.ValidationIssues) > 0 {
		return entry, nil
	}

	for _, row := range src.Seed.Rows {
		existing, found, err := lookupExistingSeedRow(ctx, db, dialect, src.Seed.Table, src.Seed.Identity.Columns, row, sortedRowKeys(row))
		if err != nil {
			return entry, err
		}
		identity := buildIdentityMap(src.Seed.Identity.Columns, row)
		if !found {
			entry.InsertCount++
			entry.Actions = append(entry.Actions, SeedRowAction{Action: "insert", Identity: identity})
			continue
		}
		if !seedRowMatches(row, existing) {
			entry.UpdateCount++
			entry.Actions = append(entry.Actions, SeedRowAction{Action: "update", Identity: identity})
		}
	}
	return entry, nil
}

func inspectSeedTableSchema(ctx context.Context, db *sql.DB, dialect Dialect, table string) (*SeedTableSchema, error) {
	if err := validateIdentifier(table); err != nil {
		return nil, fmt.Errorf("pactmigrate: seed table %q: %w", table, err)
	}
	schema := &SeedTableSchema{
		Table:             table,
		Columns:           map[string]SeedColumnSchema{},
		UniqueConstraints: [][]string{},
	}
	switch dialect {
	case DialectPostgres:
		rows, err := db.QueryContext(ctx, `
SELECT column_name, is_nullable = 'YES', column_default IS NOT NULL
FROM information_schema.columns
WHERE table_catalog = current_database()
  AND table_schema = ANY (current_schemas(true))
  AND lower(table_name) = lower($1)
ORDER BY ordinal_position
`, table)
		if err != nil {
			return nil, fmt.Errorf("inspect table columns: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var nullable bool
			var hasDefault bool
			if err := rows.Scan(&name, &nullable, &hasDefault); err != nil {
				return nil, fmt.Errorf("scan table columns: %w", err)
			}
			schema.Columns[name] = SeedColumnSchema{Name: name, Nullable: nullable, HasDefault: hasDefault}
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate table columns: %w", err)
		}
		if len(schema.Columns) == 0 {
			return nil, fmt.Errorf("pactmigrate: target table %q not found", table)
		}
		crows, err := db.QueryContext(ctx, `
SELECT tc.constraint_name, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
 AND tc.table_name = kcu.table_name
WHERE tc.table_catalog = current_database()
  AND tc.table_schema = ANY (current_schemas(true))
  AND lower(tc.table_name) = lower($1)
  AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE')
ORDER BY tc.constraint_name, kcu.ordinal_position
`, table)
		if err != nil {
			return nil, fmt.Errorf("inspect table constraints: %w", err)
		}
		defer crows.Close()
		return scanConstraintRows(schema, crows)
	case DialectMySQL:
		rows, err := db.QueryContext(ctx, `
SELECT column_name, is_nullable = 'YES', column_default IS NOT NULL
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = ?
ORDER BY ordinal_position
`, table)
		if err != nil {
			return nil, fmt.Errorf("inspect table columns: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var nullable bool
			var hasDefault bool
			if err := rows.Scan(&name, &nullable, &hasDefault); err != nil {
				return nil, fmt.Errorf("scan table columns: %w", err)
			}
			schema.Columns[name] = SeedColumnSchema{Name: name, Nullable: nullable, HasDefault: hasDefault}
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate table columns: %w", err)
		}
		if len(schema.Columns) == 0 {
			return nil, fmt.Errorf("pactmigrate: target table %q not found", table)
		}
		crows, err := db.QueryContext(ctx, `
SELECT tc.constraint_name, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
 AND tc.table_name = kcu.table_name
WHERE tc.table_schema = DATABASE()
  AND tc.table_name = ?
  AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE')
ORDER BY tc.constraint_name, kcu.ordinal_position
`, table)
		if err != nil {
			return nil, fmt.Errorf("inspect table constraints: %w", err)
		}
		defer crows.Close()
		return scanConstraintRows(schema, crows)
	default:
		return nil, fmt.Errorf("unsupported dialect %q", dialect)
	}
}

func scanConstraintRows(schema *SeedTableSchema, rows *sql.Rows) (*SeedTableSchema, error) {
	byName := map[string][]string{}
	for rows.Next() {
		var constraintName, column string
		if err := rows.Scan(&constraintName, &column); err != nil {
			return nil, fmt.Errorf("scan table constraints: %w", err)
		}
		byName[constraintName] = append(byName[constraintName], column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table constraints: %w", err)
	}
	for _, cols := range byName {
		schema.UniqueConstraints = append(schema.UniqueConstraints, cols)
	}
	return schema, nil
}

func validateSeedAgainstSchema(seed StaticSeedFile, schema *SeedTableSchema) []SeedValidationIssue {
	issues := make([]SeedValidationIssue, 0)
	if !matchesAnyConstraint(seed.Identity.Columns, schema.UniqueConstraints) {
		issues = append(issues, SeedValidationIssue{
			Code:    "identity_not_unique_constraint",
			Path:    "identity.columns",
			Message: "identity.columns must match a primary key or unique constraint on the target table",
		})
	}
	requiredCols := map[string]SeedColumnSchema{}
	for name, col := range schema.Columns {
		if !col.Nullable && !col.HasDefault {
			requiredCols[name] = col
		}
	}
	for i, row := range seed.Rows {
		for col := range row {
			if _, ok := schema.Columns[col]; !ok {
				issues = append(issues, SeedValidationIssue{
					Code:    "unknown_column",
					Path:    fmt.Sprintf("rows[%d].%s", i, col),
					Message: fmt.Sprintf("column %q does not exist on table %q", col, seed.Table),
				})
			}
		}
		for name := range requiredCols {
			if _, ok := row[name]; !ok {
				issues = append(issues, SeedValidationIssue{
					Code:    "missing_required_column",
					Path:    fmt.Sprintf("rows[%d].%s", i, name),
					Message: fmt.Sprintf("rows[%d] is missing required column %q", i, name),
				})
			}
		}
	}
	return issues
}

func matchesAnyConstraint(identity []string, constraints [][]string) bool {
	want := append([]string(nil), identity...)
	sort.Strings(want)
	for _, constraint := range constraints {
		if len(constraint) != len(want) {
			continue
		}
		got := append([]string(nil), constraint...)
		sort.Strings(got)
		match := true
		for i := range got {
			if got[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func lookupExistingSeedRow(ctx context.Context, db *sql.DB, dialect Dialect, table string, identityCols []string, row map[string]any, selectCols []string) (map[string]any, bool, error) {
	quotedCols := make([]string, 0, len(selectCols))
	for _, col := range selectCols {
		quotedCols = append(quotedCols, quoteIdentifier(dialect, col))
	}
	whereParts := make([]string, 0, len(identityCols))
	args := make([]any, 0, len(identityCols))
	for i, col := range identityCols {
		whereParts = append(whereParts, fmt.Sprintf("%s = %s", quoteIdentifier(dialect, col), bindVar(dialect, i+1)))
		args = append(args, row[col])
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s", strings.Join(quotedCols, ", "), quoteIdentifier(dialect, table), strings.Join(whereParts, " AND "))
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("query existing seed row: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, false, nil
	}
	values := make([]any, len(selectCols))
	scanTargets := make([]any, len(selectCols))
	for i := range values {
		scanTargets[i] = &values[i]
	}
	if err := rows.Scan(scanTargets...); err != nil {
		return nil, false, fmt.Errorf("scan existing seed row: %w", err)
	}
	out := make(map[string]any, len(selectCols))
	for i, col := range selectCols {
		out[col] = normalizeDBValue(values[i])
	}
	return out, true, nil
}

func upsertSeedRow(ctx context.Context, db *sql.DB, dialect Dialect, table string, identityCols []string, row map[string]any) error {
	cols := sortedRowKeys(row)
	args := make([]any, 0, len(cols))
	valueBinds := make([]string, 0, len(cols))
	for i, col := range cols {
		args = append(args, row[col])
		valueBinds = append(valueBinds, bindVar(dialect, i+1))
	}
	quotedCols := make([]string, 0, len(cols))
	for _, col := range cols {
		quotedCols = append(quotedCols, quoteIdentifier(dialect, col))
	}
	nonIdentity := make([]string, 0)
	identitySet := map[string]struct{}{}
	for _, col := range identityCols {
		identitySet[col] = struct{}{}
	}
	for _, col := range cols {
		if _, ok := identitySet[col]; !ok {
			nonIdentity = append(nonIdentity, col)
		}
	}

	base := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", quoteIdentifier(dialect, table), strings.Join(quotedCols, ", "), strings.Join(valueBinds, ", "))
	switch dialect {
	case DialectPostgres:
		if len(nonIdentity) == 0 {
			_, err := db.ExecContext(ctx, base+" ON CONFLICT ("+joinQuoted(dialect, identityCols)+") DO NOTHING", args...)
			return err
		}
		assignments := make([]string, 0, len(nonIdentity))
		for _, col := range nonIdentity {
			assignments = append(assignments, fmt.Sprintf("%s = EXCLUDED.%s", quoteIdentifier(dialect, col), quoteIdentifier(dialect, col)))
		}
		_, err := db.ExecContext(ctx, base+" ON CONFLICT ("+joinQuoted(dialect, identityCols)+") DO UPDATE SET "+strings.Join(assignments, ", "), args...)
		return err
	case DialectMySQL:
		assignments := make([]string, 0)
		if len(nonIdentity) == 0 && len(identityCols) > 0 {
			col := identityCols[0]
			assignments = append(assignments, fmt.Sprintf("%s = VALUES(%s)", quoteIdentifier(dialect, col), quoteIdentifier(dialect, col)))
		} else {
			for _, col := range nonIdentity {
				assignments = append(assignments, fmt.Sprintf("%s = VALUES(%s)", quoteIdentifier(dialect, col), quoteIdentifier(dialect, col)))
			}
		}
		_, err := db.ExecContext(ctx, base+" ON DUPLICATE KEY UPDATE "+strings.Join(assignments, ", "), args...)
		return err
	default:
		return fmt.Errorf("unsupported dialect %q", dialect)
	}
}

func joinQuoted(dialect Dialect, cols []string) string {
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		out = append(out, quoteIdentifier(dialect, col))
	}
	return strings.Join(out, ", ")
}

func quoteIdentifier(dialect Dialect, s string) string {
	switch dialect {
	case DialectMySQL:
		return "`" + strings.ReplaceAll(s, "`", "``") + "`"
	default:
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
}

func bindVar(dialect Dialect, idx int) string {
	if dialect == DialectMySQL {
		return "?"
	}
	return fmt.Sprintf("$%d", idx)
}

func sortedRowKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for col := range row {
		keys = append(keys, col)
	}
	sort.Strings(keys)
	return keys
}

func buildIdentityMap(identityCols []string, row map[string]any) map[string]any {
	out := make(map[string]any, len(identityCols))
	for _, col := range identityCols {
		out[col] = row[col]
	}
	return out
}

func seedRowMatches(seedRow, dbRow map[string]any) bool {
	for col, want := range seedRow {
		got, ok := dbRow[col]
		if !ok {
			return false
		}
		if normalizeComparable(want) != normalizeComparable(got) {
			return false
		}
	}
	return true
}

func normalizeComparable(v any) string {
	switch t := normalizeDBValue(v).(type) {
	case nil:
		return ""
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func normalizeDBValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return t
	}
}

func findSeedRow(seed StaticSeedFile, identity map[string]any) (map[string]any, bool) {
	for _, row := range seed.Rows {
		match := true
		for _, col := range seed.Identity.Columns {
			if normalizeComparable(row[col]) != normalizeComparable(identity[col]) {
				match = false
				break
			}
		}
		if match {
			return row, true
		}
	}
	return nil, false
}
