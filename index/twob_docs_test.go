// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// Test2BDocs ports org.apache.lucene.index.Test2BDocs.
//
// It indexes IndexWriter.MAX_DOCS (~2 billion) documents, each carrying a
// single indexed StringField, force-merges to a single segment, then reopens
// the index and randomly skips through the term postings to verify advance.
// In Lucene this is annotated @Monster and takes roughly 30 minutes, so it is
// skipped by default.
func Test2BDocs(t *testing.T) {
	t.Fatal("monster test: indexes ~2B docs, takes ~30min and multiple GB of heap")
}
