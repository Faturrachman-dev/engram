package plugin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCodexPreToolMatcherCoversAllWritesOnly(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "plugin", "codex", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	groups := manifest.Hooks["PreToolUse"]
	if len(groups) != 1 {
		t.Fatalf("PreToolUse groups = %d, want 1", len(groups))
	}
	pattern := groups[0].Matcher
	if !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
		t.Fatalf("matcher not anchored: %q", pattern)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatal(err)
	}
	writes := []string{"mem_save", "mem_update", "mem_review", "mem_delete", "mem_save_prompt", "mem_pin", "mem_unpin", "mem_session_summary", "mem_session_start", "mem_session_end", "mem_capture_passive", "mem_merge_projects", "mem_judge", "mem_compare"}
	for _, prefix := range []string{"mcp__engram__", "mcp__plugin_engram_engram__"} {
		for _, tool := range writes {
			if !re.MatchString(prefix + tool) {
				t.Errorf("write not bound: %s%s", prefix, tool)
			}
		}
		for _, tool := range []string{"mem_context", "mem_search", "mem_save_extra"} {
			if re.MatchString(prefix + tool) {
				t.Errorf("non-write matched: %s%s", prefix, tool)
			}
		}
	}
	if re.MatchString("mcp__other__mem_save") {
		t.Error("unrelated server matched")
	}
}
