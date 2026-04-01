package pactmigrate

// SeedKind identifies the high-level seeder workflow type.
type SeedKind string

const (
	SeedKindRequired SeedKind = "required"
	SeedKindPersona  SeedKind = "persona"
	SeedKindCapture  SeedKind = "capture"
)

// SeedDeletePolicy defines how extra live rows should be handled.
// Phase 0 only standardizes "ignore"; destructive sync modes belong to later phases.
type SeedDeletePolicy string

const (
	SeedDeletePolicyIgnore SeedDeletePolicy = "ignore"
)

// SeedIdentity defines the columns that uniquely identify a logical row.
type SeedIdentity struct {
	Columns []string `json:"columns"`
}

// StaticSeedFile is the canonical JSON shape for file-backed seeds.
// Execution behavior is intentionally not implemented in Phase 0.
type StaticSeedFile struct {
	Version      int              `json:"version"`
	Kind         SeedKind         `json:"kind"`
	ID           string           `json:"id"`
	Table        string           `json:"table"`
	Description  string           `json:"description,omitempty"`
	DependsOn    []string         `json:"depends_on,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	Identity     SeedIdentity     `json:"identity"`
	DeletePolicy SeedDeletePolicy `json:"delete_policy,omitempty"`
	Rows         []map[string]any `json:"rows"`
}
