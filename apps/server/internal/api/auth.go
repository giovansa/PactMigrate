package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
)

type actorContextKey struct{}

func withAuth(cfg *Config, next http.Handler) http.Handler {
	// If auth is disabled, pass through.
	if cfg == nil || !cfg.Auth.Enabled {
		return next
	}
	// Build key lookup once.
	keys := make([]APIKey, 0, len(cfg.Auth.APIKeys))
	keys = append(keys, cfg.Auth.APIKeys...)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow unauthenticated health checks.
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}
		// Allow preflight.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		key := readAPIKey(r)
		if key == "" {
			writeError(w, http.StatusUnauthorized, "missing api key")
			return
		}
		actor, ok := lookupActor(keys, key)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}
		ctx := context.WithValue(r.Context(), actorContextKey{}, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func readAPIKey(r *http.Request) string {
	// Prefer Authorization: Bearer <key>
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	// Fallback header for simple clients.
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

func lookupActor(keys []APIKey, presented string) (Actor, bool) {
	for _, k := range keys {
		// Constant-time compare to avoid trivial timing leaks.
		if subtle.ConstantTimeCompare([]byte(k.Key), []byte(presented)) == 1 {
			return Actor{ID: k.ID, Roles: k.Roles}, true
		}
	}
	return Actor{}, false
}

func actorFromContext(ctx context.Context) (Actor, bool) {
	v := ctx.Value(actorContextKey{})
	a, ok := v.(Actor)
	return a, ok
}
