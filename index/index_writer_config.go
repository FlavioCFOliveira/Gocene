// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
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
