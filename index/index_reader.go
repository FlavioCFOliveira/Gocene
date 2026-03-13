// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// IndexReaderInterface is a minimal interface for reading indexes.
type IndexReaderInterface interface {
	DocCount() int
	NumDocs() int
	MaxDoc() int
	Close() error
}

// IndexReader is an abstract base class for reading indexes.
type IndexReader struct {
	closed     bool
	docCount   int // total document count
	numDocs    int // number of live documents
	maxDoc     int // maximum document ID (one past the last)
	fieldInfos *FieldInfos
}

// NewIndexReader creates a new IndexReader.
func NewIndexReader() *IndexReader {
	return &IndexReader{
		closed:   false,
		docCount: 0,
		numDocs:  0,
		maxDoc:   0,
	}
}

// SetDocCount sets the total document count.
func (r *IndexReader) SetDocCount(docCount int) {
	r.docCount = docCount
}

// SetNumDocs sets the number of live documents.
func (r *IndexReader) SetNumDocs(numDocs int) {
	r.numDocs = numDocs
}

// SetMaxDoc sets the maximum document ID.
func (r *IndexReader) SetMaxDoc(maxDoc int) {
	r.maxDoc = maxDoc
}

// SetFieldInfos sets the FieldInfos.
func (r *IndexReader) SetFieldInfos(infos *FieldInfos) {
	r.fieldInfos = infos
}

// DocCount returns the total number of documents.
func (r *IndexReader) DocCount() int {
	return r.docCount
}

// NumDocs returns the number of live documents.
func (r *IndexReader) NumDocs() int {
	return r.numDocs
}

// MaxDoc returns the maximum document ID.
func (r *IndexReader) MaxDoc() int {
	return r.maxDoc
}

// GetFieldInfos returns the FieldInfos for the index.
func (r *IndexReader) GetFieldInfos() *FieldInfos {
	return r.fieldInfos
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