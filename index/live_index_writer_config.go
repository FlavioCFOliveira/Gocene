// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// LiveIndexWriterConfig provides runtime-writable settings for IndexWriter.
//
// This is the Go port of Lucene's org.apache.lucene.index.LiveIndexWriterConfig.
//
// Unlike IndexWriterConfig which is set at creation time, LiveIndexWriterConfig
// allows changing certain settings dynamically while the IndexWriter is running.
type LiveIndexWriterConfig struct {
	// mergePolicy is the merge policy in use
	mergePolicy MergePolicy

	// mergeScheduler is the merge scheduler in use
	mergeScheduler MergeScheduler

	// ramBufferSizeMB is the RAM buffer size in MB
	ramBufferSizeMB float64

	// maxBufferedDocs is the maximum number of buffered documents
	maxBufferedDocs int

	// analyzer is the analyzer in use
	analyzer analysis.Analyzer
}

// NewLiveIndexWriterConfig creates a new LiveIndexWriterConfig from an IndexWriterConfig.
func NewLiveIndexWriterConfig(config *IndexWriterConfig) *LiveIndexWriterConfig {
	return &LiveIndexWriterConfig{
		mergePolicy:     config.GetMergePolicy(),
		mergeScheduler:  config.GetMergeScheduler(),
		ramBufferSizeMB: config.RAMBufferSizeMB(),
		maxBufferedDocs: config.MaxBufferedDocs(),
		analyzer:        config.Analyzer(),
	}
}

// GetMergePolicy returns the current merge policy.
func (c *LiveIndexWriterConfig) GetMergePolicy() MergePolicy {
	return c.mergePolicy
}

// SetMergePolicy sets the merge policy.
func (c *LiveIndexWriterConfig) SetMergePolicy(policy MergePolicy) {
	c.mergePolicy = policy
}

// GetMergeScheduler returns the current merge scheduler.
func (c *LiveIndexWriterConfig) GetMergeScheduler() MergeScheduler {
	return c.mergeScheduler
}

// SetMergeScheduler sets the merge scheduler.
func (c *LiveIndexWriterConfig) SetMergeScheduler(scheduler MergeScheduler) {
	c.mergeScheduler = scheduler
}

// GetRAMBufferSizeMB returns the RAM buffer size in MB.
func (c *LiveIndexWriterConfig) GetRAMBufferSizeMB() float64 {
	return c.ramBufferSizeMB
}

// SetRAMBufferSizeMB sets the RAM buffer size in MB.
func (c *LiveIndexWriterConfig) SetRAMBufferSizeMB(size float64) {
	c.ramBufferSizeMB = size
}

// GetMaxBufferedDocs returns the maximum number of buffered documents.
func (c *LiveIndexWriterConfig) GetMaxBufferedDocs() int {
	return c.maxBufferedDocs
}

// SetMaxBufferedDocs sets the maximum number of buffered documents.
func (c *LiveIndexWriterConfig) SetMaxBufferedDocs(max int) {
	c.maxBufferedDocs = max
}

// GetAnalyzer returns the analyzer.
func (c *LiveIndexWriterConfig) GetAnalyzer() analysis.Analyzer {
	return c.analyzer
}

// SetAnalyzer sets the analyzer.
func (c *LiveIndexWriterConfig) SetAnalyzer(analyzer analysis.Analyzer) {
	c.analyzer = analyzer
}
