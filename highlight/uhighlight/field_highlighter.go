// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

import (
	"container/heap"
	"errors"
	"fmt"
	"sort"
)

// FieldHighlighter is the per-field workhorse that walks the offsets-enum
// stream against the content, builds passages between break-iterator
// boundaries, scores them, and renders the top-K through the
// PassageFormatter.
//
// Mirrors org.apache.lucene.search.uhighlight.FieldHighlighter.
type FieldHighlighter struct {
	field                  string
	offsetStrategy         FieldOffsetStrategy
	breakIterator          BreakIterator
	passageScorer          *PassageScorer
	maxPassages            int
	maxNoHighlightPassages int
	passageFormatter       PassageFormatter
}

// NewFieldHighlighter builds the highlighter. maxPassages controls the
// top-K hit selection; maxNoHighlightPassages controls how many sentences
// to surface when no hits exist (a value of -1 means "use maxPassages").
func NewFieldHighlighter(
	field string,
	strategy FieldOffsetStrategy,
	breakIter BreakIterator,
	scorer *PassageScorer,
	maxPassages, maxNoHighlightPassages int,
	formatter PassageFormatter,
) *FieldHighlighter {
	if maxPassages < 1 {
		maxPassages = 1
	}
	if scorer == nil {
		scorer = NewPassageScorer()
	}
	if formatter == nil {
		formatter = NewDefaultPassageFormatter()
	}
	if breakIter == nil {
		breakIter = SplittingBreakIterator{}
	}
	return &FieldHighlighter{
		field:                  field,
		offsetStrategy:         strategy,
		breakIterator:          breakIter,
		passageScorer:          scorer,
		maxPassages:            maxPassages,
		maxNoHighlightPassages: maxNoHighlightPassages,
		passageFormatter:       formatter,
	}
}

// Field returns the target field name.
func (h *FieldHighlighter) Field() string { return h.field }

// OffsetSource returns the OffsetSource used by the wrapped strategy.
func (h *FieldHighlighter) OffsetSource() OffsetSource {
	return h.offsetStrategy.GetOffsetSource()
}

// HighlightFieldForDoc is the primary entry-point: highlight a single
// document for a single field against the supplied content. Returns the
// formatted snippet string. If content is empty, returns "" with no
// error. If no matches are produced, returns the no-highlight summary
// (first maxNoHighlightPassages sentences).
//
// docContext is opaque payload understood by the wrapped
// FieldOffsetStrategy (e.g. an *AnalysisDocContext for the analysis
// strategy or a *TermVectorDocContext for the term-vector strategy).
func (h *FieldHighlighter) HighlightFieldForDoc(docContext any, content string) (string, error) {
	if len(content) == 0 {
		return "", nil
	}
	enum, err := h.offsetStrategy.GetOffsetsEnum(docContext)
	if err != nil {
		return "", err
	}
	defer enum.Close()

	passages, err := h.highlightOffsetsEnum(enum, content)
	if err != nil {
		return "", err
	}
	if len(passages) == 0 {
		maxNoHL := h.maxNoHighlightPassages
		if maxNoHL == -1 {
			maxNoHL = h.maxPassages
		}
		passages = h.summaryPassagesNoHighlight(content, maxNoHL)
	}
	if len(passages) == 0 {
		return "", nil
	}
	return h.passageFormatter.Format(passages, content), nil
}

// summaryPassagesNoHighlight returns the first maxPassages segments of
// content sliced by the break iterator. Mirrors
// FieldHighlighter#getSummaryPassagesNoHighlight.
func (h *FieldHighlighter) summaryPassagesNoHighlight(content string, maxPassages int) []*Passage {
	if maxPassages <= 0 {
		return nil
	}
	passages := make([]*Passage, 0, maxPassages)
	pos := 0
	for len(passages) < maxPassages {
		next := h.breakIterator.Following(content, pos)
		if next < 0 {
			break
		}
		p := NewPassage()
		p.SetStartOffset(pos)
		p.SetEndOffset(next)
		passages = append(passages, p)
		pos = next
	}
	return passages
}

// highlightOffsetsEnum walks the offsets-enum stream, segments the
// document into passages around the break iterator, scores them and
// returns the top-K in document order.
//
// Mirrors FieldHighlighter#highlightOffsetsEnums.
func (h *FieldHighlighter) highlightOffsetsEnum(enum OffsetsEnum, content string) ([]*Passage, error) {
	contentLength := len(content)
	if !enum.Next() {
		return nil, nil
	}
	pq := &passagePQ{}
	heap.Init(pq)

	passage := NewPassage()
	lastPassageEnd := 0

	for {
		start := enum.StartOffset()
		if start == -1 {
			return nil, fmt.Errorf("uhighlight: field %q was indexed without offsets, cannot highlight", h.field)
		}
		end := enum.EndOffset()
		// Skip matches that span past the content boundary (the Lucene
		// reference uses a `continue` here).
		if start < contentLength && end > contentLength {
			if !enum.Next() {
				break
			}
			continue
		}
		// If this term sits outside the current passage, close the
		// passage and open a new one.
		if start >= passage.EndOffset() {
			passage = h.maybeAddPassage(pq, passage, contentLength)
			if start >= contentLength {
				break
			}
			center := start + (end-start)/2
			// Lucene: passage.setStartOffset(
			//   min(start, max(preceding(max(start+1, center)), lastPassageEnd)));
			precedingFrom := start + 1
			if center > precedingFrom {
				precedingFrom = center
			}
			passStart := h.breakIterator.Preceding(content, precedingFrom)
			if passStart < lastPassageEnd {
				passStart = lastPassageEnd
			}
			if passStart > start {
				passStart = start
			}
			passage.SetStartOffset(passStart)
			// Lucene: lastPassageEnd = max(end,
			//   min(following(min(end-1, center)), contentLength));
			followingFrom := end - 1
			if center < followingFrom {
				followingFrom = center
			}
			passEnd := h.breakIterator.Following(content, followingFrom)
			if passEnd < 0 || passEnd > contentLength {
				passEnd = contentLength
			}
			if passEnd < end {
				passEnd = end
			}
			lastPassageEnd = passEnd
			passage.SetEndOffset(passEnd)
		}
		// Append this match to the active passage.
		term := []byte(enum.Term())
		passage.AddMatch(start, end, term, enum.FreqIndex())
		if !enum.Next() {
			break
		}
	}
	h.maybeAddPassage(pq, passage, contentLength)

	// Drain the PQ and sort by start offset (document order).
	result := make([]*Passage, 0, pq.Len())
	for pq.Len() > 0 {
		p := heap.Pop(pq).(*Passage)
		result = append(result, p)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].StartOffset() < result[j].StartOffset()
	})
	return result, nil
}

// maybeAddPassage finalises the current passage (if non-empty) and pushes
// it onto the priority queue, popping the lowest-scoring entry when
// maxPassages would be exceeded. Returns a passage object the caller can
// re-use for the next segment.
func (h *FieldHighlighter) maybeAddPassage(pq *passagePQ, passage *Passage, contentLength int) *Passage {
	if passage.StartOffset() == -1 {
		return passage
	}
	passage.SetScore(h.passageScorer.Score(passage, contentLength))
	if pq.Len() == h.maxPassages && (*pq)[0].Score() > passage.Score() {
		passage.Reset()
		return passage
	}
	heap.Push(pq, passage)
	if pq.Len() > h.maxPassages {
		evicted := heap.Pop(pq).(*Passage)
		evicted.Reset()
		return evicted
	}
	return NewPassage()
}

// passagePQ is a min-heap of passages keyed by score (lowest at the top
// so the lowest-scoring entry is evicted when maxPassages is exceeded).
type passagePQ []*Passage

func (q passagePQ) Len() int { return len(q) }

func (q passagePQ) Less(i, j int) bool {
	if q[i].Score() != q[j].Score() {
		return q[i].Score() < q[j].Score()
	}
	// Tie-break by start offset (smaller offset wins, matching Lucene).
	return q[i].StartOffset() < q[j].StartOffset()
}

func (q passagePQ) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

func (q *passagePQ) Push(x any) { *q = append(*q, x.(*Passage)) }

func (q *passagePQ) Pop() any {
	old := *q
	n := len(old)
	x := old[n-1]
	*q = old[:n-1]
	return x
}

// errFieldHasNoOffsets is the sentinel error returned when an offsets-enum
// reports start == -1.
var errFieldHasNoOffsets = errors.New("uhighlight: field was indexed without offsets, cannot highlight")
