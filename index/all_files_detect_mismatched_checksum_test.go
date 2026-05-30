// Test file: all_files_detect_mismatched_checksum_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestAllFilesDetectMismatchedChecksum.java
// Purpose: Verifies that the default codec detects mismatched checksums, either
//          on opening a reader or via CheckIntegrity.
//
// Port note (Sprint 55, option c):
//   The Lucene original builds an index with RandomIndexWriter, then for each
//   file copies the directory while flipping a single byte inside the region
//   that the codec footer protects (offset >= victimLength - footerLength).
//   Flipping a body byte makes the stored CRC32 disagree with the recomputed
//   one; flipping a footer byte corrupts the footer itself. Either way the
//   codec must report a CorruptIndexException at open or CheckIntegrity time.
//   Detection therefore relies on every file carrying a CRC32 codec footer.
//   Gocene's index.WriteSegmentInfos does not yet emit a CRC32 footer (see
//   TestAllFilesHaveChecksumFooter), so a flipped byte cannot be reliably
//   distinguished from valid data and the mismatched-checksum contract cannot
//   be exercised. The port keeps the full structure; unskip once footer
//   writing lands in the segment-infos writer.

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesDetectMismatchedChecksum builds a small index, then for each file
// in turn flips one byte inside the footer-protected region and asserts that
// the corruption is detected when opening a reader or running CheckIndex.
func TestAllFilesDetectMismatchedChecksum(t *testing.T) {
	// Skipped (Sprint 55, option c): blocked by a pre-existing infra gap.
	// index.WriteSegmentInfos does not emit a CRC32 footer yet, so a flipped
	// byte inside a file cannot be reliably detected (without a footer there
	// is no stored checksum to disagree with). Unskip once footer writing
	// lands in the segment-infos writer.
	t.Fatal("blocked: mismatched-checksum detection requires CRC32 footers, not yet written by WriteSegmentInfos")

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
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	checkMismatchedChecksum(t, dir)
}

// checkMismatchedChecksum corrupts each file of the index in turn and verifies
// that the corruption is detected.
func checkMismatchedChecksum(t *testing.T, dir store.Directory) {
	t.Helper()

	names, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	for _, name := range names {
		if name == "write.lock" {
			continue
		}
		corruptOneFile(t, dir, name)
	}
}

// corruptOneFile copies the index into a fresh directory, flipping a single
// byte of the victim file inside the footer-protected region, and asserts that
// opening a reader and running CheckIndex both fail.
func corruptOneFile(t *testing.T, dir store.Directory, victim string) {
	t.Helper()

	dirCopy := store.NewByteBuffersDirectory()
	defer dirCopy.Close()

	victimLength, err := dir.FileLength(victim)
	if err != nil {
		t.Fatalf("FileLength %q: %v", victim, err)
	}
	if victimLength <= 0 {
		t.Fatalf("victim %q has non-positive length %d", victim, victimLength)
	}

	// Flip a byte somewhere in [victimLength-footerLength, victimLength-1], the
	// region covered by the CRC32 footer, so the corruption is always detected.
	footerLen := int64(codecs.FooterLength())
	lo := victimLength - footerLen
	if lo < 0 {
		lo = 0
	}
	hi := victimLength - 1
	flipOffset := lo
	if hi > lo {
		flipOffset = lo + rand.Int63n(hi-lo+1)
	}

	names, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	for _, name := range names {
		if name != victim {
			length, err := dir.FileLength(name)
			if err != nil {
				t.Fatalf("FileLength %q: %v", name, err)
			}
			copyFileBytes(t, dir, dirCopy, name, name, length)
			continue
		}
		copyCorruptedFile(t, dir, dirCopy, name, flipOffset)
	}

	// There must be an error opening the reader; the type is unspecified.
	if reader, err := index.OpenDirectoryReader(dirCopy); err == nil {
		_ = reader.Close()
		t.Errorf("mismatched checksum of %q not detected on opening a reader", victim)
	}

	// CheckIndex must also fail.
	ci, err := index.NewCheckIndex(dirCopy)
	if err != nil {
		// A failure to even construct CheckIndex is an acceptable detection.
		return
	}
	if status, err := ci.CheckIndex(); err == nil && status != nil && status.Clean {
		t.Errorf("mismatched checksum of %q not detected by CheckIndex", victim)
	}
}

// copyCorruptedFile reads src from dir and writes it to dst in dirCopy with the
// byte at flipOffset replaced by a different value, producing a file whose
// stored CRC32 footer no longer matches its contents.
func copyCorruptedFile(t *testing.T, dir, dirCopy store.Directory, name string, flipOffset int64) {
	t.Helper()

	in, err := dir.OpenInput(name, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput %q: %v", name, err)
	}
	defer in.Close()

	length, err := dir.FileLength(name)
	if err != nil {
		t.Fatalf("FileLength %q: %v", name, err)
	}

	buf, err := in.ReadBytesN(int(length))
	if err != nil {
		t.Fatalf("ReadBytesN %q: %v", name, err)
	}

	// Add a non-zero delta in [0x01, 0xFF] so the byte is guaranteed to change.
	delta := byte(rand.Intn(0xFF) + 0x01)
	buf[flipOffset] += delta

	out, err := dirCopy.CreateOutput(name, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput %q: %v", name, err)
	}
	if err := out.WriteBytesN(buf, len(buf)); err != nil {
		t.Fatalf("WriteBytesN %q: %v", name, err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output %q: %v", name, err)
	}
}
