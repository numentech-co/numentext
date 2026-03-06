package editor

import (
	"testing"
)

// === wrapLine tests ===

func TestWrapLine_EmptyLine(t *testing.T) {
	segs := wrapLine("", 80, 4)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].startCol != 0 || segs[0].endCol != 0 {
		t.Errorf("expected {0,0}, got {%d,%d}", segs[0].startCol, segs[0].endCol)
	}
}

func TestWrapLine_ShorterThanWidth(t *testing.T) {
	segs := wrapLine("hello", 80, 4)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].startCol != 0 || segs[0].endCol != 5 {
		t.Errorf("expected {0,5}, got {%d,%d}", segs[0].startCol, segs[0].endCol)
	}
}

func TestWrapLine_ExactWidth(t *testing.T) {
	segs := wrapLine("abcde", 5, 4)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].startCol != 0 || segs[0].endCol != 5 {
		t.Errorf("expected {0,5}, got {%d,%d}", segs[0].startCol, segs[0].endCol)
	}
}

func TestWrapLine_HardBreak(t *testing.T) {
	// No spaces or punctuation to break at
	segs := wrapLine("abcdefghij", 5, 4)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if segs[0].startCol != 0 || segs[0].endCol != 5 {
		t.Errorf("seg[0] expected {0,5}, got {%d,%d}", segs[0].startCol, segs[0].endCol)
	}
	if segs[1].startCol != 5 || segs[1].endCol != 10 {
		t.Errorf("seg[1] expected {5,10}, got {%d,%d}", segs[1].startCol, segs[1].endCol)
	}
}

func TestWrapLine_BreakAtSpace(t *testing.T) {
	segs := wrapLine("hello world", 8, 4)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	// Should break after the space at index 5 -> "hello " and "world"
	if segs[0].endCol != 6 {
		t.Errorf("seg[0] expected endCol=6, got %d", segs[0].endCol)
	}
	if segs[1].startCol != 6 {
		t.Errorf("seg[1] expected startCol=6, got %d", segs[1].startCol)
	}
}

func TestWrapLine_BreakAtPunctuation(t *testing.T) {
	segs := wrapLine("foo,bar,baz", 6, 4)
	if len(segs) < 2 {
		t.Fatalf("expected at least 2 segments, got %d", len(segs))
	}
	// Should break after comma at index 3 -> "foo," and remainder
	if segs[0].endCol != 4 {
		t.Errorf("seg[0] expected endCol=4, got %d", segs[0].endCol)
	}
}

func TestWrapLine_MultipleSegments(t *testing.T) {
	// 20 chars at width 5 with no break points = 4 segments
	segs := wrapLine("abcdefghijklmnopqrst", 5, 4)
	if len(segs) != 4 {
		t.Fatalf("expected 4 segments, got %d", len(segs))
	}
	for i, seg := range segs {
		expectedStart := i * 5
		expectedEnd := (i + 1) * 5
		if seg.startCol != expectedStart || seg.endCol != expectedEnd {
			t.Errorf("seg[%d] expected {%d,%d}, got {%d,%d}", i, expectedStart, expectedEnd, seg.startCol, seg.endCol)
		}
	}
}

func TestWrapLine_MaxWidth1(t *testing.T) {
	segs := wrapLine("abc", 1, 4)
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	for i := range segs {
		if segs[i].startCol != i || segs[i].endCol != i+1 {
			t.Errorf("seg[%d] expected {%d,%d}, got {%d,%d}", i, i, i+1, segs[i].startCol, segs[i].endCol)
		}
	}
}

func TestWrapLine_MaxWidth0(t *testing.T) {
	// maxWidth=0 should be clamped to 1
	segs := wrapLine("ab", 0, 4)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
}

func TestWrapLine_Unicode(t *testing.T) {
	// "Hello" in Japanese: 5 runes but more bytes
	line := "\xe4\xb8\x96\xe7\x95\x8c" // "世界" - 2 runes, 6 bytes
	segs := wrapLine(line, 3, 4)
	// 2 runes fits in width 3, should be 1 segment
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment for 2-rune line at width 3, got %d", len(segs))
	}
	if segs[0].startCol != 0 || segs[0].endCol != 6 {
		t.Errorf("expected {0,6}, got {%d,%d}", segs[0].startCol, segs[0].endCol)
	}
}

func TestWrapLine_UnicodeWrap(t *testing.T) {
	// 4 runes at width 2: should wrap into 2 segments
	line := "\xe4\xb8\x96\xe7\x95\x8c\xe5\x85\x83\xe6\xb0\x97" // "世界元気" - 4 runes, 12 bytes
	segs := wrapLine(line, 2, 4)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	// First 2 runes = 6 bytes, second 2 runes = 6 bytes
	if segs[0].startCol != 0 || segs[0].endCol != 6 {
		t.Errorf("seg[0] expected {0,6}, got {%d,%d}", segs[0].startCol, segs[0].endCol)
	}
	if segs[1].startCol != 6 || segs[1].endCol != 12 {
		t.Errorf("seg[1] expected {6,12}, got {%d,%d}", segs[1].startCol, segs[1].endCol)
	}
}

func TestWrapLine_TabExpansion(t *testing.T) {
	// Tab at start expands to 4 cols (tabSize=4), then "ab" = 6 visual cols
	segs := wrapLine("\tab", 5, 4)
	// Visual width: tab=4 + 'a'=1 + 'b'=1 = 6, which exceeds 5, so wraps
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments for tab+ab at width 5, got %d", len(segs))
	}
}

// === Segment contiguity ===

func TestWrapLine_SegmentContiguity(t *testing.T) {
	line := "the quick brown fox jumps"
	segs := wrapLine(line, 10, 4)
	for i := 1; i < len(segs); i++ {
		if segs[i].startCol != segs[i-1].endCol {
			t.Errorf("gap between seg[%d].endCol=%d and seg[%d].startCol=%d",
				i-1, segs[i-1].endCol, i, segs[i].startCol)
		}
	}
	// Last segment must end at len(line)
	if segs[len(segs)-1].endCol != len(line) {
		t.Errorf("last segment endCol=%d, expected %d", segs[len(segs)-1].endCol, len(line))
	}
}

// === byteOffsetToScreenCol tests ===

func TestByteOffsetToScreenCol_ASCII(t *testing.T) {
	col := byteOffsetToScreenCol("hello", 3, 4)
	if col != 3 {
		t.Errorf("expected 3, got %d", col)
	}
}

func TestByteOffsetToScreenCol_Unicode(t *testing.T) {
	line := "\xe4\xb8\x96\xe7\x95\x8c" // "世界" - byte offsets: 0,3
	// Byte offset 3 should be screen col 1 (after first rune)
	col := byteOffsetToScreenCol(line, 3, 4)
	if col != 1 {
		t.Errorf("expected 1, got %d", col)
	}
	// Byte offset 6 (end) should be screen col 2
	col = byteOffsetToScreenCol(line, 6, 4)
	if col != 2 {
		t.Errorf("expected 2, got %d", col)
	}
}

func TestByteOffsetToScreenCol_Tab(t *testing.T) {
	// "\thello" with tabSize=4: tab expands to 4 cols
	col := byteOffsetToScreenCol("\thello", 1, 4)
	if col != 4 {
		t.Errorf("expected 4, got %d", col)
	}
	// Tab after 2 chars: "ab\t" -> tabSize=4, tab expands to 2 cols (4-2)
	col = byteOffsetToScreenCol("ab\t", 3, 4)
	if col != 4 {
		t.Errorf("expected 4, got %d", col)
	}
}

// === Cursor edge cases ===

func TestWrapLine_CursorAtStart(t *testing.T) {
	segs := wrapLine("hello world", 8, 4)
	// Cursor col 0 should be in first segment
	if len(segs) == 0 {
		t.Fatal("no segments")
	}
	if 0 < segs[0].startCol || 0 > segs[0].endCol {
		t.Errorf("cursor col 0 not in first segment {%d,%d}", segs[0].startCol, segs[0].endCol)
	}
}

func TestWrapLine_CursorAtEnd(t *testing.T) {
	line := "hello world"
	segs := wrapLine(line, 8, 4)
	lastSeg := segs[len(segs)-1]
	if len(line) < lastSeg.startCol || len(line) > lastSeg.endCol {
		t.Errorf("cursor at end (%d) not in last segment {%d,%d}", len(line), lastSeg.startCol, lastSeg.endCol)
	}
}

func TestWrapLine_CursorAtExactBoundary(t *testing.T) {
	// 10 chars, width 5: boundary at col 5
	segs := wrapLine("abcdefghij", 5, 4)
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	// Col 5 should be the start of segment 1
	if segs[1].startCol != 5 {
		t.Errorf("expected seg[1].startCol=5, got %d", segs[1].startCol)
	}
}
