package runner

import (
	"testing"
)

func TestParseGCCErrors(t *testing.T) {
	output := `main.c:12:5: error: expected ';' before '}' token
main.c:20:3: warning: unused variable 'x'`
	errors := ParseBuildOutput(output)
	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}

	e := errors[0]
	if e.File != "main.c" {
		t.Errorf("file: got %q, want %q", e.File, "main.c")
	}
	if e.Line != 12 {
		t.Errorf("line: got %d, want 12", e.Line)
	}
	if e.Col != 5 {
		t.Errorf("col: got %d, want 5", e.Col)
	}
	if e.Severity != "error" {
		t.Errorf("severity: got %q, want %q", e.Severity, "error")
	}
	if e.Message != "expected ';' before '}' token" {
		t.Errorf("message: got %q", e.Message)
	}

	w := errors[1]
	if w.Severity != "warning" {
		t.Errorf("severity: got %q, want %q", w.Severity, "warning")
	}
	if w.Line != 20 {
		t.Errorf("line: got %d, want 20", w.Line)
	}
}

func TestParseGoErrors(t *testing.T) {
	output := `./main.go:42:10: undefined: foo
./main.go:50:2: syntax error: unexpected newline`
	errors := ParseBuildOutput(output)
	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}

	e := errors[0]
	if e.File != "./main.go" {
		t.Errorf("file: got %q, want %q", e.File, "./main.go")
	}
	if e.Line != 42 {
		t.Errorf("line: got %d, want 42", e.Line)
	}
	if e.Col != 10 {
		t.Errorf("col: got %d, want 10", e.Col)
	}
	if e.Message != "undefined: foo" {
		t.Errorf("message: got %q", e.Message)
	}
}

func TestParsePythonErrors(t *testing.T) {
	output := `Traceback (most recent call last):
  File "app.py", line 7, in <module>
    print(undefined_var)
NameError: name 'undefined_var' is not defined`
	errors := ParseBuildOutput(output)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}

	e := errors[0]
	if e.File != "app.py" {
		t.Errorf("file: got %q, want %q", e.File, "app.py")
	}
	if e.Line != 7 {
		t.Errorf("line: got %d, want 7", e.Line)
	}
	if e.Message != "NameError: name 'undefined_var' is not defined" {
		t.Errorf("message: got %q", e.Message)
	}
}

func TestParseRustErrors(t *testing.T) {
	output := `error[E0425]: cannot find value 'x' in this scope
 --> src/main.rs:3:5
  |
3 |     x
  |     ^ not found in this scope`
	errors := ParseBuildOutput(output)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}

	e := errors[0]
	if e.File != "src/main.rs" {
		t.Errorf("file: got %q, want %q", e.File, "src/main.rs")
	}
	if e.Line != 3 {
		t.Errorf("line: got %d, want 3", e.Line)
	}
	if e.Col != 5 {
		t.Errorf("col: got %d, want 5", e.Col)
	}
	if e.Severity != "error" {
		t.Errorf("severity: got %q, want %q", e.Severity, "error")
	}
	if e.Message != "cannot find value 'x' in this scope" {
		t.Errorf("message: got %q", e.Message)
	}
}

func TestParseJavacErrors(t *testing.T) {
	output := `Main.java:5: error: cannot find symbol
    System.out.println(missing);
                       ^
  symbol:   variable missing
  location: class Main`
	errors := ParseBuildOutput(output)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}

	e := errors[0]
	if e.File != "Main.java" {
		t.Errorf("file: got %q, want %q", e.File, "Main.java")
	}
	if e.Line != 5 {
		t.Errorf("line: got %d, want 5", e.Line)
	}
	if e.Severity != "error" {
		t.Errorf("severity: got %q, want %q", e.Severity, "error")
	}
	if e.Message != "cannot find symbol" {
		t.Errorf("message: got %q", e.Message)
	}
}

func TestParseTSCErrors(t *testing.T) {
	output := `src/app.ts(10,3): error TS2304: Cannot find name 'x'
src/app.ts(15,7): error TS2551: Property 'nmae' does not exist`
	errors := ParseBuildOutput(output)
	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}

	e := errors[0]
	if e.File != "src/app.ts" {
		t.Errorf("file: got %q, want %q", e.File, "src/app.ts")
	}
	if e.Line != 10 {
		t.Errorf("line: got %d, want 10", e.Line)
	}
	if e.Col != 3 {
		t.Errorf("col: got %d, want 3", e.Col)
	}
	if e.Message != "Cannot find name 'x'" {
		t.Errorf("message: got %q", e.Message)
	}
}

func TestParseNodeStackTrace(t *testing.T) {
	output := `/app.js:5:1
    throw new Error('test');
    ^
Error: test
    at Object.<anonymous> (/app.js:5:1)
    at Module._compile (node:internal/modules/cjs/loader:1241:14)`
	errors := ParseBuildOutput(output)
	// Should find at least the stack trace entry
	found := false
	for _, e := range errors {
		if e.File == "/app.js" && e.Line == 5 && e.Col == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find /app.js:5:1 in errors, got %+v", errors)
	}
}

func TestParseMultipleErrors(t *testing.T) {
	output := `main.c:10:1: error: expected declaration
main.c:15:3: error: use of undeclared identifier
main.c:20:7: warning: implicit conversion`
	errors := ParseBuildOutput(output)
	if len(errors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(errors))
	}
	// Verify order is preserved
	if errors[0].Line != 10 {
		t.Errorf("first error line: got %d, want 10", errors[0].Line)
	}
	if errors[1].Line != 15 {
		t.Errorf("second error line: got %d, want 15", errors[1].Line)
	}
	if errors[2].Line != 20 {
		t.Errorf("third error line: got %d, want 20", errors[2].Line)
	}
}

func TestParseEmptyOutput(t *testing.T) {
	errors := ParseBuildOutput("")
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestParseNoErrors(t *testing.T) {
	output := `Build successful
Compilation finished in 0.5s`
	errors := ParseBuildOutput(output)
	if len(errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errors))
	}
}

func TestParsePythonSyntaxError(t *testing.T) {
	output := `  File "test.py", line 3
    if True
          ^
SyntaxError: expected ':'`
	errors := ParseBuildOutput(output)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	e := errors[0]
	if e.File != "test.py" {
		t.Errorf("file: got %q, want %q", e.File, "test.py")
	}
	if e.Line != 3 {
		t.Errorf("line: got %d, want 3", e.Line)
	}
	if e.Message != "SyntaxError: expected ':'" {
		t.Errorf("message: got %q", e.Message)
	}
}

func TestParseKotlincErrors(t *testing.T) {
	output := `Main.kt:5:10: error: unresolved reference: foo
Main.kt:12:3: warning: parameter 'x' is never used`
	errors := ParseBuildOutput(output)
	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}

	e := errors[0]
	if e.File != "Main.kt" {
		t.Errorf("file: got %q, want %q", e.File, "Main.kt")
	}
	if e.Line != 5 {
		t.Errorf("line: got %d, want 5", e.Line)
	}
	if e.Col != 10 {
		t.Errorf("col: got %d, want 10", e.Col)
	}
	if e.Severity != "error" {
		t.Errorf("severity: got %q, want %q", e.Severity, "error")
	}
	if e.Message != "unresolved reference: foo" {
		t.Errorf("message: got %q", e.Message)
	}

	w := errors[1]
	if w.Severity != "warning" {
		t.Errorf("severity: got %q, want %q", w.Severity, "warning")
	}
	if w.Line != 12 {
		t.Errorf("line: got %d, want 12", w.Line)
	}
	if w.Col != 3 {
		t.Errorf("col: got %d, want 3", w.Col)
	}
}

func TestParseKotlincErrorsKts(t *testing.T) {
	output := `build.gradle.kts:10:5: error: unresolved reference: something`
	errors := ParseBuildOutput(output)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	e := errors[0]
	if e.File != "build.gradle.kts" {
		t.Errorf("file: got %q, want %q", e.File, "build.gradle.kts")
	}
	if e.Line != 10 {
		t.Errorf("line: got %d, want 10", e.Line)
	}
}

func TestParseKotlincErrorNoColumn(t *testing.T) {
	output := `script.kts:7: error: expecting member declaration`
	errors := ParseBuildOutput(output)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	e := errors[0]
	if e.File != "script.kts" {
		t.Errorf("file: got %q, want %q", e.File, "script.kts")
	}
	if e.Line != 7 {
		t.Errorf("line: got %d, want 7", e.Line)
	}
	if e.Col != 0 {
		t.Errorf("col: got %d, want 0", e.Col)
	}
}

func TestParseRustMultipleErrors(t *testing.T) {
	output := `error[E0425]: cannot find value 'x' in this scope
 --> src/main.rs:3:5
  |
3 |     x
  |     ^ not found in this scope

warning: unused variable 'y'
 --> src/main.rs:10:9
   |
10 |     let y = 5;
   |         ^ help: if this is intentional, prefix it with an underscore: '_y'`
	errors := ParseBuildOutput(output)
	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}
	if errors[0].Severity != "error" {
		t.Errorf("first severity: got %q, want %q", errors[0].Severity, "error")
	}
	if errors[1].Severity != "warning" {
		t.Errorf("second severity: got %q, want %q", errors[1].Severity, "warning")
	}
	if errors[1].Line != 10 {
		t.Errorf("second line: got %d, want 10", errors[1].Line)
	}
}
