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

// crash_test.go ports org.apache.lucene.index.TestCrash (Sprint 55, option c).
//
// The Java test simulates abrupt machine crashes mid-indexing and asserts that
// the index stays openable and self-consistent afterwards. Its shared fixture
// builds an IndexWriter over a MockDirectoryWrapper (NoLockFactory) with
// setMaxBufferedDocs(10) and a ConcurrentMergeScheduler whose exceptions are
// suppressed, optionally commits once, then addDocument()s 157 identical docs.
// crash() drains the merge scheduler (cms.sync()), calls dir.crash() to drop
// all unsynced writes, drains again, then clearCrash(). It has five methods:
//
//   - testCrashWhileIndexing: crash with only the initial empty commit synced;
//     DirectoryReader.open must succeed and numDocs() < 157; then a fresh
//     Directory copied from the crashed one must accept a new RandomIndexWriter.
//   - testWriterAfterCrash: crash, reopen a second IndexWriter on the crashed
//     dir and add 157 more, close; numDocs() < 314 and the copied dir recovers.
//   - testCrashAfterReopen: close the first writer cleanly, reopen and add 157
//     more (getDocStats().maxDoc == 314), then crash; numDocs() >= 157.
//   - testCrashAfterClose: close the writer cleanly, then dir.crash();
//     numDocs() == 157 (a clean close synced everything).
//   - testCrashAfterCloseNoWait: same with commitOnClose=false plus an explicit
//     commit() before close; numDocs() == 157.
//
// Porting these assertions faithfully requires infrastructure that Gocene does
// not yet expose end-to-end:
//   - MockDirectoryWrapper.crash()/clearCrash() semantics. store.MockDirectory-
//     Wrapper models per-operation failure injection (SetFailOnCreateOutput,
//     SetRandomErrors, error rates) but has no crash()/clearCrash() that drops
//     all writes not yet sync()'d while leaving the directory reopenable; the
//     crash <  threshold assertions depend on exactly that lost-write model.
//   - A real Document/Field pipeline. index.Document is an opaque stub
//     (GetFields() []interface{}) and IndexWriter.AddDocument does not persist
//     field content, so a reader cannot observe a meaningful numDocs() and the
//     "fewer than 157/314" counting assertions cannot be made.
//   - ConcurrentMergeScheduler.sync()/setSuppressExceptions: the Gocene
//     ConcurrentMergeScheduler exposes neither, so crash() cannot quiesce
//     in-flight merges deterministically before dropping writes.
//   - DirectoryReader.numDocs() over a writer-produced index: OpenDirectory-
//     Reader exists, but IndexWriter does not yet flush postings/segments in a
//     form a DirectoryReader can count documents from.
//   - RandomIndexWriter and MockAnalyzer: not yet ported, so the post-crash
//     "writer recovers" step cannot be expressed.
//
// The five methods below preserve the upstream structure and are gated with
// t.Skip carrying the precise missing dependency, matching the established
// option-c pattern (see consistent_field_numbers_test.go, binary_terms_test.go).

const skipCrash = "GOC-4183: needs MockDirectoryWrapper.crash()/clearCrash() lost-write semantics, ConcurrentMergeScheduler.sync()/setSuppressExceptions, a real Document/Field pipeline, and DirectoryReader.numDocs() over a writer-produced index"

// TestCrash_WhileIndexing ports testCrashWhileIndexing.
func TestCrash_WhileIndexing(t *testing.T) {
	t.Skip(skipCrash)
}

// TestCrash_WriterAfterCrash ports testWriterAfterCrash.
func TestCrash_WriterAfterCrash(t *testing.T) {
	t.Skip(skipCrash)
}

// TestCrash_AfterReopen ports testCrashAfterReopen.
func TestCrash_AfterReopen(t *testing.T) {
	t.Skip(skipCrash)
}

// TestCrash_AfterClose ports testCrashAfterClose.
func TestCrash_AfterClose(t *testing.T) {
	t.Skip(skipCrash)
}

// TestCrash_AfterCloseNoWait ports testCrashAfterCloseNoWait.
func TestCrash_AfterCloseNoWait(t *testing.T) {
	t.Skip(skipCrash)
}
