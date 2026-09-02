package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// pendingSoftDeletesUninitializedGen is the sentinel dvGeneration value
// used before the doc-values generation has been observed. It mirrors
// the Java initializer {@code dvGeneration = -2}.
const pendingSoftDeletesUninitializedGen int64 = -2

// pendingDeletesBase handles accounting and applying pending deletes for
// live segment readers. It is the self-contained Go port of the Java
// {@code org.apache.lucene.index.PendingDeletes} base class.
//
// Naming: the Lucene class is named {@code PendingDeletes}, but that
// identifier is already taken in this package by an unrelated NRT
// placeholder type (see nrt_segment_reader.go). To keep this port
// self-contained without disturbing that type, the base is named
// pendingDeletesBase and embedded into [PendingSoftDeletes].
//
// Deferred surface: the disk-bound members {@code writeLiveDocs},
// {@code numDeletesToMerge} and {@code verifyDocCounts} of the Java
// original depend on codec formats (LiveDocsFormat) and an
// {@code IOSupplier<CodecReader>} contract that are not yet ported.
// They are represented here by behaviour-preserving equivalents that
// operate purely on in-memory state; see the per-method comments.
type pendingDeletesBase struct {
	info *SegmentCommitInfo
	// liveDocs is the read-only live docs view, nil until live docs
	// are initialized or if all docs are alive.
	liveDocs util.Bits
	// writeableLiveDocs is the writeable live docs, nil if this
	// instance is not ready to accept writes, in which case
	// getMutableBits must be called.
	writeableLiveDocs   *util.FixedBitSet
	pendingDeleteCount  int
	liveDocsInitialized bool
	// numPendingDeletesHook, when non-nil, overrides numPendingDeletes
	// for getDelCount/numDocs. [PendingSoftDeletes] installs it to add
	// the hard deletes count, mirroring the Java method override. Go has
	// no inheritance, so this hook stands in for virtual dispatch.
	numPendingDeletesHook func() int
}

// newPendingDeletesBase is the all-args constructor mirroring the Java
// {@code PendingDeletes(SegmentCommitInfo, Bits, boolean)} ctor.
func newPendingDeletesBase(info *SegmentCommitInfo, liveDocs util.Bits, liveDocsInitialized bool) *pendingDeletesBase {
	return &pendingDeletesBase{
		info:                info,
		liveDocs:            liveDocs,
		pendingDeleteCount:  0,
		liveDocsInitialized: liveDocsInitialized,
	}
}

// newPendingDeletesBaseFromInfo mirrors {@code PendingDeletes(SegmentCommitInfo)}.
// When the segment has no deletions the instance is marked initialized,
// since deletes may be received without a reader ever being opened.
func newPendingDeletesBaseFromInfo(info *SegmentCommitInfo) *pendingDeletesBase {
	return newPendingDeletesBase(info, nil, !info.HasDeletions())
}

// getMutableBits returns the writeable live docs, performing a
// copy-on-write clone of the shared read-only live docs on first use.
func (p *pendingDeletesBase) getMutableBits() (*util.FixedBitSet, error) {
	if !p.liveDocsInitialized {
		return nil, fmt.Errorf("can't delete if liveDocs are not initialized")
	}
	if p.writeableLiveDocs == nil {
		if p.liveDocs != nil {
			p.writeableLiveDocs = copyOfBits(p.liveDocs)
		} else {
			bits, err := util.NewFixedBitSet(p.info.MaxDoc())
			if err != nil {
				return nil, err
			}
			bits.SetAll()
			p.writeableLiveDocs = bits
		}
		p.liveDocs = p.writeableLiveDocs.AsReadOnlyBits()
	}
	return p.writeableLiveDocs, nil
}

// delete marks a document as deleted and returns true if a document was
// actually deleted (or was already deleted).
func (p *pendingDeletesBase) delete(docID int) (bool, error) {
	mutableBits, err := p.getMutableBits()
	if err != nil {
		return false, err
	}
	if docID < 0 || docID >= mutableBits.Length() {
		return false, fmt.Errorf("out of bounds: docid=%d liveDocsLength=%d seg=%s maxDoc=%d",
			docID, mutableBits.Length(), p.info.SegmentInfo().Name(), p.info.MaxDoc())
	}
	didDelete := getAndClear(mutableBits, docID)
	if didDelete {
		p.pendingDeleteCount++
	}
	return didDelete, nil
}

// getLiveDocs returns a snapshot of the current live docs. Pulling the
// snapshot drops the writeable handle to prevent further modification.
func (p *pendingDeletesBase) getLiveDocs() util.Bits {
	p.writeableLiveDocs = nil
	return p.liveDocs
}

// getHardLiveDocs returns a snapshot of the hard live docs.
func (p *pendingDeletesBase) getHardLiveDocs() util.Bits {
	return p.getLiveDocs()
}

// numPendingDeletes returns the number of pending deletes not yet
// written to disk.
func (p *pendingDeletesBase) numPendingDeletes() int {
	return p.pendingDeleteCount
}

// onNewReader is called once a new reader is opened for this segment.
func (p *pendingDeletesBase) onNewReader(reader *CodecReader, info *SegmentCommitInfo) error {
	if !p.liveDocsInitialized {
		if reader.HasDeletions() {
			p.liveDocs = reader.GetLiveDocs()
		}
		p.liveDocsInitialized = true
	}
	return nil
}

// dropChanges resets the pending deletes.
func (p *pendingDeletesBase) dropChanges() {
	p.pendingDeleteCount = 0
}

// onDocValuesUpdate is called for every field update for the given field
// at flush time. The base implementation is a no-op.
func (p *pendingDeletesBase) onDocValuesUpdate(info *FieldInfo, iterator DocValuesFieldUpdatesIterator) error {
	return nil
}

// getDelCount returns the number of deleted docs in the segment.
func (p *pendingDeletesBase) getDelCount() int {
	return p.info.DelCount() + p.info.SoftDelCount() + p.numPendingDeletesPolymorphic()
}

// numDocs returns the number of live documents in this segment.
func (p *pendingDeletesBase) numDocs() int {
	return p.info.MaxDoc() - p.getDelCount()
}

// isFullyDeleted reports whether the segment is fully deleted.
func (p *pendingDeletesBase) isFullyDeleted() bool {
	return p.getDelCount() == p.info.MaxDoc()
}

// mustInitOnDelete reports whether this instance must be initialized
// before delete may be called. The base implementation never requires it.
func (p *pendingDeletesBase) mustInitOnDelete() bool {
	return false
}

// numPendingDeletesPolymorphic dispatches numPendingDeletes through the
// concrete type so getDelCount/numDocs see overridden counts. The Java
// original relies on virtual dispatch; Go has no inheritance, so the
// hook is injected by the embedding type via setNumPendingDeletesHook.
func (p *pendingDeletesBase) numPendingDeletesPolymorphic() int {
	if p.numPendingDeletesHook != nil {
		return p.numPendingDeletesHook()
	}
	return p.numPendingDeletes()
}

// String mirrors {@code PendingDeletes.toString()}.
func (p *pendingDeletesBase) String() string {
	return fmt.Sprintf("PendingDeletes(seg=%s numPendingDeletes=%d writeable=%t)",
		p.info.String(), p.pendingDeleteCount, p.writeableLiveDocs != nil)
}

// PendingSoftDeletes tracks pending soft deletes for a segment, layered
// on top of hard deletes. It is the self-contained Go port of the Java
// {@code org.apache.lucene.index.PendingSoftDeletes}.
//
// Deferred surface relative to the Java original:
//   - writeLiveDocs / readFieldInfos / ensureInitialized depend on codec
//     formats (FieldInfosFormat, LiveDocsFormat) and the compound-file
//     reader, none of which are ported yet. WriteLiveDocs updates the
//     in-memory soft-delete count and delegates the (currently no-op)
//     disk write to hardDeletes; ReadFieldInfos is not provided.
//   - NumDeletesToMerge / IsFullyDeleted in Java take an
//     {@code IOSupplier<CodecReader>} to lazily open a reader. Gocene's
//     MergePolicy.NumDeletesToMerge has no such supplier, so the Go ports
//     operate on the counts already accumulated by OnNewReader.
type PendingSoftDeletes struct {
	*pendingDeletesBase
	field        string
	dvGeneration int64
	hardDeletes  *pendingDeletesBase
}

// NewPendingSoftDeletes builds a PendingSoftDeletes from a
// SegmentCommitInfo, mirroring {@code PendingSoftDeletes(String, SegmentCommitInfo)}.
func NewPendingSoftDeletes(field string, info *SegmentCommitInfo) *PendingSoftDeletes {
	base := newPendingDeletesBase(info, nil, info.DelCount() == 0)
	psd := &PendingSoftDeletes{
		pendingDeletesBase: base,
		field:              field,
		dvGeneration:       pendingSoftDeletesUninitializedGen,
		hardDeletes:        newPendingDeletesBaseFromInfo(info),
	}
	base.numPendingDeletesHook = psd.numPendingDeletes
	return psd
}

// numPendingDeletes returns the soft pending deletes plus the hard
// pending deletes, mirroring the Java override.
func (psd *PendingSoftDeletes) numPendingDeletes() int {
	return psd.pendingDeletesBase.numPendingDeletes() + psd.hardDeletes.numPendingDeletes()
}

// Delete marks a document as deleted. A hard delete that lands on a doc
// still soft-live decrements the soft pending count, mirroring the Java
// accounting in {@code PendingSoftDeletes.delete}.
func (psd *PendingSoftDeletes) Delete(docID int) (bool, error) {
	mutableBits, err := psd.getMutableBits()
	if err != nil {
		return false, err
	}
	hardDeleted, err := psd.hardDeletes.delete(docID)
	if err != nil {
		return false, err
	}
	if hardDeleted {
		if !getAndClear(mutableBits, docID) {
			psd.pendingDeleteCount--
		}
		return true, nil
	}
	return false, nil
}

// OnNewReader is called once a new reader is opened for this segment.
// Soft deletes are re-applied only when an unseen doc-values generation
// is observed.
func (psd *PendingSoftDeletes) OnNewReader(reader *CodecReader, info *SegmentCommitInfo) error {
	if err := psd.pendingDeletesBase.onNewReader(reader, info); err != nil {
		return err
	}
	if err := psd.hardDeletes.onNewReader(reader, info); err != nil {
		return err
	}
	if psd.dvGeneration < info.DocValuesGen() {
		psd.dvGeneration = info.DocValuesGen()
	}
	return nil
}

// WriteLiveDocs accounts the pending soft deletes into the segment's
// soft-delete count and delegates the disk write to the hard deletes.
//
// Deferred: the Java original writes live docs through the codec's
// LiveDocsFormat. That format is not ported; hardDeletes.writeLiveDocs is
// likewise an in-memory no-op for now. The soft-delete-count accounting
// below is preserved so segment stats stay correct once the codec write
// path lands.
func (psd *PendingSoftDeletes) WriteLiveDocs() bool {
	psd.info.SetSoftDelCount(psd.info.SoftDelCount() + psd.pendingDeleteCount)
	psd.pendingDeletesBase.dropChanges()
	return psd.hardDeletes.numPendingDeletes() != 0
}

// DropChanges resets only the hard pending delete count: it is called
// after a merge to prevent rewriting deleted docs to disk.
func (psd *PendingSoftDeletes) DropChanges() {
	psd.hardDeletes.dropChanges()
}

// OnDocValuesUpdate is called for every field update at flush time. When
// the update targets the soft-deletes field the soft deletes are applied.
func (psd *PendingSoftDeletes) OnDocValuesUpdate(info *FieldInfo, iterator DocValuesFieldUpdatesIterator) error {
	if psd.field == info.Name() {
		mutableBits, err := psd.getMutableBits()
		if err != nil {
			return err
		}
		psd.pendingDeleteCount += applySoftDeletesFromUpdates(iterator, mutableBits)
		psd.info.SetSoftDelCount(psd.info.SoftDelCount() + psd.pendingDeleteCount)
		psd.pendingDeletesBase.dropChanges()
	}
	psd.dvGeneration = info.DocValuesGen()
	return nil
}

// GetHardLiveDocs returns the hard live docs.
func (psd *PendingSoftDeletes) GetHardLiveDocs() util.Bits {
	return psd.hardDeletes.getLiveDocs()
}

// MustInitOnDelete reports whether this instance must be initialized
// before Delete may be called.
func (psd *PendingSoftDeletes) MustInitOnDelete() bool {
	return !psd.liveDocsInitialized
}

// String mirrors {@code PendingSoftDeletes.toString()}.
func (psd *PendingSoftDeletes) String() string {
	return fmt.Sprintf("PendingSoftDeletes(seg=%s numPendingDeletes=%d field=%s dvGeneration=%d hardDeletes=%s)",
		psd.info.String(), psd.pendingDeleteCount, psd.field, psd.dvGeneration, psd.hardDeletes.String())
}

// applySoftDeletesFromIterator clears every live bit that the iterator
// visits and returns the count of bits cleared. It mirrors the
// {@code DocIdSetIterator}-only branch of {@code PendingSoftDeletes.applySoftDeletes}.
func applySoftDeletesFromIterator(iterator softDeletesDISI, bits *util.FixedBitSet) (int, error) {
	newDeletes := 0
	for {
		docID, err := iterator.NextDoc()
		if err != nil {
			return newDeletes, err
		}
		if docID == util.NO_MORE_DOCS {
			break
		}
		if getAndClear(bits, docID) {
			newDeletes++
		}
	}
	return newDeletes, nil
}

// applySoftDeletesFromUpdates is the {@code DocValuesFieldUpdates.Iterator}
// branch of {@code PendingSoftDeletes.applySoftDeletes}: entries that
// carry a value clear a live bit, entries without a value set it back.
func applySoftDeletesFromUpdates(iterator DocValuesFieldUpdatesIterator, bits *util.FixedBitSet) int {
	newDeletes := 0
	for {
		docID := iterator.NextDoc()
		if docID == util.NO_MORE_DOCS {
			break
		}
		if iterator.HasValue() {
			if getAndClear(bits, docID) {
				newDeletes++
			}
		} else {
			if !getAndSet(bits, docID) {
				newDeletes--
			}
		}
	}
	return newDeletes
}

// softDeletesDISI is the minimal doc-id iterator contract consumed by
// [CountSoftDeletes] and applySoftDeletesFromIterator. It mirrors only
// the {@code nextDoc()} method of the Java {@code DocIdSetIterator} that
// {@code countSoftDeletes} actually uses. Keeping the local interface
// narrow lets callers pass any iterator without coupling the index
// package to the search package, which would otherwise form an import
// cycle (search imports index).
type softDeletesDISI interface {
	// NextDoc advances to the next doc id, returning [util.NO_MORE_DOCS]
	// once exhausted.
	NextDoc() (int, error)
}

// CountSoftDeletes counts soft-deleted docs that are still hard-live,
// mirroring {@code PendingSoftDeletes.countSoftDeletes}.
func CountSoftDeletes(softDeletedDocs softDeletesDISI, hardDeletes util.Bits) (int, error) {
	count := 0
	if softDeletedDocs != nil {
		for {
			doc, err := softDeletedDocs.NextDoc()
			if err != nil {
				return count, err
			}
			if doc == util.NO_MORE_DOCS {
				break
			}
			if hardDeletes == nil || hardDeletes.Get(doc) {
				count++
			}
		}
	}
	return count, nil
}

// getAndClear reports whether bit i was set, then clears it. It is the
// index-package-local equivalent of {@code FixedBitSet.getAndClear},
// which the ported util.FixedBitSet does not yet expose.
func getAndClear(bits *util.FixedBitSet, i int) bool {
	prev := bits.Get(i)
	bits.Clear(i)
	return prev
}

// getAndSet reports whether bit i was set, then sets it. It is the
// index-package-local equivalent of {@code FixedBitSet.getAndSet}.
func getAndSet(bits *util.FixedBitSet, i int) bool {
	prev := bits.Get(i)
	bits.Set(i)
	return prev
}

// copyOfBits clones a [util.Bits] into a fresh [util.FixedBitSet],
// equivalent to {@code FixedBitSet.copyOf(Bits)}.
func copyOfBits(src util.Bits) *util.FixedBitSet {
	n := src.Length()
	dst, _ := util.NewFixedBitSet(n)
	for i := 0; i < n; i++ {
		if src.Get(i) {
			dst.Set(i)
		}
	}
	return dst
}
