// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

func TestNoDeletionPolicy_NoOps(t *testing.T) {
	p := NoDeletionPolicyInstance
	if err := p.OnInit(nil); err != nil {
		t.Errorf("OnInit error: %v", err)
	}
	if err := p.OnCommit(nil); err != nil {
		t.Errorf("OnCommit error: %v", err)
	}
	if p.Clone() != p {
		t.Errorf("Clone should return singleton")
	}
}

// fakeMergePolicy is a minimal MergePolicy that records call counts.
type fakeMergePolicy struct {
	findMergesCalls int
}

func (f *fakeMergePolicy) FindMerges(_ MergeTrigger, _ *SegmentInfos, _ MergeContext) (*MergeSpecification, error) {
	f.findMergesCalls++
	spec := NewMergeSpecification()
	spec.Add(&OneMerge{})
	return spec, nil
}
func (f *fakeMergePolicy) FindForcedMerges(_ *SegmentInfos, _ int, _ map[*SegmentCommitInfo]bool, _ MergeContext) (*MergeSpecification, error) {
	return nil, nil
}
func (f *fakeMergePolicy) FindForcedDeletesMerges(_ *SegmentInfos, _ MergeContext) (*MergeSpecification, error) {
	return nil, nil
}
func (f *fakeMergePolicy) UseCompoundFile(_ *SegmentInfos, _ *SegmentInfo) bool          { return false }
func (f *fakeMergePolicy) GetMaxMergeDocs() int                                          { return 100 }
func (f *fakeMergePolicy) SetMaxMergeDocs(_ int)                                         {}
func (f *fakeMergePolicy) GetMaxMergedSegmentBytes() int64                               { return 5 * 1024 * 1024 * 1024 }
func (f *fakeMergePolicy) SetMaxMergedSegmentBytes(_ int64)                              {}
func (f *fakeMergePolicy) NumDeletesToMerge(_ *SegmentCommitInfo, c int) int             { return c }
func (f *fakeMergePolicy) KeepFullyDeletedSegment(_ *SegmentCommitInfo) bool             { return false }

func TestFilterMergePolicy_PassThrough(t *testing.T) {
	in := &fakeMergePolicy{}
	fp := NewFilterMergePolicy(in)
	if fp.Unwrap() != in {
		t.Errorf("Unwrap mismatch")
	}
	spec, err := fp.FindMerges(SEGMENT_FLUSH, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if in.findMergesCalls != 1 {
		t.Errorf("FindMerges not delegated")
	}
	if spec == nil || spec.Size() != 1 {
		t.Errorf("spec mismatch: %v", spec)
	}
	if fp.GetMaxMergeDocs() != 100 {
		t.Errorf("GetMaxMergeDocs mismatch")
	}
}

func TestFilterMergePolicy_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil 'in'")
		}
	}()
	_ = NewFilterMergePolicy(nil)
}

func TestOneMergeWrappingMergePolicy_WrapsEachOneMerge(t *testing.T) {
	in := &fakeMergePolicy{}
	calls := 0
	w := NewOneMergeWrappingMergePolicy(in, func(m *OneMerge) *OneMerge {
		calls++
		m.MergeGen = 42
		return m
	})
	spec, err := w.FindMerges(SEGMENT_FLUSH, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("wrap calls = %d, want 1", calls)
	}
	if spec.Merges[0].MergeGen != 42 {
		t.Errorf("wrap not applied: MergeGen=%d", spec.Merges[0].MergeGen)
	}
}

func TestOneMergeWrappingMergePolicy_NilWrapPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on nil wrap")
		}
	}()
	_ = NewOneMergeWrappingMergePolicy(&fakeMergePolicy{}, nil)
}
