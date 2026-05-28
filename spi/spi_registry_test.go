// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"bytes"
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newTestSegmentInfo builds a minimal SegmentInfo for the given name and
// document count, backed by an in-memory directory.
func newTestSegmentInfo(t *testing.T, name string, docCount int) *schema.SegmentInfo {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	return schema.NewSegmentInfo(name, docCount, dir)
}

// TestGetSegmentFileName pins the segments_N file-name encoding. Lucene names
// the segments file "segments_" + Long.toString(generation, MAX_RADIX), where
// MAX_RADIX is 36; the base-36 encoding is part of the on-disk contract, so a
// regression here would break directory discovery against a real index.
func TestGetSegmentFileName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		gen  int64
		want string
	}{
		{0, "segments_0"},
		{1, "segments_1"},
		{9, "segments_9"},
		{10, "segments_a"},  // base-36: 10 -> 'a'
		{35, "segments_z"},  // base-36: 35 -> 'z'
		{36, "segments_10"}, // base-36: 36 -> "10"
		{-1, ""},            // negative generation has no file
	}
	for _, tc := range cases {
		if got := GetSegmentFileName(tc.gen); got != tc.want {
			t.Errorf("GetSegmentFileName(%d) = %q, want %q", tc.gen, got, tc.want)
		}
	}
}

// TestSegmentInfosLifecycle exercises add/get/size/remove/insert plus the
// generation and counter accounting that drives segment-file naming.
func TestSegmentInfosLifecycle(t *testing.T) {
	t.Parallel()
	infos := NewSegmentInfos()

	if got := infos.Size(); got != 0 {
		t.Fatalf("fresh SegmentInfos Size() = %d, want 0", got)
	}
	if got := infos.Get(0); got != nil {
		t.Fatalf("Get(0) on empty = %v, want nil", got)
	}

	// Defaults established by NewSegmentInfos.
	if got := infos.Generation(); got != 1 {
		t.Errorf("initial Generation() = %d, want 1", got)
	}
	if got := infos.IndexCreatedVersionMajor(); got != 10 {
		t.Errorf("IndexCreatedVersionMajor() = %d, want 10 (Lucene 10.x)", got)
	}

	sciA := NewSegmentCommitInfo(newTestSegmentInfo(t, "_0", 5), 0, -1)
	sciB := NewSegmentCommitInfo(newTestSegmentInfo(t, "_1", 7), 0, -1)
	infos.Add(sciA)
	infos.Add(sciB)

	if got := infos.Size(); got != 2 {
		t.Fatalf("Size() after 2 adds = %d, want 2", got)
	}
	if got := infos.Get(0); got != sciA {
		t.Errorf("Get(0) = %v, want sciA", got)
	}
	if got := infos.Get(1); got != sciB {
		t.Errorf("Get(1) = %v, want sciB", got)
	}
	if got := infos.TotalDocCount(); got != 12 {
		t.Errorf("TotalDocCount() = %d, want 12 (5+7)", got)
	}

	// Insert in the middle shifts later segments right.
	sciC := NewSegmentCommitInfo(newTestSegmentInfo(t, "_2", 1), 0, -1)
	infos.Insert(1, sciC)
	if got := infos.Size(); got != 3 {
		t.Fatalf("Size() after insert = %d, want 3", got)
	}
	if got := infos.Get(1); got != sciC {
		t.Errorf("Get(1) after insert = %v, want sciC", got)
	}
	if got := infos.Get(2); got != sciB {
		t.Errorf("Get(2) after insert = %v, want sciB (shifted)", got)
	}

	// Remove returns the removed element and shrinks the list.
	removed := infos.Remove(1)
	if removed != sciC {
		t.Errorf("Remove(1) = %v, want sciC", removed)
	}
	if got := infos.Size(); got != 2 {
		t.Fatalf("Size() after remove = %d, want 2", got)
	}

	// List returns a defensive copy: mutating it must not affect the source.
	list := infos.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
	list[0] = nil
	if infos.Get(0) != sciA {
		t.Error("mutating List() result leaked into SegmentInfos")
	}

	infos.Clear()
	if got := infos.Size(); got != 0 {
		t.Errorf("Size() after Clear = %d, want 0", got)
	}
}

// TestSegmentInfosGenerationAndNaming verifies NextGeneration, the segment-name
// counter, and that GetFileName tracks the current generation.
func TestSegmentInfosGenerationAndNaming(t *testing.T) {
	t.Parallel()
	infos := NewSegmentInfos()

	if got := infos.NextGeneration(); got != 2 {
		t.Errorf("NextGeneration() from 1 = %d, want 2", got)
	}
	if got := infos.Generation(); got != 2 {
		t.Errorf("Generation() after NextGeneration = %d, want 2", got)
	}
	if got := infos.GetFileName(); got != "segments_2" {
		t.Errorf("GetFileName() = %q, want segments_2", got)
	}

	// GetNextSegmentName increments the counter and formats "_N".
	for i, want := range []string{"_0", "_1", "_2"} {
		if got := infos.GetNextSegmentName(); got != want {
			t.Errorf("GetNextSegmentName() call %d = %q, want %q", i, got, want)
		}
	}
	if got := infos.Counter(); got != 3 {
		t.Errorf("Counter() after 3 names = %d, want 3", got)
	}
}

// TestSegmentInfosUserData checks the commit user-data map round-trips and that
// accessors hand back defensive copies.
func TestSegmentInfosUserData(t *testing.T) {
	t.Parallel()
	infos := NewSegmentInfos()

	if got := infos.GetUserDataValue("missing"); got != "" {
		t.Errorf("GetUserDataValue(missing) = %q, want empty", got)
	}

	infos.SetUserDataValue("commitTime", "12345")
	if got := infos.GetUserDataValue("commitTime"); got != "12345" {
		t.Errorf("GetUserDataValue(commitTime) = %q, want 12345", got)
	}

	infos.SetUserData(map[string]string{"a": "1", "b": "2"})
	got := infos.GetUserData()
	if len(got) != 2 || got["a"] != "1" || got["b"] != "2" {
		t.Fatalf("GetUserData() = %v, want {a:1 b:2}", got)
	}
	// Mutating the returned map must not affect internal state.
	got["a"] = "tampered"
	if infos.GetUserDataValue("a") != "1" {
		t.Error("mutating GetUserData() result leaked into SegmentInfos")
	}
}

// TestSegmentCommitInfoDelAccounting covers the deletion / generation
// bookkeeping that SegmentCommitInfo tracks on top of its wrapped SegmentInfo.
func TestSegmentCommitInfoDelAccounting(t *testing.T) {
	t.Parallel()
	si := newTestSegmentInfo(t, "_3", 10)
	sci := NewSegmentCommitInfo(si, 0, -1)

	// Delegating accessors.
	if got := sci.Name(); got != "_3" {
		t.Errorf("Name() = %q, want _3", got)
	}
	if got := sci.DocCount(); got != 10 {
		t.Errorf("DocCount() = %d, want 10", got)
	}
	if got := sci.NumDocs(); got != 10 {
		t.Errorf("NumDocs() = %d, want 10 (no deletions)", got)
	}

	// Fresh segment: no deletions, generations all -1.
	if sci.HasDeletions() {
		t.Error("HasDeletions() = true on fresh segment, want false")
	}
	if got := sci.DelGen(); got != -1 {
		t.Errorf("DelGen() = %d, want -1", got)
	}
	if sci.HasFieldInfosGen() || sci.HasDocValuesGen() {
		t.Error("fresh segment reports a separate gen file, want none")
	}

	// Apply deletions: count and live-doc math must follow.
	sci.SetDelCount(3)
	sci.IncrDelCount(1)
	if got := sci.DelCount(); got != 4 {
		t.Errorf("DelCount() after Set(3)+Incr(1) = %d, want 4", got)
	}
	if got := sci.NumDocs(); got != 6 {
		t.Errorf("NumDocs() with 4 deletions = %d, want 6", got)
	}

	// AdvanceDelGen: -1 -> 1, then increments.
	if got := sci.AdvanceDelGen(); got != 1 {
		t.Errorf("first AdvanceDelGen() = %d, want 1", got)
	}
	if got := sci.AdvanceDelGen(); got != 2 {
		t.Errorf("second AdvanceDelGen() = %d, want 2", got)
	}
	if !sci.HasDeletions() {
		t.Error("HasDeletions() = false after AdvanceDelGen, want true")
	}
	// Del file name encodes name (sans leading underscore) and generation.
	if got := sci.GetDelFileName(); got != "_3_2.del" {
		t.Errorf("GetDelFileName() = %q, want _3_2.del", got)
	}

	// Soft deletes reduce NumDocs too.
	sci.SetSoftDelCount(2)
	if got := sci.NumDocs(); got != 4 {
		t.Errorf("NumDocs() with 4 hard + 2 soft deletions = %d, want 4", got)
	}
}

// TestSegmentCommitInfoClone confirms Clone produces an independent copy.
func TestSegmentCommitInfoClone(t *testing.T) {
	t.Parallel()
	si := newTestSegmentInfo(t, "_4", 8)
	sci := NewSegmentCommitInfo(si, 2, 5)
	sci.SetAttribute("k", "v")
	sci.SetDeletedOrdinals([]int{0, 3})

	clone := sci.Clone()
	if clone == sci {
		t.Fatal("Clone() returned the same pointer")
	}
	if clone.DelCount() != 2 || clone.DelGen() != 5 {
		t.Errorf("clone del state = (%d,%d), want (2,5)", clone.DelCount(), clone.DelGen())
	}
	if clone.GetAttribute("k") != "v" {
		t.Errorf("clone attribute k = %q, want v", clone.GetAttribute("k"))
	}

	// Mutating the clone must not affect the source.
	clone.SetDelCount(99)
	clone.SetAttribute("k", "changed")
	if sci.DelCount() != 2 {
		t.Errorf("source DelCount changed to %d after clone mutation, want 2", sci.DelCount())
	}
	if sci.GetAttribute("k") != "v" {
		t.Errorf("source attribute changed to %q after clone mutation, want v", sci.GetAttribute("k"))
	}
}

// TestSegmentCommitInfoListTotals checks the aggregate helpers on the list type.
func TestSegmentCommitInfoListTotals(t *testing.T) {
	t.Parallel()
	list := SegmentCommitInfoList{
		NewSegmentCommitInfo(newTestSegmentInfo(t, "_0", 10), 2, -1),
		NewSegmentCommitInfo(newTestSegmentInfo(t, "_1", 20), 5, -1),
	}
	if got := list.Size(); got != 2 {
		t.Errorf("Size() = %d, want 2", got)
	}
	if got := list.TotalDocCount(); got != 30 {
		t.Errorf("TotalDocCount() = %d, want 30", got)
	}
	if got := list.TotalDelCount(); got != 7 {
		t.Errorf("TotalDelCount() = %d, want 7", got)
	}
	if got := list.TotalNumDocs(); got != 23 {
		t.Errorf("TotalNumDocs() = %d, want 23 (30-7)", got)
	}
}

// TestCodecHeaderFooterRoundTrip is a binary-compatibility test for the codec
// envelope. It writes an index header + footer with CodecUtil, asserts the
// leading bytes match Lucene's CodecUtil.MAGIC exactly (big-endian, the same
// byte order Java's DataOutput.writeInt emits), then reads the envelope back
// and verifies the version and checksum validate.
func TestCodecHeaderFooterRoundTrip(t *testing.T) {
	t.Parallel()

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}

	const (
		fileName    = "_0.test"
		codecName   = "TestCodec"
		version     = int32(3)
		suffix      = "seg7"
		bodyPayload = "payload-bytes"
	)
	id := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

	// --- write ---
	out, err := dir.CreateOutput(fileName, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	cout := store.NewChecksumIndexOutput(out)
	if err := WriteIndexHeader(cout, codecName, version, id, suffix); err != nil {
		t.Fatalf("WriteIndexHeader: %v", err)
	}
	if err := cout.WriteBytes([]byte(bodyPayload)); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := WriteFooter(cout); err != nil {
		t.Fatalf("WriteFooter: %v", err)
	}
	if err := cout.Close(); err != nil {
		t.Fatalf("close output: %v", err)
	}

	// --- inspect raw leading bytes (binary-compatibility assertion) ---
	raw, err := dir.OpenInput(fileName, store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput raw: %v", err)
	}
	leading, err := raw.ReadBytesN(4)
	if err != nil {
		t.Fatalf("read magic bytes: %v", err)
	}
	if cerr := raw.Close(); cerr != nil {
		t.Fatalf("close raw input: %v", cerr)
	}
	// CodecUtil.MAGIC = 0x3FD76C17, written big-endian by writeInt.
	wantMagic := []byte{0x3F, 0xD7, 0x6C, 0x17}
	if !bytes.Equal(leading, wantMagic) {
		t.Fatalf("header magic bytes = % x, want % x", leading, wantMagic)
	}

	// --- read back through CodecUtil and validate ---
	in, err := dir.OpenInput(fileName, store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	cin := store.NewChecksumIndexInput(in)
	gotVersion, err := CheckIndexHeader(cin, codecName, version, version, id, suffix)
	if err != nil {
		t.Fatalf("CheckIndexHeader: %v", err)
	}
	if gotVersion != version {
		t.Errorf("CheckIndexHeader version = %d, want %d", gotVersion, version)
	}
	body, err := cin.ReadBytesN(len(bodyPayload))
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != bodyPayload {
		t.Errorf("body = %q, want %q", body, bodyPayload)
	}
	if _, err := CheckFooter(cin); err != nil {
		t.Fatalf("CheckFooter: %v", err)
	}
	if err := cin.Close(); err != nil {
		t.Fatalf("close input: %v", err)
	}
}

// TestCheckIndexHeaderRejectsWrongCodec verifies the reader rejects a header
// whose codec name does not match what the caller expects.
func TestCheckIndexHeaderRejectsWrongCodec(t *testing.T) {
	t.Parallel()

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	id := make([]byte, 16)

	out, err := dir.CreateOutput("_1.test", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := WriteIndexHeader(out, "Actual", 1, id, ""); err != nil {
		t.Fatalf("WriteIndexHeader: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close output: %v", err)
	}

	in, err := dir.OpenInput("_1.test", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()
	if _, err := CheckIndexHeader(in, "Expected", 1, 1, id, ""); err == nil {
		t.Fatal("CheckIndexHeader accepted a mismatched codec name, want error")
	}
}

// TestWriteIndexHeaderRejectsBadID guards the 16-byte segment-id invariant.
func TestWriteIndexHeaderRejectsBadID(t *testing.T) {
	t.Parallel()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	out, err := dir.CreateOutput("_2.test", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	defer out.Close()
	if err := WriteIndexHeader(out, "C", 1, []byte{1, 2, 3}, ""); err == nil {
		t.Fatal("WriteIndexHeader accepted a 3-byte id, want error")
	}
}

// TestIndexNotFoundException verifies the error type's wrapping and the
// errors.As-based detector.
func TestIndexNotFoundException(t *testing.T) {
	t.Parallel()

	cause := errors.New("no segments file")
	err := NewIndexNotFoundException("index missing", cause)

	if got := err.Error(); got != "index missing: no segments file" {
		t.Errorf("Error() = %q, want %q", got, "index missing: no segments file")
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is did not unwrap to the cause")
	}
	if !IsIndexNotFound(err) {
		t.Error("IsIndexNotFound returned false for an IndexNotFoundException")
	}

	// Wrapped deeper in a chain.
	wrapped := errors.Join(errors.New("outer"), err)
	if !IsIndexNotFound(wrapped) {
		t.Error("IsIndexNotFound returned false for a wrapped IndexNotFoundException")
	}

	// Message-only constructor has no cause suffix.
	msgOnly := IndexNotFoundExceptionFromMessage("empty directory")
	if got := msgOnly.Error(); got != "empty directory" {
		t.Errorf("message-only Error() = %q, want %q", got, "empty directory")
	}

	// A plain error is not an IndexNotFoundException.
	if IsIndexNotFound(errors.New("other")) {
		t.Error("IsIndexNotFound returned true for an unrelated error")
	}
}
