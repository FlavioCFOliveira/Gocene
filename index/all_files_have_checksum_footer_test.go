// Test file: all_files_have_checksum_footer_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestAllFilesHaveChecksumFooter.java
// Purpose: Verifies that a plain default index puts CRC32 footers in all files.
//
// Port note (Sprint 55, option c):
//   The Lucene original drives the index through RandomIndexWriter + LineFileDocs
//   and walks SegmentInfos.readLatestCommit, descending into compound files via
//   the codec's CompoundFormat. Gocene exposes none of those helpers yet
//   (no RandomIndexWriter, no LineFileDocs, no SegmentInfos.readLatestCommit,
//   no SegmentInfo.getUseCompoundFile). This port keeps the verifiable core:
//   build a real on-disk index with IndexWriter, then checksum every file of
//   every segment via codecs.ChecksumEntireFile. The compound-file descent is
//   omitted because the supporting API is not yet present.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesHaveChecksumFooter builds a small index and asserts that every
// file referenced by the committed segments carries a valid CRC32 footer.
func TestAllFilesHaveChecksumFooter(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(nil)
	config.SetMaxBufferedDocs(2)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		field, err := document.NewTextField("body", "the quick brown fox "+string(rune('0'+i%10)), true)
		if err != nil {
			t.Fatalf("Failed to create text field: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
		if i%7 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Failed to commit at doc %d: %v", i, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	checkFooters(t, dir)
}

// checkFooters walks the latest commit and verifies the footer of the
// segments file and of every file referenced by each segment commit.
func checkFooters(t *testing.T, dir store.Directory) {
	t.Helper()

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}

	checkFooter(t, dir, sis.GetLastFileName())

	for _, sci := range sis.List() {
		for _, file := range sci.GetFiles() {
			checkFooter(t, dir, file)
		}
	}
}

// checkFooter opens a single file and validates its CRC32 footer.
func checkFooter(t *testing.T, dir store.Directory, file string) {
	t.Helper()

	in, err := dir.OpenInput(file, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput %q: %v", file, err)
	}
	defer in.Close()

	if _, err := codecs.ChecksumEntireFile(in); err != nil {
		t.Errorf("ChecksumEntireFile %q: %v", file, err)
	}
}
