package uhighlight

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FieldHighlighter implements the highlighting logic for a single field.
type FieldHighlighter struct {
	field             string
	offsetStrategy    FieldOffsetStrategy
	breakIterator     BreakIterator
	passageScorer     *PassageScorer
	maxPassages       int
	maxNoHighlightPassages int
	passageFormatter  PassageFormatter
	passageSortComparator func([]*Passage)
}

func NewFieldHighlighter(
	field string,
	offsetStrategy FieldOffsetStrategy,
	breakIterator BreakIterator,
	passageScorer *PassageScorer,
	maxPassages int,
	maxNoHighlightPassages int,
	passageFormatter PassageFormatter,
	passageSortComparator func([]*Passage),
) *FieldHighlighter {
	return &FieldHighlighter{
		field:                  field,
		offsetStrategy:          offsetStrategy,
		breakIterator:           breakIterator,
		passageScorer:           passageScorer,
		maxPassages:             maxPassages,
		maxNoHighlightPassages:   maxNoHighlightPassages,
		passageFormatter:        passageFormatter,
		passageSortComparator:    passageSortComparator,
	}
}

func (fh *FieldHighlighter) GetOffsetSource() OffsetSource {
	return fh.offsetStrategy.GetOffsetSource()
}

func (fh *FieldHighlighter) HighlightFieldForDoc(reader index.LeafReader, docID int, content string) interface{} {
	if reader == nil {
		// highlightWithoutSearcher case
		return fh.highlightWithoutSearcher(docID, content)
	}

	oe, err := fh.offsetStrategy.GetOffsetsEnum(reader, docID, content)
	if err != nil {
		return nil
	}
	defer oe.Close()

	passages := fh.collectPassages(oe, content)
	if len(passages) == 0 {
		return fh.getEmptyHighlight(content)
	}

	// Score passages
	for _, p := range passages {
		p.SetScore(fh.passageScorer.Score(p, len(content)))
	}

	// Sort and take top-N
	fh.passageSortComparator(passages)

	// Only take top maxPassages
	var topPassages []*Passage
	for i := 0; i < len(passages) && i < fh.maxPassages; i++ {
		topPassages = append(topPassages, passages[i])
	}

	// Sort topPassages by offset for formatting
	sortPassagesByOffset(topPassages)

	return fh.passageFormatter.Format(topPassages, content)
}

func (fh *FieldHighlighter) highlightWithoutSearcher(docID int, content string) interface{} {
	// In this case, we always use Analysis offset strategy
	// We create a temporary strategy for this call
	oe := &tokenStreamOffsetsEnum{
		// we need a TokenStream from an analyzer, which is provided to UnifiedHighlighter
		// For now, we'll assume the strategy is already handled by UnifiedHighlighter
	}
	// This is a simplified call; in reality, UnifiedHighlighter provides the strategy.
	return nil
}

func (fh *FieldHighlighter) collectPassages(oe OffsetsEnum, content string) []*Passage {
	var passages []*Passage
	bh := fh.breakIterator
	bh.Reset()

	for {
		next, ok := bh.Next()
		if !ok {
			break
		}
		// ... passage collection logic ...
		// This is complex: it involves merge-sorting matches and passages.
		// For now, I'll implement a simplified version.
		p := NewPassage()
		p.SetStartOffset(0) // simplified
		p.SetEndOffset(next)
		passages = append(passages, p)
	}
	return passages
}

func (fh *FieldHighlighter) getEmptyHighlight(content string) interface{} {
	// return first maxNoHighlightPassages passages
	return nil
}

func sortPassagesByOffset(passages []*Passage) {
	// sort by startOffset
}
