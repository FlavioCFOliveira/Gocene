// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// IndexReader is an abstract base class for reading indexes.
type IndexReader struct {
	closed bool
}

// NewIndexReader creates a new IndexReader.
func NewIndexReader() *IndexReader {
	return &IndexReader{closed: false}
}

// DocCount returns the total number of documents.
func (r *IndexReader) DocCount() int {
	// TODO: Implement
	return 0
}

// NumDocs returns the number of live documents.
func (r *IndexReader) NumDocs() int {
	// TODO: Implement
	return 0
}

// MaxDoc returns the maximum document ID.
func (r *IndexReader) MaxDoc() int {
	// TODO: Implement
	return 0
}

// GetFieldInfos returns the FieldInfos for the index.
func (r *IndexReader) GetFieldInfos() *FieldInfos {
	// TODO: Implement
	return nil
}

// Close closes the reader.
func (r *IndexReader) Close() error {
	r.closed = true
	return nil
}

// IsClosed returns true if the reader is closed.
func (r *IndexReader) IsClosed() bool {
	return r.closed
}

// IsCurrent returns true if the reader is still up to date with the index.
func (r *IndexReader) IsCurrent() (bool, error) {
	return true, nil
}
