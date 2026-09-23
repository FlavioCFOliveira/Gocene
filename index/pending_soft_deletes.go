// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PendingSoftDeletes mirrors org.apache.lucene.index.PendingSoftDeletes
// (Apache Lucene 10.5.0), a final class that extends PendingDeletes. The Go
// rendering embeds the base [PendingDeletes] and implements
// [PendingDeletesInterface]; the base's this field points at the
// PendingSoftDeletes so the final base methods dispatch to the overrides.
type PendingSoftDeletes struct {
	*PendingDeletes
	field        string
	dvGeneration int64
	hardDeletes  *PendingDeletes
}

// NewPendingSoftDeletes mirrors PendingSoftDeletes(String, SegmentCommitInfo).
func NewPendingSoftDeletes(field string, info *SegmentCommitInfo) *PendingSoftDeletes {
	p := &PendingSoftDeletes{
		PendingDeletes: NewPendingDeletes(info, nil, info.DelCount()+info.SoftDelCount() == 0),
		field:          field,
		dvGeneration:   -2,
		hardDeletes:    NewPendingDeletesFromInfo(info),
	}
	p.PendingDeletes.this = p
	return p
}

// NewPendingSoftDeletesFromReader mirrors PendingSoftDeletes(String,
// SegmentReader, SegmentCommitInfo).
func NewPendingSoftDeletesFromReader(field string, reader *SegmentReader, info *SegmentCommitInfo) *PendingSoftDeletes {
	p := &PendingSoftDeletes{
		PendingDeletes: NewPendingDeletesFromReader(reader, info),
		field:          field,
		dvGeneration:   -2,
		hardDeletes:    NewPendingDeletesFromReader(reader, info),
	}
	p.PendingDeletes.this = p
	return p
}

// Delete overrides delete(int).
func (p *PendingSoftDeletes) Delete(docID int) (bool, error) {
	// we need to fetch this first it might be a shared instance with
	// hardDeletes
	mutableBits := p.getMutableBits()
	hardDeleted, err := p.hardDeletes.Delete(docID)
	if err != nil {
		return false, err
	}
	if hardDeleted {
		if mutableBits.GetAndClear(docID) { // delete it here too!
			if util.AssertsEnabled() {
				again, err := p.hardDeletes.Delete(docID)
				if err != nil {
					return false, err
				}
				if again {
					panic(util.NewAssertionError(nil))
				}
			}
		} else {
			// if it was deleted subtract the delCount
			p.pendingDeleteCount--
			if util.AssertsEnabled() {
				p.assertPendingDeletes()
			}
		}
		return true, nil
	}
	return false, nil
}

// NumPendingDeletes overrides the protected numPendingDeletes().
func (p *PendingSoftDeletes) NumPendingDeletes() int {
	return p.PendingDeletes.NumPendingDeletes() + p.hardDeletes.NumPendingDeletes()
}

// OnNewReader overrides onNewReader(CodecReader, SegmentCommitInfo).
func (p *PendingSoftDeletes) OnNewReader(reader CodecReader, info *SegmentCommitInfo) error {
	if err := p.PendingDeletes.OnNewReader(reader, info); err != nil {
		return err
	}
	if err := p.hardDeletes.OnNewReader(reader, info); err != nil {
		return err
	}
	// only re-calculate this if we haven't seen this generation
	if p.dvGeneration < info.DocValuesGen() {
		var newDelCount int
		iterator, err := spi.FieldExistsQueryGetDocValuesDocIdSetIterator(p.field, reader)
		if err != nil {
			return err
		}
		hasDocs := false
		if iterator != nil {
			doc, err := iterator.NextDoc()
			if err != nil {
				return err
			}
			hasDocs = doc != util.NO_MORE_DOCS
		}
		if hasDocs {
			iterator, err = spi.FieldExistsQueryGetDocValuesDocIdSetIterator(p.field, reader)
			if err != nil {
				return err
			}
			newDelCount, err = applySoftDeletes(iterator, p.getMutableBits())
			if err != nil {
				return err
			}
			if util.AssertsEnabled() && newDelCount < 0 {
				panic(util.NewAssertionError(fmt.Sprintf(" illegal pending delete count: %d", newDelCount)))
			}
		} else {
			// nothing is deleted we don't have a soft deletes field in this segment
			newDelCount = 0
		}
		if util.AssertsEnabled() && info.SoftDelCount() != newDelCount {
			panic(util.NewAssertionError(fmt.Sprintf("softDeleteCount doesn't match %d != %d", info.SoftDelCount(), newDelCount)))
		}
		p.dvGeneration = info.DocValuesGen()
	}
	if util.AssertsEnabled() && !(p.GetDelCount() <= info.SegmentInfo().MaxDoc()) {
		panic(util.NewAssertionError(fmt.Sprintf("%d > %d", p.GetDelCount(), info.SegmentInfo().MaxDoc())))
	}
	return nil
}

// WriteLiveDocs overrides writeLiveDocs(Directory).
func (p *PendingSoftDeletes) WriteLiveDocs(dir store.Directory) (bool, error) {
	// we need to set this here to make sure our stats in SCI are up-to-date
	// otherwise we might hit an assertion when the hard deletes are set since
	// we need to account for docs that used to be only soft-delete but now
	// hard-deleted
	p.info.SetSoftDelCount(p.info.SoftDelCount() + p.pendingDeleteCount)
	p.PendingDeletes.DropChanges()
	// delegate the write to the hard deletes - it will only write if somebody
	// used it.
	return p.hardDeletes.WriteLiveDocs(dir)
}

// DropChanges overrides dropChanges(): don't reset anything here - this is
// called after a merge (successful or not) to prevent rewriting the deleted
// docs to disk. we only pass it on and reset the number of pending deletes
func (p *PendingSoftDeletes) DropChanges() {
	p.hardDeletes.DropChanges()
}

// applySoftDeletes mirrors the static applySoftDeletes(DocIdSetIterator,
// FixedBitSet) for a plain DocIdSetIterator: every visited doc is live and
// soft-deleted.
func applySoftDeletes(iterator util.DocIdSetIterator, bits *util.FixedBitSet) (int, error) {
	if util.AssertsEnabled() && iterator == nil {
		panic(util.NewAssertionError(nil))
	}
	newDeletes := 0
	for {
		docID, err := iterator.NextDoc()
		if err != nil {
			return newDeletes, err
		}
		if docID == util.NO_MORE_DOCS {
			return newDeletes, nil
		}
		if bits.GetAndClear(docID) { // doc is live - clear it
			newDeletes++
			// now that we know we deleted it and we fully control the hard
			// deletes we can do correct accounting below.
		}
	}
}

// applySoftDeletesFromUpdates mirrors the static applySoftDeletes when the
// iterator is a DocValuesFieldUpdates.Iterator (the hasValue branch). Gocene's
// DocValuesFieldUpdatesIterator is not a DocIdSetIterator, so the two
// dynamic cases of the Java method are two Go functions.
func applySoftDeletesFromUpdates(iterator DocValuesFieldUpdatesIterator, bits *util.FixedBitSet) int {
	newDeletes := 0
	for {
		docID := iterator.NextDoc()
		if docID == util.NO_MORE_DOCS {
			return newDeletes
		}
		if iterator.HasValue() {
			if bits.GetAndClear(docID) { // doc is live - clear it
				newDeletes++
			}
		} else {
			if !bits.GetAndSet(docID) {
				newDeletes--
			}
		}
	}
}

// OnDocValuesUpdate overrides onDocValuesUpdate(FieldInfo,
// DocValuesFieldUpdates.Iterator).
func (p *PendingSoftDeletes) OnDocValuesUpdate(info *FieldInfo, iterator DocValuesFieldUpdatesIterator) error {
	if p.field == info.Name() {
		p.pendingDeleteCount += applySoftDeletesFromUpdates(iterator, p.getMutableBits())
		if util.AssertsEnabled() {
			p.assertPendingDeletes()
		}
		p.info.SetSoftDelCount(p.info.SoftDelCount() + p.pendingDeleteCount)
		p.PendingDeletes.DropChanges()
	}
	if util.AssertsEnabled() && !(p.dvGeneration < info.DocValuesGen()) {
		panic(util.NewAssertionError(fmt.Sprintf("we have seen this generation update already: %d vs. %d", p.dvGeneration, info.DocValuesGen())))
	}
	if util.AssertsEnabled() && p.dvGeneration == -2 {
		panic(util.NewAssertionError("docValues generation is still uninitialized"))
	}
	p.dvGeneration = info.DocValuesGen()
	return nil
}

// assertPendingDeletes mirrors the private assertPendingDeletes().
func (p *PendingSoftDeletes) assertPendingDeletes() bool {
	if p.pendingDeleteCount+p.info.SoftDelCount() < 0 {
		panic(util.NewAssertionError(fmt.Sprintf("illegal pending delete count: %d", p.pendingDeleteCount+p.info.SoftDelCount())))
	}
	if !(p.info.SegmentInfo().MaxDoc() >= p.GetDelCount()) {
		panic(util.NewAssertionError(nil))
	}
	return true
}

// String overrides toString().
func (p *PendingSoftDeletes) String() string {
	return fmt.Sprintf("PendingSoftDeletes(seg=%v numPendingDeletes=%d field=%s dvGeneration=%d hardDeletes=%s",
		p.info, p.pendingDeleteCount, p.field, p.dvGeneration, p.hardDeletes.String())
}

// NumDeletesToMerge overrides numDeletesToMerge(MergePolicy,
// IOSupplier<CodecReader>).
func (p *PendingSoftDeletes) NumDeletesToMerge(policy MergePolicy, readerIOSupplier func() (CodecReader, error)) (int, error) {
	// initialize to ensure we have accurate counts
	if err := p.ensureInitialized(readerIOSupplier); err != nil {
		return 0, err
	}
	return p.PendingDeletes.NumDeletesToMerge(policy, readerIOSupplier)
}

// ensureInitialized mirrors the private ensureInitialized(IOSupplier<CodecReader>).
func (p *PendingSoftDeletes) ensureInitialized(readerIOSupplier func() (CodecReader, error)) error {
	if p.dvGeneration == -2 {
		fieldInfos, err := p.readFieldInfos()
		if err != nil {
			return err
		}
		fieldInfo := fieldInfos.FieldInfo(p.field)
		// we try to only open a reader if it's really necessary ie. indices
		// that are mainly append only might have big segments that don't even
		// have any docs in the soft deletes field. In such a case it's simply
		// enough to look at the FieldInfo for the field and check if the field
		// has DocValues
		if fieldInfo != nil && fieldInfo.DocValuesType() != DocValuesTypeNone {
			// in order to get accurate numbers we need to have at least one
			// reader see here.
			reader, err := readerIOSupplier()
			if err != nil {
				return err
			}
			return p.OnNewReader(reader, p.info)
		}
		// we are safe here since we don't have any doc values for the
		// soft-delete field on disk no need to open a new reader
		if fieldInfo == nil {
			p.dvGeneration = -1
		} else {
			p.dvGeneration = fieldInfo.DocValuesGen()
		}
	}
	return nil
}

// IsFullyDeleted overrides isFullyDeleted(IOSupplier<CodecReader>).
func (p *PendingSoftDeletes) IsFullyDeleted(readerIOSupplier func() (CodecReader, error)) (bool, error) {
	// initialize to ensure we have accurate counts - only needed in the
	// soft-delete case
	if err := p.ensureInitialized(readerIOSupplier); err != nil {
		return false, err
	}
	return p.PendingDeletes.IsFullyDeleted(readerIOSupplier)
}

// readFieldInfos mirrors the private readFieldInfos().
func (p *PendingSoftDeletes) readFieldInfos() (*FieldInfos, error) {
	segInfo := p.info.SegmentInfo()
	dir := segInfo.Directory()
	if !p.info.HasFieldUpdates() {
		// updates always outside of CFS
		if segInfo.GetUseCompoundFile() {
			cfs, err := segInfo.Codec().CompoundFormat().GetCompoundReader(segInfo.Directory(), segInfo)
			if err != nil {
				return nil, err
			}
			infos, readErr := segInfo.Codec().FieldInfosFormat().Read(cfs, segInfo, "", store.IOContextReadOnce)
			closeErr := cfs.Close()
			if readErr != nil {
				return nil, readErr
			}
			return infos, closeErr
		}
		return segInfo.Codec().FieldInfosFormat().Read(dir, segInfo, "", store.IOContextReadOnce)
	}
	fisFormat := segInfo.Codec().FieldInfosFormat()
	segmentSuffix := strconv.FormatInt(p.info.FieldInfosGen(), 36)
	return fisFormat.Read(dir, segInfo, segmentSuffix, store.IOContextReadOnce)
}

// GetHardLiveDocs overrides getHardLiveDocs().
func (p *PendingSoftDeletes) GetHardLiveDocs() util.Bits {
	return p.hardDeletes.GetLiveDocs()
}

// MustInitOnDelete overrides mustInitOnDelete().
func (p *PendingSoftDeletes) MustInitOnDelete() bool {
	return !p.liveDocsInitialized
}

// CountSoftDeletes mirrors the static countSoftDeletes(DocIdSetIterator, Bits).
func CountSoftDeletes(softDeletedDocs util.DocIdSetIterator, hardDeletes util.Bits) (int, error) {
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
