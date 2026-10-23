package store

import (
	"database/sql"
	"testing"
)

func TestSessionAuthorInheritance(t *testing.T) {
	s := newTestStore(t)
	if err := s.StartSession("author-session", "test", "/work", "codex/model-a"); err != nil {
		t.Fatal(err)
	}
	for _, author := range []string{"", "pi/provider/model-b"} {
		id, err := s.AddObservation(AddObservationParams{SessionID: "author-session", Project: "test", Title: "Provenance " + author, Content: "Unique content " + author, Author: author})
		if err != nil {
			t.Fatal(err)
		}
		obs, err := s.GetObservation(id)
		if err != nil {
			t.Fatal(err)
		}
		want := author
		if want == "" {
			want = "codex/model-a"
		}
		if obs.Author == nil || *obs.Author != want {
			t.Fatalf("author = %v, want %q", obs.Author, want)
		}
	}
	if err := s.StartSession("author-session", "test", "/work", "codex/model-c"); err != nil {
		t.Fatal(err)
	}
	if err := s.StartSession("author-session", "test", "/work"); err != nil {
		t.Fatal(err)
	}
	sess, err := s.GetSession("author-session")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Author != "codex/model-c" {
		t.Fatalf("session author = %q", sess.Author)
	}
	result, err := s.PassiveCapture(PassiveCaptureParams{SessionID: sess.ID, Project: "test", Content: "## Key Learnings:\n- A durable passive learning with inherited provenance."})
	if err != nil || result.Saved != 1 {
		t.Fatalf("passive = %+v, %v", result, err)
	}
	var got string
	if err := s.DB().QueryRow("SELECT author FROM observations WHERE type = 'passive'").Scan(&got); err != nil || got != sess.Author {
		t.Fatalf("passive author = %q, %v", got, err)
	}
}

func TestSessionAuthorExportImportAndSync(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession("roundtrip", "test", "/work", "pi/router/model"); err != nil {
		t.Fatal(err)
	}
	id, err := s.AddObservation(AddObservationParams{SessionID: "roundtrip", Project: "test", Title: "Provenance", Content: "Roundtrip durable knowledge", TopicKey: "author/topic"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Sessions) != 1 || data.Sessions[0].Author != "pi/router/model" {
		t.Fatalf("sessions=%+v", data.Sessions)
	}
	other := newTestStore(t)
	if _, err := other.Import(data); err != nil {
		t.Fatal(err)
	}
	session, err := other.GetSession("roundtrip")
	if err != nil || session.Author != "pi/router/model" {
		t.Fatalf("session=%+v err=%v", session, err)
	}
	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatal(err)
	}
	pulled := newTestStore(t)
	if err := pulled.withTx(func(tx *sql.Tx) error {
		if err := pulled.applySessionPayloadTx(tx, syncSessionPayload{ID: "roundtrip", Project: "test", Author: "pi/router/model", Directory: "/work"}); err != nil {
			return err
		}
		return pulled.applyObservationUpsertTx(tx, observationPayloadFromObservation(obs))
	}); err != nil {
		t.Fatal(err)
	}
	var author string
	if err := pulled.DB().QueryRow(`SELECT author FROM observations`).Scan(&author); err != nil || author != "pi/router/model" {
		t.Fatalf("author=%q err=%v", author, err)
	}
	// A genuine topic revision adopts its current author's model; retries do not.
	if _, err := s.AddObservation(AddObservationParams{SessionID: "roundtrip", Project: "test", Title: "Updated", Content: "Revised durable knowledge", TopicKey: "author/topic", Author: "codex/new"}); err != nil {
		t.Fatal(err)
	}
	obs, err = s.GetObservation(id)
	if err != nil || obs.Author == nil || *obs.Author != "codex/new" {
		t.Fatalf("revision=%+v, %v", obs, err)
	}
}

func TestSessionAuthorRegistrationRollbackAndResume(t *testing.T) {
	s := newTestStore(t)
	if err := s.StartSessionWithOwnershipMode("owned", "a", "/work", SessionOwnershipProjectOwned, "codex/first"); err != nil {
		t.Fatal(err)
	}
	if err := s.StartSessionWithOwnershipMode("owned", "b", "/work", SessionOwnershipProjectOwned, "spoof"); err == nil {
		t.Fatal("expected ownership conflict")
	}
	sess, _ := s.GetSession("owned")
	if sess.Author != "codex/first" {
		t.Fatal("rejected registration changed author")
	}
	if err := s.EndSession("owned", "done"); err != nil {
		t.Fatal(err)
	}
	id, err := s.ResumeSessionWithOwnershipModeTimed("owned", "a", "/work", SessionOwnershipProjectOwned, nil, "codex/next")
	if err != nil {
		t.Fatal(err)
	}
	sess, _ = s.GetSession(id)
	if sess.Author != "codex/next" {
		t.Fatalf("resumed author = %q", sess.Author)
	}
}
