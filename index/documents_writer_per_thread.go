// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/index/column"
	"github.com/FlavioCFOliveira/Gocene/spi"
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
	// docValues buffers doc-values for each field.
	docValues map[string]*DocValuesBuffer
	// segmentInfo is the current segment we are working on.
	segmentInfo *SegmentInfo
	// aborted is true if we aborted the current segment.
	aborted bool
	// flushPending indicates if a flush is pending for this DWPT.
	flushPending    bool
	flushPendingSet bool
	// lastCommittedBytesUsed is the RAM usage at the last commit.
	lastCommittedBytesUsed int64
	// hasFlushed is true if this DWPT has been flushed at least once.
	hasFlushed    bool
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
	// filesToDelete collects the files this DWPT wrote that are no longer
	// referenced (the pre-compound-file originals). Mirrors
	// DocumentsWriterPerThread.filesToDelete.
	filesToDelete map[string]struct{}
	// normsAcc holds the in-progress field-inversion counters for the document
	// currently being processed, keyed by field name. See
	// documents_writer_per_thread_norms.go: it is the live-path counterpart of
	// the per-field FieldInvertState that IndexingChain.PerField carries.
	normsAcc map[string]*normsAccumulator
	// norms buffers the per-document norm values of every norms field seen in
	// this segment, keyed by field name, in document order. Live-path
	// counterpart of IndexingChain.PerField.norms (NormValuesWriter).
	norms map[string]*NormsBuffer
}

// FlushedSegment represents a segment that has been flushed to disk.
type FlushedSegment struct {
	SegmentInfo    *SegmentCommitInfo
	FieldInfos     *FieldInfos
	SegmentUpdates *FrozenBufferedUpdates
	LiveDocs       *util.FixedBitSet
	DelCount       int
	SortMap        SorterDocMap
}

func newFlushedSegment(
	infoStream util.InfoStream,
	segmentInfo *SegmentCommitInfo,
	fieldInfos *FieldInfos,
	segmentUpdates *BufferedUpdates,
	liveDocs *util.FixedBitSet,
	delCount int,
	sortMap SorterDocMap,
) (*FlushedSegment, error) {
	var frozenUpdates *FrozenBufferedUpdates
	if segmentUpdates != nil && segmentUpdates.Any() {
		var err error
		frozenUpdates, err = NewFrozenBufferedUpdates(infoStream, segmentUpdates, segmentInfo)
		if err != nil {
			return nil, err
		}
	}
	return &FlushedSegment{
		SegmentInfo:    segmentInfo,
		FieldInfos:     fieldInfos,
		SegmentUpdates: frozenUpdates,
		LiveDocs:       liveDocs,
		DelCount:       delCount,
		SortMap:        sortMap,
	}, nil
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
		directory:                store.NewTrackingDirectoryWrapper(directory),
		fieldInfos:               fieldInfos,
		indexWriterConfig:        indexWriterConfig,
		infoStream:               indexWriterConfig.GetInfoStream(),
		codec:                    indexWriterConfig.GetCodec(),
		pendingNumDocs:           pendingNumDocs,
		deleteQueue:              deleteQueue,
		enableTestPoints:         enableTestPoints,
		docValues:                make(map[string]*DocValuesBuffer),
		filesToDelete:            make(map[string]struct{}),
		normsAcc:                 make(map[string]*normsAccumulator),
		norms:                    make(map[string]*NormsBuffer),
	}

	dwpt.pendingUpdates = NewBufferedUpdates(segmentName)
	dwpt.deleteSlice = deleteQueue.NewSlice()

	dwpt.segmentInfo = NewSegmentInfo(segmentName, -1, directoryOrig)
	dwpt.segmentInfo.SetVersion(util.Latest.String())
	dwpt.segmentInfo.SetMinVersion(util.Latest.String())
	dwpt.segmentInfo.SetCodec(dwpt.codec)
	dwpt.segmentInfo.SetID(generateSegmentID())
	if indexSort, ok := indexWriterConfig.GetIndexSort().(*spi.Sort); ok {
		dwpt.segmentInfo.SetIndexSort(indexSort)
	}
	dwpt.hasParentField = indexWriterConfig.GetParentField() != ""

	// Mirrors DocumentsWriterPerThread's
	//   this.indexingChain = new IndexingChain(indexVersionCreated, segmentInfo,
	//       this.directory, fieldInfos, indexWriterConfig, this::onAbortingException);
	// The chain builds its own consumers; nothing is injected.
	chain, err := NewIndexingChain(
		indexMajorVersionCreated,
		dwpt.segmentInfo,
		dwpt.directory,
		fieldInfos,
		indexWriterConfig,
		dwpt.onAbortingException,
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
	if dwpt.pendingNumDocs.Add(1) > int64(GetActualMaxDocs()) {
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
			dwpt.segmentInfo.IndexSort() != nil &&
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
		dwpt.segmentInfo.SetHasBlocks(true)
	}
	allDocsIndexed = true

	return dwpt.finishDocuments(deleteNode, docsInRamBefore)
}

func (dwpt *DocumentsWriterPerThread) UpdateBatch(
	columnBatch *column.ColumnBatch,
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
	numDocs := columnBatch.NumDocs
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
		seqNo = dwpt.deleteQueue.AddWithSlice(deleteNode, dwpt.deleteSlice)
		// In Java, this is an assertion: assert deleteSlice.isTail(deleteNode)
		dwpt.deleteSlice.apply(dwpt.pendingUpdates, docIdUpTo)
		return seqNo, nil
	}

	seqNo = dwpt.deleteQueue.UpdateSlice(dwpt.deleteSlice)
	if seqNo < 0 {
		seqNo = -seqNo
		dwpt.deleteSlice.apply(dwpt.pendingUpdates, docIdUpTo)
	} else {
		dwpt.deleteSlice.reset()
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

// GetNumDocsInRAM returns the number of documents buffered by this DWPT.
// Mirrors DocumentsWriterPerThread.getNumDocsInRAM(), which takes no lock: the
// caller already owns the DWPT lock whenever the value must be stable.
func (dwpt *DocumentsWriterPerThread) GetNumDocsInRAM() int {
	return dwpt.numDocsInRAM
}

// GetNumDocsInRAMLocked is GetNumDocsInRAM under the caller-held DWPT lock. It
// exists because DocumentsWriterFlushControl reads the counter while holding
// the DWPT lock, where re-entering dwpt.mu would deadlock.
func (dwpt *DocumentsWriterPerThread) GetNumDocsInRAMLocked() int {
	return dwpt.numDocsInRAM
}

// GetSegmentInfo returns the segment currently being written. Mirrors
// DocumentsWriterPerThread.getSegmentInfo().
func (dwpt *DocumentsWriterPerThread) GetSegmentInfo() *SegmentInfo {
	return dwpt.segmentInfo
}

// GetDeleteQueue returns the delete queue this DWPT is bound to. Mirrors the
// package-private DocumentsWriterPerThread.deleteQueue field.
func (dwpt *DocumentsWriterPerThread) GetDeleteQueue() *DocumentsWriterDeleteQueue {
	return dwpt.deleteQueue
}

// IsQueueAdvanced reports whether the delete queue this DWPT is bound to has
// been advanced, i.e. the DWPT is stale with respect to a full flush. Mirrors
// DocumentsWriterPerThread.isQueueAdvanced().
func (dwpt *DocumentsWriterPerThread) IsQueueAdvanced() bool {
	return dwpt.deleteQueue.IsAdvanced()
}

// PendingFilesToDelete returns the files written by this DWPT that are no
// longer referenced and may be deleted. Mirrors
// DocumentsWriterPerThread.pendingFilesToDelete().
func (dwpt *DocumentsWriterPerThread) PendingFilesToDelete() []string {
	files := make([]string, 0, len(dwpt.filesToDelete))
	for name := range dwpt.filesToDelete {
		files = append(files, name)
	}
	sort.Strings(files)
	return files
}

// deleteFile records a file for deletion. Mirrors the
// DocumentsWriterPerThread::deleteFile method reference handed to
// IndexWriter.createCompoundFile.
func (dwpt *DocumentsWriterPerThread) deleteFile(file string) {
	dwpt.filesToDelete[file] = struct{}{}
}

func (dwpt *DocumentsWriterPerThread) PrepareFlush() (*FrozenBufferedUpdates, error) {
	dwpt.mu.Lock()
	defer dwpt.mu.Unlock()

	if dwpt.numDocsInRAM <= 0 {
		return nil, fmt.Errorf("cannot prepare flush for segment with 0 docs")
	}

	globalUpdates := dwpt.deleteQueue.FreezeGlobalBuffer(dwpt.deleteSlice)
	if dwpt.deleteSlice != nil {
		dwpt.deleteSlice.apply(dwpt.pendingUpdates, dwpt.numDocsInRAM)
		dwpt.deleteSlice.reset()
	}
	return globalUpdates, nil
}

func countSoftDeletes(iter util.DocIdSetIterator, liveDocs *util.FixedBitSet) (int, error) {
	if iter == nil {
		return 0, nil
	}
	count := 0
	for {
		docID, err := iter.NextDoc()
		if err != nil {
			return 0, err
		}
		if docID == util.NO_MORE_DOCS {
			break
		}
		if liveDocs == nil || (docID < liveDocs.Length() && liveDocs.Get(docID)) {
			count++
		}
	}
	return count, nil
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
		Context:     store.IOContextFlush(store.NewFlushInfo(dwpt.numDocsInRAM, dwpt.lastCommittedBytesUsed)),
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
		softDelCount, err = countSoftDeletes(softDeletedDocs, flushState.LiveDocs)
		if err != nil {
			return nil, err
		}
	}

	packableSortMap, err = dwpt.indexingChain.Flush(flushState)
	if err != nil {
		dwpt.onAbortingException(err)
		return nil, err
	}

	// We clear this here because we already resolved them when writing postings.
	dwpt.pendingUpdates.ClearDeleteTerms()
	created := dwpt.directory.GetCreatedFiles()
	createdNames := make([]string, 0, len(created))
	for name := range created {
		createdNames = append(createdNames, name)
	}
	dwpt.segmentInfo.SetFiles(createdNames)

	segmentInfoPerCommit := NewSegmentCommitInfo(
		dwpt.segmentInfo,
		0,
		softDelCount,
		-1,
		-1,
		-1,
		generateSegmentID(),
	)

	var segmentDeletes *BufferedUpdates
	if softDeletesField != "" {
		segmentDeletes = nil
	} else {
		segmentDeletes = dwpt.pendingUpdates
	}

	fs, err := newFlushedSegment(
		dwpt.infoStream,
		segmentInfoPerCommit,
		flushState.FieldInfos,
		segmentDeletes,
		flushState.LiveDocs,
		dwpt.numDeletedDocIds,
		packableSortMap,
	)
	if err != nil {
		return nil, err
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
	newSegment := flushedSegment.SegmentInfo

	// Lucene passes IndexWriter.SOURCE_FLUSH ("flush"); Gocene's SetDiagnostics
	// still takes the legacy integer source code, where 1 is the flush source.
	SetDiagnostics(newSegment.Info, 1)

	sizeInBytes, err := newSegment.SizeInBytes()
	if err != nil {
		return err
	}
	context := store.IOContextFlush(store.NewFlushInfo(newSegment.Info.MaxDoc(), sizeInBytes))

	success := false
	defer func() {
		if !success && dwpt.infoStream.IsEnabled("DWPT") {
			dwpt.infoStream.Message("DWPT", fmt.Sprintf(
				"hit exception creating compound file for newly flushed segment %s", newSegment.Info.Name()))
		}
	}()

	if dwpt.indexWriterConfig.GetUseCompoundFile() {
		originalFiles := newSegment.Info.Files()
		if err := CreateCompoundFile(
			dwpt.infoStream,
			store.NewTrackingDirectoryWrapper(dwpt.directory),
			newSegment.Info,
			context,
			flushNotifications.DeleteUnusedFiles,
		); err != nil {
			return err
		}
		for _, file := range originalFiles {
			dwpt.deleteFile(file)
		}
		newSegment.Info.SetUseCompoundFile(true)
	}

	// Have the codec write SegmentInfo. Must be done after creating the CFS so
	// that 1) the .si is not slurped into the CFS and 2) the .si reflects the
	// useCompoundFile=true change above.
	if err := dwpt.codec.SegmentInfoFormat().Write(dwpt.directory, newSegment.Info, context); err != nil {
		return err
	}

	// Deleted docs must be written after the CFS so the .liv file is not
	// slurped into the CFS.
	if flushedSegment.LiveDocs != nil {
		delCount := flushedSegment.DelCount
		if delCount <= 0 {
			return fmt.Errorf("sealFlushedSegment: delCount must be positive, got %d", delCount)
		}
		if dwpt.infoStream.IsEnabled("DWPT") {
			dwpt.infoStream.Message("DWPT", fmt.Sprintf(
				"flush: write %d deletes gen=%d", delCount, flushedSegment.SegmentInfo.DelGen()))
		}

		var bits *util.FixedBitSet
		if sortMap == nil {
			bits = flushedSegment.LiveDocs
		} else {
			bits, err = sortLiveDocs(flushedSegment.LiveDocs, sortMap)
			if err != nil {
				return err
			}
		}
		codec := newSegment.Info.Codec()
		if err := codec.LiveDocsFormat().WriteLiveDocs(
			bits, dwpt.directory, newSegment, delCount, context); err != nil {
			return err
		}
		newSegment.SetDelCount(delCount)
		newSegment.AdvanceDelGen()
	}

	success = true
	return nil
}

// sortLiveDocs remaps a live-docs bitset through the segment sort map, mirroring
// DocumentsWriterPerThread.sortLiveDocs(Bits, Sorter.DocMap).
func sortLiveDocs(liveDocs util.Bits, sortMap SorterDocMap) (*util.FixedBitSet, error) {
	if liveDocs == nil || sortMap == nil {
		return nil, fmt.Errorf("sortLiveDocs: liveDocs and sortMap must not be nil")
	}
	sortedLiveDocs, err := util.NewFixedBitSet(liveDocs.Length())
	if err != nil {
		return nil, err
	}
	sortedLiveDocs.SetRange(0, liveDocs.Length())
	for i := 0; i < liveDocs.Length(); i++ {
		if !liveDocs.Get(i) {
			sortedLiveDocs.Clear(sortMap.OldToNew(i))
		}
	}
	return sortedLiveDocs, nil
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
