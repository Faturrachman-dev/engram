package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHookAuthorOverridesUntrustedArguments(t *testing.T) {
	t.Setenv("ENGRAM_AUTHOR", "")
	t.Setenv("PASEO_AGENT_ID", "")
	for _, tc := range []struct {
		name, want string
		transform  func([]byte) []byte
	}{
		{"Claude", "claude-code/host-model", transformClaudePreToolUse},
		{"Codex", "codex/host-model", transformCodexPreToolUse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, tool := range []string{"mem_save", "mem_session_summary", "mem_capture_passive", "mem_session_start"} {
				input := []byte(`{"session_id":"host","model":"host-model","tool_name":"mcp__engram__` + tool + `","tool_input":{"author":"forged"}}`)
				var result struct {
					Hook struct {
						Input map[string]any `json:"updatedInput"`
					} `json:"hookSpecificOutput"`
				}
				if err := json.Unmarshal(tc.transform(input), &result); err != nil {
					t.Fatal(err)
				}
				if result.Hook.Input["author"] != tc.want {
					t.Fatalf("%s author=%v", tool, result.Hook.Input["author"])
				}
			}
		})
	}
}

func TestCodexLifecycleRefreshesAuthor(t *testing.T) {
	t.Setenv("ENGRAM_AUTHOR", "")
	t.Setenv("PASEO_AGENT_ID", "")
	var author string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/project/current" {
			_, _ = w.Write([]byte(`{"project":"test","project_source":"git_root"}`))
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		author, _ = body["author"].(string)
		status := "resolved"
		if r.URL.Path == "/sessions" {
			w.WriteHeader(201)
			status = "created"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": body["id"], "status": status})
	}))
	defer srv.Close()
	for _, action := range []string{"codex-register", "codex-resolve"} {
		if id := runCodexLifecycle(action, []byte(`{"session_id":"host","cwd":"/work","model":"live-model"}`), srv.URL); id != "host" {
			t.Fatalf("%s id=%q", action, id)
		}
		if author != "codex/live-model" {
			t.Fatalf("%s author=%q", action, author)
		}
	}
}
