//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LiveIndexWriterConfig holds all the configuration used by IndexWriter with few setters for settings that can be changed on an IndexWriter instance "live".
//
// This is the Go port of Lucene's org.apache.lucene.index.LiveIndexWriterConfig.
type LiveIndexWriterConfig struct {
	analyzer analysis.Analyzer

	maxBufferedDocs int
	ramBufferSizeMB float64
	mergedSegmentWarmer IndexReaderWarmer

	delPolicy IndexDeletionPolicy
	commit    *IndexCommit
	openMode  OpenMode

	createdVersionMajor int

	similarity spi.Similarity
	mergeScheduler MergeScheduler
	codec          spi.Codec
	infoStream     util.InfoStream
	mergePolicy    MergePolicy
	flushPolicy    FlushPolicy
	readerPooling  bool
	perThreadHardLimitMB int
	useCompoundFile bool
	commitOnClose bool

	indexSort any // search.Sort
	leafSorter any // Comparator<LeafReader>
	indexSortFields map[string]struct{}

	parentField string
	checkPendingFlushOnUpdate bool
	softDeletesField string
	maxFullFlushMergeWaitMillis int64
	eventListener IndexWriterEventListener

	mu sync.RWMutex
}

func NewLiveIndexWriterConfig(analyzer analysis.Analyzer) *LiveIndexWriterConfig {
	return &LiveIndexWriterConfig{
		analyzer:              analyzer,
		ramBufferSizeMB:       16.0,
		maxBufferedDocs:       -1,
		delPolicy:             &KeepOnlyLastCommitDeletionPolicy{},
		useCompoundFile:       true,
		openMode:              CreateOrAppend,
		similarity:            similarities.DefaultSimilarity,
		mergeScheduler:        &ConcurrentMergeScheduler{},
		codec:                 spi.DefaultCodec,
		infoStream:            util.DefaultInfoStream,
		mergePolicy:           &TieredMergePolicy{},
		flushPolicy:           &FlushByRamOrCountsPolicy{},
		readerPooling:        true,
		perThreadHardLimitMB: 1945,
		maxFullFlushMergeWaitMillis: 500,
		eventListener:         IndexWriterEventListenerNoOp,
	}
}

func (c *LiveIndexWriterConfig) GetAnalyzer() analysis.Analyzer {
	return c.analyzer
}

func (c *LiveIndexWriterConfig) SetRAMBufferSizeMB(ramBufferSizeMB float64) *LiveIndexWriterConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ramBufferSizeMB != -1.0 && ramBufferSizeMB <= 0.0 {
		panic("ramBufferSize should be > 0.0 MB when enabled")
	}
	if ramBufferSizeMB == -1.0 && c.maxBufferedDocs == -1 {
		panic("at least one of ramBufferSize and maxBufferedDocs must be enabled")
	}
	c.ramBufferSizeMB = ramBufferSizeMB
	return c
}

func (c *LiveIndexWriterConfig) GetRAMBufferSizeMB() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ramBufferSizeMB
}

func (c *LiveIndexWriterConfig) SetMaxBufferedDocs(maxBufferedDocs int) *LiveIndexWriterConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	if maxBufferedDocs != -1 && maxBufferedDocs < 2 {
		panic("maxBufferedDocs must at least be 2 when enabled")
	}
	if maxBufferedDocs == -1 && c.ramBufferSizeMB == -1.0 {
		panic("at least one of ramBufferSize and maxBufferedDocs must be enabled")
	}
	c.maxBufferedDocs = maxBufferedDocs
	return c
}

func (c *LiveIndexWriterConfig) GetMaxBufferedDocs() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxBufferedDocs
}

func (c *LiveIndexWriterConfig) SetMergePolicy(mergePolicy MergePolicy) *LiveIndexWriterConfig {
	if mergePolicy == nil {
		panic("mergePolicy must not be null")
	}
	c.mu.Lock()
	c.mergePolicy = mergePolicy
	c.mu.Unlock()
	return c
}

func (c *LiveIndexWriterConfig) SetMergedSegmentWarmer(warmer IndexReaderWarmer) *LiveIndexWriterConfig {
	c.mu.Lock()
	c.mergedSegmentWarmer = warmer
	c.mu.Unlock()
	return c
}

func (c *LiveIndexWriterConfig) GetMergedSegmentWarmer() IndexReaderWarmer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mergedSegmentWarmer
}

func (c *LiveIndexWriterConfig) GetOpenMode() OpenMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.openMode
}

func (c *LiveIndexWriterConfig) GetIndexCreatedVersionMajor() int {
	return c.createdVersionMajor
}

func (c *LiveIndexWriterConfig) GetIndexDeletionPolicy() IndexDeletionPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.delPolicy
}

func (c *LiveIndexWriterConfig) GetIndexCommit() *IndexCommit {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.commit
}

func (c *LiveIndexWriterConfig) GetSimilarity() spi.Similarity {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.similarity
}

func (c *LiveIndexWriterConfig) GetMergeScheduler() MergeScheduler {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mergeScheduler
}

func (c *LiveIndexWriterConfig) GetCodec() spi.Codec {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.codec
}

func (c *LiveIndexWriterConfig) GetMergePolicy() MergePolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mergePolicy
}

func (c *LiveIndexWriterConfig) GetReaderPooling() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.readerPooling
}

func (c *LiveIndexWriterConfig) GetRAMPerThreadHardLimitMB() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.perThreadHardLimitMB
}

func (c *LiveIndexWriterConfig) GetFlushPolicy() FlushPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.flushPolicy
}

func (c *LiveIndexWriterConfig) GetInfoStream() util.InfoStream {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.infoStream
}

func (c *LiveIndexWriterConfig) SetUseCompoundFile(useCompoundFile bool) *LiveIndexWriterConfig {
	c.mu.Lock()
	c.useCompoundFile = useCompoundFile
	c.mu.Unlock()
	return c
}

func (c *LiveIndexWriterConfig) GetUseCompoundFile() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.useCompoundFile
}

func (c *LiveIndexWriterConfig) GetCommitOnClose() bool {
	return c.commitOnClose
}

func (c *LiveIndexWriterConfig) GetIndexSort() any {
	return c.indexSort
}

func (c *LiveIndexWriterConfig) GetLeafSorter() any {
	return c.leafSorter
}

func (c *LiveIndexWriterConfig) IsCheckPendingFlushOnUpdate() bool {
	return c.checkPendingFlushOnUpdate
}

func (c *LiveIndexWriterConfig) SetCheckPendingFlushUpdate(check bool) *LiveIndexWriterConfig {
	c.mu.Lock()
	c.checkPendingFlushOnUpdate = check
	c.mu.Unlock()
	return c
}

func (c *LiveIndexWriterConfig) GetSoftDeletesField() string {
	return c.softDeletesField
}

func (c *LiveIndexWriterConfig) GetMaxFullFlushMergeWaitMillis() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxFullFlushMergeWaitMillis
}

func (c *LiveIndexWriterConfig) GetIndexWriterEventListener() IndexWriterEventListener {
	return c.eventListener
}

func (c *LiveIndexWriterConfig) GetParentField() string {
	return c.parentField
}
