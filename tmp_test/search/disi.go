package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// DisiWrapper wraps a Scorer and its current DocID to simplify priority queue operations.
type DisiWrapper struct {
	scorer     Scorer
	doc        int
	cost       int64
	maxScore   float32
	scaledMax  int64
	next       *DisiWrapper
}

func NewDisiWrapper(scorer Scorer, isLead bool) *DisiWrapper {
	return &DisiWrapper{
		scorer: scorer,
		doc:    scorer.DocID(),
		cost:   scorer.Iterator().Cost(),
	}
}

func (dw *DisiWrapper) UpdateDoc(doc int) {
	dw.doc = doc
}

// DisiPriorityQueue is a priority queue for DisiWrappers.
type DisiPriorityQueue struct {
	heap []*DisiWrapper
}

func NewDisiPriorityQueue(maxSize int) *DisiPriorityQueue {
	return &DisiPriorityQueue{
		heap: make([]*DisiWrapper, 0, maxSize),
	}
}

func (pq *DisiPriorityQueue) Add(w *DisiWrapper) {
	pq.heap = append(pq.heap, w)
	pq.upHeap(len(pq.heap) - 1)
}

func (pq *DisiPriorityQueue) Pop() *DisiWrapper {
	if len(pq.heap) == 0 {
		return nil
	}
	top := pq.heap[0]
	pq.heap[0] = pq.heap[len(pq.heap)-1]
	pq.heap = pq.heap[:len(pq.heap)-1]
	pq.downHeap(0)
	return top
}

func (pq *DisiPriorityQueue) Top() *DisiWrapper {
	if len(pq.heap) == 0 {
		return nil
	}
	return pq.heap[0]
}

func (pq *DisiPriorityQueue) Size() int {
	return len(pq.heap)
}

func (pq *DisiPriorityQueue) upHeap(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if pq.heap[i].doc < pq.heap[p].doc {
			pq.heap[i], pq.heap[p] = pq.heap[p], pq.heap[i]
			i = p
		} else {
			break
		}
	}
}

func (pq *DisiPriorityQueue) downHeap(i int) {
	for {
		l := 2*i + 1
		r := 2*i + 2
		smallest := i
		if l < len(pq.heap) && pq.heap[l].doc < pq.heap[smallest].doc {
			smallest = l
		}
		if r < len(pq.heap) && pq.heap[r].doc < pq.heap[smallest].doc {
			smallest = r
		}
		if smallest != i {
			pq.heap[i], pq.heap[smallest] = pq.heap[smallest], pq.heap[i]
			i = smallest
		} else {
			break
		}
	}
}

func (pq *DisiPriorityQueue) Clear() {
	pq.heap = pq.heap[:0]
}
