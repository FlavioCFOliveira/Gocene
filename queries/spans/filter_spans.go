package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AcceptStatus is the status returned from a FilterSpans filter that indicates
// whether a candidate match should be accepted, rejected, or rejected and move
// on to the next document.
type AcceptStatus int

const (
	// AcceptYes indicates the match should be accepted.
	AcceptYes AcceptStatus = iota
	// AcceptNo indicates the match should be rejected.
	AcceptNo
	// AcceptNoMoreInCurrentDoc indicates the match should be rejected, and the
	// enumeration may continue with the next document.
	AcceptNoMoreInCurrentDoc
)

// SpansFilter is the interface for the filter logic used by FilterSpans.
type SpansFilter interface {
	// Accept returns YES if the candidate should be an accepted match, NO if it
	// should not, and NO_MORE_IN_CURRENT_DOC if iteration should move on to the
	// next document.
	Accept(candidate Spans) (AcceptStatus, error)
}

// FilterSpans is a Spans implementation wrapping another spans instance,
// allowing to filter spans matches easily by implementing SpansFilter.
//
// Mirrors org.apache.lucene.queries.spans.FilterSpans.
type FilterSpans struct {
	BaseSpans
	in     Spans
	filter SpansFilter

	atFirstInCurrentDoc bool
	startPos            int
}

// NewFilterSpans creates a new FilterSpans wrapping the given Spans with the
// given filter.
func NewFilterSpans(in Spans, filter SpansFilter) *FilterSpans {
	if in == nil {
		panic("in cannot be nil")
	}
	if filter == nil {
		panic("filter cannot be nil")
	}
	return &FilterSpans{
		in:       in,
		filter:   filter,
		startPos: -1,
	}
}

// NextDoc returns the next matching document.
func (fs *FilterSpans) NextDoc() (int, error) {
	for {
		doc, err := fs.in.NextDoc()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if doc == search.NO_MORE_DOCS {
			return search.NO_MORE_DOCS, nil
		}
		matches, err := fs.twoPhaseCurrentDocMatches()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if matches {
			return doc, nil
		}
	}
}

// Advance advances to the first document at or beyond the target that matches.
func (fs *FilterSpans) Advance(target int) (int, error) {
	doc, err := fs.in.Advance(target)
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	for doc != search.NO_MORE_DOCS {
		matches, err := fs.twoPhaseCurrentDocMatches()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if matches {
			return doc, nil
		}
		doc, err = fs.in.NextDoc()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
	}
	return search.NO_MORE_DOCS, nil
}

// DocID returns the current document ID.
func (fs *FilterSpans) DocID() int {
	return fs.in.DocID()
}

// NextStartPosition returns the next start position for the current doc.
func (fs *FilterSpans) NextStartPosition() (int, error) {
	if fs.atFirstInCurrentDoc {
		fs.atFirstInCurrentDoc = false
		return fs.startPos, nil
	}

	for {
		startPos, err := fs.in.NextStartPosition()
		if err != nil {
			return NoMorePositions, err
		}
		if startPos == NoMorePositions {
			return NoMorePositions, nil
		}

		status, err := fs.filter.Accept(fs.in)
		if err != nil {
			return NoMorePositions, err
		}

		switch status {
		case AcceptYes:
			fs.startPos = startPos
			return startPos, nil
		case AcceptNo:
			// continue
		case AcceptNoMoreInCurrentDoc:
			fs.startPos = NoMorePositions
			return NoMorePositions, nil
		}
	}
}

// StartPosition returns the start position in the current doc.
func (fs *FilterSpans) StartPosition() int {
	if fs.atFirstInCurrentDoc {
		return -1
	}
	return fs.startPos
}

// EndPosition returns the end position for the current start position.
func (fs *FilterSpans) EndPosition() int {
	if fs.atFirstInCurrentDoc {
		return -1
	}
	if fs.startPos == NoMorePositions {
		return NoMorePositions
	}
	return fs.in.EndPosition()
}

// Width returns the width of the match.
func (fs *FilterSpans) Width() int {
	return fs.in.Width()
}

// Collect collects postings data from the leaves of the current Spans.
func (fs *FilterSpans) Collect(collector SpanCollector) error {
	return fs.in.Collect(collector)
}

// Cost returns an estimation of the cost.
func (fs *FilterSpans) Cost() int64 {
	return fs.in.Cost()
}

// AsTwoPhaseIterator returns an optional TwoPhaseIterator view of this Spans.
func (fs *FilterSpans) AsTwoPhaseIterator() *search.TwoPhaseIterator {
	inner := fs.in.AsTwoPhaseIterator()
	if inner != nil {
		// wrapped instance has an approximation
		return search.NewTwoPhaseIteratorWithMatchCost(
			inner.Approximation(),
			func() (bool, error) {
				matches, err := inner.Matches()
				if err != nil {
					return false, err
				}
				if !matches {
					return false, nil
				}
				return fs.twoPhaseCurrentDocMatches()
			},
			inner.MatchCost(), // underestimate
		)
	}

	// wrapped instance has no approximation, but
	// we can still defer matching until absolutely needed.
	return search.NewTwoPhaseIteratorWithMatchCost(
		fs.in,
		func() (bool, error) {
			return fs.twoPhaseCurrentDocMatches()
		},
		fs.in.PositionsCost(), // overestimate
	)
}

// PositionsCost returns an estimation of the cost of using positions of this
// Spans for any single document.
func (fs *FilterSpans) PositionsCost() float32 {
	panic("UnsupportedOperationException")
}

func (fs *FilterSpans) twoPhaseCurrentDocMatches() (bool, error) {
	fs.atFirstInCurrentDoc = false
	startPos, err := fs.in.NextStartPosition()
	if err != nil {
		return false, err
	}
	if startPos == NoMorePositions {
		fs.startPos = -1
		return false, nil
	}

	for {
		status, err := fs.filter.Accept(fs.in)
		if err != nil {
			return false, err
		}

		switch status {
		case AcceptYes:
			fs.atFirstInCurrentDoc = true
			fs.startPos = startPos
			return true, nil
		case AcceptNo:
			startPos, err = fs.in.NextStartPosition()
			if err != nil {
				return false, err
			}
			if startPos == NoMorePositions {
				goto noMore
			}
		case AcceptNoMoreInCurrentDoc:
			goto noMore
		}
	}
noMore:
	fs.startPos = -1
	return false, nil
}

func (fs *FilterSpans) String() string {
	return fmt.Sprintf("Filter(%v)", fs.in)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0, which FilterSpans inherits without overriding.
func (fs *FilterSpans) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(fs)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which FilterSpans inherits without overriding.
func (fs *FilterSpans) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(fs, upTo, bitSet, offset)
}

// Ensure FilterSpans implements Spans
var _ Spans = (*FilterSpans)(nil)
