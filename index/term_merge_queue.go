// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// termMergeQueue is a binary min-heap of termsEnumWithSlice entries ordered by
// their current term (least term at the top). It mirrors
// org.apache.lucene.index.MultiTermsEnum.TermMergeQueue, which extends
// org.apache.lucene.util.PriorityQueue.
//
// Like Lucene's PriorityQueue the heap array is 1-based: index 0 is unused, the
// root lives at index 1, and a node at index i has children at 2i and 2i+1.
// This layout lets fillTop walk the heap with the same index arithmetic Lucene
// uses to collect every entry tied on the least term.
type termMergeQueue struct {
	// heap is the 1-based backing array; heap[0] is a sentinel slot.
	heap []*termsEnumWithSlice
	// sz is the number of elements currently in the heap.
	sz int
	// maxSize is the configured capacity (number of subs).
	maxSize int
	// stack scratch reused by fillTop to avoid per-call allocation.
	stack []int
}

// newTermMergeQueue builds an empty queue able to hold maxSize entries.
func newTermMergeQueue(maxSize int) *termMergeQueue {
	// stack is sized maxSize+1 to mirror the 1-based heap index range it walks
	// and to leave headroom for the transient two-child push per pop.
	return &termMergeQueue{
		heap:    make([]*termsEnumWithSlice, maxSize+1),
		maxSize: maxSize,
		stack:   make([]int, maxSize+1),
	}
}

// lessThan reports whether a's current term sorts before b's. Mirrors
// TermMergeQueue.lessThan (compareTo on the current BytesRef). Within a single
// field Term.CompareTo reduces to byte comparison, matching Lucene exactly.
func (q *termMergeQueue) lessThan(a, b *termsEnumWithSlice) bool {
	return a.current.CompareTo(b.current) < 0
}

// size returns the number of entries in the queue.
func (q *termMergeQueue) size() int { return q.sz }

// clear empties the queue, releasing entry references.
func (q *termMergeQueue) clear() {
	for i := 1; i <= q.sz; i++ {
		q.heap[i] = nil
	}
	q.sz = 0
}

// top returns the least entry without removing it, or nil when empty.
func (q *termMergeQueue) top() *termsEnumWithSlice {
	if q.sz == 0 {
		return nil
	}
	return q.heap[1]
}

// add inserts an entry, sifting it up to restore the heap invariant. Mirrors
// PriorityQueue.add.
func (q *termMergeQueue) add(entry *termsEnumWithSlice) {
	if q.sz >= q.maxSize {
		panic("termMergeQueue.add: queue is full")
	}
	q.sz++
	q.heap[q.sz] = entry
	q.upHeap(q.sz)
}

// pop removes and returns the least entry, or nil when empty. Mirrors
// PriorityQueue.pop.
func (q *termMergeQueue) pop() *termsEnumWithSlice {
	if q.sz == 0 {
		return nil
	}
	result := q.heap[1]
	q.heap[1] = q.heap[q.sz]
	q.heap[q.sz] = nil
	q.sz--
	q.downHeap(1)
	return result
}

func (q *termMergeQueue) upHeap(origPos int) {
	i := origPos
	node := q.heap[i]
	j := i >> 1
	for j > 0 && q.lessThan(node, q.heap[j]) {
		q.heap[i] = q.heap[j]
		i = j
		j = j >> 1
	}
	q.heap[i] = node
}

func (q *termMergeQueue) downHeap(i int) {
	node := q.heap[i]
	j := i << 1 // left child
	k := j + 1  // right child
	if k <= q.sz && q.lessThan(q.heap[k], q.heap[j]) {
		j = k
	}
	for j <= q.sz && q.lessThan(q.heap[j], node) {
		q.heap[i] = q.heap[j]
		i = j
		j = i << 1
		k = j + 1
		if k <= q.sz && q.lessThan(q.heap[k], q.heap[j]) {
			j = k
		}
	}
	q.heap[i] = node
}

// updateTop re-establishes the heap invariant after the caller has mutated the
// root entry in place (e.g. advanced its sub-TermsEnum to a larger term) and
// returns the new least entry. Mirrors PriorityQueue.updateTop.
func (q *termMergeQueue) updateTop() *termsEnumWithSlice {
	q.downHeap(1)
	return q.heap[1]
}

// fillTop collects the top entry plus every entry tied with it on the least
// term into tops and returns the count. The collected entries are LEFT IN the
// queue: MultiTermsEnum.pushTop later advances each of them in place (via
// updateTop) or removes it (via pop) when its sub-enum is exhausted. This
// mirrors org.apache.lucene.index.MultiTermsEnum.TermMergeQueue.fillTop exactly.
//
// It walks the heap with an explicit stack, exploiting the heap property that
// entries tied with the root can only be reached by descending through other
// tied entries: starting from the root, a child is part of the tie iff its term
// equals the root's, and only tied children are expanded further.
func (q *termMergeQueue) fillTop(tops []*termsEnumWithSlice) int {
	size := q.sz
	if size == 0 {
		return 0
	}
	tops[0] = q.top()
	numTop := 1
	q.stack[0] = 1
	stackLen := 1

	for stackLen != 0 {
		stackLen--
		index := q.stack[stackLen]
		leftChild := index << 1
		end := leftChild + 1
		if size < end {
			end = size
		}
		for child := leftChild; child <= end; child++ {
			te := q.heap[child]
			if te.current.CompareTo(tops[0].current) == 0 {
				tops[numTop] = te
				numTop++
				q.stack[stackLen] = child
				stackLen++
			}
		}
	}
	return numTop
}
