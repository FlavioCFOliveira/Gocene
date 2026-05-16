// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// DataAccessHint is the Go port of org.apache.lucene.store.DataAccessHint.
//
// It hints at the data access pattern likely to be used when reading a file.
type DataAccessHint uint8

const (
	// DataAccessRandom indicates the access pattern is completely random.
	DataAccessRandom DataAccessHint = iota
	// DataAccessSequential indicates the access pattern is only sequential
	// (forwards-only).
	DataAccessSequential
)

// fileOpenHint satisfies the FileOpenHint marker interface.
func (DataAccessHint) fileOpenHint() {}

// String returns the Lucene-equivalent constant name for the hint.
func (h DataAccessHint) String() string {
	switch h {
	case DataAccessRandom:
		return "RANDOM"
	case DataAccessSequential:
		return "SEQUENTIAL"
	default:
		return "UNKNOWN"
	}
}
