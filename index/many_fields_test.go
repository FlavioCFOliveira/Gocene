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

package index_test

import "testing"

// many_fields_test.go ports org.apache.lucene.index.TestManyFields
// (Lucene 10.4.0, core/src/test/org/apache/lucene/index/TestManyFields.java)
// into the Gocene tree, under GOC-4186 / Sprint 55 (option c).
//
// The Java suite "creates way, way, way too many fields" and has three
// test methods:
//
//   - testManyFields: writes 100 docs, each with six fields (a..f j-indexed),
//     then reopens the index and asserts maxDoc/numDocs == 100 and that every
//     term has docFreq == 1.
//   - testDiverseDocs: writes a stress mix of unique-term, single-term, and
//     very-long-term docs, then asserts a TermQuery for "aaa" counts n*100.
//   - testRotatingFieldNames (LUCENE-4398): repeatedly fills the RAM buffer
//     with ten fresh field names per doc, recycling field names past upto 5000
//     to avoid unbounded FieldInfo growth, and asserts each segment flushes
//     after a doc count within 90% of the first segment's.
//
// All three methods are gated with t.Skip carrying the precise missing
// dependency, matching the established option-c pattern (see
// binary_terms_test.go and consistent_field_numbers_test.go). Three distinct
// infrastructure gaps block a faithful port today:
//
//   - DirectoryReader exposes no DocFreq: index.IndexReader.docFreq has no
//     counterpart on the reopened-reader path (index/directory_reader.go),
//     so testManyFields' per-term docFreq assertions cannot be expressed.
//   - OpenDirectoryReader materialises each segment via NewSegmentReader
//     (index/directory_reader.go:462/497), leaving SegmentReader.coreReaders
//     nil; term lookups therefore match no documents, so testDiverseDocs'
//     TermQuery count would be 0 regardless.
//   - IndexWriter.GetFlushCount is a placeholder hardcoded to return 0
//     (index/index_writer.go:412), so testRotatingFieldNames' driving loop
//     `for w.GetFlushCount() == startFlushCount` would never terminate.
//
// Unskip each method once its dependency lands.

const skipManyFieldsTermStats = "GOC-4186: DirectoryReader has no DocFreq counterpart for IndexReader.docFreq, and OpenDirectoryReader builds SegmentReader without core readers (index/directory_reader.go:462/497) so TermQuery matches no documents"

const skipManyFieldsFlushCount = "GOC-4186: IndexWriter.GetFlushCount is a placeholder returning 0 (index/index_writer.go:412); the flush-driven loop would never terminate"

// TestManyFields_ManyFields ports testManyFields: it would index 100 docs of
// six fields each and assert maxDoc/numDocs == 100 plus docFreq == 1 per term.
func TestManyFields_ManyFields(t *testing.T) {
	t.Skip(skipManyFieldsTermStats)
}

// TestManyFields_DiverseDocs ports testDiverseDocs: it would index a stress
// mix of unique-term, single-term, and very-long-term docs and assert a
// TermQuery for "field:aaa" counts n*100.
func TestManyFields_DiverseDocs(t *testing.T) {
	t.Skip(skipManyFieldsTermStats)
}

// TestManyFields_RotatingFieldNames ports testRotatingFieldNames (LUCENE-4398):
// it would repeatedly fill the RAM buffer with ten fresh field names per doc,
// recycling field names past upto 5000, and assert each segment flushes after
// a doc count within 90% of the first segment's.
func TestManyFields_RotatingFieldNames(t *testing.T) {
	t.Skip(skipManyFieldsFlushCount)
}
