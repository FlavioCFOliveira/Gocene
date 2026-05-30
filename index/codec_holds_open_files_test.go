// Test file: codec_holds_open_files_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestCodecHoldsOpenFiles.java
// Purpose: Verifies that a codec keeps its backing files open: an already-open
//          reader must remain fully usable even after every index file has been
//          deleted from the directory.
//
// Port note (Sprint 55, option c):
//   The Lucene original opens an NRT reader via RandomIndexWriter.getReader(),
//   commits and closes the writer, deletes every file in the directory, then
//   calls TestUtil.checkReader on each leaf to prove the reader still works
//   off its open file handles.
//
//   This port is skipped: the verifiable core depends on two infrastructure
//   pieces Gocene does not yet provide.
//     1. IndexWriter.GetReader (the near-real-time reader): there is no API to
//        obtain a reader that holds the segment files open across deletion.
//        OpenDirectoryReader re-resolves the directory instead of retaining
//        open handles, so the "files deleted, reader survives" invariant
//        cannot be set up.
//     2. TestUtil.checkReader: the per-leaf integrity sweep (live docs, field
//        infos, norms, postings, stored fields, term vectors, doc values,
//        points) has no Gocene equivalent, and OpenDirectoryReader builds
//        segment readers with nil core readers, so leaf Terms/Postings access
//        fails for an infrastructure reason rather than a real defect.
//   Unskip once a near-real-time GetReader and a checkReader-style leaf
//   verifier are available.

package index_test

import (
	"testing"
)

// TestCodecHoldsOpenFiles indexes a batch of documents, opens a reader, deletes
// every file in the directory, and asserts the reader still works.
func TestCodecHoldsOpenFiles(t *testing.T) {
	t.Fatal("blocked: no IndexWriter.GetReader (NRT reader) and no TestUtil.checkReader equivalent yet")
}
