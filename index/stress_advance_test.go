// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains a port of Lucene's TestStressAdvance.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestStressAdvance
// Source: lucene/core/src/test/org/apache/lucene/index/TestStressAdvance.java
//
// GOC-4229: Port TestStressAdvance.
//
// This stress test indexes at least 4097 documents carrying either an "a" or
// a "b" token, force-merges to a single segment, and then repeatedly seeks
// each term and exercises PostingsEnum.NextDoc / Advance against the known
// expected doc-ID lists.
//
// Sprint 55 option (c): the test methods are structured to mirror the Java
// original, but the bodies are skipped because the supporting infrastructure
// is not yet available in Gocene:
//
//   - RandomIndexWriter (random-config writer used to add documents and
//     force-merge).
//   - IndexWriter.getReader (NRT reader obtained directly from the writer).
//   - getOnlyLeafReader (LuceneTestCase single-leaf accessor) plus reader-side
//     term/posting traversal: TermsEnum/PostingsEnum obtained from a reader
//     opened via OpenDirectoryReader currently fail because such readers carry
//     no core readers.
//   - TestUtil.docs (the random PostingsEnum reuse helper).
//
// Each skipped test names the exact missing dependency so the work can be
// picked up once those pieces land.
package index_test

import "testing"

// testStressAdvanceSkip marks a test as blocked and records the missing
// infrastructure. Centralised so the reason stays consistent across methods.
func testStressAdvanceSkip(t *testing.T, missing string) {
	t.Helper()
	t.Fatalf("GOC-4229: TestStressAdvance port blocked - missing %s", missing)
}

// TestStressAdvance_StressAdvance ports testStressAdvance.
//
// Java: indexes atLeast(4097) docs each tagged "a" or "b", force-merges to a
// single segment, builds the expected aDocIDs/bDocIDs lists from stored "id"
// fields, then runs 10 iterations seeking each term and verifying NextDoc and
// Advance via testOne.
func TestStressAdvance_StressAdvance(t *testing.T) {
	testStressAdvanceSkip(t, "RandomIndexWriter, IndexWriter.getReader (NRT reader), getOnlyLeafReader and TestUtil.docs")

	// Intended port (pseudo-flow):
	//   for iter := 0; iter < numIters; iter++ {
	//       dir := newDirectory(t)
	//       w := newRandomIndexWriter(t, dir)
	//       aDocs := map[int]bool{}
	//       num := atLeast(t, 4097)
	//       for id := 0; id < num; id++ {
	//           if random.Intn(4) == 3 { f.SetStringValue("a"); aDocs[id] = true }
	//           else { f.SetStringValue("b") }
	//           idField.SetStringValue(strconv.Itoa(id))
	//           w.AddDocument(doc)
	//       }
	//       w.ForceMerge(1)
	//       r := w.GetReader()
	//       idToDocID := make([]int, r.MaxDoc())
	//       build aDocIDs / bDocIDs from stored "id" field
	//       te := getOnlyLeafReader(r).Terms("field").Iterator()
	//       for iter2 := 0; iter2 < 10; iter2++ {
	//           assertEquals(SeekStatusFound, te.SeekCeil(bytesRef("a")))
	//           de = testUtilDocs(random, te, de, PostingsEnumNone)
	//           testStressAdvanceTestOne(t, de, aDocIDs)
	//           assertEquals(SeekStatusFound, te.SeekCeil(bytesRef("b")))
	//           de = testUtilDocs(random, te, de, PostingsEnumNone)
	//           testStressAdvanceTestOne(t, de, bDocIDs)
	//       }
	//       w.Close(); r.Close(); dir.Close()
	//   }
}

// testStressAdvanceTestOne ports the private testOne helper.
//
// Java: walks the expected doc-ID list, randomly choosing NextDoc or Advance
// (always NextDoc on the final element), and asserts the returned doc ID
// equals the expected one, ending with NO_MORE_DOCS once the list is consumed.
//
// Skipped for the same reason as the parent: it consumes a PostingsEnum that
// cannot yet be obtained from a reader opened via OpenDirectoryReader.
func testStressAdvanceTestOne(t *testing.T, expectedLen int) {
	testStressAdvanceSkip(t, "PostingsEnum from getOnlyLeafReader-backed TermsEnum")

	// Intended port (pseudo-flow):
	//   upto := -1
	//   for upto < expectedLen {
	//       var docID int
	//       if random.Intn(4) == 1 || upto == expectedLen-1 {
	//           upto++
	//           docID = docs.NextDoc()
	//       } else {
	//           inc := nextInt(random, 1, expectedLen-1-upto)
	//           upto += inc
	//           docID = docs.Advance(expected[upto])
	//       }
	//       if upto == expectedLen {
	//           assertEquals(NoMoreDocs, docID)
	//       } else {
	//           assertTrue(docID != NoMoreDocs)
	//           assertEquals(expected[upto], docID)
	//       }
	//   }
}
