package runner

import (
	"os"
	"testing"
	"time"
)

func TestParseGoTestOutput(t *testing.T) {
	output := `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestSubtract
    math_test.go:15: expected 3 but got 2
--- FAIL: TestSubtract (0.01s)
=== RUN   TestSkipped
--- SKIP: TestSkipped (0.00s)
FAIL
exit status 1`

	entries := ParseTestOutput("go", output)
	if len(entries) != 3 {
		t.Fatalf("expected 3 test entries, got %d", len(entries))
	}

	if entries[0].Name != "TestAdd" || entries[0].Status != "pass" {
		t.Errorf("entry 0: got %s/%s, want TestAdd/pass", entries[0].Name, entries[0].Status)
	}
	if entries[1].Name != "TestSubtract" || entries[1].Status != "fail" {
		t.Errorf("entry 1: got %s/%s, want TestSubtract/fail", entries[1].Name, entries[1].Status)
	}
	if entries[1].File != "math_test.go" || entries[1].Line != 15 {
		t.Errorf("entry 1 location: got %s:%d, want math_test.go:15", entries[1].File, entries[1].Line)
	}
	if entries[2].Name != "TestSkipped" || entries[2].Status != "skip" {
		t.Errorf("entry 2: got %s/%s, want TestSkipped/skip", entries[2].Name, entries[2].Status)
	}
}

func TestParseGoTestDuration(t *testing.T) {
	output := `--- PASS: TestFoo (1.50s)`
	entries := ParseTestOutput("go", output)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	expected := 1500 * time.Millisecond
	if entries[0].Duration != expected {
		t.Errorf("duration: got %v, want %v", entries[0].Duration, expected)
	}
}

func TestParsePytestOutput(t *testing.T) {
	output := `test_math.py::test_add PASSED
test_math.py::test_subtract FAILED
test_math.py::test_skip SKIPPED`

	entries := ParseTestOutput("pytest", output)
	if len(entries) != 3 {
		t.Fatalf("expected 3 test entries, got %d", len(entries))
	}

	if entries[0].Name != "test_add" || entries[0].Status != "pass" {
		t.Errorf("entry 0: got %s/%s, want test_add/pass", entries[0].Name, entries[0].Status)
	}
	if entries[1].Name != "test_subtract" || entries[1].Status != "fail" {
		t.Errorf("entry 1: got %s/%s, want test_subtract/fail", entries[1].Name, entries[1].Status)
	}
	if entries[2].Name != "test_skip" || entries[2].Status != "skip" {
		t.Errorf("entry 2: got %s/%s, want test_skip/skip", entries[2].Name, entries[2].Status)
	}
}

func TestParseCargoTestOutput(t *testing.T) {
	output := `running 3 tests
test tests::test_add ... ok
test tests::test_subtract ... FAILED
test tests::test_skip ... ignored

test result: FAILED. 1 passed; 1 failed; 1 ignored; 0 measured; 0 filtered out`

	entries := ParseTestOutput("cargo", output)
	if len(entries) != 3 {
		t.Fatalf("expected 3 test entries, got %d", len(entries))
	}

	if entries[0].Name != "tests::test_add" || entries[0].Status != "pass" {
		t.Errorf("entry 0: got %s/%s, want tests::test_add/pass", entries[0].Name, entries[0].Status)
	}
	if entries[1].Name != "tests::test_subtract" || entries[1].Status != "fail" {
		t.Errorf("entry 1: got %s/%s, want tests::test_subtract/fail", entries[1].Name, entries[1].Status)
	}
	if entries[2].Name != "tests::test_skip" || entries[2].Status != "skip" {
		t.Errorf("entry 2: got %s/%s, want tests::test_skip/skip", entries[2].Name, entries[2].Status)
	}
}

func TestTestSummary(t *testing.T) {
	tests := []TestEntry{
		{Name: "A", Status: "pass"},
		{Name: "B", Status: "pass"},
		{Name: "C", Status: "fail"},
		{Name: "D", Status: "skip"},
	}
	s := summarizeTests(tests)
	if s.Passed != 2 || s.Failed != 1 || s.Skipped != 1 || s.Total != 4 {
		t.Errorf("summary: got %+v", s)
	}
}

func TestFormatTestSummary(t *testing.T) {
	s := TestSummary{Passed: 5, Failed: 2, Skipped: 1, Total: 8}
	result := FormatTestSummary(s)
	if result != "5 passed, 2 failed, 1 skipped" {
		t.Errorf("got %q", result)
	}
}

func TestFormatTestSummaryEmpty(t *testing.T) {
	s := TestSummary{}
	result := FormatTestSummary(s)
	if result != "No tests found" {
		t.Errorf("got %q", result)
	}
}

func TestFormatTestSummaryAllPass(t *testing.T) {
	s := TestSummary{Passed: 3, Total: 3}
	result := FormatTestSummary(s)
	if result != "3 passed" {
		t.Errorf("got %q", result)
	}
}

func TestDetectTestCommandGo(t *testing.T) {
	// Create temp dir with go.mod
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module test\n")

	cmd, args := DetectTestCommand(dir, "")
	if cmd != "go" {
		t.Errorf("expected go, got %s", cmd)
	}
	if len(args) < 2 || args[0] != "test" {
		t.Errorf("expected test args, got %v", args)
	}
}

func TestDetectTestCommandRust(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"test\"\n")

	cmd, args := DetectTestCommand(dir, "")
	if cmd != "cargo" {
		t.Errorf("expected cargo, got %s", cmd)
	}
	if len(args) < 1 || args[0] != "test" {
		t.Errorf("expected test arg, got %v", args)
	}
}

func TestDetectTestCommandNode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", "{}")

	cmd, args := DetectTestCommand(dir, "")
	if cmd != "npm" {
		t.Errorf("expected npm, got %s", cmd)
	}
	if len(args) < 1 || args[0] != "test" {
		t.Errorf("expected test arg, got %v", args)
	}
}

func TestDetectTestCommandCustom(t *testing.T) {
	cmd, args := DetectTestCommand("", "make test-all")
	if cmd != "make" {
		t.Errorf("expected make, got %s", cmd)
	}
	if len(args) != 1 || args[0] != "test-all" {
		t.Errorf("expected test-all arg, got %v", args)
	}
}

func TestDetectTestCommandForFileGo(t *testing.T) {
	cmd, args := DetectTestCommandForFile("main.go", "/tmp", "")
	if cmd != "go" {
		t.Errorf("expected go, got %s", cmd)
	}
	if len(args) < 2 || args[0] != "test" {
		t.Errorf("expected test args, got %v", args)
	}
}

func TestDetectTestCommandForFileRust(t *testing.T) {
	cmd, args := DetectTestCommandForFile("main.rs", "/tmp", "")
	if cmd != "cargo" {
		t.Errorf("expected cargo, got %s", cmd)
	}
	if len(args) < 1 || args[0] != "test" {
		t.Errorf("expected test arg, got %v", args)
	}
}

func TestColorizeTestOutput(t *testing.T) {
	input := "--- PASS: TestFoo (0.00s)\n--- FAIL: TestBar (0.01s)"
	result := ColorizeTestOutput(input)
	if result == input {
		t.Error("expected color tags to be applied")
	}
	if len(result) <= len(input) {
		t.Error("colored output should be longer than input")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
