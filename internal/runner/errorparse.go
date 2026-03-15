package runner

import (
	"regexp"
	"strconv"
	"strings"
)

// BuildError represents a single error extracted from build/run output.
type BuildError struct {
	File     string
	Line     int
	Col      int // 0 if unknown
	Severity string // "error", "warning", "note"
	Message  string
}

// errorPattern defines a regex pattern for extracting errors from compiler output.
type errorPattern struct {
	re       *regexp.Regexp
	file     int // capture group index for file path
	line     int // capture group index for line number
	col      int // capture group index for column (0 = not available)
	severity int // capture group index for severity (0 = not available)
	message  int // capture group index for message (0 = not available)
}

// Compiled patterns for each language's error format.
var errorPatterns = []errorPattern{
	// gcc/g++/clang: main.c:12:5: error: expected ';'
	{
		re:       regexp.MustCompile(`^([^\s:]+):(\d+):(\d+):\s+(error|warning|note):\s+(.+)$`),
		file:     1,
		line:     2,
		col:      3,
		severity: 4,
		message:  5,
	},
	// Go: ./main.go:42:10: undefined: foo  (handles ./relative, /absolute, and bare paths)
	{
		re:       regexp.MustCompile(`^([^\s:]+\.go):(\d+):(\d+):\s+(.+)$`),
		file:     1,
		line:     2,
		col:      3,
		severity: 0,
		message:  4,
	},
	// rustc: error[E0425]: ... --> src/main.rs:3:5
	{
		re:       regexp.MustCompile(`^\s*-->\s+([^\s:]+):(\d+):(\d+)$`),
		file:     1,
		line:     2,
		col:      3,
		severity: 0,
		message:  0,
	},
	// javac: Main.java:5: error: cannot find symbol
	{
		re:       regexp.MustCompile(`^([^\s:]+\.java):(\d+):\s+(error|warning):\s+(.+)$`),
		file:     1,
		line:     2,
		col:      0,
		severity: 3,
		message:  4,
	},
	// tsc: src/app.ts(10,3): error TS2304: Cannot find name 'x'
	{
		re:       regexp.MustCompile(`^([^\s(]+)\((\d+),(\d+)\):\s+(error|warning)\s+\w+:\s+(.+)$`),
		file:     1,
		line:     2,
		col:      3,
		severity: 4,
		message:  5,
	},
	// node stack trace: at Object.<anonymous> (/app.js:5:1)
	{
		re:       regexp.MustCompile(`\(([^()]+):(\d+):(\d+)\)$`),
		file:     1,
		line:     2,
		col:      3,
		severity: 0,
		message:  0,
	},
}

// Python error patterns are handled separately due to multi-line format.
var pythonFileRe = regexp.MustCompile(`^\s*File "([^"]+)", line (\d+)`)
var pythonErrorRe = regexp.MustCompile(`^(\w+Error|\w+Exception|SyntaxError|IndentationError|TabError):\s+(.+)$`)

// rustc severity line: error[E0425]: cannot find value `x` in this scope
var rustSeverityRe = regexp.MustCompile(`^(error|warning)(?:\[E\d+\])?:\s+(.+)$`)

// ParseBuildOutput parses compiler/interpreter output and returns a list of build errors.
func ParseBuildOutput(output string) []BuildError {
	lines := strings.Split(output, "\n")
	var errors []BuildError

	// Track rustc context: the most recent error/warning line provides the message
	// for the subsequent --> line.
	var rustMsg string
	var rustSeverity string

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		// Check for rustc severity line (provides context for --> pattern)
		if m := rustSeverityRe.FindStringSubmatch(line); m != nil {
			rustSeverity = m[1]
			rustMsg = m[2]
			continue
		}

		// Check Python multi-line pattern
		if m := pythonFileRe.FindStringSubmatch(line); m != nil {
			lineNum, _ := strconv.Atoi(m[2])
			be := BuildError{
				File:     m[1],
				Line:     lineNum,
				Severity: "error",
			}
			// Look ahead for the error message line (skip the source code line)
			for j := i + 1; j < len(lines) && j <= i+3; j++ {
				errLine := strings.TrimRight(lines[j], "\r")
				if pm := pythonErrorRe.FindStringSubmatch(errLine); pm != nil {
					be.Message = pm[1] + ": " + pm[2]
					break
				}
			}
			if be.Message == "" {
				be.Message = "error"
			}
			errors = append(errors, be)
			continue
		}

		// Try each pattern
		matched := false
		for _, pat := range errorPatterns {
			m := pat.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}

			be := BuildError{
				File: m[pat.file],
			}
			be.Line, _ = strconv.Atoi(m[pat.line])
			if pat.col > 0 && pat.col < len(m) {
				be.Col, _ = strconv.Atoi(m[pat.col])
			}
			if pat.severity > 0 && pat.severity < len(m) {
				be.Severity = m[pat.severity]
			}
			if pat.message > 0 && pat.message < len(m) {
				be.Message = m[pat.message]
			}

			// For rustc --> lines, use the previously captured message/severity
			if be.Message == "" && rustMsg != "" {
				be.Message = rustMsg
				rustMsg = ""
			}
			if be.Severity == "" && rustSeverity != "" {
				be.Severity = rustSeverity
				rustSeverity = ""
			}
			if be.Severity == "" {
				be.Severity = "error"
			}
			if be.Message == "" {
				be.Message = "error"
			}

			errors = append(errors, be)
			matched = true
			break
		}

		if !matched {
			// Reset rust context if no match
			// (don't reset on blank lines, as --> may come after blank)
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "|") {
				// Only reset if it's not a rustc continuation line
				if !strings.HasPrefix(strings.TrimSpace(line), "=") {
					rustMsg = ""
					rustSeverity = ""
				}
			}
		}
	}

	return errors
}
