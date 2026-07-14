package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/minicago/gooj/manage"
	"github.com/minicago/gooj/sql_service"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

func parseProblemID(raw string) (uint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("missing problem id")
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid problem id %q", raw)
	}
	return uint(id), nil
}

// ProblemsHandler returns paginated problems. Query params: page (1-based), per (default 10)
func ProblemsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := 1
	per := 10
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if p := q.Get("per"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			per = v
		}
	}

	currentUser := manage.CurrentUsername(r)
	canEdit := manage.CheckUserPermission(currentUser, "EditPermission")
	db := sql_service.DB()
	if db == nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	query := db.Model(&sql_service.Problem{})
	if !canEdit {
		query = query.Where("problem_visible = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var probs []sql_service.Problem
	offset := (page - 1) * per
	// Only select fields needed for list view (avoid selecting large Description field)
	query.Select("id, title, tests_count, time_limit_ms, mem_limit_mb, problem_visible, test_visible").
		Order("id asc").Offset(offset).Limit(per).Find(&probs)

	// Return only necessary fields for problem list (exclude Description which can be large)
	safeProbs := make([]map[string]interface{}, len(probs))
	for i, p := range probs {
		safeProbs[i] = map[string]interface{}{
			"id":              p.ID,
			"title":           p.Title,
			"tests_count":     p.TestsCount,
			"time_limit_ms":   p.TimeLimitMs,
			"mem_limit_mb":    p.MemLimitMB,
			"problem_visible": p.ProblemVisible,
			"test_visible":    p.TestVisible,
		}
	}

	out := map[string]interface{}{"problems": safeProbs, "total": total, "page": page, "per": per}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// LastSubmissionHandler returns the latest submission and test results for a user and numeric problem id.
func LastSubmissionHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	user := q.Get("username")
	prob := q.Get("problem")
	if user == "" || prob == "" {
		http.Error(w, "missing params", http.StatusBadRequest)
		return
	}

	problemID, err := parseProblemID(prob)
	if err != nil {
		http.Error(w, "invalid problem id", http.StatusBadRequest)
		return
	}

	currentUsername := manage.CurrentUsername(r)
	db := sql_service.DB()

	// Gate: check TestVisible + EditPermission before returning any evaluation info
	var problem sql_service.Problem
	if err := db.First(&problem, problemID).Error; err == nil {
		canView := manage.CheckUserPermission(currentUsername, "EditPermission") || problem.TestVisible
		if !canView {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"submission": nil, "results": []interface{}{}})
			return
		}
	}

	sub, results, err := sql_service.GetLastSubmission(user, strconv.FormatUint(uint64(problemID), 10))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// if no submission, return empty
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"submission": sub, "results": results})
}

// ProblemDataHandler returns the statement.md and config.json for a given problem id.
func ProblemDataHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	problemID, err := parseProblemID(vars["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var problem sql_service.Problem
	db := sql_service.DB()
	if db == nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if err := db.First(&problem, problemID).Error; err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	currentUser := manage.CurrentUsername(r)
	if !problem.ProblemVisible && !manage.CheckUserPermission(currentUser, "EditPermission") {
		http.Error(w, "problem is not visible yet", http.StatusForbidden)
		return
	}

	// Use the problem ID as directory name
	problemDirID := strconv.FormatUint(uint64(problem.ID), 10)
	base := filepath.Join("data", "problem", problemDirID)
	stmtPath := filepath.Join(base, "statement.md")
	cfgPath := filepath.Join(base, "config.json")
	stmtBytes, err := os.ReadFile(stmtPath)
	if err != nil {
		if os.IsNotExist(err) {
			stmtBytes = []byte("# Problem statement not found\n\nPlease contact the administrator.")
			return
		}
		http.Error(w, "internal read error", http.StatusInternalServerError)
		return
	}
	cfgBytes, err := os.ReadFile(cfgPath)
	var cfg interface{}
	if err == nil {
		_ = json.Unmarshal(cfgBytes, &cfg)
	} else {
		// if config missing, leave cfg nil
		cfg = nil
	}
	// render markdown to HTML
	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.Typographer,
		),
	)
	_ = md.Convert(stmtBytes, &buf)
	out := map[string]interface{}{
		"id": problem.ID,
		// "name":           problem.Name,
		"title":           problem.Title,
		"description":     problem.Description,
		"time_limit_ms":   problem.TimeLimitMs,
		"mem_limit_mb":    problem.MemLimitMB,
		"tests_count":     problem.TestsCount,
		"problem_visible": problem.ProblemVisible,
		"test_visible":    problem.TestVisible,
		"statement":       string(stmtBytes),
		"statement_html":  buf.String(),
		"config":          cfg,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// UpdateProblemHandler handles updating problem metadata
func UpdateProblemHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	problemID, err := parseProblemID(vars["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check permission
	currentUser := manage.CurrentUsername(r)
	if !manage.CheckUserPermission(currentUser, "EditPermission") {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}

	// Parse request body
	type UpdateRequest struct {
		Name           string `json:"name"`
		Title          string `json:"title"`
		Description    string `json:"description"`
		TimeLimitMs    int    `json:"time_limit_ms"`
		MemLimitMB     int    `json:"mem_limit_mb"`
		ProblemVisible *bool  `json:"problem_visible"`
		TestVisible    *bool  `json:"test_visible"`
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Find problem in database
	var problem sql_service.Problem
	db := sql_service.DB()
	if db == nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if err := db.First(&problem, problemID).Error; err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	// Update problem fields if provided
	// if req.Name != "" {
	// 	problem.Name = req.Name
	// }
	if req.Title != "" {
		problem.Title = req.Title
	}
	if req.Description != "" {
		problem.Description = req.Description
	}
	if req.TimeLimitMs > 0 {
		problem.TimeLimitMs = req.TimeLimitMs
	}
	if req.MemLimitMB > 0 {
		problem.MemLimitMB = req.MemLimitMB
	}
	if req.ProblemVisible != nil {
		problem.ProblemVisible = *req.ProblemVisible
	}
	if req.TestVisible != nil {
		problem.TestVisible = *req.TestVisible
	}

	// Save to database
	if err := db.Save(&problem).Error; err != nil {
		http.Error(w, "failed to save problem", http.StatusInternalServerError)
		return
	}

	// Update config.json if time_limit or mem_limit changed
	if req.TimeLimitMs > 0 || req.MemLimitMB > 0 {
		problemDir := filepath.Join("data", "problem", strconv.FormatUint(uint64(problem.ID), 10))
		cfgPath := filepath.Join(problemDir, "config.json")

		var cfg map[string]interface{}
		if cfgBytes, err := os.ReadFile(cfgPath); err == nil {
			_ = json.Unmarshal(cfgBytes, &cfg)
		} else {
			cfg = make(map[string]interface{})
		}

		if req.TimeLimitMs > 0 {
			cfg["time_limit"] = req.TimeLimitMs
		}
		if req.MemLimitMB > 0 {
			cfg["memory_limit"] = req.MemLimitMB
		}

		if cfgBytes, err := json.MarshalIndent(cfg, "", "    "); err == nil {
			_ = os.WriteFile(cfgPath, cfgBytes, 0644)
		}
	}

	// Update statement.md if description changed
	if req.Description != "" {
		problemDir := filepath.Join("data", "problem", strconv.FormatUint(uint64(problem.ID), 10))
		stmtPath := filepath.Join(problemDir, "statement.md")
		_ = os.WriteFile(stmtPath, []byte(req.Description), 0644)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"problem": map[string]interface{}{
			"id": problem.ID,
			// "name":          problem.Name,
			"title":           problem.Title,
			"description":     problem.Description,
			"time_limit_ms":   problem.TimeLimitMs,
			"mem_limit_mb":    problem.MemLimitMB,
			"tests_count":     problem.TestsCount,
			"problem_visible": problem.ProblemVisible,
		},
	})
}
