// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "io"

// FilterDirectoryReader is a DirectoryReader that wraps another DirectoryReader.
//
// This is the Go port of Lucene's org.apache.lucene.index.FilterDirectoryReader.
type FilterDirectoryReader struct {
	*DirectoryReader
	in *DirectoryReader
}

// NewFilterDirectoryReader creates a new FilterDirectoryReader wrapping the given reader.
func NewFilterDirectoryReader(in *DirectoryReader) *FilterDirectoryReader {
	return &FilterDirectoryReader{
		DirectoryReader: in,
		in:              in,
	}
}

// GetDelegate returns the wrapped DirectoryReader.
func (r *FilterDirectoryReader) GetDelegate() *DirectoryReader {
	return r.in
}

// Close closes the wrapped reader.
func (r *FilterDirectoryReader) Close() error {
	return r.in.Close()
}

// LeafReader is a wrapper for a LeafReader.
type FilterLeafReader struct {
	*LeafReader
	in *LeafReader
}

// NewFilterLeafReader creates a new FilterLeafReader wrapping the given reader.
func NewFilterLeafReader(in *LeafReader) *FilterLeafReader {
	return &FilterLeafReader{
		LeafReader: in,
		in:         in,
	}
}

// Close closes the wrapped reader.
func (r *FilterLeafReader) Close() error {
	if closer, ok := interface{}(r.in).(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
