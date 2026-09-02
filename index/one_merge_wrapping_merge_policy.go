// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// OneMergeWrappingMergePolicy decorates each OneMerge produced by the wrapped
// merge policy. Mirrors
// org.apache.lucene.index.OneMergeWrappingMergePolicy from Apache Lucene 10.4.0.
//
// The WrapOneMerge function is invoked once per OneMerge returned by the
// wrapped policy's FindMerges / FindForcedMerges / FindForcedDeletesMerges
// (and FindFullFlushMerges, when ported). It must return a non-nil OneMerge.
type OneMergeWrappingMergePolicy struct {
	*FilterMergePolicy
	WrapOneMerge func(*OneMerge) *OneMerge
}

// NewOneMergeWrappingMergePolicy wraps in and uses wrap to transform every
// OneMerge in the produced specifications.
func NewOneMergeWrappingMergePolicy(in MergePolicy, wrap func(*OneMerge) *OneMerge) *OneMergeWrappingMergePolicy {
	if wrap == nil {
		panic("OneMergeWrappingMergePolicy: wrap must not be nil")
	}
	return &OneMergeWrappingMergePolicy{
		FilterMergePolicy: NewFilterMergePolicy(in),
		WrapOneMerge:      wrap,
	}
}

// FindMerges wraps the OneMerges produced by the underlying policy.
func (w *OneMergeWrappingMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mc MergeContext) (*MergeSpecification, error) {
	spec, err := w.In.FindMerges(trigger, infos, mc)
	if err != nil {
		return nil, err
	}
	return w.wrapSpec(spec), nil
}

// FindForcedMerges wraps the OneMerges produced by the underlying policy.
func (w *OneMergeWrappingMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segs map[*SegmentCommitInfo]bool, mc MergeContext) (*MergeSpecification, error) {
	spec, err := w.In.FindForcedMerges(infos, maxSegmentCount, segs, mc)
	if err != nil {
		return nil, err
	}
	return w.wrapSpec(spec), nil
}

// FindForcedDeletesMerges wraps the OneMerges produced by the underlying policy.
func (w *OneMergeWrappingMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos, mc MergeContext) (*MergeSpecification, error) {
	spec, err := w.In.FindForcedDeletesMerges(infos, mc)
	if err != nil {
		return nil, err
	}
	return w.wrapSpec(spec), nil
}

func (w *OneMergeWrappingMergePolicy) wrapSpec(spec *MergeSpecification) *MergeSpecification {
	if spec == nil {
		return nil
	}
	wrapped := NewMergeSpecification()
	for _, m := range spec.Merges {
		wrapped.Add(w.WrapOneMerge(m))
	}
	return wrapped
}
