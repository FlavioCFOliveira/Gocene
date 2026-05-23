package document

// SuggestScoreDocPriorityQueue is a bounded min-heap of SuggestScoreDoc
// entries. Priority is based on score (lower score = higher priority in the
// heap so that the minimum can be evicted when the heap is full). Ties in
// score are broken by key (lexicographic) and then by doc id.
//
// The heap keeps up to size entries with the highest scores; call GetResults
// to drain them in descending score order.
//
// Mirrors org.apache.lucene.search.suggest.document.SuggestScoreDocPriorityQueue.
type SuggestScoreDocPriorityQueue struct {
	maxSize int
	heap    []*SuggestScoreDoc
}

// NewSuggestScoreDocPriorityQueue creates a new bounded priority queue with
// the given maximum capacity.
func NewSuggestScoreDocPriorityQueue(size int) *SuggestScoreDocPriorityQueue {
	return &SuggestScoreDocPriorityQueue{maxSize: size}
}

// lessThan returns true when a should be evicted before b (a has lower
// priority). Mirrors SuggestScoreDocPriorityQueue.lessThan.
func (q *SuggestScoreDocPriorityQueue) lessThan(a, b *SuggestScoreDoc) bool {
	if a.Score == b.Score {
		cmp := compareCharSequence(a.Key, b.Key)
		if cmp != 0 {
			return cmp > 0 // prefer lexicographically smaller key
		}
		return a.Doc > b.Doc // prefer smaller doc id
	}
	return a.Score < b.Score
}

// Add inserts doc into the queue. If the queue is already full and doc has a
// higher priority than the current minimum, the minimum is evicted.
// Returns the evicted element (or nil if nothing was evicted).
func (q *SuggestScoreDocPriorityQueue) Add(doc *SuggestScoreDoc) *SuggestScoreDoc {
	if len(q.heap) < q.maxSize {
		q.heap = append(q.heap, doc)
		q.upHeap(len(q.heap) - 1)
		return nil
	}
	if len(q.heap) == 0 {
		return doc
	}
	top := q.heap[0]
	if !q.lessThan(doc, top) {
		// doc has higher or equal priority; evict the current minimum.
		q.heap[0] = doc
		q.downHeap(0)
		return top
	}
	return doc
}

// Size returns the number of elements currently in the queue.
func (q *SuggestScoreDocPriorityQueue) Size() int { return len(q.heap) }

// GetResults drains the heap and returns the accumulated entries in
// descending score order (best first). Mirrors
// SuggestScoreDocPriorityQueue.getResults().
func (q *SuggestScoreDocPriorityQueue) GetResults() []*SuggestScoreDoc {
	n := len(q.heap)
	res := make([]*SuggestScoreDoc, n)
	for i := n - 1; i >= 0; i-- {
		res[i] = q.pop()
	}
	return res
}

// pop removes and returns the minimum element (root of the min-heap).
func (q *SuggestScoreDocPriorityQueue) pop() *SuggestScoreDoc {
	if len(q.heap) == 0 {
		return nil
	}
	top := q.heap[0]
	last := len(q.heap) - 1
	q.heap[0] = q.heap[last]
	q.heap = q.heap[:last]
	if len(q.heap) > 0 {
		q.downHeap(0)
	}
	return top
}

// upHeap restores the heap invariant after an insertion at position i.
func (q *SuggestScoreDocPriorityQueue) upHeap(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if q.lessThan(q.heap[parent], q.heap[i]) {
			// parent has lower priority — stop.
			break
		}
		q.heap[i], q.heap[parent] = q.heap[parent], q.heap[i]
		i = parent
	}
}

// downHeap restores the heap invariant after replacing the root at position i.
func (q *SuggestScoreDocPriorityQueue) downHeap(i int) {
	n := len(q.heap)
	for {
		left := 2*i + 1
		right := 2*i + 2
		smallest := i
		if left < n && q.lessThan(q.heap[left], q.heap[smallest]) {
			smallest = left
		}
		if right < n && q.lessThan(q.heap[right], q.heap[smallest]) {
			smallest = right
		}
		if smallest == i {
			break
		}
		q.heap[i], q.heap[smallest] = q.heap[smallest], q.heap[i]
		i = smallest
	}
}
