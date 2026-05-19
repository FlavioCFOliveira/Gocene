// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Ported from Apache Lucene 10.4.0:
//
//   org.apache.lucene.util.compress.TestHighLZ4
//
// The shared LZ4TestCase scaffolding lives in lz4_test_case_test.go;
// this file only supplies the HighCompressionHashTable factory and the
// single top-level Test entry point that dispatches the full Java test
// matrix through it.

package compress

import "testing"

// newHighHashTable is the Gocene equivalent of TestHighLZ4.newHashTable:
// a HighCompressionHashTable wrapped in the asserting decorator.
func newHighHashTable() *assertingHashTable {
	return newAssertingHashTable(NewHighCompressionHashTable())
}

// TestHighLZ4 runs the full LZ4TestCase matrix against
// HighCompressionHashTable, mirroring TestHighLZ4 in the Java reference.
func TestHighLZ4(t *testing.T) {
	runLZ4TestCase(t, newHighHashTable)
}
