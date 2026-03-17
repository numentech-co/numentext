package mergeview

import (
	"testing"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input string
		want  int // expected word count
	}{
		{"hello world", 3},     // "hello", " ", "world"
		{"  spaced  ", 3},      // "  ", "spaced", "  "
		{"single", 1},          // "single"
		{"", 0},                // empty
		{"a b c", 5},           // "a", " ", "b", " ", "c"
	}

	for _, tt := range tests {
		words := splitWords(tt.input)
		if len(words) != tt.want {
			t.Errorf("splitWords(%q) = %d words %v, want %d", tt.input, len(words), words, tt.want)
		}
	}
}

func TestComputeLCS(t *testing.T) {
	a := []string{"the", " ", "quick", " ", "brown", " ", "fox"}
	b := []string{"the", " ", "slow", " ", "brown", " ", "dog"}

	lcs := computeLCS(a, b)

	// Common: "the", " ", " ", "brown", " "
	if len(lcs) < 4 {
		t.Errorf("LCS length = %d, expected at least 4, got %v", len(lcs), lcs)
	}
}

func TestComputeWordDiffs(t *testing.T) {
	localLine := "the quick brown fox"
	remoteLine := "the slow brown dog"

	localDiffs, remoteDiffs := ComputeWordDiffs(localLine, remoteLine)

	// Check that common words are marked as such
	foundCommonLocal := false
	foundUniqueLocal := false
	for _, d := range localDiffs {
		if d.Text == "brown" && d.IsCommon {
			foundCommonLocal = true
		}
		if d.Text == "quick" && !d.IsCommon {
			foundUniqueLocal = true
		}
	}

	if !foundCommonLocal {
		t.Error("expected 'brown' to be common in local diffs")
	}
	if !foundUniqueLocal {
		t.Error("expected 'quick' to be unique in local diffs")
	}

	foundCommonRemote := false
	foundUniqueRemote := false
	for _, d := range remoteDiffs {
		if d.Text == "brown" && d.IsCommon {
			foundCommonRemote = true
		}
		if d.Text == "slow" && !d.IsCommon {
			foundUniqueRemote = true
		}
	}

	if !foundCommonRemote {
		t.Error("expected 'brown' to be common in remote diffs")
	}
	if !foundUniqueRemote {
		t.Error("expected 'slow' to be unique in remote diffs")
	}
}

func TestComputeWordDiffsIdentical(t *testing.T) {
	line := "same line here"
	localDiffs, remoteDiffs := ComputeWordDiffs(line, line)

	for _, d := range localDiffs {
		if !d.IsCommon {
			t.Errorf("expected all local diffs to be common, got %v", d)
		}
	}
	for _, d := range remoteDiffs {
		if !d.IsCommon {
			t.Errorf("expected all remote diffs to be common, got %v", d)
		}
	}
}

func TestComputeWordDiffsCompleteDiff(t *testing.T) {
	localDiffs, remoteDiffs := ComputeWordDiffs("abc", "xyz")

	if len(localDiffs) == 0 {
		t.Fatal("expected local diffs")
	}
	if localDiffs[0].IsCommon {
		t.Error("completely different words should not be common")
	}
	if len(remoteDiffs) == 0 {
		t.Fatal("expected remote diffs")
	}
	if remoteDiffs[0].IsCommon {
		t.Error("completely different words should not be common")
	}
}

func TestComputeLineDiff(t *testing.T) {
	local := []string{"line one", "changed local", "line three"}
	remote := []string{"line one", "changed remote", "line three"}

	result := ComputeLineDiff(local, remote)

	// "changed local" (idx 1) should be local-only
	if !result.LocalOnly[1] {
		t.Error("expected idx 1 to be local-only")
	}

	// "changed remote" (idx 1) should be remote-only
	if !result.RemoteOnly[1] {
		t.Error("expected idx 1 to be remote-only")
	}

	// "line one" (idx 0) should be common
	if _, ok := result.CommonMap[0]; !ok {
		t.Error("expected idx 0 to be common")
	}

	// "line three" (idx 2) should be common
	if _, ok := result.CommonMap[2]; !ok {
		t.Error("expected idx 2 to be common")
	}
}

func TestComputeWordDiffsEmpty(t *testing.T) {
	localDiffs, remoteDiffs := ComputeWordDiffs("", "something")

	if len(localDiffs) != 0 {
		t.Errorf("expected 0 local diffs for empty local, got %d", len(localDiffs))
	}
	if len(remoteDiffs) == 0 {
		t.Error("expected remote diffs for non-empty remote")
	}
}
