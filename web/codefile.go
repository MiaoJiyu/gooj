package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/minicago/gooj/manage"
	"github.com/minicago/gooj/sql_service"
)

// CodeFileHandler returns last submitted code and result for a user/problem
func CodeFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	user := vars["user"]
	problem := vars["problem"]
	problemID, err := parseProblemID(problem)
	if err != nil {
		http.Error(w, "invalid problem id", http.StatusBadRequest)
		return
	}

	currentUsername := manage.CurrentUsername(r)
	db := sql_service.DB()

	// Gate: check TestVisible + EditPermission before returning evaluation info
	var problemRecord sql_service.Problem
	if err := db.First(&problemRecord, problemID).Error; err == nil {
		canView := manage.CheckUserPermission(currentUsername, "EditPermission") || problemRecord.TestVisible
		if !canView {
			// Return only code, strip evaluation details
			sub, _, _ := sql_service.GetLastSubmission(user, strconv.FormatUint(uint64(problemID), 10))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    sub.Code,
				"summary": map[string]interface{}{"status": ""},
			})
			return
		}
	}

	// fetch last submission from DB
	sub, results, err := sql_service.GetLastSubmission(user, strconv.FormatUint(uint64(problemID), 10))
	if err != nil {
		http.Error(w, "no submission", http.StatusNotFound)
		return
	}
	// return code and a summary
	summary := map[string]interface{}{"status": sub.Status, "test_results": results}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": sub.Code, "summary": summary})
}
