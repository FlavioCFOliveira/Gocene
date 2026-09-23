// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PendingDeletesInterface is the overridable surface of the Java class
// org.apache.lucene.index.PendingDeletes (Apache Lucene 10.5.0). Java
// PendingSoftDeletes extends PendingDeletes and overrides part of it; Go
// renders the class hierarchy as this interface, implemented by the base
// struct [PendingDeletes] and by [PendingSoftDeletes], which embeds it. Every
// holder of a Java PendingDeletes reference (ReadersAndUpdates, ReaderPool)
// holds this interface, so calls dispatch to the overrides exactly as Java's
// virtual calls do.
type PendingDeletesInterface interface {
	// Delete marks a document as deleted in this segment and returns true if
	// a document got actually deleted or if the document was already
	// deleted. Mirrors delete(int).
	Delete(docID int) (bool, error)
	// GetLiveDocs returns a snapshot of the current live docs. Mirrors
	// getLiveDocs().
	GetLiveDocs() util.Bits
	// GetHardLiveDocs returns a snapshot of the hard live docs. Mirrors
	// getHardLiveDocs().
	GetHardLiveDocs() util.Bits
	// NumPendingDeletes returns the number of pending deletes that are not
	// written to disk. Mirrors the protected numPendingDeletes().
	NumPendingDeletes() int
	// OnNewReader is called once a new reader is opened for this segment,
	// for instance when this segment is flushed or merged. Mirrors
	// onNewReader(CodecReader, SegmentCommitInfo).
	OnNewReader(reader CodecReader, info *SegmentCommitInfo) error
	// DropChanges resets the pending docs. Mirrors dropChanges().
	DropChanges()
	// WriteLiveDocs writes the live docs to disk and returns true iff any new
	// docs were deleted since the last writeLiveDocs. Mirrors
	// writeLiveDocs(Directory).
	WriteLiveDocs(dir store.Directory) (bool, error)
	// IsFullyDeleted returns true iff the segment represented by this
	// instance contains only deleted docs. Mirrors
	// isFullyDeleted(IOSupplier<CodecReader>).
	IsFullyDeleted(readerIOSupplier func() (CodecReader, error)) (bool, error)
	// OnDocValuesUpdate is called for every field update for the given
	// field at flush time. Mirrors onDocValuesUpdate(FieldInfo,
	// DocValuesFieldUpdates.Iterator).
	OnDocValuesUpdate(info *FieldInfo, iterator DocValuesFieldUpdatesIterator) error
	// NumDeletesToMerge mirrors numDeletesToMerge(MergePolicy,
	// IOSupplier<CodecReader>).
	NumDeletesToMerge(policy MergePolicy, readerIOSupplier func() (CodecReader, error)) (int, error)
	// NeedsRefresh returns true if the given reader needs to be refreshed in
	// order to see the latest deletes. Mirrors the final needsRefresh(CodecReader).
	NeedsRefresh(reader CodecReader) bool
	// GetDelCount returns the number of deleted docs in the segment. Mirrors
	// the final getDelCount().
	GetDelCount() int
	// NumDocs returns the number of live documents in this segment. Mirrors
	// the final numDocs().
	NumDocs() int
	// VerifyDocCounts mirrors verifyDocCounts(CodecReader), called from
	// assertions only.
	VerifyDocCounts(reader CodecReader) bool
	// MustInitOnDelete returns true if we have to initialize this
	// PendingDeletes before delete(int); otherwise this PendingDeletes is
	// ready to accept deletes. A PendingDeletes can be initialized by
	// providing it a reader via OnNewReader. Mirrors mustInitOnDelete().
	MustInitOnDelete() bool
	// String mirrors toString().
	String() string

	// pendingDeletesBase returns the embedded PendingDeletes state.
	pendingDeletesBase() *PendingDeletes
}

// PendingDeletes handles accounting and applying pending deletes for live
// segment readers. Mirrors org.apache.lucene.index.PendingDeletes from Apache
// Lucene 10.5.0; see [PendingDeletesInterface] for the rendering of its
// subclass PendingSoftDeletes.
type PendingDeletes struct {
	info *SegmentCommitInfo
	// Read-only live docs, null until live docs are initialized or if all
	// docs are alive.
	liveDocs util.Bits
	// Writeable live docs, null if this instance is not ready to accept
	// writes, in which case getMutableBits needs to be called.
	writeableLiveDocs  *util.FixedBitSet
	pendingDeleteCount int
	// liveDocsInitialized is package-private in Java.
	liveDocsInitialized bool

	// this is the dynamic type of the Java object: the PendingDeletes itself
	// or the PendingSoftDeletes that embeds it. The final methods of the base
	// class (getDelCount, numDocs, needsRefresh, verifyDocCounts,
	// isFullyDeleted) call the overridable ones through it, as Java's
	// virtual dispatch does.
	this PendingDeletesInterface
}

// NewPendingDeletesFromReader mirrors PendingDeletes(SegmentReader,
// SegmentCommitInfo).
func NewPendingDeletesFromReader(reader *SegmentReader, info *SegmentCommitInfo) *PendingDeletes {
	p := NewPendingDeletes(info, reader.GetLiveDocs(), true)
	p.pendingDeleteCount = reader.NumDeletedDocs() - info.DelCount()
	return p
}

// NewPendingDeletesFromInfo mirrors PendingDeletes(SegmentCommitInfo): a
// segment without deletions needs no reader to initialize its live docs,
// since deletes may be received before a reader was ever opened.
func NewPendingDeletesFromInfo(info *SegmentCommitInfo) *PendingDeletes {
	return NewPendingDeletes(info, nil, !info.HasDeletions())
}

// NewPendingDeletes mirrors PendingDeletes(SegmentCommitInfo, Bits, boolean).
func NewPendingDeletes(info *SegmentCommitInfo, liveDocs util.Bits, liveDocsInitialized bool) *PendingDeletes {
	p := &PendingDeletes{
		info:                info,
		liveDocs:            liveDocs,
		pendingDeleteCount:  0,
		liveDocsInitialized: liveDocsInitialized,
	}
	p.this = p
	return p
}

func (p *PendingDeletes) pendingDeletesBase() *PendingDeletes { return p }

// getMutableBits mirrors the protected getMutableBits().
func (p *PendingDeletes) getMutableBits() *util.FixedBitSet {
	// if we pull mutable bits but we haven't been initialized something is
	// completely off. this means we receive deletes without having the
	// bitset that is on-disk ready to be cloned
	if util.AssertsEnabled() && !p.liveDocsInitialized {
		panic(util.NewAssertionError("can't delete if liveDocs are not initialized"))
	}
	if p.writeableLiveDocs == nil {
		// Copy on write: this means we've cloned a SegmentReader sharing the
		// current liveDocs instance; must now make a private clone so we can
		// change it:
		if p.liveDocs != nil {
			p.writeableLiveDocs = util.FixedBitSetCopy(p.liveDocs)
		} else {
			maxDoc := p.info.SegmentInfo().MaxDoc()
			p.writeableLiveDocs, _ = util.NewFixedBitSet(maxDoc)
			p.writeableLiveDocs.SetRange(0, maxDoc)
		}
		p.liveDocs = p.writeableLiveDocs.AsReadOnlyBits()
	}
	return p.writeableLiveDocs
}

// Delete mirrors delete(int).
func (p *PendingDeletes) Delete(docID int) (bool, error) {
	if util.AssertsEnabled() && !(p.info.SegmentInfo().MaxDoc() > 0) {
		panic(util.NewAssertionError(nil))
	}
	mutableBits := p.getMutableBits()
	if util.AssertsEnabled() && !(docID >= 0 && docID < mutableBits.Length()) {
		panic(util.NewAssertionError(fmt.Sprintf("out of bounds: docid=%d liveDocsLength=%d seg=%s maxDoc=%d",
			docID, mutableBits.Length(), p.info.SegmentInfo().Name(), p.info.SegmentInfo().MaxDoc())))
	}
	didDelete := mutableBits.GetAndClear(docID)
	if didDelete {
		p.pendingDeleteCount++
	}
	return didDelete, nil
}

// GetLiveDocs mirrors getLiveDocs(). Prevent modifications to the returned
// live docs.
func (p *PendingDeletes) GetLiveDocs() util.Bits {
	p.writeableLiveDocs = nil
	return p.liveDocs
}

// GetHardLiveDocs mirrors getHardLiveDocs().
func (p *PendingDeletes) GetHardLiveDocs() util.Bits {
	return p.this.GetLiveDocs()
}

// NumPendingDeletes mirrors the protected numPendingDeletes().
func (p *PendingDeletes) NumPendingDeletes() int {
	return p.pendingDeleteCount
}

// OnNewReader mirrors onNewReader(CodecReader, SegmentCommitInfo).
func (p *PendingDeletes) OnNewReader(reader CodecReader, info *SegmentCommitInfo) error {
	if !p.liveDocsInitialized {
		if util.AssertsEnabled() && p.writeableLiveDocs != nil {
			panic(util.NewAssertionError(nil))
		}
		if reader.HasDeletions() {
			// we only initialize this once either in the ctor or here
			// if we use the live docs from a reader it has to be in a
			// situation where we don't have any existing live docs
			if util.AssertsEnabled() && p.pendingDeleteCount != 0 {
				panic(util.NewAssertionError(fmt.Sprintf("pendingDeleteCount: %d", p.pendingDeleteCount)))
			}
			p.liveDocs = reader.GetLiveDocs()
			if util.AssertsEnabled() && p.liveDocs != nil {
				p.assertCheckLiveDocs(p.liveDocs, info.SegmentInfo().MaxDoc(), info.DelCount())
			}
		}
		p.liveDocsInitialized = true
	}
	return nil
}

// assertCheckLiveDocs mirrors the private assertCheckLiveDocs(Bits, int, int).
func (p *PendingDeletes) assertCheckLiveDocs(bits util.Bits, expectedLength, expectedDeleteCount int) bool {
	if bits.Length() != expectedLength {
		panic(util.NewAssertionError(nil))
	}
	deletedCount := 0
	for i := 0; i < bits.Length(); i++ {
		if !bits.Get(i) {
			deletedCount++
		}
	}
	if deletedCount != expectedDeleteCount {
		panic(util.NewAssertionError(fmt.Sprintf("deleted: %d != expected: %d", deletedCount, expectedDeleteCount)))
	}
	return true
}

// DropChanges mirrors dropChanges(). Resets the pending docs.
func (p *PendingDeletes) DropChanges() {
	p.pendingDeleteCount = 0
}

// String mirrors toString().
func (p *PendingDeletes) String() string {
	return fmt.Sprintf("PendingDeletes(seg=%v numPendingDeletes=%d writeable=%t",
		p.info, p.pendingDeleteCount, p.writeableLiveDocs != nil)
}

// WriteLiveDocs mirrors writeLiveDocs(Directory).
func (p *PendingDeletes) WriteLiveDocs(dir store.Directory) (bool, error) {
	if p.pendingDeleteCount == 0 {
		return false, nil
	}

	liveDocs := p.liveDocs
	if util.AssertsEnabled() && liveDocs == nil {
		panic(util.NewAssertionError(nil))
	}
	// We have new deletes
	if util.AssertsEnabled() && liveDocs.Length() != p.info.SegmentInfo().MaxDoc() {
		panic(util.NewAssertionError(nil))
	}

	// Do this so we can delete any created files on exception; this saves
	// all codecs from having to do it:
	trackingDir := store.NewTrackingDirectoryWrapper(dir)

	// We can write directly to the actual name (vs to a .tmp & renaming it)
	// because the file is not live until segments file is written:
	codec := p.info.SegmentInfo().Codec()
	if err := codec.LiveDocsFormat().WriteLiveDocs(liveDocs, trackingDir, p.info, p.pendingDeleteCount, store.IOContextDefault); err != nil {
		// Advance only the nextWriteDelGen so that a 2nd attempt to write
		// will write to a new file
		p.info.AdvanceNextWriteDelGen()

		// Delete any partially created file(s):
		for fileName := range trackingDir.GetCreatedFiles() {
			store.DeleteFilesIgnoringExceptions(dir, fileName)
		}
		return false, err
	}

	// If we hit an exc in the line above (eg disk full) then info's delGen
	// remains pointing to the previous (successfully written) del docs:
	p.info.AdvanceDelGen()
	p.info.SetDelCount(p.info.DelCount() + p.pendingDeleteCount)
	p.this.DropChanges()
	return true, nil
}

// IsFullyDeleted mirrors isFullyDeleted(IOSupplier<CodecReader>).
func (p *PendingDeletes) IsFullyDeleted(readerIOSupplier func() (CodecReader, error)) (bool, error) {
	return p.GetDelCount() == p.info.SegmentInfo().MaxDoc(), nil
}

// OnDocValuesUpdate mirrors onDocValuesUpdate(FieldInfo,
// DocValuesFieldUpdates.Iterator): a no-op in the base class.
func (p *PendingDeletes) OnDocValuesUpdate(info *FieldInfo, iterator DocValuesFieldUpdatesIterator) error {
	return nil
}

// NumDeletesToMerge mirrors numDeletesToMerge(MergePolicy,
// IOSupplier<CodecReader>). Gocene's MergePolicy.NumDeletesToMerge does not
// take the reader supplier.
func (p *PendingDeletes) NumDeletesToMerge(policy MergePolicy, readerIOSupplier func() (CodecReader, error)) (int, error) {
	return policy.NumDeletesToMerge(p.info, p.GetDelCount()), nil
}

// NeedsRefresh mirrors the final needsRefresh(CodecReader).
func (p *PendingDeletes) NeedsRefresh(reader CodecReader) bool {
	return !sameBits(reader.GetLiveDocs(), p.this.GetLiveDocs()) || reader.NumDeletedDocs() != p.GetDelCount()
}

// sameBits renders Java reference equality between two Bits instances.
func sameBits(a, b util.Bits) bool {
	return sameDeleteQueueItem(a, b)
}

// GetDelCount mirrors the final getDelCount(): the number of deleted docs in
// the segment.
func (p *PendingDeletes) GetDelCount() int {
	return p.info.DelCount() + p.info.SoftDelCount() + p.this.NumPendingDeletes()
}

// NumDocs mirrors the final numDocs(): the number of live documents in this
// segment.
func (p *PendingDeletes) NumDocs() int {
	return p.info.SegmentInfo().MaxDoc() - p.GetDelCount()
}

// VerifyDocCounts mirrors verifyDocCounts(CodecReader), a consistency check
// called from assertions.
func (p *PendingDeletes) VerifyDocCounts(reader CodecReader) bool {
	count := 0
	liveDocs := p.this.GetLiveDocs()
	maxDoc := p.info.SegmentInfo().MaxDoc()
	if liveDocs != nil {
		for docID := 0; docID < maxDoc; docID++ {
			if liveDocs.Get(docID) {
				count++
			}
		}
	} else {
		count = maxDoc
	}
	if p.NumDocs() != count {
		panic(util.NewAssertionError(fmt.Sprintf(
			"info.maxDoc=%d info.getDelCount()=%d info.getSoftDelCount()=%d pendingDeletes=%s count=%d numDocs: %d",
			maxDoc, p.info.DelCount(), p.info.SoftDelCount(), p.this.String(), count, p.NumDocs())))
	}
	if reader.NumDocs() != p.NumDocs() {
		panic(util.NewAssertionError(fmt.Sprintf("reader.numDocs() = %d numDocs() %d", reader.NumDocs(), p.NumDocs())))
	}
	if reader.NumDeletedDocs() > maxDoc {
		panic(util.NewAssertionError(fmt.Sprintf(
			"delCount=%d info.maxDoc=%d rld.pendingDeleteCount=%d info.getDelCount()=%d",
			reader.NumDeletedDocs(), maxDoc, p.this.NumPendingDeletes(), p.info.DelCount())))
	}
	return true
}

// MustInitOnDelete mirrors mustInitOnDelete(): false in the base class.
func (p *PendingDeletes) MustInitOnDelete() bool {
	return false
}
