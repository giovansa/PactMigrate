package api

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Server.ReadTimeoutSeconds <= 0 {
		cfg.Server.ReadTimeoutSeconds = 15
	}
	if cfg.Server.WriteTimeoutSeconds <= 0 {
		cfg.Server.WriteTimeoutSeconds = 30
	}
	if cfg.Migrations.Source == "" {
		cfg.Migrations.Source = "dir"
	}
	switch cfg.Migrations.Source {
	case "dir":
		if cfg.Migrations.Dir == "" {
			cfg.Migrations.Dir = "apps/pactmigrate-cli/migrations"
		}
		if cfg.Migrations.FSDir == "" {
			cfg.Migrations.FSDir = "."
		}
	default:
		return nil, fmt.Errorf("unsupported migrations.source %q in server; use dir", cfg.Migrations.Source)
	}
	if len(cfg.Environments) == 0 {
		return nil, fmt.Errorf("config must include at least one environment")
	}
	if cfg.Auth.Enabled {
		if len(cfg.Auth.APIKeys) == 0 {
			return nil, fmt.Errorf("auth.enabled is true but auth.api_keys is empty")
		}
		for i := range cfg.Auth.APIKeys {
			k := &cfg.Auth.APIKeys[i]
			if k.ID == "" {
				return nil, fmt.Errorf("auth.api_keys[%d] has empty id", i)
			}
			if k.Key == "" {
				return nil, fmt.Errorf("auth.api_keys[%d] (%s) has empty key", i, k.ID)
			}
			if len(k.Roles) == 0 {
				return nil, fmt.Errorf("auth.api_keys[%d] (%s) has empty roles", i, k.ID)
			}
		}
	}
	for i := range cfg.Environments {
		e := &cfg.Environments[i]
		if e.Name == "" {
			return nil, fmt.Errorf("environment at index %d has empty name", i)
		}
		if e.Driver == "" || e.DSN == "" {
			return nil, fmt.Errorf("environment %q must define driver and dsn", e.Name)
		}
		if e.Dialect == "" {
			e.Dialect = "postgres"
		}
		if e.TableName == "" {
			e.TableName = "schema_migrations"
		}
	}
	return &cfg, nil
}
