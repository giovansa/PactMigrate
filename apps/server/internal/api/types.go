package api

import (
	"sync"
	"time"
)

type Config struct {
	Server struct {
		Addr                string `json:"addr"`
		ReadTimeoutSeconds  int    `json:"read_timeout_seconds"`
		WriteTimeoutSeconds int    `json:"write_timeout_seconds"`
	} `json:"server"`
	Auth       AuthConfig `json:"auth"`
	Seeds      SeedsConfig `json:"seeds"`
	Migrations struct {
		Source string `json:"source"`
		Dir    string `json:"dir"`
		FSDir  string `json:"fs_dir"`
	} `json:"migrations"`
	Environments []EnvironmentConfig `json:"environments"`
}

type SeedsConfig struct {
	Enabled bool   `json:"enabled"`
	Dir     string `json:"dir"`
}

type AuthConfig struct {
	Enabled bool     `json:"enabled"`
	APIKeys []APIKey `json:"api_keys"`
}

type APIKey struct {
	ID    string   `json:"id"`
	Key   string   `json:"key"`
	Roles []string `json:"roles"`
}

type EnvironmentConfig struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	DSN        string            `json:"dsn"`
	ScratchDSN string            `json:"scratch_dsn"`
	Dialect    string            `json:"dialect"`
	TableName  string            `json:"table_name"`
	Policies   EnvironmentPolicy `json:"policies"`
}

type EnvironmentPolicy struct {
	RequirePlanBeforeApply bool `json:"require_plan_before_apply"`

	// Safety toggles that map to core migrator options.
	AllowOutOfOrder       bool `json:"allow_out_of_order"`
	AllowChecksumMismatch bool `json:"allow_checksum_mismatch"`
	AllowLateMigrations   bool `json:"allow_late_migrations"`
}

type API struct {
	cfg *Config

	failureMu sync.RWMutex
	failures  map[string]map[string]string // env -> migration_key -> error string

	planMu         sync.Mutex
	latestPlan     map[string]planCacheEntry // env -> latest migration plan
	latestSeedPlan map[string]planCacheEntry // env -> latest seed plan
}

type Actor struct {
	ID    string
	Roles []string
}

type planCacheEntry struct {
	PlanID    string
	CreatedAt time.Time
}

type runRecord struct {
	ID             string    `json:"id"`
	RequestID      string    `json:"request_id"`
	Environment    string    `json:"environment"`
	Kind           string    `json:"kind,omitempty"` // migration|seed
	Target         string    `json:"target,omitempty"`
	Status         string    `json:"status"` // succeeded/failed
	Mode           string    `json:"mode"`   // plan|apply
	PlanID         string    `json:"plan_id,omitempty"`
	ActorID        string    `json:"actor_id,omitempty"`
	ActorRoles     string    `json:"actor_roles,omitempty"` // comma-separated
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	DurationMillis int64     `json:"duration_ms"`
	PlannedCount   int       `json:"planned_count"`
	AppliedCount   int       `json:"applied_count"`
	RemainingCount int       `json:"remaining_count"`
	Error          string    `json:"error,omitempty"`
}
