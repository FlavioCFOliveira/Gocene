// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// ForceMergePolicy is a merge policy that disallows background merges.
// It only allows forced merges (optimize/forceMerge).
//
// This is the Go port of Lucene's org.apache.lucene.tests.index.ForceMergePolicy.
type ForceMergePolicy struct {
	*FilterMergePolicy
}

// NewForceMergePolicy creates a new ForceMergePolicy wrapping another.
func NewForceMergePolicy(in MergePolicy) *ForceMergePolicy {
	return &ForceMergePolicy{
		FilterMergePolicy: NewFilterMergePolicy(in),
	}
}

// FindMerges returns nil to disallow any background merges.
func (f *ForceMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mc MergeContext) (*MergeSpecification, error) {
	return nil, nil
}
