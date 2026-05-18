package spell

// LevenshteinDistance is the classic edit-distance metric normalised to
// [0, 1]. Mirrors org.apache.lucene.search.spell.LevenshteinDistance.
type LevenshteinDistance struct{}

// GetDistance returns 1 - editDistance/max(len(s1), len(s2)).
func (LevenshteinDistance) GetDistance(s1, s2 string) float32 {
	r1, r2 := []rune(s1), []rune(s2)
	if len(r1) == 0 && len(r2) == 0 {
		return 1
	}
	if d := editDistance(r1, r2); d == 0 {
		return 1
	} else {
		maxLen := len(r1)
		if len(r2) > maxLen {
			maxLen = len(r2)
		}
		return 1 - float32(d)/float32(maxLen)
	}
}

func editDistance(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

var _ StringDistance = LevenshteinDistance{}

// LuceneLevenshteinDistance applies the optimal-string-alignment variant used
// inside FuzzyQuery: like Levenshtein but also penalises swaps of adjacent
// characters (Damerau-Levenshtein restricted). Mirrors
// org.apache.lucene.search.spell.LuceneLevenshteinDistance.
type LuceneLevenshteinDistance struct{}

// GetDistance returns 1 - osaDistance / max(len(s1), len(s2)).
func (LuceneLevenshteinDistance) GetDistance(s1, s2 string) float32 {
	r1, r2 := []rune(s1), []rune(s2)
	if len(r1) == 0 && len(r2) == 0 {
		return 1
	}
	d := osaDistance(r1, r2)
	maxLen := len(r1)
	if len(r2) > maxLen {
		maxLen = len(r2)
	}
	return 1 - float32(d)/float32(maxLen)
}

func osaDistance(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := 0; i <= len(a); i++ {
		dp[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		dp[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			dp[i][j] = min3(dp[i-1][j]+1, dp[i][j-1]+1, dp[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				if dp[i-2][j-2]+1 < dp[i][j] {
					dp[i][j] = dp[i-2][j-2] + 1
				}
			}
		}
	}
	return dp[len(a)][len(b)]
}

var _ StringDistance = LuceneLevenshteinDistance{}
