// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Node represents an operation in the delete queue.
type Node interface {
	Apply(bufferedDeletes *BufferedUpdates, docIDUpTo int)
	IsDelete() bool
	String() string
	Next() Node
	SetNext(Node)
}

type baseNode struct {
	next Node
}

func (b *baseNode) Next() Node      { return b.next }
func (b *baseNode) SetNext(n Node) { b.next = n }

type termNode struct {
	baseNode
	term Term
}

func (n *termNode) Apply(bufferedDeletes *BufferedUpdates, docIDUpTo int) {
	bufferedDeletes.AddTerm(n.term, docIDUpTo)
}

func (n *termNode) IsDelete() bool { return true }
func (n *termNode) String() string { return "del=" + n.term.String() }

type queryNode struct {
	baseNode
	query Query
}

func (n *queryNode) Apply(bufferedDeletes *BufferedUpdates, docIDUpTo int) {
	bufferedDeletes.AddQuery(n.query, docIDUpTo)
}

func (n *queryNode) IsDelete() bool { return true }
func (n *queryNode) String() string { return "del=" + n.query.String() }

type queryArrayNode struct {
	baseNode
	queries []Query
}

func (n *queryArrayNode) Apply(bufferedDeletes *BufferedUpdates, docIDUpTo int) {
	for _, q := range n.queries {
		bufferedDeletes.AddQuery(q, docIDUpTo)
	}
}

func (n *queryArrayNode) IsDelete() bool { return true }
func (n *queryArrayNode) String() string { return "dels=..." }

type termArrayNode struct {
	baseNode
	terms []Term
}

func (n *termArrayNode) Apply(bufferedDeletes *BufferedUpdates, docIDUpTo int) {
	for _, t := range n.terms {
		bufferedDeletes.AddTerm(t, docIDUpTo)
	}
}

func (n *termArrayNode) IsDelete() bool { return true }
func (n *termArrayNode) String() string { return "dels=..." }

type docValuesUpdatesNode struct {
	baseNode
	updates []*DocValuesUpdate
}

func (n *docValuesUpdatesNode) Apply(bufferedDeletes *BufferedUpdates, docIDUpTo int) {
	for _, u := range n.updates {
		switch u.Type() {
		case DocValuesTypeNumeric:
			bufferedDeletes.AddNumericUpdate(u.(*NumericDocValuesUpdate), docIDUpTo)
		case DocValuesTypeBinary:
			bufferedDeletes.AddBinaryUpdate(u.(*BinaryDocValuesUpdate), docIDUpTo)
		}
	}
}

func (n *docValuesUpdatesNode) IsDelete() bool { return false }
func (n *docValuesUpdatesNode) String() string {
	if len(n.updates) == 0 {
		return "docValuesUpdates: []"
	}
	s := "docValuesUpdates: term=" + n.updates[0].Term().String() + "; updates: ["
	for i, u := range n.updates {
		if i > 0 {
			s += ","
		}
		s += u.Field() + ":" + u.ValueToString()
	}
	s += "]"
	return s
}

// NewTermNode creates a new node for a term deletion.
func NewTermNode(term Term) Node {
	return &termNode{term: term}
}

// NewQueryNode creates a new node for a query deletion.
func NewQueryNode(query Query) Node {
	return &queryNode{query: query}
}

// NewDocValuesUpdatesNode creates a new node for doc values updates.
func NewDocValuesUpdatesNode(updates []*DocValuesUpdate) Node {
	return &docValuesUpdatesNode{updates: updates}
}

// DeleteSlice maintains a head and tail of the delete queue.
type DeleteSlice struct {
	sliceHead Node
	sliceTail Node
}

func NewDeleteSlice(currentTail Node) *DeleteSlice {
	return &DeleteSlice{
		sliceHead: currentTail,
		sliceTail: currentTail,
	}
}

func (s *DeleteSlice) Apply(bufferedDeletes *BufferedUpdates, docIDUpTo int) {
	if s.sliceHead == s.sliceTail {
		return
	}
	current := s.sliceHead
	for {
		current = current.Next()
		if current == nil {
			panic("slice property violated: head and tail must be connected")
		}
		current.Apply(bufferedDeletes, docIDUpTo)
		if current == s.sliceTail {
			break
		}
	}
	s.reset()
}

func (s *DeleteSlice) reset() {
	s.sliceHead = s.sliceTail
}

func (s *DeleteSlice) IsEmpty() bool {
	return s.sliceHead == s.sliceTail
}

// DocumentsWriterDeleteQueue is a non-blocking linked pending deletes queue.
type DocumentsWriterDeleteQueue struct {
	mu sync.Mutex

	tail Node
	closed bool

	globalSlice *DeleteSlice
	globalBufferedUpdates *BufferedUpdates

	generation int64
	nextSeqNo atomic.Int64
	startSeqNo int64
	maxSeqNo atomic.Int64
	advanced bool

	previousMaxSeqId func() int64
	infoStream InfoStream
}

func NewDocumentsWriterDeleteQueue(infoStream InfoStream) *DocumentsWriterDeleteQueue {
	return NewDocumentsWriterDeleteQueueWithParams(infoStream, 0, 1, func() int64 { return 0 })
}

func NewDocumentsWriterDeleteQueueWithParams(infoStream InfoStream, generation int64, startSeqNo int64, prevMaxSeqId func() int64) *DocumentsWriterDeleteQueue {
	// Sentinel node
	sentinel := &termNode{} // minimal implementation of Node
	// We override Apply for sentinel to do nothing
	sentinel.Apply = func(b *BufferedUpdates, d int) {}

	dq := &DocumentsWriterDeleteQueue{
		infoStream: infoStream,
		generation: generation,
		startSeqNo: startSeqNo,
		previousMaxSeqId: prevMaxSeqId,
	}
	dq.nextSeqNo.Store(startSeqNo)
	dq.tail = sentinel
	dq.globalSlice = NewDeleteSlice(sentinel)
	dq.globalBufferedUpdates = NewBufferedUpdates("global")
	dq.maxSeqNo.Store(int64(9223372036854775807)) // Long.MAX_VALUE

	return dq
}

func (dq *DocumentsWriterDeleteQueue) GetNextSequenceNumber() int64 {
	seqNo := dq.nextSeqNo.Add(1) - 1
	if seqNo > dq.maxSeqNo.Load() {
		panic(fmt.Sprintf("seqNo=%d vs maxSeqNo=%d", seqNo, dq.maxSeqNo.Load()))
	}
	return seqNo
}

func (dq *DocumentsWriterDeleteQueue) Add(newNode Node) int64 {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	dq.ensureOpen()
	dq.tail.SetNext(newNode)
	dq.tail = newNode

	return dq.GetNextSequenceNumber()
}

func (dq *DocumentsWriterDeleteQueue) AddWithSlice(newNode Node, slice *DeleteSlice) int64 {
	seqNo := dq.Add(newNode)
	slice.sliceTail = newNode
	return seqNo
}

func (dq *DocumentsWriterDeleteQueue) AdvanceQueue(maxNumPendingOps int) *DocumentsWriterDeleteQueue {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	if dq.advanced {
		panic("queue was already advanced")
	}
	dq.advanced = true

	lastSeqNo := dq.nextSeqNo.Load() - 1
	seqNo := lastSeqNo + int64(maxNumPendingOps) + 1
	dq.maxSeqNo.Store(seqNo)

	// Create successor queue
	prevMax := func() int64 {
		return dq.nextSeqNo.Load() - 1
	}

	return NewDocumentsWriterDeleteQueueWithParams(
		dq.infoStream,
		dq.generation+1,
		seqNo+1,
		prevMax,
	)
}

func (dq *DocumentsWriterDeleteQueue) ensureOpen() {
	if dq.closed {
		panic("delete queue is already closed")
	}
}

func (dq *DocumentsWriterDeleteQueue) Close() {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	if dq.anyChanges() {
		panic("Can't close queue unless all changes are applied")
	}
	dq.closed = true
}

func (dq *DocumentsWriterDeleteQueue) anyChanges() bool {
	return dq.globalBufferedUpdates.Any() ||
		!dq.globalSlice.IsEmpty() ||
		dq.globalSlice.sliceTail != dq.tail ||
		dq.tail.Next() != nil
}

func (dq *DocumentsWriterDeleteQueue) TryApplyGlobalSlice() bool {
	dq.mu.Lock()
	defer dq.mu.Unlock()

	dq.ensureOpen()
	if dq.globalSlice.sliceTail != dq.tail {
		dq.globalSlice.sliceTail = dq.tail
		dq.globalSlice.Apply(dq.globalBufferedUpdates, 0) // docIDUpTo not used by global
		return true
	}
	return false
}
