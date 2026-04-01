package pactmigrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

type SeedValidationIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// StaticSeedInventoryEntry is a read-only summary of a file-backed seed asset.
type StaticSeedInventoryEntry struct {
	ID                    string                `json:"id"`
	Kind                  string                `json:"kind"`
	Table                 string                `json:"table"`
	Description           string                `json:"description,omitempty"`
	Filename              string                `json:"filename"`
	RowCount              int                   `json:"row_count"`
	IdentityColumns       []string              `json:"identity_columns"`
	DeletePolicy          string                `json:"delete_policy"`
	SupportedEnvironments []string              `json:"supported_environments"`
	ValidationIssues      []SeedValidationIssue `json:"validation_issues"`
}

func (e StaticSeedInventoryEntry) Valid() bool {
	return len(e.ValidationIssues) == 0
}

// LoadStaticSeedInventory walks the seed root and returns a summary for each
// static seed file. Invalid files are reported with validation issues instead
// of failing the full inventory load.
func LoadStaticSeedInventory(fsys fs.FS, root string) ([]StaticSeedInventoryEntry, error) {
	root = strings.TrimPrefix(root, "./")
	if root == "" {
		root = "."
	}

	var out []StaticSeedInventoryEntry
	err := fs.WalkDir(fsys, root, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".seed.json") {
			return nil
		}

		entry, rerr := loadStaticSeedEntry(fsys, fpath)
		if rerr != nil {
			return rerr
		}
		out = append(out, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pactmigrate: walk seed root %q: %w", root, err)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Filename < out[j].Filename
	})

	seenIDs := map[string]int{}
	for i := range out {
		if strings.TrimSpace(out[i].ID) == "" {
			continue
		}
		if prev, ok := seenIDs[out[i].ID]; ok {
			appendIssue(&out[prev], "duplicate_seed_id", "id",
				fmt.Sprintf("seed id %q is also used by %s", out[prev].ID, out[i].Filename))
			appendIssue(&out[i], "duplicate_seed_id", "id",
				fmt.Sprintf("seed id %q is also used by %s", out[i].ID, out[prev].Filename))
			continue
		}
		seenIDs[out[i].ID] = i
	}
	return out, nil
}

func LoadStaticSeedInventoryByID(fsys fs.FS, root, seedID string) (*StaticSeedInventoryEntry, error) {
	seeds, err := LoadStaticSeedInventory(fsys, root)
	if err != nil {
		return nil, err
	}
	for i := range seeds {
		if seeds[i].ID == seedID {
			return &seeds[i], nil
		}
	}
	return nil, fs.ErrNotExist
}

func loadStaticSeedEntry(fsys fs.FS, fpath string) (StaticSeedInventoryEntry, error) {
	b, err := fs.ReadFile(fsys, fpath)
	if err != nil {
		return StaticSeedInventoryEntry{}, fmt.Errorf("pactmigrate: read seed %q: %w", fpath, err)
	}

	entry := StaticSeedInventoryEntry{
		Filename:              fpath,
		DeletePolicy:          string(SeedDeletePolicyIgnore),
		IdentityColumns:       []string{},
		SupportedEnvironments: []string{},
		ValidationIssues:      []SeedValidationIssue{},
	}

	var seed StaticSeedFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&seed); err != nil {
		appendIssue(&entry, "invalid_json_schema", "", fmt.Sprintf("invalid JSON schema: %v", err))
		return entry, nil
	}

	entry.ID = seed.ID
	entry.Kind = string(seed.Kind)
	entry.Table = seed.Table
	entry.Description = seed.Description
	entry.RowCount = len(seed.Rows)
	entry.IdentityColumns = append([]string(nil), seed.Identity.Columns...)
	if seed.DeletePolicy != "" {
		entry.DeletePolicy = string(seed.DeletePolicy)
	}
	entry.ValidationIssues = validateStaticSeed(seed)
	return entry, nil
}

func validateStaticSeed(seed StaticSeedFile) []SeedValidationIssue {
	issues := make([]SeedValidationIssue, 0)

	if seed.Version != 1 {
		issues = append(issues, SeedValidationIssue{
			Code: "invalid_version", Path: "version", Message: "version must be 1",
		})
	}
	switch seed.Kind {
	case SeedKindRequired, SeedKindCapture:
	default:
		issues = append(issues, SeedValidationIssue{
			Code: "invalid_kind", Path: "kind", Message: `kind must be "required" or "capture" for static seed files`,
		})
	}
	if strings.TrimSpace(seed.ID) == "" {
		issues = append(issues, SeedValidationIssue{
			Code: "missing_id", Path: "id", Message: "id is required",
		})
	}
	if strings.TrimSpace(seed.Table) == "" {
		issues = append(issues, SeedValidationIssue{
			Code: "missing_table", Path: "table", Message: "table is required",
		})
	}
	if len(seed.Identity.Columns) == 0 {
		issues = append(issues, SeedValidationIssue{
			Code: "missing_identity_columns", Path: "identity.columns", Message: "identity.columns must be non-empty",
		})
	}
	if seed.DeletePolicy == "" {
		seed.DeletePolicy = SeedDeletePolicyIgnore
	}
	if seed.DeletePolicy != SeedDeletePolicyIgnore {
		issues = append(issues, SeedValidationIssue{
			Code: "invalid_delete_policy", Path: "delete_policy", Message: `delete_policy must be "ignore"`,
		})
	}
	if seed.Rows == nil {
		issues = append(issues, SeedValidationIssue{
			Code: "missing_rows", Path: "rows", Message: "rows is required",
		})
		return issues
	}

	seenCols := map[string]struct{}{}
	for i, col := range seed.Identity.Columns {
		name := strings.TrimSpace(col)
		if name == "" {
			issues = append(issues, SeedValidationIssue{
				Code: "empty_identity_column", Path: fmt.Sprintf("identity.columns[%d]", i), Message: "identity.columns cannot contain empty names",
			})
			continue
		}
		if _, ok := seenCols[name]; ok {
			issues = append(issues, SeedValidationIssue{
				Code: "duplicate_identity_column", Path: "identity.columns", Message: fmt.Sprintf("identity.columns contains duplicate column %q", name),
			})
			continue
		}
		seenCols[name] = struct{}{}
	}

	seenRows := map[string]int{}
	for i, row := range seed.Rows {
		parts := make([]string, 0, len(seed.Identity.Columns))
		missing := false
		for _, col := range seed.Identity.Columns {
			v, ok := row[col]
			if !ok {
				issues = append(issues, SeedValidationIssue{
					Code: "missing_identity_value", Path: fmt.Sprintf("rows[%d].%s", i, col), Message: fmt.Sprintf("rows[%d] is missing identity column %q", i, col),
				})
				missing = true
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%v", col, v))
		}
		if missing {
			continue
		}
		key := strings.Join(parts, "|")
		if prev, ok := seenRows[key]; ok {
			issues = append(issues, SeedValidationIssue{
				Code: "duplicate_identity_tuple", Path: fmt.Sprintf("rows[%d]", i), Message: fmt.Sprintf("rows[%d] duplicates identity tuple from rows[%d]", i, prev),
			})
			continue
		}
		seenRows[key] = i
	}

	return issues
}

func appendIssue(entry *StaticSeedInventoryEntry, code, path, message string) {
	entry.ValidationIssues = append(entry.ValidationIssues, SeedValidationIssue{
		Code:    code,
		Path:    path,
		Message: message,
	})
}
