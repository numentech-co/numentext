package editor

import (
	"testing"
)

func TestParseDiffHunks_Empty(t *testing.T) {
	hunks := ParseDiffHunks("")
	if len(hunks) != 0 {
		t.Errorf("expected 0 hunks, got %d", len(hunks))
	}
}

func TestParseDiffHunks_SingleAdd(t *testing.T) {
	diff := `diff --git a/file.go b/file.go
index abc1234..def5678 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,5 @@
 line1
+added1
+added2
 line2
 line3
`
	hunks := ParseDiffHunks(diff)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	h := hunks[0]
	if h.OldStart != 1 || h.OldCount != 3 {
		t.Errorf("old range: got %d,%d, want 1,3", h.OldStart, h.OldCount)
	}
	if h.NewStart != 1 || h.NewCount != 5 {
		t.Errorf("new range: got %d,%d, want 1,5", h.NewStart, h.NewCount)
	}
	if len(h.Lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(h.Lines))
	}
}

func TestParseDiffHunks_Modify(t *testing.T) {
	diff := `@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3
`
	hunks := ParseDiffHunks(diff)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if len(hunks[0].Lines) != 4 {
		t.Errorf("expected 4 lines, got %d", len(hunks[0].Lines))
	}
}

func TestParseDiffHunks_Delete(t *testing.T) {
	diff := `@@ -1,5 +1,3 @@
 line1
-deleted1
-deleted2
 line2
 line3
`
	hunks := ParseDiffHunks(diff)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
}

func TestParseDiffHunks_NoCount(t *testing.T) {
	// Single line change: @@ -1 +1 @@
	diff := `@@ -1 +1 @@
-old
+new
`
	hunks := ParseDiffHunks(diff)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].OldCount != 1 || hunks[0].NewCount != 1 {
		t.Errorf("expected counts of 1,1, got %d,%d", hunks[0].OldCount, hunks[0].NewCount)
	}
}

func TestDiffMarkersFromHunks_Added(t *testing.T) {
	diff := `@@ -1,3 +1,5 @@
 line1
+added1
+added2
 line2
 line3
`
	hunks := ParseDiffHunks(diff)
	markers := make(map[int]DiffChangeType)

	for _, hunk := range hunks {
		newLine := hunk.NewStart
		i := 0
		for i < len(hunk.Lines) {
			dl := hunk.Lines[i]
			switch dl.Type {
			case ' ':
				newLine++
				i++
			case '-':
				removedCount := 0
				for i+removedCount < len(hunk.Lines) && hunk.Lines[i+removedCount].Type == '-' {
					removedCount++
				}
				addedCount := 0
				for i+removedCount+addedCount < len(hunk.Lines) && hunk.Lines[i+removedCount+addedCount].Type == '+' {
					addedCount++
				}
				if addedCount > 0 && removedCount > 0 {
					modCount := removedCount
					if addedCount < modCount {
						modCount = addedCount
					}
					for j := 0; j < modCount; j++ {
						markers[newLine-1] = DiffModified
						newLine++
					}
					for j := modCount; j < addedCount; j++ {
						markers[newLine-1] = DiffAdded
						newLine++
					}
					if removedCount > modCount {
						markers[newLine-1] = DiffDeleted
					}
				} else {
					lineNum := newLine - 1
					if lineNum < 0 {
						lineNum = 0
					}
					markers[lineNum] = DiffDeleted
				}
				i += removedCount + addedCount
			case '+':
				markers[newLine-1] = DiffAdded
				newLine++
				i++
			}
		}
	}

	// Lines added at index 1 and 2 (0-based)
	if markers[1] != DiffAdded {
		t.Errorf("line 1: expected DiffAdded, got %v", markers[1])
	}
	if markers[2] != DiffAdded {
		t.Errorf("line 2: expected DiffAdded, got %v", markers[2])
	}
	if _, ok := markers[0]; ok {
		t.Errorf("line 0 should not have a marker")
	}
}

func TestDiffMarkersFromHunks_Modified(t *testing.T) {
	diff := `@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3
`
	hunks := ParseDiffHunks(diff)
	markers := make(map[int]DiffChangeType)

	for _, hunk := range hunks {
		newLine := hunk.NewStart
		i := 0
		for i < len(hunk.Lines) {
			dl := hunk.Lines[i]
			switch dl.Type {
			case ' ':
				newLine++
				i++
			case '-':
				removedCount := 0
				for i+removedCount < len(hunk.Lines) && hunk.Lines[i+removedCount].Type == '-' {
					removedCount++
				}
				addedCount := 0
				for i+removedCount+addedCount < len(hunk.Lines) && hunk.Lines[i+removedCount+addedCount].Type == '+' {
					addedCount++
				}
				if addedCount > 0 && removedCount > 0 {
					modCount := removedCount
					if addedCount < modCount {
						modCount = addedCount
					}
					for j := 0; j < modCount; j++ {
						markers[newLine-1] = DiffModified
						newLine++
					}
					for j := modCount; j < addedCount; j++ {
						markers[newLine-1] = DiffAdded
						newLine++
					}
				} else {
					lineNum := newLine - 1
					if lineNum < 0 {
						lineNum = 0
					}
					markers[lineNum] = DiffDeleted
				}
				i += removedCount + addedCount
			case '+':
				markers[newLine-1] = DiffAdded
				newLine++
				i++
			}
		}
	}

	// Line at index 1 should be modified
	if markers[1] != DiffModified {
		t.Errorf("line 1: expected DiffModified, got %v", markers[1])
	}
}

func TestParseHunkHeader(t *testing.T) {
	tests := []struct {
		line     string
		oldStart int
		oldCount int
		newStart int
		newCount int
	}{
		{"@@ -1,3 +1,5 @@", 1, 3, 1, 5},
		{"@@ -10,20 +15,25 @@ func foo()", 10, 20, 15, 25},
		{"@@ -0,0 +1,3 @@", 0, 0, 1, 3},
		{"@@ -1 +1 @@", 1, 1, 1, 1},
	}

	for _, tt := range tests {
		h := parseHunkHeader(tt.line)
		if h == nil {
			t.Errorf("parseHunkHeader(%q) returned nil", tt.line)
			continue
		}
		if h.OldStart != tt.oldStart || h.OldCount != tt.oldCount {
			t.Errorf("parseHunkHeader(%q): old got %d,%d, want %d,%d",
				tt.line, h.OldStart, h.OldCount, tt.oldStart, tt.oldCount)
		}
		if h.NewStart != tt.newStart || h.NewCount != tt.newCount {
			t.Errorf("parseHunkHeader(%q): new got %d,%d, want %d,%d",
				tt.line, h.NewStart, h.NewCount, tt.newStart, tt.newCount)
		}
	}
}

func TestDiffMarkerChar(t *testing.T) {
	ch, _ := diffMarkerChar(DiffAdded)
	if ch != '+' {
		t.Errorf("DiffAdded marker: got %c, want +", ch)
	}
	ch, _ = diffMarkerChar(DiffModified)
	if ch != '~' {
		t.Errorf("DiffModified marker: got %c, want ~", ch)
	}
	ch, _ = diffMarkerChar(DiffDeleted)
	if ch != '-' {
		t.Errorf("DiffDeleted marker: got %c, want -", ch)
	}
}
