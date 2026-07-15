package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	// "io/os"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/minicago/gooj/sql_service"
)

// JudgeConfig contains configuration for judging a single test case
type JudgeConfig struct {
	TimeLimit    float64 // time limit in seconds
	MemLimit     int     // memory limit in MB
	InputPath    string  // path to input file
	ExpectedPath string  // path to expected output file
	WorkTmpPath  string  // path to temporary working directory for this test
}

// JudgeResult contains the result of judging a single test case
type JudgeResult struct {
	RunTimeMs int    // execution time in milliseconds
	MemoryKB  int    // memory usage in kilobytes
	Passed    bool   // whether output matches (ignoring trailing spaces and newlines)
	Info      string // the differing character from output, empty if passed
	Status    string // "accepted", "time_limit_exceeded", "memory_limit_exceeded", "runtime_error", "wrong_answer"
}

// JudgeTest judges a single test case with the given configuration
// It runs the solution binary in a Docker container and returns the result
func JudgeTest(cfg JudgeConfig) JudgeResult {
	result := JudgeResult{
		RunTimeMs: 0,
		MemoryKB:  0,
		Passed:    false,
		Info:      "",
		Status:    "runtime_error",
	}

	// Read input file
	inputData, err := os.ReadFile(cfg.InputPath)
	if err != nil {
		result.Info = fmt.Sprintf("Failed to read input: %v", err)
		return result
	}

	// Write input file
	if err := os.WriteFile(filepath.Join(cfg.WorkTmpPath, "in.in"), inputData, 0644); err != nil {
		result.Info = fmt.Sprintf("Failed to write input: %v", err)
		return result
	}

	// Prepare Docker command with time and memory limits
	absTmp, _ := filepath.Abs(cfg.WorkTmpPath)

	// Simple shell command - run solution. Program output/errors are redirected to
	// files in the mounted work dir because a detached container's stdio is not
	// captured by the docker client.
	shellCmd := fmt.Sprintf("/usr/bin/time -v -o time.log ./solution < in.in > out.out 2>runtime.err; echo $? > rc")
	dockerArgs := []string{
		"run", "--rm",
		"-v", absTmp + ":/work",
		"-w", "/work",
		"--network", "none",
		"--memory", fmt.Sprintf("%dm", cfg.MemLimit*2),
		"--pids-limit", "64",
		"--cpu-shares", "128",
		"gcc-with-time",
		"bash", "-lc", shellCmd,
	}

	// Run the container detached and enforce the time limit ourselves. This is the
	// fix for the memory leak: previously exec.CommandContext killed the docker
	// CLIENT on timeout, but the container kept running inside the daemon and was
	// never removed (--rm only removes a container when *it* exits). Now the
	// container is always forcibly removed on timeout, so it cannot leak memory.
	timeout := time.Duration(int(cfg.TimeLimit*2)+5) * time.Second
	timedOut, runErr := runContainerDetached(dockerArgs, timeout)
	if runErr != nil {
		result.Info = fmt.Sprintf("Failed to run docker: %v", runErr)
		result.Status = "runtime_error"
		return result
	}

	if timedOut {
		err = fmt.Errorf("killed")
	}

	// Parse time and memory from time.log
	parseTimeLog := func(path string) (timeMs int, memKB int) {
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, 0
		}
		text := string(data)
		memRe := regexp.MustCompile(`Maximum resident set size \(kbytes\):\s*(\d+)`)
		if m := memRe.FindStringSubmatch(text); len(m) >= 2 {
			if v, err := strconv.Atoi(m[1]); err == nil {
				memKB = v
			}
		}
		userRe := regexp.MustCompile(`User time \(seconds\):\s*([0-9.]+)`)
		var userF float64
		if m := userRe.FindStringSubmatch(text); len(m) >= 2 {
			if f, err := strconv.ParseFloat(m[1], 64); err == nil {
				userF = f
			}
		}
		timeMs = int((userF) * 1000.0)
		return timeMs, memKB
	}

	// Check for errors
	if err != nil {
		fmt.Printf("error : %v\n", err.Error())
		// Read return code
		rc := -1
		if b, e := os.ReadFile(filepath.Join(absTmp, "rc")); e == nil {
			if v, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				rc = v
				fmt.Printf("Return code: %d\n", rc)
			}
		}
		stderrBytes, _ := os.ReadFile(filepath.Join(absTmp, "runtime.err"))
		stderr := string(stderrBytes)

		if strings.Contains(err.Error(), "killed") || rc == 124 {
			result.Status = "time_limit_exceeded"
			result.Info = "Time limit exceeded"
			tms, _ := parseTimeLog(filepath.Join(absTmp, "time.log"))
			result.RunTimeMs = tms
			return result
		} else if rc == 137 {
			result.Status = "memory_limit_exceeded"
			result.Info = "Memory limit exceeded"
			_, memKB := parseTimeLog(filepath.Join(absTmp, "time.log"))
			result.MemoryKB = memKB
			return result
		} else {
			result.Status = "runtime_error"
			result.Info = stderr
			// Also capture any output that was produced
			if ob, oe := os.ReadFile(filepath.Join(absTmp, "out.out")); oe == nil && len(ob) > 0 {
				result.Info += "\nProgram output:\n" + string(ob)
			}
			tms, memKB := parseTimeLog(filepath.Join(absTmp, "time.log"))
			result.RunTimeMs = tms
			result.MemoryKB = memKB
			return result
		}
	}

	result.RunTimeMs, result.MemoryKB = parseTimeLog(filepath.Join(absTmp, "time.log"))

	if result.RunTimeMs > int(cfg.TimeLimit*1000) {
		result.Status = "time_limit_exceeded"
		result.Info = "Time limit exceeded"
		return result
	}

	if result.MemoryKB > cfg.MemLimit*1024 {
		result.Status = "memory_limit_exceeded"
		result.Info = "Memory limit exceeded"
		return result
	}

	// Success - read output and compare with expected
	gotBytes, _ := os.ReadFile(filepath.Join(absTmp, "out.out"))
	expectedBytes, _ := os.ReadFile(cfg.ExpectedPath)

	// Normalize: convert \r\n to \n and trim trailing whitespace
	normalize := func(b []byte) string {
		s := string(b)
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.TrimRight(s, " \t\n\r")
		return s
	}

	got := normalize(gotBytes)
	expected := normalize(expectedBytes)

	// Parse time and memory

	posGot := 0
	posExpected := 0

	lineNum := 1
	columnNum := 1

	for {
		if posGot >= len(got) && posExpected >= len(expected) {
			// Both ended, no mismatch found
			result.Passed = true
			result.Status = "accepted"
			result.Info = "Accepted"
			break
		}

		if posExpected >= len(expected) {
			if got[posGot] == '\n' {
				posGot++
				continue
			}
		}

		if posGot >= len(got) {
			if expected[posExpected] == '\n' {
				posExpected++
				continue
			}
		}

		if posGot >= len(got) || posExpected >= len(expected) {
			result.Passed = false
			result.Status = "wrong_answer"
			if posGot >= len(got) {
				result.Info = fmt.Sprintf("Output ended early in line %d, column %d, expected '%c'", lineNum, columnNum, expected[posExpected])
			} else {
				result.Info = fmt.Sprintf("Output has extra character '%c' in line %d, column %d", got[posGot], lineNum, columnNum)
			}
			break
		}

		if got[posGot] == '\n' && expected[posExpected] == ' ' {
			posExpected++
		} else if got[posGot] == ' ' && expected[posExpected] == '\n' {
			posGot++
		}

		if got[posGot] != expected[posExpected] {
			result.Passed = false
			result.Status = "wrong_answer"
			result.Info = fmt.Sprintf("Mismatch at line %d, column %d: got '%c', expected '%c'", lineNum, columnNum, got[posGot], expected[posExpected])
			break
		}

		if got[posGot] == '\n' {
			lineNum++
			columnNum = 1
		} else {
			columnNum++
		}

		posGot++
		posExpected++
	}

	return result
}

// runContainerDetached starts a Docker container in detached mode, waits for it to
// finish (up to timeout), and guarantees the container is removed.
//
// Why detached instead of exec.CommandContext with "docker run --rm": when the Go
// context deadline fires, exec.CommandContext sends SIGKILL to the docker CLI. That
// kills the client but the container keeps running inside the Docker daemon (the
// workload has no internal timeout), and because --rm only removes a container when
// *it* exits, a hung container is never removed. Over many submissions (e.g. TLE /
// infinite-loop solutions) these orphaned containers accumulate and leak memory in
// the Docker daemon. Running detached and always "docker rm -f" at the end ensures
// every container is cleaned up, so the daemon cannot leak containers/memory.
func runContainerDetached(dockerArgs []string, timeout time.Duration) (timedOut bool, err error) {
	// Build "docker run -d ..." so we obtain the container ID from stdout and can
	// forcibly remove it later if needed.
	startArgs := make([]string, 0, len(dockerArgs)+1)
	if len(dockerArgs) > 0 && dockerArgs[0] == "run" {
		startArgs = append(startArgs, "run", "-d")
		startArgs = append(startArgs, dockerArgs[1:]...)
	} else {
		startArgs = append(startArgs, "-d")
		startArgs = append(startArgs, dockerArgs...)
	}

	startCmd := exec.Command("docker", startArgs...)
	idOut, startErr := startCmd.Output()
	if startErr != nil {
		return false, fmt.Errorf("docker run failed: %w", startErr)
	}
	containerID := strings.TrimSpace(string(idOut))
	if containerID == "" {
		return false, fmt.Errorf("docker run returned empty container id")
	}

	// Always attempt to remove the container. On normal exit --rm already removed it
	// (so rm -f is a harmless no-op); on timeout or error this guarantees cleanup so
	// the Docker daemon never leaks the container and its memory.
	defer func() {
		if rmErr := exec.Command("docker", "rm", "-f", containerID).Run(); rmErr != nil {
			log.Printf("warning: failed to remove container %s: %v", containerID, rmErr)
		}
	}()

	waitCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	waitCmd := exec.CommandContext(waitCtx, "docker", "wait", containerID)
	var waitOut bytes.Buffer
	waitCmd.Stdout = &waitOut
	waitCmd.Stderr = &waitOut
	if werr := waitCmd.Run(); werr != nil {
		if waitCtx.Err() == context.DeadlineExceeded {
			return true, nil
		}
		return false, fmt.Errorf("docker wait failed: %w", werr)
	}
	return false, nil
}

// StartJudge starts the judge loop as a goroutine. It polls the DB for queued submissions.
func StartJudge() {
	go func() {
		// ensure required docker images are present to avoid long pulls during processing
		// ensureDockerImage("gcc-with-time")
		for {
			sub, err := sql_service.PopQueuedSubmission()
			if err != nil {
				// no job or DB error; sleep briefly
				time.Sleep(time.Second)
				continue
			}
			processJob(sub)
		}
	}()
}

// ensureDockerImage pulls the given image (with timeout) so compile/run won't block on pulls
// func ensureDockerImage(image string) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
// 	defer cancel()
// 	cmd := exec.CommandContext(ctx, "docker", "pull", image)
// 	var out bytes.Buffer
// 	cmd.Stdout = &out
// 	cmd.Stderr = &out
// 	if err := cmd.Run(); err != nil {
// 		log.Printf("docker pull %s failed: %v output=%s", image, err, out.String())
// 	} else {
// 		log.Printf("docker image %s available", image)
// 	}
// }

func appendMessage(line string) {
	_ = os.MkdirAll("data", 0755)
	f, err := os.OpenFile("data/message.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("append message failed: %v", err)
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

// runJudge compiles and runs all test cases for a submission and returns the
// resulting status and per-test results. It does NOT write to the database, so it
// can be reused by both the local judge loop and distributed workers.
func runJudge(sub sql_service.Submission) (status string, results []sql_service.TestResult) {
	// create temp working dir under repository root ./tmp (ensure base exists)
	tmpBase := "./tmp"
	if err := os.MkdirAll(tmpBase, 0755); err != nil {
		log.Printf("failed to create tmp base dir %s: %v", tmpBase, err)
	}
	// ensure base has world-readable/executable so tools like `go build` won't fail when tmp subdirs exist
	_ = os.Chmod(tmpBase, 0755)
	tmpDir, err := os.MkdirTemp(tmpBase, fmt.Sprintf("sub-%d-", sub.ID))
	if err != nil {
		// fallback to system temp
		log.Printf("failed to create tmp in %s: %v, falling back to system temp", tmpBase, err)
		tmpDir, err = os.MkdirTemp("", fmt.Sprintf("sub-%d-", sub.ID))
		if err != nil {
			log.Printf("failed to create system tmp dir: %v", err)
			return "internal_error", nil
		}
	}
	// try to make tmpDir world-readable/executable so other processes can inspect
	_ = os.Chmod(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// write code file
	codePath := filepath.Join(tmpDir, "solution.cpp")

	if err := os.WriteFile(codePath, []byte(sub.Code), 0644); err != nil {
		log.Printf("failed to write code file: %v", err)
		return "internal_error", nil
	}

	// verify file actually exists and is writable (some environments may hide errors)
	// if fi, err := os.Stat(codePath); err != nil {
	// 	log.Printf("code file stat failed after write: %v", err)
	// 	// fallback: try explicit open/create and write
	// 	f, ferr := os.OpenFile(codePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	// 	if ferr != nil {
	// 		log.Printf("fallback open failed for %s: %v", codePath, ferr)
	// 		_ = sql_service.UpdateSubmissionResult(sub.ID, "internal_error", nil)
	// 		appendMessage(fmt.Sprintf("%s submitted %s => INTERNAL_ERROR (write-fallback)", sub.Username, sub.Problem))
	// 		return
	// 	}
	// 	if _, werr := f.Write([]byte(sub.Code)); werr != nil {
	// 		log.Printf("fallback write failed for %s: %v", codePath, werr)
	// 		f.Close()
	// 		_ = sql_service.UpdateSubmissionResult(sub.ID, "internal_error", nil)
	// 		appendMessage(fmt.Sprintf("%s submitted %s => INTERNAL_ERROR (write-fallback2)", sub.Username, sub.Problem))
	// 		return
	// 	}
	// 	f.Close()
	// 	if fi2, err2 := os.Stat(codePath); err2 == nil {
	// 		log.Printf("code file created by fallback: %s size=%d mode=%v", codePath, fi2.Size(), fi2.Mode())
	// 	} else {
	// 		log.Printf("code file still missing after fallback: %v", err2)
	// 	}
	// } else {
	// 	log.Printf("code file created: %s size=%d mode=%v", codePath, fi.Size(), fi.Mode())
	// }

	// read problem config from disk
	cfgPath := filepath.Join("data", "problem", fmt.Sprintf("%d", sub.ProblemID), "config.json")
	cfgData, _ := os.ReadFile(cfgPath)

	timeLimit := 1.0
	memMB := 256

	if len(cfgData) > 0 {
		var obj map[string]any
		if err := json.Unmarshal(cfgData, &obj); err == nil {
			// tests: accept tests, tests_count, TestsCount
			// if v, ok := obj["tests"].(float64); ok {
			// 	tests = int(v)
			// } else if v, ok := obj["tests_count"].(float64); ok {
			// 	tests = int(v)
			// }
			// time limit: accept time_limit (milliseconds, common in Chinese OI problems)
			// or time_limit_s/time_limit (seconds)
			if v, ok := obj["time_limit"].(float64); ok {
				// Most config files use milliseconds (1000 = 1 second)
				// If value > 100, assume it's in milliseconds
				if v > 100 {
					timeLimit = v / 1000.0
				} else {
					timeLimit = v
				}
			} else if v, ok := obj["time_limit_ms"].(float64); ok {
				timeLimit = v / 1000.0
			} else if v, ok := obj["time_limit_s"].(float64); ok {
				timeLimit = v
			}
			// memory: accept mem_mb or mem_limit_mb
			if v, ok := obj["memory_limit"].(float64); ok {
				memMB = int(v)
			} else if v, ok := obj["memory_limit_mb"].(float64); ok {
				memMB = int(v)
			}
		}
	}

	results = []sql_service.TestResult{}
	status = "ok"

	// compile inside docker
	// use absolute paths to avoid stray files
	absTmp, _ := filepath.Abs(tmpDir)
	// use absolute g++ path to avoid PATH issues inside image.
	// Redirect compiler output and exit code to files in the work dir so they
	// survive the detached run (a detached container's stdio is not captured).
	compileCmd := "g++ solution.cpp -O2 -std=c++17 -o solution > compile.err 2>&1; echo $? > compile.rc"
	// compilation can require significantly more memory than runtime limits; raise compile memory cap
	compileMem := 512
	dockerCompileArgs := []string{"run", "--rm", "-v", absTmp + ":/work", "-w", "/work", "--network", "none", "--memory", fmt.Sprintf("%dm", compileMem), "--cpus", "1.0", "gcc-with-time", "bash", "-lc", fmt.Sprintf("%v", compileCmd)}
	// increase compile timeout to allow for image/pulled layers and heavier builds;
	// run detached so a timeout cannot leak the container (see runContainerDetached).
	compileTimedOut, compileErr := runContainerDetached(dockerCompileArgs, 10*time.Second)
	if compileErr != nil || compileTimedOut {
		status = "compile_error"
		var outStr string
		if compileTimedOut {
			outStr = "compilation timed out"
		} else {
			outStr = fmt.Sprintf("compilation failed to start: %v", compileErr)
		}
		if eb, e := os.ReadFile(filepath.Join(absTmp, "compile.err")); e == nil {
			outStr += "\n" + string(eb)
		}
		results = append(results, sql_service.TestResult{TestIndex: 0, Passed: false, Output: outStr, TimeMs: 0, MemoryKB: 0})
		return status, results
	}

	// read compile result written by the container
	rcBytes, _ := os.ReadFile(filepath.Join(absTmp, "compile.rc"))
	compileRC, _ := strconv.Atoi(strings.TrimSpace(string(rcBytes)))
	if compileRC != 0 {
		status = "compile_error"
		eb, _ := os.ReadFile(filepath.Join(absTmp, "compile.err"))
		outStr := string(eb)
		results = append(results, sql_service.TestResult{TestIndex: 0, Passed: false, Output: outStr, TimeMs: 0, MemoryKB: 0})
		return status, results
	}

	// run tests sequentially using JudgeTest

	obj := make(map[string]any)
	_ = json.Unmarshal(cfgData, &obj)
	testGroups := []interface{}{}
	if v, ok := obj["test_cases"].([]interface{}); ok {
		testGroups = v
	} else {
		// return error if test groups not found; we require test groups to determine how many tests to run
		return "internal_error", nil
	}

	allPassed := true

	for _, testGroup := range testGroups {
		testGroupMap, ok := testGroup.(map[string]any)
		if !ok {
			// skip invalid test group
			continue
		}

		// support both "cases" to specify the test indices in this group
		tests := []int{}
		if v, ok := testGroupMap["cases"].([]interface{}); ok {
			for _, caseVal := range v {
				if num, ok := caseVal.(float64); ok {
					tests = append(tests, int(num))
				}
			}
		} else {
			continue
		}

		groupPassed := true

		for _, i := range tests {
			if !groupPassed {
				// skip remaining tests in this group if one already failed
				results = append(results, sql_service.TestResult{
					TestIndex: i,
					Passed:    false,
					Output:    "Skipped due to previous failure in group",
					TimeMs:    0,
					MemoryKB:  0,
					Status:    "skipped",
				})
				continue
			}

			inPath := filepath.Join("data", "problem", fmt.Sprintf("%d", sub.ProblemID), fmt.Sprintf("%d.in", i))
			expectedPath := filepath.Join("data", "problem", fmt.Sprintf("%d", sub.ProblemID), fmt.Sprintf("%d.ans", i))

			// Prepare configuration for this test
			cfg := JudgeConfig{
				TimeLimit:    timeLimit,
				MemLimit:     memMB,
				InputPath:    inPath,
				WorkTmpPath:  tmpDir,
				ExpectedPath: expectedPath,
			}

			// Run the test using the encapsulated function
			testResult := JudgeTest(cfg)

			// Convert JudgeResult to TestResult
			testIdx := i
			testPassed := testResult.Passed
			testOutput := testResult.Info // for WA, Info contains the mismatch details; for RE, it contains the error message
			testTimeMs := testResult.RunTimeMs
			testMemKB := testResult.MemoryKB
			testStatus := testResult.Status

			// Store test result
			results = append(results, sql_service.TestResult{
				TestIndex: testIdx,
				Passed:    testPassed,
				Output:    testOutput,
				// Expected:  testExpected,
				TimeMs:   testTimeMs,
				MemoryKB: testMemKB,
				Status:   testStatus,
				Score:    0, // scoring can be implemented later based on test groups or other criteria
			})

			// Handle different statuses
			if !testPassed {
				groupPassed = false
				allPassed = false
			}
		}

		if groupPassed {
			// if all tests in this group passed, assign the group score to the last test
			results[len(results)-1].Score = int(testGroupMap["score"].(float64))
		}
	}

	// all passed
	if allPassed {
		status = "accepted"
	} else {
		status = "not accepted"
	}

	return status, results
}

// processJob runs the judge and writes the result to the database. Used by the
// local judge loop and by the coordinator's embedded judge.
func processJob(sub sql_service.Submission) {
	status, results := runJudge(sub)
	if status == "" {
		status = "internal_error"
	}
	_ = sql_service.UpdateSubmissionResult(sub.ID, status, results)
	appendMessage(fmt.Sprintf("%s submitted %d => %s", sub.Username, sub.ProblemID, status))
}
