package mergeview

// WordDiff represents a word-level diff result.
type WordDiff struct {
	Text      string
	InLocal   bool // present in LOCAL
	InRemote  bool // present in REMOTE
	IsCommon  bool // present in both (unchanged)
}

// ComputeWordDiffs computes word-level diffs between two lines.
// Returns two slices of WordDiff: one for local, one for remote,
// where each word is annotated as common or unique.
func ComputeWordDiffs(localLine, remoteLine string) ([]WordDiff, []WordDiff) {
	localWords := splitWords(localLine)
	remoteWords := splitWords(remoteLine)

	lcs := computeLCS(localWords, remoteWords)

	localDiffs := annotateWords(localWords, lcs, true)
	remoteDiffs := annotateWords(remoteWords, lcs, false)

	return localDiffs, remoteDiffs
}

// splitWords splits a line into words, preserving whitespace as separate tokens.
func splitWords(line string) []string {
	var words []string
	current := []byte{}
	inSpace := false

	for i := 0; i < len(line); i++ {
		ch := line[i]
		isSpace := ch == ' ' || ch == '\t'

		if i == 0 {
			inSpace = isSpace
			current = append(current, ch)
			continue
		}

		if isSpace != inSpace {
			if len(current) > 0 {
				words = append(words, string(current))
			}
			current = []byte{ch}
			inSpace = isSpace
		} else {
			if !isSpace {
				// Non-space chars: split each "word" by space boundaries
				current = append(current, ch)
			} else {
				current = append(current, ch)
			}
		}
	}
	if len(current) > 0 {
		words = append(words, string(current))
	}
	return words
}

// computeLCS finds the longest common subsequence of two word slices.
// Returns the set of common words in order.
func computeLCS(a, b []string) []string {
	m := len(a)
	n := len(b)

	// Build DP table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	// Backtrack to find the LCS
	lcs := make([]string, dp[m][n])
	idx := dp[m][n] - 1
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs[idx] = a[i-1]
			idx--
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return lcs
}

// annotateWords marks each word as common (in LCS) or unique.
func annotateWords(words []string, lcs []string, isLocal bool) []WordDiff {
	diffs := make([]WordDiff, len(words))
	lcsIdx := 0

	for i, w := range words {
		if lcsIdx < len(lcs) && w == lcs[lcsIdx] {
			diffs[i] = WordDiff{Text: w, IsCommon: true, InLocal: true, InRemote: true}
			lcsIdx++
		} else {
			diffs[i] = WordDiff{Text: w, InLocal: isLocal, InRemote: !isLocal}
		}
	}

	return diffs
}

// LineDiff computes line-level diff between two sets of lines.
// Returns which lines are common and which are unique to each side.
type LineDiffResult struct {
	LocalOnly  map[int]bool // line indices unique to local
	RemoteOnly map[int]bool // line indices unique to remote
	CommonMap  map[int]int  // local line idx -> remote line idx for common lines
}

// ComputeLineDiff finds common and unique lines between two slices.
func ComputeLineDiff(local, remote []string) LineDiffResult {
	m := len(local)
	n := len(remote)

	// LCS on lines
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if local[i-1] == remote[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack
	commonMap := make(map[int]int)
	i, j := m, n
	for i > 0 && j > 0 {
		if local[i-1] == remote[j-1] {
			commonMap[i-1] = j-1
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	localOnly := make(map[int]bool)
	for idx := 0; idx < m; idx++ {
		if _, ok := commonMap[idx]; !ok {
			localOnly[idx] = true
		}
	}

	// Build reverse map to find remote-only
	remoteCommon := make(map[int]bool)
	for _, rIdx := range commonMap {
		remoteCommon[rIdx] = true
	}
	remoteOnly := make(map[int]bool)
	for idx := 0; idx < n; idx++ {
		if !remoteCommon[idx] {
			remoteOnly[idx] = true
		}
	}

	return LineDiffResult{
		LocalOnly:  localOnly,
		RemoteOnly: remoteOnly,
		CommonMap:  commonMap,
	}
}
