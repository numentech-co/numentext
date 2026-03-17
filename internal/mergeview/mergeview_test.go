package mergeview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConflictsStandard(t *testing.T) {
	content := `line before
<<<<<<< HEAD
local change
=======
remote change
>>>>>>> feature
line after`

	local, base, remote, result, conflicts := ParseConflicts(content)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	// Non-conflict lines should be in all panes
	if local[0] != "line before" {
		t.Errorf("local[0] = %q, want %q", local[0], "line before")
	}
	if base[0] != "line before" {
		t.Errorf("base[0] = %q, want %q", base[0], "line before")
	}
	if remote[0] != "line before" {
		t.Errorf("remote[0] = %q, want %q", remote[0], "line before")
	}

	// Conflict content
	c := conflicts[0]
	if len(c.LocalContent) != 1 || c.LocalContent[0] != "local change" {
		t.Errorf("LocalContent = %v, want [local change]", c.LocalContent)
	}
	if len(c.RemoteContent) != 1 || c.RemoteContent[0] != "remote change" {
		t.Errorf("RemoteContent = %v, want [remote change]", c.RemoteContent)
	}

	// Result should contain all lines including markers
	if len(result) == 0 {
		t.Fatal("result is empty")
	}
	resultStr := strings.Join(result, "\n")
	if !strings.Contains(resultStr, "<<<<<<< HEAD") {
		t.Error("result should contain conflict markers")
	}
}

func TestParseConflictsDiff3(t *testing.T) {
	content := `common line
<<<<<<< HEAD
local version
||||||| merged common ancestors
base version
=======
remote version
>>>>>>> feature
end line`

	local, base, remote, _, conflicts := ParseConflicts(content)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	c := conflicts[0]

	if len(c.LocalContent) != 1 || c.LocalContent[0] != "local version" {
		t.Errorf("LocalContent = %v, want [local version]", c.LocalContent)
	}
	if len(c.BaseContent) != 1 || c.BaseContent[0] != "base version" {
		t.Errorf("BaseContent = %v, want [base version]", c.BaseContent)
	}
	if len(c.RemoteContent) != 1 || c.RemoteContent[0] != "remote version" {
		t.Errorf("RemoteContent = %v, want [remote version]", c.RemoteContent)
	}

	// Verify all panes have the common lines
	if local[0] != "common line" {
		t.Errorf("local[0] = %q", local[0])
	}
	if base[0] != "common line" {
		t.Errorf("base[0] = %q", base[0])
	}
	if remote[0] != "common line" {
		t.Errorf("remote[0] = %q", remote[0])
	}
}

func TestParseConflictsMultiple(t *testing.T) {
	content := `top
<<<<<<< HEAD
first local
=======
first remote
>>>>>>> branch
middle
<<<<<<< HEAD
second local
=======
second remote
>>>>>>> branch
bottom`

	_, _, _, _, conflicts := ParseConflicts(content)

	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(conflicts))
	}

	if conflicts[0].LocalContent[0] != "first local" {
		t.Errorf("conflict 0 local = %q", conflicts[0].LocalContent[0])
	}
	if conflicts[1].RemoteContent[0] != "second remote" {
		t.Errorf("conflict 1 remote = %q", conflicts[1].RemoteContent[0])
	}
}

func TestHasConflictMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"no markers", "just normal text\nnothing special", false},
		{"partial markers", "<<<<<<< HEAD\nsome content", false},
		{"full markers", "<<<<<<< HEAD\nlocal\n=======\nremote\n>>>>>>> branch", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasConflictMarkers(tt.content)
			if got != tt.want {
				t.Errorf("HasConflictMarkers = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveConflictLocal(t *testing.T) {
	content := `before
<<<<<<< HEAD
local line
=======
remote line
>>>>>>> branch
after`

	mv := NewFromConflictFile(content, "")

	if mv.ConflictCount() != 1 {
		t.Fatalf("expected 1 conflict, got %d", mv.ConflictCount())
	}
	if mv.UnresolvedCount() != 1 {
		t.Fatalf("expected 1 unresolved, got %d", mv.UnresolvedCount())
	}

	mv.conflictIdx = 0
	mv.AcceptLocal()

	if mv.UnresolvedCount() != 0 {
		t.Errorf("expected 0 unresolved after accept, got %d", mv.UnresolvedCount())
	}
	if !mv.conflicts[0].Resolved {
		t.Error("conflict should be resolved")
	}
	if mv.conflicts[0].Resolution != "local" {
		t.Errorf("resolution = %q, want %q", mv.conflicts[0].Resolution, "local")
	}
}

func TestResolveConflictRemote(t *testing.T) {
	content := `before
<<<<<<< HEAD
local line
=======
remote line
>>>>>>> branch
after`

	mv := NewFromConflictFile(content, "")
	mv.conflictIdx = 0
	mv.AcceptRemote()

	if !mv.conflicts[0].Resolved {
		t.Error("conflict should be resolved")
	}
	if mv.conflicts[0].Resolution != "remote" {
		t.Errorf("resolution = %q, want %q", mv.conflicts[0].Resolution, "remote")
	}
}

func TestResolveConflictBoth(t *testing.T) {
	content := `before
<<<<<<< HEAD
local line
=======
remote line
>>>>>>> branch
after`

	mv := NewFromConflictFile(content, "")
	mv.conflictIdx = 0
	mv.AcceptBoth()

	if !mv.conflicts[0].Resolved {
		t.Error("conflict should be resolved")
	}
	if mv.conflicts[0].Resolution != "both" {
		t.Errorf("resolution = %q, want %q", mv.conflicts[0].Resolution, "both")
	}
}

func TestNextConflict(t *testing.T) {
	content := `top
<<<<<<< HEAD
c1 local
=======
c1 remote
>>>>>>> branch
mid
<<<<<<< HEAD
c2 local
=======
c2 remote
>>>>>>> branch
bot`

	mv := NewFromConflictFile(content, "")

	if mv.ConflictCount() != 2 {
		t.Fatalf("expected 2 conflicts, got %d", mv.ConflictCount())
	}

	// Start at conflict 0
	mv.conflictIdx = 0
	mv.NextConflict()

	// Should move to conflict 1
	if mv.conflictIdx != 1 {
		t.Errorf("after NextConflict: idx = %d, want 1", mv.conflictIdx)
	}

	mv.NextConflict()
	// Should wrap to conflict 0
	if mv.conflictIdx != 0 {
		t.Errorf("after second NextConflict: idx = %d, want 0", mv.conflictIdx)
	}
}

func TestNextConflictSkipsResolved(t *testing.T) {
	content := `top
<<<<<<< HEAD
c1 local
=======
c1 remote
>>>>>>> branch
mid
<<<<<<< HEAD
c2 local
=======
c2 remote
>>>>>>> branch
bot`

	mv := NewFromConflictFile(content, "")
	mv.conflictIdx = -1

	// Resolve first conflict
	mv.conflictIdx = 0
	mv.AcceptLocal()

	// Next should go to unresolved conflict 1
	mv.NextConflict()
	if mv.conflictIdx != 1 {
		t.Errorf("after NextConflict: idx = %d, want 1 (skipping resolved 0)", mv.conflictIdx)
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "result.txt")

	mv := New(
		[]string{"local line"},
		[]string{"base line"},
		[]string{"remote line"},
		[]string{"result line"},
		nil,
		outPath,
	)

	err := mv.Save()
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if string(data) != "result line\n" {
		t.Errorf("saved content = %q, want %q", string(data), "result line\n")
	}

	if mv.Modified() {
		t.Error("should not be modified after save")
	}
}

func TestStatusText(t *testing.T) {
	mv := New(
		[]string{"a"},
		[]string{"b"},
		[]string{"c"},
		[]string{"result"},
		[]Conflict{{Resolved: false}},
		"test.txt",
	)
	mv.conflictIdx = 0
	mv.focusPane = PaneResult

	status := mv.StatusText()
	if !strings.Contains(status, "RESULT") {
		t.Errorf("status should contain RESULT, got %q", status)
	}
	if !strings.Contains(status, "Conflict 1 of 1") {
		t.Errorf("status should contain conflict info, got %q", status)
	}
}

func TestInsertChar(t *testing.T) {
	mv := New(
		nil, nil, nil,
		[]string{"hello"},
		nil,
		"",
	)
	mv.focusPane = PaneResult
	mv.cursorRow = 0
	mv.cursorCol = 5

	mv.insertChar('!')

	if mv.resultLines[0] != "hello!" {
		t.Errorf("after insert: %q, want %q", mv.resultLines[0], "hello!")
	}
	if mv.cursorCol != 6 {
		t.Errorf("cursorCol = %d, want 6", mv.cursorCol)
	}
	if !mv.modified {
		t.Error("should be modified")
	}
}

func TestInsertNewline(t *testing.T) {
	mv := New(
		nil, nil, nil,
		[]string{"hello world"},
		nil,
		"",
	)
	mv.focusPane = PaneResult
	mv.cursorRow = 0
	mv.cursorCol = 5

	mv.insertNewline()

	if len(mv.resultLines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(mv.resultLines))
	}
	if mv.resultLines[0] != "hello" {
		t.Errorf("line 0 = %q, want %q", mv.resultLines[0], "hello")
	}
	if mv.resultLines[1] != " world" {
		t.Errorf("line 1 = %q, want %q", mv.resultLines[1], " world")
	}
	if mv.cursorRow != 1 || mv.cursorCol != 0 {
		t.Errorf("cursor = (%d,%d), want (1,0)", mv.cursorRow, mv.cursorCol)
	}
}

func TestDeleteCharBack(t *testing.T) {
	mv := New(
		nil, nil, nil,
		[]string{"hello"},
		nil,
		"",
	)
	mv.focusPane = PaneResult
	mv.cursorRow = 0
	mv.cursorCol = 5

	mv.deleteCharBack()

	if mv.resultLines[0] != "hell" {
		t.Errorf("after backspace: %q, want %q", mv.resultLines[0], "hell")
	}
	if mv.cursorCol != 4 {
		t.Errorf("cursorCol = %d, want 4", mv.cursorCol)
	}
}

func TestDeleteCharBackJoinLines(t *testing.T) {
	mv := New(
		nil, nil, nil,
		[]string{"hello", "world"},
		nil,
		"",
	)
	mv.focusPane = PaneResult
	mv.cursorRow = 1
	mv.cursorCol = 0

	mv.deleteCharBack()

	if len(mv.resultLines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(mv.resultLines))
	}
	if mv.resultLines[0] != "helloworld" {
		t.Errorf("after join: %q, want %q", mv.resultLines[0], "helloworld")
	}
}

func TestFocusCycle(t *testing.T) {
	mv := New(nil, nil, nil, []string{""}, nil, "")
	mv.focusPane = PaneLocal

	mv.focusPane = (mv.focusPane + 1) % 4
	if mv.focusPane != PaneBase {
		t.Errorf("after tab: pane = %d, want %d", mv.focusPane, PaneBase)
	}

	mv.focusPane = (mv.focusPane + 1) % 4
	if mv.focusPane != PaneRemote {
		t.Errorf("after tab: pane = %d, want %d", mv.focusPane, PaneRemote)
	}

	mv.focusPane = (mv.focusPane + 1) % 4
	if mv.focusPane != PaneResult {
		t.Errorf("after tab: pane = %d, want %d", mv.focusPane, PaneResult)
	}

	mv.focusPane = (mv.focusPane + 1) % 4
	if mv.focusPane != PaneLocal {
		t.Errorf("after tab: pane = %d, want %d (wrap)", mv.focusPane, PaneLocal)
	}
}

func TestNewFromThreeFiles(t *testing.T) {
	dir := t.TempDir()

	localPath := filepath.Join(dir, "local.txt")
	basePath := filepath.Join(dir, "base.txt")
	remotePath := filepath.Join(dir, "remote.txt")
	outPath := filepath.Join(dir, "merged.txt")

	os.WriteFile(localPath, []byte("line one\nlocal change\nline three\n"), 0644)
	os.WriteFile(basePath, []byte("line one\nbase line\nline three\n"), 0644)
	os.WriteFile(remotePath, []byte("line one\nremote change\nline three\n"), 0644)

	mv, err := NewFromThreeFiles(localPath, basePath, remotePath, outPath)
	if err != nil {
		t.Fatalf("NewFromThreeFiles error: %v", err)
	}

	if mv == nil {
		t.Fatal("MergeView is nil")
	}

	if mv.OutputPath() != outPath {
		t.Errorf("outputPath = %q, want %q", mv.OutputPath(), outPath)
	}
}
