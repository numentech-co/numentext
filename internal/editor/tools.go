package editor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"numentext/internal/config"
)

// FormatResult holds the outcome of running formatters on a file.
type FormatResult struct {
	// Changed is true if at least one formatter modified the file.
	Changed bool
	// Error is set when a formatter fails.
	Error error
}

// LintDiagnostic is a single diagnostic produced by a linter.
type LintDiagnostic struct {
	File     string
	Line     int // 1-based
	Col      int // 1-based, 0 means unknown
	Severity int // 1=error, 2=warning
	Message  string
}

// LintResult holds the outcome of running linters on a file.
type LintResult struct {
	Diagnostics []LintDiagnostic
	Error       error
}

// defaultErrorPattern matches "file:line:col: message" or "file:line: message".
var defaultErrorPattern = regexp.MustCompile(`^(.+?):(\d+):(?:(\d+):)?\s*(.+)$`)

// RunFormatters executes the given formatters in order on the specified file.
// The file is expected to already be saved to disk.
// Each formatter is run with a 5-second timeout.
// If any formatter fails, the original file content is restored and an error is returned.
func RunFormatters(filePath string, formatters []config.ToolDef) FormatResult {
	if len(formatters) == 0 {
		return FormatResult{}
	}

	// Read original content for rollback
	original, err := os.ReadFile(filePath)
	if err != nil {
		return FormatResult{Error: fmt.Errorf("read file: %w", err)}
	}

	for _, f := range formatters {
		args := substituteArgs(f.Args, filePath)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, f.Command, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		cancel()

		if ctx.Err() == context.DeadlineExceeded {
			// Timeout: restore original and return error
			_ = os.WriteFile(filePath, original, 0644)
			return FormatResult{Error: fmt.Errorf("formatter %s timed out after 5s", f.Command)}
		}
		if err != nil {
			// Formatter failed: restore original
			_ = os.WriteFile(filePath, original, 0644)
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg == "" {
				errMsg = err.Error()
			}
			return FormatResult{Error: fmt.Errorf("formatter %s failed: %s", f.Command, errMsg)}
		}
	}

	// Check if file was modified
	modified, err := os.ReadFile(filePath)
	if err != nil {
		return FormatResult{Error: fmt.Errorf("read formatted file: %w", err)}
	}

	return FormatResult{Changed: !bytes.Equal(original, modified)}
}

// RunLinters executes the given linters on the specified file and parses their output.
// Linters run with a 30-second timeout.
func RunLinters(filePath string, linters []config.ToolDef) LintResult {
	if len(linters) == 0 {
		return LintResult{}
	}

	var allDiags []LintDiagnostic

	for _, l := range linters {
		args := substituteArgs(l.Args, filePath)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, l.Command, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		_ = cmd.Run() // Linters often exit with non-zero on findings
		cancel()

		if ctx.Err() == context.DeadlineExceeded {
			return LintResult{Error: fmt.Errorf("linter %s timed out", l.Command)}
		}

		// Parse output (stdout first, then stderr)
		output := stdout.String()
		if output == "" {
			output = stderr.String()
		}

		pattern := defaultErrorPattern
		if l.ErrorPattern != "" {
			if compiled, err := regexp.Compile(l.ErrorPattern); err == nil {
				pattern = compiled
			}
		}

		diags := parseLinterOutput(output, filePath, pattern)
		allDiags = append(allDiags, diags...)
	}

	return LintResult{Diagnostics: allDiags}
}

// parseLinterOutput parses linter output lines into diagnostics.
func parseLinterOutput(output, targetFile string, pattern *regexp.Regexp) []LintDiagnostic {
	var diags []LintDiagnostic
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := pattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		// For default pattern: [full, file, line, col, message]
		// For custom patterns with named groups, use group indices
		var file, lineStr, colStr, message string
		if len(matches) >= 5 {
			file = matches[1]
			lineStr = matches[2]
			colStr = matches[3]
			message = matches[4]
		} else if len(matches) >= 4 {
			file = matches[1]
			lineStr = matches[2]
			message = matches[3]
		} else {
			continue
		}

		// Only include diagnostics for the target file
		if file != targetFile && !strings.HasSuffix(targetFile, file) && !strings.HasSuffix(file, targetFile) {
			// Also accept basename match
			targetBase := filepath.Base(targetFile)
			fileBase := filepath.Base(file)
			if targetBase != fileBase {
				continue
			}
		}

		lineNum, err := strconv.Atoi(lineStr)
		if err != nil {
			continue
		}

		colNum := 0
		if colStr != "" {
			colNum, _ = strconv.Atoi(colStr)
		}

		// Determine severity from message content
		severity := 2 // default to warning
		lowerMsg := strings.ToLower(message)
		if strings.Contains(lowerMsg, "error") || strings.HasPrefix(lowerMsg, "e") {
			severity = 1
		}

		diags = append(diags, LintDiagnostic{
			File:     file,
			Line:     lineNum,
			Col:      colNum,
			Severity: severity,
			Message:  message,
		})
	}
	return diags
}

// substituteArgs replaces {file} placeholders in args with the actual file path.
func substituteArgs(args []string, filePath string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		result[i] = strings.ReplaceAll(arg, "{file}", filePath)
	}
	return result
}
