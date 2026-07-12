package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/minicago/gooj/sql_service"
)

// ListContestsHandler returns all contests plus the mapped problems for each contest.
func ListContestsHandler(w http.ResponseWriter, r *http.Request) {
	contests, err := sql_service.ListContests()
	if err != nil {
		http.Error(w, "failed to list contests", http.StatusInternalServerError)
		return
	}

	response := make([]map[string]interface{}, 0, len(contests))
	for _, contest := range contests {
		problems, err := sql_service.ListContestProblems(contest.ID)
		if err != nil {
			problems = []sql_service.Problem{}
		}
		contestInfo := map[string]interface{}{
			"id":          contest.ID,
			"name":        contest.Name,
			"title":       contest.Title,
			"description": contest.Description,
			"start_at":    contest.StartAt,
			"end_at":      contest.EndAt,
			"created_by":  contest.CreatedBy,
			"problems":    problems,
		}
		response = append(response, contestInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"contests": response,
		"total":    len(response),
	})
}

// ContestDetailHandler returns a single contest's metadata and linked problems.
func ContestDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid contest id", http.StatusBadRequest)
		return
	}

	contest, err := sql_service.GetContestByID(uint(id))
	if err != nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	problems, err := sql_service.ListContestProblems(contest.ID)
	if err != nil {
		http.Error(w, "failed to load contest problems", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          contest.ID,
		"name":        contest.Name,
		"title":       contest.Title,
		"description": contest.Description,
		"start_at":    contest.StartAt,
		"end_at":      contest.EndAt,
		"created_by":  contest.CreatedBy,
		"problems":    problems,
	})
}

// ContestLeaderboardHandler returns the current ranking for a contest.
func ContestLeaderboardHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid contest id", http.StatusBadRequest)
		return
	}

	rows, err := sql_service.GetContestLeaderboard(uint(id))
	if err != nil {
		http.Error(w, "failed to load leaderboard", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"contest_id": id,
		"rows":       rows,
		"total":      len(rows),
	})
}
