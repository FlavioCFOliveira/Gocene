//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"io"
)

// AssertingDirectoryReader is a DirectoryReader that wraps all its subreaders with AssertingLeafReader.
//
// This is the Go port of Lucene's org.apache.lucene.tests.index.AssertingDirectoryReader.
type AssertingDirectoryReader struct {
	*DirectoryReader
}

// NewAssertingDirectoryReader creates a new AssertingDirectoryReader wrapping the given reader.
func NewAssertingDirectoryReader(in *DirectoryReader) *AssertingDirectoryReader {
	return &AssertingDirectoryReader{
		DirectoryReader: in,
	}
}

// Leaves returns the leaf reader contexts, but with the readers wrapped in AssertingLeafReader.
func (r *AssertingDirectoryReader) Leaves() ([]*LeafReaderContext, error) {
	leaves, err := r.DirectoryReader.Leaves()
	if err != nil {
		return nil, err
	}

	wrappedLeaves := make([]*LeafReaderContext, len(leaves))
	for i, lrc := range leaves {
		// Copy the context and wrap the reader to avoid mutating the original
		newLrc := *lrc
		newLrc.reader = NewAssertingLeafReader(lrc.reader)
		wrappedLeaves[i] = &newLrc
	}

	return wrappedLeaves, nil
}

// Close closes the wrapped reader.
func (r *AssertingDirectoryReader) Close() error {
	return r.DirectoryReader.Close()
}

// Ensure AssertingDirectoryReader implements IndexReaderInterface
var _ IndexReaderInterface = (*AssertingDirectoryReader)(nil)
