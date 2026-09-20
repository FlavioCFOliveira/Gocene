// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

import "math"

// PassageScorer ranks passages found by UnifiedHighlighter using a
// BM25-flavoured formula. Each passage is scored as
//
//	norm * Σ ( weight * tf )
//
// where weight, tf, and norm are computed from the passage's match list
// and content length.
//
// Mirrors org.apache.lucene.search.uhighlight.PassageScorer.
type PassageScorer struct {
	k1    float32
	b     float32
	pivot float32
}

// NewPassageScorer returns a scorer with the Lucene defaults (k1=1.2,
// b=0.75, pivot=87) — 87 is the typical average English sentence length
// used by the Lucene reference.
func NewPassageScorer() *PassageScorer {
	return NewPassageScorerWith(1.2, 0.75, 87)
}

// NewPassageScorerWith returns a scorer with the supplied BM25 parameters.
func NewPassageScorerWith(k1, b, pivot float32) *PassageScorer {
	return &PassageScorer{k1: k1, b: b, pivot: pivot}
}

// K1 returns the BM25 k1 parameter (term-frequency saturation).
func (s *PassageScorer) K1() float32 { return s.k1 }

// B returns the BM25 b parameter (length normalisation).
func (s *PassageScorer) B() float32 { return s.b }

// Pivot returns the length-normalisation pivot.
func (s *PassageScorer) Pivot() float32 { return s.pivot }

// Weight computes the term importance term given its in-document
// statistics. numDocs is approximated from contentLength / pivot.
func (s *PassageScorer) Weight(contentLength, totalTermFreq int) float32 {
	numDocs := 1 + float32(contentLength)/s.pivot
	x := float64(1 + (float64(numDocs)+0.5)/(float64(totalTermFreq)+0.5))
	return (s.k1 + 1) * float32(math.Log(x))
}

// TF computes the term-frequency contribution given the in-passage
// frequency and the passage length.
func (s *PassageScorer) TF(freq, passageLen int) float32 {
	norm := s.k1 * ((1 - s.b) + s.b*(float32(passageLen)/s.pivot))
	return float32(freq) / (float32(freq) + norm)
}

// Norm computes the passage-position boost. Passages towards the
// beginning of the document are weighed more heavily by default.
func (s *PassageScorer) Norm(passageStart int) float32 {
	return 1 + 1/float32(math.Log(float64(s.pivot)+float64(passageStart)))
}

// Score computes the score of the given passage relative to the document
// length contentLength.
func (s *PassageScorer) Score(passage *Passage, contentLength int) float32 {
	if passage == nil || passage.NumMatches() == 0 {
		return 0
	}
	// We need to aggregate matches that share the same term text. Build a
	// small dedup table backed by string keys derived from the term bytes
	// so the BM25 sum mirrors the Lucene BytesRefHash-driven loop in
	// PassageScorer#score.
	hitCount := passage.NumMatches()
	termIndex := make(map[string]int, hitCount)
	termFreqsInPassage := make([]int, 0, hitCount)
	termFreqsInDoc := make([]int, 0, hitCount)

	terms := passage.MatchTerms()
	freqsInDoc := passage.MatchTermFreqsInDoc()
	for i := 0; i < hitCount; i++ {
		key := string(terms[i])
		idx, ok := termIndex[key]
		if !ok {
			idx = len(termFreqsInPassage)
			termIndex[key] = idx
			termFreqsInPassage = append(termFreqsInPassage, 0)
			termFreqsInDoc = append(termFreqsInDoc, freqsInDoc[i])
		}
		termFreqsInPassage[idx]++
	}

	var score float64
	passageLen := passage.Length()
	for i := range termFreqsInPassage {
		score += float64(s.TF(termFreqsInPassage[i], passageLen)) *
			float64(s.Weight(contentLength, termFreqsInDoc[i]))
	}
	score *= float64(s.Norm(passage.StartOffset()))
	return float32(score)
}
