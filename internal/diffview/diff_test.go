package diffview

import (
	"testing"
)

func TestComputeLCS(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want []string
	}{
		{
			name: "identical",
			a:    []string{"a", "b", "c"},
			b:    []string{"a", "b", "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "completely different",
			a:    []string{"a", "b"},
			b:    []string{"c", "d"},
			want: []string{},
		},
		{
			name: "one added",
			a:    []string{"a", "b"},
			b:    []string{"a", "x", "b"},
			want: []string{"a", "b"},
		},
		{
			name: "one deleted",
			a:    []string{"a", "x", "b"},
			b:    []string{"a", "b"},
			want: []string{"a", "b"},
		},
		{
			name: "empty left",
			a:    []string{},
			b:    []string{"a", "b"},
			want: []string{},
		},
		{
			name: "empty right",
			a:    []string{"a", "b"},
			b:    []string{},
			want: []string{},
		},
		{
			name: "both empty",
			a:    []string{},
			b:    []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeLCS(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Fatalf("computeLCS len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("computeLCS[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestComputeDiffIdentical(t *testing.T) {
	lines := []string{"func foo() {", "  return 42", "}"}
	result := ComputeDiff(lines, lines)

	if len(result.Left) != 3 || len(result.Right) != 3 {
		t.Fatalf("expected 3 lines each, got left=%d right=%d", len(result.Left), len(result.Right))
	}
	for i := 0; i < 3; i++ {
		if result.Left[i].Type != DiffNormal {
			t.Errorf("left[%d].Type = %d, want DiffNormal", i, result.Left[i].Type)
		}
		if result.Right[i].Type != DiffNormal {
			t.Errorf("right[%d].Type = %d, want DiffNormal", i, result.Right[i].Type)
		}
	}
}

func TestComputeDiffAddedLine(t *testing.T) {
	left := []string{"a", "b"}
	right := []string{"a", "new", "b"}
	result := ComputeDiff(left, right)

	if len(result.Left) != len(result.Right) {
		t.Fatalf("mismatched lengths: left=%d right=%d", len(result.Left), len(result.Right))
	}

	// Expected: a(normal), filler/new(added), b(normal)
	if len(result.Left) != 3 {
		t.Fatalf("expected 3 aligned rows, got %d", len(result.Left))
	}

	if result.Left[0].Type != DiffNormal || result.Left[0].Text != "a" {
		t.Errorf("left[0]: type=%d text=%q, want Normal/a", result.Left[0].Type, result.Left[0].Text)
	}
	if result.Left[1].Type != DiffFiller {
		t.Errorf("left[1]: type=%d, want DiffFiller", result.Left[1].Type)
	}
	if result.Right[1].Type != DiffAdded || result.Right[1].Text != "new" {
		t.Errorf("right[1]: type=%d text=%q, want Added/new", result.Right[1].Type, result.Right[1].Text)
	}
	if result.Left[2].Type != DiffNormal || result.Left[2].Text != "b" {
		t.Errorf("left[2]: type=%d text=%q, want Normal/b", result.Left[2].Type, result.Left[2].Text)
	}
}

func TestComputeDiffDeletedLine(t *testing.T) {
	left := []string{"a", "old", "b"}
	right := []string{"a", "b"}
	result := ComputeDiff(left, right)

	if len(result.Left) != 3 {
		t.Fatalf("expected 3 aligned rows, got %d", len(result.Left))
	}

	if result.Left[1].Type != DiffDeleted || result.Left[1].Text != "old" {
		t.Errorf("left[1]: type=%d text=%q, want Deleted/old", result.Left[1].Type, result.Left[1].Text)
	}
	if result.Right[1].Type != DiffFiller {
		t.Errorf("right[1]: type=%d, want DiffFiller", result.Right[1].Type)
	}
}

func TestComputeDiffModifiedLine(t *testing.T) {
	left := []string{"a", "return 42", "b"}
	right := []string{"a", "return 43", "b"}
	result := ComputeDiff(left, right)

	if len(result.Left) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Left))
	}

	if result.Left[1].Type != DiffModified {
		t.Errorf("left[1].Type = %d, want DiffModified", result.Left[1].Type)
	}
	if result.Right[1].Type != DiffModified {
		t.Errorf("right[1].Type = %d, want DiffModified", result.Right[1].Type)
	}

	// Word diffs should highlight "42" on left and "43" on right
	if len(result.Left[1].WordDiffs) == 0 {
		t.Error("expected word diffs on left modified line")
	}
	if len(result.Right[1].WordDiffs) == 0 {
		t.Error("expected word diffs on right modified line")
	}
}

func TestComputeDiffAlignment(t *testing.T) {
	// Left and right should always have equal length
	left := []string{"a", "b", "c", "d"}
	right := []string{"a", "x", "y", "d"}
	result := ComputeDiff(left, right)

	if len(result.Left) != len(result.Right) {
		t.Fatalf("misaligned: left=%d right=%d", len(result.Left), len(result.Right))
	}
}

func TestComputeDiffEmpty(t *testing.T) {
	result := ComputeDiff([]string{}, []string{})
	if len(result.Left) != 0 || len(result.Right) != 0 {
		t.Errorf("expected empty result, got left=%d right=%d", len(result.Left), len(result.Right))
	}
}

func TestComputeDiffLeftEmpty(t *testing.T) {
	right := []string{"a", "b"}
	result := ComputeDiff([]string{}, right)

	if len(result.Left) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Left))
	}
	for i := 0; i < 2; i++ {
		if result.Left[i].Type != DiffFiller {
			t.Errorf("left[%d].Type = %d, want DiffFiller", i, result.Left[i].Type)
		}
		if result.Right[i].Type != DiffAdded {
			t.Errorf("right[%d].Type = %d, want DiffAdded", i, result.Right[i].Type)
		}
	}
}

func TestComputeDiffRightEmpty(t *testing.T) {
	left := []string{"a", "b"}
	result := ComputeDiff(left, []string{})

	if len(result.Left) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Left))
	}
	for i := 0; i < 2; i++ {
		if result.Left[i].Type != DiffDeleted {
			t.Errorf("left[%d].Type = %d, want DiffDeleted", i, result.Left[i].Type)
		}
		if result.Right[i].Type != DiffFiller {
			t.Errorf("right[%d].Type = %d, want DiffFiller", i, result.Right[i].Type)
		}
	}
}

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"  leading spaces", []string{"leading", "spaces"}},
		{"tabs\there", []string{"tabs", "here"}},
		{"", []string{}},
		{"single", []string{"single"}},
	}

	for _, tt := range tests {
		words := splitWords(tt.input)
		if len(words) != len(tt.want) {
			t.Errorf("splitWords(%q) len = %d, want %d", tt.input, len(words), len(tt.want))
			continue
		}
		for i, w := range words {
			if w.text != tt.want[i] {
				t.Errorf("splitWords(%q)[%d] = %q, want %q", tt.input, i, w.text, tt.want[i])
			}
		}
	}
}

func TestSplitWordsOffsets(t *testing.T) {
	words := splitWords("  hello world")
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(words))
	}
	if words[0].start != 2 || words[0].end != 7 {
		t.Errorf("word 0: start=%d end=%d, want 2,7", words[0].start, words[0].end)
	}
	if words[1].start != 8 || words[1].end != 13 {
		t.Errorf("word 1: start=%d end=%d, want 8,13", words[1].start, words[1].end)
	}
}

func TestWordDiff(t *testing.T) {
	leftDiffs, rightDiffs := computeWordDiff("return 42", "return 43")

	if len(leftDiffs) != 1 {
		t.Fatalf("expected 1 left word diff, got %d", len(leftDiffs))
	}
	if len(rightDiffs) != 1 {
		t.Fatalf("expected 1 right word diff, got %d", len(rightDiffs))
	}

	// "42" starts at offset 7
	if leftDiffs[0].Start != 7 || leftDiffs[0].End != 9 {
		t.Errorf("left diff: start=%d end=%d, want 7,9", leftDiffs[0].Start, leftDiffs[0].End)
	}
	if leftDiffs[0].Type != DiffDeleted {
		t.Errorf("left diff type = %d, want DiffDeleted", leftDiffs[0].Type)
	}

	if rightDiffs[0].Start != 7 || rightDiffs[0].End != 9 {
		t.Errorf("right diff: start=%d end=%d, want 7,9", rightDiffs[0].Start, rightDiffs[0].End)
	}
	if rightDiffs[0].Type != DiffAdded {
		t.Errorf("right diff type = %d, want DiffAdded", rightDiffs[0].Type)
	}
}

func TestWordDiffMultipleChanges(t *testing.T) {
	leftDiffs, rightDiffs := computeWordDiff("the quick brown fox", "the slow brown dog")

	// "quick" -> "slow" and "fox" -> "dog"
	if len(leftDiffs) != 2 {
		t.Fatalf("expected 2 left word diffs, got %d", len(leftDiffs))
	}
	if len(rightDiffs) != 2 {
		t.Fatalf("expected 2 right word diffs, got %d", len(rightDiffs))
	}

	if leftDiffs[0].Start != 4 || leftDiffs[0].End != 9 {
		t.Errorf("left diff 0: start=%d end=%d, want 4,9 (quick)", leftDiffs[0].Start, leftDiffs[0].End)
	}
	if leftDiffs[1].Start != 16 || leftDiffs[1].End != 19 {
		t.Errorf("left diff 1: start=%d end=%d, want 16,19 (fox)", leftDiffs[1].Start, leftDiffs[1].End)
	}
}

func TestLineNumbers(t *testing.T) {
	left := []string{"a", "old", "b"}
	right := []string{"a", "new", "b"}
	result := ComputeDiff(left, right)

	// "a" is line 1 on both sides
	if result.Left[0].LineNum != 1 || result.Right[0].LineNum != 1 {
		t.Errorf("row 0 line nums: left=%d right=%d, want 1,1",
			result.Left[0].LineNum, result.Right[0].LineNum)
	}

	// Modified lines keep their original line numbers
	if result.Left[1].LineNum != 2 || result.Right[1].LineNum != 2 {
		t.Errorf("row 1 line nums: left=%d right=%d, want 2,2",
			result.Left[1].LineNum, result.Right[1].LineNum)
	}

	// "b" is line 3 on both sides
	if result.Left[2].LineNum != 3 || result.Right[2].LineNum != 3 {
		t.Errorf("row 2 line nums: left=%d right=%d, want 3,3",
			result.Left[2].LineNum, result.Right[2].LineNum)
	}
}

func TestFillerLineNumber(t *testing.T) {
	left := []string{"a"}
	right := []string{"a", "b"}
	result := ComputeDiff(left, right)

	// Filler line should have LineNum 0
	if result.Left[1].Type != DiffFiller {
		t.Fatalf("left[1].Type = %d, want DiffFiller", result.Left[1].Type)
	}
	if result.Left[1].LineNum != 0 {
		t.Errorf("filler line LineNum = %d, want 0", result.Left[1].LineNum)
	}
}
