// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/util"

// TermsHash is passed each token produced by the analyzer on each field
// during indexing. It stores those tokens in a hash table and allocates
// separate byte streams per token. Consumers of this type, such as the
// freq/prox writer and the term-vectors consumer, write their own byte
// streams under each term.
//
// This is the Go port of the package-private abstract class
// org.apache.lucene.index.TermsHash from Apache Lucene 10.4.0 (104 lines).
//
// Lucene models TermsHash as an abstract class with a single abstract
// method (addField) and a set of concrete methods shared by every
// subclass. Go has no abstract classes, so the port splits into:
//
//   - TermsHash, an interface that captures the full polymorphic surface
//     (the abstract addField plus every concrete method). A chain of
//     inversion handlers is wired through values of this interface.
//
//   - TermsHashBase, a struct that holds the concrete state (the two
//     pools, the shared term byte pool, the bytes counter and the
//     next-in-chain handler) and implements every concrete method.
//     Subclass ports embed *TermsHashBase and supply only AddField,
//     exactly as a Java subclass would override the single abstract
//     method.
//
// Divergences from Lucene 10.4.0:
//
//   - flush() takes a NormsProducer in Lucene. NormsProducer lives in the
//     Gocene codecs package, and index must not import codecs (the
//     dependency runs the other way). The norms argument is therefore
//     typed as the empty interface here and carried for parity only; it
//     is forwarded down the chain untouched. This matches the existing
//     treatment in FreqProxTermsWriter.Flush.
//
//   - Sorter.DocMap is the locally defined SorterDocMap interface, as
//     elsewhere in the index package.
type TermsHash interface {
	// Abort clears all accumulated state and propagates the call down
	// the chain.
	Abort()

	// Reset drops every pool buffer for this handler. It does not
	// propagate; callers that need the whole chain reset use Abort.
	Reset()

	// Flush hands the accumulated per-field handlers to the downstream
	// handler, mapping each entry to its next-in-chain per-field writer.
	// The base implementation only forwards to the next handler;
	// subclasses override to write their own segment data first.
	Flush(fieldsToFlush map[string]*TermsHashPerField, state *SegmentWriteState, sortMap SorterDocMap, norms any) error

	// AddField returns the per-field handler for the supplied inversion
	// state and field. This is the single abstract method in Lucene;
	// every concrete TermsHash supplies its own implementation.
	AddField(fieldInvertState *FieldInvertState, fieldInfo *FieldInfo) *TermsHashPerField

	// StartDocument signals the start of a document and propagates down
	// the chain.
	StartDocument() error

	// FinishDocument signals the end of the document with the given ID
	// and propagates down the chain.
	FinishDocument(docID int) error

	// NextTermsHash returns the next handler in the chain, or nil when
	// this handler is the tail. It exposes the package-private
	// nextTermsHash field so chain wiring (notably the term byte pool
	// hand-off) can be inspected and reused by embedding types.
	NextTermsHash() TermsHash
}

// TermsHashBase holds the concrete state shared by every TermsHash
// implementation and implements every concrete method of the TermsHash
// interface. It is the Go counterpart of the non-abstract members of
// Lucene's TermsHash class and is meant to be embedded by subclass ports.
//
// AddField is intentionally not implemented here: it is the abstract
// method, so an embedding type that does not supply its own AddField
// does not satisfy the TermsHash interface, which is the Go equivalent
// of failing to override an abstract method.
type TermsHashBase struct {
	// next is the next handler in the inversion chain. It is nil for the
	// tail handler. Mirrors Lucene's final TermsHash nextTermsHash.
	next TermsHash

	// IntPool and BytePool are owned by this handler. Mirrors Lucene's
	// final IntBlockPool intPool / final ByteBlockPool bytePool.
	IntPool  *util.IntBlockPool
	BytePool *util.ByteBlockPool

	// TermBytePool is the byte pool that stores the term bytes. For the
	// primary (head) handler in a chain it aliases BytePool and is also
	// published onto the next handler; for a non-primary handler it is
	// assigned by the primary. Mirrors Lucene's ByteBlockPool
	// termBytePool.
	TermBytePool *util.ByteBlockPool

	// BytesUsed tracks the memory consumed by this handler. Mirrors
	// Lucene's final Counter bytesUsed.
	BytesUsed *util.Counter
}

// NewTermsHashBase constructs the concrete state shared by a TermsHash
// implementation. It mirrors Lucene's TermsHash constructor:
//
//   - a fresh IntBlockPool and ByteBlockPool are created from the supplied
//     allocators;
//
//   - when nextTermsHash is non-nil this handler is the primary, so its
//     BytePool is published as the TermBytePool of both this handler and
//     the next one in the chain.
//
// Pass nil for intAlloc or byteAlloc to use the default direct allocators,
// matching Lucene's IntBlockPool.DirectAllocator / ByteBlockPool.DirectAllocator.
func NewTermsHashBase(
	intAlloc util.IntAllocator,
	byteAlloc util.Allocator,
	bytesUsed *util.Counter,
	nextTermsHash TermsHash,
) *TermsHashBase {
	if intAlloc == nil {
		intAlloc = util.NewDirectIntAllocator()
	}
	if byteAlloc == nil {
		byteAlloc = util.NewDirectAllocator()
	}

	t := &TermsHashBase{
		next:      nextTermsHash,
		BytesUsed: bytesUsed,
		IntPool:   util.NewIntBlockPoolWithAllocator(intAlloc),
		BytePool:  util.NewByteBlockPool(byteAlloc),
	}

	if nextTermsHash != nil {
		// We are primary: publish our byte pool as the term byte pool
		// for this handler and the next one in the chain.
		t.TermBytePool = t.BytePool
		if holder, ok := nextTermsHash.(termsHashBaseHolder); ok {
			holder.baseTermsHash().TermBytePool = t.BytePool
		}
	}
	return t
}

// termsHashBaseHolder is satisfied by any TermsHash whose implementation
// embeds *TermsHashBase (the method is promoted from the embedded base).
// The primary handler uses it to publish its byte pool onto the next
// handler without the next handler having to expose a public setter. An
// implementation that does not embed *TermsHashBase does not satisfy this
// interface and is treated as a non-primary handler that manages its own
// term byte pool.
type termsHashBaseHolder interface {
	baseTermsHash() *TermsHashBase
}

// baseTermsHash returns the embedded base of a TermsHash, satisfying
// termsHashBaseHolder.
func (t *TermsHashBase) baseTermsHash() *TermsHashBase { return t }

// Abort clears all state for this handler and propagates the call to the
// next handler in the chain. Mirrors Lucene's TermsHash.abort: Reset runs
// first and the downstream Abort runs even if Reset were to fail.
func (t *TermsHashBase) Abort() {
	t.Reset()
	if t.next != nil {
		t.next.Abort()
	}
}

// Reset drops every buffer held by the two pools. Buffers are not reused
// and not zero-filled, matching Lucene's TermsHash.reset.
func (t *TermsHashBase) Reset() {
	t.IntPool.Reset(false, false)
	t.BytePool.Reset(false, false)
}

// Flush forwards the accumulated per-field handlers to the next handler in
// the chain, remapping each entry to its next-in-chain per-field writer.
// Subclasses override this to write their own segment data before (or
// instead of) delegating; the base behaviour mirrors Lucene's
// TermsHash.flush, which only delegates.
//
// The norms argument is carried untouched for parity; see the package
// divergence note on the codecs import.
func (t *TermsHashBase) Flush(
	fieldsToFlush map[string]*TermsHashPerField,
	state *SegmentWriteState,
	sortMap SorterDocMap,
	norms any,
) error {
	if t.next == nil {
		return nil
	}
	nextChildFields := make(map[string]*TermsHashPerField, len(fieldsToFlush))
	for name, perField := range fieldsToFlush {
		nextChildFields[name] = perField.GetNextPerField()
	}
	return t.next.Flush(nextChildFields, state, sortMap, norms)
}

// StartDocument propagates the start-of-document signal to the next
// handler in the chain. Mirrors Lucene's TermsHash.startDocument.
func (t *TermsHashBase) StartDocument() error {
	if t.next != nil {
		return t.next.StartDocument()
	}
	return nil
}

// FinishDocument propagates the end-of-document signal to the next handler
// in the chain. Mirrors Lucene's TermsHash.finishDocument.
func (t *TermsHashBase) FinishDocument(docID int) error {
	if t.next != nil {
		return t.next.FinishDocument(docID)
	}
	return nil
}

// NextTermsHash returns the next handler in the chain, or nil when this
// handler is the tail. It exposes the package-private nextTermsHash field.
func (t *TermsHashBase) NextTermsHash() TermsHash { return t.next }
