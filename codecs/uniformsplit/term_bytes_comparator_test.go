// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit_test

import "testing"

// The test below is a port of
// org.apache.lucene.codecs.uniformsplit.TestTermBytesComparator
// (Lucene 10.4.0).
//
// TestTermBytesComparator_Comparison is skipped because BlockReader.seekInBlock
// and the lineIndexInBlock field are not yet ported to the Gocene uniformsplit
// package. The test body will be filled in once that logic lands.
//
// Java test summary:
//
//	Builds a vocabulary of 11 TermBytes entries and exercises BlockReader.seekInBlock
//	with four look-up terms:
//	  - "z"       → END (all 11 entries scanned, status END)
//	  - "abacu"   → NOT_FOUND, landed at position 1 ("amiga")
//	  - "bar"     → NOT_FOUND, landed at position 4 ("bloom")
//	  - "amigas"  → NOT_FOUND, landed at position 2 ("arco")
//	  - "friendez"→ FOUND, landed at position 10
//
// The Java test uses a MockBlockReader that overrides readLineInBlock to serve
// a pre-built list instead of reading from disk, compareToMiddleAndJump to
// disable the mid-block optimization, and initializeHeader / readHeader to
// avoid file I/O. An equivalent mock would be needed in Go once the
// interface is in place.

func TestTermBytesComparator_Comparison(t *testing.T) {
	t.Skip(
		"BlockReader.seekInBlock and lineIndexInBlock are not yet ported to " +
			"the Gocene uniformsplit package; test body deferred until those " +
			"components land",
	)
}
