package uhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Passage represents a passage (typically a sentence of the document).
// A passage contains getNumMatches highlights from the query, and the offsets and query
// terms that correspond with each match.
type Passage struct {
	startOffset int
	endOffset   int
	score       float32

	matchStarts        []int
	matchEnds          []int
	matchTerms         []util.BytesRef
	matchTermFreqInDoc []int
	numMatches         int
}

// NewPassage creates a new Passage.
func NewPassage() *Passage {
	return &Passage{
		startOffset:        -1,
		endOffset:          -1,
		matchStarts:        make([]int, 8),
		matchEnds:          make([]int, 8),
		matchTerms:         make([]util.BytesRef, 8),
		matchTermFreqInDoc: make([]int, 8),
	}
}

// AddMatch adds a match to the passage.
func (p *Passage) AddMatch(startOffset, endOffset int, term util.BytesRef, termFreqInDoc int) {
	if p.numMatches == len(p.matchStarts) {
		newLen := grow(p.numMatches + 1)
		newMatchStarts := make([]int, newLen)
		newMatchEnds := make([]int, newLen)
		newMatchTermFreqInDoc := make([]int, newLen)
		newMatchTerms := make([]util.BytesRef, newLen)

		copy(newMatchStarts, p.matchStarts)
		copy(newMatchEnds, p.matchEnds)
		copy(newMatchTerms, p.matchTerms)
		copy(newMatchTermFreqInDoc, p.matchTermFreqInDoc)

		p.matchStarts = newMatchStarts
		p.matchEnds = newMatchEnds
		p.matchTerms = newMatchTerms
		p.matchTermFreqInDoc = newMatchTermFreqInDoc
	}

	p.matchStarts[p.numMatches] = startOffset
	p.matchEnds[p.numMatches] = endOffset
	p.matchTerms[p.numMatches] = term
	p.matchTermFreqInDoc[p.numMatches] = termFreqInDoc
	p.numMatches++
}

func grow(n int) int {
	if n <= 16 {
		return n + (n >> 1)
	}
	return n + (n >> 2)
}

// Reset resets the passage for reuse.
func (p *Passage) Reset() {
	p.startOffset = -1
	p.endOffset = -1
	p.score = 0.0
	p.numMatches = 0
}

func (p *Passage) String() string {
	res := fmt.Sprintf("Passage[%d-%d]{", p.startOffset, p.endOffset)
	for i := 0; i < p.numMatches; i++ {
		if i != 0 {
			res += ","
		}
		res += fmt.Sprintf("%s[%d-%d]", p.matchTerms[i].String(), p.matchStarts[i]-p.startOffset, p.matchEnds[i]-p.startOffset)
	}
	res += fmt.Sprintf("}score=%f", p.score)
	return res
}

func (p *Passage) StartOffset() int {
	return p.startOffset
}

func (p *Passage) EndOffset() int {
	return p.endOffset
}

func (p *Passage) Length() int {
	return p.endOffset - p.startOffset
}

func (p *Passage) Score() float32 {
	return p.score
}

func (p *Passage) SetScore(score float32) {
	p.score = score
}

func (p *Passage) NumMatches() int {
	return p.numMatches
}

func (p *Passage) MatchStarts() []int {
	return p.matchStarts
}

func (p *Passage) MatchEnds() []int {
	return p.matchEnds
}

func (p *Passage) MatchTerms() []util.BytesRef {
	return p.matchTerms
}

func (p *Passage) MatchTermFreqsInDoc() []int {
	return p.matchTermFreqInDoc
}

func (p *Passage) SetStartOffset(offset int) {
	p.startOffset = offset
}

func (p *Passage) SetEndOffset(offset int) {
	p.endOffset = offset
}
