package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	pactmigrate "pactmigrate.local/packages/core"
)

type runRequest struct {
	Mode   string `json:"mode"`    // plan|apply
	PlanID string `json:"plan_id"` // required when policies.require_plan_before_apply=true
}

func requestID() string {
	// 16 bytes -> 32 hex chars
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (s *API) handleEnvironments(w http.ResponseWriter, r *http.Request) {
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
	out := make([]map[string]string, 0, len(s.cfg.Environments))
	for _, e := range s.cfg.Environments {
		out = append(out, map[string]string{
			"name":      e.Name,
			"dialect":   e.Dialect,
			"tableName": e.TableName,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": out})
}

func (s *API) handleRuns(w http.ResponseWriter, r *http.Request) {
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
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := parsePositiveInt(v, 200); err == nil {
			limit = n
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	hist, err := s.listRecentRuns(ctx, limit)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("list runs: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": hist})
}

func (s *API) handleDashboard(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	migrations, err := s.loadMigrations()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	envApplied := make(map[string]map[string]pactmigrate.AppliedRecord, len(s.cfg.Environments))
	for _, env := range s.cfg.Environments {
		applied, err := s.fetchApplied(ctx, env)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Sprintf("environment %s: %v", env.Name, err))
			return
		}
		envApplied[env.Name] = applied
	}

	rows := make([]map[string]any, 0, len(migrations))
	failedCount := 0
	for _, m := range migrations {
		statusByEnv := map[string]string{}
		for _, env := range s.cfg.Environments {
			status := "pending"
			if _, ok := envApplied[env.Name][m.Key()]; ok {
				status = "success"
				s.clearFailure(env.Name, m.Key())
			} else if s.hasFailure(env.Name, m.Key()) {
				status = "failed"
			}
			statusByEnv[env.Name] = status
			if status == "failed" {
				failedCount++
			}
		}
		rows = append(rows, map[string]any{
			"id":        m.Key(),
			"name":      strings.ReplaceAll(m.Title, "_", " "),
			"version":   "v1.0",
			"statuses":  statusByEnv,
			"filename":  m.Filename,
			"timestamp": m.Timestamp,
		})
	}

	lastEvent, err := s.latestRunFinishedAt(ctx)
	if err != nil {
		// Keep dashboard available even if audit table is temporarily unavailable.
		lastEvent = ""
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"total_migrations":      len(migrations),
			"active_environments":   len(s.cfg.Environments),
			"environment_names":     environmentNames(s.cfg.Environments),
			"migrations_failed":     failedCount,
			"last_event_at":         lastEvent,
			"last_event_human_hint": humanSince(lastEvent),
		},
		"rows": rows,
	})
}

func (s *API) handleEnvironmentActions(w http.ResponseWriter, r *http.Request) {
	// expected: /api/v1/environments/{name}/run
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/environments/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[1] != "run" {
		writeError(w, http.StatusNotFound, "unknown route")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.cfg.Auth.Enabled {
		a, ok := actorFromContext(r.Context())
		if !ok || !hasAnyRole(a, "operator", "admin") {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
	}
	envName := parts[0]
	env, ok := s.findEnvironment(envName)
	if !ok {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}
	s.runMigrationsForEnvironment(w, r, env)
}

func (s *API) runMigrationsForEnvironment(w http.ResponseWriter, r *http.Request, env EnvironmentConfig) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	req := runRequest{Mode: "apply"}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	if req.Mode == "" {
		req.Mode = "apply"
	}
	if req.Mode != "apply" && req.Mode != "plan" {
		writeError(w, http.StatusBadRequest, `mode must be "plan" or "apply"`)
		return
	}

	fsys, fsDir, err := s.resolveFS()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	db, err := sql.Open(env.Driver, env.DSN)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("sql open: %v", err))
		return
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("ping: %v", err))
		return
	}

	dialect, err := parseDialect(env.Dialect)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	m, err := pactmigrate.New(
		db,
		fsys,
		pactmigrate.WithFSDir(fsDir),
		pactmigrate.WithDialect(dialect),
		pactmigrate.WithTableName(env.TableName),
		pactmigrate.WithAllowChecksumMismatch(env.Policies.AllowChecksumMismatch),
		pactmigrate.WithOutOfOrderPolicy(outOfOrderPolicyForEnv(env)),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("migrator: %v", err))
		return
	}

	prePlan, err := m.Plan(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("plan: %v", err))
		return
	}
	planned := make([]string, 0, len(prePlan.Pending))
	for _, p := range prePlan.Pending {
		planned = append(planned, p.Key())
	}

	if req.Mode == "plan" {
		planID := s.rememberPlan(env.Name)
		writeJSON(w, http.StatusOK, map[string]any{
			"mode":              "plan",
			"plan_id":           planID,
			"request_id":        requestID(),
			"environment":       env.Name,
			"planned_count":     len(planned),
			"remaining_pending": planned,
		})
		return
	}

	if env.Policies.RequirePlanBeforeApply {
		if !s.isPlanAccepted(env.Name, req.PlanID) {
			writeJSON(w, http.StatusPreconditionFailed, map[string]any{
				"error":                "plan required before apply",
				"required":             true,
				"mode":                 "apply",
				"expected_plan_action": "POST /api/v1/environments/{name}/run with {\"mode\":\"plan\"}",
			})
			return
		}
	}

	reqID := requestID()
	runID := fmt.Sprintf("%s-%d", env.Name, time.Now().UnixNano())
	started := time.Now()
	upErr := m.Up(ctx)
	finished := time.Now()

	postPlan, planErr := m.Plan(ctx)
	remaining := []string{}
	if planErr == nil {
		for _, p := range postPlan.Pending {
			remaining = append(remaining, p.Key())
		}
	} else {
		remaining = planned
	}

	applied := len(planned) - len(remaining)
	if applied < 0 {
		applied = 0
	}

	record := runRecord{
		ID:             runID,
		RequestID:      reqID,
		Environment:    env.Name,
		StartedAt:      started.UTC(),
		FinishedAt:     finished.UTC(),
		DurationMillis: finished.Sub(started).Milliseconds(),
		PlannedCount:   len(planned),
		AppliedCount:   applied,
		RemainingCount: len(remaining),
		Status:         "succeeded",
		Mode:           "apply",
		PlanID:         req.PlanID,
	}
	if s.cfg.Auth.Enabled {
		if a, ok := actorFromContext(r.Context()); ok {
			record.ActorID = a.ID
			record.ActorRoles = strings.Join(a.Roles, ",")
		}
	}
	if upErr != nil {
		record.Status = "failed"
		record.Error = upErr.Error()
	}

	auditErr := persistRunRecord(ctx, env, record)

	if upErr != nil {
		failedKey := inferFailedKey(planned, remaining)
		if failedKey != "" {
			s.markFailure(env.Name, failedKey, upErr.Error())
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"run_id":            runID,
			"request_id":        reqID,
			"status":            "failed",
			"error":             upErr.Error(),
			"mode":              "apply",
			"planned_count":     len(planned),
			"applied_count":     applied,
			"remaining_pending": remaining,
			"audit_persisted":   auditErr == nil,
			"audit_error":       errorText(auditErr),
		})
		return
	}
	for _, key := range planned {
		s.clearFailure(env.Name, key)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"run_id":            runID,
		"request_id":        reqID,
		"status":            "succeeded",
		"mode":              "apply",
		"planned_count":     len(planned),
		"applied_count":     applied,
		"remaining_pending": remaining,
		"audit_persisted":   auditErr == nil,
		"audit_error":       errorText(auditErr),
	})
}
