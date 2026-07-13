package edit

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/minicago/gooj/manage"
	"github.com/minicago/gooj/sql_service"
	"github.com/minicago/gooj/tuack"
)

// clearTestFiles removes old test files (.in and .ans) from the problem directory
func clearTestFiles(problemDir string) error {
	entries, err := os.ReadDir(problemDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && (hasSuffix(name, ".in") || hasSuffix(name, ".ans")) {
			if err := os.Remove(filepath.Join(problemDir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// hasSuffix is a helper to check string suffix
func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// isPathSafe checks if a path is safe from directory traversal attacks.
// It ensures the resolved path is within the expected base directory.
func isPathSafe(baseDir, userPath string) bool {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	absUserPath, err := filepath.Abs(filepath.Join(baseDir, userPath))
	if err != nil {
		return false
	}
	// Ensure the resolved path starts with the base directory
	return strings.HasPrefix(absUserPath, absBase)
}

// validateProblemID checks if a problem ID is safe to use in paths
func validateProblemID(problemID string) bool {
	// Problem ID must be a valid number (positive integer)
	id, err := strconv.Atoi(problemID)
	if err != nil || id <= 0 {
		return false
	}
	// Additional check: ensure no path traversal attempts
	if strings.ContainsAny(problemID, "/\\..") {
		return false
	}
	return true
}

// calcScore distributes totalScore evenly across count items, returning the i-th item's share
// For example: calcScore(100, 3, 0) = 34, calcScore(100, 3, 1) = 33, calcScore(100, 3, 2) = 33
func calcScore(totalScore, count, index int) int {
	if count <= 0 {
		return 0
	}
	base := totalScore / count
	remainder := totalScore % count
	if index < remainder {
		return base + 1
	}
	return base
}

func ModifyProblemStatementHandler(w http.ResponseWriter, r *http.Request) {
	type reqBody struct {
		ProblemID    string `json:"problem_id"`
		NewStatement string `json:"new_statement"`
	}
	var req reqBody
	_ = json.NewDecoder(r.Body).Decode(&req)
	if !manage.CheckUserPermission(manage.CurrentUsername(r), "EditPermission") {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}
	if req.ProblemID == "" || req.NewStatement == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}
	// Validate problem ID to prevent path traversal
	if !validateProblemID(req.ProblemID) {
		http.Error(w, "invalid problem id", http.StatusBadRequest)
		return
	}
	baseDir := filepath.Join("data", "problem")
	if !isPathSafe(baseDir, req.ProblemID) {
		http.Error(w, "invalid problem path", http.StatusBadRequest)
		return
	}
	statementPath := filepath.Join(baseDir, req.ProblemID, "statement.md")
	if err := os.WriteFile(statementPath, []byte(req.NewStatement), 0644); err != nil {
		http.Error(w, "failed to modify statement", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func AddTestDataHandler(w http.ResponseWriter, r *http.Request) {
	type reqBody struct {
		ProblemID  string `json:"problem_id"`
		TestIndex  int    `json:"test_index"`
		InputData  string `json:"input_data"`
		OutputData string `json:"output_data"`
	}
	var req reqBody
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.ProblemID == "" || req.TestIndex <= 0 || req.InputData == "" || req.OutputData == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}
	// Validate problem ID to prevent path traversal
	if !validateProblemID(req.ProblemID) {
		http.Error(w, "invalid problem id", http.StatusBadRequest)
		return
	}
	baseDir := filepath.Join("data", "problem", req.ProblemID, "tests")
	if !isPathSafe(filepath.Join("data", "problem"), req.ProblemID) {
		http.Error(w, "invalid problem path", http.StatusBadRequest)
		return
	}
	inputPath := filepath.Join(baseDir, fmt.Sprintf("%d.in", req.TestIndex))
	outputPath := filepath.Join(baseDir, fmt.Sprintf("%d.out", req.TestIndex))
	if err := os.WriteFile(inputPath, []byte(req.InputData), 0644); err != nil {
		http.Error(w, "failed to add input data", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(outputPath, []byte(req.OutputData), 0644); err != nil {
		http.Error(w, "failed to add output data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ImportTuackHandler handles importing a tuack package from a zip file
func ImportTuackHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user has edit permission
	currentUser := manage.CurrentUsername(r)
	if !manage.CheckUserPermission(currentUser, "EditPermission") {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}

	// Parse multipart form with 100MB max memory
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	// Get problem ID from form
	problemID := r.FormValue("problem_id")
	if problemID == "" {
		http.Error(w, "problem_id is required", http.StatusBadRequest)
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get uploaded file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type
	if filepath.Ext(header.Filename) != ".zip" {
		http.Error(w, "Only .zip files are allowed", http.StatusBadRequest)
		return
	}

	// Create temporary file to save the zip
	tempDir, err := os.MkdirTemp("", "tuack-import-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp directory: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "problem.zip")
	dst, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp file: %v", err), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy uploaded file to temp location
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save uploaded file: %v", err), http.StatusInternalServerError)
		return
	}

	// Import the tuack package using existing function
	// Note: ImportTuackPackage expects name and title parameters, but we're importing to an existing problem
	// We need to get the existing problem's name and title from the database
	db := sql_service.DB()
	if db == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	var problem sql_service.Problem
	if err := db.First(&problem, problemID).Error; err != nil {
		http.Error(w, fmt.Sprintf("Problem not found: %v", err), http.StatusNotFound)
		return
	}

	// Use existing problem name and title and update the problem
	result, err := tuack.UpdateTuackPackage(zipPath, problem.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update tuack package: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"problem_id": result.ProblemID,
		"name":       result.Name,
		"title":      result.Title,
		"message":    result.Message,
	})
}

// ImportDataZipHandler imports a simple data.zip containing only *.in and *.ans test files.
// It auto-generates config.json with one group containing all test cases.
func ImportDataZipHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := manage.CurrentUsername(r)
	if !manage.CheckUserPermission(currentUser, "EditPermission") {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	problemIDStr := r.FormValue("problem_id")
	if problemIDStr == "" {
		http.Error(w, "problem_id is required", http.StatusBadRequest)
		return
	}

	problemID, err := strconv.ParseUint(problemIDStr, 10, 64)
	if err != nil || problemID == 0 {
		http.Error(w, "invalid problem_id", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get uploaded file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if filepath.Ext(header.Filename) != ".zip" {
		http.Error(w, "Only .zip files are allowed", http.StatusBadRequest)
		return
	}

	db := sql_service.DB()
	if db == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	var problem sql_service.Problem
	if err := db.First(&problem, uint(problemID)).Error; err != nil {
		http.Error(w, "Problem not found", http.StatusNotFound)
		return
	}

	// Define problem directory
	problemDir := filepath.Join("data", "problem", problemIDStr)

	// Save zip to temp file
	tempDir, err := os.MkdirTemp("", "datazip-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp dir: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "data.zip")
	dst, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create temp file: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		http.Error(w, fmt.Sprintf("Failed to save zip: %v", err), http.StatusInternalServerError)
		return
	}
	dst.Close()

	// Open and extract
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open zip: %v", err), http.StatusBadRequest)
		return
	}
	defer reader.Close()

	// Clear old test files in problem directory first
	if err := clearTestFiles(problemDir); err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear old test files: %v", err), http.StatusInternalServerError)
		return
	}

	// Collect .in and .ans files keyed by base name (e.g. "1" -> {"in":"1.in content", "ans":"1.ans content"})
	inRE := regexp.MustCompile(`^(.+)\.in$`)
	ansRE := regexp.MustCompile(`^(.+)\.ans$`)

	type filePair struct {
		in  []byte
		ans []byte
	}
	pairs := make(map[string]*filePair)

	for _, f := range reader.File {
		name := filepath.Base(f.Name)
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read %s: %v", name, err), http.StatusInternalServerError)
			return
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read %s: %v", name, err), http.StatusInternalServerError)
			return
		}

		if m := inRE.FindStringSubmatch(name); m != nil {
			base := m[1]
			if pairs[base] == nil {
				pairs[base] = &filePair{}
			}
			pairs[base].in = content
		} else if m := ansRE.FindStringSubmatch(name); m != nil {
			base := m[1]
			if pairs[base] == nil {
				pairs[base] = &filePair{}
			}
			pairs[base].ans = content
		}
		// Skip any other file types silently
	}

	if len(pairs) == 0 {
		http.Error(w, "No .in / .ans files found in zip", http.StatusBadRequest)
		return
	}

	// Sort bases numerically
	var bases []string
	for base := range pairs {
		bases = append(bases, base)
	}
	sort.Slice(bases, func(i, j int) bool {
		vi, _ := strconv.Atoi(bases[i])
		vj, _ := strconv.Atoi(bases[j])
		return vi < vj
	})

	// Write test files and build config
	type testCaseConfig struct {
		InputFile  string `json:"input_file"`
		OutputFile string `json:"output_file"`
		Score      int    `json:"score"`
	}
	testCases := make([]testCaseConfig, 0, len(bases))
	// Each test case is its own group
	testGroups := make([]map[string]interface{}, 0, len(bases))

	for i, base := range bases {
		pair := pairs[base]
		if pair == nil {
			continue
		}
		idx := i + 1

		// Write .in directly in problem folder
		inPath := filepath.Join(problemDir, fmt.Sprintf("%d.in", idx))
		if err := os.WriteFile(inPath, pair.in, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write %s: %v", inPath, err), http.StatusInternalServerError)
			return
		}
		// Write .ans directly in problem folder
		ansPath := filepath.Join(problemDir, fmt.Sprintf("%d.ans", idx))
		if err := os.WriteFile(ansPath, pair.ans, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write %s: %v", ansPath, err), http.StatusInternalServerError)
			return
		}

		testCases = append(testCases, testCaseConfig{
			InputFile:  fmt.Sprintf("%d.in", idx),
			OutputFile: fmt.Sprintf("%d.ans", idx),
			Score:      calcScore(100, len(bases), i),
		})
		// Each test case is its own group with evenly distributed score
		testGroups = append(testGroups, map[string]interface{}{
			"cases": []int{idx},
			"score": calcScore(100, len(bases), i),
		})
	}

	// Write config.json with one group per test case
	config := map[string]interface{}{
		"test_cases": testGroups,
	}
	configPath := filepath.Join("data", "problem", problemIDStr, "config.json")
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write config.json: %v", err), http.StatusInternalServerError)
		return
	}

	// Update database
	problem.TestsCount = len(bases)
	if err := db.Save(&problem).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to update problem: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"problem_id": problemID,
		"test_count": len(bases),
		"tests":      testCases,
		"message":    fmt.Sprintf("Successfully imported %d test cases", len(bases)),
	})
}

// SaveTestGroupsHandler saves the test groups configuration for a problem.
func SaveTestGroupsHandler(w http.ResponseWriter, r *http.Request) {
	type TestGroup struct {
		Cases []int `json:"cases"`
		Score int   `json:"score"`
	}
	type reqBody struct {
		ProblemID  string      `json:"problem_id"`
		TestGroups []TestGroup `json:"test_groups"`
	}
	var req reqBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !manage.CheckUserPermission(manage.CurrentUsername(r), "EditPermission") {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}
	if req.ProblemID == "" {
		http.Error(w, "missing problem_id", http.StatusBadRequest)
		return
	}

	// Read existing config to preserve time_limit and memory_limit
	configPath := filepath.Join("data", "problem", req.ProblemID, "config.json")
	var config map[string]interface{}
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &config)
	} else {
		config = make(map[string]interface{})
	}

	// Update test_cases
	groups := make([]map[string]interface{}, 0, len(req.TestGroups))
	for _, g := range req.TestGroups {
		groups = append(groups, map[string]interface{}{
			"cases": g.Cases,
			"score": g.Score,
		})
	}
	config["test_cases"] = groups

	// Write updated config
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write config.json: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// CreateProblemHandler creates a new problem with optional statement and data zip.
func CreateProblemHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := manage.CurrentUsername(r)
	if !manage.CheckUserPermission(currentUser, "EditPermission") {
		http.Error(w, "permission denied", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	statement := r.FormValue("statement")

	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	// Save problem to database first to get the ID
	db := sql_service.DB()
	if db == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	problem := sql_service.Problem{
		Title:       title,
		TestsCount:  0,
		TimeLimitMs: 1000,
		MemLimitMB:  512,
	}
	if err := db.Create(&problem).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to save problem to database: %v", err), http.StatusInternalServerError)
		return
	}

	// Now create directory using the database ID
	problemDir := filepath.Join("data", "problem", strconv.FormatUint(uint64(problem.ID), 10))
	if err := os.MkdirAll(problemDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create problem directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Save statement if provided
	if statement != "" {
		statementPath := filepath.Join(problemDir, "statement.md")
		if err := os.WriteFile(statementPath, []byte(statement), 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write statement: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Handle data zip if provided
	testCount := 0
	dataZip, dataZipHeader, err := r.FormFile("data_zip")
	if err == nil && dataZipHeader != nil {
		defer dataZip.Close()

		if filepath.Ext(dataZipHeader.Filename) == ".zip" {
			// Save zip to temp file
			tempDir, err := os.MkdirTemp("", "datazip-*")
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create temp dir: %v", err), http.StatusInternalServerError)
				return
			}
			defer os.RemoveAll(tempDir)

			zipPath := filepath.Join(tempDir, "data.zip")
			dst, err := os.Create(zipPath)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create temp file: %v", err), http.StatusInternalServerError)
				return
			}
			if _, err := io.Copy(dst, dataZip); err != nil {
				dst.Close()
				http.Error(w, fmt.Sprintf("Failed to save zip: %v", err), http.StatusInternalServerError)
				return
			}
			dst.Close()

			// Open and extract
			reader, err := zip.OpenReader(zipPath)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to open zip: %v", err), http.StatusBadRequest)
				return
			}
			defer reader.Close()

			// Collect .in and .ans files keyed by base name
			inRE := regexp.MustCompile(`^(.+)\.in$`)
			ansRE := regexp.MustCompile(`^(.+)\.ans$`)
			type filePair struct {
				in  []byte
				ans []byte
			}
			pairs := make(map[string]*filePair)

			for _, f := range reader.File {
				name := filepath.Base(f.Name)
				if f.FileInfo().IsDir() {
					continue
				}
				rc, err := f.Open()
				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to read %s: %v", name, err), http.StatusInternalServerError)
					return
				}
				content, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to read %s: %v", name, err), http.StatusInternalServerError)
					return
				}

				if m := inRE.FindStringSubmatch(name); m != nil {
					base := m[1]
					if pairs[base] == nil {
						pairs[base] = &filePair{}
					}
					pairs[base].in = content
				} else if m := ansRE.FindStringSubmatch(name); m != nil {
					base := m[1]
					if pairs[base] == nil {
						pairs[base] = &filePair{}
					}
					pairs[base].ans = content
				}
			}

			// Sort bases numerically
			var bases []string
			for base := range pairs {
				bases = append(bases, base)
			}
			sort.Slice(bases, func(i, j int) bool {
				vi, _ := strconv.Atoi(bases[i])
				vj, _ := strconv.Atoi(bases[j])
				return vi < vj
			})

			// Write test files, each test case is its own group
			testGroups := make([]map[string]interface{}, 0, len(bases))
			for i, base := range bases {
				pair := pairs[base]
				if pair == nil {
					continue
				}
				idx := i + 1
				testCount++

				// Write .in directly in problem folder
				inPath := filepath.Join(problemDir, fmt.Sprintf("%d.in", idx))
				if err := os.WriteFile(inPath, pair.in, 0644); err != nil {
					http.Error(w, fmt.Sprintf("Failed to write %s: %v", inPath, err), http.StatusInternalServerError)
					return
				}
				// Write .ans directly in problem folder
				ansPath := filepath.Join(problemDir, fmt.Sprintf("%d.ans", idx))
				if err := os.WriteFile(ansPath, pair.ans, 0644); err != nil {
					http.Error(w, fmt.Sprintf("Failed to write %s: %v", ansPath, err), http.StatusInternalServerError)
					return
				}

				// Each test case is its own group with evenly distributed score
				testGroups = append(testGroups, map[string]interface{}{
					"cases": []int{idx},
					"score": calcScore(100, len(bases), i),
				})
			}

			// Write config.json with one group per test case
			config := map[string]interface{}{
				"test_cases":   testGroups,
				"time_limit":   1000,
				"memory_limit": 512,
			}
			configPath := filepath.Join(problemDir, "config.json")
			configJSON, _ := json.MarshalIndent(config, "", "  ")
			if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
				http.Error(w, fmt.Sprintf("Failed to write config.json: %v", err), http.StatusInternalServerError)
				return
			}

			// Update problem test count
			problem.TestsCount = testCount
			db.Save(&problem)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"problem_id": problem.ID,
		"title":      title,
		"test_count": testCount,
		"message":    "Problem created successfully",
	})
}
