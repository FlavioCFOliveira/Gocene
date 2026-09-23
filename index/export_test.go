// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "sync"

// This file is compiled only with the tests of package index. It exposes
// package-private members of org.apache.lucene.index to the external
// index_test package. Lucene's tests live in org.apache.lucene.index and read
// these members directly; the Gocene ports that need a class from a package
// importing index (for example search.TermQuery) must live in index_test to
// avoid an import cycle, and reach the same members through these accessors.

// NumGlobalTermDeletes exposes DocumentsWriterDeleteQueue.numGlobalTermDeletes().
func (d *DocumentsWriterDeleteQueue) NumGlobalTermDeletes() int { return d.numGlobalTermDeletes() }

// TryApplyGlobalSlice exposes DocumentsWriterDeleteQueue.tryApplyGlobalSlice().
func (d *DocumentsWriterDeleteQueue) TryApplyGlobalSlice() error { return d.tryApplyGlobalSlice() }

// GlobalBufferLock exposes the DocumentsWriterDeleteQueue.globalBufferLock field.
func (d *DocumentsWriterDeleteQueue) GlobalBufferLock() *sync.Mutex { return &d.globalBufferLock }

// Apply exposes DeleteSlice.apply(BufferedUpdates, int).
func (s *DeleteSlice) Apply(del *BufferedUpdates, docIDUpto int) { s.apply(del, docIDUpto) }

// IsTail exposes DeleteSlice.isTail(Node).
func (s *DeleteSlice) IsTail(n Node) bool { return s.isTail(n) }

// IsTailItem exposes DeleteSlice.isTailItem(Object).
func (s *DeleteSlice) IsTailItem(item any) bool { return s.isTailItem(item) }

// DeleteTerms exposes the BufferedUpdates.deleteTerms field.
func (b *BufferedUpdates) DeleteTerms() *DeletedTerms { return b.deleteTerms }

// DeletedTerms names BufferedUpdates.DeletedTerms for the external tests.
type DeletedTerms = deletedTerms

// NewDeletedTerms exposes the BufferedUpdates.DeletedTerms constructor.
func NewDeletedTerms() *DeletedTerms { return newDeletedTerms() }

// Get exposes DeletedTerms.get(Term).
func (dt *deletedTerms) Get(term Term) int { return dt.get(term) }

// Put exposes DeletedTerms.put(Term, int).
func (dt *deletedTerms) Put(term Term, value int) { dt.put(term, value) }

// Clear exposes DeletedTerms.clear().
func (dt *deletedTerms) Clear() { dt.clear() }

// Size exposes DeletedTerms.size().
func (dt *deletedTerms) Size() int { return dt.size() }

// IsEmpty exposes DeletedTerms.isEmpty().
func (dt *deletedTerms) IsEmpty() bool { return dt.isEmpty() }

// KeySet exposes DeletedTerms.keySet().
func (dt *deletedTerms) KeySet() []Term { return dt.keySet() }

// RamBytesUsed exposes DeletedTerms.ramBytesUsed().
func (dt *deletedTerms) RamBytesUsed() int64 { return dt.ramBytesUsed() }

// PoolBuffer exposes DeletedTerms.getPool().buffer.
func (dt *deletedTerms) PoolBuffer() []byte { return dt.getPool().Buffer }

// DeleteQueriesLength exposes FrozenBufferedUpdates.deleteQueries.length.
func (f *FrozenBufferedUpdates) DeleteQueriesLength() int { return len(f.deleteQueries) }

// IsSingletonSortedSetDocValues renders `v instanceof SingletonSortedSetDocValues`.
func IsSingletonSortedSetDocValues(v SortedSetDocValues) bool {
	_, ok := v.(*singletonSortedSet)
	return ok
}

// IsSingletonSortedNumericDocValues renders
// `v instanceof SingletonSortedNumericDocValues`.
func IsSingletonSortedNumericDocValues(v SortedNumericDocValues) bool {
	_, ok := v.(*singletonSortedNumeric)
	return ok
}

// IndexWriterReadFieldInfos exposes the package-private static
// IndexWriter.readFieldInfos(SegmentCommitInfo).
func IndexWriterReadFieldInfos(si *SegmentCommitInfo) (*FieldInfos, error) { return readFieldInfos(si) }
