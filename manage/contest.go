package manage

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/minicago/gooj/sql_service"
)

// CreateContestHandler creates a contest with a collection of problem IDs.
func CreateContestHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := CurrentUsername(r)
	if !CheckUserPermission(currentUser, "EditPermission") {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}

	type reqBody struct {
		Name        string `json:"name"`
		Title       string `json:"title"`
		Description string `json:"description"`
		StartAt     string `json:"start_at"`
		EndAt       string `json:"end_at"`
		ProblemIDs  []uint `json:"problem_ids"`
	}
	var req reqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Title == "" || req.StartAt == "" || req.EndAt == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}

	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		http.Error(w, "invalid start_at format", http.StatusBadRequest)
		return
	}
	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		http.Error(w, "invalid end_at format", http.StatusBadRequest)
		return
	}
	if !endAt.After(startAt) {
		http.Error(w, "end_at must be after start_at", http.StatusBadRequest)
		return
	}

	contest, err := sql_service.CreateContest(strings.TrimSpace(req.Name), strings.TrimSpace(req.Title), strings.TrimSpace(req.Description), currentUser, startAt, endAt, req.ProblemIDs)
	if err != nil {
		http.Error(w, "create contest failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "contest": contest})
}

// DeleteContestHandler deletes a contest by ID.
func DeleteContestHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := CurrentUsername(r)
	if !CheckUserPermission(currentUser, "EditPermission") {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}

	type reqBody struct {
		ContestID uint `json:"contest_id"`
	}
	var req reqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ContestID == 0 {
		http.Error(w, "missing contest_id", http.StatusBadRequest)
		return
	}
	if err := sql_service.DeleteContest(req.ContestID); err != nil {
		http.Error(w, "delete contest failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ReadContestProblemIDsFromQuery parses contest problem ids from a list field.
func ReadContestProblemIDsFromQuery(query string) []uint {
	if query == "" {
		return nil
	}
	parts := strings.Split(query, ",")
	ids := make([]uint, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		id, err := strconv.ParseUint(trimmed, 10, 8)
		if err == nil {
			ids = append(ids, uint(id))
		}
	}
	return ids
}
