// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// HitQueue is a bounded min-priority-queue of ScoreDoc instances ordered so
// the head of the queue is the weakest hit (lowest score, ties broken by
// highest doc id, which is "smaller" in Lucene's HitQueue sense).
//
// Mirrors org.apache.lucene.search.HitQueue. Pre-populate fills the heap with
// sentinel entries so the queue immediately holds Size elements, matching
// Lucene's idiom for batch insertion paths.
type HitQueue struct {
	heap []*ScoreDoc
	size int // maximum capacity
}

// NewHitQueue creates a HitQueue with the given capacity. If prePopulate is
// true, the queue is filled with sentinel entries (MaxInt doc, -Inf score) so
// the heap is immediately full.
func NewHitQueue(size int, prePopulate bool) *HitQueue {
	q := &HitQueue{size: size, heap: make([]*ScoreDoc, 0, size+1)}
	if prePopulate {
		neg := float32(math.Inf(-1))
		for i := 0; i < size; i++ {
			q.heap = append(q.heap, &ScoreDoc{Doc: math.MaxInt32, Score: neg})
		}
	}
	return q
}

// lessThan reports whether a ranks below b: lower score wins, ties broken by
// higher doc id (matching HitQueue's comparison contract).
func (q *HitQueue) lessThan(a, b *ScoreDoc) bool {
	if a.Score != b.Score {
		return a.Score < b.Score
	}
	return a.Doc > b.Doc
}

// Top returns the current minimum element (the weakest hit), or nil if empty.
func (q *HitQueue) Top() *ScoreDoc {
	if len(q.heap) == 0 {
		return nil
	}
	return q.heap[0]
}

// Size returns the number of elements currently in the queue.
func (q *HitQueue) Size() int { return len(q.heap) }

// Capacity returns the configured maximum size.
func (q *HitQueue) Capacity() int { return q.size }

// Add appends a new element to the queue. The queue may exceed its capacity
// briefly; callers that want to enforce capacity should use InsertWithOverflow.
func (q *HitQueue) Add(sd *ScoreDoc) {
	q.heap = append(q.heap, sd)
	q.siftUp(len(q.heap) - 1)
}

// InsertWithOverflow inserts sd and, if the queue is at capacity, pops and
// returns the previous minimum element (which sd has just displaced). If sd is
// not competitive (i.e. weaker than the current minimum), sd itself is
// returned and the queue is unchanged.
func (q *HitQueue) InsertWithOverflow(sd *ScoreDoc) *ScoreDoc {
	if len(q.heap) < q.size {
		q.Add(sd)
		return nil
	}
	if !q.lessThan(q.heap[0], sd) {
		// sd is weaker or equal: do not insert.
		return sd
	}
	displaced := q.heap[0]
	q.heap[0] = sd
	q.siftDown(0)
	return displaced
}

// Pop removes and returns the minimum element, or nil if empty.
func (q *HitQueue) Pop() *ScoreDoc {
	if len(q.heap) == 0 {
		return nil
	}
	top := q.heap[0]
	last := len(q.heap) - 1
	q.heap[0] = q.heap[last]
	q.heap[last] = nil
	q.heap = q.heap[:last]
	if len(q.heap) > 0 {
		q.siftDown(0)
	}
	return top
}

// UpdateTop must be called after the caller has mutated the head element to
// re-establish the heap invariant. It returns the new head.
func (q *HitQueue) UpdateTop() *ScoreDoc {
	if len(q.heap) == 0 {
		return nil
	}
	q.siftDown(0)
	return q.heap[0]
}

func (q *HitQueue) siftUp(i int) {
	for i > 0 {
		parent := (i - 1) >> 1
		if !q.lessThan(q.heap[i], q.heap[parent]) {
			return
		}
		q.heap[parent], q.heap[i] = q.heap[i], q.heap[parent]
		i = parent
	}
}

func (q *HitQueue) siftDown(i int) {
	n := len(q.heap)
	for {
		l := i*2 + 1
		if l >= n {
			return
		}
		c := l
		if r := l + 1; r < n && q.lessThan(q.heap[r], q.heap[l]) {
			c = r
		}
		if !q.lessThan(q.heap[c], q.heap[i]) {
			return
		}
		q.heap[c], q.heap[i] = q.heap[i], q.heap[c]
		i = c
	}
}
