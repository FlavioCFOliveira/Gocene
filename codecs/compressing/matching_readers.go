// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package compressing

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// MergeState is a minimal placeholder of
// org.apache.lucene.index.MergeState capturing only the fields needed by
// [MatchingReaders]. The full MergeState wiring (max-docs arrays,
// SegmentReaders, doc-maps, info streams, ...) is deferred to the index
// core sprint (Sprint 22) where the canonical type will live in package
// index. Once that lands, the bulk-merge call sites in this package will
// migrate to the canonical type and this placeholder will be removed.
//
// The placeholder exists so that MatchingReaders can be ported now with a
// stable, exported public surface that mirrors the Lucene reference, even
// though the only caller in flight today is the test in this same package.
type MergeState struct {
	// MaxDocs has one entry per source reader; only its length is
	// observed by MatchingReaders. Production code populates it with the
	// per-reader maxDoc counts.
	MaxDocs []int

	// FieldInfos is the per-reader FieldInfos snapshot. Index i
	// corresponds to source reader i; len(FieldInfos) must equal
	// len(MaxDocs).
	FieldInfos []*index.FieldInfos

	// MergeFieldInfos is the merged FieldInfos of the destination
	// segment, against which every source reader's field name->number
	// mapping is compared.
	MergeFieldInfos *index.FieldInfos
}

// MatchingReaders computes which source readers in a MergeState share the
// destination segment's field name <-> number mapping. When every field
// of a reader maps identically, that reader's stored fields and term
// vectors can be merged with a fast bulk-copy path; otherwise a per-
// document re-mapping is required.
//
// This is the Go port of org.apache.lucene.codecs.compressing.MatchingReaders.
//
// The Java reference also emits a diagnostic line to MergeState.infoStream
// when the "SM" category is enabled; the Go port omits this until the
// index core sprint (Sprint 22) provides the canonical MergeState with
// its InfoStream field.
type MatchingReaders struct {
	// MatchingReaders[i] is true when source reader i can be bulk-merged.
	MatchingReaders []bool

	// Count is the number of true entries in MatchingReaders.
	Count int
}

// NewMatchingReaders builds a MatchingReaders for the given mergeState.
// Mirrors the constructor MatchingReaders(MergeState) in the Java
// reference (Lucene 10.4.0, MatchingReaders.java:38..71). The decision is
// made per source reader: if every FieldInfo in that reader's FieldInfos
// maps to a FieldInfo in mergeState.MergeFieldInfos with the same name
// for the same field number, the reader is marked as matching.
//
// A nil mergeState, a nil MergeFieldInfos, or a mergeState with no
// readers is permitted: the result is a MatchingReaders with an empty
// MatchingReaders slice and Count == 0.
func NewMatchingReaders(mergeState *MergeState) *MatchingReaders {
	if mergeState == nil {
		return &MatchingReaders{MatchingReaders: nil, Count: 0}
	}
	numReaders := len(mergeState.MaxDocs)
	matching := make([]bool, numReaders)
	matched := 0

	merged := mergeState.MergeFieldInfos
	for i := 0; i < numReaders; i++ {
		// Guard against under-sized FieldInfos slices: in the canonical
		// Java code the arrays are always parallel; the Go port accepts
		// the same invariant but treats a missing entry as a non-match
		// rather than panicking, which is safer in the placeholder phase.
		if merged == nil || i >= len(mergeState.FieldInfos) || mergeState.FieldInfos[i] == nil {
			continue
		}
		fis := mergeState.FieldInfos[i]
		if !readerMatchesMerged(fis, merged) {
			continue
		}
		matching[i] = true
		matched++
	}

	return &MatchingReaders{MatchingReaders: matching, Count: matched}
}

// readerMatchesMerged returns true when every FieldInfo in fis maps to a
// FieldInfo of the same name (for the same field number) in merged.
// Returns at the first mismatch — mirroring the "continue nextReader"
// label in the Java source.
func readerMatchesMerged(fis, merged *index.FieldInfos) bool {
	it := fis.Iterator()
	for fi := it.Next(); fi != nil; fi = it.Next() {
		other := merged.GetByNumber(fi.Number())
		if other == nil || other.Name() != fi.Name() {
			return false
		}
	}
	return true
}
