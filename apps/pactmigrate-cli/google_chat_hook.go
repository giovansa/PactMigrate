package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type googleChatReporter struct {
	webhookURL string
	client     *http.Client
	meta       chatHookMeta
}

type chatHookMeta struct {
	Environment string
	Service     string
	Database    string
}

func newGoogleChatReporter(webhookURL string, meta chatHookMeta) *googleChatReporter {
	return &googleChatReporter{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		meta: meta,
	}
}

type runEvent struct {
	Status         string
	Emoji          string
	Title          string
	Duration       time.Duration
	PlannedCount   int
	AppliedCount   int
	RemainingCount int
	PlannedItems   []string
	RemainingItems []string
	FailedKey      string
	Err            error
}

func (r *googleChatReporter) sendRunStarting(ctx context.Context, planned []string) error {
	return r.sendCard(ctx, runEvent{
		Status:       "starting",
		Emoji:        "🟡",
		Title:        "PactMigrate Run Starting",
		PlannedCount: len(planned),
		PlannedItems: planned,
	})
}

func (r *googleChatReporter) sendRunSucceeded(ctx context.Context, planned []string, duration time.Duration) error {
	return r.sendCard(ctx, runEvent{
		Status:         "succeeded",
		Emoji:          "🟢",
		Title:          "PactMigrate Run Succeeded",
		Duration:       duration,
		PlannedCount:   len(planned),
		AppliedCount:   len(planned),
		RemainingCount: 0,
		PlannedItems:   planned,
	})
}

func (r *googleChatReporter) sendRunFailed(ctx context.Context, planned, remaining []string, duration time.Duration, runErr error) error {
	applied := len(planned) - len(remaining)
	if applied < 0 {
		applied = 0
	}
	failedKey := ""
	if len(remaining) > 0 {
		failedKey = remaining[0] // first remaining item is the failed migration in sorted execution order.
	}
	return r.sendCard(ctx, runEvent{
		Status:         "failed",
		Emoji:          "🔴",
		Title:          "PactMigrate Run Failed",
		Duration:       duration,
		PlannedCount:   len(planned),
		AppliedCount:   applied,
		RemainingCount: len(remaining),
		PlannedItems:   planned,
		RemainingItems: remaining,
		FailedKey:      failedKey,
		Err:            runErr,
	})
}

func (r *googleChatReporter) sendCard(ctx context.Context, e runEvent) error {
	cardPayload := r.cardPayload(e)
	if err := r.sendJSON(ctx, cardPayload); err != nil {
		// Graceful fallback for schema incompatibilities/validation issues.
		fallback := r.fallbackText(e)
		if err2 := r.sendJSON(ctx, map[string]string{"text": fallback}); err2 != nil {
			return fmt.Errorf("send Google Chat card failed (%v); fallback failed (%v)", err, err2)
		}
	}
	return nil
}

func (r *googleChatReporter) sendJSON(ctx context.Context, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal Google Chat payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.webhookURL, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create Google Chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send Google Chat webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("google chat webhook returned status %s", resp.Status)
	}
	return nil
}

func (r *googleChatReporter) fallbackText(e runEvent) string {
	var sb strings.Builder
	sb.WriteString(e.Emoji)
	sb.WriteString(" ")
	sb.WriteString(e.Title)
	sb.WriteString("\nPlanned: ")
	sb.WriteString(fmt.Sprintf("%d", e.PlannedCount))
	sb.WriteString("\nApplied in run: ")
	sb.WriteString(fmt.Sprintf("%d", e.AppliedCount))
	sb.WriteString("\nRemaining pending: ")
	sb.WriteString(fmt.Sprintf("%d", e.RemainingCount))
	if e.FailedKey != "" {
		sb.WriteString("\nFailed key: ")
		sb.WriteString(e.FailedKey)
	}
	if e.Duration > 0 {
		sb.WriteString("\nDuration: ")
		sb.WriteString(e.Duration.Round(time.Millisecond).String())
	}
	if e.Err != nil {
		sb.WriteString("\nError: ")
		sb.WriteString(truncateForChat(e.Err.Error(), 1200))
	}
	if len(e.RemainingItems) > 0 {
		sb.WriteString("\nRemaining:\n")
		for _, k := range firstN(e.RemainingItems, 5) {
			sb.WriteString("- ")
			sb.WriteString(k)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (r *googleChatReporter) cardPayload(e runEvent) map[string]any {
	statusLine := fmt.Sprintf("%s %s", e.Emoji, e.Status)
	if e.Duration > 0 {
		statusLine = fmt.Sprintf("%s (%s)", statusLine, e.Duration.Round(time.Millisecond))
	}

	rows := []map[string]any{
		decorated("Status", statusLine),
		decorated("Planned count", fmt.Sprintf("%d", e.PlannedCount)),
	}
	if e.Status != "starting" {
		rows = append(rows, decorated("Applied in this run", fmt.Sprintf("%d", e.AppliedCount)))
		rows = append(rows, decorated("Remaining pending", fmt.Sprintf("%d", e.RemainingCount)))
	}
	if r.meta.Environment != "" {
		rows = append(rows, decorated("Environment", r.meta.Environment))
	}
	if r.meta.Service != "" {
		rows = append(rows, decorated("Service", r.meta.Service))
	}
	if r.meta.Database != "" {
		rows = append(rows, decorated("Database", r.meta.Database))
	}
	if e.Duration > 0 {
		rows = append(rows, decorated("Duration", e.Duration.Round(time.Millisecond).String()))
	}
	if e.FailedKey != "" {
		rows = append(rows, decorated("Failed migration key", e.FailedKey))
	}

	sections := []map[string]any{
		{
			"widgets": rows,
		},
	}
	if len(e.RemainingItems) > 0 {
		lines := firstN(e.RemainingItems, 10)
		var b strings.Builder
		for _, k := range lines {
			b.WriteString("• ")
			b.WriteString(escapeHTML(k))
			b.WriteString("<br>")
		}
		if len(e.RemainingItems) > len(lines) {
			b.WriteString(fmt.Sprintf("...and %d more", len(e.RemainingItems)-len(lines)))
		}
		sections = append(sections, map[string]any{
			"header": "Remaining pending migrations",
			"widgets": []map[string]any{
				{
					"textParagraph": map[string]any{
						"text": b.String(),
					},
				},
			},
		})
	} else if len(e.PlannedItems) > 0 {
		lines := firstN(e.PlannedItems, 10)
		var b strings.Builder
		for _, k := range lines {
			b.WriteString("• ")
			b.WriteString(escapeHTML(k))
			b.WriteString("<br>")
		}
		if len(e.PlannedItems) > len(lines) {
			b.WriteString(fmt.Sprintf("...and %d more", len(e.PlannedItems)-len(lines)))
		}
		sections = append(sections, map[string]any{
			"header": "Planned migrations",
			"widgets": []map[string]any{
				{
					"textParagraph": map[string]any{
						"text": b.String(),
					},
				},
			},
		})
	}
	if e.Err != nil {
		sections = append(sections, map[string]any{
			"header": "Error",
			"widgets": []map[string]any{
				{
					"textParagraph": map[string]any{
						"text": escapeHTML(truncateForChat(e.Err.Error(), 3500)),
					},
				},
			},
		})
	}

	return map[string]any{
		"cardsV2": []map[string]any{
			{
				"cardId": "pactmigrate-" + e.Status,
				"card": map[string]any{
					"header": map[string]any{
						"title":    e.Title,
						"subtitle": fmt.Sprintf("%s status report", strings.ToUpper(e.Status)),
					},
					"sections": sections,
				},
			},
		},
	}
}

func firstN(items []string, n int) []string {
	if n <= 0 || len(items) == 0 {
		return nil
	}
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func decorated(label, text string) map[string]any {
	return map[string]any{
		"decoratedText": map[string]any{
			"topLabel": label,
			"text":     escapeHTML(truncateForChat(text, 1200)),
			"wrapText": true,
		},
	}
}

func truncateForChat(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
