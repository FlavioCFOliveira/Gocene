// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanPositionQueue.java

package spans

// SpanPositionQueue is a min-heap of Spans ordered by (startPosition, endPosition).
//
// Mirrors org.apache.lucene.queries.spans.SpanPositionQueue (Lucene 10.4.0).
type SpanPositionQueue struct {
	heap []Spans
	size int
}

// NewSpanPositionQueue creates a new SpanPositionQueue with the given maximum size.
func NewSpanPositionQueue(maxSize int) *SpanPositionQueue {
	return &SpanPositionQueue{
		heap: make([]Spans, maxSize),
		size: 0,
	}
}

// Size returns the number of elements.
func (pq *SpanPositionQueue) Size() int { return pq.size }

// Top returns the Spans with the smallest position.
func (pq *SpanPositionQueue) Top() Spans {
	if pq.size == 0 {
		return nil
	}
	return pq.heap[0]
}

// Add adds a Spans to the queue.
func (pq *SpanPositionQueue) Add(s Spans) {
	pq.heap[pq.size] = s
	pq.upHeap(pq.size)
	pq.size++
}

// Pop removes and returns the minimum element.
func (pq *SpanPositionQueue) Pop() Spans {
	if pq.size == 0 {
		return nil
	}
	result := pq.heap[0]
	pq.size--
	pq.heap[0] = pq.heap[pq.size]
	pq.heap[pq.size] = nil
	pq.downHeap(pq.size)
	return result
}

// UpdateTop re-heapifies after the top has been mutated.
func (pq *SpanPositionQueue) UpdateTop() Spans {
	pq.downHeap(pq.size)
	return pq.heap[0]
}

func spanLessThan(s1, s2 Spans) bool {
	start1 := s1.StartPosition()
	start2 := s2.StartPosition()
	if start1 < start2 {
		return true
	}
	if start1 == start2 {
		return s1.EndPosition() < s2.EndPosition()
	}
	return false
}

func (pq *SpanPositionQueue) upHeap(i int) {
	node := pq.heap[i]
	j := (i - 1) >> 1
	for j >= 0 && spanLessThan(node, pq.heap[j]) {
		pq.heap[i] = pq.heap[j]
		i = j
		if i == 0 {
			break
		}
		j = (i - 1) >> 1
	}
	pq.heap[i] = node
}

func (pq *SpanPositionQueue) downHeap(size int) {
	if size == 0 {
		return
	}
	i := 0
	node := pq.heap[0]
	j := 1
	if j < size {
		k := j + 1
		if k < size && spanLessThan(pq.heap[k], pq.heap[j]) {
			j = k
		}
		if spanLessThan(pq.heap[j], node) {
			for {
				pq.heap[i] = pq.heap[j]
				i = j
				j = (i << 1) + 1
				k = j + 1
				if k < size && spanLessThan(pq.heap[k], pq.heap[j]) {
					j = k
				}
				if !(j < size && spanLessThan(pq.heap[j], node)) {
					break
				}
			}
			pq.heap[i] = node
		}
	}
}
