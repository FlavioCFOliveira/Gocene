// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/search/similarities"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexReaderWarmer is an interface for warming up segments after a merge.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReaderWarmer.
type IndexReaderWarmer interface {
	// Warm is called to warm up the given segment reader.
	Warm(reader *SegmentReader)
}

type OpenMode int

const (
	Create OpenMode = iota
	Append
	CreateOrAppend
)

// IndexWriterConfig holds all the configuration that is used to create an IndexWriter.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexWriterConfig.
type IndexWriterConfig struct {
	*LiveIndexWriterConfig
	writer any // IndexWriter
}

const (
	DisableAutoFlush = -1
	DefaultMaxBufferedDeleteTerms = DisableAutoFlush
	DefaultMaxBufferedDocs        = DisableAutoFlush
	DefaultRAMBufferSizeMB        = 16.0
	DefaultReaderPooling          = true
	DefaultRAMPerThreadHardLimitMB = 1945
	DefaultUseCompoundFileSystem  = true
	DefaultCommitOnClose          = true
	DefaultMaxFullFlushMergeWaitMillis = 500
)

func NewIndexWriterConfig() *IndexWriterConfig {
	return NewIndexWriterConfigWithAnalyzer(analysis.NewStandardAnalyzer())
}

func NewIndexWriterConfigWithAnalyzer(analyzer analysis.Analyzer) *IndexWriterConfig {
	return &IndexWriterConfig{
		LiveIndexWriterConfig: NewLiveIndexWriterConfig(analyzer),
	}
}

func (c *IndexWriterConfig) SetOpenMode(openMode OpenMode) *IndexWriterConfig {
	c.openMode = openMode
	return c
}

func (c *IndexWriterConfig) SetIndexCreatedVersionMajor(version int) *IndexWriterConfig {
	// simplified validation
	c.createdVersionMajor = version
	return c
}

func (c *IndexWriterConfig) SetIndexDeletionPolicy(delPolicy IndexDeletionPolicy) *IndexWriterConfig {
	if delPolicy == nil {
		panic("indexDeletionPolicy must not be null")
	}
	c.delPolicy = delPolicy
	return c
}

func (c *IndexWriterConfig) SetIndexCommit(commit *IndexCommit) *IndexWriterConfig {
	c.commit = commit
	return c
}

func (c *IndexWriterConfig) SetSimilarity(similarity spi.Similarity) *IndexWriterConfig {
	if similarity == nil {
		panic("similarity must not be null")
	}
	c.similarity = similarity
	return c
}

func (c *IndexWriterConfig) SetMergeScheduler(mergeScheduler MergeScheduler) *IndexWriterConfig {
	if mergeScheduler == nil {
		panic("mergeScheduler must not be null")
	}
	c.mergeScheduler = mergeScheduler
	return c
}

func (c *IndexWriterConfig) SetCodec(codec spi.Codec) *IndexWriterConfig {
	if codec == nil {
		panic("codec must not be null")
	}
	c.codec = codec
	return c
}

func (c *IndexWriterConfig) SetReaderPooling(readerPooling bool) *IndexWriterConfig {
	c.readerPooling = readerPooling
	return c
}

func (c *IndexWriterConfig) SetFlushPolicy(flushPolicy FlushPolicy) *IndexWriterConfig {
	if flushPolicy == nil {
		panic("flushPolicy must not be null")
	}
	c.flushPolicy = flushPolicy
	return c
}

func (c *IndexWriterConfig) SetRAMPerThreadHardLimitMB(limit int) *IndexWriterConfig {
	if limit <= 0 || limit >= 2048 {
		panic("PerThreadHardLimit must be between 0 and 2048MB")
	}
	c.perThreadHardLimitMB = limit
	return c
}

func (c *IndexWriterConfig) SetInfoStream(infoStream util.InfoStream) *IndexWriterConfig {
	if infoStream == nil {
		panic("Cannot set InfoStream to null")
	}
	c.infoStream = infoStream
	return c
}

func (c *IndexWriterConfig) SetCommitOnClose(commitOnClose bool) *IndexWriterConfig {
	c.commitOnClose = commitOnClose
	return c
}

func (c *IndexWriterConfig) SetMaxFullFlushMergeWaitMillis(wait int64) *IndexWriterConfig {
	c.maxFullFlushMergeWaitMillis = wait
	return c
}

func (c *IndexWriterConfig) SetIndexSort(sort any) *IndexWriterConfig {
	c.indexSort = sort
	return c
}

func (c *IndexWriterConfig) SetLeafSorter(sorter any) *IndexWriterConfig {
	c.leafSorter = sorter
	return c
}

func (c *IndexWriterConfig) SetSoftDeletesField(field string) *IndexWriterConfig {
	c.softDeletesField = field
	return c
}

func (c *IndexWriterConfig) SetIndexWriterEventListener(listener IndexWriterEventListener) *IndexWriterConfig {
	c.eventListener = listener
	return c
}

func (c *IndexWriterConfig) SetParentField(field string) *IndexWriterConfig {
	c.parentField = field
	return c
}

func (c *IndexWriterConfig) setIndexWriter(writer any) *IndexWriterConfig {
	if c.writer != nil {
		panic("do not share IndexWriterConfig instances across IndexWriters")
	}
	c.writer = writer
	return c
}
