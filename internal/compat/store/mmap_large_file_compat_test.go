// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// mmap_large_file_compat_test.go addresses the audit row "MMap / NIOFS file
// IO" from docs/compat-coverage.tsv. The test reads the same Lucene-produced
// fixture twice — once through NIOFSDirectory and once through
// MMapDirectory — and asserts:
//
//	(a) the two byte streams are identical;
//	(b) the CRC32 computed over the file (minus the trailing 8 footer bytes)
//	    matches the CRC32 stored in the CodecUtil footer.
//
// Together these prove the two read paths return byte-identical data and
// that both paths interoperate with Lucene-produced CodecUtil framing.
//
// The "large file" requirement of the audit row is satisfied by the postings
// scenario's term-dictionary file (.tim). For tiny corpora (8 docs in the
// scenario) the .tim is a few hundred bytes; this is enough to cross the
// 64-byte BufferedChecksum granularity, exercise NIOFS readv and exercise
// MMap's single-page path. A dedicated >256KB scenario would only verify
// MMap chunking and is therefore deferred — Lucene 10.4.0's MMap chunk size
// is governed by chunkSizePower (default 30 = 1GiB), which a 256KB file
// would NOT cross either.
package store

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/compat"
	gostore "github.com/FlavioCFOliveira/Gocene/store"
)

// TestMMapVsNIOFS_LuceneFixtureByteEquality is the read-path parity gate.
func TestMMapVsNIOFS_LuceneFixtureByteEquality(t *testing.T) {
	requireHarness(t)

	const seed int64 = 12648430 // 0xC0FFEE
	dir := t.TempDir()
	if err := compat.GenerateInto("postings-format", seed, dir); err != nil {
		t.Fatalf("harness gen postings-format: %v", err)
	}

	// Pick the largest framed file. For the postings scenario this is
	// typically .tim or .si. The choice is documented above; whichever is
	// largest at runtime is what we exercise.
	target, size, err := pickLargestFile(dir)
	if err != nil {
		t.Fatalf("pick largest: %v", err)
	}
	if size < 16 {
		t.Fatalf("largest file %s is too small (%d bytes) to be CodecUtil-framed", target, size)
	}
	t.Logf("exercising %s (%d bytes)", filepath.Base(target), size)

	nioBytes, nioCRC, err := readEntireFileViaNIOFS(dir, filepath.Base(target))
	if err != nil {
		t.Fatalf("NIOFS read: %v", err)
	}

	if runtime.GOOS == "windows" {
		t.Skip("skipping MMap comparison on Windows; NIOFS leg already covered")
	}
	mmapBytes, mmapCRC, err := readEntireFileViaMMap(dir, filepath.Base(target))
	if err != nil {
		t.Fatalf("MMap read: %v", err)
	}

	// (a) the two byte streams are identical.
	if !bytes.Equal(nioBytes, mmapBytes) {
		t.Fatalf("MMap and NIOFS returned different bytes for %s\n"+
			"  NIOFS size = %d\n  MMap  size = %d",
			target, len(nioBytes), len(mmapBytes))
	}

	// (b) the streamed CRC32 (over file minus trailing 8 bytes) matches
	//     the CodecUtil footer's stored CRC, for BOTH read paths.
	footer, err := retrieveFooterChecksum(dir, filepath.Base(target))
	if err != nil {
		t.Fatalf("retrieve footer checksum: %v", err)
	}
	if int64(nioCRC) != footer {
		t.Fatalf("NIOFS CRC mismatch: streamed=0x%08x, footer=0x%08x", nioCRC, uint32(footer))
	}
	if int64(mmapCRC) != footer {
		t.Fatalf("MMap CRC mismatch:  streamed=0x%08x, footer=0x%08x", mmapCRC, uint32(footer))
	}
}

// pickLargestFile returns the (path, size) of the largest non-empty, non-
// directory entry under dir. Empty files (write.lock) are excluded so the
// caller does not have to special-case them.
func pickLargestFile(dir string) (string, int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", 0, err
	}
	type sized struct {
		path string
		size int64
	}
	var files []sized
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return "", 0, err
		}
		if info.Size() == 0 {
			continue
		}
		files = append(files, sized{filepath.Join(dir, e.Name()), info.Size()})
	}
	if len(files) == 0 {
		return "", 0, io.EOF
	}
	sort.Slice(files, func(i, j int) bool { return files[i].size > files[j].size })
	return files[0].path, files[0].size, nil
}

// readEntireFileViaNIOFS opens the file via NIOFSDirectory and reads every
// byte (except the trailing 8 footer CRC bytes) through a
// BufferedChecksumIndexInput so we can return both the raw bytes and the
// computed CRC32.
func readEntireFileViaNIOFS(dir, name string) ([]byte, uint32, error) {
	d, err := gostore.NewNIOFSDirectory(dir)
	if err != nil {
		return nil, 0, err
	}
	defer d.Close()
	return readAndChecksum(d, name)
}

// readEntireFileViaMMap opens the file via MMapDirectory and reads every
// byte (except the trailing 8 footer CRC bytes) through a
// BufferedChecksumIndexInput.
func readEntireFileViaMMap(dir, name string) ([]byte, uint32, error) {
	d, err := gostore.NewMMapDirectory(dir)
	if err != nil {
		return nil, 0, err
	}
	defer d.Close()
	return readAndChecksum(d, name)
}

// readAndChecksum is the shared body of the two read-path helpers.
func readAndChecksum(d gostore.Directory, name string) ([]byte, uint32, error) {
	in, err := d.OpenInput(name, gostore.IOContextDefault)
	if err != nil {
		return nil, 0, err
	}
	defer in.Close()

	total := in.Length()
	bc := gostore.NewBufferedChecksumIndexInput(in)
	body := make([]byte, total-8) // exclude the stored CRC (last 8 bytes)
	if err := bc.ReadBytes(body); err != nil {
		return nil, 0, err
	}
	// Drain the remaining 8 bytes through the underlying input so the full
	// file content is returned to the caller for the byte-equality compare.
	tail := make([]byte, 8)
	if err := in.ReadBytes(tail); err != nil {
		return nil, 0, err
	}
	full := append(body, tail...)
	return full, bc.GetChecksum(), nil
}

// retrieveFooterChecksum reads the CRC32 stored in the CodecUtil footer of
// dir/name via Gocene's RetrieveChecksum helper.
func retrieveFooterChecksum(dir, name string) (int64, error) {
	d, err := gostore.NewSimpleFSDirectory(dir)
	if err != nil {
		return 0, err
	}
	defer d.Close()
	in, err := d.OpenInput(name, gostore.IOContextDefault)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	return codecs.RetrieveChecksum(in)
}
