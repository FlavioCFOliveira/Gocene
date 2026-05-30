// Test file: all_files_check_index_header_test.go
// Source: lucene/core/src/test/org/apache/lucene/index/TestAllFilesCheckIndexHeader.java
// Purpose: Verifies that a plain default detects broken index headers early
//          (i.e. when a reader is opened over the corrupted index).
//
// Port note (Sprint 55, option c):
//   The Lucene original drives the index through RandomIndexWriter + LineFileDocs,
//   then for every file rebuilds the directory into a fresh ByteBuffersDirectory,
//   randomizing the first 1..100 bytes of one "victim" file while keeping its
//   length, and asserts DirectoryReader.open throws CorruptIndexException /
//   EOFException / IndexFormatTooOldException.
//
//   This port keeps the verifiable core: build a real on-disk index with
//   IndexWriter, then for each file copy every file into a fresh directory,
//   corrupting the victim's leading header bytes, and assert OpenDirectoryReader
//   fails. RandomIndexWriter, LineFileDocs, MockDirectoryWrapper and the
//   IOContext.READONCE copy helpers (Directory.copyFrom, IndexOutput.copyBytes)
//   are not present in Gocene, so file copies are done explicitly via
//   ReadBytes/WriteBytes.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesCheckIndexHeader builds a small index, then for every file
// rebuilds the index into a fresh directory with that file's leading header
// bytes corrupted, and asserts that opening a reader fails.
func TestAllFilesCheckIndexHeader(t *testing.T) {
	// Skipped (Sprint 55, option c): blocked by the same infra gap as the
	// sibling all_files_have_checksum_footer_test.go. OpenDirectoryReader
	// does not yet validate per-file codec headers eagerly on open (core
	// readers are loaded lazily and the segments file carries no CRC32
	// footer), so a corrupted header is not detected at open time. Unskip
	// once eager header validation lands in the directory reader.
	t.Fatal("blocked: OpenDirectoryReader does not validate codec headers on open yet")

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

	checkIndexHeader(t, dir)
}

// checkIndexHeader breaks the header of each file in turn and verifies the
// corruption is detected when a reader is opened.
func checkIndexHeader(t *testing.T, dir store.Directory) {
	t.Helper()

	names, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, name := range names {
		// Skip the write lock; Gocene exposes no exported constant for it,
		// so the literal "write.lock" is used (see index/check_index.go).
		if name == "write.lock" {
			continue
		}
		checkOneFile(t, dir, name)
	}
}

// checkOneFile rebuilds the index into a fresh directory with the leading
// header bytes of victim replaced by deterministic non-matching bytes, then
// asserts that opening a reader over the corrupted index fails.
func checkOneFile(t *testing.T, dir store.Directory, victim string) {
	t.Helper()

	victimLength, err := dir.FileLength(victim)
	if err != nil {
		t.Fatalf("FileLength %q: %v", victim, err)
	}
	if victimLength <= 0 {
		t.Fatalf("victim %q has non-positive length %d", victim, victimLength)
	}

	wrongBytes := int64(100)
	if victimLength < wrongBytes {
		wrongBytes = victimLength
	}

	dirCopy := store.NewByteBuffersDirectory()
	defer dirCopy.Close()

	names, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, name := range names {
		if name == victim {
			copyCorrupted(t, dir, dirCopy, name, wrongBytes, victimLength)
		} else {
			copyFile(t, dir, dirCopy, name)
		}
	}

	reader, err := index.OpenDirectoryReader(dirCopy)
	if err == nil {
		_ = reader.Close()
		t.Errorf("corruption of file %q was not detected when opening a reader", victim)
	}
}

// copyFile copies name from src to dst byte-for-byte.
func copyFile(t *testing.T, src, dst store.Directory, name string) {
	t.Helper()

	in, err := src.OpenInput(name, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput %q: %v", name, err)
	}
	defer in.Close()

	length, err := src.FileLength(name)
	if err != nil {
		t.Fatalf("FileLength %q: %v", name, err)
	}

	out, err := dst.CreateOutput(name, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput %q: %v", name, err)
	}
	defer out.Close()

	buf := make([]byte, length)
	if err := in.ReadBytes(buf); err != nil {
		t.Fatalf("ReadBytes %q: %v", name, err)
	}
	if err := out.WriteBytes(buf); err != nil {
		t.Fatalf("WriteBytes %q: %v", name, err)
	}
}

// copyCorrupted copies name from src to dst, replacing the first wrongBytes
// with deterministic bytes that are guaranteed to differ from the originals
// while preserving the total file length.
func copyCorrupted(t *testing.T, src, dst store.Directory, name string, wrongBytes, totalLength int64) {
	t.Helper()

	in, err := src.OpenInput(name, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput %q: %v", name, err)
	}
	defer in.Close()

	original := make([]byte, totalLength)
	if err := in.ReadBytes(original); err != nil {
		t.Fatalf("ReadBytes %q: %v", name, err)
	}

	// Flip every bit of the leading header bytes: the result is guaranteed
	// to differ from the original at every position.
	corrupted := make([]byte, totalLength)
	copy(corrupted, original)
	for i := int64(0); i < wrongBytes; i++ {
		corrupted[i] = ^original[i]
	}

	out, err := dst.CreateOutput(name, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput %q: %v", name, err)
	}
	defer out.Close()

	if err := out.WriteBytes(corrupted); err != nil {
		t.Fatalf("WriteBytes %q: %v", name, err)
	}
}
