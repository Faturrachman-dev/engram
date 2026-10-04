package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Gentleman-Programming/engram/v3/internal/memorypolicy"
	projectpkg "github.com/Gentleman-Programming/engram/v3/internal/project"
	"github.com/Gentleman-Programming/engram/v3/internal/store"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
)

// seedManyRelations creates one searchable observation linked to n relation
// rows (synthetic; never the user's live memory). Targets do not match the
// search query, so only the hub entry is returned.
func seedManyRelations(t *testing.T, s *store.Store, n int) (int64, string) {
	t.Helper()
	if err := s.CreateSession("s-many-rel", "engram", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	hubID, err := s.AddObservation(store.AddObservationParams{SessionID: "s-many-rel", Type: "decision", Title: "Zephyrhub routing decision", Content: "zephyrhub content", Project: "engram", Scope: "project"})
	if err != nil {
		t.Fatalf("add hub: %v", err)
	}
	hub, _ := s.GetObservation(hubID)
	for i := range n {
		tid, err := s.AddObservation(store.AddObservationParams{SessionID: "s-many-rel", Type: "decision", Title: fmt.Sprintf("Unrelated target %d", i), Content: fmt.Sprintf("target body %d", i), Project: "engram", Scope: "project"})
		if err != nil {
			t.Fatalf("add target %d: %v", i, err)
		}
		target, _ := s.GetObservation(tid)
		if _, err := s.SaveRelation(store.SaveRelationParams{SyncID: fmt.Sprintf("rel-many-%04d", i), SourceID: hub.SyncID, TargetID: target.SyncID}); err != nil {
			t.Fatalf("save relation %d: %v", i, err)
		}
	}
	return hubID, hub.SyncID
}

func callSearch(t *testing.T, s *store.Store, mode memorypolicy.Mode, activity *SessionActivity, args map[string]any) (string, map[string]any) {
	t.Helper()
	res, err := handleSearch(s, MCPConfig{Mode: mode}, activity)(context.Background(), mcppkg.CallToolRequest{Params: mcppkg.CallToolParams{Arguments: args}})
	if err != nil || res.IsError {
		t.Fatalf("search: err=%v text=%q", err, callResultText(t, res))
	}
	text := callResultText(t, res)
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("search payload is not valid JSON: %v\n%s", err, text)
	}
	return text, body
}

// TestCompactSearchManyRelationsStaysWithinBudget covers PRD T3: an entry with
// hundreds of relations, include_relations=true and a small budget still
// returns valid JSON within max_bytes, with truthful omitted-relation counts.
func TestCompactSearchManyRelationsStaysWithinBudget(t *testing.T) {
	s := newMCPTestStore(t)
	const relations = 300
	hubID, hubSync := seedManyRelations(t, s, relations)

	for _, budget := range []int{1024, 2048, 8192} {
		t.Run(fmt.Sprintf("max_bytes=%d", budget), func(t *testing.T) {
			text, body := callSearch(t, s, memorypolicy.ModeMinimal, NewSessionActivity(10*time.Minute), map[string]any{
				"query": "zephyrhub", "project": "engram", "scope": "project",
				"include_relations": true, "max_bytes": float64(budget),
			})
			if len(text) > budget {
				t.Fatalf("payload %d bytes exceeds max_bytes=%d", len(text), budget)
			}
			results, _ := body["results"].([]any)
			if len(results) != 1 {
				t.Fatalf("results = %d, want the hub entry", len(results))
			}
			entry := results[0].(map[string]any)
			if int64(entry["id"].(float64)) != hubID || entry["sync_id"] != hubSync {
				t.Fatalf("hub identity corrupted: %v %v", entry["id"], entry["sync_id"])
			}
			omitted, _ := body["omitted_relations"].(float64)
			kept := 0
			if rels, ok := entry["relations"].(map[string]any); ok {
				src, _ := rels["as_source"].([]any)
				dst, _ := rels["as_target"].([]any)
				kept = len(src) + len(dst)
			}
			if kept+int(omitted) != relations {
				t.Fatalf("kept %d + omitted %v relations != %d", kept, omitted, relations)
			}
			if omitted == 0 {
				t.Fatal("expected omitted_relations for 300 relations under a small budget")
			}
			if _, ok := body["more"]; !ok {
				t.Error("trimmed response must say how to retrieve more")
			}
			t.Logf("max_bytes=%d: payload=%d bytes, relations kept=%d omitted=%v", budget, len(text), kept, omitted)
		})
	}

	// Baseline for the PRD measurement: the unbudgeted rendering is what the
	// pre-fix compact path returned with include_relations=true.
	results, err := s.SearchPreviewsContext(context.Background(), "zephyrhub", store.SearchOptions{Project: "engram", Scope: "project", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	rm, err := s.GetRelationsForObservationsContext(context.Background(), []string{results[0].SyncID})
	if err != nil {
		t.Fatal(err)
	}
	fullText, fullExtra := renderSearchResults(results, rm, true, fullSearchBudget(len(results)), "")
	t.Logf("unbudgeted compact+relations payload=%d bytes", len(projectEnvelopeJSON(projectpkg.DetectionResult{Project: "engram"}, fullText, fullExtra)))
}

// TestCompactSearchOmittedRelationsNeverQueriesStore covers PRD T3: when
// relation histories are not rendered the relation store query is not run.
func TestCompactSearchOmittedRelationsNeverQueriesStore(t *testing.T) {
	s := newMCPTestStore(t)
	seedManyRelations(t, s, 5)

	calls := 0
	orig := searchRelationsLoader
	searchRelationsLoader = func(ctx context.Context, st *store.Store, ids []string) (map[string]store.ObservationRelations, error) {
		calls++
		return orig(ctx, st, ids)
	}
	t.Cleanup(func() { searchRelationsLoader = orig })

	with := func(extra map[string]any) map[string]any {
		out := map[string]any{"query": "zephyrhub", "project": "engram", "scope": "project"}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	cases := []struct {
		name      string
		mode      memorypolicy.Mode
		args      map[string]any
		wantCalls int
	}{
		{"minimal default compact", memorypolicy.ModeMinimal, with(nil), 0},
		{"explicit compact in legacy mode", memorypolicy.ModeLegacy, with(map[string]any{"response_format": "compact"}), 0},
		{"legacy with include_relations=false", memorypolicy.ModeLegacy, with(map[string]any{"include_relations": false}), 0},
		{"compact with include_relations=true", memorypolicy.ModeMinimal, with(map[string]any{"include_relations": true}), 1},
		{"legacy default", memorypolicy.ModeLegacy, with(nil), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls = 0
			callSearch(t, s, tc.mode, NewSessionActivity(10*time.Minute), tc.args)
			if calls != tc.wantCalls {
				t.Fatalf("relation store queries = %d, want %d", calls, tc.wantCalls)
			}
		})
	}
}

// TestCompactSearchUTF8LongTitlesSmallBudget covers PRD T3: multi-byte
// previews, long titles and max_bytes=1024 stay valid, bounded and keep IDs.
func TestCompactSearchUTF8LongTitlesSmallBudget(t *testing.T) {
	s := newMCPTestStore(t)
	if err := s.CreateSession("s-utf8", "engram", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	ids := map[int64]bool{}
	for i := range 12 {
		id, err := s.AddObservation(store.AddObservationParams{
			SessionID: "s-utf8",
			Type:      "discovery",
			Title:     fmt.Sprintf("Ünïcödé quillword %d %s", i, strings.Repeat("漢字🙂é", 60)),
			Content:   fmt.Sprintf("quillword %d %s", i, strings.Repeat("ß漢🙂 ", 120)),
			Project:   "engram",
			Scope:     "project",
		})
		if err != nil {
			t.Fatalf("add obs %d: %v", i, err)
		}
		ids[id] = true
	}

	for _, format := range []string{"compact", "legacy"} {
		t.Run(format, func(t *testing.T) {
			text, body := callSearch(t, s, memorypolicy.ModeMinimal, NewSessionActivity(10*time.Minute), map[string]any{
				"query": "quillword", "project": "engram", "scope": "project",
				"limit": float64(12), "max_bytes": float64(1024), "response_format": format,
			})
			if len(text) > 1024 {
				t.Fatalf("payload %d bytes exceeds max_bytes=1024", len(text))
			}
			if !utf8.ValidString(text) {
				t.Fatal("payload is not valid UTF-8")
			}
			results, _ := body["results"].([]any)
			omitted, _ := body["omitted_results"].(float64)
			if len(results)+int(omitted) != 12 {
				t.Fatalf("shown %d + omitted %v != 12", len(results), omitted)
			}
			for _, raw := range results {
				entry := raw.(map[string]any)
				id := int64(entry["id"].(float64))
				if !ids[id] {
					t.Fatalf("result id %d is not one of the seeded ids", id)
				}
				if sid, _ := entry["sync_id"].(string); sid == "" {
					t.Fatalf("entry %d lost its sync_id", id)
				}
				if p, ok := entry["preview"].(string); ok && !utf8.ValidString(p) {
					t.Fatalf("entry %d preview split UTF-8", id)
				}
			}
		})
	}
}

// TestSearchSingleOversizedEntryReducesToIdentity covers the last-resort path:
// an entry that cannot fit even with an empty preview is reduced to identity
// fields, and the payload still fits the budget.
func TestSearchSingleOversizedEntryReducesToIdentity(t *testing.T) {
	results := []store.SearchPreviewResult{{
		ID: 42, SyncID: "obs-42", Type: "decision", Scope: "project",
		Title:   strings.Repeat("🙂", 2000),
		Preview: strings.Repeat("é", 300),
	}}
	render := func(b searchBudget) (string, map[string]any) {
		return renderSearchResults(results, nil, true, b, "")
	}
	res := projectpkg.DetectionResult{Project: "engram"}
	text, extra := fitSearchBudget(res, len(results), 1024, false, render)
	payload := projectEnvelopeJSON(res, text, extra)
	if len(payload) > 1024 {
		t.Fatalf("payload %d bytes exceeds 1024", len(payload))
	}
	kept := extra["results"].([]map[string]any)
	if len(kept) != 1 || kept[0]["id"] != int64(42) || kept[0]["sync_id"] != "obs-42" || kept[0]["reduced"] != true {
		t.Fatalf("want one identity-reduced entry, got %#v", kept)
	}
	if _, ok := extra["more"]; !ok {
		t.Fatal("reduced response must say how to retrieve more")
	}
}

// TestLegacySearchHonorsExplicitMaxBytes covers PRD R7: legacy output is
// unchanged by default but obeys an explicitly requested budget.
func TestLegacySearchHonorsExplicitMaxBytes(t *testing.T) {
	s := newMCPTestStore(t)
	seedManyRelations(t, s, 200)
	// Pending relations render as legacy annotation lines.
	unbounded, unboundedBody := callSearch(t, s, memorypolicy.ModeLegacy, NewSessionActivity(10*time.Minute), map[string]any{
		"query": "zephyrhub", "project": "engram", "scope": "project",
	})
	if len(unbounded) <= 2048 {
		t.Fatalf("fixture too small to prove budgeting: %d bytes", len(unbounded))
	}
	if _, ok := unboundedBody["max_bytes"]; ok {
		t.Fatal("legacy without an explicit budget must keep its prior output")
	}
	bounded, body := callSearch(t, s, memorypolicy.ModeLegacy, NewSessionActivity(10*time.Minute), map[string]any{
		"query": "zephyrhub", "project": "engram", "scope": "project", "max_bytes": float64(2048),
	})
	if len(bounded) > 2048 {
		t.Fatalf("legacy payload %d bytes exceeds explicit max_bytes=2048", len(bounded))
	}
	if n, _ := body["omitted_relations"].(float64); n == 0 {
		t.Fatal("legacy budget must report omitted_relations")
	}
}

// TestSearchBudgetKeepsActivityNudge guards that budget messages never replace
// the activity nudge text.
func TestSearchBudgetKeepsActivityNudge(t *testing.T) {
	s := newMCPTestStore(t)
	if err := s.CreateSession("s-nudge", "engram", "/tmp"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	for i := range 15 {
		if _, err := s.AddObservation(store.AddObservationParams{SessionID: "s-nudge", Type: "decision", Title: fmt.Sprintf("Nudgeword %d %s", i, strings.Repeat("y", 60)), Content: "nudgeword " + strings.Repeat("filler ", 50), Project: "engram", Scope: "project"}); err != nil {
			t.Fatalf("add obs: %v", err)
		}
	}
	activity := NewSessionActivity(0)
	args := map[string]any{"query": "nudgeword", "project": "engram", "scope": "project", "limit": float64(15), "max_bytes": float64(1024)}
	var body map[string]any
	for range 7 {
		_, body = callSearch(t, s, memorypolicy.ModeMinimal, activity, args)
	}
	result, _ := body["result"].(string)
	if !strings.Contains(result, "within the 1024-byte budget") {
		t.Fatalf("expected budget message, got %q", result)
	}
	if !strings.Contains(result, "No mem_save calls") {
		t.Fatalf("budget message replaced the activity nudge: %q", result)
	}
}

// TestCompactSearchEntriesCarryCreatedAt guards the recency field agents need.
func TestCompactSearchEntriesCarryCreatedAt(t *testing.T) {
	s := newMCPTestStore(t)
	seedManyRelations(t, s, 0)
	_, body := callSearch(t, s, memorypolicy.ModeMinimal, NewSessionActivity(10*time.Minute), map[string]any{"query": "zephyrhub", "project": "engram", "scope": "project"})
	entry := body["results"].([]any)[0].(map[string]any)
	if ca, _ := entry["created_at"].(string); ca == "" {
		t.Fatalf("compact entry missing created_at: %#v", entry)
	}
}

// TestBuildServerInstructions_CustomAndConditional restores allowlist
// truthfulness coverage for the compact instruction format (PRD T4): only
// registered tools are named.
func TestBuildServerInstructions_CustomAndConditional(t *testing.T) {
	for _, mode := range []memorypolicy.Mode{memorypolicy.ModeMinimal, memorypolicy.ModeLegacy} {
		t.Run(string(mode)+"/only mem_save and mem_update", func(t *testing.T) {
			got := buildServerInstructions(map[string]bool{"mem_save": true, "mem_update": true}, mode)
			if !strings.Contains(got, "Core tools: mem_save\n") {
				t.Errorf("expected mem_save as the only core tool:\n%s", got)
			}
			if !strings.Contains(got, "Deferred tools (ToolSearch when needed): mem_update\n") {
				t.Errorf("expected mem_update as the only deferred tool:\n%s", got)
			}
			for _, absent := range []string{"mem_search", "mem_review", "mem_judge", "mem_context"} {
				if strings.Contains(got, absent) {
					t.Errorf("%s is not registered but is advertised:\n%s", absent, got)
				}
			}
			if !strings.Contains(got, "must never block the user's answer") {
				t.Errorf("writer allowlist must keep the delivery guarantee:\n%s", got)
			}
		})
		t.Run(string(mode)+"/admin profile", func(t *testing.T) {
			got := buildServerInstructions(ProfileAdmin, mode)
			if strings.Contains(got, "Core tools:") {
				t.Errorf("admin profile has no core tools:\n%s", got)
			}
			for _, absent := range []string{"mem_save", "mem_search", "mem_judge", "mem_context", "Mode "} {
				if strings.Contains(got, absent) {
					t.Errorf("admin profile instructions mention %q:\n%s", absent, got)
				}
			}
			for tool := range ProfileAdmin {
				if !strings.Contains(got, tool) {
					t.Errorf("registered admin tool %s not listed:\n%s", tool, got)
				}
			}
		})
	}
	t.Run("only mem_judge", func(t *testing.T) {
		got := buildServerInstructions(map[string]bool{"mem_judge": true}, memorypolicy.ModeMinimal)
		if !strings.Contains(got, "Core tools: mem_judge") {
			t.Errorf("expected mem_judge as core tool:\n%s", got)
		}
		if strings.Contains(got, "mem_save") || strings.Contains(got, "Deferred tools") {
			t.Errorf("unregistered tools advertised:\n%s", got)
		}
	})
}

// TestSaveCapturePromptDescriptionIsOptIn guards the R8 schema text.
func TestSaveCapturePromptDescriptionIsOptIn(t *testing.T) {
	srv := NewServerWithConfig(newMCPTestStore(t), MCPConfig{}, nil)
	tool := srv.GetTool("mem_save")
	if tool == nil {
		t.Fatal("mem_save not registered")
	}
	raw, _ := json.Marshal(tool.Tool.InputSchema.Properties["capture_prompt"])
	desc := string(raw)
	if !strings.Contains(desc, "default: false") || strings.Contains(desc, "default: true") {
		t.Fatalf("capture_prompt description must state default false, got %s", desc)
	}
}

// TestToolDescriptionsWithinWordBudget enforces PRD R4: every tool description
// (excluding parameter schemas) is at most 80 words and carries no per-action
// save ritual.
func TestToolDescriptionsWithinWordBudget(t *testing.T) {
	srv := NewServerWithConfig(newMCPTestStore(t), MCPConfig{}, nil)
	for name, tool := range srv.ListTools() {
		desc := tool.Tool.Description
		if n := len(strings.Fields(desc)); n > 80 {
			t.Errorf("%s description is %d words (>80)", name, n)
		}
		lower := strings.ToLower(desc)
		for _, banned := range []string{"proactively", "after each", "don't wait to be asked", "when significant work is complete"} {
			if strings.Contains(lower, banned) {
				t.Errorf("%s description contains compulsory directive %q", name, banned)
			}
		}
	}
}
