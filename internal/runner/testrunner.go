package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TestResult holds the outcome of a test run.
type TestResult struct {
	Command  string
	Output   string
	Error    string
	ExitCode int
	Duration time.Duration
	Tests    []TestEntry
	Summary  TestSummary
}

// TestEntry represents a single parsed test result.
type TestEntry struct {
	Name     string
	Status   string // "pass", "fail", "skip"
	Duration time.Duration
	File     string // file path (if known)
	Line     int    // line number (if known)
	Message  string // failure message (if any)
}

// TestSummary holds aggregate counts.
type TestSummary struct {
	Passed  int
	Failed  int
	Skipped int
	Total   int
}

// TestRunner runs unit tests and parses results.
type TestRunner struct {
	cancelFunc context.CancelFunc
	running    bool
}

func NewTestRunner() *TestRunner {
	return &TestRunner{}
}

func (tr *TestRunner) IsRunning() bool {
	return tr.running
}

func (tr *TestRunner) Stop() {
	if tr.cancelFunc != nil {
		tr.cancelFunc()
	}
}

// DetectTestCommand returns the test command and args for a project directory.
// It inspects marker files to determine the framework.
// customCmd is checked first; if non-empty it is used directly.
func DetectTestCommand(dir string, customCmd string) (string, []string) {
	if customCmd != "" {
		parts := strings.Fields(customCmd)
		if len(parts) == 0 {
			return "", nil
		}
		return parts[0], parts[1:]
	}

	// Go: look for go.mod
	if fileExists(filepath.Join(dir, "go.mod")) {
		return "go", []string{"test", "-v", "./..."}
	}

	// Rust: look for Cargo.toml
	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		return "cargo", []string{"test"}
	}

	// Node.js: look for package.json
	if fileExists(filepath.Join(dir, "package.json")) {
		return "npm", []string{"test"}
	}

	// Maven project
	if fileExists(filepath.Join(dir, "pom.xml")) {
		return "mvn", []string{"test"}
	}

	// Gradle project (Kotlin DSL or Groovy DSL)
	if fileExists(filepath.Join(dir, "build.gradle.kts")) || fileExists(filepath.Join(dir, "build.gradle")) {
		return "gradle", []string{"test"}
	}

	// Python: check for conftest.py (pytest) or fallback to unittest
	if fileExists(filepath.Join(dir, "conftest.py")) {
		return "pytest", []string{"-v"}
	}
	// Check for common Python project markers
	if fileExists(filepath.Join(dir, "setup.py")) || fileExists(filepath.Join(dir, "pyproject.toml")) {
		// Try pytest first, fall back to unittest
		if _, err := exec.LookPath("pytest"); err == nil {
			return "pytest", []string{"-v"}
		}
		return "python", []string{"-m", "unittest", "discover"}
	}

	return "", nil
}

// DetectTestCommandForFile returns test command based on the file's language.
func DetectTestCommandForFile(filePath, dir, customCmd string) (string, []string) {
	if customCmd != "" {
		parts := strings.Fields(customCmd)
		if len(parts) == 0 {
			return "", nil
		}
		return parts[0], parts[1:]
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go", []string{"test", "-v", "./..."}
	case ".rs":
		return "cargo", []string{"test"}
	case ".py":
		if fileExists(filepath.Join(dir, "conftest.py")) {
			return "pytest", []string{"-v"}
		}
		if _, err := exec.LookPath("pytest"); err == nil {
			return "pytest", []string{"-v"}
		}
		return "python", []string{"-m", "unittest", "discover"}
	case ".js", ".jsx", ".ts", ".tsx":
		return "npm", []string{"test"}
	case ".java":
		// Gradle project (check both Kotlin DSL and Groovy DSL)
		if fileExists(filepath.Join(dir, "build.gradle.kts")) || fileExists(filepath.Join(dir, "build.gradle")) {
			return "gradle", []string{"test"}
		}
		// Maven project
		return "mvn", []string{"test"}
	case ".kt", ".kts":
		// Gradle project (preferred for Kotlin)
		if fileExists(filepath.Join(dir, "build.gradle.kts")) || fileExists(filepath.Join(dir, "build.gradle")) {
			return "gradle", []string{"test"}
		}
		// Maven fallback
		if fileExists(filepath.Join(dir, "pom.xml")) {
			return "mvn", []string{"test"}
		}
		return "gradle", []string{"test"}
	default:
		// Fall back to directory-based detection
		return DetectTestCommand(dir, "")
	}
}

// RunTests executes the test command and returns parsed results.
func (tr *TestRunner) RunTests(dir string, cmdName string, args []string) *TestResult {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	tr.cancelFunc = cancel
	tr.running = true
	defer func() {
		tr.running = false
		cancel()
	}()

	fullCmd := cmdName
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &TestResult{
		Command:  fullCmd,
		Output:   string(output),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = string(output)
		} else {
			result.Error = err.Error()
			result.ExitCode = -1
		}
	}

	// Parse test output based on command
	result.Tests = ParseTestOutput(cmdName, result.Output)
	result.Summary = summarizeTests(result.Tests)

	return result
}

func summarizeTests(tests []TestEntry) TestSummary {
	s := TestSummary{Total: len(tests)}
	for _, t := range tests {
		switch t.Status {
		case "pass":
			s.Passed++
		case "fail":
			s.Failed++
		case "skip":
			s.Skipped++
		}
	}
	return s
}

// ParseTestOutput parses test runner output into structured results.
func ParseTestOutput(cmdName string, output string) []TestEntry {
	switch {
	case cmdName == "go" || strings.HasSuffix(cmdName, "/go"):
		return parseGoTestOutput(output)
	case cmdName == "pytest" || strings.HasSuffix(cmdName, "/pytest"):
		return parsePytestOutput(output)
	case cmdName == "cargo" || strings.HasSuffix(cmdName, "/cargo"):
		return parseCargoTestOutput(output)
	case cmdName == "mvn" || strings.HasSuffix(cmdName, "/mvn"):
		return parseMavenTestOutput(output)
	case cmdName == "gradle" || strings.HasSuffix(cmdName, "/gradle") || cmdName == "gradlew" || strings.HasSuffix(cmdName, "/gradlew"):
		return parseGradleTestOutput(output)
	default:
		return parseGenericTestOutput(output)
	}
}

// go test -v output patterns:
// --- PASS: TestFoo (0.00s)
// --- FAIL: TestBar (0.05s)
// --- SKIP: TestBaz (0.00s)
var goTestResultRe = regexp.MustCompile(`^--- (PASS|FAIL|SKIP): (\S+) \((\d+\.\d+)s\)$`)

// go test output file location: filename_test.go:42: error message
var goTestFileRe = regexp.MustCompile(`^\s+(\S+_test\.go):(\d+):\s+(.+)$`)

func parseGoTestOutput(output string) []TestEntry {
	lines := strings.Split(output, "\n")
	var entries []TestEntry

	// Collect file references that appear between test entries
	var pendingFile string
	var pendingLine int
	var pendingMsg string

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if m := goTestFileRe.FindStringSubmatch(line); m != nil {
			pendingFile = m[1]
			pendingLine, _ = strconv.Atoi(m[2])
			pendingMsg = m[3]
			continue
		}

		if m := goTestResultRe.FindStringSubmatch(line); m != nil {
			status := strings.ToLower(m[1])
			dur, _ := strconv.ParseFloat(m[3], 64)
			entry := TestEntry{
				Name:     m[2],
				Status:   status,
				Duration: time.Duration(dur * float64(time.Second)),
			}
			if status == "fail" && pendingFile != "" {
				entry.File = pendingFile
				entry.Line = pendingLine
				entry.Message = pendingMsg
			}
			entries = append(entries, entry)
			pendingFile = ""
			pendingLine = 0
			pendingMsg = ""
		}
	}
	return entries
}

// pytest -v output patterns:
// test_foo.py::test_something PASSED
// test_foo.py::test_other FAILED
// test_foo.py::test_skip SKIPPED
var pytestResultRe = regexp.MustCompile(`^(\S+)::(\S+)\s+(PASSED|FAILED|SKIPPED|ERROR)`)

// pytest failure location: filename.py:42: AssertionError
var pytestFailLocRe = regexp.MustCompile(`^(\S+\.py):(\d+):\s+(.+)$`)

func parsePytestOutput(output string) []TestEntry {
	lines := strings.Split(output, "\n")
	var entries []TestEntry

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if m := pytestResultRe.FindStringSubmatch(line); m != nil {
			status := "pass"
			switch m[3] {
			case "FAILED", "ERROR":
				status = "fail"
			case "SKIPPED":
				status = "skip"
			}
			entries = append(entries, TestEntry{
				Name:   m[2],
				Status: status,
				File:   m[1],
			})
		}
	}

	// Second pass: look for failure locations
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if m := pytestFailLocRe.FindStringSubmatch(line); m != nil {
			lineNum, _ := strconv.Atoi(m[2])
			// Associate with the nearest failed test
			for j := range entries {
				if entries[j].Status == "fail" && entries[j].Line == 0 {
					filePart := strings.TrimSuffix(entries[j].File, filepath.Ext(entries[j].File))
					if strings.Contains(m[1], filePart) || i > 0 {
						entries[j].Line = lineNum
						entries[j].Message = m[3]
						break
					}
				}
			}
		}
	}

	return entries
}

// cargo test output patterns:
// test tests::test_foo ... ok
// test tests::test_bar ... FAILED
// test tests::test_baz ... ignored
var cargoTestResultRe = regexp.MustCompile(`^test (\S+) \.\.\. (ok|FAILED|ignored)$`)

func parseCargoTestOutput(output string) []TestEntry {
	lines := strings.Split(output, "\n")
	var entries []TestEntry

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		if m := cargoTestResultRe.FindStringSubmatch(line); m != nil {
			status := "pass"
			switch m[2] {
			case "FAILED":
				status = "fail"
			case "ignored":
				status = "skip"
			}
			entries = append(entries, TestEntry{
				Name:   m[1],
				Status: status,
			})
		}
	}
	return entries
}

// Maven individual test result (from Surefire verbose output):
// [INFO] Running com.example.AppTest
// [ERROR] Tests run: 1, Failures: 1, Errors: 0, Skipped: 0, Time elapsed: 0.01 s <<< FAILURE! - in com.example.AppTest
// [INFO] Tests run: 1, Failures: 0, Errors: 0, Skipped: 0, Time elapsed: 0.01 s - in com.example.AppTest
var mavenTestClassRe = regexp.MustCompile(`(?:INFO|ERROR)\]\s+Tests run:\s*(\d+),\s*Failures:\s*(\d+),\s*Errors:\s*(\d+),\s*Skipped:\s*(\d+).*?(?:in\s+(\S+))`)

// Maven individual test failure:
// [ERROR]   AppTest.testSomething:25 expected: <1> but was: <2>
var mavenTestFailRe = regexp.MustCompile(`\[ERROR\]\s+(\w+)\.(\w+):(\d+)\s+(.+)$`)

func parseMavenTestOutput(output string) []TestEntry {
	lines := strings.Split(output, "\n")
	var entries []TestEntry

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Parse individual test failures
		if m := mavenTestFailRe.FindStringSubmatch(line); m != nil {
			lineNum, _ := strconv.Atoi(m[3])
			entries = append(entries, TestEntry{
				Name:    m[1] + "." + m[2],
				Status:  "fail",
				Line:    lineNum,
				Message: m[4],
			})
			continue
		}

		// Parse test class results
		if m := mavenTestClassRe.FindStringSubmatch(line); m != nil {
			failures, _ := strconv.Atoi(m[2])
			errors, _ := strconv.Atoi(m[3])
			skipped, _ := strconv.Atoi(m[4])
			className := ""
			if len(m) > 5 {
				className = m[5]
			}
			// Only add a pass entry if there were no failures/errors already added for this class
			if failures == 0 && errors == 0 && className != "" {
				status := "pass"
				if skipped > 0 {
					status = "skip"
				}
				entries = append(entries, TestEntry{
					Name:   className,
					Status: status,
				})
			}
		}
	}
	return entries
}

// Gradle test output patterns:
// com.example.AppTest > testSomething PASSED
// com.example.AppTest > testOther FAILED
// com.example.AppTest > testSkip SKIPPED
var gradleTestResultRe = regexp.MustCompile(`^(\S+)\s+>\s+(\S+)\s+(PASSED|FAILED|SKIPPED)$`)

func parseGradleTestOutput(output string) []TestEntry {
	lines := strings.Split(output, "\n")
	var entries []TestEntry

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		// Strip leading whitespace for matching
		trimmed := strings.TrimSpace(line)

		if m := gradleTestResultRe.FindStringSubmatch(trimmed); m != nil {
			status := "pass"
			switch m[3] {
			case "FAILED":
				status = "fail"
			case "SKIPPED":
				status = "skip"
			}
			entries = append(entries, TestEntry{
				Name:   m[1] + "." + m[2],
				Status: status,
			})
		}
	}
	return entries
}

// Generic: look for common PASS/FAIL patterns
var genericPassRe = regexp.MustCompile(`(?i)(PASS|OK|SUCCESS)`)
var genericFailRe = regexp.MustCompile(`(?i)(FAIL|ERROR|FAILURE)`)

func parseGenericTestOutput(output string) []TestEntry {
	// For generic output, we don't try to parse individual tests
	return nil
}

// FormatTestSummary returns a human-readable summary line.
func FormatTestSummary(s TestSummary) string {
	parts := []string{}
	if s.Passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", s.Passed))
	}
	if s.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", s.Failed))
	}
	if s.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", s.Skipped))
	}
	if len(parts) == 0 {
		return "No tests found"
	}
	return strings.Join(parts, ", ")
}

// ColorizeTestOutput applies tview color tags to test output lines.
func ColorizeTestOutput(output string) string {
	lines := strings.Split(output, "\n")
	var result []string
	for _, line := range lines {
		colored := colorizeTestLine(line)
		result = append(result, colored)
	}
	return strings.Join(result, "\n")
}

func colorizeTestLine(line string) string {
	trimmed := strings.TrimSpace(line)

	// Go test pass/fail
	if strings.HasPrefix(trimmed, "--- PASS:") || strings.HasPrefix(trimmed, "ok ") {
		return "[green]" + line + "[-]"
	}
	if strings.HasPrefix(trimmed, "--- FAIL:") || strings.HasPrefix(trimmed, "FAIL") {
		return "[red]" + line + "[-]"
	}
	if strings.HasPrefix(trimmed, "--- SKIP:") {
		return "[yellow]" + line + "[-]"
	}

	// pytest
	if strings.HasSuffix(trimmed, "PASSED") {
		return "[green]" + line + "[-]"
	}
	if strings.HasSuffix(trimmed, "FAILED") || strings.HasSuffix(trimmed, "ERROR") {
		return "[red]" + line + "[-]"
	}
	if strings.HasSuffix(trimmed, "SKIPPED") {
		return "[yellow]" + line + "[-]"
	}

	// cargo test
	if strings.HasSuffix(trimmed, "... ok") {
		return "[green]" + line + "[-]"
	}
	if strings.HasSuffix(trimmed, "... FAILED") {
		return "[red]" + line + "[-]"
	}
	if strings.HasSuffix(trimmed, "... ignored") {
		return "[yellow]" + line + "[-]"
	}

	// Maven
	if strings.Contains(trimmed, "BUILD SUCCESS") {
		return "[green]" + line + "[-]"
	}
	if strings.Contains(trimmed, "BUILD FAILURE") {
		return "[red]" + line + "[-]"
	}
	if strings.HasPrefix(trimmed, "[ERROR]") {
		return "[red]" + line + "[-]"
	}
	if strings.HasPrefix(trimmed, "[WARNING]") {
		return "[yellow]" + line + "[-]"
	}

	// Gradle
	if strings.HasSuffix(trimmed, "PASSED") {
		return "[green]" + line + "[-]"
	}
	if strings.HasSuffix(trimmed, "FAILED") {
		return "[red]" + line + "[-]"
	}
	if strings.HasSuffix(trimmed, "SKIPPED") {
		return "[yellow]" + line + "[-]"
	}
	if strings.Contains(trimmed, "BUILD SUCCESSFUL") {
		return "[green]" + line + "[-]"
	}

	return line
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
