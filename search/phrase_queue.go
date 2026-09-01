// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "container/heap"

// PhraseQueue is a priority queue for advancing the min position among PhrasePositions.
type PhraseQueue []*PhrasePositions

func (pq PhraseQueue) Len() int { return len(pq) }
func (pq PhraseQueue) Less(i, j int) bool {
	if pq[i].position != pq[j].position {
		return pq[i].position < pq[j].position
	}
	return pq[i].offset < pq[j].offset
}
func (pq PhraseQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

func (pq *PhraseQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*PhrasePositions))
}

func (pq *PhraseQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func (pq *PhraseQueue) Clear() {
	*pq = (*pq)[:0]
}

func (pq *PhraseQueue) Add(pp *PhrasePositions) {
	heap.Push(pq, pp)
}

func (pq *PhraseQueue) PopMin() *PhrasePositions {
	return heap.Pop(pq).(*PhrasePositions)
}

func (pq *PhraseQueue) Top() *PhrasePositions {
	if len(*pq) == 0 {
		return nil
	}
	return (*pq)[0]
}
