//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// ReaderSlice represents a subreader slice from a parent composite reader.
// Mirrors org.apache.lucene.index.ReaderSlice from Apache Lucene 10.5.0.
type ReaderSlice struct {
	// Start is the document ID this slice starts from.
	Start int
	// Length is the number of documents in this slice.
	Length int
	// ReaderIndex is the sub-reader index for this slice.
	ReaderIndex int
}

// NewReaderSlice constructs a ReaderSlice.
func NewReaderSlice(start, length, readerIndex int) ReaderSlice {
	return ReaderSlice{
		Start:       start,
		Length:      length,
		ReaderIndex: readerIndex,
	}
}
