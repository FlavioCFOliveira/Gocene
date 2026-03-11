// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// IndexSearcher searches an index.
type IndexSearcher struct {
	reader *index.IndexReader
}

// NewIndexSearcher creates a new IndexSearcher.
func NewIndexSearcher(reader *index.IndexReader) *IndexSearcher {
	return &IndexSearcher{reader: reader}
}

// Search executes a query and returns TopDocs.
func (s *IndexSearcher) Search(query Query, n int) (*TopDocs, error) {
	// TODO: Implement search
	return &TopDocs{}, nil
}

// Doc returns the stored fields for a document.
func (s *IndexSearcher) Doc(docID int) (*index.Document, error) {
	// TODO: Implement
	return nil, nil
}

// GetIndexReader returns the IndexReader.
func (s *IndexSearcher) GetIndexReader() *index.IndexReader {
	return s.reader
}

// Close closes the searcher.
func (s *IndexSearcher) Close() error {
	return nil
}
