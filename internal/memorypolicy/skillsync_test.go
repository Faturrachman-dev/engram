package memorypolicy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot resolves the repository root from this test file's location so the
// sync check works regardless of the test working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller for repo root")
	}
	// file = <root>/internal/memorypolicy/skillsync_test.go
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

const regenHint = "regenerate with: go run ./internal/memorypolicy/cmd/genskills"

// readNormalized reads a repository file with CRLF normalized to LF, so a
// Windows checkout with core.autocrlf=true cannot cause a false drift report.
func readNormalized(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

// TestShippedSkillsMatchCanonicalPolicy enforces PRD R4: every shipped memory
// skill must equal memorypolicy.Skill(client) (line endings normalized).
func TestShippedSkillsMatchCanonicalPolicy(t *testing.T) {
	root := repoRoot(t)
	for _, tc := range ShippedSkills {
		t.Run(tc.Client, func(t *testing.T) {
			if readNormalized(t, root, tc.Path) != Skill(tc.Client) {
				t.Errorf("%s is out of sync with memorypolicy.Skill(%q); %s", tc.Path, tc.Client, regenHint)
			}
		})
	}
}

// TestTSPluginsEmbedCanonicalPolicy enforces the same sync for the generated
// ENGRAM_POLICY block in the OpenCode and Pi TypeScript plugins.
func TestTSPluginsEmbedCanonicalPolicy(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range TSPolicyFiles {
		t.Run(rel, func(t *testing.T) {
			src := readNormalized(t, root, rel)
			if !strings.Contains(src, TSPolicyBlock()) {
				t.Errorf("%s generated memorypolicy block is missing or stale; %s", rel, regenHint)
			}
		})
	}
}

// TestDocsEmbedCanonicalPolicy keeps the documented protocol identical to the
// policy agents actually receive.
func TestDocsEmbedCanonicalPolicy(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range MDPolicyFiles {
		if !strings.Contains(readNormalized(t, root, rel), MDPolicyBlock()) {
			t.Errorf("%s generated memorypolicy block is missing or stale; %s", rel, regenHint)
		}
	}
}

// TestCodexInstructionsAssetMatchesCanonical guards that the Codex
// model_instructions body setup writes equals the canonical body.
func TestCodexInstructionsAssetMatchesCanonical(t *testing.T) {
	if CodexInstructions() != Body() {
		t.Fatal("CodexInstructions drifted from canonical body")
	}
}
