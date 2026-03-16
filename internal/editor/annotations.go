package editor

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Annotation represents a TODO/FIXME/etc. found in a source file.
type Annotation struct {
	File string // file path
	Line int    // 1-based line number
	Tag  string // e.g. "TODO", "FIXME", "BUG"
	Text string // description text after the tag
}

// Known annotation tags (matched case-insensitively).
var annotationTags = []string{"TODO", "FIXME", "BUG", "HACK", "WONTFIX", "XXX", "NOTE"}

// commentPrefixes are single-line comment markers for various languages.
var commentPrefixes = []string{"//", "#", "--", ";"}

// tagPattern matches an annotation tag followed by optional colon/space and text.
// Built once at init time.
var tagPattern *regexp.Regexp

func init() {
	// Build pattern like (?i)\b(TODO|FIXME|BUG|HACK|WONTFIX|XXX|NOTE)\b[:\s]*(.*)
	joined := strings.Join(annotationTags, "|")
	tagPattern = regexp.MustCompile(`(?i)\b(` + joined + `)\b[:\s]*(.*)`)
}

// ScanBuffer scans a buffer's lines for annotation tags inside comments.
// filePath is used to label results. Lines are 1-based in the output.
func ScanBuffer(filePath string, lines []string) []Annotation {
	var results []Annotation
	inBlockComment := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track multi-line /* ... */ comments
		if inBlockComment {
			if idx := strings.Index(trimmed, "*/"); idx >= 0 {
				// Check the portion inside the block comment
				inside := trimmed[:idx]
				if ann := matchTag(filePath, i+1, inside); ann != nil {
					results = append(results, *ann)
				}
				inBlockComment = false
				// Check remainder after */ for a new comment start
				remainder := trimmed[idx+2:]
				if ann := scanLineForAnnotation(filePath, i+1, remainder); ann != nil {
					results = append(results, *ann)
				}
			} else {
				// Entire line is inside block comment
				if ann := matchTag(filePath, i+1, trimmed); ann != nil {
					results = append(results, *ann)
				}
			}
			continue
		}

		// Check for block comment start: /* ... */  or  /* ...
		if idx := strings.Index(trimmed, "/*"); idx >= 0 {
			after := trimmed[idx+2:]
			if endIdx := strings.Index(after, "*/"); endIdx >= 0 {
				// Single-line block comment
				inside := after[:endIdx]
				if ann := matchTag(filePath, i+1, inside); ann != nil {
					results = append(results, *ann)
				}
			} else {
				// Multi-line block comment starts here
				inBlockComment = true
				if ann := matchTag(filePath, i+1, after); ann != nil {
					results = append(results, *ann)
				}
			}
			continue
		}

		// Check single-line comments
		if ann := scanLineForAnnotation(filePath, i+1, trimmed); ann != nil {
			results = append(results, *ann)
		}
	}

	return results
}

// scanLineForAnnotation checks if a line starts with a comment prefix
// and contains an annotation tag.
func scanLineForAnnotation(filePath string, lineNum int, line string) *Annotation {
	trimmed := strings.TrimSpace(line)

	for _, prefix := range commentPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			commentText := trimmed[len(prefix):]
			return matchTag(filePath, lineNum, commentText)
		}
	}

	// Also check if the line contains // or # after some code
	// (inline comments). Look for the comment prefix anywhere.
	for _, prefix := range commentPrefixes {
		idx := strings.Index(line, prefix)
		if idx > 0 {
			commentText := line[idx+len(prefix):]
			return matchTag(filePath, lineNum, commentText)
		}
	}

	return nil
}

// matchTag checks text for an annotation tag pattern, returning an Annotation or nil.
func matchTag(filePath string, lineNum int, text string) *Annotation {
	m := tagPattern.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	tag := strings.ToUpper(m[1])
	desc := strings.TrimSpace(m[2])
	return &Annotation{
		File: filePath,
		Line: lineNum,
		Tag:  tag,
		Text: desc,
	}
}

// ScanFile reads a file and scans it for annotations.
func ScanFile(filePath string) []Annotation {
	// Determine if the file extension is one we should scan.
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go", ".c", ".cpp", ".cc", ".cxx", ".h", ".hpp",
		".py", ".js", ".ts", ".jsx", ".tsx",
		".java", ".rs", ".rb", ".sh", ".bash", ".zsh",
		".lua", ".sql", ".r", ".pl", ".pm",
		".cs", ".swift", ".kt", ".scala", ".zig",
		".hs", ".el", ".clj", ".ex", ".exs",
		".yaml", ".yml", ".toml", ".cfg", ".ini", ".conf",
		".css", ".scss", ".less", ".html", ".xml", ".md",
		".php", ".m", ".mm", ".v", ".sv":
		// supported
	default:
		return nil
	}

	// Read lines from disk is not needed since we scan open buffers.
	// This function is here as a utility but the main path is ScanBuffer.
	return nil
}

// ScanAllTabs scans all open editor tabs for annotations.
func ScanAllTabs(tabs []*Tab) []Annotation {
	var all []Annotation
	for _, tab := range tabs {
		if tab.Buffer == nil {
			continue
		}
		filePath := tab.FilePath
		if filePath == "" {
			filePath = tab.Name
		}
		anns := ScanBuffer(filePath, tab.Buffer.Lines())
		all = append(all, anns...)
	}
	return all
}
