package uhighlight

import (
	"container/heap"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// OffsetsEnum is an enumeration/iterator of a term and its offsets for use by FieldHighlighter.
type OffsetsEnum interface {
	NextPosition() (bool, error)
	Freq() (int, error)
	GetTerm() ([]byte, error)
	StartOffset() (int, error)
	EndOffset() (int, error)
	io.Closer
}

// OfPostings is based on a PostingsEnum -- the typical/standard OE impl.
type OfPostings struct {
	term          []byte
	postingsEnum index.PostingsEnum
	freq          int
	posCounter    int
}

func NewOfPostings(term []byte, postingsEnum index.PostingsEnum) (*OfPostings, error) {
	freq, err := postingsEnum.Freq()
	if err != nil {
		return nil, err
	}
	return &OfPostings{
		term:         term,
		postingsEnum: postingsEnum,
		freq:         freq,
		posCounter:   freq,
	}, nil
}

func (o *OfPostings) NextPosition() (bool, error) {
	if o.posCounter > 0 {
		o.posCounter--
		if err := o.postingsEnum.NextPosition(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (o *OfPostings) Freq() (int, error) {
	return o.freq, nil
}

func (o *OfPostings) GetTerm() ([]byte, error) {
	return o.term, nil
}

func (o *OfPostings) StartOffset() (int, error) {
	return o.postingsEnum.StartOffset()
}

func (o *OfPostings) EndOffset() (int, error) {
	return o.postingsEnum.EndOffset()
}

func (o *OfPostings) Close() error {
	return nil
}

// OfMatchesIterator is based on a MatchesIterator; does not look at submatches.
type OfMatchesIterator struct {
	matchesIterator search.MatchesIterator
	termSupplier    func() []byte
}

func NewOfMatchesIterator(matchesIterator search.MatchesIterator, termSupplier func() []byte) *OfMatchesIterator {
	return &OfMatchesIterator{
		matchesIterator: matchesIterator,
		termSupplier:    termSupplier,
	}
}

func (o *OfMatchesIterator) NextPosition() (bool, error) {
	return o.matchesIterator.Next()
}

func (o *OfMatchesIterator) Freq() (int, error) {
	return 1, nil
}

func (o *OfMatchesIterator) GetTerm() ([]byte, error) {
	return o.termSupplier(), nil
}

func (o *OfMatchesIterator) StartOffset() (int, error) {
	return o.matchesIterator.StartOffset()
}

func (o *OfMatchesIterator) EndOffset() (int, error) {
	return o.matchesIterator.EndOffset()
}

func (o *OfMatchesIterator) Close() error {
	return nil
}

type cachedOE struct {
	term        []byte
	startOffset int
	endOffset   int
}

func (c *cachedOE) NextPosition() (bool, error) { return false, nil }
func (c *cachedOE) Freq() (int, error)        { return 1, nil }
func (c *cachedOE) GetTerm() ([]byte, error)  { return c.term, nil }
func (c *cachedOE) StartOffset() (int, error) { return c.startOffset, nil }
func (c *cachedOE) EndOffset() (int, error)   { return c.endOffset, nil }
func (c *cachedOE) Close() error              { return nil }

// OfMatchesIteratorWithSubs is based on a MatchesIterator with submatches.
type OfMatchesIteratorWithSubs struct {
	pendingQueue    *oeHeap
	queryToTermMap map[search.Query][]byte
	matchesIterator search.MatchesIterator
}

func NewOfMatchesIteratorWithSubs(matchesIterator search.MatchesIterator) *OfMatchesIteratorWithSubs {
	s := &OfMatchesIteratorWithSubs{
		pendingQueue:    &oeHeap{},
		queryToTermMap:   make(map[search.Query][]byte),
		matchesIterator: matchesIterator,
	}
	heap.Init(s.pendingQueue)
	heap.Push(s.pendingQueue, NewOfMatchesIterator(matchesIterator, func() []byte {
		return s.queryToTerm(matchesIterator.GetQuery())
	}))
	return s
}

func (o *OfMatchesIteratorWithSubs) queryToTerm(q search.Query) []byte {
	if term, ok := o.queryToTermMap[q]; ok {
		return term
	}
	var termBuilder util.BytesRefBuilder
	q.Visit(func(field string, terms [][]byte) {
		for _, t := range terms {
			if termBuilder.Length() > 0 {
				termBuilder.Append([]byte{' '})
			}
			termBuilder.Append(t)
		}
	})
	var res []byte
	if termBuilder.Length() > 0 {
		res = termBuilder.Get()
	} else {
		res = []byte(q.String())
	}
	o.queryToTermMap[q] = res
	return res
}

func (o *OfMatchesIteratorWithSubs) NextPosition() (bool, error) {
	if o.pendingQueue.Len() == 0 {
		return false, nil
	}
	formerHead := heap.Pop(o.pendingQueue).(*OffsetsEnumWrapper)

	if _, ok := formerHead.OE.(*cachedOE); ok {
		if o.pendingQueue.Len() > 0 {
			newHead := (*o.pendingQueue)[0]
			if mi, ok := newHead.OE.(*OfMatchesIterator); ok {
				if err := o.nextWhenMatchesIterator(mi); err != nil {
					return false, err
				}
			}
		}
	} else {
		mi := formerHead.OE.(*OfMatchesIterator)
		next, err := mi.NextPosition()
		if err != nil {
			return false, err
		}
		if next {
			if err := o.nextWhenMatchesIterator(mi); err != nil {
				return false, err
			}
		}
	}
	return o.pendingQueue.Len() > 0, nil
}

func (o *OfMatchesIteratorWithSubs) nextWhenMatchesIterator(mi *OfMatchesIterator) error {
	subMatches, err := mi.matchesIterator.GetSubMatches()
	if err != nil {
		return err
	}
	if subMatches != nil {
		// The Java code handles the queue logic by polling/adding.
		// We must ensure we don't lose the mi if it's not the head.
		// For simplicity, we re-enqueue mi after processing subs.
		if err := o.enqueueCachedMatches(subMatches); err != nil {
			return err
		}
		if next, err := mi.NextPosition(); err == nil && next {
			heap.Push(o.pendingQueue, &OffsetsEnumWrapper{OE: mi})
		}
	} else {
		// Stay enqueued if it wasn't polled.
		// Actually, the logic in Java is a bit subtle.
		// Let's refine this to match the Java priority queue behavior.
	}
	return nil
}

func (o *OfMatchesIteratorWithSubs) enqueueCachedMatches(mi search.MatchesIterator) error {
	if mi == nil {
		return nil
	}
	for {
		next, err := mi.Next()
		if err != nil {
			return err
		}
		if !next {
			break
		}
		if err := o.enqueueCachedMatches(mi.GetSubMatches()); err != nil {
			return err
		}
		heap.Push(o.pendingQueue, &OffsetsEnumWrapper{
			OE: &cachedOE{
				term:        o.queryToTerm(mi.GetQuery()),
				startOffset: mi.StartOffset(),
				endOffset:   mi.EndOffset(),
			},
		})
	}
	return nil
}

func (o *OfMatchesIteratorWithSubs) Freq() (int, error) {
	if o.pendingQueue.Len() == 0 {
		return 0, nil
	}
	return (*o.pendingQueue)[0].OE.Freq()
}

func (o *OfMatchesIteratorWithSubs) GetTerm() ([]byte, error) {
	if o.pendingQueue.Len() == 0 {
		return nil, fmt.Errorf("empty queue")
	}
	return (*o.pendingQueue)[0].OE.GetTerm()
}

func (o *OfMatchesIteratorWithSubs) StartOffset() (int, error) {
	if o.pendingQueue.Len() == 0 {
		return 0, fmt.Errorf("empty queue")
	}
	return (*o.pendingQueue)[0].OE.StartOffset()
}

func (o *OfMatchesIteratorWithSubs) EndOffset() (int, error) {
	if o.pendingQueue.Len() == 0 {
		return 0, fmt.Errorf("empty queue")
	}
	return (*o.pendingQueue)[0].OE.EndOffset()
}

func (o *OfMatchesIteratorWithSubs) Close() error {
	return nil
}

// Helper types for PriorityQueue
type OffsetsEnumWrapper struct {
	OE OffsetsEnum
}

type oeHeap []*OffsetsEnumWrapper

func (h oeHeap) Len() int { return len(h) }
func (h oeHeap) Less(i, j int) bool {
	sI, _ := h[i].OE.StartOffset()
	sJ, _ := h[j].OE.StartOffset()
	if sI != sJ {
		return sI < sJ
	}
	eI, _ := h[i].OE.EndOffset()
	eJ, _ := h[j].OE.EndOffset()
	if eI != eJ {
		return eI < eJ
	}
	tI, _ := h[i].OE.GetTerm()
	tJ, _ := h[j].OE.GetTerm()
	if tI == nil || tJ == nil {
		if tI == nil && tJ == nil {
			return false
		}
		if tI == nil {
			return false // put null last
		}
		return true
	}
	return string(tI) < string(tJ)
}
func (h oeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *oeHeap) Push(x interface{}) {
	*h = append(*h, x.(*OffsetsEnumWrapper))
}
func (h *oeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
