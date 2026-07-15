package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/minicago/gooj/manage"
	"github.com/minicago/gooj/sql_service"
)

// requireEdit ensures the caller is authenticated and has the EditPermission.
// It returns the username and true, or writes an error response and returns false.
func requireEdit(w http.ResponseWriter, r *http.Request) (string, bool) {
	username := manage.CurrentUsername(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return "", false
	}
	if !manage.CheckUserPermission(username, "EditPermission") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return "", false
	}
	return username, true
}

// parseIDParam extracts the numeric "id" path variable.
func parseIDParam(r *http.Request) (uint, error) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

// RejudgeHandler POST /api/submission/{id}/rejudge
// Re-queues a single submission so it is judged again.
func RejudgeHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireEdit(w, r); !ok {
		return
	}
	id, err := parseIDParam(r)
	if err != nil {
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}
	if err := sql_service.RejudgeSubmission(uint(id)); err != nil {
		http.Error(w, "failed to rejudge: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONMessage(w, "ok", "rejudge queued")
}

// CancelEvalHandler POST /api/submission/{id}/cancel_eval
// Cancels a queued or running submission (its eventual result is discarded).
func CancelEvalHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireEdit(w, r); !ok {
		return
	}
	id, err := parseIDParam(r)
	if err != nil {
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}
	if err := sql_service.CancelEvaluation(uint(id)); err != nil {
		http.Error(w, "failed to cancel evaluation: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONMessage(w, "ok", "evaluation cancelled")
}

// CancelScoreHandler POST /api/submission/{id}/cancel_score
// Disqualifies a submission: keeps the record but its score no longer counts.
func CancelScoreHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireEdit(w, r); !ok {
		return
	}
	id, err := parseIDParam(r)
	if err != nil {
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}
	if err := sql_service.CancelScore(uint(id)); err != nil {
		http.Error(w, "failed to cancel score: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONMessage(w, "ok", "score cancelled")
}

// RestoreScoreHandler POST /api/submission/{id}/restore_score
// Undoes CancelScore.
func RestoreScoreHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireEdit(w, r); !ok {
		return
	}
	id, err := parseIDParam(r)
	if err != nil {
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}
	if err := sql_service.RestoreScore(uint(id)); err != nil {
		http.Error(w, "failed to restore score: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONMessage(w, "ok", "score restored")
}

// BatchHandler POST /api/submissions/batch
// Applies an administrative action to many submissions at once.
// Body: {"action": "rejudge"|"cancel_eval"|"cancel_score"|"restore_score", "ids": [1,2,3]}
func BatchHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireEdit(w, r); !ok {
		return
	}
	var body struct {
		Action string `json:"action"`
		IDs    []uint `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	switch body.Action {
	case "rejudge", "cancel_eval", "cancel_score", "restore_score":
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if len(body.IDs) == 0 {
		http.Error(w, "no submission ids provided", http.StatusBadRequest)
		return
	}
	affected, err := sql_service.BatchSubmissionAction(body.Action, body.IDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"action":   body.Action,
		"affected": affected,
	})
}

// writeJSONMessage writes a simple {status, message} JSON response.
func writeJSONMessage(w http.ResponseWriter, status, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status, "message": message})
}
