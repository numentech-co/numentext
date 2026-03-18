package diffview

// diff.go contains the diff computation algorithms:
// - Line-level diff using LCS (Longest Common Subsequence)
// - Word-level diff for modified lines
// - Line alignment with filler lines for synchronized scrolling

// DiffLineType categorizes a line in the diff.
type DiffLineType int

const (
	DiffNormal   DiffLineType = iota // Unchanged line
	DiffAdded                        // Line exists only in the right (working) version
	DiffDeleted                      // Line exists only in the left (committed) version
	DiffModified                     // Line exists in both but differs
	DiffFiller                       // Blank filler line for alignment
)

// WordDiff marks a span of text within a modified line that changed.
type WordDiff struct {
	Start int // byte offset in the line text
	End   int // byte offset (exclusive)
	Type  DiffLineType
}

// DiffLine represents a single line in the aligned diff output.
type DiffLine struct {
	Text      string
	LineNum   int          // original line number (1-based), 0 for filler lines
	Type      DiffLineType
	WordDiffs []WordDiff // word-level diffs for Modified lines
}

// DiffResult holds the complete aligned diff between two files.
type DiffResult struct {
	Left  []DiffLine
	Right []DiffLine
}

// ComputeDiff computes a side-by-side diff between the committed (left) and
// working (right) versions of a file. It returns aligned DiffLine slices with
// filler lines inserted so that matching lines appear at the same row index.
func ComputeDiff(leftLines, rightLines []string) DiffResult {
	// Compute LCS to identify matching lines
	lcs := computeLCS(leftLines, rightLines)

	var left []DiffLine
	var right []DiffLine

	li, ri, lcsIdx := 0, 0, 0

	for li < len(leftLines) || ri < len(rightLines) {
		if lcsIdx < len(lcs) && li < len(leftLines) && ri < len(rightLines) &&
			leftLines[li] == lcs[lcsIdx] && rightLines[ri] == lcs[lcsIdx] {
			// Both match the LCS: normal line
			left = append(left, DiffLine{
				Text:    leftLines[li],
				LineNum: li + 1,
				Type:    DiffNormal,
			})
			right = append(right, DiffLine{
				Text:    rightLines[ri],
				LineNum: ri + 1,
				Type:    DiffNormal,
			})
			li++
			ri++
			lcsIdx++
		} else {
			// Consume non-matching lines from both sides
			// Collect contiguous deleted lines from left and added lines from right
			var deletedChunk []int
			var addedChunk []int

			for li < len(leftLines) && (lcsIdx >= len(lcs) || leftLines[li] != lcs[lcsIdx]) {
				deletedChunk = append(deletedChunk, li)
				li++
			}
			for ri < len(rightLines) && (lcsIdx >= len(lcs) || rightLines[ri] != lcs[lcsIdx]) {
				addedChunk = append(addedChunk, ri)
				ri++
			}

			// Pair up lines that exist on both sides as "modified"
			paired := min(len(deletedChunk), len(addedChunk))
			for i := 0; i < paired; i++ {
				lIdx := deletedChunk[i]
				rIdx := addedChunk[i]
				wordDiffsL, wordDiffsR := computeWordDiff(leftLines[lIdx], rightLines[rIdx])
				left = append(left, DiffLine{
					Text:      leftLines[lIdx],
					LineNum:   lIdx + 1,
					Type:      DiffModified,
					WordDiffs: wordDiffsL,
				})
				right = append(right, DiffLine{
					Text:      rightLines[rIdx],
					LineNum:   rIdx + 1,
					Type:      DiffModified,
					WordDiffs: wordDiffsR,
				})
			}

			// Remaining deleted lines (left only) with filler on right
			for i := paired; i < len(deletedChunk); i++ {
				lIdx := deletedChunk[i]
				left = append(left, DiffLine{
					Text:    leftLines[lIdx],
					LineNum: lIdx + 1,
					Type:    DiffDeleted,
				})
				right = append(right, DiffLine{
					Type: DiffFiller,
				})
			}

			// Remaining added lines (right only) with filler on left
			for i := paired; i < len(addedChunk); i++ {
				rIdx := addedChunk[i]
				left = append(left, DiffLine{
					Type: DiffFiller,
				})
				right = append(right, DiffLine{
					Text:    rightLines[rIdx],
					LineNum: rIdx + 1,
					Type:    DiffAdded,
				})
			}
		}
	}

	return DiffResult{Left: left, Right: right}
}

// computeLCS computes the Longest Common Subsequence of two string slices.
func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)
	// Build DP table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to get the LCS
	lcsLen := dp[m][n]
	lcs := make([]string, lcsLen)
	i, j := m, n
	k := lcsLen - 1
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs[k] = a[i-1]
			i--
			j--
			k--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return lcs
}

// computeWordDiff splits two lines into words and computes word-level diffs.
// Returns WordDiff slices for the left and right lines respectively.
func computeWordDiff(leftText, rightText string) ([]WordDiff, []WordDiff) {
	leftWords := splitWords(leftText)
	rightWords := splitWords(rightText)

	// Compute LCS of words
	lcsWords := computeWordLCS(leftWords, rightWords)

	// Mark non-LCS words as changed
	leftDiffs := markWordDiffs(leftText, leftWords, lcsWords, DiffDeleted)
	rightDiffs := markWordDiffs(rightText, rightWords, lcsWords, DiffAdded)

	return leftDiffs, rightDiffs
}

// wordSpan represents a word and its position in the original string.
type wordSpan struct {
	text  string
	start int // byte offset in original line
	end   int // byte offset (exclusive)
}

// splitWords splits a line into words, preserving their byte offsets.
// Words are separated by whitespace.
func splitWords(s string) []wordSpan {
	var words []wordSpan
	i := 0
	for i < len(s) {
		// Skip whitespace
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		// Consume word
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		words = append(words, wordSpan{
			text:  s[start:i],
			start: start,
			end:   i,
		})
	}
	return words
}

// computeWordLCS computes the LCS of two word slices.
func computeWordLCS(a, b []wordSpan) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1].text == b[j-1].text {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	lcsLen := dp[m][n]
	lcs := make([]string, lcsLen)
	i, j := m, n
	k := lcsLen - 1
	for i > 0 && j > 0 {
		if a[i-1].text == b[j-1].text {
			lcs[k] = a[i-1].text
			i--
			j--
			k--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return lcs
}

// markWordDiffs marks words that are not in the LCS as changed.
func markWordDiffs(lineText string, words []wordSpan, lcs []string, diffType DiffLineType) []WordDiff {
	var diffs []WordDiff
	lcsIdx := 0
	for _, w := range words {
		if lcsIdx < len(lcs) && w.text == lcs[lcsIdx] {
			lcsIdx++
			continue
		}
		diffs = append(diffs, WordDiff{
			Start: w.start,
			End:   w.end,
			Type:  diffType,
		})
	}
	return diffs
}
