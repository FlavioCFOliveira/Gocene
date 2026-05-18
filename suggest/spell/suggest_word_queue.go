package spell

import "container/heap"

// CompareFn is the comparator used by SuggestWordQueue. Mirrors Java's
// Comparator<SuggestWord> semantics: returns -1, 0, +1.
type CompareFn func(a, b *SuggestWord) int

// SuggestWordScoreComparator orders by descending score and, when scores are
// equal, by descending frequency. Mirrors
// org.apache.lucene.search.spell.SuggestWordScoreComparator.
func SuggestWordScoreComparator(a, b *SuggestWord) int {
	if a.Score < b.Score {
		return -1
	}
	if a.Score > b.Score {
		return 1
	}
	if a.Freq < b.Freq {
		return -1
	}
	if a.Freq > b.Freq {
		return 1
	}
	return 0
}

// SuggestWordFrequencyComparator orders by descending frequency, then score.
// Mirrors org.apache.lucene.search.spell.SuggestWordFrequencyComparator.
func SuggestWordFrequencyComparator(a, b *SuggestWord) int {
	if a.Freq < b.Freq {
		return -1
	}
	if a.Freq > b.Freq {
		return 1
	}
	if a.Score < b.Score {
		return -1
	}
	if a.Score > b.Score {
		return 1
	}
	return 0
}

// SuggestWordQueue is a bounded priority queue of SuggestWord pointers. The
// internal ordering is min-first under the supplied comparator so that the
// queue keeps the top-N "largest" entries (the smallest entry sits at the
// top and is evicted first). Mirrors
// org.apache.lucene.search.spell.SuggestWordQueue.
type SuggestWordQueue struct {
	cap     int
	cmp     CompareFn
	entries suggestWordHeap
}

// NewSuggestWordQueue builds a queue with the supplied capacity. The default
// comparator is SuggestWordScoreComparator.
func NewSuggestWordQueue(capacity int, cmp CompareFn) *SuggestWordQueue {
	if capacity < 1 {
		capacity = 1
	}
	if cmp == nil {
		cmp = SuggestWordScoreComparator
	}
	q := &SuggestWordQueue{cap: capacity, cmp: cmp}
	q.entries.cmp = cmp
	return q
}

// Capacity returns the configured capacity.
func (q *SuggestWordQueue) Capacity() int { return q.cap }

// Size returns the current number of entries.
func (q *SuggestWordQueue) Size() int { return q.entries.Len() }

// Insert adds w; if the queue is full the smallest entry is evicted when w
// outranks it. Returns whether w was kept.
func (q *SuggestWordQueue) Insert(w *SuggestWord) bool {
	if q.entries.Len() < q.cap {
		heap.Push(&q.entries, w)
		return true
	}
	if q.cmp(w, q.entries.peek()) > 0 {
		q.entries.entries[0] = w
		heap.Fix(&q.entries, 0)
		return true
	}
	return false
}

// Pop removes and returns the smallest entry.
func (q *SuggestWordQueue) Pop() *SuggestWord {
	if q.entries.Len() == 0 {
		return nil
	}
	return heap.Pop(&q.entries).(*SuggestWord)
}

type suggestWordHeap struct {
	entries []*SuggestWord
	cmp     CompareFn
}

func (h suggestWordHeap) Len() int { return len(h.entries) }
func (h suggestWordHeap) Less(i, j int) bool {
	return h.cmp(h.entries[i], h.entries[j]) < 0
}
func (h suggestWordHeap) Swap(i, j int) { h.entries[i], h.entries[j] = h.entries[j], h.entries[i] }
func (h *suggestWordHeap) Push(x any) { h.entries = append(h.entries, x.(*SuggestWord)) }
func (h *suggestWordHeap) Pop() any {
	old := h.entries
	n := len(old)
	x := old[n-1]
	h.entries = old[:n-1]
	return x
}
func (h *suggestWordHeap) peek() *SuggestWord {
	if len(h.entries) == 0 {
		return nil
	}
	return h.entries[0]
}
