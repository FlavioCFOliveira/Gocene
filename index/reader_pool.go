// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ReaderPool holds shared SegmentReader instances. IndexWriter uses
// SegmentReaders for 1) applying deletes/DV updates, 2) doing merges, 3)
// handing out a real-time reader. This pool reuses instances of the
// SegmentReaders in all these places if it is in "near real-time mode"
// (getReader() has been called on this instance).
//
// Mirrors org.apache.lucene.index.ReaderPool (Apache Lucene 10.5.0). Java's
// synchronized methods take mu; Java's LongSupplier completedDelGenSupplier is
// a func() int64.
type ReaderPool struct {
	mu sync.Mutex

	readerMap               map[*SegmentCommitInfo]*ReadersAndUpdates
	directory               store.Directory
	originalDirectory       store.Directory
	fieldNumbers            *FieldNumbers
	completedDelGenSupplier func() int64
	infoStream              util.InfoStream
	segmentInfos            *SegmentInfos
	softDeletesField        string
	// This is a "write once" variable (like the organic dye on a DVD-R that
	// may or may not be heated by a laser and then cooled to permanently
	// record the event): it's false, by default until enableReaderPooling()
	// is called for the first time, at which point it's switched to true and
	// never changes back to false. Once this is true, we hold open and reuse
	// SegmentReader instances internally for applying deletes, doing merges,
	// and reopening near real-time readers. in practice this should be called
	// once the readers are likely to be needed and reused ie if
	// IndexWriter#getReader is called.
	poolReaders atomic.Bool
	closed      atomic.Bool
}

// NewReaderPool mirrors the constructor ReaderPool(Directory directory,
// Directory originalDirectory, SegmentInfos segmentInfos,
// FieldInfos.FieldNumbers fieldNumbers, LongSupplier completedDelGenSupplier,
// InfoStream infoStream, String softDeletesField, StandardDirectoryReader
// reader). Java's null softDeletesField is the empty string. The reader
// argument is the reader IndexWriter is opened from (nil otherwise); Gocene's
// IndexCommit.getReader() yields a *DirectoryReader.
func NewReaderPool(
	directory store.Directory,
	originalDirectory store.Directory,
	segmentInfos *SegmentInfos,
	fieldNumbers *FieldNumbers,
	completedDelGenSupplier func() int64,
	infoStream util.InfoStream,
	softDeletesField string,
	reader *DirectoryReader,
) (*ReaderPool, error) {
	rp := &ReaderPool{
		readerMap:               make(map[*SegmentCommitInfo]*ReadersAndUpdates),
		directory:               directory,
		originalDirectory:       originalDirectory,
		segmentInfos:            segmentInfos,
		fieldNumbers:            fieldNumbers,
		completedDelGenSupplier: completedDelGenSupplier,
		infoStream:              infoStream,
		softDeletesField:        softDeletesField,
	}
	if reader != nil {
		// Pre-enroll all segment readers into the reader pool; this is
		// necessary so any in-memory NRT live docs are correctly carried
		// over, and so NRT readers pulled from this IW share the same
		// segment reader:
		leaves, err := reader.Leaves()
		if err != nil {
			return nil, err
		}
		if util.AssertsEnabled() && segmentInfos.Size() != len(leaves) {
			panic(util.NewAssertionError(nil))
		}
		for i, leaf := range leaves {
			segReader, ok := leaf.LeafReader().(*SegmentReader)
			if !ok {
				return nil, fmt.Errorf("ReaderPool: leaf %d is a %T, not a SegmentReader", i, leaf.LeafReader())
			}
			newReader := NewSegmentReaderClone(
				segmentInfos.Get(i),
				segReader,
				segReader.GetLiveDocs(),
				segReader.GetHardLiveDocs(),
				segReader.NumDocs(),
				true)
			info := newReader.GetSegmentCommitInfo()
			rld, err := NewReadersAndUpdatesFromReader(
				int(segmentInfos.GetIndexCreatedVersionMajor()),
				newReader,
				rp.newPendingDeletesFromReader(newReader, info))
			if err != nil {
				return nil, err
			}
			rp.readerMap[info] = rld
		}
	}
	return rp, nil
}

// assertInfoIsLive asserts this info still exists in IW's segment infos.
// Mirrors the synchronized assertInfoIsLive(SegmentCommitInfo).
func (rp *ReaderPool) assertInfoIsLive(info *SegmentCommitInfo) bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return rp.assertInfoIsLiveLocked(info)
}

func (rp *ReaderPool) assertInfoIsLiveLocked(info *SegmentCommitInfo) bool {
	idx := rp.segmentInfos.IndexOf(info)
	if util.AssertsEnabled() && idx == -1 {
		panic(util.NewAssertionError(fmt.Sprintf("info=%v isn't live", info)))
	}
	if util.AssertsEnabled() && rp.segmentInfos.Get(idx) != info {
		panic(util.NewAssertionError(fmt.Sprintf("info=%v doesn't match live info in segmentInfos", info)))
	}
	return true
}

// Drop drops the reader for the given SegmentCommitInfo if it's pooled and
// returns true if a reader is pooled. Mirrors drop(SegmentCommitInfo).
func (rp *ReaderPool) Drop(info *SegmentCommitInfo) (bool, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rld := rp.readerMap[info]
	if rld != nil {
		if util.AssertsEnabled() && info != rld.info {
			panic(util.NewAssertionError(nil))
		}
		delete(rp.readerMap, info)
		if err := rld.DropReaders(); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// RamBytesUsed returns the sum of the ram used by all the buffered readers
// and updates in MB. Mirrors ramBytesUsed().
func (rp *ReaderPool) RamBytesUsed() int64 {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	var bytes int64
	for _, rld := range rp.readerMap {
		bytes += rld.ramBytesUsed.Load()
	}
	return bytes
}

// AnyDeletions returns true iff any of the buffered readers and updates has
// at least one pending delete. Mirrors anyDeletions().
func (rp *ReaderPool) AnyDeletions() bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	for _, rld := range rp.readerMap {
		if rld.GetDelCount() > 0 {
			return true
		}
	}
	return false
}

// EnableReaderPooling enables reader pooling for this pool. This should be
// called once the readers in this pool are shared with an outside resource
// like an NRT reader. Once reader pooling is enabled a ReadersAndUpdates will
// be kept around in the reader pool on calling Release until the segment
// get dropped via calls to Drop or DropAll or Close. Reader pooling is
// disabled upon construction but can't be disabled again once it's enabled.
// Mirrors enableReaderPooling().
func (rp *ReaderPool) EnableReaderPooling() {
	rp.poolReaders.Store(true)
}

// IsReaderPoolingEnabled mirrors isReaderPoolingEnabled().
func (rp *ReaderPool) IsReaderPoolingEnabled() bool {
	return rp.poolReaders.Load()
}

// Release releases the ReadersAndUpdates. This should only be called if the
// Get is called with the 'create' parameter set to true. Returns true if any
// files were written by this release call. Mirrors
// release(ReadersAndUpdates, boolean).
func (rp *ReaderPool) Release(rld *ReadersAndUpdates, assertInfoLive bool) (bool, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	changed := false
	// Matches incRef in get:
	if err := rld.DecRef(); err != nil {
		return false, err
	}

	if rld.RefCount() == 0 {
		// This happens if the segment was just merged away, while a buffered
		// deletes packet was still applying deletes/updates to it.
		if _, present := rp.readerMap[rld.info]; util.AssertsEnabled() && present {
			panic(util.NewAssertionError(fmt.Sprintf(
				"seg=%v has refCount 0 but still unexpectedly exists in the reader pool", rld.info)))
		}
	} else {
		// Pool still holds a ref:
		if util.AssertsEnabled() && !(rld.RefCount() > 0) {
			panic(util.NewAssertionError(fmt.Sprintf("refCount=%d reader=%v", rld.RefCount(), rld.info)))
		}

		if _, pooled := rp.readerMap[rld.info]; !rp.poolReaders.Load() && rld.RefCount() == 1 && pooled {
			// This is the last ref to this RLD, and we're not pooling, so
			// remove it:
			wrote, err := rld.WriteLiveDocs(rp.directory)
			if err != nil {
				return changed, err
			}
			if wrote {
				// Make sure we only write del docs for a live segment:
				if util.AssertsEnabled() && assertInfoLive {
					rp.assertInfoIsLiveLocked(rld.info)
				}
				// Must checkpoint because we just created new _X_N.del and
				// field updates files; don't call IW.checkpoint because that
				// also increments SIS.version, which we do not want to do
				// here: it was done previously (after we invoked
				// BDS.applyDeletes), whereas here all we did was move the
				// state to disk:
				changed = true
			}
			wrote, err = rld.WriteFieldUpdates(rp.directory, rp.fieldNumbers, rp.completedDelGenSupplier(), rp.infoStream)
			if err != nil {
				return changed, err
			}
			if wrote {
				changed = true
			}
			if rld.GetNumDVUpdates() == 0 {
				if err := rld.DropReaders(); err != nil {
					return changed, err
				}
				delete(rp.readerMap, rld.info)
			}
			// else: We are forced to pool this segment until its deletes
			// fully apply (no delGen gaps)
		}
	}
	return changed, nil
}

// Close mirrors close().
func (rp *ReaderPool) Close() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if rp.closed.CompareAndSwap(false, true) {
		return rp.dropAllLocked()
	}
	return nil
}

// WriteAllDocValuesUpdates writes all doc values updates to disk if there are
// any and returns true iff any files were written. Mirrors
// writeAllDocValuesUpdates().
func (rp *ReaderPool) WriteAllDocValuesUpdates() (bool, error) {
	// this needs to be protected by the reader pool lock otherwise we hit
	// ConcurrentModificationException
	rp.mu.Lock()
	cp := make([]*ReadersAndUpdates, 0, len(rp.readerMap))
	for _, rld := range rp.readerMap {
		cp = append(cp, rld)
	}
	rp.mu.Unlock()
	anyWritten := false
	for _, rld := range cp {
		wrote, err := rld.WriteFieldUpdates(rp.directory, rp.fieldNumbers, rp.completedDelGenSupplier(), rp.infoStream)
		if err != nil {
			return anyWritten, err
		}
		anyWritten = anyWritten || wrote
	}
	return anyWritten, nil
}

// WriteDocValuesUpdatesForMerge writes all doc values updates of the given
// segments to disk if there are any and returns true iff any files were
// written. Mirrors writeDocValuesUpdatesForMerge(List<SegmentCommitInfo>).
func (rp *ReaderPool) WriteDocValuesUpdatesForMerge(infos []*SegmentCommitInfo) (bool, error) {
	anyWritten := false
	for _, info := range infos {
		rld, err := rp.Get(info, false)
		if err != nil {
			return anyWritten, err
		}
		if rld != nil {
			wrote, err := rld.WriteFieldUpdates(rp.directory, rp.fieldNumbers, rp.completedDelGenSupplier(), rp.infoStream)
			if err != nil {
				return anyWritten, err
			}
			anyWritten = anyWritten || wrote
			if err := rld.SetIsMerging(); err != nil {
				return anyWritten, err
			}
		}
	}
	return anyWritten, nil
}

// GetReadersByRam returns a list of all currently maintained
// ReadersAndUpdates sorted by it's ram consumption largest to smallest. This
// list can also contain readers that don't consume any ram at this point ie.
// don't have any updates buffered. Mirrors getReadersByRam().
func (rp *ReaderPool) GetReadersByRam() []*ReadersAndUpdates {
	type ramRecordingHolder struct {
		updates      *ReadersAndUpdates
		ramBytesUsed int64
	}
	rp.mu.Lock()
	if len(rp.readerMap) == 0 {
		rp.mu.Unlock()
		return []*ReadersAndUpdates{}
	}
	readersByRam := make([]ramRecordingHolder, 0, len(rp.readerMap))
	for _, rld := range rp.readerMap {
		// we have to record the RAM usage once and then sort since the RAM
		// usage can change concurrently and that will confuse the sort or
		// hit an assertion the we can acquire here is not enough we would
		// need to lock all ReadersAndUpdates to make sure it doesn't change
		readersByRam = append(readersByRam, ramRecordingHolder{updates: rld, ramBytesUsed: rld.ramBytesUsed.Load()})
	}
	rp.mu.Unlock()
	// Sort this outside of the lock by largest ramBytesUsed:
	sort.Slice(readersByRam, func(a, b int) bool {
		return readersByRam[a].ramBytesUsed > readersByRam[b].ramBytesUsed
	})
	out := make([]*ReadersAndUpdates, len(readersByRam))
	for i, h := range readersByRam {
		out[i] = h.updates
	}
	return out
}

// DropAll removes all our references to readers, and commits any pending
// changes. Mirrors dropAll().
func (rp *ReaderPool) DropAll() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	return rp.dropAllLocked()
}

func (rp *ReaderPool) dropAllLocked() error {
	var priorE error
	for info, rld := range rp.readerMap {
		// Important to remove as-we-go, not with .clear() in the end, in case
		// we hit an exception; otherwise we could over-decref if close() is
		// called again:
		delete(rp.readerMap, info)

		// NOTE: it is allowed that these decRefs do not actually close the
		// SRs; this happens when a near real-time reader is kept open after
		// the IndexWriter instance is closed:
		if err := rld.DropReaders(); err != nil {
			priorE = errors.Join(priorE, err)
		}
	}
	if util.AssertsEnabled() && len(rp.readerMap) != 0 {
		panic(util.NewAssertionError(nil))
	}
	return priorE
}

// Commit commits live docs changes for the segment readers for the provided
// infos. Mirrors commit(SegmentInfos).
func (rp *ReaderPool) Commit(infos *SegmentInfos) (bool, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	atLeastOneChange := false
	for info := range infos.Iterator() {
		rld := rp.readerMap[info]
		if rld != nil {
			if util.AssertsEnabled() && rld.info != info {
				panic(util.NewAssertionError(nil))
			}
			changed, err := rld.WriteLiveDocs(rp.directory)
			if err != nil {
				return atLeastOneChange, err
			}
			wrote, err := rld.WriteFieldUpdates(rp.directory, rp.fieldNumbers, rp.completedDelGenSupplier(), rp.infoStream)
			if err != nil {
				return atLeastOneChange, err
			}
			changed = changed || wrote

			if changed {
				// Make sure we only write del docs for a live segment:
				if util.AssertsEnabled() {
					rp.assertInfoIsLiveLocked(info)
				}
				// Must checkpoint because we just created new _X_N.del and
				// field updates files; don't call IW.checkpoint because that
				// also increments SIS.version, which we do not want to do
				// here: it was done previously (after we invoked
				// BDS.applyDeletes), whereas here all we did was move the
				// state to disk:
				atLeastOneChange = true
			}
		}
	}
	return atLeastOneChange, nil
}

// AnyDocValuesChanges returns true iff there are any buffered doc values
// updates. Mirrors anyDocValuesChanges().
func (rp *ReaderPool) AnyDocValuesChanges() bool {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	for _, rld := range rp.readerMap {
		// NOTE: we don't check for pending deletes because deletes carry over
		// in RAM to NRT readers
		if rld.GetNumDVUpdates() != 0 {
			return true
		}
	}
	return false
}

// Get obtains a ReadersAndLiveDocs instance from the readerPool. If create is
// true, you must later call Release. Mirrors get(SegmentCommitInfo,
// boolean); Java's unchecked AlreadyClosedException is returned as an error.
func (rp *ReaderPool) Get(info *SegmentCommitInfo, create bool) (*ReadersAndUpdates, error) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if util.AssertsEnabled() && info.SegmentInfo().Directory() != rp.originalDirectory {
		panic(util.NewAssertionError(fmt.Sprintf("info.dir=%v vs %v", info.SegmentInfo().Directory(), rp.originalDirectory)))
	}
	if rp.closed.Load() {
		if util.AssertsEnabled() && len(rp.readerMap) != 0 {
			panic(util.NewAssertionError(fmt.Sprintf("Reader map is not empty: %v", rp.readerMap)))
		}
		return nil, store.NewAlreadyClosedException("ReaderPool is already closed", nil)
	}

	rld := rp.readerMap[info]
	if rld == nil {
		if !create {
			return nil, nil
		}
		var err error
		rld, err = NewReadersAndUpdates(int(rp.segmentInfos.GetIndexCreatedVersionMajor()), info, rp.newPendingDeletes(info))
		if err != nil {
			return nil, err
		}
		// Steal initial reference:
		rp.readerMap[info] = rld
	} else if util.AssertsEnabled() && rld.info != info {
		panic(util.NewAssertionError(fmt.Sprintf("rld.info=%v info=%v isLive?=%t vs %t",
			rld.info, info, rp.assertInfoIsLiveLocked(rld.info), rp.assertInfoIsLiveLocked(info))))
	}

	if create {
		// Return ref to caller:
		if err := rld.IncRef(); err != nil {
			return nil, err
		}
	}

	if util.AssertsEnabled() {
		rp.noDups()
	}

	return rld, nil
}

// newPendingDeletes mirrors the private newPendingDeletes(SegmentCommitInfo).
func (rp *ReaderPool) newPendingDeletes(info *SegmentCommitInfo) PendingDeletesInterface {
	if rp.softDeletesField == "" {
		return NewPendingDeletesFromInfo(info)
	}
	return NewPendingSoftDeletes(rp.softDeletesField, info)
}

// newPendingDeletesFromReader mirrors the private
// newPendingDeletes(SegmentReader, SegmentCommitInfo).
func (rp *ReaderPool) newPendingDeletesFromReader(reader *SegmentReader, info *SegmentCommitInfo) PendingDeletesInterface {
	if rp.softDeletesField == "" {
		return NewPendingDeletesFromReader(reader, info)
	}
	return NewPendingSoftDeletesFromReader(rp.softDeletesField, reader, info)
}

// noDups makes sure that every segment appears only once in the pool.
// Mirrors the private noDups(), called from assertions.
func (rp *ReaderPool) noDups() bool {
	seen := make(map[string]struct{})
	for info := range rp.readerMap {
		name := info.SegmentInfo().Name()
		if _, dup := seen[name]; dup {
			panic(util.NewAssertionError("seen twice: " + name))
		}
		seen[name] = struct{}{}
	}
	return true
}
