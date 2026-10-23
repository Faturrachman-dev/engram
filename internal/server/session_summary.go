package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Gentleman-Programming/engram/v3/internal/store"
)

// A summary is an observation; it does not end or reopen the parent session.
func (s *Server) handleSessionSummary(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
		Project string `json:"project"`
		Author  string `json:"author"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || strings.TrimSpace(body.Content) == "" {
		jsonError(w, http.StatusBadRequest, "content is required in valid JSON")
		return
	}
	id := r.PathValue("id")
	session, err := s.store.GetSession(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "session not found")
		} else {
			jsonError(w, http.StatusInternalServerError, "session unavailable")
		}
		return
	}
	if body.Project == "" {
		body.Project = session.Project
	}
	if !s.validateSessionProject(w, id, body.Project) {
		return
	}
	result, err := s.store.AddObservation(store.AddObservationParams{SessionID: id, Project: body.Project, Type: "session_summary", Title: "Session summary", Content: body.Content, Author: s.resolveAuthor(body.Author, id), Timings: requestWriteTimings(r)})
	if err != nil {
		if !writeOwnershipError(w, id, err) {
			jsonError(w, http.StatusInternalServerError, "summary save failed")
		}
		return
	}
	s.notifyWrite()
	jsonResponse(w, http.StatusCreated, map[string]any{"id": result, "status": "saved"})
}
