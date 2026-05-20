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

// doc_count_test.go ports org.apache.lucene.index.TestDocCount, which exercises
// the Terms.getDocCount() statistic.
//
// The Java test indexes atLeast(100) randomized documents, opens a reader,
// and for every indexed field iterates the term postings while marking each
// visited doc in a FixedBitSet; it then asserts the bitset cardinality equals
// terms.getDocCount(). It repeats the check after forceMerge(1).
//
// Porting the assertion requires three pieces of infrastructure that Gocene
// does not yet provide:
//   - A RandomIndexWriter equivalent (randomized add/commit/merge driver).
//   - FieldInfos.getIndexedFields and MultiTerms.getTerms helpers to enumerate
//     indexed fields and obtain merged Terms across leaves.
//   - Postings read-back, which currently fails with "core readers are nil"
//     because OpenDirectoryReader builds leaves via NewSegmentReader without
//     coreReaders.

// TestDocCount_Simple ports testSimple.
func TestDocCount_Simple(t *testing.T) {
	t.Skip("GOC-4140: needs RandomIndexWriter plus MultiTerms.getTerms / postings read-back (SegmentReader coreReaders gap)")
}
