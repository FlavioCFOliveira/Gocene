// Package crossengine provides helpers for tests that verify Gocene can
// read binary index files written by Apache Lucene 10.4.0 (Java).
package crossengine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// luceneMagic is the CodecUtil magic number written at the start of every
// Lucene index file (org.apache.lucene.codecs.CodecUtil.MAGIC = 0x3fd76c17).
const luceneMagic = uint32(0x3fd76c17)

// LuceneMagicBytes is the 4-byte header expected at offset 0 of index files.
var LuceneMagicBytes = [4]byte{0x3f, 0xd7, 0x6c, 0x17}

// FixturesDir returns the absolute path to testdata/lucene-10.4.0-fixtures.
// It locates the fixtures relative to this source file so the path is valid
// regardless of the working directory when tests are invoked.
func FixturesDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = .../internal/crossengine/crossengine.go
	// repo root = two levels up
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Clean(filepath.Join(repoRoot, "testdata", "lucene-10.4.0-fixtures"))
}

// SkipIfNoFixtures calls t.Skip when the fixtures directory is absent or empty.
// Call this at the start of any test that requires the pre-generated fixtures.
func SkipIfNoFixtures(t *testing.T) {
	t.Helper()
	dir := FixturesDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) || (err == nil && len(entries) == 0) {
		t.Skipf("lucene-10.4.0 fixtures not found at %s — run tools/fixture-gen/run.sh to generate", dir)
	}
	if err != nil {
		t.Skipf("cannot access fixtures dir %s: %v", dir, err)
	}
}

// OpenFixturesDir opens the fixtures directory as a read-only MMapDirectory
// and registers t.Cleanup to close it. The test is skipped if fixtures are absent.
func OpenFixturesDir(t *testing.T) store.Directory {
	t.Helper()
	SkipIfNoFixtures(t)
	d, err := store.NewMMapDirectory(FixturesDir())
	if err != nil {
		t.Fatalf("OpenFixturesDir: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// HasMagic reports whether the first 4 bytes of data match the Lucene magic.
func HasMagic(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return data[0] == LuceneMagicBytes[0] &&
		data[1] == LuceneMagicBytes[1] &&
		data[2] == LuceneMagicBytes[2] &&
		data[3] == LuceneMagicBytes[3]
}
