package memorypolicy

import (
	"fmt"
	"strings"
)

// Mode selects the MCP memory behavior contract (PRD §6).
type Mode string

const (
	// ModeMinimal is the default: no automatic conflict scan after ordinary
	// saves, compact search by default, deliberate prompt capture.
	ModeMinimal Mode = "minimal"
	// ModeLegacy preserves the historical response/review behavior: automatic
	// candidate scan after saves and legacy search representation. It is a
	// compatibility option only, never a license to weaken safeguards.
	ModeLegacy Mode = "legacy"
)

// DefaultMode is used when neither CLI nor environment selects a mode.
const DefaultMode = ModeMinimal

// ParseMode validates an explicit mode string. Empty is rejected by callers
// that require an explicit value; use ResolveMode for precedence handling.
func ParseMode(value string) (Mode, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(ModeMinimal):
		return ModeMinimal, nil
	case string(ModeLegacy):
		return ModeLegacy, nil
	default:
		return "", fmt.Errorf("invalid memory mode %q: must be %q or %q", value, ModeMinimal, ModeLegacy)
	}
}

// ResolveMode applies PRD precedence: explicit CLI value > environment value >
// minimal default. An explicitly provided but invalid CLI or environment value
// is an error (fail clearly). Empty strings mean "not provided".
func ResolveMode(cliValue, envValue string) (Mode, error) {
	if strings.TrimSpace(cliValue) != "" {
		m, err := ParseMode(cliValue)
		if err != nil {
			return "", fmt.Errorf("--memory-mode: %w", err)
		}
		return m, nil
	}
	if strings.TrimSpace(envValue) != "" {
		m, err := ParseMode(envValue)
		if err != nil {
			return "", fmt.Errorf("ENGRAM_MEMORY_MODE: %w", err)
		}
		return m, nil
	}
	return DefaultMode, nil
}
