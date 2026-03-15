// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// OpenMode specifies how to open/create an index.
type OpenMode int

const (
	// CREATE creates a new index, removing any existing index.
	CREATE OpenMode = iota
	// APPEND opens an existing index.
	APPEND
	// CREATE_OR_APPEND creates a new index or opens an existing one.
	CREATE_OR_APPEND
)

// Default configuration constants for IndexWriterConfig.
// These match Apache Lucene's default values for byte-level compatibility.
const (
	// DISABLE_AUTO_FLUSH is a special value indicating auto-flush is disabled.
	DISABLE_AUTO_FLUSH = -1

	// DefaultRAMBufferSizeMB is the default RAM buffer size in MB (16.0 MB).
	DefaultRAMBufferSizeMB = 16.0

	// DefaultMaxBufferedDocs is the default number of buffered documents (-1 = disabled).
	DefaultMaxBufferedDocs = -1

	// DefaultMaxBufferedDeleteTerms is the default number of buffered delete terms (-1 = disabled).
	DefaultMaxBufferedDeleteTerms = -1

	// DefaultReaderPooling indicates whether reader pooling is enabled by default.
	DefaultReaderPooling = true

	// DefaultUseCompoundFile indicates whether compound files are used by default.
	DefaultUseCompoundFile = true
)

// IndexWriterConfig holds configuration for IndexWriter.
type IndexWriterConfig struct {
	openMode               OpenMode
	analyzer               analysis.Analyzer
	ramBufferSizeMB        float64
	maxBufferedDocs        int
	maxBufferedDeleteTerms int
	mergePolicy            MergePolicy
	mergeScheduler         MergeScheduler
	indexDeletionPolicy    IndexDeletionPolicy
	softDeletesField       string
	parentField            string
	useCompoundFile        bool
	codec                  Codec
	maxDocs                int
	indexSort              *Sort
}

// NewIndexWriterConfig creates a new IndexWriterConfig with default settings.
func NewIndexWriterConfig(analyzer analysis.Analyzer) *IndexWriterConfig {
	return &IndexWriterConfig{
		openMode:               CREATE_OR_APPEND,
		analyzer:               analyzer,
		ramBufferSizeMB:        16.0,
		maxBufferedDocs:        1000,
		maxBufferedDeleteTerms: -1,  // Disabled by default
		mergePolicy:            nil, // Will be set by IndexWriter
		mergeScheduler:         nil, // Will be set by IndexWriter
		indexDeletionPolicy:    nil, // Will be set by IndexWriter
		useCompoundFile:        DefaultUseCompoundFile,
		maxDocs:                0, // 0 means unlimited
	}
}

// GetMergePolicy returns the merge policy.
func (c *IndexWriterConfig) GetMergePolicy() MergePolicy {
	return c.mergePolicy
}

// SetMergePolicy sets the merge policy.
func (c *IndexWriterConfig) SetMergePolicy(policy MergePolicy) {
	c.mergePolicy = policy
}

// GetMergeScheduler returns the merge scheduler.
func (c *IndexWriterConfig) GetMergeScheduler() MergeScheduler {
	return c.mergeScheduler
}

// SetMergeScheduler sets the merge scheduler.
func (c *IndexWriterConfig) SetMergeScheduler(scheduler MergeScheduler) {
	c.mergeScheduler = scheduler
}

// GetIndexDeletionPolicy returns the index deletion policy.
func (c *IndexWriterConfig) GetIndexDeletionPolicy() IndexDeletionPolicy {
	return c.indexDeletionPolicy
}

// SetIndexDeletionPolicy sets the index deletion policy.
func (c *IndexWriterConfig) SetIndexDeletionPolicy(policy IndexDeletionPolicy) {
	c.indexDeletionPolicy = policy
}

func (c *IndexWriterConfig) OpenMode() OpenMode                { return c.openMode }
func (c *IndexWriterConfig) SetOpenMode(mode OpenMode)         { c.openMode = mode }
func (c *IndexWriterConfig) Analyzer() analysis.Analyzer       { return c.analyzer }
func (c *IndexWriterConfig) SetAnalyzer(a analysis.Analyzer)   { c.analyzer = a }
func (c *IndexWriterConfig) RAMBufferSizeMB() float64          { return c.ramBufferSizeMB }
func (c *IndexWriterConfig) SetRAMBufferSizeMB(size float64)   { c.ramBufferSizeMB = size }
func (c *IndexWriterConfig) MaxBufferedDocs() int              { return c.maxBufferedDocs }
func (c *IndexWriterConfig) SetMaxBufferedDocs(max int)        { c.maxBufferedDocs = max }
func (c *IndexWriterConfig) MaxBufferedDeleteTerms() int       { return c.maxBufferedDeleteTerms }
func (c *IndexWriterConfig) SetMaxBufferedDeleteTerms(max int) { c.maxBufferedDeleteTerms = max }

// SoftDeletesField returns the soft deletes field name.
func (c *IndexWriterConfig) SoftDeletesField() string { return c.softDeletesField }

// SetSoftDeletesField sets the soft deletes field name.
func (c *IndexWriterConfig) SetSoftDeletesField(field string) { c.softDeletesField = field }

// ParentField returns the parent field name.
func (c *IndexWriterConfig) ParentField() string { return c.parentField }

// SetParentField sets the parent field name.
func (c *IndexWriterConfig) SetParentField(field string) { c.parentField = field }

// UseCompoundFile returns whether to use compound files.
func (c *IndexWriterConfig) UseCompoundFile() bool { return c.useCompoundFile }

// SetUseCompoundFile sets whether to use compound files.
func (c *IndexWriterConfig) SetUseCompoundFile(use bool) { c.useCompoundFile = use }

// Codec returns the codec.
func (c *IndexWriterConfig) Codec() Codec { return c.codec }

// SetCodec sets the codec.
func (c *IndexWriterConfig) SetCodec(codec Codec) { c.codec = codec }

// MaxDocs returns the maximum number of documents.
func (c *IndexWriterConfig) MaxDocs() int { return c.maxDocs }

// SetMaxDocs sets the maximum number of documents.
func (c *IndexWriterConfig) SetMaxDocs(max int) { c.maxDocs = max }

// IndexSort returns the index sort.
func (c *IndexWriterConfig) IndexSort() *Sort { return c.indexSort }

// SetIndexSort sets the index sort.
func (c *IndexWriterConfig) SetIndexSort(sort *Sort) { c.indexSort = sort }

// String returns a string representation of the IndexWriterConfig.
// This includes all configuration settings for debugging purposes.
func (c *IndexWriterConfig) String() string {
	return fmt.Sprintf("IndexWriterConfig{openMode=%v, ramBufferSizeMB=%f, maxBufferedDocs=%d, maxBufferedDeleteTerms=%d, mergePolicy=%v, mergeScheduler=%v, indexDeletionPolicy=%v}",
		c.openMode,
		c.ramBufferSizeMB,
		c.maxBufferedDocs,
		c.maxBufferedDeleteTerms,
		c.mergePolicy,
		c.mergeScheduler,
		c.indexDeletionPolicy,
	)
}
