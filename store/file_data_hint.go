// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "github.com/FlavioCFOliveira/Gocene/spi"

// FileDataHint is the Go port of org.apache.lucene.store.FileDataHint.
//
// It hints at the type of data stored in the file. Lucene declares a single
// enum; the type lives in package spi (which breaks the store/index import
// cycle) and is aliased here so that there is exactly one Go type and every
// value satisfies FileOpenHint.
type FileDataHint = spi.FileDataHint

const (
	// FileDataPostings indicates the file contains postings data
	// (FileDataHint.POSTINGS).
	FileDataPostings = spi.FileDataPostings
	// FileDataKNNVectors indicates the file contains vector data for kNN
	// search (FileDataHint.KNN_VECTORS).
	FileDataKNNVectors = spi.FileDataKNNVectors
)
