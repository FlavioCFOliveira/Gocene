// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// DocumentsWriterDeleteQueue
// Source: lucene/core/src/java/org/apache/lucene/index/DocumentsWriterDeleteQueue.java
// Purpose: A non-blocking single-linked pending-deletes queue. Only the tail
// is maintained centrally; each DocumentsWriterPerThread and the global delete
// pool keep their own head as a DeleteSlice. Because the list is single-linked
// and a slice references only the nodes it still needs, the garbage collector
// prunes unreachable nodes automatically.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Node is a single entry in the DocumentsWriterDeleteQueue linked list.
//
// It is the Go port of Lucene's DocumentsWriterDeleteQueue.Node<T>. The
// sentinel node (created with nil item) is never applied; concrete node
// kinds embed baseNode and override Apply.
type Node interface {
	// Apply pushes this node's delete(s)/update(s) into the given
	// BufferedUpdates against the supplied docID upper bound.
	Apply(bufferedUpdates *BufferedUpdates, docIDUpto int)
	// IsDelete reports whether this node represents a delete (true) as
	// opposed to a doc-values update (false).
	IsDelete() bool
	// next returns the successor node, or nil at the tail.
	next() Node
	// setNext links the successor node.
	setNext(n Node)
}

// baseNode is the sentinel node and the shared base for every concrete node
// kind. The Java original lets a bare Node act as the sentinel; here baseNode
// fills that role and embedding provides the next-pointer plumbing.
type baseNode struct {
	// nextNode is the volatile successor pointer. It is published via an
	// atomic store in add and read via an atomic load so concurrent
	// slice readers observe a fully constructed successor.
	nextNode atomic.Pointer[nodeBox]
}

// nodeBox wraps a Node so it can be stored in an atomic.Pointer (Go's
// atomic.Pointer needs a single concrete pointee type).
type nodeBox struct {
	node Node
}

func (b *baseNode) next() Node {
	box := b.nextNode.Load()
	if box == nil {
		return nil
	}
	return box.node
}

func (b *baseNode) setNext(n Node) {
	if n == nil {
		b.nextNode.Store(nil)
		return
	}
	b.nextNode.Store(&nodeBox{node: n})
}

// Apply on the sentinel must never run: the head of a slice is always omitted.
func (b *baseNode) Apply(*BufferedUpdates, int) {
	panic("sentinel item must never be applied")
}

// IsDelete reports the sentinel/default node kind as a delete, matching the
// Java base Node.isDelete which returns true.
func (b *baseNode) IsDelete() bool { return true }

// termNode is a Node holding a single term delete.
type termNode struct {
	baseNode
	term *Term
}

// Apply records the term delete in bufferedUpdates.
func (n *termNode) Apply(bufferedUpdates *BufferedUpdates, docIDUpto int) {
	bufferedUpdates.AddTerm(n.term, docIDUpto)
}

// String mirrors Java's TermNode.toString ("del=" + item).
func (n *termNode) String() string { return fmt.Sprintf("del=%v", n.term) }

// queryNode is a Node holding a single query delete.
type queryNode struct {
	baseNode
	query Query
}

// Apply records the query delete in bufferedUpdates.
func (n *queryNode) Apply(bufferedUpdates *BufferedUpdates, docIDUpto int) {
	bufferedUpdates.AddQuery(n.query, docIDUpto)
}

// String mirrors Java's QueryNode.toString ("del=" + item).
func (n *queryNode) String() string { return fmt.Sprintf("del=%v", n.query) }

// queryArrayNode is a Node holding a batch of query deletes.
type queryArrayNode struct {
	baseNode
	queries []Query
}

// Apply records every query delete in bufferedUpdates.
func (n *queryArrayNode) Apply(bufferedUpdates *BufferedUpdates, docIDUpto int) {
	for _, q := range n.queries {
		bufferedUpdates.AddQuery(q, docIDUpto)
	}
}

// termArrayNode is a Node holding a batch of term deletes.
type termArrayNode struct {
	baseNode
	terms []*Term
}

// Apply records every term delete in bufferedUpdates.
func (n *termArrayNode) Apply(bufferedUpdates *BufferedUpdates, docIDUpto int) {
	for _, t := range n.terms {
		bufferedUpdates.AddTerm(t, docIDUpto)
	}
}

// String mirrors Java's TermArrayNode.toString ("dels=" + Arrays.toString).
func (n *termArrayNode) String() string { return fmt.Sprintf("dels=%v", n.terms) }

// docValuesUpdatesNode is a Node holding a batch of doc-values updates.
type docValuesUpdatesNode struct {
	baseNode
	updates []*DocValuesUpdate
}

// Apply dispatches each update by its doc-values type. SORTED, SORTED_SET,
// SORTED_NUMERIC and NONE are rejected via panic, matching Java's
// IllegalArgumentException for unsupported update types.
func (n *docValuesUpdatesNode) Apply(bufferedUpdates *BufferedUpdates, docIDUpto int) {
	for _, update := range n.updates {
		switch update.Type {
		case DocValuesTypeNumeric:
			bufferedUpdates.AddNumericUpdate(numericFromUpdate(update), docIDUpto)
		case DocValuesTypeBinary:
			bufferedUpdates.AddBinaryUpdate(binaryFromUpdate(update), docIDUpto)
		default:
			panic(fmt.Sprintf("%v DocValues updates not supported yet!", update.Type))
		}
	}
}

// IsDelete reports false: a doc-values update is not a delete.
func (n *docValuesUpdatesNode) IsDelete() bool { return false }

// String mirrors Java's DocValuesUpdatesNode.toString.
func (n *docValuesUpdatesNode) String() string {
	if len(n.updates) == 0 {
		return "docValuesUpdates: "
	}
	s := fmt.Sprintf("docValuesUpdates: term=%v; updates: [", n.updates[0].Term)
	for i, update := range n.updates {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%s:%v", update.Field, update.Value)
	}
	return s + "]"
}

// numericFromUpdate reconstructs a NumericDocValuesUpdate from a generic
// DocValuesUpdate carried by the node. The embedded copy preserves the
// field/term identity; the numeric value is taken from Value.
func numericFromUpdate(update *DocValuesUpdate) *NumericDocValuesUpdate {
	var value int64
	if v, ok := update.Value.(int64); ok {
		value = v
	}
	return &NumericDocValuesUpdate{DocValuesUpdate: *update, NumericValue: value}
}

// binaryFromUpdate reconstructs a BinaryDocValuesUpdate from a generic
// DocValuesUpdate carried by the node.
func binaryFromUpdate(update *DocValuesUpdate) *BinaryDocValuesUpdate {
	var value []byte
	if v, ok := update.Value.([]byte); ok {
		value = v
	}
	return &BinaryDocValuesUpdate{DocValuesUpdate: *update, BinaryValue: value}
}

// NewTermNode creates a Node for a single term delete. It is the Go port of
// the static factory DocumentsWriterDeleteQueue.newNode(Term).
func NewTermNode(term *Term) Node {
	return &termNode{term: term}
}

// NewQueryNode creates a Node for a single query delete. It is the Go port of
// the static factory DocumentsWriterDeleteQueue.newNode(Query).
func NewQueryNode(query Query) Node {
	return &queryNode{query: query}
}

// NewDocValuesUpdatesNode creates a Node for a batch of doc-values updates.
// It is the Go port of the static factory
// DocumentsWriterDeleteQueue.newNode(DocValuesUpdate...).
func NewDocValuesUpdatesNode(updates ...*DocValuesUpdate) Node {
	return &docValuesUpdatesNode{updates: updates}
}

// DeleteSlice is a thread-captive view over a contiguous run of the delete
// queue. It is the Go port of DocumentsWriterDeleteQueue.DeleteSlice.
//
// A slice keeps its own head (never applied) and tail. Fields need no
// synchronization because a slice is only ever touched by one goroutine.
type DeleteSlice struct {
	sliceHead Node // omitted on apply
	sliceTail Node
}

// newDeleteSlice creates a zero-length slice anchored at the current tail.
func newDeleteSlice(currentTail Node) *DeleteSlice {
	if currentTail == nil {
		panic("currentTail must not be nil")
	}
	return &DeleteSlice{sliceHead: currentTail, sliceTail: currentTail}
}

// Apply pushes every node strictly after the slice head, up to and including
// the slice tail, into del with the given docID upper bound, then resets the
// slice to zero length. A zero-length slice is a no-op.
func (s *DeleteSlice) Apply(del *BufferedUpdates, docIDUpto int) {
	if s.sliceHead == s.sliceTail {
		// 0 length slice
		return
	}
	// Take the head, advance to its successor as the first item to apply,
	// and continue until the tail is applied. With unequal head and tail
	// there is at least one more non-nil node in the slice.
	current := s.sliceHead
	for {
		current = current.next()
		if current == nil {
			panic("slice property violated between the head and the tail must not be a null node")
		}
		current.Apply(del, docIDUpto)
		if current == s.sliceTail {
			break
		}
	}
	s.Reset()
}

// Reset collapses the slice back to zero length at its current tail.
func (s *DeleteSlice) Reset() {
	s.sliceHead = s.sliceTail
}

// IsTail reports whether node is identical to the slice tail.
func (s *DeleteSlice) IsTail(node Node) bool {
	return s.sliceTail == node
}

// IsTailItem reports whether item is identical to the item held by the slice
// tail. It mirrors Java's DeleteSlice.isTailItem(Object), which compares by
// reference identity. Pointer-typed node items (Term, Query) use ==; the
// batch node items are Go slices, for which identity means the same backing
// array and length, so they are compared via sameSliceIdentity.
func (s *DeleteSlice) IsTailItem(item any) bool {
	switch n := s.sliceTail.(type) {
	case *termNode:
		return any(n.term) == item
	case *queryNode:
		return any(n.query) == item
	case *termArrayNode:
		other, ok := item.([]*Term)
		return ok && sameSliceIdentity(n.terms, other)
	case *queryArrayNode:
		other, ok := item.([]Query)
		return ok && sameSliceIdentity(n.queries, other)
	case *docValuesUpdatesNode:
		other, ok := item.([]*DocValuesUpdate)
		return ok && sameSliceIdentity(n.updates, other)
	default:
		return item == nil
	}
}

// sameSliceIdentity reports whether a and b refer to the same slice, i.e. the
// same backing array and the same length. It is the slice analogue of Java's
// reference == used by DeleteSlice.isTailItem.
func sameSliceIdentity[T any](a, b []T) bool {
	if len(a) != len(b) || (a == nil) != (b == nil) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	return &a[0] == &b[0]
}

// IsEmpty reports whether the slice currently spans no nodes.
func (s *DeleteSlice) IsEmpty() bool {
	return s.sliceHead == s.sliceTail
}

// DocumentsWriterDeleteQueue is a non-blocking linked pending-deletes queue.
//
// It is the Go port of the package-private final class
// org.apache.lucene.index.DocumentsWriterDeleteQueue. The queue implements the
// Accountable contract via RamBytesUsed and a Closeable-style Close.
type DocumentsWriterDeleteQueue struct {
	// mu serializes the operations the Java original marks synchronized
	// (add, updateSlice, close, advanceQueue, isAdvanced).
	mu sync.Mutex

	// tail is the latest delete operation in the queue. It is published
	// atomically so lock-free readers (anyChanges, freeze, slices) see a
	// consistent value.
	tail atomic.Pointer[nodeBox]

	// closed is the volatile closed flag.
	closed atomic.Bool

	// globalSlice records deletes against all prior (already flushed)
	// segments. On any segment flush this set is bundled into a frozen
	// updates packet inserted ahead of the newly flushed segment(s).
	globalSlice *DeleteSlice

	globalBufferedUpdates *BufferedUpdates

	// GlobalBufferLock is acquired only to update the global deletes. It is
	// exported for parity with the package-private final field accessed by
	// the Java test peer.
	GlobalBufferLock sync.Mutex

	// Generation carries over across advanceQueue and identifies the queue
	// generation. It is exported to match the package-private final field.
	Generation int64

	// nextSeqNo generates the sequence numbers IndexWriter returns to
	// callers, expressing the effective serialization of all operations.
	nextSeqNo atomic.Int64

	infoStream util.InfoStream

	// maxSeqNo is the volatile upper bound on completed sequence numbers.
	maxSeqNo atomic.Int64

	startSeqNo       int64
	previousMaxSeqID func() int64
	advanced         bool
}

// NewDocumentsWriterDeleteQueue creates a fresh queue. The infoStream is
// optional: passing nil installs util.NoOpInfoStream. seqNo starts at 1
// because some APIs negate it to also return a boolean.
//
// It is the Go port of the public constructor
// DocumentsWriterDeleteQueue(InfoStream).
func NewDocumentsWriterDeleteQueue(infoStream util.InfoStream) *DocumentsWriterDeleteQueue {
	return newDocumentsWriterDeleteQueue(infoStream, 0, 1, func() int64 { return 0 })
}

// newDocumentsWriterDeleteQueue is the Go port of the private all-args
// constructor. It seeds the queue with a sentinel tail and anchors the global
// slice on it.
func newDocumentsWriterDeleteQueue(
	infoStream util.InfoStream,
	generation int64,
	startSeqNo int64,
	previousMaxSeqID func() int64,
) *DocumentsWriterDeleteQueue {
	if infoStream == nil {
		infoStream = util.NoOpInfoStream
	}
	if previousMaxSeqID == nil {
		previousMaxSeqID = func() int64 { return 0 }
	}
	value := previousMaxSeqID()
	if value > startSeqNo {
		panic(fmt.Sprintf("illegal max sequence ID: %d start was: %d", value, startSeqNo))
	}
	q := &DocumentsWriterDeleteQueue{
		infoStream:            infoStream,
		globalBufferedUpdates: NewBufferedUpdates("global"),
		Generation:            generation,
		startSeqNo:            startSeqNo,
		previousMaxSeqID:      previousMaxSeqID,
	}
	q.nextSeqNo.Store(startSeqNo)
	q.maxSeqNo.Store(int64(^uint64(0) >> 1)) // Long.MAX_VALUE
	// Sentinel tail: no slice ever applies it because the head is omitted.
	sentinel := &baseNode{}
	q.tail.Store(&nodeBox{node: sentinel})
	q.globalSlice = newDeleteSlice(sentinel)
	return q
}

// loadTail returns the current tail node.
func (q *DocumentsWriterDeleteQueue) loadTail() Node {
	return q.tail.Load().node
}

// AddDeleteQueries adds a batch of query deletes and returns the assigned
// sequence number. It is the Go port of addDelete(Query...).
func (q *DocumentsWriterDeleteQueue) AddDeleteQueries(queries ...Query) (int64, error) {
	seqNo, err := q.addNode(&queryArrayNode{queries: queries})
	if err != nil {
		return 0, err
	}
	q.TryApplyGlobalSlice()
	return seqNo, nil
}

// AddDeleteTerms adds a batch of term deletes and returns the assigned
// sequence number. It is the Go port of addDelete(Term...).
func (q *DocumentsWriterDeleteQueue) AddDeleteTerms(terms ...*Term) (int64, error) {
	seqNo, err := q.addNode(&termArrayNode{terms: terms})
	if err != nil {
		return 0, err
	}
	q.TryApplyGlobalSlice()
	return seqNo, nil
}

// AddDocValuesUpdates adds a batch of doc-values updates and returns the
// assigned sequence number. It is the Go port of
// addDocValuesUpdates(DocValuesUpdate...).
func (q *DocumentsWriterDeleteQueue) AddDocValuesUpdates(updates ...*DocValuesUpdate) (int64, error) {
	seqNo, err := q.addNode(&docValuesUpdatesNode{updates: updates})
	if err != nil {
		return 0, err
	}
	q.TryApplyGlobalSlice()
	return seqNo, nil
}

// AddWithSlice appends deleteNode and atomically pins it as the tail of slice.
//
// It is the Go port of add(Node, DeleteSlice), the document-update invariant:
// it guarantees the insert is atomic with regard to slice so that when two
// goroutines update the same document with the same delTerm, one wins and the
// other applies the delete on its next slice update.
func (q *DocumentsWriterDeleteQueue) AddWithSlice(deleteNode Node, slice *DeleteSlice) (int64, error) {
	seqNo, err := q.addNode(deleteNode)
	if err != nil {
		return 0, err
	}
	slice.sliceTail = deleteNode
	if slice.sliceHead == slice.sliceTail {
		panic("slice head and tail must differ after add")
	}
	q.TryApplyGlobalSlice() // TODO doing this each time is not necessary; could batch every n.
	return seqNo, nil
}

// Add appends newNode to the tail of the queue and returns the assigned
// sequence number. It is the Go port of the synchronized add(Node).
func (q *DocumentsWriterDeleteQueue) Add(newNode Node) (int64, error) {
	return q.addNode(newNode)
}

// addNode is the synchronized core shared by Add and AddWithSlice.
func (q *DocumentsWriterDeleteQueue) addNode(newNode Node) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureOpen(); err != nil {
		return 0, err
	}
	if newNode == nil {
		// Java dereferences a null node here and throws NPE under
		// ensureOpen; the queue test only adds nil to a closed queue,
		// which ensureOpen already rejects above.
		return 0, NewAlreadyClosedException("cannot add a nil node", nil)
	}
	q.loadTail().setNext(newNode)
	q.tail.Store(&nodeBox{node: newNode})
	return q.getNextSequenceNumber(), nil
}

// AnyChanges reports whether the queue holds deletes/updates not yet applied
// to the global buffered updates. It is the Go port of anyChanges().
func (q *DocumentsWriterDeleteQueue) AnyChanges() bool {
	q.GlobalBufferLock.Lock()
	defer q.GlobalBufferLock.Unlock()
	// True if the global buffer has changes, or the global slice is
	// non-empty, or the global slice tail trails the queue tail, or a
	// successor was linked after the tail.
	return q.globalBufferedUpdates.Any() ||
		!q.globalSlice.IsEmpty() ||
		q.globalSlice.sliceTail != q.loadTail() ||
		q.loadTail().next() != nil
}

// TryApplyGlobalSlice applies the global slice into the global buffered
// updates if the global buffer lock can be acquired without blocking.
//
// It is the Go port of tryApplyGlobalSlice(). Failing to acquire the lock is
// fine: deletes added after the in-flight slice tail are applied next time.
func (q *DocumentsWriterDeleteQueue) TryApplyGlobalSlice() {
	if !q.GlobalBufferLock.TryLock() {
		return
	}
	defer q.GlobalBufferLock.Unlock()
	if q.closed.Load() {
		panic(q.alreadyClosedMessage())
	}
	if q.updateSliceNoSeqNo(q.globalSlice) {
		q.globalSlice.Apply(q.globalBufferedUpdates, MaxInt)
	}
}

// FreezeGlobalBuffer freezes the global buffered updates and resets the global
// slice so the garbage collector can prune the queue. callerSlice, if
// non-nil, is advanced to the captured tail so the caller stays in sync.
//
// It is the Go port of freezeGlobalBuffer(DeleteSlice). Returns nil when there
// is nothing to freeze.
func (q *DocumentsWriterDeleteQueue) FreezeGlobalBuffer(callerSlice *DeleteSlice) (*FrozenBufferedUpdates, error) {
	q.GlobalBufferLock.Lock()
	defer q.GlobalBufferLock.Unlock()
	if err := q.ensureOpen(); err != nil {
		return nil, err
	}
	// Capture the current tail locally; changes after this point are
	// applied later and are not relevant here.
	currentTail := q.loadTail()
	if callerSlice != nil {
		// Bring the caller's slice onto the same page.
		callerSlice.sliceTail = currentTail
	}
	return q.freezeGlobalBufferInternal(currentTail)
}

// MaybeFreezeGlobalBuffer freezes the global buffer unless the queue is
// already closed, in which case it returns nil.
//
// It is the Go port of maybeFreezeGlobalBuffer().
func (q *DocumentsWriterDeleteQueue) MaybeFreezeGlobalBuffer() (*FrozenBufferedUpdates, error) {
	q.GlobalBufferLock.Lock()
	defer q.GlobalBufferLock.Unlock()
	if !q.closed.Load() {
		return q.freezeGlobalBufferInternal(q.loadTail())
	}
	if q.anyChangesLocked() {
		panic("we are closed but have changes")
	}
	return nil, nil
}

// freezeGlobalBufferInternal applies any outstanding global-slice deletes,
// then snapshots the global buffered updates into a FrozenBufferedUpdates and
// clears the buffer. The caller must hold GlobalBufferLock.
func (q *DocumentsWriterDeleteQueue) freezeGlobalBufferInternal(currentTail Node) (*FrozenBufferedUpdates, error) {
	if q.globalSlice.sliceTail != currentTail {
		q.globalSlice.sliceTail = currentTail
		q.globalSlice.Apply(q.globalBufferedUpdates, MaxInt)
	}
	if !q.globalBufferedUpdates.Any() {
		return nil, nil
	}
	packet, err := NewFrozenBufferedUpdates(q.infoStream, q.globalBufferedUpdates, nil)
	if err != nil {
		return nil, err
	}
	q.globalBufferedUpdates.Clear()
	return packet, nil
}

// NewSlice creates a fresh zero-length DeleteSlice anchored on the current
// tail. It is the Go port of newSlice().
func (q *DocumentsWriterDeleteQueue) NewSlice() *DeleteSlice {
	return newDeleteSlice(q.loadTail())
}

// UpdateSlice advances slice to the current tail and returns a sequence
// number. A negative result signals that new deletes arrived since the slice
// was last checked. It is the Go port of the synchronized updateSlice.
func (q *DocumentsWriterDeleteQueue) UpdateSlice(slice *DeleteSlice) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if err := q.ensureOpen(); err != nil {
		return 0, err
	}
	seqNo := q.getNextSequenceNumber()
	if slice.sliceTail != q.loadTail() {
		// New deletes arrived since the slice was last checked.
		slice.sliceTail = q.loadTail()
		seqNo = -seqNo
	}
	return seqNo, nil
}

// updateSliceNoSeqNo advances slice to the current tail without assigning a
// sequence number, reporting whether the tail moved. It is the Go port of
// updateSliceNoSeqNo.
func (q *DocumentsWriterDeleteQueue) updateSliceNoSeqNo(slice *DeleteSlice) bool {
	if slice.sliceTail != q.loadTail() {
		slice.sliceTail = q.loadTail()
		return true
	}
	return false
}

// ensureOpen returns an AlreadyClosedException-typed error when the queue is
// closed. It is the Go port of ensureOpen(): callers surface the error
// instead of throwing.
func (q *DocumentsWriterDeleteQueue) ensureOpen() error {
	if q.closed.Load() {
		return NewAlreadyClosedException(q.alreadyClosedMessage(), nil)
	}
	return nil
}

// alreadyClosedMessage returns the closed-queue diagnostic string used by both
// the returned error and the panic path of TryApplyGlobalSlice.
func (q *DocumentsWriterDeleteQueue) alreadyClosedMessage() string {
	return "This DocumentsWriterDeleteQueue is already closed"
}

// IsOpen reports whether the queue is still open. It is the Go port of
// isOpen().
func (q *DocumentsWriterDeleteQueue) IsOpen() bool {
	return !q.closed.Load()
}

// Close closes the queue. It is the Go port of the synchronized
// Closeable.close(). It returns an error (in place of Java's
// IllegalStateException) when unapplied changes remain.
func (q *DocumentsWriterDeleteQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.GlobalBufferLock.Lock()
	defer q.GlobalBufferLock.Unlock()
	if q.anyChangesLocked() {
		return fmt.Errorf("can't close queue unless all changes are applied")
	}
	q.closed.Store(true)
	seqNo := q.nextSeqNo.Load()
	maxSeqNo := q.maxSeqNo.Load()
	if seqNo > maxSeqNo {
		panic(fmt.Sprintf("maxSeqNo must be greater or equal to %d but was %d", seqNo, maxSeqNo))
	}
	q.nextSeqNo.Store(maxSeqNo + 1)
	return nil
}

// anyChangesLocked is the body of AnyChanges for callers that already hold
// GlobalBufferLock.
func (q *DocumentsWriterDeleteQueue) anyChangesLocked() bool {
	return q.globalBufferedUpdates.Any() ||
		!q.globalSlice.IsEmpty() ||
		q.globalSlice.sliceTail != q.loadTail() ||
		q.loadTail().next() != nil
}

// NumGlobalTermDeletes returns the count of buffered global term deletes. It
// is the Go port of the test-only numGlobalTermDeletes().
func (q *DocumentsWriterDeleteQueue) NumGlobalTermDeletes() int {
	return q.globalBufferedUpdates.NumDeleteTerms()
}

// Clear resets the global slice and discards the global buffered updates. It
// is the Go port of clear().
func (q *DocumentsWriterDeleteQueue) Clear() {
	q.GlobalBufferLock.Lock()
	defer q.GlobalBufferLock.Unlock()
	currentTail := q.loadTail()
	q.globalSlice.sliceHead = currentTail
	q.globalSlice.sliceTail = currentTail
	q.globalBufferedUpdates.Clear()
}

// GetBufferedUpdatesTermsSize applies any pending global-slice deletes and
// returns the number of buffered global term deletes. It is the Go port of
// getBufferedUpdatesTermsSize().
func (q *DocumentsWriterDeleteQueue) GetBufferedUpdatesTermsSize() int {
	q.GlobalBufferLock.Lock()
	defer q.GlobalBufferLock.Unlock()
	currentTail := q.loadTail()
	if q.globalSlice.sliceTail != currentTail {
		q.globalSlice.sliceTail = currentTail
		q.globalSlice.Apply(q.globalBufferedUpdates, MaxInt)
	}
	return q.globalBufferedUpdates.NumDeleteTerms()
}

// RamBytesUsed returns the estimated RAM held by the global buffered updates.
// It is the Go port of the Accountable method ramBytesUsed().
func (q *DocumentsWriterDeleteQueue) RamBytesUsed() int64 {
	return q.globalBufferedUpdates.RamBytesUsed()
}

// String mirrors Java's toString.
func (q *DocumentsWriterDeleteQueue) String() string {
	return fmt.Sprintf("DWDQ: [ generation: %d ]", q.Generation)
}

// GetNextSequenceNumber atomically returns the next sequence number. It is the
// Go port of the public getNextSequenceNumber().
func (q *DocumentsWriterDeleteQueue) GetNextSequenceNumber() int64 {
	return q.getNextSequenceNumber()
}

// getNextSequenceNumber is the unexported core shared by the public accessor
// and the synchronized add/updateSlice paths.
func (q *DocumentsWriterDeleteQueue) getNextSequenceNumber() int64 {
	seqNo := q.nextSeqNo.Add(1) - 1
	if seqNo > q.maxSeqNo.Load() {
		panic(fmt.Sprintf("seqNo=%d vs maxSeqNo=%d", seqNo, q.maxSeqNo.Load()))
	}
	return seqNo
}

// GetLastSequenceNumber returns the most recently assigned sequence number. It
// is the Go port of getLastSequenceNumber().
func (q *DocumentsWriterDeleteQueue) GetLastSequenceNumber() int64 {
	return q.nextSeqNo.Load() - 1
}

// SkipSequenceNumbers inserts a gap of jump sequence numbers. IndexWriter uses
// this on flush/commit so in-flight goroutines land inside the gap. It is the
// Go port of skipSequenceNumbers(long).
func (q *DocumentsWriterDeleteQueue) SkipSequenceNumbers(jump int64) {
	q.nextSeqNo.Add(jump)
}

// GetMaxCompletedSeqNo returns the maximum completed sequence number for this
// queue, falling back to the previous queue when this one never advanced its
// seqNo. It is the Go port of getMaxCompletedSeqNo().
func (q *DocumentsWriterDeleteQueue) GetMaxCompletedSeqNo() int64 {
	if q.startSeqNo < q.nextSeqNo.Load() {
		return q.GetLastSequenceNumber()
	}
	// Fall back to the previous queue if the seqNo was never advanced.
	value := q.previousMaxSeqID()
	if value >= q.startSeqNo {
		panic(fmt.Sprintf("illegal max sequence ID: %d start was: %d", value, q.startSeqNo))
	}
	return value
}

// getPrevMaxSeqIDSupplier returns a supplier closing only over nextSeqNo, not
// over the queue itself. It is the Go port of the static
// getPrevMaxSeqIdSupplier; the static form avoids the LUCENE-9478 memory leak
// where the lambda implicitly retained the whole delete queue.
func getPrevMaxSeqIDSupplier(nextSeqNo *atomic.Int64) func() int64 {
	return func() int64 { return nextSeqNo.Load() - 1 }
}

// AdvanceQueue advances to a successor queue on flush, carrying over the
// generation and fixing this queue's max sequence number. maxNumPendingOps is
// the number of DWPTs that still own this queue and may each bump the seqId
// after the advance. It is the Go port of the synchronized advanceQueue(int);
// it may only be called once.
func (q *DocumentsWriterDeleteQueue) AdvanceQueue(maxNumPendingOps int) (*DocumentsWriterDeleteQueue, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.advanced {
		return nil, fmt.Errorf("queue was already advanced")
	}
	q.advanced = true
	seqNo := q.GetLastSequenceNumber() + int64(maxNumPendingOps) + 1
	q.maxSeqNo.Store(seqNo)
	return newDocumentsWriterDeleteQueue(
		q.infoStream,
		q.Generation+1,
		seqNo+1,
		// Do not pass GetMaxCompletedSeqNo: that would retain a reference
		// to this queue and leak it (queues could never be GCed).
		getPrevMaxSeqIDSupplier(&q.nextSeqNo),
	), nil
}

// GetMaxSeqNo returns the maximum sequence number for this queue. The value
// changes once the queue is advanced. It is the Go port of getMaxSeqNo().
func (q *DocumentsWriterDeleteQueue) GetMaxSeqNo() int64 {
	return q.maxSeqNo.Load()
}

// IsAdvanced reports whether the queue was advanced. It is the Go port of the
// synchronized isAdvanced().
func (q *DocumentsWriterDeleteQueue) IsAdvanced() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.advanced
}
