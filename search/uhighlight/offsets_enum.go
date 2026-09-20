// Package uhighlight implements org.apache.lucene.search.uhighlight: the
// "unified" highlighter that consumes per-document offsets from postings,
// term vectors, or re-analysis on demand.
package uhighlight

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// OffsetsEnum is an enumeration/iterator of a term and its offsets for use by
// FieldHighlighter. It is advanced and is placed in a priority queue by
// FieldHighlighter.highlightOffsetsEnums(OffsetsEnum) based on the start
// offset.
//
// This is the Go port of the abstract class
// org.apache.lucene.search.uhighlight.OffsetsEnum from Apache Lucene 10.5.0;
// its member set is that class's abstract methods plus close(). Every Java
// member declares `throws IOException`, which Gocene renders as a trailing
// error result. Java's BytesRef term is rendered as []byte throughout this
// package (see Passage.AddMatch).
type OffsetsEnum interface {
	// NextPosition advances to the next position and returns true, or if it
	// can't then returns false. Note that the initial state of an
	// implementation is not positioned. Mirrors
	// OffsetsEnum.nextPosition().
	NextPosition() (bool, error)

	// Freq returns an estimate of the number of occurrences of this
	// term/OffsetsEnum. Mirrors OffsetsEnum.freq().
	Freq() (int, error)

	// GetTerm returns the term at this position. The returned slice is safe
	// to continue to refer to, even after we move to the next position.
	// Mirrors OffsetsEnum.getTerm().
	GetTerm() ([]byte, error)

	// StartOffset returns the start character offset of the current position.
	// Mirrors OffsetsEnum.startOffset().
	StartOffset() (int, error)

	// EndOffset returns the end (exclusive) character offset of the current
	// position. Mirrors OffsetsEnum.endOffset().
	EndOffset() (int, error)

	// Close releases resources held by the enum. Mirrors
	// OffsetsEnum.close(), whose body in Java is empty.
	Close() error
}

// errOffsetsEnumUnsupported renders the UnsupportedOperationException raised
// by the accessors of OffsetsEnum.EMPTY (OffsetsEnum.java:314).
var errOffsetsEnumUnsupported = errors.New("uhighlight: operation not supported by OffsetsEnum.EMPTY")

// BaseOffsetsEnum carries the one concrete body the Java abstract class
// supplies to every subclass: `public void close() throws IOException {}`
// (OffsetsEnum.java:97). Go has no class inheritance, so subclasses embed it.
type BaseOffsetsEnum struct{}

// Close renders OffsetsEnum.close(), whose Java body is empty.
func (BaseOffsetsEnum) Close() error { return nil }

// CompareOffsetsEnum renders OffsetsEnum.compareTo(OffsetsEnum)
// (OffsetsEnum.java:50). Note: the ordering clearly changes as the postings
// enum advances. Java wraps the IOException of the accessors in a
// RuntimeException; Go renders that as a panic.
func CompareOffsetsEnum(a, b OffsetsEnum) int {
	aStart, err := a.StartOffset()
	if err != nil {
		panic(err)
	}
	bStart, err := b.StartOffset()
	if err != nil {
		panic(err)
	}
	if cmp := compareInt(aStart, bStart); cmp != 0 {
		return cmp // vast majority of the time we return here.
	}
	aEnd, err := a.EndOffset()
	if err != nil {
		panic(err)
	}
	bEnd, err := b.EndOffset()
	if err != nil {
		panic(err)
	}
	if cmp := compareInt(aEnd, bEnd); cmp != 0 {
		return cmp
	}
	thisTerm, err := a.GetTerm()
	if err != nil {
		panic(err)
	}
	otherTerm, err := b.GetTerm()
	if err != nil {
		panic(err)
	}
	if thisTerm == nil || otherTerm == nil {
		switch {
		case thisTerm == nil && otherTerm == nil:
			return 0
		case thisTerm == nil:
			return 1 // put "this" (wildcard mtq enum) last
		default:
			return -1
		}
	}
	return bytes.Compare(thisTerm, otherTerm)
}

// compareInt renders java.lang.Integer.compare(int, int).
func compareInt(x, y int) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	default:
		return 0
	}
}

// offsetsEnumString renders OffsetsEnum.toString() (OffsetsEnum.java:99),
// which Java inherits into every subclass; Go has no inherited toString, so
// each concrete type's String method delegates here with itself.
func offsetsEnumString(oe OffsetsEnum) string {
	name := fmt.Sprintf("%T", oe)
	offset := ""
	if start, err := oe.StartOffset(); err == nil {
		if end, err := oe.EndOffset(); err == nil {
			offset = fmt.Sprintf(",[%d-%d]", start, end)
		}
	}
	term, err := oe.GetTerm()
	if err != nil || term == nil {
		return name
	}
	return name + "(term:" + string(term) + offset + ")"
}

// offsetsEnumPQ renders the java.util.PriorityQueue<OffsetsEnum> used by
// OfMatchesIteratorWithSubs and MultiOffsetsEnum; its ordering is
// OffsetsEnum.compareTo.
type offsetsEnumPQ struct{ items []OffsetsEnum }

func (pq *offsetsEnumPQ) Len() int { return len(pq.items) }
func (pq *offsetsEnumPQ) Less(i, j int) bool {
	return CompareOffsetsEnum(pq.items[i], pq.items[j]) < 0
}
func (pq *offsetsEnumPQ) Swap(i, j int) { pq.items[i], pq.items[j] = pq.items[j], pq.items[i] }
func (pq *offsetsEnumPQ) Push(x any)    { pq.items = append(pq.items, x.(OffsetsEnum)) }
func (pq *offsetsEnumPQ) Pop() any {
	old := pq.items
	n := len(old)
	it := old[n-1]
	pq.items = old[:n-1]
	return it
}

// add renders java.util.PriorityQueue.add(E).
func (pq *offsetsEnumPQ) add(oe OffsetsEnum) { heap.Push(pq, oe) }

// poll renders java.util.PriorityQueue.poll(): it removes and returns the
// head, or nil when the queue is empty.
func (pq *offsetsEnumPQ) poll() OffsetsEnum {
	if len(pq.items) == 0 {
		return nil
	}
	return heap.Pop(pq).(OffsetsEnum)
}

// peek renders java.util.PriorityQueue.peek(): it returns the head without
// removing it, or nil when the queue is empty.
func (pq *offsetsEnumPQ) peek() OffsetsEnum {
	if len(pq.items) == 0 {
		return nil
	}
	return pq.items[0]
}

// isEmpty renders java.util.PriorityQueue.isEmpty().
func (pq *offsetsEnumPQ) isEmpty() bool { return len(pq.items) == 0 }

// size renders java.util.PriorityQueue.size().
func (pq *offsetsEnumPQ) size() int { return len(pq.items) }

// closers renders the Iterable<Closeable> that IOUtils.close(queue) consumes
// in MultiOffsetsEnum.close().
func (pq *offsetsEnumPQ) closers() []OffsetsEnum { return pq.items }

// OfPostings is based on a PostingsEnum -- the typical/standard OffsetsEnum
// implementation. Mirrors the public static class OffsetsEnum.OfPostings
// (OffsetsEnum.java:128).
type OfPostings struct {
	BaseOffsetsEnum
	term         []byte
	postingsEnum index.PostingsEnum // with offsets
	freq         int

	posCounter int
}

// NewOfPostings renders `OfPostings(BytesRef term, int freq, PostingsEnum
// postingsEnum)` (OffsetsEnum.java:135).
func NewOfPostings(term []byte, freq int, postingsEnum index.PostingsEnum) (*OfPostings, error) {
	if term == nil {
		return nil, errors.New("uhighlight: OfPostings term must not be nil")
	}
	if postingsEnum == nil {
		return nil, errors.New("uhighlight: OfPostings postingsEnum must not be nil")
	}
	posCounter, err := postingsEnum.Freq()
	if err != nil {
		return nil, err
	}
	return &OfPostings{
		term:         term,
		postingsEnum: postingsEnum,
		freq:         freq,
		posCounter:   posCounter,
	}, nil
}

// NewOfPostingsFromPostings renders `OfPostings(BytesRef term, PostingsEnum
// postingsEnum)` (OffsetsEnum.java:142), the overload that takes the
// frequency from the postings enum itself. Go has no overloading, so the
// delegating constructor carries a distinguishing suffix.
func NewOfPostingsFromPostings(term []byte, postingsEnum index.PostingsEnum) (*OfPostings, error) {
	if postingsEnum == nil {
		return nil, errors.New("uhighlight: OfPostings postingsEnum must not be nil")
	}
	freq, err := postingsEnum.Freq()
	if err != nil {
		return nil, err
	}
	return NewOfPostings(term, freq, postingsEnum)
}

// GetPostingsEnum renders OffsetsEnum.OfPostings.getPostingsEnum()
// (OffsetsEnum.java:146).
func (e *OfPostings) GetPostingsEnum() index.PostingsEnum { return e.postingsEnum }

// NextPosition renders OffsetsEnum.OfPostings.nextPosition().
func (e *OfPostings) NextPosition() (bool, error) {
	if e.posCounter > 0 {
		e.posCounter--
		// note: we don't need to save the position
		if _, err := e.postingsEnum.NextPosition(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// GetTerm renders OffsetsEnum.OfPostings.getTerm().
func (e *OfPostings) GetTerm() ([]byte, error) { return e.term, nil }

// StartOffset renders OffsetsEnum.OfPostings.startOffset().
func (e *OfPostings) StartOffset() (int, error) { return e.postingsEnum.StartOffset() }

// EndOffset renders OffsetsEnum.OfPostings.endOffset().
func (e *OfPostings) EndOffset() (int, error) { return e.postingsEnum.EndOffset() }

// Freq renders OffsetsEnum.OfPostings.freq().
func (e *OfPostings) Freq() (int, error) { return e.freq, nil }

// String renders the inherited OffsetsEnum.toString().
func (e *OfPostings) String() string { return offsetsEnumString(e) }

var _ OffsetsEnum = (*OfPostings)(nil)

// OfMatchesIteratorWithSubs is based on a MatchesIterator with submatches.
// Mirrors the public static class OffsetsEnum.OfMatchesIteratorWithSubs
// (OffsetsEnum.java:179).
type OfMatchesIteratorWithSubs struct {
	BaseOffsetsEnum
	// Either cachedOE impls (which are the submatches) or OfMatchesIterator impls
	pendingQueue   *offsetsEnumPQ
	queryToTermMap map[search.Query][]byte
}

// NewOfMatchesIteratorWithSubs renders
// `OfMatchesIteratorWithSubs(MatchesIterator matchesIterator)`
// (OffsetsEnum.java:184).
func NewOfMatchesIteratorWithSubs(matchesIterator search.MatchesIterator) *OfMatchesIteratorWithSubs {
	e := &OfMatchesIteratorWithSubs{
		pendingQueue:   &offsetsEnumPQ{},
		queryToTermMap: make(map[search.Query][]byte),
	}
	e.pendingQueue.add(NewOfMatchesIterator(matchesIterator, func() []byte {
		return e.queryToTerm(matchesIterator.GetQuery())
	}))
	return e
}

// NextPosition renders
// OffsetsEnum.OfMatchesIteratorWithSubs.nextPosition().
func (e *OfMatchesIteratorWithSubs) NextPosition() (bool, error) {
	formerHeadOE := e.pendingQueue.poll() // removes the head
	if _, isCached := formerHeadOE.(*cachedOE); isCached {
		// we're done with the former head.  cachedOE's are one use only.
		// Look at the new head...
		newHeadOE := e.pendingQueue.peek()
		if miOE, ok := newHeadOE.(*OfMatchesIterator); ok {
			// We found the matchesIterator.  Requires processing.
			// May or may not remove or re-queue itself
			if err := e.nextWhenMatchesIterator(miOE); err != nil {
				return false, err
			}
		} // else new head is a cachedOE or no more.  Nothing to do with it.
	} else { // formerHeadOE is OfMatchesIterator; advance it
		miOE := formerHeadOE.(*OfMatchesIterator)
		more, err := miOE.NextPosition()
		if err != nil {
			return false, err
		}
		if more {
			// requires processing.  May or may not re-enqueue itself
			if err := e.nextWhenMatchesIterator(miOE); err != nil {
				return false, err
			}
		}
	}
	return !e.pendingQueue.isEmpty(), nil
}

// nextWhenMatchesIterator renders the private
// OfMatchesIteratorWithSubs.nextWhenMatchesIterator(OfMatchesIterator)
// (OffsetsEnum.java:206).
func (e *OfMatchesIteratorWithSubs) nextWhenMatchesIterator(miOE *OfMatchesIterator) error {
	isHead := OffsetsEnum(miOE) == e.pendingQueue.peek()
	subMatches, err := miOE.matchesIterator.GetSubMatches()
	if err != nil {
		return err
	}
	if subMatches != nil {
		// remove this miOE from the queue, add it's submatches, next() it, then re-enqueue it
		if isHead {
			e.pendingQueue.poll() // remove
		}

		if _, err := e.enqueueCachedMatches(subMatches); err != nil {
			return err
		}

		more, err := miOE.NextPosition()
		if err != nil {
			return err
		}
		if more {
			e.pendingQueue.add(miOE)
		}
	} else { // else has no subMatches.  It will stay enqueued.
		if !isHead {
			e.pendingQueue.add(miOE)
		} // else it's *already* in pendingQueue
	}
	return nil
}

// enqueueCachedMatches renders the private
// OfMatchesIteratorWithSubs.enqueueCachedMatches(MatchesIterator)
// (OffsetsEnum.java:227).
func (e *OfMatchesIteratorWithSubs) enqueueCachedMatches(thisMI search.MatchesIterator) (bool, error) {
	if thisMI == nil {
		return false, nil
	}
	for {
		more, err := thisMI.Next()
		if err != nil {
			return false, err
		}
		if !more {
			break
		}
		sub, err := thisMI.GetSubMatches()
		if err != nil {
			return false, err
		}
		enqueued, err := e.enqueueCachedMatches(sub) // recursion
		if err != nil {
			return false, err
		}
		if !enqueued {
			// if no sub-matches then add ourselves
			start, err := thisMI.StartOffset()
			if err != nil {
				return false, err
			}
			end, err := thisMI.EndOffset()
			if err != nil {
				return false, err
			}
			e.pendingQueue.add(newCachedOE(e.queryToTerm(thisMI.GetQuery()), start, end))
		}
	}
	return true, nil
}

// queryToTerm maps a Query from MatchesIterator.getQuery() to
// OffsetsEnum.getTerm(). See Passage.getMatchTerms(). Renders the private
// OfMatchesIteratorWithSubs.queryToTerm(Query) (OffsetsEnum.java:246).
func (e *OfMatchesIteratorWithSubs) queryToTerm(query search.Query) []byte {
	// compute an approximate BytesRef term of a Query.  We cache this since we're likely to see
	// the same query again.
	// Our approach is to visit each matching term in order, concatenating them with an adjoining
	// space.
	//  If we don't have any (perhaps due to an MTQ like a wildcard) then we fall back on the
	// toString() of the query.
	if cached, ok := e.queryToTermMap[query]; ok {
		return cached
	}
	bytesRefBuilder := util.NewBytesRefBuilder()
	visitQuery(query, &queryToTermVisitor{builder: bytesRefBuilder})
	var term []byte
	if bytesRefBuilder.Get().Length > 0 {
		term = bytesRefBuilder.Get().ValidBytes()
	} else {
		// fallback:  (likely a MultiTermQuery)
		term = []byte(queryLabel(query))
	}
	e.queryToTermMap[query] = term
	return term
}

// queryToTermVisitor renders the anonymous QueryVisitor of
// OfMatchesIteratorWithSubs.queryToTerm (OffsetsEnum.java:258).
type queryToTermVisitor struct {
	search.EmptyQueryVisitorBase
	builder *util.BytesRefBuilder
}

// ConsumeTerms renders the visitor's consumeTerms(Query, Term...) override.
func (v *queryToTermVisitor) ConsumeTerms(_ search.Query, terms ...*index.Term) {
	for _, term := range terms {
		if v.builder.Get().Length > 0 {
			v.builder.Append(util.NewBytesRef([]byte{' '}))
		}
		v.builder.Append(term.Bytes)
	}
}

// Freq renders OffsetsEnum.OfMatchesIteratorWithSubs.freq().
func (e *OfMatchesIteratorWithSubs) Freq() (int, error) { return e.pendingQueue.peek().Freq() }

// GetTerm renders OffsetsEnum.OfMatchesIteratorWithSubs.getTerm().
func (e *OfMatchesIteratorWithSubs) GetTerm() ([]byte, error) {
	return e.pendingQueue.peek().GetTerm()
}

// StartOffset renders OffsetsEnum.OfMatchesIteratorWithSubs.startOffset().
func (e *OfMatchesIteratorWithSubs) StartOffset() (int, error) {
	return e.pendingQueue.peek().StartOffset()
}

// EndOffset renders OffsetsEnum.OfMatchesIteratorWithSubs.endOffset().
func (e *OfMatchesIteratorWithSubs) EndOffset() (int, error) {
	return e.pendingQueue.peek().EndOffset()
}

// String renders the inherited OffsetsEnum.toString().
func (e *OfMatchesIteratorWithSubs) String() string { return offsetsEnumString(e) }

var _ OffsetsEnum = (*OfMatchesIteratorWithSubs)(nil)

// cachedOE renders the private static class
// OffsetsEnum.OfMatchesIteratorWithSubs.CachedOE (OffsetsEnum.java:293).
type cachedOE struct {
	BaseOffsetsEnum
	term        []byte
	startOffset int
	endOffset   int
}

// newCachedOE renders the private CachedOE constructor
// (OffsetsEnum.java:298).
func newCachedOE(term []byte, startOffset, endOffset int) *cachedOE {
	return &cachedOE{term: term, startOffset: startOffset, endOffset: endOffset}
}

// NextPosition renders CachedOE.nextPosition().
func (e *cachedOE) NextPosition() (bool, error) { return false, nil }

// Freq renders CachedOE.freq(); the constant 1 is a documented short-coming
// of the MatchesIterator based UnifiedHighlighter.
func (e *cachedOE) Freq() (int, error) { return 1, nil }

// GetTerm renders CachedOE.getTerm().
func (e *cachedOE) GetTerm() ([]byte, error) { return e.term, nil }

// StartOffset renders CachedOE.startOffset().
func (e *cachedOE) StartOffset() (int, error) { return e.startOffset, nil }

// EndOffset renders CachedOE.endOffset().
func (e *cachedOE) EndOffset() (int, error) { return e.endOffset, nil }

// String renders the inherited OffsetsEnum.toString().
func (e *cachedOE) String() string { return offsetsEnumString(e) }

var _ OffsetsEnum = (*cachedOE)(nil)

// OfMatchesIterator is based on a MatchesIterator; it does not look at
// submatches. Mirrors the public static class
// OffsetsEnum.OfMatchesIterator (OffsetsEnum.java:332).
type OfMatchesIterator struct {
	BaseOffsetsEnum
	matchesIterator search.MatchesIterator
	termSupplier    func() []byte
}

// NewOfMatchesIterator renders `OfMatchesIterator(MatchesIterator
// matchesIterator, Supplier<BytesRef> termSupplier)` (OffsetsEnum.java:336).
func NewOfMatchesIterator(matchesIterator search.MatchesIterator, termSupplier func() []byte) *OfMatchesIterator {
	return &OfMatchesIterator{matchesIterator: matchesIterator, termSupplier: termSupplier}
}

// NextPosition renders OfMatchesIterator.nextPosition().
func (e *OfMatchesIterator) NextPosition() (bool, error) { return e.matchesIterator.Next() }

// Freq renders OfMatchesIterator.freq(); the constant 1 is a documented
// short-coming of the MatchesIterator based UnifiedHighlighter.
func (e *OfMatchesIterator) Freq() (int, error) { return 1, nil }

// GetTerm renders OfMatchesIterator.getTerm().
func (e *OfMatchesIterator) GetTerm() ([]byte, error) { return e.termSupplier(), nil }

// StartOffset renders OfMatchesIterator.startOffset().
func (e *OfMatchesIterator) StartOffset() (int, error) { return e.matchesIterator.StartOffset() }

// EndOffset renders OfMatchesIterator.endOffset().
func (e *OfMatchesIterator) EndOffset() (int, error) { return e.matchesIterator.EndOffset() }

// String renders the inherited OffsetsEnum.toString().
func (e *OfMatchesIterator) String() string { return offsetsEnumString(e) }

var _ OffsetsEnum = (*OfMatchesIterator)(nil)

// OffsetsEnumEMPTY renders the empty enumeration OffsetsEnum.EMPTY
// (OffsetsEnum.java:363), an anonymous OffsetsEnum subclass. Java writes it as
// OffsetsEnum.EMPTY at every use site; Go has no class-scoped constants, so
// the owning class is carried in the name.
var OffsetsEnumEMPTY OffsetsEnum = &emptyOffsetsEnum{}

// emptyOffsetsEnum is the type of the anonymous subclass behind
// OffsetsEnum.EMPTY.
type emptyOffsetsEnum struct{ BaseOffsetsEnum }

// NextPosition renders the anonymous subclass's nextPosition().
func (e *emptyOffsetsEnum) NextPosition() (bool, error) { return false, nil }

// GetTerm renders the anonymous subclass's getTerm(), which throws
// UnsupportedOperationException.
func (e *emptyOffsetsEnum) GetTerm() ([]byte, error) { return nil, errOffsetsEnumUnsupported }

// StartOffset renders the anonymous subclass's startOffset(), which throws
// UnsupportedOperationException.
func (e *emptyOffsetsEnum) StartOffset() (int, error) { return 0, errOffsetsEnumUnsupported }

// EndOffset renders the anonymous subclass's endOffset(), which throws
// UnsupportedOperationException.
func (e *emptyOffsetsEnum) EndOffset() (int, error) { return 0, errOffsetsEnumUnsupported }

// Freq renders the anonymous subclass's freq().
func (e *emptyOffsetsEnum) Freq() (int, error) { return 0, nil }

// String renders the inherited OffsetsEnum.toString().
func (e *emptyOffsetsEnum) String() string { return offsetsEnumString(e) }

var _ OffsetsEnum = (*emptyOffsetsEnum)(nil)

// MultiOffsetsEnum is a view over several OffsetsEnum instances, merging them
// in-place. Mirrors the public static class OffsetsEnum.MultiOffsetsEnum
// (OffsetsEnum.java:394).
//
// If OffsetsEnum and MatchesIterator ever truly merge then this could go away
// in lieu of DisjunctionMatchesIterator.
type MultiOffsetsEnum struct {
	BaseOffsetsEnum
	queue   *offsetsEnumPQ
	started bool
}

// NewMultiOffsetsEnum renders `MultiOffsetsEnum(List<OffsetsEnum> inner)`
// (OffsetsEnum.java:399).
func NewMultiOffsetsEnum(inner []OffsetsEnum) (*MultiOffsetsEnum, error) {
	e := &MultiOffsetsEnum{queue: &offsetsEnumPQ{}}
	for _, oe := range inner {
		more, err := oe.NextPosition()
		if err != nil {
			return nil, err
		}
		if more {
			e.queue.add(oe)
		}
	}
	return e, nil
}

// NextPosition renders MultiOffsetsEnum.nextPosition().
func (e *MultiOffsetsEnum) NextPosition() (bool, error) {
	if !e.started {
		e.started = true
		return e.queue.size() > 0, nil
	}
	if e.queue.size() > 0 {
		top := e.queue.poll()
		more, err := top.NextPosition()
		if err != nil {
			return false, err
		}
		if more {
			e.queue.add(top)
			return true, nil
		}
		if err := top.Close(); err != nil {
			return false, err
		}
		return e.queue.size() > 0, nil
	}
	return false, nil
}

// GetTerm renders MultiOffsetsEnum.getTerm().
func (e *MultiOffsetsEnum) GetTerm() ([]byte, error) { return e.queue.peek().GetTerm() }

// StartOffset renders MultiOffsetsEnum.startOffset().
func (e *MultiOffsetsEnum) StartOffset() (int, error) { return e.queue.peek().StartOffset() }

// EndOffset renders MultiOffsetsEnum.endOffset().
func (e *MultiOffsetsEnum) EndOffset() (int, error) { return e.queue.peek().EndOffset() }

// Freq renders MultiOffsetsEnum.freq().
func (e *MultiOffsetsEnum) Freq() (int, error) { return e.queue.peek().Freq() }

// Close renders MultiOffsetsEnum.close(): most child enums will have been
// closed in NextPosition; here all remaining non-exhausted enums are closed.
func (e *MultiOffsetsEnum) Close() error {
	remaining := e.queue.closers()
	closeables := make([]io.Closer, 0, len(remaining))
	for _, oe := range remaining {
		closeables = append(closeables, oe)
	}
	return util.CloseAll(closeables...)
}

// String renders the inherited OffsetsEnum.toString().
func (e *MultiOffsetsEnum) String() string { return offsetsEnumString(e) }

var _ OffsetsEnum = (*MultiOffsetsEnum)(nil)
