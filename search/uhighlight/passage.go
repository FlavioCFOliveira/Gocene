// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

import (
	"fmt"
	"strings"
)

// Passage represents a single highlight passage — typically a sentence of
// the document — together with the term matches inside it that the
// PassageScorer should rank.
//
// Mirrors org.apache.lucene.search.uhighlight.Passage.
type Passage struct {
	startOffset int
	endOffset   int
	score       float32

	matchStarts        []int
	matchEnds          []int
	matchTerms         [][]byte
	matchTermFreqInDoc []int
	numMatches         int
}

// NewPassage builds an empty passage with offsets reset to -1 and zero
// matches. The match arrays are pre-allocated to 8 entries to match the
// Lucene reference.
func NewPassage() *Passage {
	return &Passage{
		startOffset:        -1,
		endOffset:          -1,
		matchStarts:        make([]int, 8),
		matchEnds:          make([]int, 8),
		matchTerms:         make([][]byte, 8),
		matchTermFreqInDoc: make([]int, 8),
	}
}

// AddMatch appends a term match to the passage. The Lucene reference
// enforces start >= passage.startOffset && start <= passage.endOffset via
// assert; the Go port enforces it via a defensive check that mirrors the
// runtime contract.
func (p *Passage) AddMatch(startOffset, endOffset int, term []byte, termFreqInDoc int) {
	if startOffset < p.startOffset || startOffset > p.endOffset {
		panic(fmt.Sprintf(
			"uhighlight: passage AddMatch out of range: start=%d, end=%d, passage=[%d,%d]",
			startOffset, endOffset, p.startOffset, p.endOffset))
	}
	if p.numMatches == len(p.matchStarts) {
		newLen := oversizeNumMatches(p.numMatches + 1)
		newStarts := make([]int, newLen)
		newEnds := make([]int, newLen)
		newTerms := make([][]byte, newLen)
		newFreqs := make([]int, newLen)
		copy(newStarts, p.matchStarts[:p.numMatches])
		copy(newEnds, p.matchEnds[:p.numMatches])
		copy(newTerms, p.matchTerms[:p.numMatches])
		copy(newFreqs, p.matchTermFreqInDoc[:p.numMatches])
		p.matchStarts = newStarts
		p.matchEnds = newEnds
		p.matchTerms = newTerms
		p.matchTermFreqInDoc = newFreqs
	}
	p.matchStarts[p.numMatches] = startOffset
	p.matchEnds[p.numMatches] = endOffset
	p.matchTerms[p.numMatches] = term
	p.matchTermFreqInDoc[p.numMatches] = termFreqInDoc
	p.numMatches++
}

// Reset clears the passage so it can be re-used by FieldHighlighter.
func (p *Passage) Reset() {
	p.startOffset = -1
	p.endOffset = -1
	p.score = 0
	p.numMatches = 0
}

// String returns a debug representation of the passage in the same form as
// the Lucene reference: Passage[0-22]{yin[0-3],yang[4-8]}score=2.4964213.
func (p *Passage) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Passage[%d-%d]{", p.startOffset, p.endOffset)
	for i := 0; i < p.numMatches; i++ {
		if i != 0 {
			sb.WriteByte(',')
		}
		sb.Write(p.matchTerms[i])
		fmt.Fprintf(&sb, "[%d-%d]", p.matchStarts[i]-p.startOffset, p.matchEnds[i]-p.startOffset)
	}
	fmt.Fprintf(&sb, "}score=%v", p.score)
	return sb.String()
}

// StartOffset returns the inclusive start offset of the passage in the
// original content.
func (p *Passage) StartOffset() int { return p.startOffset }

// EndOffset returns the exclusive end offset of the passage in the
// original content.
func (p *Passage) EndOffset() int { return p.endOffset }

// Length returns the passage length in characters (endOffset - startOffset).
func (p *Passage) Length() int { return p.endOffset - p.startOffset }

// Score returns the score assigned by the PassageScorer.
func (p *Passage) Score() float32 { return p.score }

// SetScore sets the passage score.
func (p *Passage) SetScore(score float32) { p.score = score }

// NumMatches returns the number of term matches stored in this passage.
func (p *Passage) NumMatches() int { return p.numMatches }

// MatchStarts returns the underlying match-start array. Only the first
// NumMatches entries are valid. Offsets are absolute (not relative to
// StartOffset).
func (p *Passage) MatchStarts() []int { return p.matchStarts }

// MatchEnds returns the underlying match-end array. Only the first
// NumMatches entries are valid.
func (p *Passage) MatchEnds() []int { return p.matchEnds }

// MatchTerms returns the underlying match-term array. Only the first
// NumMatches entries are valid.
func (p *Passage) MatchTerms() [][]byte { return p.matchTerms }

// MatchTermFreqsInDoc returns the underlying per-term doc-frequency array.
// Only the first NumMatches entries are valid.
func (p *Passage) MatchTermFreqsInDoc() []int { return p.matchTermFreqInDoc }

// SetStartOffset sets the inclusive start offset. Internal helper used by
// FieldHighlighter when growing a passage.
func (p *Passage) SetStartOffset(start int) { p.startOffset = start }

// SetEndOffset sets the exclusive end offset. Mirrors the Lucene
// reference invariant startOffset <= endOffset.
func (p *Passage) SetEndOffset(end int) {
	if end < p.startOffset {
		panic(fmt.Sprintf(
			"uhighlight: passage SetEndOffset(%d) < startOffset(%d)", end, p.startOffset))
	}
	p.endOffset = end
}

// oversizeNumMatches mirrors ArrayUtil.oversize(numMatches+1, NUM_BYTES_OBJECT_REF).
// For the typical small-N case the growth factor is roughly 1.25x with a
// minimum step of 4 entries, matching the Lucene array-grow heuristic.
func oversizeNumMatches(min int) int {
	if min < 8 {
		return 8
	}
	// Lucene's ArrayUtil.oversize with NUM_BYTES_OBJECT_REF == 4 grows by
	// roughly 7/8 + min; we use the same 1/8-step formula.
	grow := min + (min >> 3) + 3
	// Round up to the next multiple of 4 for cache friendliness, matching
	// the Lucene array-grow alignment.
	return (grow + 3) &^ 3
}
