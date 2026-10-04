// Package memorypolicy is the single maintained source for Engram's compact
// agent memory policy. Setup-generated instructions, MCP server instructions,
// and shipped memory skills all derive from the embedded policy here so the
// behavioral rules cannot drift between consumers (PRD R4).
//
// This package intentionally has no Engram dependencies: it is a leaf that both
// internal/setup and internal/mcp may import without creating a cycle or making
// those packages depend on each other.
package memorypolicy

import (
	_ "embed"
	"strings"
)

//go:generate go run ./cmd/genskills -root ../..

//go:embed policy.md
var policyMarkdown string

// Body returns the canonical compact policy Markdown body (no skill
// frontmatter). This is the <=300-word policy shared by every consumer.
func Body() string {
	return strings.TrimRight(normalizeEOL(policyMarkdown), "\n") + "\n"
}

// normalizeEOL makes the embedded asset immune to CRLF checkouts.
func normalizeEOL(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// WordCount counts whitespace-separated tokens in the policy body. Used by the
// size guard test enforcing the <=300-word budget.
func WordCount() int {
	return len(strings.Fields(Body()))
}

// CodexInstructions returns the body written to the Codex
// model_instructions_file. It is exactly the canonical body: Codex loads it as
// persistent model instructions, so no client-specific frontmatter is added.
func CodexInstructions() string {
	return Body()
}

// GeminiInstructions returns the Gemini system-prompt rendering. Gemini has no
// SessionEnd hook, so a single explicit session-close reminder is appended
// without reintroducing per-turn save/search rituals.
func GeminiInstructions() string {
	return Body() + "\nGemini CLI has no SessionEnd hook: at a genuine session" +
		" close call mem_session_summary, then mem_session_end. Not after a" +
		" normal reply.\n"
}

// Client frontmatter blocks. Only frontmatter differs by client; the shared
// Body() carries the behavioral rules so they never drift (PRD R4).
const (
	codexFrontmatter = `---
name: engram-memory
description: "Engram persistent memory — compact policy. Work first; save meaningful outcomes. Do not search/save/judge before every action."
---`

	claudeFrontmatter = "---\n" +
		"name: engram-memory\n" +
		"description: \"Engram persistent memory — compact policy. Work first; save meaningful outcomes. Do not search/save/judge before every action.\"\n" +
		"---\n\n" +
		"## Claude session binding\n\n" +
		"For Engram write and session MCP tools, the Claude plugin binds `session_id` to Claude Code's authoritative session identity before the tool runs. Do not invent, reuse, or override it. Read-only tools are unchanged.\n\n" +
		"**Fallback:** if Engram tools are unexpectedly unavailable, rerun `engram setup claude-code` and restart Claude Code.\n"

	contributorFrontmatter = `---
name: engram-memory-protocol
description: >
  Persistent memory discipline for Engram contributors.
  Trigger: Decisions, bugfixes, discoveries, preferences, or session closure.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "2.0"
---`
)

// Skill renders a client memory skill: client-specific YAML frontmatter plus
// the shared canonical body. Only the frontmatter differs by client; the
// behavioral rules never drift (PRD R4).
func Skill(client string) string {
	return frontmatter(client) + "\n" + Body()
}

func frontmatter(client string) string {
	switch client {
	case "codex":
		return strings.TrimRight(codexFrontmatter, "\n") + "\n"
	case "claude", "claude-code":
		return strings.TrimRight(claudeFrontmatter, "\n") + "\n"
	default:
		return strings.TrimRight(contributorFrontmatter, "\n") + "\n"
	}
}

// ShippedSkill is a repository file rendered from Skill(Client).
type ShippedSkill struct {
	Path   string // slash-separated, relative to the repository root
	Client string
}

// ShippedSkills lists every maintained SKILL.md generated from the canonical
// policy. Regenerate with: go run ./internal/memorypolicy/cmd/genskills
var ShippedSkills = []ShippedSkill{
	{"plugin/codex/skills/memory/SKILL.md", "codex"},
	{"plugin/claude-code/skills/memory/SKILL.md", "claude"},
	{"skills/memory-protocol/SKILL.md", "contributor"},
}

// TSPolicyFiles lists TypeScript plugins that embed the canonical body in a
// generated block delimited by TSBeginMarker / TSEndMarker.
var TSPolicyFiles = []string{
	"plugin/opencode/engram.ts",
	"plugin/pi/index.ts",
}

// TSMirrors maps a generated TypeScript source to byte-identical embedded
// copies that genskills refreshes after regeneration (same as setup's
// go:generate copy step).
var TSMirrors = map[string]string{
	"plugin/opencode/engram.ts": "internal/setup/plugins/opencode/engram.ts",
}

const (
	TSBeginMarker = "// BEGIN GENERATED memorypolicy (go run ./internal/memorypolicy/cmd/genskills)"
	TSEndMarker   = "// END GENERATED memorypolicy"
)

// TSPolicyBlock renders the generated TypeScript block (markers included) that
// declares ENGRAM_POLICY as the canonical body in a template literal.
func TSPolicyBlock() string {
	esc := strings.NewReplacer(`\`, `\\`, "`", "\\`", "${", `\${`).Replace(Body())
	return TSBeginMarker + "\nconst ENGRAM_POLICY = `" + esc + "`\n" + TSEndMarker
}

// ReplaceTSPolicyBlock swaps the generated block inside a TypeScript source.
// ok is false when the markers are missing or out of order.
func ReplaceTSPolicyBlock(src string) (out string, ok bool) {
	return replaceBlock(src, TSBeginMarker, TSEndMarker, TSPolicyBlock())
}

// MDPolicyFiles lists Markdown docs that embed the canonical body between
// MDBeginMarker / MDEndMarker, with headings demoted one level.
var MDPolicyFiles = []string{"DOCS.md"}

const (
	MDBeginMarker = "<!-- BEGIN GENERATED memorypolicy -->"
	MDEndMarker   = "<!-- END GENERATED memorypolicy -->"
)

// MDPolicyBlock renders the generated Markdown block (markers included).
func MDPolicyBlock() string {
	lines := strings.Split(strings.TrimRight(Body(), "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			lines[i] = "#" + line
		}
	}
	return MDBeginMarker + "\n\n" + strings.Join(lines, "\n") + "\n\n" + MDEndMarker
}

// ReplaceMDPolicyBlock swaps the generated block inside a Markdown document,
// preserving the document's CRLF line endings when it uses them.
func ReplaceMDPolicyBlock(src string) (out string, ok bool) {
	crlf := strings.Contains(src, "\r\n")
	out, ok = replaceBlock(normalizeEOL(src), MDBeginMarker, MDEndMarker, MDPolicyBlock())
	if ok && crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return out, ok
}

func replaceBlock(src, begin, end, block string) (string, bool) {
	start := strings.Index(src, begin)
	stop := strings.Index(src, end)
	if start < 0 || stop < start {
		return src, false
	}
	return src[:start] + block + src[stop+len(end):], true
}
