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

// sum_doc_freq_test.go ports org.apache.lucene.index.TestSumDocFreq, which
// exercises the Terms.getSumDocFreq() statistic.
//
// The Java test indexes atLeast(500) randomized documents across two text
// fields, opens a reader, and for every indexed field iterates the TermsEnum
// summing docFreq(); it asserts that sum equals terms.getSumDocFreq(). It then
// applies atLeast(20) randomized deletions, forceMerge(1)s, and repeats.
//
// Porting the assertion requires three pieces of infrastructure that Gocene
// does not yet provide:
//   - A RandomIndexWriter equivalent (randomized add/delete/commit/merge driver).
//   - FieldInfos.getIndexedFields and MultiTerms.getTerms helpers to enumerate
//     indexed fields and obtain merged Terms across leaves.
//   - Postings read-back, which currently fails with "core readers are nil"
//     because OpenDirectoryReader builds leaves via NewSegmentReader without
//     coreReaders.

// TestSumDocFreq ports testSumDocFreq.
func TestSumDocFreq(t *testing.T) {
	t.Skip("GOC-4173: needs RandomIndexWriter plus MultiTerms.getTerms / postings read-back (SegmentReader coreReaders gap)")
}
