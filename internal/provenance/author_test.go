package provenance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrecedenceAndPaseoLiveModel(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PASEO_HOME", root)
	t.Setenv("PASEO_AGENT_ID", "test-agent")
	t.Setenv("ENGRAM_AUTHOR", "fallback/model")
	if err := os.MkdirAll(filepath.Join(root, "agents"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "agents", "test-agent.json")
	if err := os.WriteFile(file, []byte(`{"config":{"provider":"codex","model":"old"},"runtimeInfo":{"model":"new"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ explicit, session, want string }{{"adapter/live", "session/old", "adapter/live"}, {"", "session/live", "session/live"}, {"", "", "codex/new"}} {
		if got := Resolve(tc.explicit, tc.session); got != tc.want {
			t.Fatalf("Resolve = %q, want %q", got, tc.want)
		}
	}
	if err := os.WriteFile(file, []byte(`{"provider":"omp","runtimeInfo":{"model":"provider/next"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := Resolve("", ""); got != "omp/provider/next" {
		t.Fatal(got)
	}
	t.Setenv("PASEO_AGENT_ID", "../test-agent")
	if got := Resolve("", ""); got != "fallback/model" {
		t.Fatal(got)
	}
}

func TestHookAuthorSources(t *testing.T) {
	t.Setenv("ENGRAM_AUTHOR", "")
	t.Setenv("PASEO_AGENT_ID", "")
	t.Setenv("ANTHROPIC_MODEL", "")
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("model = \"live\" # comment\nmodel_provider = 'router'\n[profiles.other]\nmodel = \"wrong\""), 0600); err != nil {
		t.Fatal(err)
	}
	if got := HookAuthor("codex", nil); got != "codex/router/live" {
		t.Fatal(got)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"model":"opus"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := HookAuthor("claude-code", nil); got != "claude-code/opus" {
		t.Fatal(got)
	}
	transcript := filepath.Join(root, "tail.jsonl")
	if err := os.WriteFile(transcript, []byte("bad first line\n{\"type\":\"assistant\",\"message\":{\"model\":\"opus-exact\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	payload := map[string]json.RawMessage{"transcript_path": json.RawMessage(`"` + filepath.ToSlash(transcript) + `"`)}
	if got := HookAuthor("claude-code", payload); got != "claude-code/opus-exact" {
		t.Fatal(got)
	}
	payload["model"] = json.RawMessage(`"host-live"`)
	if got := HookAuthor("codex", payload); got != "codex/host-live" {
		t.Fatal(got)
	}
	t.Setenv("ENGRAM_AUTHOR", "custom/override")
	if got := HookAuthor("codex", payload); got != "custom/override" {
		t.Fatal(got)
	}
}

func TestCodexRolloutAndProviderDedup(t *testing.T) {
	t.Setenv("ENGRAM_AUTHOR", "")
	t.Setenv("PASEO_AGENT_ID", "")
	root := t.TempDir()
	t.Setenv("CODEX_HOME", root)
	dir := filepath.Join(root, "sessions", "2026")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-time-11111111-1111-4111-8111-111111111111.jsonl"), []byte("{\"type\":\"turn_context\",\"payload\":{\"model\":\"exact\"}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := HookAuthor("codex", map[string]json.RawMessage{"session_id": json.RawMessage(`"11111111-1111-4111-8111-111111111111"`)}); got != "codex/exact" {
		t.Fatal(got)
	}
	if got := Label("pi", "router", "router/model"); got != "pi/router/model" {
		t.Fatal(got)
	}
}
