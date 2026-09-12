// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// StoredFields provides access to stored fields for documents.
// This is the Go port of Lucene's org.apache.lucene.index.StoredFields.
//
// StoredFields allows retrieving stored field values for documents.
// It wraps a StoredFieldsReader and provides thread-safe access.
type StoredFields interface {
	// Prefetch prefetches stored fields for the given document IDs.
	// This is a hint to the implementation that these documents
	// will likely be accessed soon.
	Prefetch(docIDs []int) error

	// Document retrieves the stored fields for a single document.
	// The visitor callback receives each stored field value.
	Document(docID int, visitor StoredFieldVisitor) error
}
