// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PendingDeletes handles accounting and applying pending deletes for live segment readers.
// Mirrors org.apache.lucene.index.PendingDeletes from Apache Lucene 10.5.0.
type PendingDeletes struct {
	info                *SegmentCommitInfo
	liveDocs            util.Bits
	writeableLiveDocs   *util.FixedBitSet
	pendingDeleteCount  int
	liveDocsInitialized bool
}

// NewPendingDeletes constructs a PendingDeletes.
func NewPendingDeletes(info *SegmentCommitInfo, liveDocs util.Bits, liveDocsInitialized bool) *PendingDeletes {
	return &PendingDeletes{
		info:                info,
		liveDocs:            liveDocs,
		liveDocsInitialized: liveDocsInitialized,
		pendingDeleteCount:  0,
	}
}

// NewPendingDeletesFromReader constructs a PendingDeletes from a SegmentReader.
func NewPendingDeletesFromReader(reader *SegmentReader, info *SegmentCommitInfo) *PendingDeletes {
	pd := NewPendingDeletes(info, reader.GetLiveDocs(), true)
	pd.pendingDeleteCount = reader.NumDeletedDocs() - info.GetDelCount()
	return pd
}

func (p *PendingDeletes) getMutableBits() *util.FixedBitSet {
	if !p.liveDocsInitialized {
		panic("can't delete if liveDocs are not initialized")
	}
	if p.writeableLiveDocs == nil {
		if p.liveDocs != nil {
			p.writeableLiveDocs = util.FixedBitSetCopy(p.liveDocs)
		} else {
			p.writeableLiveDocs, _ = util.NewFixedBitSet(p.info.SegmentInfo().DocCount())
			p.writeableLiveDocs.SetRange(0, p.info.SegmentInfo().DocCount())
		}
		p.liveDocs = p.writeableLiveDocs.AsReadOnlyBits()
	}
	return p.writeableLiveDocs
}

// Delete marks a document as deleted in this segment.
func (p *PendingDeletes) Delete(docID int) bool {
	maxDoc := p.info.SegmentInfo().DocCount()
	if maxDoc <= 0 {
		panic("segment maxDoc must be positive")
	}
	mutableBits := p.getMutableBits()
	if docID < 0 || docID >= mutableBits.Length() {
		panic(fmt.Sprintf("out of bounds: docid=%d liveDocsLength=%d seg=%s maxDoc=%d",
			docID, mutableBits.Length(), p.info.SegmentInfo().Name(), maxDoc))
	}
	didDelete := mutableBits.GetAndClear(docID)
	if didDelete {
		p.pendingDeleteCount++
	}
	return didDelete
}

func (p *PendingDeletes) GetLiveDocs() util.Bits {
	p.writeableLiveDocs = nil
	return p.liveDocs
}

func (p *PendingDeletes) GetHardLiveDocs() util.Bits {
	return p.GetLiveDocs()
}

func (p *PendingDeletes) NumPendingDeletes() int {
	return p.pendingDeleteCount
}

func (p *PendingDeletes) OnNewReader(reader CodecReader, info *SegmentCommitInfo) error {
	if !p.liveDocsInitialized {
		if reader.HasDeletions() {
			if p.pendingDeleteCount != 0 {
				panic(fmt.Sprintf("pendingDeleteCount: %d", p.pendingDeleteCount))
			}
			p.liveDocs = reader.GetLiveDocs()
		}
		p.liveDocsInitialized = true
	}
	return nil
}

func (p *PendingDeletes) DropChanges() {
	p.pendingDeleteCount = 0
}

func (p *PendingDeletes) WriteLiveDocs(dir store.Directory) (bool, error) {
	if p.pendingDeleteCount == 0 {
		return false, nil
	}

	liveDocs := p.liveDocs
	if liveDocs == nil {
		panic("liveDocs must not be nil when writing")
	}
	if liveDocs.Length() != p.info.SegmentInfo().DocCount() {
		panic("liveDocs length mismatch")
	}

	// In Gocene, we don't have a TrackingDirectoryWrapper exactly like Lucene's
	// but we can implement a simple one or use a temporary file and rename.
	// For now, we'll call the codec directly.
	success := false
	defer func() {
		if !success {
			p.info.AdvanceNextWriteDelGen()
		}
	}()

	codec := p.info.SegmentInfo().GetCodec()
	err := codec.LiveDocsFormat().WriteLiveDocs(liveDocs, dir, p.info, p.pendingDeleteCount, store.IOContextDefault)
	if err != nil {
		return false, err
	}
	success = true

	p.info.AdvanceDelGen()
	p.info.SetDelCount(p.info.GetDelCount() + p.pendingDeleteCount)
	p.DropChanges()
	return true, nil
}

func (p *PendingDeletes) IsFullyDeleted(readerIOSupplier func() (CodecReader, error)) (bool, error) {
	return p.GetDelCount() == p.info.SegmentInfo().DocCount(), nil
}

func (p *PendingDeletes) NumDeletesToMerge(policy MergePolicy, readerIOSupplier func() (CodecReader, error)) (int, error) {
	return policy.NumDeletesToMerge(p.info, p.GetDelCount(), readerIOSupplier)
}

func (p *PendingDeletes) NeedsRefresh(reader CodecReader) bool {
	return reader.GetLiveDocs() != p.GetLiveDocs() || reader.NumDeletedDocs() != p.GetDelCount()
}

func (p *PendingDeletes) GetDelCount() int {
	return p.info.GetDelCount() + p.info.GetSoftDelCount() + p.pendingDeleteCount
}

func (p *PendingDeletes) NumDocs() int {
	return p.info.SegmentInfo().DocCount() - p.GetDelCount()
}

func (p *PendingDeletes) String() string {
	return fmt.Sprintf("PendingDeletes(seg=%v numPendingDeletes=%d writeable=%v)",
		p.info, p.pendingDeleteCount, p.writeableLiveDocs != nil)
}
