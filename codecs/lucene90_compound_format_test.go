// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Mirrors selected cases from
// org.apache.lucene.codecs.lucene90.TestLucene90CompoundFormat (Lucene 10.4.0).
// The existing codecs/compound_format_test.go is intentionally left as-is
// (all tests are t.Skip until a richer harness is wired up). This file
// drives the round-trip and metadata paths that exist on the present API.

package codecs_test

import (
	"bytes"
	"crypto/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90CompoundFormat_RoundTripSingleFile writes a single small
// embedded file into a compound segment and reads it back via the
// CompoundDirectory, asserting byte equality.
func TestLucene90CompoundFormat_RoundTripSingleFile(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, payload := writeSegmentWithFiles(t, dir, map[string][]byte{
		"_0.tmp": []byte("hello lucene90 compound format"),
	})
	_ = payload

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	files, err := reader.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("ListAll: got %d files want 1: %v", len(files), files)
	}

	got, err := readFileFromCompound(reader, files[0])
	if err != nil {
		t.Fatalf("readFileFromCompound: %v", err)
	}
	want := payload[stripPrefix(t, files[0])]
	if !bytes.Equal(got, want) {
		t.Fatalf("payload mismatch: got %x want %x", got, want)
	}
}

// TestLucene90CompoundFormat_RoundTripManyFiles writes several files with
// varying sizes into a compound segment, then reads each back and
// verifies byte equality. Exercises the size-based ordering and alignment
// padding.
func TestLucene90CompoundFormat_RoundTripManyFiles(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	files := map[string][]byte{}
	sizes := []int{1, 16, 200, 2048, 4096 * 7}
	for i, sz := range sizes {
		buf := make([]byte, sz)
		_, _ = rand.Read(buf)
		files[segmentFileName(i)] = buf
	}

	si, payloads := writeSegmentWithFiles(t, dir, files)
	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	got, err := reader.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(got) != len(sizes) {
		t.Fatalf("ListAll: got %d files want %d", len(got), len(sizes))
	}
	for _, name := range got {
		body, err := readFileFromCompound(reader, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		want := payloads[stripPrefix(t, name)]
		if !bytes.Equal(body, want) {
			t.Fatalf("body mismatch for %s: got %d bytes want %d bytes", name, len(body), len(want))
		}
	}
}

// TestLucene90CompoundFormat_CompoundDirectoryIsReadOnly checks the
// CompoundDirectory rejects write operations.
func TestLucene90CompoundFormat_CompoundDirectoryIsReadOnly(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{"_0.tmp": []byte("x")})
	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	if _, err := reader.CreateOutput("x", store.IOContext{Context: store.ContextWrite}); err == nil {
		t.Error("CreateOutput should error on read-only compound dir")
	}
	if err := reader.DeleteFile("x"); err == nil {
		t.Error("DeleteFile should error on read-only compound dir")
	}
}

// TestLucene90CompoundFormat_CheckIntegrity verifies the CheckIntegrity
// hook validates per-file footers.
func TestLucene90CompoundFormat_CheckIntegrity(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{
		"_0.foo": []byte("alpha"),
		"_0.bar": []byte("beta"),
	})
	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	if err := reader.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// segmentFileName builds a deterministic file name that looks like a
// per-segment artifact (starts with "_0", takes a random ASCII suffix).
func segmentFileName(i int) string {
	switch i {
	case 0:
		return "_0.alpha"
	case 1:
		return "_0.beta"
	case 2:
		return "_0.gamma"
	case 3:
		return "_0.delta"
	default:
		return "_0.eps"
	}
}

// writeSegmentWithFiles materialises each (name, body) pair as a file in
// dir wrapped in a valid Lucene10 index header + footer, returns a
// SegmentInfo whose Files() lists them all, and returns the original
// (raw) payloads keyed by their stripped suffix (after _segName) so test
// callers can compare bytes.
func writeSegmentWithFiles(t *testing.T, dir store.Directory, files map[string][]byte) (*index.SegmentInfo, map[string][]byte) {
	t.Helper()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	si := index.NewSegmentInfo("_0", 0, dir)
	if err := si.SetID(id); err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	payloads := make(map[string][]byte, len(files))
	for _, name := range names {
		body := files[name]
		raw, err := dir.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
		if err != nil {
			t.Fatalf("CreateOutput %s: %v", name, err)
		}
		// Wrap in a checksum stream so WriteFooter has a checksum source.
		out := store.NewChecksumIndexOutput(raw)
		// Stamp a legitimate Lucene index header (codec name "Test") so
		// CheckIntegrity / footer paths agree with the source bytes.
		if err := codecs.WriteIndexHeader(out, "Test", 0, id, ""); err != nil {
			t.Fatalf("WriteIndexHeader %s: %v", name, err)
		}
		if err := out.WriteBytes(body); err != nil {
			t.Fatalf("WriteBytes %s: %v", name, err)
		}
		if err := codecs.WriteFooter(out); err != nil {
			t.Fatalf("WriteFooter %s: %v", name, err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close %s: %v", name, err)
		}
		payloads[stripPrefixInName(name)] = body
	}
	si.SetFiles(names)
	return si, payloads
}

// readFileFromCompound reads the entire (header + body + footer) payload
// of the named file from a compound directory.
func readFileFromCompound(dir codecs.CompoundDirectory, name string) ([]byte, error) {
	in, err := dir.OpenInput(name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, err
	}
	defer in.Close()
	n := in.Length()
	buf := make([]byte, n)
	if err := in.ReadBytes(buf); err != nil {
		return nil, err
	}
	// Strip the index header (length is variable per codec name) and the
	// footer (16 bytes) so the caller can compare against the original
	// payload bytes only.
	header := int64(codecs.IndexHeaderLength("Test", ""))
	footer := int64(codecs.FooterLength())
	if n < header+footer {
		return nil, nil
	}
	return buf[header : n-footer], nil
}

// stripPrefix returns the suffix of name after the leading "_segName"
// component, matching IndexFileNames.stripSegmentName.
func stripPrefix(t *testing.T, name string) string {
	t.Helper()
	return stripPrefixInName(name)
}

func stripPrefixInName(name string) string {
	if len(name) < 2 || name[0] != '_' {
		return name
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if c == '_' || c == '.' {
			return name[i:]
		}
	}
	return name
}
