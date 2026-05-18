package spell

// NGramDistance computes similarity from the overlap of character n-grams.
// Mirrors org.apache.lucene.search.spell.NGramDistance.
type NGramDistance struct {
	N int
}

// NewNGramDistance builds the metric with the supplied n-gram size
// (default 2 when n <= 0).
func NewNGramDistance(n int) *NGramDistance {
	if n <= 0 {
		n = 2
	}
	return &NGramDistance{N: n}
}

// GetDistance returns the Jaccard similarity over the multiset of n-grams.
func (d *NGramDistance) GetDistance(s1, s2 string) float32 {
	if s1 == s2 {
		return 1
	}
	if s1 == "" || s2 == "" {
		return 0
	}
	a := ngrams(s1, d.N)
	b := ngrams(s2, d.N)
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	intersection := 0
	for k, ca := range a {
		if cb, ok := b[k]; ok {
			if cb < ca {
				intersection += cb
			} else {
				intersection += ca
			}
		}
	}
	union := 0
	for _, c := range a {
		union += c
	}
	for k, c := range b {
		if _, ok := a[k]; !ok {
			union += c
		} else {
			// add the difference (b's surplus over a)
			diff := c - a[k]
			if diff > 0 {
				union += diff
			}
		}
	}
	if union == 0 {
		return 0
	}
	return float32(intersection) / float32(union)
}

func ngrams(s string, n int) map[string]int {
	r := []rune(s)
	out := make(map[string]int)
	if len(r) < n {
		out[string(r)] = 1
		return out
	}
	for i := 0; i <= len(r)-n; i++ {
		key := string(r[i : i+n])
		out[key]++
	}
	return out
}

var _ StringDistance = (*NGramDistance)(nil)
