package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPAutomaticAuthorAcrossWrites(t *testing.T) {
	t.Setenv("ENGRAM_AUTHOR", "server/unrelated")
	s := newServerTestStore(t)
	h := New(s, 0).Handler()
	call := func(path, body string, status int) {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s = %d: %s", path, w.Code, w.Body.String())
		}
	}
	call("/sessions", `{"id":"author-http","project":"test","author":"codex/live"}`, 201)
	call("/observations", `{"session_id":"author-http","title":"Automatic","content":"Durable HTTP observation","project":"test"}`, 201)
	call("/observations/passive", `{"session_id":"author-http","project":"test","content":"## Key Learnings:\n- Durable passive HTTP observation with provenance."}`, 200)
	call("/sessions/author-http/summary", `{"content":"A durable session summary"}`, 201)
	rows, err := s.DB().Query(`SELECT author FROM observations`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var author string
		if err := rows.Scan(&author); err != nil {
			t.Fatal(err)
		}
		if author != "codex/live" {
			t.Fatalf("author=%q", author)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count=%d", count)
	}
	call("/sessions", `{"id":"missing-project","author":"bad"}`, 400)
	call("/observations/passive", `{`, 400)
	call("/sessions/author-http/summary", `{"content":" "}`, 400)
	call("/sessions/missing/summary", `{"content":"not saved"}`, 404)
}
