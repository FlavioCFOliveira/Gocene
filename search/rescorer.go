// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Rescorer re-scores the top documents from an initial search.
// This is the Go port of Lucene's org.apache.lucene.search.Rescorer.
type Rescorer interface {
	// Rescore re-scores the top documents.
	Rescore(searcher *IndexSearcher, topDocs *TopDocs) (*TopDocs, error)
}

// RescorerContext provides context for rescoring.
type RescorerContext struct {
	// WindowSize is the number of top documents to rescore
	WindowSize int
}

// NewRescorerContext creates a new RescorerContext.
func NewRescorerContext(windowSize int) *RescorerContext {
	return &RescorerContext{WindowSize: windowSize}
}
