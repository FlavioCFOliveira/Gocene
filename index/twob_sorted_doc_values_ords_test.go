// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// Test2BSortedDocValuesOrds ports org.apache.lucene.index.Test2BSortedDocValuesOrds.
//
// It indexes IndexWriter.MAX_DOCS (~2 billion) documents, each carrying a fixed
// 4-byte SortedDocValuesField, force-merges to a single segment, and verifies
// every ordinal and looked-up term. In Lucene this is annotated @Monster and
// takes roughly six hours with a 5 GB heap, so it is skipped by default.
func Test2BSortedDocValuesOrds(t *testing.T) {
	t.Fatal("monster test: indexes ~2B docs, takes hours and multiple GB of heap")
}
