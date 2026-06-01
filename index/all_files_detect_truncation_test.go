// Test file: all_files_detect_truncation_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestAllFilesDetectTruncation.java
// Purpose: Verifies that a plain default index detects file truncation early,
//          on opening a reader and via CheckIndex.
//
// Port note (Sprint 55, option c):
//   The Lucene original builds an index with RandomIndexWriter, then for each
//   file copies the directory while truncating one file by 1..100 bytes, and
//   asserts that DirectoryReader.open and CheckIndex both throw. Truncation is
//   only reliably detectable because every file carries a CRC32 codec footer
//   (the original skips a file when CodecUtil.checkFooter still passes after
//   truncation). Gocene's index.WriteSegmentInfos does not yet emit a CRC32
//   footer (see TestAllFilesHaveChecksumFooter), so truncated files cannot be
//   reliably distinguished and the truncation-detection contract cannot be
//   exercised. The port keeps the full structure; unskip once footer writing
//   lands in the segment-infos writer.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesDetectTruncation builds a small index, then truncates each file
// in turn and asserts that opening a reader and running CheckIndex both fail.
func TestAllFilesDetectTruncation(t *testing.T) {
	// Blocked: truncation detection requires per-file CRC32 checksum verification
	// on open/check. While WriteSegmentInfos now writes footers, OpenDirectoryReader
	// and CheckIndex do not yet verify the CRC32 checksum of individual files, so
	// truncation is not detected. Unskip once the per-file checksum verification
	// is implemented in the reader/check path.
	t.Fatal("blocked: truncation detection requires per-file CRC32 verification on open/check, not yet implemented")

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

	checkTruncation(t, dir)
}

// checkTruncation truncates each file of the index in turn and verifies that
// the corruption is detected.
func checkTruncation(t *testing.T, dir store.Directory) {
	t.Helper()

	names, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	for _, name := range names {
		if name == "write.lock" {
			continue
		}
		truncateOneFile(t, dir, name)
	}
}

// copyFileBytes reads the first n bytes of src from dir and writes them to a
// new file dst in dirCopy. With n == full length it is a faithful copy; with
// n < length it produces a truncated copy.
func copyFileBytes(t *testing.T, dir, dirCopy store.Directory, src, dst string, n int64) {
	t.Helper()

	in, err := dir.OpenInput(src, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput %q: %v", src, err)
	}
	defer in.Close()

	buf, err := in.ReadBytesN(int(n))
	if err != nil {
		t.Fatalf("ReadBytesN %q: %v", src, err)
	}

	out, err := dirCopy.CreateOutput(dst, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput %q: %v", dst, err)
	}
	if err := out.WriteBytesN(buf, len(buf)); err != nil {
		t.Fatalf("WriteBytesN %q: %v", dst, err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output %q: %v", dst, err)
	}
}

// truncateOneFile copies the index into a fresh directory, truncating the
// victim file, and asserts that opening a reader and running CheckIndex both
// fail. Files whose codec footer still validates after truncation are skipped,
// matching the Lucene original.
func truncateOneFile(t *testing.T, dir store.Directory, victim string) {
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

	lostBytes := int64(1)
	if victimLength > 1 {
		lostBytes = min(int64(100), victimLength) / 2
		if lostBytes < 1 {
			lostBytes = 1
		}
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

		// If the codec footer still validates after truncation, the Lucene
		// original skips this file: the corruption is undetectable.
		raw, err := dir.OpenInput(name, store.IOContextReadOnce)
		if err != nil {
			t.Fatalf("OpenInput %q: %v", name, err)
		}
		csIn := store.NewChecksumIndexInput(raw)
		_, footerErr := codecs.CheckFooter(csIn)
		_ = raw.Close()
		if footerErr == nil {
			return
		}

		copyFileBytes(t, dir, dirCopy, name, name, victimLength-lostBytes)
	}

	// There must be an error opening the reader; the type is unspecified.
	if reader, err := index.OpenDirectoryReader(dirCopy); err == nil {
		_ = reader.Close()
		t.Errorf("truncation of %q not detected on opening a reader", victim)
	}

	// CheckIndex must also fail.
	ci, err := index.NewCheckIndex(dirCopy)
	if err != nil {
		// A failure to even construct CheckIndex is an acceptable detection.
		return
	}
	if status, err := ci.CheckIndex(); err == nil && status != nil && status.Clean {
		t.Errorf("truncation of %q not detected by CheckIndex", victim)
	}
}
