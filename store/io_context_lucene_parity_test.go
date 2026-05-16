// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "testing"

func TestIOContext_Default(t *testing.T) {
	if IOContextDefault.Context != ContextRead {
		t.Fatalf("IOContextDefault.Context = %v, want ContextRead", IOContextDefault.Context)
	}
	if IOContextDefault.MergeInfo != nil {
		t.Fatalf("IOContextDefault.MergeInfo = %v, want nil", IOContextDefault.MergeInfo)
	}
	if IOContextDefault.FlushInfo != nil {
		t.Fatalf("IOContextDefault.FlushInfo = %v, want nil", IOContextDefault.FlushInfo)
	}
}

func TestIOContext_ReadOnceCarriesHints(t *testing.T) {
	// Lucene 10.4.0: READONCE = new DefaultIOContext(DataAccessHint.SEQUENTIAL, ReadOnceHint.INSTANCE)
	if len(IOContextReadOnce.Hints) != 2 {
		t.Fatalf("IOContextReadOnce.Hints len = %d, want 2", len(IOContextReadOnce.Hints))
	}
	wantHints := map[FileOpenHint]bool{
		DataAccessSequential: false,
		ReadOnceInstance:     false,
	}
	for _, h := range IOContextReadOnce.Hints {
		if _, ok := wantHints[h]; ok {
			wantHints[h] = true
		}
	}
	for h, seen := range wantHints {
		if !seen {
			t.Errorf("missing expected hint %v in IOContextReadOnce.Hints", h)
		}
	}
}

func TestIOContext_WithHints_AppendsOnDefault(t *testing.T) {
	base := IOContextDefault
	got := base.WithHints(FileTypeData, FileDataPostings)
	if len(got.Hints) != 2 {
		t.Fatalf("got.Hints len = %d, want 2", len(got.Hints))
	}
	if got.Hints[0] != FileTypeData {
		t.Fatalf("got.Hints[0] = %v, want FileTypeData", got.Hints[0])
	}
	if got.Hints[1] != FileDataPostings {
		t.Fatalf("got.Hints[1] = %v, want FileDataPostings", got.Hints[1])
	}
	// Original must remain unchanged.
	if len(base.Hints) != 0 {
		t.Fatalf("base.Hints len = %d, want 0", len(base.Hints))
	}
}

func TestIOContext_WithHints_NoOpOnMerge(t *testing.T) {
	merge := NewMergeContext(&MergeInfo{TotalMaxDoc: 1})
	got := merge.WithHints(PreloadInstance)
	if len(got.Hints) != 0 {
		t.Fatalf("merge.WithHints should be a no-op, got %d hints", len(got.Hints))
	}
}

func TestIOContext_WithHints_NoOpOnFlush(t *testing.T) {
	flush := NewFlushContext(&FlushInfo{NumDocs: 1})
	got := flush.WithHints(PreloadInstance)
	if len(got.Hints) != 0 {
		t.Fatalf("flush.WithHints should be a no-op, got %d hints", len(got.Hints))
	}
}

func TestMergeInfo_MergeMaxNumSegments(t *testing.T) {
	// Both the canonical Lucene field name and the legacy alias must be
	// independently assignable so back-compat is preserved.
	info := MergeInfo{
		TotalMaxDoc:         42,
		EstimatedMergeBytes: 1024,
		IsExternal:          true,
		MergeMaxNumSegments: 3,
		MergeFactor:         3,
	}
	if info.MergeMaxNumSegments != info.MergeFactor {
		t.Fatalf("MergeMaxNumSegments=%d, MergeFactor=%d", info.MergeMaxNumSegments, info.MergeFactor)
	}
}

func TestFlushInfo_LuceneFields(t *testing.T) {
	// Lucene 10.4.0 record components: numDocs, estimatedSegmentSize.
	info := FlushInfo{NumDocs: 7, EstimatedSegmentSize: 8192}
	if info.NumDocs != 7 {
		t.Fatalf("NumDocs = %d, want 7", info.NumDocs)
	}
	if info.EstimatedSegmentSize != 8192 {
		t.Fatalf("EstimatedSegmentSize = %d, want 8192", info.EstimatedSegmentSize)
	}
}
