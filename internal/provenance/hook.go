package provenance

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// HookAuthor accepts host hook metadata, not model-supplied tool arguments.
// Unavailable metadata remains unknown: never invent an exact model label.
func HookAuthor(harness string, payload map[string]json.RawMessage) string {
	if os.Getenv("ENGRAM_AUTHOR_SOURCE") == "paseo" {
		if author := PaseoAuthor(); author != "" {
			return author
		}
	}
	if author := strings.TrimSpace(os.Getenv("ENGRAM_AUTHOR")); author != "" {
		return author
	}
	if author := PaseoAuthor(); author != "" {
		return author
	}
	field := func(name string) string {
		var v string
		_ = json.Unmarshal(payload[name], &v)
		return strings.TrimSpace(v)
	}
	if model := field("model"); model != "" {
		return Label(harness, "", model)
	}
	if model := transcriptModel(field("transcript_path")); model != "" {
		return Label(harness, "", model)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if harness == "claude-code" {
		if model := strings.TrimSpace(os.Getenv("ANTHROPIC_MODEL")); model != "" {
			return Label(harness, "", model)
		}
		root := os.Getenv("CLAUDE_CONFIG_DIR")
		if root == "" {
			root = filepath.Join(home, ".claude")
		}
		data, _ := readBounded(filepath.Join(root, "settings.json"), 1<<20)
		var config struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(data, &config)
		return Label(harness, "", config.Model)
	}
	if harness == "codex" {
		root := os.Getenv("CODEX_HOME")
		if root == "" {
			root = filepath.Join(home, ".codex")
		}
		if id := field("session_id"); codexRolloutID(id) {
			patterns := []string{
				filepath.Join(root, "sessions", "*", "*", "*", "rollout-*-"+id+".jsonl"),
				filepath.Join(root, "sessions", "*", "rollout-*-"+id+".jsonl"),
				filepath.Join(root, "sessions", "rollout-*-"+id+".jsonl"),
			}
			for _, pattern := range patterns {
				files, _ := filepath.Glob(pattern)
				for _, file := range files {
					if model := transcriptModel(file); model != "" {
						return Label(harness, "", model)
					}
				}
			}
		}
		data, _ := readBounded(filepath.Join(root, "config.toml"), 1<<20)
		model, provider := codexModelConfig(string(data))
		// openai is Codex's default provider, not a useful extra model qualifier.
		if provider == "openai" {
			provider = ""
		}
		return Label(harness, provider, model)
	}
	return ""
}

// transcriptModel reads only a bounded tail; no transcript content escapes.
func transcriptModel(path string) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	const limit int64 = 512 << 10
	if info.Size() > limit {
		if _, err := f.Seek(-limit, io.SeekEnd); err != nil {
			return ""
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var row struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
			} `json:"message"`
			Payload struct {
				Model string `json:"model"`
			} `json:"payload"`
		}
		if json.Unmarshal([]byte(lines[i]), &row) != nil {
			continue
		}
		if row.Type == "assistant" && row.Message.Model != "" {
			return row.Message.Model
		}
		if (row.Type == "turn_context" || row.Type == "session_meta") && row.Payload.Model != "" {
			return row.Payload.Model
		}
	}
	return ""
}

// Only the two top-level string fields are relevant. Table-scoped model keys
// (profiles/provider definitions) must never leak into the active model label.
func codexModelConfig(data string) (model, provider string) {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			break
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, raw = strings.TrimSpace(key), strings.TrimSpace(raw)
		if key != "model" && key != "model_provider" {
			continue
		}
		var value string
		if strings.HasPrefix(raw, "'") {
			if end := strings.Index(raw[1:], "'"); end >= 0 {
				value = raw[1 : end+1]
			}
		} else if strings.HasPrefix(raw, `"`) {
			// Locate the closing unescaped quote before any inline comment.
			for n := 1; n < len(raw); n++ {
				if raw[n] == '\\' {
					n++
					continue
				}
				if raw[n] == '"' {
					value, _ = strconv.Unquote(raw[:n+1])
					break
				}
			}
		}
		if key == "model" {
			model = value
		} else {
			provider = value
		}
	}
	return
}

// Codex rollout filenames carry UUID host IDs. Do not recursively scan a user's
// entire session archive for arbitrary opaque Engram continuation/test IDs.
func codexRolloutID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, ch := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if ch != '-' {
				return false
			}
			continue
		}
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F') {
			return false
		}
	}
	return true
}
