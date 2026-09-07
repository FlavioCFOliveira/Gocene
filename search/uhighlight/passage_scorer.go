package uhighlight

import (
	"math"
)

// PassageScorer ranks passages found by UnifiedHighlighter.
// Each passage is scored as a miniature document within the document.
// The final score is computed as norm * sum(weight * tf).
type PassageScorer struct {
	k1    float32
	b     float32
	pivot float32
}

// NewPassageScorer creates a PassageScorer with default values:
// k1 = 1.2, b = 0.75, pivot = 87.
func NewPassageScorer() *PassageScorer {
	return &PassageScorer{
		k1:    1.2,
		b:     0.75,
		pivot: 87,
	}
}

// NewPassageScorerWithParams creates a PassageScorer with specified scoring parameters.
func NewPassageScorerWithParams(k1, b, pivot float32) *PassageScorer {
	return &PassageScorer{
		k1:    k1,
		b:     b,
		pivot: pivot,
	}
}

// Weight computes term importance, given its in-document statistics.
func (ps *PassageScorer) Weight(contentLength, totalTermFreq int) float32 {
	numDocs := 1.0 + float64(contentLength)/float64(ps.pivot)
	return float32((float64(ps.k1) + 1.0) * math.Log(1.0+(numDocs+0.5)/(float64(totalTermFreq)+0.5)))
}

// Tf computes term weight, given the frequency within the passage and the passage's length.
func (ps *PassageScorer) Tf(freq, passageLen int) float32 {
	norm := ps.k1 * ((1 - ps.b) + ps.b*(float32(passageLen)/ps.pivot))
	return float32(freq) / (float32(freq) + norm)
}

// Norm normalize a passage according to its position in the document.
func (ps *PassageScorer) Norm(passageStart int) float32 {
	return 1 + 1/float32(math.Log(float64(ps.pivot)+float64(passageStart)))
}

// Score computes the score for a passage.
func (ps *PassageScorer) Score(passage *Passage, contentLength int) float32 {
	var score float64

	// Map to track term frequency in the passage
	termFreqsInPassage := make(map[string]int)
	termFreqsInDoc := make(map[string]int)

	numMatches := passage.NumMatches()
	matchTerms := passage.MatchTerms()
	matchTermFreqsInDoc := passage.MatchTermFreqsInDoc()

	for i := 0; i < numMatches; i++ {
		term := matchTerms[i].String()
		termFreqsInPassage[term]++
		termFreqsInDoc[term] = matchTermFreqsInDoc[i]
	}

	for term, freq := range termFreqsInPassage {
		tf := ps.Tf(freq, passage.Length())
		weight := ps.Weight(contentLength, termFreqsInDoc[term])
		score += float64(tf * weight)
	}

	score *= float64(ps.Norm(passage.StartOffset()))
	return float32(score)
}
