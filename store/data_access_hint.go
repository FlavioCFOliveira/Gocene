// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "github.com/FlavioCFOliveira/Gocene/spi"

// DataAccessHint is the Go port of org.apache.lucene.store.DataAccessHint.
//
// It hints at the data access pattern likely to be used when reading a file.
// The type lives in package spi (which breaks the store/index import cycle)
// and is aliased here so that there is exactly one Go type and every value
// satisfies FileOpenHint.
type DataAccessHint = spi.DataAccessHint

const (
	// DataAccessRandom indicates the access pattern is completely random
	// (DataAccessHint.RANDOM).
	DataAccessRandom = spi.DataAccessRandom
	// DataAccessSequential indicates the access pattern is only sequential
	// (DataAccessHint.SEQUENTIAL).
	DataAccessSequential = spi.DataAccessSequential
)
