// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// TermVectors provides access to term vectors for documents.
// This is the Go port of Lucene's org.apache.lucene.index.TermVectors.
//
// TermVectors allows retrieving term vectors for documents.
// It wraps a TermVectorsReader and provides thread-safe access.
type TermVectors interface {
	// Prefetch prefetches term vectors for the given document IDs.
	// This is a hint to the implementation that these documents
	// will likely be accessed soon.
	Prefetch(docIDs []int) error

	// Get retrieves the term vectors for a single document.
	// Returns a Fields object containing all term vectors for the document.
	Get(docID int) (Fields, error)

	// GetField retrieves the term vector for a specific field in a document.
	// Returns nil if the field has no term vector.
	GetField(docID int, field string) (Terms, error)
}
