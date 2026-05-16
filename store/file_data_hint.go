// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// FileDataHint is the Go port of org.apache.lucene.store.FileDataHint.
//
// It hints at the type of data stored in the file.
type FileDataHint uint8

const (
	// FileDataPostings indicates the file contains postings data.
	FileDataPostings FileDataHint = iota
	// FileDataKNNVectors indicates the file contains vector data for kNN
	// search.
	FileDataKNNVectors
)

// fileOpenHint satisfies the FileOpenHint marker interface.
func (FileDataHint) fileOpenHint() {}

// String returns the Lucene-equivalent constant name for the hint.
func (h FileDataHint) String() string {
	switch h {
	case FileDataPostings:
		return "POSTINGS"
	case FileDataKNNVectors:
		return "KNN_VECTORS"
	default:
		return "UNKNOWN"
	}
}
