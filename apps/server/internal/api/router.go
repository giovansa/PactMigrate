package api

import "net/http"

func New(cfg *Config) (*API, error) {
	return &API{
		cfg:        cfg,
		failures:   make(map[string]map[string]string),
		latestPlan: make(map[string]planCacheEntry),
	}, nil
}

func (s *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/environments", s.handleEnvironments)
	mux.HandleFunc("/api/v1/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/v1/runs", s.handleRuns)
	mux.HandleFunc("/api/v1/drift/schema", s.handleSchemaDrift)         // ?env={name}
	mux.HandleFunc("/api/v1/environments/", s.handleEnvironmentActions) // /api/v1/environments/{env}/run
	return withJSON(withAuth(s.cfg, mux))
}

func withJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
