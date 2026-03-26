package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	apipkg "pactmigrate.local/apps/server/internal/api"
)

func main() {
	configPath := flag.String("config", "apps/server/config.json", "path to JSON server config")
	flag.Parse()

	cfg, err := apipkg.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}

	api, err := apipkg.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      api.Routes(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
	}

	fmt.Printf("PactMigrate server listening on %s\n", cfg.Server.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "server: listen: %v\n", err)
		os.Exit(1)
	}
}
