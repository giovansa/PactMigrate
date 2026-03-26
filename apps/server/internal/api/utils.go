package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

func inferFailedKey(planned, remaining []string) string {
	if len(remaining) > 0 {
		return remaining[0]
	}
	return ""
}

func environmentNames(in []EnvironmentConfig) []string {
	names := make([]string, 0, len(in))
	for _, e := range in {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names
}

func humanSince(ts string) string {
	if ts == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

func parsePositiveInt(v string, max int) (int, error) {
	n := 0
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		n = n*10 + int(ch-'0')
		if n > max {
			return max, nil
		}
	}
	if n <= 0 {
		return 0, fmt.Errorf("must be > 0")
	}
	return n, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": msg,
	})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func hasAnyRole(a Actor, roles ...string) bool {
	roleSet := map[string]struct{}{}
	for _, r := range a.Roles {
		roleSet[strings.ToLower(strings.TrimSpace(r))] = struct{}{}
	}
	for _, want := range roles {
		if _, ok := roleSet[strings.ToLower(strings.TrimSpace(want))]; ok {
			return true
		}
	}
	return false
}
