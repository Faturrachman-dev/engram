package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Gentleman-Programming/engram/v3/internal/memorypolicy"
	"github.com/Gentleman-Programming/engram/v3/internal/store"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
)

// relationRowsFor reports how many relation rows reference the given sync_ids.
// Minimal saves must never create pending relation rows (PRD R5/T2).
func relationRowsFor(t *testing.T, s *store.Store, syncIDs ...string) int {
	t.Helper()
	rm, err := s.GetRelationsForObservationsContext(context.Background(), syncIDs)
	if err != nil {
		t.Fatalf("load relations: %v", err)
	}
	total := 0
	for _, r := range rm {
		total += len(r.AsSource) + len(r.AsTarget)
	}
	return total
}

// TestMinimalSaveSkipsCandidateScan verifies PRD R5: a minimal-mode save of a
// second, lexically similar observation does not scan for candidates, does not
// create pending rows, reports judgment_required=false and
// conflict_review="manual", and carries no candidate arrays.
func TestMinimalSaveSkipsCandidateScan(t *testing.T) {
	s := newMCPTestStore(t)
	h := handleSave(s, MCPConfig{Mode: memorypolicy.ModeMinimal}, NewSessionActivity(10*time.Minute))

	save := func(title, content string) map[string]any {
		res, err := h(context.Background(), mcppkg.CallToolRequest{Params: mcppkg.CallToolParams{Arguments: map[string]any{
			"title":   title,
			"content": content,
			"type":    "architecture",
		}}})
		if err != nil || res.IsError {
			t.Fatalf("save %q: err=%v text=%q", title, err, callResultText(t, res))
		}
		return callResultJSON(t, res)
	}

	env1 := save("We use sessions for auth middleware", "Session-based auth keeps things simple")
	env2 := save("Switched from sessions to JWT for auth", "JWT replaces session auth for scalability")

	if jr, _ := env2["judgment_required"].(bool); jr {
		t.Fatalf("minimal save must report judgment_required=false, got true")
	}
	if cr, _ := env2["conflict_review"].(string); cr != "manual" {
		t.Fatalf("conflict_review = %q, want \"manual\"", cr)
	}
	for _, field := range []string{"candidates", "judgment_id", "judgment_status"} {
		if _, ok := env2[field]; ok {
			t.Errorf("minimal save envelope must not contain %q", field)
		}
	}
	sync1, _ := env1["sync_id"].(string)
	sync2, _ := env2["sync_id"].(string)
	if sync2 == "" {
		t.Errorf("minimal save must still return sync_id")
	}
	if n := relationRowsFor(t, s, sync1, sync2); n != 0 {
		t.Fatalf("minimal save created %d relation rows, want 0", n)
	}
}

// TestLegacySaveStillScans verifies legacy mode preserves auto-scan behavior.
func TestLegacySaveStillScans(t *testing.T) {
	s := newMCPTestStore(t)
	h := handleSave(s, MCPConfig{Mode: memorypolicy.ModeLegacy}, NewSessionActivity(10*time.Minute))
	save := func(title, content string) map[string]any {
		res, err := h(context.Background(), mcppkg.CallToolRequest{Params: mcppkg.CallToolParams{Arguments: map[string]any{
			"title": title, "content": content, "type": "architecture",
		}}})
		if err != nil || res.IsError {
			t.Fatalf("save %q: err=%v text=%q", title, err, callResultText(t, res))
		}
		return callResultJSON(t, res)
	}
	save("We use sessions for auth middleware", "Session-based auth keeps things simple")
	env := save("Switched from sessions to JWT for auth", "JWT replaces session auth for scalability")
	if cr, _ := env["conflict_review"].(string); cr != "auto" {
		t.Fatalf("legacy conflict_review = %q, want \"auto\"", cr)
	}
	if jr, _ := env["judgment_required"].(bool); !jr {
		t.Fatalf("legacy save of similar obs should surface judgment_required=true")
	}
}

// TestCompactSearchOmitsRelationsByDefault verifies PRD R7: compact default
// (minimal mode) omits relation arrays from results.
func TestCompactSearchOmitsRelationsByDefault(t *testing.T) {
	s := newMCPTestStore(t)
	if err := s.CreateSession("s-compact", "engram", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	aID, _ := s.AddObservation(store.AddObservationParams{SessionID: "s-compact", Type: "decision", Title: "Alpha decision", Content: "alpha budget content", Project: "engram", Scope: "project"})
	bID, _ := s.AddObservation(store.AddObservationParams{SessionID: "s-compact", Type: "decision", Title: "Beta decision", Content: "beta budget content", Project: "engram", Scope: "project"})
	a, _ := s.GetObservation(aID)
	b, _ := s.GetObservation(bID)
	if _, err := s.SaveRelation(store.SaveRelationParams{SyncID: "rel-compact-omit", SourceID: a.SyncID, TargetID: b.SyncID}); err != nil {
		t.Fatalf("save relation: %v", err)
	}

	search := handleSearch(s, MCPConfig{Mode: memorypolicy.ModeMinimal}, NewSessionActivity(10*time.Minute))
	res, err := search(context.Background(), mcppkg.CallToolRequest{Params: mcppkg.CallToolParams{Arguments: map[string]any{
		"query": "decision", "project": "engram", "scope": "project",
	}}})
	if err != nil || res.IsError {
		t.Fatalf("search: err=%v text=%q", err, callResultText(t, res))
	}
	body := callResultJSON(t, res)
	results, _ := body["results"].([]any)
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	for _, raw := range results {
		if _, ok := raw.(map[string]any)["relations"]; ok {
			t.Errorf("compact default must omit relations, got %#v", raw)
		}
	}
}

// TestCompactSearchIncludeRelationsOverride verifies include_relations=true adds
// relation arrays even in compact format.
func TestCompactSearchIncludeRelationsOverride(t *testing.T) {
	s := newMCPTestStore(t)
	if err := s.CreateSession("s-incl", "engram", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	aID, _ := s.AddObservation(store.AddObservationParams{SessionID: "s-incl", Type: "decision", Title: "Gamma decision", Content: "gamma content", Project: "engram", Scope: "project"})
	bID, _ := s.AddObservation(store.AddObservationParams{SessionID: "s-incl", Type: "decision", Title: "Delta decision", Content: "delta content", Project: "engram", Scope: "project"})
	a, _ := s.GetObservation(aID)
	b, _ := s.GetObservation(bID)
	if _, err := s.SaveRelation(store.SaveRelationParams{SyncID: "rel-incl", SourceID: a.SyncID, TargetID: b.SyncID}); err != nil {
		t.Fatalf("save relation: %v", err)
	}
	search := handleSearch(s, MCPConfig{Mode: memorypolicy.ModeMinimal}, NewSessionActivity(10*time.Minute))
	res, err := search(context.Background(), mcppkg.CallToolRequest{Params: mcppkg.CallToolParams{Arguments: map[string]any{
		"query": "decision", "project": "engram", "scope": "project",
		"include_relations": true,
	}}})
	if err != nil || res.IsError {
		t.Fatalf("search: err=%v text=%q", err, callResultText(t, res))
	}
	body := callResultJSON(t, res)
	results, _ := body["results"].([]any)
	foundRelations := false
	for _, raw := range results {
		if _, ok := raw.(map[string]any)["relations"]; ok {
			foundRelations = true
		}
	}
	if !foundRelations {
		t.Fatalf("include_relations=true must attach relations in compact format")
	}
}

// TestCompactSearchByteBudget verifies PRD R7: oversized compact results stay
// within max_bytes, report omitted counts, and remain valid JSON.
func TestCompactSearchByteBudget(t *testing.T) {
	s := newMCPTestStore(t)
	if err := s.CreateSession("s-budget", "engram", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	// Distinct shared term "budgetword" guarantees all rows match the query,
	// while unique per-row filler makes the serialized set exceed the budget.
	for i := range 20 {
		_, err := s.AddObservation(store.AddObservationParams{
			SessionID: "s-budget",
			Type:      "decision",
			Title:     fmt.Sprintf("Budgetword decision number %d %s", i, strings.Repeat("x", 40)),
			Content:   fmt.Sprintf("budgetword row %d %s", i, strings.Repeat("detail ", 60)),
			Project:   "engram",
			Scope:     "project",
		})
		if err != nil {
			t.Fatalf("add obs %d: %v", i, err)
		}
	}
	search := handleSearch(s, MCPConfig{Mode: memorypolicy.ModeMinimal}, NewSessionActivity(10*time.Minute))
	res, err := search(context.Background(), mcppkg.CallToolRequest{Params: mcppkg.CallToolParams{Arguments: map[string]any{
		"query": "budget", "project": "engram", "scope": "project",
		"limit": float64(20), "max_bytes": float64(1024),
	}}})
	if err != nil || res.IsError {
		t.Fatalf("search: err=%v text=%q", err, callResultText(t, res))
	}
	text := callResultText(t, res)
	if len(text) > 1024 {
		t.Fatalf("compact payload is %d bytes, exceeds max_bytes=1024", len(text))
	}
	body := callResultJSON(t, res)
	if _, ok := body["omitted_results"]; !ok {
		t.Fatalf("expected omitted_results when results exceed budget")
	}
}

// TestSearchInvalidMaxBytesFails verifies out-of-range max_bytes is rejected.
func TestSearchInvalidMaxBytesFails(t *testing.T) {
	s := newMCPTestStore(t)
	search := handleSearch(s, MCPConfig{Mode: memorypolicy.ModeMinimal}, NewSessionActivity(10*time.Minute))
	for _, bad := range []float64{512, 70000} {
		res, err := search(context.Background(), mcppkg.CallToolRequest{Params: mcppkg.CallToolParams{Arguments: map[string]any{
			"query": "anything", "max_bytes": bad,
		}}})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if !res.IsError {
			t.Fatalf("max_bytes=%v should be rejected", bad)
		}
	}
}
