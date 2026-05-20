// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import "testing"

// consistent_field_numbers_test.go ports
// org.apache.lucene.index.TestConsistentFieldNumbers (Sprint 55, option c).
//
// The Java test verifies that field numbers assigned by IndexWriter stay
// stable across segments, across IndexWriter sessions, and across addIndexes
// and merge operations, and that gaps left by deleted fields are preserved
// until a merge can reclaim them. It contains four test methods:
//
//   - testSameFieldNumbersAcrossSegments: writes two segments (the second
//     either after commit() or after a writer reopen) and asserts f1/f2 keep
//     numbers 0/1 in both, with f3/f4 taking 2/3 in the second; a forceMerge(1)
//     then collapses them while keeping 0..3 stable.
//   - testAddIndexes: builds an "external" index and addIndexes() it into a
//     target, asserting the external segment's own field ordering (f2,f1,f3,f4)
//     is preserved rather than renumbered to match the target.
//   - testFieldNumberGaps: across atLeast(13) iterations, indexes segments
//     whose field sets differ so that segment 2 has a null at slot 1, then
//     deletes/forceMergeDeletes the first segment and forceMerge(1)s the rest,
//     asserting the gap is closed to f1/f2/f3 = 0/1/2.
//   - testManyFields: indexes atLeast(200) docs with atLeast(50) randomized
//     fields drawn from 16 distinct FieldType configurations, forceMerge(1)s,
//     and asserts every persisted FieldInfo's indexOptions and term-vector
//     flags match the FieldType that produced it.
//
// Porting these assertions faithfully requires infrastructure that Gocene
// does not yet expose end-to-end:
//   - A real Document / Field pipeline. index.Document is currently an opaque
//     stub (GetFields() []interface{}) and IndexWriter.AddDocument does not
//     persist field names, FieldType options, or term-vector flags, so the
//     name/number and indexOptions/hasTermVectors assertions cannot be made.
//   - IndexWriter.readFieldInfos(SegmentCommitInfo): no equivalent of the Java
//     static IndexWriter.readFieldInfos exists; only ReadSegmentInfos is
//     available, and there is no per-segment FieldInfos read-back.
//   - SegmentInfos.readLatestCommit: ReadSegmentInfos exists but the writer
//     does not yet flush field-numbering state into committed segments in a
//     form these tests can inspect.
//   - MockAnalyzer, StoredField, FieldType, and FailOnNonBulkMergesInfoStream
//     are not yet ported.
//
// The four methods below preserve the upstream structure and are gated with
// t.Skip carrying the precise missing dependency, matching the established
// option-c pattern (see doc_count_test.go, flex_test.go, binary_terms_test.go).

const skipConsistentFieldNumbers = "GOC-4174: needs a real Document/Field pipeline plus IndexWriter.readFieldInfos and SegmentInfos.readLatestCommit; IndexWriter.AddDocument does not yet persist field names, FieldType options, or term-vector flags"

// TestConsistentFieldNumbers_SameFieldNumbersAcrossSegments ports
// testSameFieldNumbersAcrossSegments.
func TestConsistentFieldNumbers_SameFieldNumbersAcrossSegments(t *testing.T) {
	t.Skip(skipConsistentFieldNumbers)
}

// TestConsistentFieldNumbers_AddIndexes ports testAddIndexes.
func TestConsistentFieldNumbers_AddIndexes(t *testing.T) {
	t.Skip(skipConsistentFieldNumbers)
}

// TestConsistentFieldNumbers_FieldNumberGaps ports testFieldNumberGaps.
func TestConsistentFieldNumbers_FieldNumberGaps(t *testing.T) {
	t.Skip(skipConsistentFieldNumbers)
}

// TestConsistentFieldNumbers_ManyFields ports testManyFields.
func TestConsistentFieldNumbers_ManyFields(t *testing.T) {
	t.Skip(skipConsistentFieldNumbers)
}
