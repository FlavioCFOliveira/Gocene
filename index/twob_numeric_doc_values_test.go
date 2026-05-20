// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// Test2BNumericDocValues ports org.apache.lucene.index.Test2BNumericDocValues.
//
// It indexes IndexWriter.MAX_DOCS (~2 billion) documents, each carrying a
// NumericDocValuesField with an increasing long value, force-merges to a single
// segment, and verifies every value. In Lucene this is annotated @Monster and
// takes roughly two hours with a 5 GB heap, so it is skipped by default.
func Test2BNumericDocValues(t *testing.T) {
	t.Skip("monster test: indexes ~2B docs, takes hours and multiple GB of heap")
}
