package mcp

import (
	"context"
	"testing"
	"time"

	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestMCPAutomaticAuthorAcrossWrites(t *testing.T) {
	t.Setenv("ENGRAM_AUTHOR", "server/unrelated")
	s := newMCPTestStore(t)
	if err := s.StartSession("author-mcp", "test", "", "omp/router/live"); err != nil {
		t.Fatal(err)
	}
	a := NewSessionActivity(10 * time.Minute)
	for _, tc := range []struct {
		handler server.ToolHandlerFunc
		args    map[string]any
	}{
		{handleSave(s, MCPConfig{}, a), map[string]any{"title": "Automatic", "content": "Durable MCP observation"}},
		{handleSessionSummary(s, MCPConfig{}, a), map[string]any{"content": "Durable MCP summary"}},
		{handleCapturePassive(s, MCPConfig{}, a), map[string]any{"content": "## Key Learnings:\n- Durable MCP passive observation with provenance."}},
	} {
		tc.args["session_id"] = "author-mcp"
		result, err := tc.handler(context.Background(), mcppkg.CallToolRequest{Params: mcppkg.CallToolParams{Arguments: tc.args}})
		if err != nil || result.IsError {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	var count int
	if err := s.DB().QueryRow(`SELECT count(*) FROM observations WHERE author = 'omp/router/live'`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("count=%d, err=%v", count, err)
	}
}
