package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	pactmigrate "pactmigrate.local/packages/core"
)

func main() {
	configPath := flag.String("config", "apps/pactmigrate-cli/config.json", "path to JSON config file")
	planOnly := flag.Bool("plan", false, "list pending migrations and exit (no DB writes, no advisory lock)")
	flag.Parse()

	if err := run(*configPath, *planOnly); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string, planOnly bool) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	if cfg.Database.Driver == "" {
		return errors.New("missing database.driver (e.g. pgx, postgres, mysql)")
	}
	if cfg.Database.DSN == "" {
		return errors.New("missing database.dsn")
	}

	db, err := sql.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("sql open: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	dialect, err := parseDialect(cfg.Migrations.Dialect)
	if err != nil {
		return err
	}

	opts := []pactmigrate.Option{
		pactmigrate.WithFSDir(cfg.Migrations.FSDir),
		pactmigrate.WithDialect(dialect),
		pactmigrate.WithTableName(cfg.Migrations.TableName),
		pactmigrate.WithVerifyContent(cfg.Migrations.VerifyContent),
	}
	oop, err := parseOutOfOrderPolicy(cfg.Migrations.OutOfOrderPolicy)
	if err != nil {
		return err
	}
	opts = append(opts, pactmigrate.WithOutOfOrderPolicy(oop))
	opts = append(opts, pactmigrate.WithAllowChecksumMismatch(cfg.Migrations.AllowChecksumMismatch))
	if cfg.Migrations.TimeoutSec > 0 {
		opts = append(opts, pactmigrate.WithTimeout(time.Duration(cfg.Migrations.TimeoutSec)*time.Second))
	}

	fsys, err := resolveMigrationFS(cfg)
	if err != nil {
		return err
	}

	m, err := pactmigrate.New(db, fsys, opts...)
	if err != nil {
		return fmt.Errorf("migrator: %w", err)
	}

	runCtx := context.Background()
	var reporter *googleChatReporter
	if cfg.Notifications.GoogleChatWebhookURL != "" {
		meta := chatHookMeta{
			Environment: cfg.Notifications.Environment,
			Service:     cfg.Notifications.Service,
			Database:    cfg.Notifications.Database,
		}
		reporter = newGoogleChatReporter(cfg.Notifications.GoogleChatWebhookURL, meta)
	}
	if planOnly || cfg.Migrations.DryRun {
		plan, err := m.Plan(runCtx)
		if err != nil {
			return fmt.Errorf("plan: %w", err)
		}
		if len(plan.Pending) == 0 {
			fmt.Println("No pending migrations.")
			return nil
		}
		fmt.Println("Pending migrations:")
		for _, p := range plan.Pending {
			fmt.Printf("  %s  (%s)\n", p.Key(), p.Filename)
		}
		return nil
	}

	plan, err := m.Plan(runCtx)
	if err != nil {
		return fmt.Errorf("pre-up plan: %w", err)
	}
	keys := make([]string, 0, len(plan.Pending))
	for _, p := range plan.Pending {
		keys = append(keys, p.Key())
	}
	if reporter != nil {
		if err := reporter.sendRunStarting(runCtx, keys); err != nil {
			fmt.Fprintf(os.Stderr, "warn: google chat starting report failed: %v\n", err)
		}
	}

	startedAt := time.Now()
	if err := m.Up(runCtx); err != nil {
		if reporter != nil {
			remaining := keys
			postPlan, perr := m.Plan(runCtx)
			if perr == nil {
				remaining = make([]string, 0, len(postPlan.Pending))
				for _, p := range postPlan.Pending {
					remaining = append(remaining, p.Key())
				}
			} else {
				fmt.Fprintf(os.Stderr, "warn: post-failure plan failed: %v\n", perr)
			}
			if rerr := reporter.sendRunFailed(runCtx, keys, remaining, time.Since(startedAt), err); rerr != nil {
				fmt.Fprintf(os.Stderr, "warn: google chat failed report failed: %v\n", rerr)
			}
		}
		return fmt.Errorf("up: %w", err)
	}
	if reporter != nil {
		if err := reporter.sendRunSucceeded(runCtx, keys, time.Since(startedAt)); err != nil {
			fmt.Fprintf(os.Stderr, "warn: google chat succeeded report failed: %v\n", err)
		}
	}
	return nil
}

type config struct {
	Database struct {
		Driver string `json:"driver"`
		DSN    string `json:"dsn"`
	} `json:"database"`
	Notifications struct {
		GoogleChatWebhookURL string `json:"google_chat_webhook_url"`
		Environment          string `json:"environment"`
		Service              string `json:"service"`
		Database             string `json:"database"`
	} `json:"notifications"`
	Migrations struct {
		// Source is "dir" (read SQL from disk at runtime) or "embed" (compiled into the binary).
		Source string `json:"source"`
		// Dir is the host directory used when source is "dir" (absolute or relative to the process working directory).
		Dir           string `json:"dir"`
		FSDir         string `json:"fs_dir"`
		Dialect       string `json:"dialect"`
		TableName     string `json:"table_name"`
		TimeoutSec    int    `json:"timeout_seconds"`
		VerifyContent bool   `json:"verify_content"`
		// OutOfOrderPolicy: "allow_late" (default) or "strict".
		OutOfOrderPolicy      string `json:"out_of_order_policy"`
		AllowChecksumMismatch bool   `json:"allow_checksum_mismatch"`
		DryRun                bool   `json:"dry_run"`
	} `json:"migrations"`
}

func loadConfig(path string) (*config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
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
	case "embed":
		if cfg.Migrations.FSDir == "" {
			cfg.Migrations.FSDir = "migrations"
		}
	default:
		return nil, fmt.Errorf("unknown migrations.source %q (use dir or embed)", cfg.Migrations.Source)
	}
	if cfg.Migrations.TableName == "" {
		cfg.Migrations.TableName = "schema_migrations"
	}
	if cfg.Migrations.Dialect == "" {
		cfg.Migrations.Dialect = "postgres"
	}
	return &cfg, nil
}

func resolveMigrationFS(cfg *config) (fs.FS, error) {
	switch cfg.Migrations.Source {
	case "embed":
		return migrationsFS, nil
	case "dir":
		abs, err := filepath.Abs(cfg.Migrations.Dir)
		if err != nil {
			return nil, fmt.Errorf("migrations.dir: %w", err)
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("migrations.dir %q: %w", abs, err)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("migrations.dir is not a directory: %s", abs)
		}
		return os.DirFS(abs), nil
	default:
		return nil, fmt.Errorf("unknown migrations.source %q", cfg.Migrations.Source)
	}
}

func parseDialect(s string) (pactmigrate.Dialect, error) {
	switch s {
	case "postgres", "postgresql":
		return pactmigrate.DialectPostgres, nil
	case "mysql", "mariadb":
		return pactmigrate.DialectMySQL, nil
	default:
		return 0, fmt.Errorf("unknown migrations.dialect %q (use postgres or mysql)", s)
	}
}

func parseOutOfOrderPolicy(s string) (pactmigrate.OutOfOrderPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "allow_late", "allowlate":
		return pactmigrate.OutOfOrderAllowLate, nil
	case "strict":
		return pactmigrate.OutOfOrderStrict, nil
	default:
		return 0, fmt.Errorf("unknown migrations.out_of_order_policy %q (use allow_late or strict)", s)
	}
}
