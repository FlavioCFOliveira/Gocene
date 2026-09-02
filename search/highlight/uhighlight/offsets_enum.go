package uhighlight

import (
	"container/heap"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// OffsetsEnum is an enumeration/iterator of a term and its offsets for use by FieldHighlighter.
type OffsetsEnum interface {
	io.Closer
	NextPosition() (bool, error)
	Freq() (int, error)
	Term() (util.BytesRef, error)
	StartOffset() (int, error)
	EndOffset() (int, error)
}

// CompareOffsetsEnum compares two OffsetsEnum based on start offset, then end offset, then term.
func CompareOffsetsEnum(a, b OffsetsEnum) (int, error) {
	startA, err := a.StartOffset()
	if err != nil {
		return 0, err
	}
	startB, err := b.StartOffset()
	if err != nil {
		return 0, err
	}
	if startA != startB {
		if startA < startB {
			return -1, nil
		}
		return 1, nil
	}

	endA, err := a.EndOffset()
	if err != nil {
		return 0, err
	}
	endB, err := b.EndOffset()
	if err != nil {
		return 0, err
	}
	if endA != endB {
		if endA < endB {
			return -1, nil
		}
		return 1, nil
	}

	termA, err := a.Term()
	if err != nil {
		return 0, err
	}
	termB, err := b.Term()
	if err != nil {
		return 0, err
	}

	if termA == nil || termB == nil {
		if termA == nil && termB == nil {
			return 0, nil
		} else if termA == nil {
			return 1, nil // put wildcard last
		} else {
			return -1, nil
		}
	}

	return termA.Compare(termB), nil
}

// OfPostings is based on a PostingsEnum.
type OfPostings struct {
	term        util.BytesRef
	postingsEnum index.PostingsEnum
	freq        int
	posCounter  int
}

func NewOfPostings(term util.BytesRef, postingsEnum index.PostingsEnum) (*OfPostings, error) {
	if term == nil {
		return nil, fmt.Errorf("term is required")
	}
	if postingsEnum == nil {
		return nil, fmt.Errorf("postingsEnum is required")
	}
	freq, err := postingsEnum.Freq()
	if err != nil {
		return nil, err
	}
	return &OfPostings{
		term:        term,
		postingsEnum: postingsEnum,
		freq:        freq,
		posCounter:  freq,
	}, nil
}

func NewOfPostingsWithFreq(term util.BytesRef, freq int, postingsEnum index.PostingsEnum) (*OfPostings, error) {
	if term == nil {
		return nil, fmt.Errorf("term is required")
	}
	if postingsEnum == nil {
		return nil, fmt.Errorf("postingsEnum is required")
	}
	return &OfPostings{
		term:        term,
		postingsEnum: postingsEnum,
		freq:        freq,
		posCounter:  freq,
	}, nil
}

func (oe *OfPostings) NextPosition() (bool, error) {
	if oe.posCounter > 0 {
		oe.posCounter--
		if err := oe.postingsEnum.NextPosition(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (oe *OfPostings) Term() (util.BytesRef, error) {
	return oe.term, nil
}

func (oe *OfPostings) StartOffset() (int, error) {
	return oe.postingsEnum.StartOffset()
}

func (oe *OfPostings) EndOffset() (int, error) {
	return oe.postingsEnum.EndOffset()
}

func (oe *OfPostings) Freq() (int, error) {
	return oe.freq, nil
}

func (oe *OfPostings) Close() error {
	return nil
}

// EmptyOffsetsEnum is an empty enumeration.
type EmptyOffsetsEnum struct{}

func (oe *EmptyOffsetsEnum) NextPosition() (bool, error) { return false, nil }
func (oe *EmptyOffsetsEnum) Freq() (int, error)         { return 0, nil }
func (oe *EmptyOffsetsEnum) Term() (util.BytesRef, error) {
	return nil, fmt.Errorf("unsupported operation")
}
func (oe *EmptyOffsetsEnum) StartOffset() (int, error) {
	return 0, fmt.Errorf("unsupported operation")
}
func (oe *EmptyOffsetsEnum) EndOffset() (int, error) {
	return 0, fmt.Errorf("unsupported operation")
}
func (oe *EmptyOffsetsEnum) Close() error { return nil }

var Empty = &EmptyOffsetsEnum{}

// MultiOffsetsEnum is a view over several OffsetsEnum instances, merging them in-place.
type MultiOffsetsEnum struct {
	queue   offsetsHeap
	started bool
}

func NewMultiOffsetsEnum(inner []OffsetsEnum) (*MultiOffsetsEnum, error) {
	moe := &MultiOffsetsEnum{
		queue: make(offsetsHeap, 0),
	}
	for _, oe := range inner {
		if next, err := oe.NextPosition(); err != nil {
			return nil, err
		} else if next {
			heap.Push(&moe.queue, oe)
		}
	}
	return moe, nil
}

func (moe *MultiOffsetsEnum) NextPosition() (bool, error) {
	if !moe.started {
		moe.started = true
		return moe.queue.Len() > 0, nil
	}
	if moe.queue.Len() > 0 {
		top := heap.Pop(&moe.queue).(OffsetsEnum)
		if next, err := top.NextPosition(); err != nil {
			return false, err
		} else if next {
			heap.Push(&moe.queue, top)
			return true, nil
		} else {
			top.Close()
		}
		return moe.queue.Len() > 0, nil
	}
	return false, nil
}

func (moe *MultiOffsetsEnum) Term() (util.BytesRef, error) {
	if moe.queue.Len() == 0 {
		return nil, fmt.Errorf("no offsets enum available")
	}
	return moe.queue[0].Term()
}

func (moe *MultiOffsetsEnum) StartOffset() (int, error) {
	if moe.queue.Len() == 0 {
		return 0, fmt.Errorf("no offsets enum available")
	}
	return moe.queue[0].StartOffset()
}

func (moe *MultiOffsetsEnum) EndOffset() (int, error) {
	if moe.queue.Len() == 0 {
		return 0, fmt.Errorf("no offsets enum available")
	}
	return moe.queue[0].EndOffset()
}

func (moe *MultiOffsetsEnum) Freq() (int, error) {
	if moe.queue.Len() == 0 {
		return 0, fmt.Errorf("no offsets enum available")
	}
	return moe.queue[0].Freq()
}

func (moe *MultiOffsetsEnum) Close() error {
	for moe.queue.Len() > 0 {
		oe := heap.Pop(&moe.queue).(OffsetsEnum)
		oe.Close()
	}
	return nil
}

type offsetsHeap []OffsetsEnum

func (h offsetsHeap) Len() int { return len(h) }
func (h offsetsHeap) Less(i, j int) bool {
	cmp, err := CompareOffsetsEnum(h[i], h[j])
	if err != nil {
		return false
	}
	return cmp < 0
}
func (h offsetsHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *offsetsHeap) Push(x interface{}) {
	*h = append(*h, x.(OffsetsEnum))
}
func (h *offsetsHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
