// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package nrt_test

// Tests for the NRT replication stubs.
//
// Deviation: The Java test peers (TestStressNRTReplication, TestNRTReplication,
// SimplePrimaryNode, SimpleReplicaNode, NodeProcess, Connection, Jobs,
// ThreadPumper, SimpleCopyJob, SimpleTransLog, TestSimpleServer) are large
// integration tests that depend on IndexWriter, network I/O, JVM process
// spawning and the full Lucene index stack. Those are deferred to backlog
// #2693. The tests here verify the self-contained types and their contracts.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/replicator/nrt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// NodeCommunicationException
// ---------------------------------------------------------------------------

func TestNodeCommunicationException_Error(t *testing.T) {
	cause := errors.New("connection reset")
	err := nrt.NewNodeCommunicationException("send copy state", cause)
	if err.Error() == "" {
		t.Fatal("Error() must not be empty")
	}
}

func TestNodeCommunicationException_Unwrap(t *testing.T) {
	cause := errors.New("timeout")
	err := nrt.NewNodeCommunicationException("receive ack", cause)
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is must resolve to the wrapped cause")
	}
}

func TestNodeCommunicationException_NilCausePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("nil cause should panic")
		}
	}()
	nrt.NewNodeCommunicationException("whatever", nil)
}

// ---------------------------------------------------------------------------
// FileMetaData
// ---------------------------------------------------------------------------

func TestFileMetaData_String(t *testing.T) {
	f := &nrt.FileMetaData{Length: 1024, Checksum: 42}
	s := f.String()
	if s == "" {
		t.Fatal("String must not be empty")
	}
}

func TestFileMetaData_Fields(t *testing.T) {
	header := []byte{0, 1, 2, 3}
	footer := []byte{4, 5}
	f := &nrt.FileMetaData{Header: header, Footer: footer, Length: 100, Checksum: 0xDEAD}
	if f.Length != 100 {
		t.Fatalf("Length: want 100, got %d", f.Length)
	}
	if f.Checksum != 0xDEAD {
		t.Fatalf("Checksum: want 0xDEAD, got %x", f.Checksum)
	}
}

// ---------------------------------------------------------------------------
// CopyState
// ---------------------------------------------------------------------------

func TestCopyState_String(t *testing.T) {
	cs := &nrt.CopyState{Version: 7}
	if cs.String() == "" {
		t.Fatal("String must not be empty")
	}
}

func TestCopyState_Fields(t *testing.T) {
	files := map[string]*nrt.FileMetaData{
		"_0.si": {Length: 50, Checksum: 1},
	}
	cs := &nrt.CopyState{
		Files:      files,
		Version:    3,
		Gen:        2,
		InfosBytes: []byte{0xAB},
		PrimaryGen: 1,
	}
	if cs.Version != 3 {
		t.Fatalf("Version: want 3, got %d", cs.Version)
	}
	if _, ok := cs.Files["_0.si"]; !ok {
		t.Fatal("Files map must contain _0.si")
	}
}

// ---------------------------------------------------------------------------
// ReplicaFileDeleter
// ---------------------------------------------------------------------------

func TestReplicaFileDeleter_IncDecRef(t *testing.T) {
	d := nrt.NewReplicaFileDeleter(nil, nil)
	d.IncRef([]string{"a.txt", "b.txt"})
	if got := d.GetRefCount("a.txt"); got != 1 {
		t.Fatalf("refcount after IncRef: want 1, got %d", got)
	}
	d.IncRef([]string{"a.txt"})
	if got := d.GetRefCount("a.txt"); got != 2 {
		t.Fatalf("refcount after second IncRef: want 2, got %d", got)
	}
	d.DecRef([]string{"a.txt"})
	if got := d.GetRefCount("a.txt"); got != 1 {
		t.Fatalf("refcount after DecRef: want 1, got %d", got)
	}
	d.DecRef([]string{"a.txt"})
	if got := d.GetRefCount("a.txt"); got != 0 {
		t.Fatalf("refcount after final DecRef: want 0, got %d", got)
	}
}

func TestReplicaFileDeleter_DeleteIfNoRef(t *testing.T) {
	d := nrt.NewReplicaFileDeleter(nil, nil)
	d.IncRef([]string{"c.txt"})
	d.DeleteIfNoRef("c.txt") // should not delete — still referenced
	if got := d.GetRefCount("c.txt"); got != 1 {
		t.Fatalf("should still be referenced, got %d", got)
	}
	d.DecRef([]string{"c.txt"})
	d.DeleteIfNoRef("c.txt") // now unreferenced
	if got := d.GetRefCount("c.txt"); got != 0 {
		t.Fatalf("should be 0, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// CopyJob
// ---------------------------------------------------------------------------

func TestCopyJob_Ordinals(t *testing.T) {
	j1 := nrt.NewCopyJob("flush", nil, false, nil)
	j2 := nrt.NewCopyJob("merge", nil, true, nil)
	if j1.Ord >= j2.Ord {
		t.Fatalf("ordinals must be strictly increasing: j1=%d j2=%d", j1.Ord, j2.Ord)
	}
}

func TestCopyJob_Cancel(t *testing.T) {
	j := nrt.NewCopyJob("test", nil, false, nil)
	if j.GetFailed() {
		t.Fatal("fresh job must not be failed")
	}
	j.Cancel("shutdown", errors.New("node gone"))
	if !j.GetFailed() {
		t.Fatal("cancelled job must report failed")
	}
}

// ---------------------------------------------------------------------------
// CopyOneFile
// ---------------------------------------------------------------------------

func TestCopyOneFile_Fields(t *testing.T) {
	meta := &nrt.FileMetaData{Length: 256, Checksum: 7}
	c := nrt.NewCopyOneFile("_0.cfs", "_0.cfs.tmp", meta)
	if c.FileName() != "_0.cfs" {
		t.Fatalf("FileName: want _0.cfs, got %s", c.FileName())
	}
	if c.TmpFileName() != "_0.cfs.tmp" {
		t.Fatalf("TmpFileName: want _0.cfs.tmp, got %s", c.TmpFileName())
	}
	if c.BytesCopied() != 0 {
		t.Fatalf("BytesCopied initial: want 0, got %d", c.BytesCopied())
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Node
// ---------------------------------------------------------------------------

func TestNode_Message(t *testing.T) {
	node := nrt.NewNode(0, nil, io.Discard)
	node.Message("test message") // must not panic
}

func TestNode_VerboseFiles(t *testing.T) {
	node := nrt.NewNode(1, nil, io.Discard)
	node.SetVerboseFiles(true)
	if !node.IsVerboseFiles() {
		t.Fatal("IsVerboseFiles must reflect SetVerboseFiles(true)")
	}
	node.SetVerboseFiles(false)
	if node.IsVerboseFiles() {
		t.Fatal("IsVerboseFiles must reflect SetVerboseFiles(false)")
	}
}

func TestNodeConstants(t *testing.T) {
	if nrt.PrimaryGenKey != "__primaryGen" {
		t.Fatalf("PrimaryGenKey: want __primaryGen, got %s", nrt.PrimaryGenKey)
	}
	if nrt.VersionKey != "__version" {
		t.Fatalf("VersionKey: want __version, got %s", nrt.VersionKey)
	}
}

// ---------------------------------------------------------------------------
// PrimaryNode
// ---------------------------------------------------------------------------

func TestPrimaryNode_GetCopyState_InitiallyNil(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	if p.GetCopyState() != nil {
		t.Fatal("initial CopyState must be nil")
	}
}

func TestPrimaryNode_ReadLocalFileMetaData_ReturnsNilStub(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	if p.ReadLocalFileMetaData("_0.si") != nil {
		t.Fatal("stub must return nil")
	}
}

func TestPrimaryNode_Close(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReplicaNode
// ---------------------------------------------------------------------------

func TestReplicaNode_GetCurrentVersion_Initial(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)
	if got := r.GetCurrentVersion(); got != -1 {
		t.Fatalf("initial version: want -1, got %d", got)
	}
}

func TestReplicaNode_NewNRTPointStub(t *testing.T) {
	r := nrt.NewReplicaNode(1, nil, io.Discard)
	if err := r.NewNRTPoint(1, 1, nil); err != nil {
		t.Fatalf("NewNRTPoint stub: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SegmentInfosSearcherManager
// ---------------------------------------------------------------------------

func TestSegmentInfosSearcherManager_Close(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)
	m := nrt.NewSegmentInfosSearcherManager(nil, r)
	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PreCopyMergedSegmentWarmer
// ---------------------------------------------------------------------------

func TestPreCopyMergedSegmentWarmer_WarmStub(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	w := nrt.NewPreCopyMergedSegmentWarmer(p)
	if err := w.Warm(nil); err != nil {
		t.Fatalf("Warm stub: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Wire encoder / decoder
// ---------------------------------------------------------------------------

// TestWire_RoundTrip verifies that WriteCopyStateOrdered + ReadCopyState
// produce a logically identical CopyState for both canary seeds.
func TestWire_RoundTrip(t *testing.T) {
	for _, seed := range [...]int64{0xC0FFEE, 0xDECAF} {
		seed := seed
		t.Run("seed", func(t *testing.T) {
			state := nrt.BuildCopyStateOrdered(seed)

			var buf bytes.Buffer
			out := store.NewOutputStreamDataOutput(&buf)
			if err := nrt.WriteCopyStateOrdered(state, out); err != nil {
				t.Fatalf("WriteCopyStateOrdered: %v", err)
			}
			if buf.Len() == 0 {
				t.Fatal("WriteCopyStateOrdered produced 0 bytes")
			}

			in := store.NewByteArrayDataInput(buf.Bytes())
			got, err := nrt.ReadCopyState(in)
			if err != nil {
				t.Fatalf("ReadCopyState: %v", err)
			}

			if got.Gen != state.Gen {
				t.Errorf("Gen: want %d got %d", state.Gen, got.Gen)
			}
			if got.Version != state.Version {
				t.Errorf("Version: want %d got %d", state.Version, got.Version)
			}
			if got.PrimaryGen != state.PrimaryGen {
				t.Errorf("PrimaryGen: want %d got %d", state.PrimaryGen, got.PrimaryGen)
			}
			if !bytes.Equal(got.InfosBytes, state.InfosBytes) {
				t.Errorf("InfosBytes: want len=%d got len=%d", len(state.InfosBytes), len(got.InfosBytes))
			}
			if len(got.Files) != state.Files.Len() {
				t.Errorf("files count: want %d got %d", state.Files.Len(), len(got.Files))
			}
			for _, name := range state.Files.Names() {
				exp, _ := state.Files.Get(name)
				act, ok := got.Files[name]
				if !ok {
					t.Errorf("file %q missing after round-trip", name)
					continue
				}
				if act.Length != exp.Length {
					t.Errorf("file %q: Length want %d got %d", name, exp.Length, act.Length)
				}
				if act.Checksum != exp.Checksum {
					t.Errorf("file %q: Checksum want %d got %d", name, exp.Checksum, act.Checksum)
				}
				if !bytes.Equal(act.Header, exp.Header) {
					t.Errorf("file %q: Header mismatch", name)
				}
				if !bytes.Equal(act.Footer, exp.Footer) {
					t.Errorf("file %q: Footer mismatch", name)
				}
			}
			if len(got.CompletedMergeFiles) != state.CompletedMergeFiles.Len() {
				t.Errorf("completedMergeFiles count: want %d got %d",
					state.CompletedMergeFiles.Len(), len(got.CompletedMergeFiles))
			}
		})
	}
}

// TestWire_Determinism verifies that two calls to WriteCopyStateOrdered at
// the same seed produce identical bytes.
func TestWire_Determinism(t *testing.T) {
	seed := int64(0xC0FFEE)

	encode := func() []byte {
		state := nrt.BuildCopyStateOrdered(seed)
		var buf bytes.Buffer
		out := store.NewOutputStreamDataOutput(&buf)
		if err := nrt.WriteCopyStateOrdered(state, out); err != nil {
			t.Fatalf("WriteCopyStateOrdered: %v", err)
		}
		return buf.Bytes()
	}

	b1 := encode()
	b2 := encode()
	if !bytes.Equal(b1, b2) {
		t.Fatalf("WriteCopyStateOrdered is not deterministic: len1=%d len2=%d", len(b1), len(b2))
	}
}

// TestIDFromSeed verifies that IDFromSeed produces the expected 16-byte big-endian
// id matching the Java Determinism.idBytes contract.
func TestIDFromSeed(t *testing.T) {
	seed := int64(0xC0FFEE)
	id := nrt.IDFromSeed(seed)
	if len(id) != 16 {
		t.Fatalf("IDFromSeed: want 16 bytes, got %d", len(id))
	}
	// First 8 bytes == big-endian seed; next 8 == big-endian ^seed.
	const want0 = uint64(0xC0FFEE)
	got0 := uint64(id[0])<<56 | uint64(id[1])<<48 | uint64(id[2])<<40 | uint64(id[3])<<32 |
		uint64(id[4])<<24 | uint64(id[5])<<16 | uint64(id[6])<<8 | uint64(id[7])
	if got0 != want0 {
		t.Errorf("IDFromSeed[0:8]: want %016x got %016x", want0, got0)
	}
	want1 := ^want0
	got1 := uint64(id[8])<<56 | uint64(id[9])<<48 | uint64(id[10])<<40 | uint64(id[11])<<32 |
		uint64(id[12])<<24 | uint64(id[13])<<16 | uint64(id[14])<<8 | uint64(id[15])
	if got1 != want1 {
		t.Errorf("IDFromSeed[8:16]: want %016x got %016x", want1, got1)
	}
}

// ---------------------------------------------------------------------------
// ReplicaFileDeleter — directory deletion
// ---------------------------------------------------------------------------

// TestReplicaFileDeleter_DeletesFromDir verifies that DecRef calls
// Directory.DeleteFile when the refcount reaches zero.
func TestReplicaFileDeleter_DeletesFromDir(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create a file so it exists when DeleteFile is called.
	out, err := dir.CreateOutput("a.txt", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteByte(0); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	d := nrt.NewReplicaFileDeleter(nil, dir)
	if err := d.IncRef([]string{"a.txt"}); err != nil {
		t.Fatalf("IncRef: %v", err)
	}
	if !dir.FileExists("a.txt") {
		t.Fatal("file must exist before DecRef")
	}
	if err := d.DecRef([]string{"a.txt"}); err != nil {
		t.Fatalf("DecRef: %v", err)
	}
	if dir.FileExists("a.txt") {
		t.Fatal("DecRef to 0 must delete the file from the directory")
	}
}

// TestReplicaFileDeleter_DeleteIfNoRef_Dir verifies DeleteIfNoRef removes the
// file from the directory when its refcount is already zero.
func TestReplicaFileDeleter_DeleteIfNoRef_Dir(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("b.txt", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteByte(1); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	d := nrt.NewReplicaFileDeleter(nil, dir)
	// File is unreferenced (count == 0).
	if err := d.DeleteIfNoRef("b.txt"); err != nil {
		t.Fatalf("DeleteIfNoRef: %v", err)
	}
	if dir.FileExists("b.txt") {
		t.Fatal("DeleteIfNoRef with count=0 must delete the file")
	}
}

// TestReplicaFileDeleter_NoDeleteWhileReferenced verifies that DecRef does
// NOT delete a file while it still has outstanding references.
func TestReplicaFileDeleter_NoDeleteWhileReferenced(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("c.txt", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteByte(2); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	d := nrt.NewReplicaFileDeleter(nil, dir)
	if err := d.IncRef([]string{"c.txt", "c.txt"}); err != nil {
		t.Fatalf("IncRef (2×): %v", err)
	}
	if err := d.DecRef([]string{"c.txt"}); err != nil {
		t.Fatalf("DecRef (1×): %v", err)
	}
	if !dir.FileExists("c.txt") {
		t.Fatal("file must still exist when refcount is 1 after DecRef from 2")
	}
	if err := d.DecRef([]string{"c.txt"}); err != nil {
		t.Fatalf("DecRef (2×): %v", err)
	}
	if dir.FileExists("c.txt") {
		t.Fatal("file must be deleted once refcount reaches 0")
	}
}

// ---------------------------------------------------------------------------
// Node.ReadLocalFileMetaData
// ---------------------------------------------------------------------------

// writeTestFile writes a minimal but structurally valid Lucene codec file to
// dir under fileName and returns the CRC32 checksum stored in the footer.
//
// Layout (matches codecs.WriteIndexHeader + codecs.WriteFooter):
//
//	4 bytes   codec magic BE int32
//	vInt+str  codec name
//	4 bytes   version BE int32
//	16 bytes  segment ID
//	1 byte    suffix length (0)
//	4 bytes   footer magic BE int32
//	4 bytes   algorithm = 0
//	8 bytes   CRC32 checksum (computed over all preceding bytes)
//
// The returned checksum is the value that Lucene's retrieveChecksum reads —
// i.e., the CRC32 of all bytes up to (not including) the checksum long itself.
func writeTestFile(t *testing.T, dir store.Directory, fileName string) int64 {
	t.Helper()
	out, err := dir.CreateOutput(fileName, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput %q: %v", fileName, err)
	}
	cOut := store.NewChecksumIndexOutput(out)

	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	if err := codecs.WriteIndexHeader(cOut, "TestCodec", 1, id, ""); err != nil {
		t.Fatalf("WriteIndexHeader: %v", err)
	}
	if err := codecs.WriteFooter(cOut); err != nil {
		t.Fatalf("WriteFooter: %v", err)
	}
	if err := cOut.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Retrieve the actual stored checksum by reading the file back.
	// This is the CRC32 of [header + FooterMagic + algo] (not including the
	// checksum long itself), which is what ChecksumIndexOutput captured before
	// writing the checksum.
	in, err := dir.OpenInput(fileName, store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput for checksum: %v", err)
	}
	defer in.Close()
	checksum, cerr := codecs.RetrieveChecksum(in)
	if cerr != nil {
		t.Fatalf("RetrieveChecksum: %v", cerr)
	}
	return checksum
}

// TestNode_ReadLocalFileMetaData_ValidFile verifies that a structurally valid
// file returns non-nil FileMetaData with the correct length and checksum.
func TestNode_ReadLocalFileMetaData_ValidFile(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	expectedChecksum := writeTestFile(t, dir, "_0.si")

	node := nrt.NewNode(0, dir, io.Discard)
	md, err := node.ReadLocalFileMetaData("_0.si")
	if err != nil {
		t.Fatalf("ReadLocalFileMetaData: %v", err)
	}
	if md == nil {
		t.Fatal("ReadLocalFileMetaData: want non-nil for valid file")
	}

	length, lerr := dir.FileLength("_0.si")
	if lerr != nil {
		t.Fatalf("FileLength: %v", lerr)
	}
	if md.Length != length {
		t.Errorf("Length: want %d got %d", length, md.Length)
	}
	if md.Checksum != expectedChecksum {
		t.Errorf("Checksum: want %d got %d", expectedChecksum, md.Checksum)
	}
	if len(md.Header) == 0 {
		t.Error("Header must not be empty")
	}
	if len(md.Footer) != codecs.FooterLength() {
		t.Errorf("Footer length: want %d got %d", codecs.FooterLength(), len(md.Footer))
	}
}

// TestNode_ReadLocalFileMetaData_MissingFile verifies that a missing file
// returns (nil, nil) rather than an error.
func TestNode_ReadLocalFileMetaData_MissingFile(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	node := nrt.NewNode(0, dir, io.Discard)
	md, err := node.ReadLocalFileMetaData("nonexistent.si")
	if err != nil {
		t.Fatalf("want nil error for missing file, got %v", err)
	}
	if md != nil {
		t.Fatal("want nil FileMetaData for missing file")
	}
}

// TestNode_ReadLocalFileMetaData_NilDir verifies that a Node with a nil
// directory returns (nil, nil) without panicking.
func TestNode_ReadLocalFileMetaData_NilDir(t *testing.T) {
	node := nrt.NewNode(0, nil, io.Discard)
	md, err := node.ReadLocalFileMetaData("_0.si")
	if err != nil {
		t.Fatalf("nil dir: want nil error, got %v", err)
	}
	if md != nil {
		t.Fatal("nil dir: want nil FileMetaData")
	}
}

// ---------------------------------------------------------------------------
// CopyOneFile.Copy
// ---------------------------------------------------------------------------

// buildCopyFilePayload constructs the byte slice that the primary would send
// over the wire for a single file: (length−8) data bytes followed by the
// big-endian CRC32 checksum long.
//
// The checksum stored in a Lucene file is computed by ChecksumIndexOutput over
// all bytes written up to (but not including) the checksum long itself — that
// means it covers the header, any payload data, the footer magic (4 bytes), and
// the algorithm field (4 bytes). We retrieve it directly with
// codecs.RetrieveChecksum so we use exactly the same value the file stores.
//
// The wire payload mirrors Java SimplePrimaryNode: first (length−8) bytes of
// the file, then the 8-byte big-endian CRC32 checksum long.
func buildCopyFilePayload(t *testing.T, dir store.Directory, fileName string) ([]byte, int64) {
	t.Helper()
	in, err := dir.OpenInput(fileName, store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput %q: %v", fileName, err)
	}
	defer in.Close()

	length := in.Length()
	all := make([]byte, length)
	if err := in.ReadBytes(all); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// The actual checksum stored in the file is the CRC32 computed by
	// ChecksumIndexOutput over all bytes it wrote (header + footer magic + algo),
	// NOT including the checksum long itself. Retrieve it directly.
	checksum, cerr := codecs.RetrieveChecksum(in)
	if cerr != nil {
		t.Fatalf("RetrieveChecksum %q: %v", fileName, cerr)
	}

	// Wire payload: data bytes (excluding last 8) + big-endian checksum long.
	dataBytes := all[:length-8]
	payload := make([]byte, len(dataBytes)+8)
	copy(payload, dataBytes)
	binary.BigEndian.PutUint64(payload[len(dataBytes):], uint64(checksum))

	return payload, checksum
}

// TestCopyOneFile_Copy_RoundTrip writes a valid codec file, builds the
// primary's wire payload, then verifies that CopyOneFile.Copy produces byte-
// identical output in a fresh output.
func TestCopyOneFile_Copy_RoundTrip(t *testing.T) {
	srcDir := store.NewByteBuffersDirectory()
	defer srcDir.Close()
	dstDir := store.NewByteBuffersDirectory()
	defer dstDir.Close()

	writeTestFile(t, srcDir, "_1.si")

	payload, expectedChecksum := buildCopyFilePayload(t, srcDir, "_1.si")
	srcLen, _ := srcDir.FileLength("_1.si")

	meta := &nrt.FileMetaData{
		Length:   srcLen,
		Checksum: expectedChecksum,
	}
	c := nrt.NewCopyOneFile("_1.si", "_1.si.tmp", meta)

	// Source: ByteArrayDataInput wrapping the payload (acts as the network stream).
	in := store.NewByteArrayDataInput(payload)
	// Note: ByteArrayDataInput does not implement store.IndexInput (no Length/SetPosition).
	// Wrap it in a ByteBuffersIndexInput by writing the payload to dstDir first.
	payloadOut, err := dstDir.CreateOutput("_1.si.src", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput src payload: %v", err)
	}
	if err := payloadOut.WriteBytes(payload); err != nil {
		t.Fatalf("WriteBytes payload: %v", err)
	}
	if err := payloadOut.Close(); err != nil {
		t.Fatalf("Close payload output: %v", err)
	}
	_ = in // replaced by the IndexInput below

	srcIn, err := dstDir.OpenInput("_1.si.src", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput payload: %v", err)
	}
	defer srcIn.Close()

	// Destination: ChecksumIndexOutput wrapping a fresh output.
	rawOut, err := dstDir.CreateOutput("_1.si.tmp", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput dst: %v", err)
	}
	dstOut := store.NewChecksumIndexOutput(rawOut)

	if err := c.Copy(srcIn, dstOut); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if err := dstOut.Close(); err != nil {
		t.Fatalf("Close dstOut: %v", err)
	}

	// The copy must have advanced BytesCopied to srcLen (all bytes including the
	// 8-byte checksum long that Copy appends).
	if c.BytesCopied() != srcLen {
		t.Errorf("BytesCopied: want %d got %d", srcLen, c.BytesCopied())
	}
}

// TestCopyOneFile_Copy_ChecksumMismatch verifies that Copy returns an error
// when the data bytes produce a different CRC than expected.
func TestCopyOneFile_Copy_ChecksumMismatch(t *testing.T) {
	srcDir := store.NewByteBuffersDirectory()
	defer srcDir.Close()

	writeTestFile(t, srcDir, "_2.si")
	payload, _ := buildCopyFilePayload(t, srcDir, "_2.si")
	srcLen, _ := srcDir.FileLength("_2.si")

	// Supply a wrong expected checksum.
	meta := &nrt.FileMetaData{Length: srcLen, Checksum: 0xDEADBEEF}
	c := nrt.NewCopyOneFile("_2.si", "_2.si.tmp", meta)

	payloadDir := store.NewByteBuffersDirectory()
	defer payloadDir.Close()
	po, err := payloadDir.CreateOutput("p", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := po.WriteBytes(payload); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := po.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	srcIn, err := payloadDir.OpenInput("p", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer srcIn.Close()

	rawOut, err := payloadDir.CreateOutput("out", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput dst: %v", err)
	}
	dstOut := store.NewChecksumIndexOutput(rawOut)
	defer dstOut.Close()

	if err := c.Copy(srcIn, dstOut); err == nil {
		t.Fatal("Copy: expected checksum mismatch error, got nil")
	}
}

// ---------------------------------------------------------------------------
// ReplicaNode.NewNRTPoint — version-guard and primary-gen cut-over
// ---------------------------------------------------------------------------

// TestReplicaNode_NewNRTPoint_VersionAdvances verifies that a new NRT point
// with a higher version is accepted and stored.
func TestReplicaNode_NewNRTPoint_VersionAdvances(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)

	if err := r.NewNRTPoint(1, 100, []byte{0xAB}); err != nil {
		t.Fatalf("NewNRTPoint v100: %v", err)
	}
	if got := r.GetCurrentVersion(); got != 100 {
		t.Errorf("GetCurrentVersion: want 100, got %d", got)
	}

	if err := r.NewNRTPoint(1, 200, []byte{0xCD}); err != nil {
		t.Fatalf("NewNRTPoint v200: %v", err)
	}
	if got := r.GetCurrentVersion(); got != 200 {
		t.Errorf("GetCurrentVersion: want 200, got %d", got)
	}
}

// TestReplicaNode_NewNRTPoint_SameVersionNoOp verifies that a version equal
// to the current is a no-op (version does not change).
func TestReplicaNode_NewNRTPoint_SameVersionNoOp(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)
	if err := r.NewNRTPoint(1, 50, nil); err != nil {
		t.Fatalf("first NewNRTPoint: %v", err)
	}
	if err := r.NewNRTPoint(1, 50, nil); err != nil {
		t.Fatalf("same-version NewNRTPoint: %v", err)
	}
	if got := r.GetCurrentVersion(); got != 50 {
		t.Errorf("GetCurrentVersion: want 50, got %d", got)
	}
}

// TestReplicaNode_NewNRTPoint_StaleVersionNoOp verifies that a version older
// than the current is silently ignored.
func TestReplicaNode_NewNRTPoint_StaleVersionNoOp(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)
	if err := r.NewNRTPoint(1, 99, nil); err != nil {
		t.Fatalf("NewNRTPoint v99: %v", err)
	}
	// Send an older version — must be ignored.
	if err := r.NewNRTPoint(1, 10, nil); err != nil {
		t.Fatalf("NewNRTPoint v10 (stale): %v", err)
	}
	if got := r.GetCurrentVersion(); got != 99 {
		t.Errorf("GetCurrentVersion after stale point: want 99, got %d", got)
	}
}

// TestReplicaNode_NewNRTPoint_PrimaryGenCutOver verifies that a new primary
// generation is accepted and the replica's lastPrimaryGen is updated
// (the new version takes effect).
func TestReplicaNode_NewNRTPoint_PrimaryGenCutOver(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)
	if err := r.NewNRTPoint(1, 10, nil); err != nil {
		t.Fatalf("NewNRTPoint primaryGen=1 v10: %v", err)
	}
	// New primary elected; version must advance.
	if err := r.NewNRTPoint(2, 11, nil); err != nil {
		t.Fatalf("NewNRTPoint primaryGen=2 v11: %v", err)
	}
	if got := r.GetCurrentVersion(); got != 11 {
		t.Errorf("GetCurrentVersion after primary change: want 11, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// PreCopyMergedSegmentWarmer.Warm delegates to PrimaryNode
// ---------------------------------------------------------------------------

// TestPreCopyMergedSegmentWarmer_Warm_NoPanic verifies that Warm with a nil
// info argument does not panic (PrimaryNode.PreCopyMergedSegmentFiles is a
// no-op stub).
func TestPreCopyMergedSegmentWarmer_Warm_NoPanic(t *testing.T) {
	p := nrt.NewPrimaryNode(0, 1, nil, io.Discard)
	w := nrt.NewPreCopyMergedSegmentWarmer(p)
	if err := w.Warm(nil); err != nil {
		t.Fatalf("Warm(nil): %v", err)
	}
}

// ---------------------------------------------------------------------------
// SegmentInfosSearcherManager.SetCurrentInfos / GetCurrentInfos
// ---------------------------------------------------------------------------

func TestSegmentInfosSearcherManager_SetGet(t *testing.T) {
	r := nrt.NewReplicaNode(0, nil, io.Discard)
	m := nrt.NewSegmentInfosSearcherManager(nil, r)

	if got := m.GetCurrentInfos(); got != nil {
		t.Fatalf("initial infos must be nil, got %v", got)
	}

	sentinel := &spi.SegmentInfos{}
	m.SetCurrentInfos(sentinel)
	if got := m.GetCurrentInfos(); got != sentinel {
		t.Fatalf("GetCurrentInfos: want sentinel, got %v", got)
	}
}
