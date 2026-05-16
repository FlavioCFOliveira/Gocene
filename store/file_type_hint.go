// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// FileTypeHint is the Go port of org.apache.lucene.store.FileTypeHint.
//
// It hints at the type of file being opened. Note that metadata files should
// be opened with Directory.OpenChecksumInput which does not accept hints, so
// no INDEX_METADATA constant is provided.
type FileTypeHint uint8

const (
	// FileTypeIndex indicates the file contains indexes. It is small (~1% or
	// less of the data size) and generally fits in the page cache.
	FileTypeIndex FileTypeHint = iota
	// FileTypeData indicates the file contains field data.
	FileTypeData
)

// fileOpenHint satisfies the FileOpenHint marker interface.
func (FileTypeHint) fileOpenHint() {}

// String returns the Lucene-equivalent constant name for the hint.
func (h FileTypeHint) String() string {
	switch h {
	case FileTypeIndex:
		return "INDEX"
	case FileTypeData:
		return "DATA"
	default:
		return "UNKNOWN"
	}
}
