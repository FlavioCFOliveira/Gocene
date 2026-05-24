// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.index.TestMergeOnFlushMergePolicy.
package index

import (
	"testing"

	gindex "github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// makeSegmentWithSize builds a SegmentCommitInfo whose SizeInBytes() returns
// exactly sizeBytes by writing a single file of that size to a
// ByteBuffersDirectory.
func makeSegmentWithSize(t *testing.T, name string, sizeBytes int64) *gindex.SegmentCommitInfo {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput(name+".seg", store.IOContextDefault)
	if err != nil {
		t.Fatalf("makeSegmentWithSize: create output: %v", err)
	}
	// Write sizeBytes worth of zero bytes.
	buf := make([]byte, sizeBytes)
	if err := out.WriteBytes(buf); err != nil {
		t.Fatalf("makeSegmentWithSize: write bytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("makeSegmentWithSize: close: %v", err)
	}

	si := gindex.NewSegmentInfo(name, 10, dir)
	si.SetFiles([]string{name + ".seg"})
	sci := gindex.NewSegmentCommitInfo(si, 0, -1)
	return sci
}

// makeBaseMergeContext returns a BaseMergeContext with the given segments
// pre-populated as merging.
func makeBaseMergeContext(merging []*gindex.SegmentCommitInfo) *gindex.BaseMergeContext {
	ctx := gindex.NewBaseMergeContext()
	for _, seg := range merging {
		ctx.AddMergingSegment(seg)
	}
	return ctx
}

// TestMergeOnFlushMergePolicy_DefaultThreshold verifies the default threshold
// is 100 MB.
func TestMergeOnFlushMergePolicy_DefaultThreshold(t *testing.T) {
	p := NewMergeOnFlushMergePolicy(gindex.NewNRTMergePolicy())
	got := p.GetSmallSegmentThresholdMB()
	if got != 100.0 {
		t.Errorf("default threshold = %.1f MB; want 100.0 MB", got)
	}
}

// TestMergeOnFlushMergePolicy_SetGet verifies round-trip of the threshold.
func TestMergeOnFlushMergePolicy_SetGet(t *testing.T) {
	p := NewMergeOnFlushMergePolicy(gindex.NewNRTMergePolicy())
	p.SetSmallSegmentThresholdMB(50.0)
	got := p.GetSmallSegmentThresholdMB()
	if got != 50.0 {
		t.Errorf("threshold = %.1f MB; want 50.0 MB", got)
	}
}

// TestMergeOnFlushMergePolicy_Units verifies bytesToMB and mbToBytes are
// consistent inverses.
func TestMergeOnFlushMergePolicy_Units(t *testing.T) {
	const mb = 100.0
	bytes := MergeOnFlushUnits.MBToBytes(mb)
	back := MergeOnFlushUnits.BytesToMB(bytes)
	if back != mb {
		t.Errorf("BytesToMB(MBToBytes(%.1f)) = %.4f; want %.1f", mb, back, mb)
	}
}

// TestMergeOnFlushMergePolicy_NilWhenFewerThanTwoSmallSegments verifies that
// no merge is returned when there is only one non-merging small segment.
func TestMergeOnFlushMergePolicy_NilWhenFewerThanTwoSmallSegments(t *testing.T) {
	p := NewMergeOnFlushMergePolicy(gindex.NewNRTMergePolicy())
	p.SetSmallSegmentThresholdMB(10.0)
	thresholdBytes := MergeOnFlushUnits.MBToBytes(10.0)

	infos := gindex.NewSegmentInfos()
	// One small segment.
	infos.Add(makeSegmentWithSize(t, "_0", thresholdBytes-1))

	ctx := makeBaseMergeContext(nil)
	spec, err := p.FindFullFlushMerges(gindex.COMMIT, infos, ctx)
	if err != nil {
		t.Fatalf("FindFullFlushMerges: %v", err)
	}
	if spec != nil {
		t.Errorf("expected nil spec for single small segment, got %v", spec)
	}
}

// TestMergeOnFlushMergePolicy_TwoSmallSegmentsMerged verifies that two small
// segments produce a merge spec.
func TestMergeOnFlushMergePolicy_TwoSmallSegmentsMerged(t *testing.T) {
	p := NewMergeOnFlushMergePolicy(gindex.NewNRTMergePolicy())
	p.SetSmallSegmentThresholdMB(10.0)
	thresholdBytes := MergeOnFlushUnits.MBToBytes(10.0)

	seg0 := makeSegmentWithSize(t, "_0", thresholdBytes-1)
	seg1 := makeSegmentWithSize(t, "_1", thresholdBytes-1)

	infos := gindex.NewSegmentInfos()
	infos.Add(seg0)
	infos.Add(seg1)

	ctx := makeBaseMergeContext(nil)
	spec, err := p.FindFullFlushMerges(gindex.COMMIT, infos, ctx)
	if err != nil {
		t.Fatalf("FindFullFlushMerges: %v", err)
	}
	if spec == nil {
		t.Fatal("expected merge spec for two small segments, got nil")
	}
	if len(spec.Merges) != 1 {
		t.Fatalf("expected 1 merge, got %d", len(spec.Merges))
	}
	if len(spec.Merges[0].Segments) != 2 {
		t.Fatalf("expected 2 segments in merge, got %d", len(spec.Merges[0].Segments))
	}
}

// TestMergeOnFlushMergePolicy_LargeSegmentExcluded verifies that segments
// at or above the threshold are excluded.
func TestMergeOnFlushMergePolicy_LargeSegmentExcluded(t *testing.T) {
	p := NewMergeOnFlushMergePolicy(gindex.NewNRTMergePolicy())
	p.SetSmallSegmentThresholdMB(10.0)
	thresholdBytes := MergeOnFlushUnits.MBToBytes(10.0)

	// seg0 is large (exactly at threshold — not less than, so excluded).
	// seg1 and seg2 are small.
	seg0 := makeSegmentWithSize(t, "_0", thresholdBytes) // exactly at threshold: excluded
	seg1 := makeSegmentWithSize(t, "_1", thresholdBytes-1)
	seg2 := makeSegmentWithSize(t, "_2", thresholdBytes-1)

	infos := gindex.NewSegmentInfos()
	infos.Add(seg0)
	infos.Add(seg1)
	infos.Add(seg2)

	ctx := makeBaseMergeContext(nil)
	spec, err := p.FindFullFlushMerges(gindex.COMMIT, infos, ctx)
	if err != nil {
		t.Fatalf("FindFullFlushMerges: %v", err)
	}
	if spec == nil {
		t.Fatal("expected merge spec for two small segments")
	}
	// Merge must only contain small segments.
	for _, m := range spec.Merges {
		for _, seg := range m.Segments {
			if seg == seg0 {
				t.Errorf("large segment should not be included in merge")
			}
		}
	}
}

// TestMergeOnFlushMergePolicy_AlreadyMergingExcluded verifies that segments
// already participating in a merge are not selected.
func TestMergeOnFlushMergePolicy_AlreadyMergingExcluded(t *testing.T) {
	p := NewMergeOnFlushMergePolicy(gindex.NewNRTMergePolicy())
	p.SetSmallSegmentThresholdMB(10.0)
	thresholdBytes := MergeOnFlushUnits.MBToBytes(10.0)

	seg0 := makeSegmentWithSize(t, "_0", thresholdBytes-1)
	seg1 := makeSegmentWithSize(t, "_1", thresholdBytes-1)

	infos := gindex.NewSegmentInfos()
	infos.Add(seg0)
	infos.Add(seg1)

	// Mark both as already merging.
	ctx := makeBaseMergeContext([]*gindex.SegmentCommitInfo{seg0, seg1})
	spec, err := p.FindFullFlushMerges(gindex.COMMIT, infos, ctx)
	if err != nil {
		t.Fatalf("FindFullFlushMerges: %v", err)
	}
	if spec != nil {
		t.Errorf("expected nil spec when all small segments are already merging, got %v", spec)
	}
}

// TestMergeOnFlushMergePolicy_MixedMergingAndNonMerging verifies that only
// non-merging small segments are included.
func TestMergeOnFlushMergePolicy_MixedMergingAndNonMerging(t *testing.T) {
	p := NewMergeOnFlushMergePolicy(gindex.NewNRTMergePolicy())
	p.SetSmallSegmentThresholdMB(10.0)
	thresholdBytes := MergeOnFlushUnits.MBToBytes(10.0)

	seg0 := makeSegmentWithSize(t, "_0", thresholdBytes-1)
	seg1 := makeSegmentWithSize(t, "_1", thresholdBytes-1)
	seg2 := makeSegmentWithSize(t, "_2", thresholdBytes-1)

	infos := gindex.NewSegmentInfos()
	infos.Add(seg0)
	infos.Add(seg1)
	infos.Add(seg2)

	// Mark seg0 as already merging.
	ctx := makeBaseMergeContext([]*gindex.SegmentCommitInfo{seg0})
	spec, err := p.FindFullFlushMerges(gindex.COMMIT, infos, ctx)
	if err != nil {
		t.Fatalf("FindFullFlushMerges: %v", err)
	}
	if spec == nil {
		t.Fatal("expected merge spec for two non-merging small segments")
	}
	// Merged segments must not contain seg0.
	for _, m := range spec.Merges {
		for _, seg := range m.Segments {
			if seg == seg0 {
				t.Errorf("already-merging segment must not be included")
			}
		}
	}
}
