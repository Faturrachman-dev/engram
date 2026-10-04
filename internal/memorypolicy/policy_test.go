package memorypolicy

import (
	"strings"
	"testing"
)

// TestPolicyBodyWithinWordBudget enforces the PRD R4 ceiling: the full compact
// policy body must be at most 300 words.
func TestPolicyBodyWithinWordBudget(t *testing.T) {
	const maxWords = 300
	if n := WordCount(); n > maxWords {
		t.Fatalf("policy body is %d words (>%d) — trim policy.md", n, maxWords)
	}
}

// TestPolicyBodyRetainsSafetyRules guards the non-negotiable R3 content so a
// future trim cannot silently drop a safety or routing rule.
func TestPolicyBodyRetainsSafetyRules(t *testing.T) {
	body := strings.Join(strings.Fields(Body()), " ") // ignore line wrapping
	for _, want := range []string{
		"Never store credentials",
		"semantic owner",
		"codex-config",
		"authoritative registered",
		"never invent",
		"If memory work fails or needs follow-up, still send the answer.",
		"genuine session close",
		"COMPACTION CHECKPOINT",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("policy body missing required safety/routing rule: %q", want)
		}
	}
}

// TestPolicyBodyHasNoCompulsoryRituals asserts the minimal-mode intent: the
// body must not reintroduce per-turn save/search mandates.
func TestPolicyBodyHasNoCompulsoryRituals(t *testing.T) {
	lower := strings.ToLower(Body())
	for _, banned := range []string{
		"immediately after any",
		"before saying \"done\"",
		"mandatory",
		"first action required",
		"call mem_save now",
	} {
		if strings.Contains(lower, banned) {
			t.Errorf("policy body contains banned compulsory directive: %q", banned)
		}
	}
}

// TestSkillsShareCanonicalBody verifies every client skill embeds the exact
// shared body (R4: behavioral rules must not drift; only frontmatter differs).
func TestSkillsShareCanonicalBody(t *testing.T) {
	body := Body()
	for _, client := range []string{"codex", "claude", "claude-code", "contributor"} {
		skill := Skill(client)
		if !strings.Contains(skill, body) {
			t.Errorf("skill %q does not embed the canonical policy body verbatim", client)
		}
		if !strings.HasPrefix(skill, "---\n") {
			t.Errorf("skill %q missing YAML frontmatter", client)
		}
	}
}

func TestCodexInstructionsAreBody(t *testing.T) {
	if CodexInstructions() != Body() {
		t.Fatal("Codex instructions must equal the canonical body")
	}
}

func TestResolveModePrecedence(t *testing.T) {
	cases := []struct {
		name, cli, env string
		want           Mode
		wantErr        bool
	}{
		{"default", "", "", ModeMinimal, false},
		{"env legacy", "", "legacy", ModeLegacy, false},
		{"env minimal", "", "minimal", ModeMinimal, false},
		{"cli overrides env", "minimal", "legacy", ModeMinimal, false},
		{"cli legacy over env minimal", "legacy", "minimal", ModeLegacy, false},
		{"invalid cli", "bogus", "", "", true},
		{"invalid env", "", "bogus", "", true},
		{"cli valid ignores invalid env", "legacy", "", ModeLegacy, false},
		{"uppercase cli", "LEGACY", "", ModeLegacy, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveMode(tc.cli, tc.env)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for cli=%q env=%q", tc.cli, tc.env)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ResolveMode(%q,%q) = %q, want %q", tc.cli, tc.env, got, tc.want)
			}
		})
	}
}
