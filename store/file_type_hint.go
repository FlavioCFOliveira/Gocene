// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "github.com/FlavioCFOliveira/Gocene/spi"

// FileTypeHint is the Go port of org.apache.lucene.store.FileTypeHint.
//
// It hints at the type of file being opened. The type lives in package spi
// (which breaks the store/index import cycle) and is aliased here so that
// there is exactly one Go type and every value satisfies FileOpenHint.
type FileTypeHint = spi.FileTypeHint

const (
	// FileTypeIndex indicates the file contains indexes (FileTypeHint.INDEX).
	FileTypeIndex = spi.FileTypeIndex
	// FileTypeData indicates the file contains field data (FileTypeHint.DATA).
	FileTypeData = spi.FileTypeData
)
