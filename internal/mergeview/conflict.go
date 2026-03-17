package mergeview

import (
	"strings"
)

// Conflict represents a single merge conflict region.
type Conflict struct {
	// Line ranges in the respective pane content
	LocalStart, LocalEnd   int
	BaseStart, BaseEnd     int
	RemoteStart, RemoteEnd int
	ResultStart, ResultEnd int

	// Content extracted from the conflict
	LocalContent  []string
	BaseContent   []string
	RemoteContent []string

	Resolved   bool
	Resolution string // "local", "remote", "base", "both", "manual", ""
}

// ParseConflicts parses conflict markers from file content.
// Supports both standard format and diff3 format (with ||||||| base marker).
// Returns the separated LOCAL, BASE, REMOTE content and a list of conflicts,
// plus the initial RESULT content (same as original file content).
func ParseConflicts(content string) (local, base, remote, result []string, conflicts []Conflict) {
	lines := strings.Split(content, "\n")

	// Remove trailing empty line from Split if content ends with \n
	if len(lines) > 0 && lines[len(lines)-1] == "" && strings.HasSuffix(content, "\n") {
		lines = lines[:len(lines)-1]
	}

	type region int
	const (
		regionNone region = iota
		regionLocal
		regionBase
		regionRemote
	)

	currentRegion := regionNone
	var currentConflict *Conflict
	var localContent, baseContent, remoteContent []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "<<<<<<<"):
			// Start of conflict - LOCAL section
			currentConflict = &Conflict{
				LocalStart: len(local),
				BaseStart:  len(base),
				RemoteStart: len(remote),
				ResultStart: len(result),
			}
			currentRegion = regionLocal
			localContent = nil
			baseContent = nil
			remoteContent = nil
			// The result keeps the full conflict markers for initial display
			result = append(result, line)

		case strings.HasPrefix(trimmed, "|||||||") && currentRegion == regionLocal:
			// diff3 base marker
			currentRegion = regionBase
			result = append(result, line)

		case trimmed == "=======" && (currentRegion == regionLocal || currentRegion == regionBase):
			// Separator between local/base and remote
			currentRegion = regionRemote
			result = append(result, line)

		case strings.HasPrefix(trimmed, ">>>>>>>") && currentRegion == regionRemote:
			// End of conflict - REMOTE section done
			if currentConflict != nil {
				currentConflict.LocalContent = localContent
				currentConflict.BaseContent = baseContent
				currentConflict.RemoteContent = remoteContent

				// Add content to each pane
				for _, l := range localContent {
					local = append(local, l)
				}
				currentConflict.LocalEnd = len(local) - 1

				if len(baseContent) > 0 {
					for _, l := range baseContent {
						base = append(base, l)
					}
				} else {
					// No base content in non-diff3 format, use empty placeholder
					base = append(base, "")
				}
				currentConflict.BaseEnd = len(base) - 1

				for _, l := range remoteContent {
					remote = append(remote, l)
				}
				currentConflict.RemoteEnd = len(remote) - 1

				result = append(result, line)
				currentConflict.ResultEnd = len(result) - 1

				conflicts = append(conflicts, *currentConflict)
				currentConflict = nil
			}
			currentRegion = regionNone

		default:
			switch currentRegion {
			case regionLocal:
				localContent = append(localContent, line)
				result = append(result, line)
			case regionBase:
				baseContent = append(baseContent, line)
				result = append(result, line)
			case regionRemote:
				remoteContent = append(remoteContent, line)
				result = append(result, line)
			case regionNone:
				// Non-conflict line - same in all panes
				local = append(local, line)
				base = append(base, line)
				remote = append(remote, line)
				result = append(result, line)
			}
		}
	}

	return local, base, remote, result, conflicts
}

// ParseThreeWay takes three separate file contents (local, base, remote)
// and builds the merge view data. The result starts as a copy of local.
// Conflicts are detected where local and remote differ from base.
func ParseThreeWay(localContent, baseContent, remoteContent string) (local, baseParsed, remote, result []string, conflicts []Conflict) {
	local = splitLines(localContent)
	baseParsed = splitLines(baseContent)
	remote = splitLines(remoteContent)

	// Simple approach: find regions where local or remote differ from base
	// For now, start result as local content and mark differing regions as conflicts
	result = make([]string, len(local))
	copy(result, local)

	// Compute diffs: local vs base, remote vs base
	localDiff := ComputeLineDiff(baseParsed, local)
	remoteDiff := ComputeLineDiff(baseParsed, remote)

	// Find base lines that are modified by both local and remote differently
	// This is a simplified conflict detection
	for baseIdx := 0; baseIdx < len(baseParsed); baseIdx++ {
		_, localChanged := localDiff.LocalOnly[baseIdx]
		_, remoteChanged := remoteDiff.LocalOnly[baseIdx]

		if localChanged && remoteChanged {
			// Both sides changed this base line - conflict
			// Find the corresponding lines in local and remote
			c := Conflict{
				LocalStart:  baseIdx,
				LocalEnd:    baseIdx,
				BaseStart:   baseIdx,
				BaseEnd:     baseIdx,
				RemoteStart: baseIdx,
				RemoteEnd:   baseIdx,
				ResultStart: baseIdx,
				ResultEnd:   baseIdx,
				LocalContent:  []string{safeGetLine(local, baseIdx)},
				BaseContent:   []string{baseParsed[baseIdx]},
				RemoteContent: []string{safeGetLine(remote, baseIdx)},
			}
			conflicts = append(conflicts, c)
		}
	}

	return
}

// SplitLinesPublic is an exported version of splitLines for use by other packages.
func SplitLinesPublic(content string) []string {
	return splitLines(content)
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	// Remove trailing empty line
	if len(lines) > 0 && lines[len(lines)-1] == "" && strings.HasSuffix(content, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func safeGetLine(lines []string, idx int) string {
	if idx >= 0 && idx < len(lines) {
		return lines[idx]
	}
	return ""
}

// HasConflictMarkers checks if content contains git merge conflict markers.
func HasConflictMarkers(content string) bool {
	return strings.Contains(content, "<<<<<<<") &&
		strings.Contains(content, "=======") &&
		strings.Contains(content, ">>>>>>>")
}
