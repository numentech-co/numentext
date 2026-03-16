package editor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// DiffChangeType represents the type of change for a line in a git diff.
type DiffChangeType int

const (
	DiffAdded    DiffChangeType = iota // new line added
	DiffModified                       // line modified
	DiffDeleted                        // line was deleted (marker at this position)
)

// DiffHunk represents a single hunk from a unified diff.
type DiffHunk struct {
	OldStart int // starting line in old file
	OldCount int // number of lines in old file
	NewStart int // starting line in new file
	NewCount int // number of lines in new file
	Lines    []DiffLine
}

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
	Type    rune   // '+', '-', or ' ' (context)
	Content string
}

// RunGitDiff executes git diff HEAD -- <file> and returns the output.
// Returns empty string and nil error if the file is not in a git repo or git is not installed.
func RunGitDiff(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	dir := filepath.Dir(filePath)

	// Check if git is available
	_, err := exec.LookPath("git")
	if err != nil {
		return "", nil // git not installed, no error
	}

	// Check if file is in a git repo
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err != nil {
		return "", nil // not a git repo, no error
	}

	// Check if file is tracked by git
	checkCmd := exec.Command("git", "-C", dir, "ls-files", "--error-unmatch", filePath)
	if err := checkCmd.Run(); err != nil {
		// File is not tracked -- treat all lines as added
		return "", fmt.Errorf("untracked")
	}

	// Run git diff HEAD -- <file>
	diffCmd := exec.Command("git", "-C", dir, "diff", "HEAD", "--", filePath)
	output, err := diffCmd.Output()
	if err != nil {
		// git diff can return exit code 1 when there are differences
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return string(output), nil
			}
		}
		return "", nil // ignore errors gracefully
	}
	return string(output), nil
}

// ParseDiffHunks parses unified diff output into structured hunks.
func ParseDiffHunks(diff string) []DiffHunk {
	if diff == "" {
		return nil
	}

	var hunks []DiffHunk
	lines := strings.Split(diff, "\n")

	var currentHunk *DiffHunk
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			// Parse hunk header: @@ -oldStart,oldCount +newStart,newCount @@
			hunk := parseHunkHeader(line)
			if hunk != nil {
				hunks = append(hunks, *hunk)
				currentHunk = &hunks[len(hunks)-1]
			}
			continue
		}

		if currentHunk == nil {
			continue
		}

		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case '+':
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: '+', Content: line[1:]})
		case '-':
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: '-', Content: line[1:]})
		case ' ':
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: ' ', Content: line[1:]})
		case '\\':
			// "\ No newline at end of file" -- ignore
		}
	}

	return hunks
}

// parseHunkHeader parses a line like "@@ -1,5 +1,7 @@" or "@@ -0,0 +1,3 @@".
func parseHunkHeader(line string) *DiffHunk {
	// Find the @@ markers
	idx := strings.Index(line, "@@")
	if idx < 0 {
		return nil
	}
	rest := line[idx+2:]
	endIdx := strings.Index(rest, "@@")
	if endIdx < 0 {
		return nil
	}
	header := strings.TrimSpace(rest[:endIdx])

	// Parse "-oldStart,oldCount +newStart,newCount"
	parts := strings.Fields(header)
	if len(parts) < 2 {
		return nil
	}

	hunk := &DiffHunk{}

	// Parse old range
	oldPart := strings.TrimPrefix(parts[0], "-")
	oldParts := strings.Split(oldPart, ",")
	hunk.OldStart, _ = strconv.Atoi(oldParts[0])
	if len(oldParts) > 1 {
		hunk.OldCount, _ = strconv.Atoi(oldParts[1])
	} else {
		hunk.OldCount = 1
	}

	// Parse new range
	newPart := strings.TrimPrefix(parts[1], "+")
	newParts := strings.Split(newPart, ",")
	hunk.NewStart, _ = strconv.Atoi(newParts[0])
	if len(newParts) > 1 {
		hunk.NewCount, _ = strconv.Atoi(newParts[1])
	} else {
		hunk.NewCount = 1
	}

	return hunk
}

// DiffMarkers computes per-line markers from a git diff.
// Returns a map from 0-based line number to DiffChangeType.
// For untracked files, returns nil (caller should treat all lines as added).
func DiffMarkers(filePath string) (map[int]DiffChangeType, error) {
	diff, err := RunGitDiff(filePath)
	if err != nil {
		if err.Error() == "untracked" {
			// Return a sentinel: nil map with special error
			return nil, err
		}
		return nil, nil
	}

	if diff == "" {
		return nil, nil // no changes
	}

	hunks := ParseDiffHunks(diff)
	markers := make(map[int]DiffChangeType)

	for _, hunk := range hunks {
		newLine := hunk.NewStart // 1-based line in new file

		// Collect consecutive removed and added lines to detect modifications
		i := 0
		for i < len(hunk.Lines) {
			dl := hunk.Lines[i]

			switch dl.Type {
			case ' ':
				// Context line -- advance position
				newLine++
				i++

			case '-':
				// Count consecutive removed lines
				removedCount := 0
				for i+removedCount < len(hunk.Lines) && hunk.Lines[i+removedCount].Type == '-' {
					removedCount++
				}

				// Count consecutive added lines following the removed lines
				addedCount := 0
				for i+removedCount+addedCount < len(hunk.Lines) && hunk.Lines[i+removedCount+addedCount].Type == '+' {
					addedCount++
				}

				if addedCount > 0 && removedCount > 0 {
					// Modified lines: min(removed, added) are modifications
					modCount := removedCount
					if addedCount < modCount {
						modCount = addedCount
					}
					for j := 0; j < modCount; j++ {
						markers[newLine-1] = DiffModified // 0-based
						newLine++
					}
					// Extra added lines beyond modifications
					for j := modCount; j < addedCount; j++ {
						markers[newLine-1] = DiffAdded
						newLine++
					}
					// Extra removed lines (pure deletions) -- mark at current position
					if removedCount > modCount {
						markers[newLine-1] = DiffDeleted
					}
				} else {
					// Pure deletion -- mark at the line where deletion occurred
					lineNum := newLine - 1 // 0-based, mark at line before or at current position
					if lineNum < 0 {
						lineNum = 0
					}
					markers[lineNum] = DiffDeleted
				}

				i += removedCount + addedCount

			case '+':
				// Pure addition (no preceding removal)
				markers[newLine-1] = DiffAdded
				newLine++
				i++
			}
		}
	}

	return markers, nil
}

// diffMarkerChar returns the marker character and foreground color for a diff change type.
func diffMarkerChar(changeType DiffChangeType) (rune, tcell.Color) {
	switch changeType {
	case DiffAdded:
		return '+', tcell.ColorGreen
	case DiffModified:
		return '~', tcell.ColorBlue
	case DiffDeleted:
		return '-', tcell.ColorRed
	default:
		return ' ', tcell.ColorWhite
	}
}
