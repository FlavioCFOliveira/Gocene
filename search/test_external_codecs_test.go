// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestExternalCodecs.java
//
// Deviation: all test methods skipped — TestExternalCodecs requires
// IndexWriter + DirectoryReader + IndexSearcher integration as well as
// external codec SPI (RAMOnly, AssertingCodec) not yet available in Gocene.

package search

import "testing"

// TestExternalCodecs_PerFieldCodec mirrors the Java testPerFieldCodec method.
// It verifies that a custom per-field PostingsFormat codec can be used
// with IndexWriter and that term queries still return correct results.
func TestExternalCodecs_PerFieldCodec(t *testing.T) {
	t.Fatal("requires complete IndexWriter+IndexSearcher integration and codec SPI (pre-existing failure in Gocene)")
}
