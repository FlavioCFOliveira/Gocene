// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocumentsWriterDeleteQueue is a non-blocking linked pending deletes queue.
//
// In contrast to other queue implementation we only maintain the tail of the queue.
// A delete queue is always used in a context of a set of DWPTs and a global delete pool.
// Each of the DWPT and the global pool need to maintain their 'own' head of the queue
// (as a DeleteSlice instance per DocumentsWriterPerThread).
// The difference between the DWPT and the global pool is that the DWPT starts
// maintaining a head once it has added its first document since for its segments
// private deletes only the deletes after that document are relevant.
// The global pool instead starts maintaining the head once this instance is created
// by taking the sentinel instance as its initial head.
//
// Since each DeleteSlice maintains its own head and the list is only single linked
// the garbage collector takes care of pruning the list for us. All Nodes in the
// list that are still relevant should be either directly or indirectly referenced
// by one of the DWPT's private DeleteSlice or by the global BufferedUpdates slice.
//
// Each DWPT as well as the global delete pool maintain their private DeleteSlice instance.
// In the DWPT case updating a slice is equivalent to atomically finishing the document.
// The slice update guarantees a "happens before" relationship to all other updates in
// the same indexing session. When a DWPT updates a document it:
//
//  1. consumes a document and finishes its processing
//  2. updates its private DeleteSlice either by calling UpdateSlice(DeleteSlice)
//     or Add(Node, DeleteSlice) (if the document has a delTerm)
//  3. applies all deletes in the slice to its private BufferedUpdates and resets it
//  4. increments its internal document id
//
// The DWPT also doesn't apply its current documents delete term until it has updated
// its delete slice which ensures the consistency of the update. If the update fails
// before the DeleteSlice could have been updated the deleteTerm will also not be
// added to its private deletes neither to the global deletes.
type DocumentsWriterDeleteQueue struct {
	// the current end (latest delete operation) in the delete queue.
	//
	// Java declares this slot as `volatile Node<?> tail`. Go cannot publish
	// an interface value with atomic.Pointer, and atomic.Value rejects a
	// second Store with a different dynamic type — every node kind here is a
	// different type — so the volatile slot is rendered as an atomic.Pointer
	// over the nodeRef holder below. Reads go through loadTail.
	tail atomic.Pointer[nodeRef]

	closed atomic.Bool

	// Used to record deletes against all prior (already written to disk) segments.
	// Whenever any segment flushes, we bundle up this set of deletes and insert
	// into the buffered updates stream before the newly flushed segment(s).
	globalSlice *DeleteSlice

	globalBufferedUpdates *BufferedUpdates

	// only acquired to update the global deletes:
	globalBufferLock sync.Mutex

	generation int64

	// Generates the sequence number that IW returns to callers changing the index,
	// showing the effective serialization of all operations.
	nextSeqNo atomic.Int64

	infoStream util.InfoStream

	maxSeqNo int64

	startSeqNo       int64
	previousMaxSeqId func() int64
	advanced         bool

	// mu protects the queue tail and the advanced state for synchronized operations.
	mu sync.Mutex
}

// nodeRef boxes a Node so the queue tail can be published atomically. See
// the tail field on DocumentsWriterDeleteQueue.
type nodeRef struct {
	node Node
}

// loadTail reads the current queue tail. Mirrors a volatile read of Java's
// `tail` field.
func (d *DocumentsWriterDeleteQueue) loadTail() Node {
	return d.tail.Load().node
}

// storeTail publishes a new queue tail. Mirrors a volatile write of Java's
// `tail` field.
func (d *DocumentsWriterDeleteQueue) storeTail(n Node) {
	d.tail.Store(&nodeRef{node: n})
}

const maxInt = 2147483647

func NewDocumentsWriterDeleteQueue(infoStream util.InfoStream) *DocumentsWriterDeleteQueue {
	// seqNo must start at 1 because some APIs negate this to also return a boolean
	return newDocumentsWriterDeleteQueue(infoStream, 0, 1, func() int64 { return 0 })
}

func newDocumentsWriterDeleteQueue(
	infoStream util.InfoStream,
	generation int64,
	startSeqNo int64,
	previousMaxSeqId func() int64,
) *DocumentsWriterDeleteQueue {
	globalBufferedUpdates := NewBufferedUpdates("global")
	nextSeqNo := atomic.Int64{}
	nextSeqNo.Store(startSeqNo)

	value := previousMaxSeqId()
	if value > startSeqNo {
		panic(fmt.Sprintf("illegal max sequence ID: %d start was: %d", value, startSeqNo))
	}

	/*
	 * we use a sentinel instance as our initial tail. No slice will ever try to
	 * apply this tail since the head is always omitted.
	 */
	sentinel := &NodeBase{item: nil}
	dwdq := &DocumentsWriterDeleteQueue{
		globalBufferedUpdates: globalBufferedUpdates,
		generation:            generation,
		nextSeqNo:             nextSeqNo,
		startSeqNo:            startSeqNo,
		previousMaxSeqId:      previousMaxSeqId,
		infoStream:            infoStream,
		maxSeqNo:              int64(2147483647), // Long.MAX_VALUE in Java, but seqNo uses int32 logically in some places? No, it's long.
		// Wait, Java Long.MAX_VALUE is 9223372036854775807.
	}
	// Correction: Use actual MaxInt64 for maxSeqNo.
	dwdq.maxSeqNo = 9223372036854775807

	dwdq.storeTail(sentinel)
	dwdq.globalSlice = NewDeleteSlice(sentinel)

	return dwdq
}

func (d *DocumentsWriterDeleteQueue) AddDelete(queries ...Query) int64 {
	seqNo := d.AddNode(newNodeQueryArray(queries))
	d.tryApplyGlobalSlice()
	return seqNo
}

func (d *DocumentsWriterDeleteQueue) AddDeleteTerms(terms ...Term) int64 {
	seqNo := d.AddNode(newNodeTermArray(terms))
	d.tryApplyGlobalSlice()
	return seqNo
}

func (d *DocumentsWriterDeleteQueue) AddDocValuesUpdates(updates ...DocValuesUpdate) int64 {
	seqNo := d.AddNode(newNodeDocValuesUpdates(updates))
	d.tryApplyGlobalSlice()
	return seqNo
}

func NewTermNode(term Term) Node {
	return &termNode{NodeBase: NodeBase{item: term}, term: term}
}

func NewQueryNode(query Query) Node {
	return &queryNode{NodeBase: NodeBase{item: query}, query: query}
}

func NewDocValuesUpdatesNode(updates ...DocValuesUpdate) Node {
	return &docValuesUpdatesNode{NodeBase: NodeBase{item: updates}, updates: updates}
}

// invariant for document update
func (d *DocumentsWriterDeleteQueue) AddWithSlice(deleteNode Node, slice *DeleteSlice) int64 {
	seqNo := d.AddNode(deleteNode)
	/*
	 * this is an update request where the term is the updated documents
	 * delTerm. in that case we need to guarantee that this insert is atomic
	 * with regards to the given delete slice. This means if two threads try to
	 * update the same document with in turn the same delTerm one of them must
	 * win. By taking the Node we have created for our del term as the new tail
	 * it is guaranteed that if another thread adds the same right after us we
	 * will apply this delete next time we update our slice and one of the two
	 * competing updates wins!
	 */
	slice.sliceTail = deleteNode
	if slice.sliceHead == slice.sliceTail {
		panic("slice head and tail must differ after add")
	}
	d.tryApplyGlobalSlice()
	return seqNo
}

func (d *DocumentsWriterDeleteQueue) AddNode(newNode Node) int64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.ensureOpen()

	// The Java code does: tail.next = newNode; this.tail = newNode;
	// Since tail is a Node, and Node is an interface, we need to handle the next pointer.
	// The implementation Nodes will embed NodeBase.

	currentTail := d.loadTail()
	currentTail.base().next = newNode
	d.storeTail(newNode)

	return d.getNextSequenceNumber()
}

func (d *DocumentsWriterDeleteQueue) AnyChanges() bool {
	d.globalBufferLock.Lock()
	defer d.globalBufferLock.Unlock()

	return d.anyChangesLocked()
}

// anyChangesLocked is the body of AnyChanges with globalBufferLock already
// held by the caller.
//
// Java's globalBufferLock is a ReentrantLock, so anyChanges() may be -- and is
// -- called from close() and maybeFreezeGlobalBuffer() while the calling thread
// already holds it. Go's sync.Mutex is not reentrant, so those two callers use
// this form instead. The predicate, and the exclusion it is evaluated under,
// are identical either way; this mirrors the split Lucene already makes between
// freezeGlobalBuffer() and freezeGlobalBufferInternal(), whose own contract is
// `assert globalBufferLock.isHeldByCurrentThread()`.
func (d *DocumentsWriterDeleteQueue) anyChangesLocked() bool {
	return d.globalBufferedUpdates.Any() ||
		!d.globalSlice.IsEmpty() ||
		d.globalSlice.sliceTail != d.loadTail() ||
		d.loadTail().base().next != nil
}

func (d *DocumentsWriterDeleteQueue) tryApplyGlobalSlice() {
	if d.globalBufferLock.TryLock() {
		defer d.globalBufferLock.Unlock()
		d.ensureOpen()
		if d.updateSliceNoSeqNo(d.globalSlice) {
			d.globalSlice.apply(d.globalBufferedUpdates, maxInt)
		}
	}
}

func (d *DocumentsWriterDeleteQueue) FreezeGlobalBuffer(callerSlice *DeleteSlice) *FrozenBufferedUpdates {
	d.globalBufferLock.Lock()
	defer d.globalBufferLock.Unlock()

	d.ensureOpen()
	currentTail := d.loadTail()
	if callerSlice != nil {
		callerSlice.sliceTail = currentTail
	}
	return d.freezeGlobalBufferInternal(currentTail)
}

func (d *DocumentsWriterDeleteQueue) MaybeFreezeGlobalBuffer() *FrozenBufferedUpdates {
	d.globalBufferLock.Lock()
	defer d.globalBufferLock.Unlock()

	if !d.closed.Load() {
		return d.freezeGlobalBufferInternal(d.loadTail())
	}
	if d.anyChangesLocked() {
		panic("we are closed but have changes")
	}
	return nil
}

func (d *DocumentsWriterDeleteQueue) freezeGlobalBufferInternal(currentTail Node) *FrozenBufferedUpdates {
	if d.globalSlice.sliceTail != currentTail {
		d.globalSlice.sliceTail = currentTail
		d.globalSlice.apply(d.globalBufferedUpdates, maxInt)
	}

	if d.globalBufferedUpdates.Any() {
		packet, err := NewFrozenBufferedUpdates(d.infoStream, d.globalBufferedUpdates, nil)
		if err != nil {
			panic(err)
		}
		d.globalBufferedUpdates.Clear()
		return packet
	}
	return nil
}

// Clear resets the global slice to the current tail and discards every
// buffered global update. Mirrors DocumentsWriterDeleteQueue.clear().
func (d *DocumentsWriterDeleteQueue) Clear() {
	d.globalBufferLock.Lock()
	defer d.globalBufferLock.Unlock()

	currentTail := d.loadTail()
	d.globalSlice.sliceHead = currentTail
	d.globalSlice.sliceTail = currentTail
	d.globalBufferedUpdates.Clear()
}

// GetBufferedUpdatesTermsSize returns the number of buffered global delete
// terms. Mirrors DocumentsWriterDeleteQueue.getBufferedUpdatesTermsSize().
func (d *DocumentsWriterDeleteQueue) GetBufferedUpdatesTermsSize() int {
	return d.getBufferedUpdatesTermsSize()
}

// GetNextSequenceNumber hands out the next sequence number. Mirrors
// DocumentsWriterDeleteQueue.getNextSequenceNumber().
func (d *DocumentsWriterDeleteQueue) GetNextSequenceNumber() int64 {
	return d.getNextSequenceNumber()
}

func (d *DocumentsWriterDeleteQueue) NewSlice() *DeleteSlice {
	return NewDeleteSlice(d.loadTail())
}

func (d *DocumentsWriterDeleteQueue) UpdateSlice(slice *DeleteSlice) int64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.ensureOpen()
	seqNo := d.getNextSequenceNumber()
	if slice.sliceTail != d.loadTail() {
		slice.sliceTail = d.loadTail()
		seqNo = -seqNo
	}
	return seqNo
}

func (d *DocumentsWriterDeleteQueue) updateSliceNoSeqNo(slice *DeleteSlice) bool {
	if slice.sliceTail != d.loadTail() {
		slice.sliceTail = d.loadTail()
		return true
	}
	return false
}

func (d *DocumentsWriterDeleteQueue) ensureOpen() {
	if d.closed.Load() {
		panic(fmt.Sprintf("This DocumentsWriterDeleteQueue is already closed"))
	}
}

func (d *DocumentsWriterDeleteQueue) IsOpen() bool {
	return !d.closed.Load()
}

func (d *DocumentsWriterDeleteQueue) Close() error {
	d.globalBufferLock.Lock()
	defer d.globalBufferLock.Unlock()

	if d.anyChangesLocked() {
		return fmt.Errorf("Can't close queue unless all changes are applied")
	}
	d.closed.Store(true)
	seqNo := d.nextSeqNo.Load()
	if seqNo > d.maxSeqNo {
		panic(fmt.Sprintf("maxSeqNo must be greater or equal to %d but was %d", seqNo, d.maxSeqNo))
	}
	d.nextSeqNo.Store(d.maxSeqNo + 1)
	return nil
}

func (d *DocumentsWriterDeleteQueue) getBufferedUpdatesTermsSize() int {
	d.globalBufferLock.Lock()
	defer d.globalBufferLock.Unlock()

	currentTail := d.loadTail()
	if d.globalSlice.sliceTail != currentTail {
		d.globalSlice.sliceTail = currentTail
		d.globalSlice.apply(d.globalBufferedUpdates, maxInt)
	}
	return d.globalBufferedUpdates.deleteTerms.size()
}

func (d *DocumentsWriterDeleteQueue) RamBytesUsed() int64 {
	return d.globalBufferedUpdates.RamBytesUsed()
}

func (d *DocumentsWriterDeleteQueue) String() string {
	return fmt.Sprintf("DWDQ: [ generation: %d ]", d.generation)
}

func (d *DocumentsWriterDeleteQueue) getNextSequenceNumber() int64 {
	seqNo := d.nextSeqNo.Add(1) - 1
	if seqNo > d.maxSeqNo {
		panic(fmt.Sprintf("seqNo=%d vs maxSeqNo=%d", seqNo, d.maxSeqNo))
	}
	return seqNo
}

func (d *DocumentsWriterDeleteQueue) GetLastSequenceNumber() int64 {
	return d.nextSeqNo.Load() - 1
}

func (d *DocumentsWriterDeleteQueue) SkipSequenceNumbers(jump int64) {
	d.nextSeqNo.Add(jump)
}

func (d *DocumentsWriterDeleteQueue) GetMaxCompletedSeqNo() int64 {
	if d.startSeqNo < d.nextSeqNo.Load() {
		return d.GetLastSequenceNumber()
	}
	value := d.previousMaxSeqId()
	if value >= d.startSeqNo {
		panic(fmt.Sprintf("illegal max sequence ID: %d start was: %d", value, d.startSeqNo))
	}
	return value
}

func (d *DocumentsWriterDeleteQueue) AdvanceQueue(maxNumPendingOps int) *DocumentsWriterDeleteQueue {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.advanced {
		panic("queue was already advanced")
	}
	d.advanced = true
	seqNo := d.GetLastSequenceNumber() + int64(maxNumPendingOps) + 1
	d.maxSeqNo = seqNo

	nextSeqNoVal := d.nextSeqNo.Load()
	return newDocumentsWriterDeleteQueue(
		d.infoStream,
		d.generation+1,
		seqNo+1,
		func() int64 { return nextSeqNoVal - 1 },
	)
}

func (d *DocumentsWriterDeleteQueue) GetMaxSeqNo() int64 {
	return d.maxSeqNo
}

func (d *DocumentsWriterDeleteQueue) IsAdvanced() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.advanced
}

// Node and its implementations

// Node mirrors the package-private Node<T> class of
// DocumentsWriterDeleteQueue. Java gets the linked-list slot and the item
// from the superclass fields; Go exposes them through base(), which only
// NodeBase implements, so every node in the queue necessarily embeds it.
type Node interface {
	apply(bu *BufferedUpdates, docIDUpto int)
	isDelete() bool
	base() *NodeBase
}

// NodeBase is the Go rendering of the Node<T> base class itself. An instance
// of NodeBase (never a subclass) is used as the queue sentinel, exactly as
// Java uses `new Node<>(null)`; its apply therefore reproduces the base
// class behaviour of refusing to be applied.
type NodeBase struct {
	next Node
	item any
}

func (n *NodeBase) base() *NodeBase { return n }

func (n *NodeBase) apply(bu *BufferedUpdates, docIDUpto int) {
	panic("sentinel item must never be applied")
}

func (n *NodeBase) isDelete() bool { return true }

type DeleteSlice struct {
	sliceHead Node
	sliceTail Node
}

func NewDeleteSlice(currentTail Node) *DeleteSlice {
	if currentTail == nil {
		panic("currentTail must not be nil")
	}
	return &DeleteSlice{
		sliceHead: currentTail,
		sliceTail: currentTail,
	}
}

func (s *DeleteSlice) apply(del *BufferedUpdates, docIDUpto int) {
	if s.sliceHead == s.sliceTail {
		return
	}
	current := s.sliceHead
	for {
		current = current.base().next
		if current == nil {
			panic("slice property violated between the head on the tail must not be a null Node")
		}
		current.apply(del, docIDUpto)
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

func (s *DeleteSlice) isTail(n Node) bool {
	return s.sliceTail == n
}

func (s *DeleteSlice) isTailItem(item any) bool {
	return s.sliceTail.base().item == item
}

type termNode struct {
	NodeBase
	term Term
}

func (n *termNode) apply(bu *BufferedUpdates, docIDUpto int) {
	bu.AddTerm(n.term, docIDUpto)
}
func (n *termNode) isDelete() bool { return true }

type queryNode struct {
	NodeBase
	query Query
}

func (n *queryNode) apply(bu *BufferedUpdates, docIDUpto int) {
	bu.AddQuery(n.query, docIDUpto)
}
func (n *queryNode) isDelete() bool { return true }

type queryArrayNode struct {
	NodeBase
	queries []Query
}

func (n *queryArrayNode) apply(bu *BufferedUpdates, docIDUpto int) {
	for _, q := range n.queries {
		bu.AddQuery(q, docIDUpto)
	}
}
func (n *queryArrayNode) isDelete() bool { return true }

type termArrayNode struct {
	NodeBase
	terms []Term
}

func (n *termArrayNode) apply(bu *BufferedUpdates, docIDUpto int) {
	for _, t := range n.terms {
		bu.AddTerm(t, docIDUpto)
	}
}
func (n *termArrayNode) isDelete() bool { return true }

type docValuesUpdatesNode struct {
	NodeBase
	updates []DocValuesUpdate
}

func (n *docValuesUpdatesNode) apply(bu *BufferedUpdates, docIDUpto int) {
	for _, update := range n.updates {
		switch u := update.(type) {
		case *NumericDocValuesUpdate:
			bu.AddNumericUpdate(u, docIDUpto)
		case *BinaryDocValuesUpdate:
			bu.AddBinaryUpdate(u, docIDUpto)
		default:
			panic(fmt.Sprintf("%T DocValues updates not supported yet!", update))
		}
	}
}
func (n *docValuesUpdatesNode) isDelete() bool { return false }

func newNodeQueryArray(queries []Query) Node {
	return &queryArrayNode{
		NodeBase: NodeBase{item: queries},
		queries:  queries,
	}
}

func newNodeTermArray(terms []Term) Node {
	return &termArrayNode{
		NodeBase: NodeBase{item: terms},
		terms:    terms,
	}
}

func newNodeDocValuesUpdates(updates []DocValuesUpdate) Node {
	return &docValuesUpdatesNode{
		NodeBase: NodeBase{item: updates},
		updates:  updates,
	}
}
