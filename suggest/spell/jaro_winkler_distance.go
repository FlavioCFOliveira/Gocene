package spell

// JaroWinklerDistance is a similarity metric that favours strings sharing a
// common prefix. Mirrors org.apache.lucene.search.spell.JaroWinklerDistance.
type JaroWinklerDistance struct {
	// ScalingFactor weights the common-prefix bonus (Lucene default: 0.1).
	ScalingFactor float32
}

// NewJaroWinklerDistance builds the metric with the Lucene default scaling.
func NewJaroWinklerDistance() *JaroWinklerDistance {
	return &JaroWinklerDistance{ScalingFactor: 0.1}
}

// GetDistance returns the Jaro-Winkler similarity in [0, 1].
func (d *JaroWinklerDistance) GetDistance(s1, s2 string) float32 {
	r1, r2 := []rune(s1), []rune(s2)
	if len(r1) == 0 && len(r2) == 0 {
		return 1
	}
	if len(r1) == 0 || len(r2) == 0 {
		return 0
	}
	jaro := jaroSimilarity(r1, r2)
	if jaro < 0.7 {
		return jaro
	}
	commonPrefix := 0
	for i := 0; i < len(r1) && i < len(r2) && i < 4; i++ {
		if r1[i] != r2[i] {
			break
		}
		commonPrefix++
	}
	return jaro + float32(commonPrefix)*d.ScalingFactor*(1-jaro)
}

func jaroSimilarity(a, b []rune) float32 {
	matchDistance := max(len(a), len(b))/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}
	aMatches := make([]bool, len(a))
	bMatches := make([]bool, len(b))
	matches := 0
	for i := 0; i < len(a); i++ {
		start := i - matchDistance
		end := i + matchDistance + 1
		if start < 0 {
			start = 0
		}
		if end > len(b) {
			end = len(b)
		}
		for j := start; j < end; j++ {
			if bMatches[j] {
				continue
			}
			if a[i] != b[j] {
				continue
			}
			aMatches[i] = true
			bMatches[j] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}
	transpositions := 0
	k := 0
	for i := 0; i < len(a); i++ {
		if !aMatches[i] {
			continue
		}
		for !bMatches[k] {
			k++
		}
		if a[i] != b[k] {
			transpositions++
		}
		k++
	}
	transpositions /= 2
	m := float32(matches)
	return (m/float32(len(a)) + m/float32(len(b)) + (m-float32(transpositions))/m) / 3
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ StringDistance = (*JaroWinklerDistance)(nil)
