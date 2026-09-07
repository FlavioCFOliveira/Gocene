// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func generateSegmentID() []byte {
	return util.RandomId()
}

// DocumentsWriterPerThread handles document processing for a single thread.

// This is the Go port of org.apache.lucene.index.DocumentsWriterPerThread.
type DocumentsWriterPerThread struct {
	mu sync.Mutex

	abortingException error

	// codec is the codec used to write the segment.
	codec Codec
	// directory is the directory where the segment is written.
	directory *store.TrackingDirectoryWrapper
	// indexingChain handles the actual indexing of documents.
	indexingChain *IndexingChain

	// Updates for our still-in-RAM (to be flushed next) segment.
	pendingUpdates *BufferedUpdates
	// segmentInfo is the current segment we are working on.
	segmentInfo *SegmentInfo
	// aborted is true if we aborted the current segment.
	aborted bool
	// flushPending indicates if a flush is pending for this DWPT.
	flushPending bool
	flushPendingSet bool
	// lastCommittedBytesUsed is the RAM usage at the last commit.
	lastCommittedBytesUsed int64
	// hasFlushed is true if this DWPT has been flushed at least once.
	hasFlushed bool
	hasFlushedSet bool

	// fieldInfos builds field info as documents are added.
	fieldInfos *FieldInfosBuilder
	// infoStream is the info stream for logging.
	infoStream util.InfoStream
	// numDocsInRAM is the number of documents currently in RAM.
	numDocsInRAM int
	// deleteQueue handles the pending deletes.
	deleteQueue *DocumentsWriterDeleteQueue
	// deleteSlice is our local view of the delete queue.
	deleteSlice *DeleteSlice
	// pendingNumDocs tracks the total number of documents pending in the index.
	pendingNumDocs *atomic.Int64
	// indexWriterConfig is the configuration for the index writer.
	indexWriterConfig *LiveIndexWriterConfig
	// enableTestPoints enables test points for debugging.
	enableTestPoints bool
	// deleteDocIDs holds doc IDs that were marked as deleted due to non-aborting exceptions.
	deleteDocIDs []int
	// numDeletedDocIds is the number of docs in deleteDocIDs.
	numDeletedDocIds int
	// indexMajorVersionCreated is the version of the index being created.
	indexMajorVersionCreated int
	// hasParentField is true if a parent field is configured.
	hasParentField bool
}

// FlushedSegment represents a segment that has been flushed to disk.
type FlushedSegment struct {
	segmentInfo    *SegmentCommitInfo
	fieldInfos     *FieldInfos
	segmentUpdates *FrozenBufferedUpdates
	liveDocs       *util.FixedBitSet
	delCount       int
	sortMap        SorterDocMap
}

func newFlushedSegment(
	infoStream util.InfoStream,
	segmentInfo *SegmentCommitInfo,
	fieldInfos *FieldInfos,
	segmentUpdates *BufferedUpdates,
	liveDocs *util.FixedBitSet,
	delCount int,
	sortMap SorterDocMap,
) *FlushedSegment {
	var frozenUpdates *FrozenBufferedUpdates
	if segmentUpdates != nil && segmentUpdates.Any() {
		frozenUpdates = NewFrozenBufferedUpdates(infoStream, segmentUpdates, segmentInfo)
	}
	return &FlushedSegment{
		segmentInfo:    segmentInfo,
		fieldInfos:     fieldInfos,
		segmentUpdates: frozenUpdates,
		liveDocs:       liveDocs,
		delCount:       delCount,
		sortMap:        sortMap,
	}
}

func (dwpt *DocumentsWriterPerThread) onAbortingException(err error) {
	if err == nil {
		panic("aborting exception must not be nil")
	}
	if dwpt.abortingException != nil {
		panic("aborting exception has already been set")
	}
	dwpt.abortingException = err
}

// Abort discards all currently buffered docs and resets state.
func (dwpt *DocumentsWriterPerThread) Abort() error {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	dwpt.aborted = true
	dwpt.pendingNumDocs.Add(-int64(dwpt.numDocsInRAM))

	if dwpt.infoStream.IsEnabled("DWPT") {
		dwpt.infoStream.Message("DWPT", "now abort")
	}

	defer func() {
		if dwpt.infoStream.IsEnabled("DWPT") {
			dwpt.infoStream.Message("DWPT", "done abort")
		}
	}()

	dwpt.indexingChain.Abort()
	dwpt.pendingUpdates.Clear()

	return nil
}

func (dwpt *DocumentsWriterPerThread) IsAborted() bool {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	return dwpt.aborted
}

func NewDocumentsWriterPerThread(
	indexMajorVersionCreated int,
	segmentName string,
	directoryOrig store.Directory,
	directory store.Directory,
	indexWriterConfig *LiveIndexWriterConfig,
	deleteQueue *DocumentsWriterDeleteQueue,
	fieldInfos *FieldInfosBuilder,
	pendingNumDocs *atomic.Int64,
	enableTestPoints bool,
) *DocumentsWriterPerThread {
	dwpt := &DocumentsWriterPerThread{
		indexMajorVersionCreated: indexMajorVersionCreated,
		directory:               store.NewTrackingDirectoryWrapper(directory),
		fieldInfos:               fieldInfos,
		indexWriterConfig:        indexWriterConfig,
		infoStream:               indexWriterConfig.GetInfoStream(),
		codec:                    indexWriterConfig.GetCodec(),
		pendingNumDocs:           pendingNumDocs,
		deleteQueue:              deleteQueue,
		enableTestPoints:         enableTestPoints,
	}

	dwpt.pendingUpdates = NewBufferedUpdates(segmentName)
	dwpt.deleteSlice = deleteQueue.NewSlice()

		dwpt.segmentInfo = NewSegmentInfo(segmentName, -1, directoryOrig)
		dwpt.segmentInfo.SetVersion(util.Latest.String())
		dwpt.segmentInfo.SetMinVersion(util.Latest.String())
		dwpt.segmentInfo.SetCodec(dwpt.codec.Name())
		dwpt.segmentInfo.SetID(generateSegmentID())
		dwpt.segmentInfo.SetIndexSort(indexWriterConfig.GetIndexSort())
	// IndexingChain constructor in Gocene requires handles.
	// These are injected here to mirror Lucene's constructor logic.
	chain, err := NewIndexingChain(
		indexMajorVersionCreated,
		fieldInfos, // Assuming FieldInfosBuilder implements FieldInfosBuilderHandle
		indexWriterConfig,
		dwpt.onAbortingException,
		nil, // termsHash injected later or by factory
		nil, // storedFieldsConsumer
		nil, // vectorValuesConsumer
		nil, // termVectorsWriter
	)
	if err != nil {
		panic(err)
	}
	dwpt.indexingChain = chain

	return dwpt
}

func (dwpt *DocumentsWriterPerThread) TestPoint(message string) {
	if dwpt.enableTestPoints {
		if !dwpt.infoStream.IsEnabled("TP") {
			panic("TP info stream must be enabled to use test points")
		}
		dwpt.infoStream.Message("TP", message)
	}
}

func (dwpt *DocumentsWriterPerThread) SetTestSegmentIDSeed(seed string) {
	util.SetRandomIdSeed(seed)
}

func (dwpt *DocumentsWriterPerThread) reserveOneDoc() {
	if dwpt.pendingNumDocs.Add(1) > GetActualMaxDocs() {
		dwpt.pendingNumDocs.Add(-1)
		panic(fmt.Sprintf("number of documents in the index cannot exceed %d", GetActualMaxDocs()))
	}
}

func (dwpt *DocumentsWriterPerThread) reserveDocs(n int) {
	if n <= 0 {
		return
	}
	maxDocs := int64(GetActualMaxDocs())
	for {
		current := dwpt.pendingNumDocs.Load()
		next := current + int64(n)
		if next > maxDocs {
			panic(fmt.Sprintf("number of documents in the index cannot exceed %d", maxDocs))
		}
		if dwpt.pendingNumDocs.CompareAndSwap(current, next) {
			return
		}
	}
}

func (dwpt *DocumentsWriterPerThread) UpdateDocuments(
	docs [][]IndexableField,
	deleteNode Node,
	flushNotifications FlushNotifications,
	onNewDocOnRAM func(),
) (int64, error) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	if dwpt.abortingException != nil {
		return 0, fmt.Errorf("DWPT has hit aborting exception but is still indexing")
	}

	docsInRamBefore := dwpt.numDocsInRAM
	allDocsIndexed := false

	defer func() {
		if !allDocsIndexed && !dwpt.aborted {
			dwpt.deleteLastDocs(dwpt.numDocsInRAM - docsInRamBefore)
		}
		dwpt.maybeAbort("updateDocuments", flushNotifications)
	}()

	for i, doc := range docs {
		isLastDoc := i == len(docs)-1
		if !dwpt.hasParentField &&
			dwpt.segmentInfo.GetIndexSort() != nil &&
			!isLastDoc &&
			dwpt.indexMajorVersionCreated >= 10 { // LUCENE_10_0_0
			panic("a parent field must be set in order to use document blocks with index sorting; see IndexWriterConfig#setParentField")
		}

		dwpt.reserveOneDoc()
		dwpt.indexingChain.ProcessDocument(dwpt.numDocsInRAM, doc)
		dwpt.numDocsInRAM++
		onNewDocOnRAM()
	}

	if dwpt.numDocsInRAM-docsInRamBefore > 1 {
		dwpt.segmentInfo.SetHasBlocks()
	}
	allDocsIndexed = true

	return dwpt.finishDocuments(deleteNode, docsInRamBefore)
}

func (dwpt *DocumentsWriterPerThread) UpdateBatch(
	columnBatch ColumnBatch,
	deleteNode Node,
	flushNotifications FlushNotifications,
	onNewDocsOnRAM func(int),
) (int64, error) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	if dwpt.abortingException != nil {
		return 0, fmt.Errorf("DWPT has hit aborting exception but is still indexing")
	}

	docsInRamBefore := dwpt.numDocsInRAM
	numDocs := columnBatch.NumDocs()
	allDocsIndexed := false

	dwpt.reserveDocs(numDocs)
	dwpt.numDocsInRAM += numDocs
	onNewDocsOnRAM(numDocs)

	defer func() {
		if !allDocsIndexed && !dwpt.aborted {
			dwpt.deleteLastDocs(dwpt.numDocsInRAM - docsInRamBefore)
		}
		dwpt.maybeAbort("updateBatch", flushNotifications)
	}()

	dwpt.indexingChain.ProcessBatch(docsInRamBefore, columnBatch)
	allDocsIndexed = true

	return dwpt.finishDocuments(deleteNode, docsInRamBefore)
}

func (dwpt *DocumentsWriterPerThread) finishDocuments(deleteNode Node, docIdUpTo int) (int64, error) {
	var seqNo int64
	if deleteNode != nil {
		seqNo = dwpt.deleteQueue.Add(deleteNode)
		// In Java, this is an assertion: assert deleteSlice.isTail(deleteNode)
		dwpt.deleteSlice.Apply(dwpt.pendingUpdates, docIdUpTo)
		return seqNo, nil
	}

	seqNo = dwpt.deleteQueue.UpdateSlice(dwpt.deleteSlice)
	if seqNo < 0 {
		seqNo = -seqNo
		dwpt.deleteSlice.Apply(dwpt.pendingUpdates, docIdUpTo)
	} else {
		dwpt.deleteSlice.Reset()
	}

	return seqNo, nil
}

func (dwpt *DocumentsWriterPerThread) deleteLastDocs(docCount int) {
	from := dwpt.numDocsInRAM - docCount
	to := dwpt.numDocsInRAM

	// grow slice
	newLen := len(dwpt.deleteDocIDs) + (to - from)
	if cap(dwpt.deleteDocIDs) < newLen {
		newSlice := make([]int, newLen)
		copy(newSlice, dwpt.deleteDocIDs)
		dwpt.deleteDocIDs = newSlice
	} else {
		dwpt.deleteDocIDs = dwpt.deleteDocIDs[:newLen]
	}

	for docID := from; docID < to; docID++ {
		dwpt.deleteDocIDs[dwpt.numDeletedDocIds] = docID
		dwpt.numDeletedDocIds++
	}
}

func (dwpt *DocumentsWriterPerThread) GetNumDocsInRAM() int {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	return dwpt.numDocsInRAM
}

func (dwpt *DocumentsWriterPerThread) PrepareFlush() (*FrozenBufferedUpdates, error) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	if dwpt.numDocsInRAM <= 0 {
		return nil, fmt.Errorf("cannot prepare flush for segment with 0 docs")
	}

	globalUpdates := dwpt.deleteQueue.FreezeGlobalBuffer(dwpt.deleteSlice)
	if dwpt.deleteSlice != nil {
		dwpt.deleteSlice.Apply(dwpt.pendingUpdates, dwpt.numDocsInRAM)
		dwpt.deleteSlice.Reset()
	}
	return globalUpdates, nil
}

func countSoftDeletes(iter util.DocIdSetIterator, liveDocs *util.FixedBitSet) int {
	if iter == nil {
		return 0
	}
	count := 0
	for {
		docID := iter.NextDoc()
		if docID == util.NoDoc {
			break
		}
		if docID < liveDocs.Length() && liveDocs.Get(docID) {
			count++
		}
	}
	return count
}

func (dwpt *DocumentsWriterPerThread) Flush(flushNotifications FlushNotifications) (*FlushedSegment, error) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	if !dwpt.flushPending {
		panic("flush called but flushPending is not set")
	}
	if dwpt.numDocsInRAM <= 0 {
		panic("flush called on segment with 0 docs")
	}
	if !dwpt.deleteSlice.IsEmpty() {
		panic("all deletes must be applied in prepareFlush")
	}

	dwpt.segmentInfo.SetMaxDoc(dwpt.numDocsInRAM)

	flushState := &SegmentWriteState{
		Directory:   dwpt.directory,
		SegmentInfo: dwpt.segmentInfo,
		FieldInfos:  dwpt.fieldInfos.Build(),
		IOContext:    store.IOContextFlush(store.NewFlushInfo(dwpt.numDocsInRAM, dwpt.lastCommittedBytesUsed)),
	}

	if dwpt.aborted {
		if dwpt.infoStream.IsEnabled("DWPT") {
			dwpt.infoStream.Message("DWPT", "flush: skip because aborting is set")
		}
		return nil, nil
	}

	startTime := time.Now()

	if dwpt.infoStream.IsEnabled("DWPT") {
		dwpt.infoStream.Message("DWPT", fmt.Sprintf("flush postings as segment %s numDocs=%d", flushState.SegmentInfo.Name(), dwpt.numDocsInRAM))
	}

	var packableSortMap SorterDocMap
	var err error

	defer func() {
		dwpt.maybeAbort("flush", flushNotifications)
		dwpt.hasFlushed = true
		dwpt.hasFlushedSet = true
	}()

	// Soft deletes calculation
	var softDelCount int
	softDeletesField := dwpt.indexWriterConfig.GetSoftDeletesField()
	if softDeletesField != "" {
		softDeletedDocs := dwpt.indexingChain.GetHasDocValues(softDeletesField)
		softDelCount = countSoftDeletes(softDeletedDocs, flushState.LiveDocs)
	}

	packableSortMap, err = dwpt.indexingChain.Flush(flushState)
	if err != nil {
		dwpt.onAbortingException(err)
		return nil, err
	}

	// We clear this here because we already resolved them when writing postings.
	dwpt.pendingUpdates.ClearDeleteTerms()
	dwpt.segmentInfo.SetFiles(dwpt.directory.GetCreatedFiles())

		segmentInfoPerCommit := NewSegmentCommitInfo(
			dwpt.segmentInfo,
			0,
			-1,
		)
		segmentInfoPerCommit.SetSoftDelCount(softDelCount)
		segmentInfoPerCommit.SetID(generateSegmentID())
		segmentDeletes = nil
	} else {
		segmentDeletes = dwpt.pendingUpdates
	}

	var fs *FlushedSegment
	if packableSortMap != nil {
		// Assume pack() method exists on SorterDocMap to get the final version.
		// If not, we just use the map.
		fs = newFlushedSegment(
			dwpt.infoStream,
			segmentInfoPerCommit,
			flushState.FieldInfos,
			segmentDeletes,
			flushState.LiveDocs,
			dwpt.numDeletedDocIds,
			packableSortMap,
		)
	} else {
		fs = newFlushedSegment(
			dwpt.infoStream,
			segmentInfoPerCommit,
			flushState.FieldInfos,
			segmentDeletes,
			flushState.LiveDocs,
			dwpt.numDeletedDocIds,
			nil,
		)
	}

	err = dwpt.sealFlushedSegment(fs, packableSortMap, flushNotifications)
	if err != nil {
		return nil, err
	}

	if dwpt.infoStream.IsEnabled("DWPT") {
		dwpt.infoStream.Message("DWPT", fmt.Sprintf("flush time %v", time.Since(startTime)))
	}

	return fs, nil
}

func (dwpt *DocumentsWriterPerThread) maybeAbort(location string, flushNotifications FlushNotifications) {
	if dwpt.abortingException != nil && !dwpt.aborted {
		defer func() {
			flushNotifications.OnTragicEvent(dwpt.abortingException, location)
		}()
		dwpt.Abort()
	}
}

func (dwpt *DocumentsWriterPerThread) sealFlushedSegment(
	flushedSegment *FlushedSegment,
	sortMap SorterDocMap,
	flushNotifications FlushNotifications,
) error {
	newSegment := flushedSegment.segmentInfo

	SetDiagnostics(newSegment.Info, 1) // SOURCE_FLUSH

	context := store.IOContextFlush(store.NewFlushInfo(newSegment.Info.DocCount(), newSegment.SizeInBytes()))

	success := false
	defer func() {
		if !success && dwpt.infoStream.IsEnabled("DWPT") {
			dwpt.infoStream.Message("DWPT", fmt.Sprintf("hit exception creating compound file for newly flushed segment %s", newSegment.Info.Name()))
		}
	}()

	if dwpt.indexWriterConfig.UseCompoundFile() {
		originalFiles := newSegment.Info.Files()
		// Gocene should have this utility.
		CreateCompoundFile(
			dwpt.infoStream,
			dwpt.directory,
			newSegment.Info,
			context,
			flushNotifications.DeleteUnusedFiles,
		)
		// Mark original files for deletion.
		// In Java: filesToDelete.addAll(originalFiles).
		// We can handle this via the FlushNotifications.
		newSegment.Info.SetUseCompoundFile(true)
	}

	dwpt.codec.SegmentInfoFormat().Write(dwpt.directory, newSegment.Info, context)

	if flushedSegment.liveDocs != nil {
		delCount := flushedSegment.delCount
		if dwpt.infoStream.IsEnabled("DWPT") {
			dwpt.infoStream.Message("DWPT", fmt.Sprintf("flush: write %d deletes gen=%d", delCount, newSegment.Info.GetDelGen()))
		}

		var bits *util.FixedBitSet
		if sortMap == nil {
			bits = flushedSegment.liveDocs
		} else {
			// sortLiveDocs logic
			sortedLiveDocs := util.NewFixedBitSet(flushedSegment.liveDocs.Length())
			sortedLiveDocs.Set(0, flushedSegment.liveDocs.Length())
			for i := 0; i < flushedSegment.liveDocs.Length(); i++ {
				if !flushedSegment.liveDocs.Get(i) {
					sortedLiveDocs.Clear(sortMap.OldToNew(i))
				}
			}
			bits = sortedLiveDocs
		}

		dwpt.codec.LiveDocsFormat().WriteLiveDocs(bits, dwpt.directory, newSegment.Info, delCount, context)
		newSegment.SetDelCount(delCount)
		newSegment.AdvanceDelGen()
	}

	success = true
	return nil
}

func (dwpt *DocumentsWriterPerThread) RamBytesUsed() int64 {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	return int64(len(dwpt.deleteDocIDs)*4) +
		dwpt.pendingUpdates.RamBytesUsed() +
		dwpt.indexingChain.RamBytesUsed()
}

func (dwpt *DocumentsWriterPerThread) IsFlushPending() bool {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	return dwpt.flushPending
}

func (dwpt *DocumentsWriterPerThread) SetFlushPending() {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	if dwpt.flushPendingSet {
		panic("flushPending can only be set once")
	}
	dwpt.flushPending = true
	dwpt.flushPendingSet = true
}

func (dwpt *DocumentsWriterPerThread) GetLastCommittedBytesUsed() int64 {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	return dwpt.lastCommittedBytesUsed
}

func (dwpt *DocumentsWriterPerThread) GetCommitLastBytesUsedDelta() int64 {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	return dwpt.RamBytesUsedUnsafe() - dwpt.lastCommittedBytesUsed
}

func (dwpt *DocumentsWriterPerThread) CommitLastBytesUsed(delta int64) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	dwpt.lastCommittedBytesUsed += delta
}

func (dwpt *DocumentsWriterPerThread) Lock() {
	dwpt.mu.Lock()
}

func (dwpt *DocumentsWriterPerThread) Unlock() {
	dwpt.mu.Unlock()
}

func (dwpt *DocumentsWriterPerThread) TryLock() bool {
	return dwpt.mu.TryLock()
}

func (dwpt *DocumentsWriterPerThread) IsHeldByCurrentThread() bool {
	// Go's sync.Mutex does not provide a way to check if it is held by the current goroutine.
	// This is used as an assertion in Lucene.
	return true
}

func (dwpt *DocumentsWriterPerThread) RamBytesUsedUnsafe() int64 {
	return int64(len(dwpt.deleteDocIDs)*4) +
		dwpt.pendingUpdates.RamBytesUsed() +
		dwpt.indexingChain.RamBytesUsed()
}

func (dwpt *DocumentsWriterPerThread) HasFlushed() bool {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()
	return dwpt.hasFlushed
}

// CreateCompoundFile is a stub for the Lucene compound file creation logic.
// This is a GAP in the current port.
func CreateCompoundFile(
	infoStream util.InfoStream,
	directory store.Directory,
	info *SegmentInfo,
	context store.IOContext,
	deleteUnusedFiles func([]string),
) error {
	// In a real implementation, this would bundle multiple segment files into one .cfs file.
	return nil
}
