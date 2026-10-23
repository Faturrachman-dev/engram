// Package provenance resolves agent/model labels from trusted runtime metadata.
// It never reads tool_input or persists transcripts, configuration, or agent IDs.
package provenance

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Resolve applies write-boundary precedence. Session inheritance outranks the
// server process environment, which may belong to a different connected agent.
func Resolve(explicit, session string) string {
	for _, value := range []string{explicit, session} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	if value := PaseoAuthor(); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("ENGRAM_AUTHOR"))
}

// Label avoids repeating a provider already embedded in a model identifier.
func Label(harness, provider, model string) string {
	harness, provider, model = strings.TrimSpace(harness), strings.TrimSpace(provider), strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if provider != "" && !strings.HasPrefix(model, provider+"/") {
		model = provider + "/" + model
	}
	return harness + "/" + model
}

// PaseoAuthor consults only allowlisted metadata, refreshing on every write so
// runtime model changes take effect without restarting a long-lived MCP process.
func PaseoAuthor() string {
	id := os.Getenv("PASEO_AGENT_ID")
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\:`) {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	root := strings.TrimSpace(os.Getenv("PASEO_HOME"))
	if root == "" {
		root = filepath.Join(home, ".paseo")
	}
	file := filepath.Join(root, "agents", id+".json")
	if _, err := os.Stat(file); os.IsNotExist(err) {
		// Current Paseo groups agent records one level below agents by cwd slug.
		entries, _ := os.ReadDir(filepath.Join(root, "agents"))
		for _, entry := range entries {
			if entry.IsDir() {
				candidate := filepath.Join(root, "agents", entry.Name(), id+".json")
				if _, err := os.Stat(candidate); err == nil {
					file = candidate
					break
				}
			}
		}
	}
	data, err := readBounded(file, 4<<20)
	if err != nil {
		return ""
	}
	var state struct {
		Provider    string `json:"provider"`
		RuntimeInfo struct {
			Model string `json:"model"`
		} `json:"runtimeInfo"`
		Config struct {
			Provider string `json:"provider"`
			Model    string `json:"model"`
		} `json:"config"`
	}
	if json.Unmarshal(data, &state) != nil {
		return ""
	}
	provider := state.Provider
	if provider == "" {
		provider = state.Config.Provider
	}
	model := state.RuntimeInfo.Model
	if model == "" {
		model = state.Config.Model
	}
	switch provider {
	case "claude":
		provider = "claude-code"
	case "oh-my-pi":
		provider = "omp"
	}
	if provider == "" {
		return ""
	}
	return Label(provider, "", model)
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}
