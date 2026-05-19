// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func newIFDTestWriter(t *testing.T, dir store.Directory) *IndexWriter {
	t.Helper()
	cfg := NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return w
}

func TestIndexFileDeleter_InitEmptyDirectory(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	w := newIFDTestWriter(t, dir)
	defer w.Close()

	infos := NewSegmentInfos()
	ifd, err := NewIndexFileDeleter(
		nil, dir, dir,
		&NoDeletionPolicy{}, infos, nil, w, false, false,
	)
	if err != nil {
		t.Fatalf("NewIndexFileDeleter: %v", err)
	}
	if ifd.StartingCommitDeleted() {
		t.Fatalf("starting commit should not be deleted for empty dir")
	}
	if ifd.LastSegmentInfos() != nil {
		t.Fatalf("LastSegmentInfos = %v, want nil for empty dir", ifd.LastSegmentInfos())
	}
	if err := ifd.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestIndexFileDeleter_RejectsNilWriter(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	_, err = NewIndexFileDeleter(nil, dir, dir, nil, NewSegmentInfos(), nil, nil, false, false)
	if err == nil {
		t.Fatalf("expected error for nil writer")
	}
}

func TestIndexFileDeleter_RefreshDeletesOrphans(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	w := newIFDTestWriter(t, dir)
	defer w.Close()

	infos := NewSegmentInfos()
	ifd, err := NewIndexFileDeleter(
		nil, dir, dir, &NoDeletionPolicy{}, infos, nil, w, false, false,
	)
	if err != nil {
		t.Fatalf("NewIndexFileDeleter: %v", err)
	}
	defer ifd.Close()

	// Create a codec-shaped orphan file directly on disk.
	out, err := dir.CreateOutput("_0.cfs", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteByte(0xAA); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	if err := ifd.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if dir.FileExists("_0.cfs") {
		t.Fatalf("orphan _0.cfs should have been deleted")
	}
}

func TestIndexFileDeleter_RefreshIgnoresNonCodecFiles(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	w := newIFDTestWriter(t, dir)
	defer w.Close()

	ifd, err := NewIndexFileDeleter(
		nil, dir, dir, &NoDeletionPolicy{}, NewSegmentInfos(), nil, w, false, false,
	)
	if err != nil {
		t.Fatalf("NewIndexFileDeleter: %v", err)
	}
	defer ifd.Close()

	out, err := dir.CreateOutput("write.lock", store.IOContextDefault)
	if err == nil {
		_ = out.Close()
	}
	out2, err := dir.CreateOutput("random_other.txt", store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	_ = out2.Close()

	if err := ifd.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !dir.FileExists("random_other.txt") {
		t.Fatalf("non-codec file random_other.txt should be left untouched")
	}
}

func TestIndexFileDeleter_DeleteNewFilesUntracked(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	w := newIFDTestWriter(t, dir)
	defer w.Close()

	ifd, err := NewIndexFileDeleter(
		nil, dir, dir, &NoDeletionPolicy{}, NewSegmentInfos(), nil, w, false, false,
	)
	if err != nil {
		t.Fatalf("NewIndexFileDeleter: %v", err)
	}
	defer ifd.Close()

	for _, name := range []string{"_x.foo", "_y.bar"} {
		out, err := dir.CreateOutput(name, store.IOContextDefault)
		if err != nil {
			t.Fatalf("CreateOutput %q: %v", name, err)
		}
		_ = out.Close()
	}

	if err := ifd.DeleteNewFiles([]string{"_x.foo", "_y.bar"}); err != nil {
		t.Fatalf("DeleteNewFiles: %v", err)
	}
	for _, name := range []string{"_x.foo", "_y.bar"} {
		if dir.FileExists(name) {
			t.Errorf("file %q should have been deleted", name)
		}
	}
}

func TestIndexFileDeleter_IncRefDecRef(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	w := newIFDTestWriter(t, dir)
	defer w.Close()

	ifd, err := NewIndexFileDeleter(
		nil, dir, dir, &NoDeletionPolicy{}, NewSegmentInfos(), nil, w, false, false,
	)
	if err != nil {
		t.Fatalf("NewIndexFileDeleter: %v", err)
	}
	defer ifd.Close()

	files := []string{"_a.foo", "_a.bar"}
	for _, f := range files {
		out, err := dir.CreateOutput(f, store.IOContextDefault)
		if err != nil {
			t.Fatalf("CreateOutput %q: %v", f, err)
		}
		_ = out.Close()
	}
	ifd.IncRefFiles(files)
	for _, f := range files {
		if !ifd.Exists(f) {
			t.Errorf("file %q should exist after IncRefFiles", f)
		}
	}

	// Two refs each -> needs two DecRefAll calls before disappearing.
	ifd.IncRefFiles(files)
	if err := ifd.DecRefFiles(files); err != nil {
		t.Fatalf("DecRefFiles (1): %v", err)
	}
	for _, f := range files {
		if !ifd.Exists(f) {
			t.Errorf("file %q should still be tracked after first decref", f)
		}
	}
	if err := ifd.DecRefFiles(files); err != nil {
		t.Fatalf("DecRefFiles (2): %v", err)
	}
	for _, f := range files {
		if ifd.Exists(f) {
			t.Errorf("file %q should be gone after second decref", f)
		}
	}
}

func TestInflateGens_AdvancesCounterAndGeneration(t *testing.T) {
	infos := NewSegmentInfos()
	infos.SetGeneration(2)
	infos.SetCounter(0)

	files := []string{
		"segments_5", // gen 5 in base-36
		"_a.cfs",     // segment "_a" => 10 in base-36
		"_a_3.liv",   // gen 3 for segment _a
		"pending_segments_7",
		"write.lock",
		"_b.tmp", // ignored: .tmp
	}

	inflateGens(infos, files, nil)

	if infos.Generation() < 7 {
		t.Errorf("generation = %d, want >= 7", infos.Generation())
	}
	if infos.Counter() < 11 { // 1 + max("a" in base36) = 1 + 10
		t.Errorf("counter = %d, want >= 11", infos.Counter())
	}
}

func TestParseSegmentsGen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in     string
		prefix string
		want   int64
		ok     bool
	}{
		{"segments_a", SegmentsPrefix, 10, true},
		{"segments_0", SegmentsPrefix, 0, true},
		{"segments", SegmentsPrefix, 0, true},
		{"segments_xyz!", SegmentsPrefix, 0, false},
		{"foo", SegmentsPrefix, 0, false},
	}
	for _, tc := range tests {
		got, ok := parseSegmentsGen(tc.in, tc.prefix)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseSegmentsGen(%q) = (%d,%t), want (%d,%t)",
				tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestFilesFromInfos_IncludesSegmentsFile(t *testing.T) {
	infos := NewSegmentInfos()
	infos.SetGeneration(3)

	got := filesFromInfos(infos, true)
	want := infos.GetFileName()
	found := false
	for _, f := range got {
		if f == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("filesFromInfos missing segments file %q, got %v", want, got)
	}

	got = filesFromInfos(infos, false)
	for _, f := range got {
		if f == want {
			t.Errorf("filesFromInfos(false) leaked segments file %q", want)
		}
	}
}

func TestUniqueStrings_Dedup(t *testing.T) {
	t.Parallel()
	got := uniqueStrings([]string{"a", "b", "a", "c", "b"})
	if len(got) != 3 {
		t.Fatalf("uniqueStrings len = %d, want 3 (got %v)", len(got), got)
	}
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "a") ||
		!strings.Contains(joined, "b") ||
		!strings.Contains(joined, "c") {
		t.Errorf("missing entries in %v", got)
	}
}

func TestIndexFileDeleter_CloseIsIdempotentWithNoLastFiles(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	w := newIFDTestWriter(t, dir)
	defer w.Close()

	ifd, err := NewIndexFileDeleter(
		nil, dir, dir, &NoDeletionPolicy{}, NewSegmentInfos(), nil, w, false, false,
	)
	if err != nil {
		t.Fatalf("NewIndexFileDeleter: %v", err)
	}
	if err := ifd.Close(); err != nil {
		t.Fatalf("Close (1): %v", err)
	}
	if err := ifd.Close(); err != nil {
		t.Fatalf("Close (2): %v", err)
	}
}
