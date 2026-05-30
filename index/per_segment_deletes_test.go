// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestPerSegmentDeletes ports org.apache.lucene.index.TestPerSegmentDeletes.
//
// The Lucene test installs a custom RangeMergePolicy, buffers documents across
// several commits, deletes terms, then drives writer.maybeMerge() to verify
// that per-segment deletes are applied during a merge (id:2 disappears once
// segments 0 and 1 are merged).
//
// It is skipped because the required IndexWriter surface is not yet ported:
//   - IndexWriter.MaybeMerge
//   - IndexWriter.HasChangesInRam
//   - DirectoryReader.open(IndexWriter) NRT factory
//   - the DocHelper.createDocument test fixture
func TestPerSegmentDeletes(t *testing.T) {
	t.Fatal("deferred: IndexWriter.MaybeMerge, HasChangesInRam, NRT DirectoryReader-from-writer, and DocHelper.createDocument not yet ported")
}
