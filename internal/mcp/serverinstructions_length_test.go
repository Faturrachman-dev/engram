package mcp

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Gentleman-Programming/engram/v3/internal/memorypolicy"
)

// TestServerInstructionsStaysUnderClientTruncationLimit enforces the
// boundary documented for the Claude Code MCP client: it truncates the
// `instructions` field returned during handshake at 2048 runes (UTF-8
// chars after decode). Keeping the constant under that ceiling avoids
// a class of handshake-validity regressions reported against Claude
// Code sessions where `mem_*` tools stopped responding shortly after
// connect.
//
// Source-side `len(serverInstructions)` is BYTES (Go strings are UTF-8
// byte slices). The figure the client uses is the RUNE count —
// em-dashes, ⏤ arrows, etc. each decode as one rune but several
// bytes. Asserting on runes here matches the client's truncation rule.
//
// The reported `Server instructions truncated from 2539 to 2048 chars`
// log line is the empirical evidence for the 2048-rune ceiling; the
// client-side handling after truncation (rejection, reconnect, or
// SIGINT) is documented in the field reports but not reproduced in
// this repo.
func TestServerInstructionsStaysUnderClientTruncationLimit(t *testing.T) {
	const clientTruncationCeiling = 2048

	profiles := map[string]map[string]bool{
		"all (nil)": nil,
		"agent":     ProfileAgent,
		"admin":     ProfileAdmin,
		"empty":     {},
		"custom": {
			"mem_save":   true,
			"mem_search": true,
			"mem_judge":  true,
		},
	}

	for name, allowlist := range profiles {
		t.Run(name, func(t *testing.T) {
			for _, mode := range []memorypolicy.Mode{memorypolicy.ModeMinimal, memorypolicy.ModeLegacy} {
				instructions := buildServerInstructions(allowlist, mode)
				runes := utf8.RuneCountInString(instructions)
				t.Logf("profile %q mode %q instructions rune count: %d (byte count: %d)", name, mode, runes, len(instructions))
				if runes >= clientTruncationCeiling {
					t.Errorf("profile %q mode %q instructions is %d runes (>=%d) — exceeds the documented 2048-rune MCP client truncation ceiling. Trim prose.",
						name, mode, runes, clientTruncationCeiling)
				}
			}
		})
	}
}

func TestServerInstructionsHasNoCompulsoryRituals(t *testing.T) {
	// PRD R4: no maintained minimal-mode instruction may force memory calls on
	// every turn. Guard against regressions reintroducing the old wording.
	for _, allowlist := range []map[string]bool{nil, ProfileAgent, ProfileAdmin} {
		instructions := buildServerInstructions(allowlist, memorypolicy.ModeMinimal)
		for _, banned := range []string{
			"PROACTIVE SAVE RULE",
			"immediately after ANY",
			"CONFLICT SURFACING",
			"MANDATORY",
			"before saying",
		} {
			if strings.Contains(instructions, banned) {
				t.Errorf("minimal instructions must not contain compulsory directive %q", banned)
			}
		}
	}
}

func TestServerInstructionsMinimalStatesManualReview(t *testing.T) {
	instructions := buildServerInstructions(ProfileAgent, memorypolicy.ModeMinimal)
	if !strings.Contains(instructions, "conflict_review=\"manual\"") {
		t.Errorf("minimal server instructions must explain conflict_review is manual")
	}
}

func TestServerInstructionsLegacyUsesCandidateJudgmentIDs(t *testing.T) {
	// The candidate-judgment flow only applies in legacy mode, where mem_save
	// auto-scans. The minimal default must not carry this directive.
	legacy := buildServerInstructions(ProfileAgent, memorypolicy.ModeLegacy)
	if !strings.Contains(legacy, "judgment_id") {
		t.Errorf("legacy instructions must reference the judgment_id flow")
	}
	if strings.Contains(serverInstructions, "judgment_id") {
		t.Errorf("minimal default instructions must not carry the candidate judgment_id flow")
	}
}
