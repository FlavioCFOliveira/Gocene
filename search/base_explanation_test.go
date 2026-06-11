// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Shared harness for the explanation test suites ported from Apache Lucene
// 10.4.0's BaseExplanationTestCase.
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/search/BaseExplanationTestCase.java
//
// The Lucene base class indexes four documents into FIELD/ALTFIELD (text) and
// KEY (string + sorted doc-values), then exposes qtest/bqtest plus the
// query-construction macros (optB, reqB, matchTheseItems, ta) used by the
// TestSimpleExplanations / TestComplexExplanations subclasses.
//
// qtest is a faithful port: it optionally wraps the query in a BooleanQuery
// with a never-matching SHOULD clause (random, like the Java original), then
// runs CheckHits.checkHitCollector. Because Gocene's CheckHitCollector ports
// only the set-collector equality half of Lucene's checkHitCollector (the
// other half, QueryUtils.check, validates explanations), qtest additionally
// runs CheckHits.checkExplanations so the explanation contract these suites
// exist to verify is actually exercised.
package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/testutil"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Register the production codec so postings / doc-values are flushed.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

const (
	// explKey is the BaseExplanationTestCase KEY field (string + sorted DV).
	explKey = "KEY"
	// explField is the boosted text field used by most assertions.
	explField = "field"
	// explAltField holds the same content with no field boost.
	explAltField = "alt"
)

// explDocFields mirrors BaseExplanationTestCase.docFields exactly.
var explDocFields = []string{
	"w1 w2 w3 w4 w5",
	"w1 w3 w2 w3 zz",
	"w1 xx w2 yy w3",
	"w1 w3 xx w2 yy w3 zz",
}

// explanationTestCase bundles the searcher + cleanup for one explanation suite
// run, mirroring the static searcher/reader/directory of the Java base class
// (Go test isolation favours per-test setup over @BeforeClass statics).
type explanationTestCase struct {
	t        *testing.T
	rng      *rand.Rand
	searcher *search.IndexSearcher
	cleanup  func()
	// nonMatches selects the TestSimpleExplanationsOfNonMatches behaviour: when
	// true, qtest verifies the explanations of the NON-matching documents
	// (CheckHits.checkNoMatchExplanations) instead of the matching ones.
	nonMatches bool
}

// newExplanationTestCase builds the four-document index and returns a harness.
// The seed is fixed so the random qtest BooleanQuery wrapping is reproducible.
func newExplanationTestCase(t *testing.T) *explanationTestCase {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := range explDocFields {
		if addErr := w.AddDocument(createExplDoc(t, i)); addErr != nil {
			t.Fatalf("AddDocument(%d): %v", i, addErr)
		}
	}
	if err = w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return &explanationTestCase{
		t:        t,
		rng:      rand.New(rand.NewSource(int64(len(explDocFields)))), //nolint:gosec // deterministic test seed
		searcher: search.NewIndexSearcher(reader),
		cleanup: func() {
			_ = reader.Close()
			_ = dir.Close()
		},
	}

// createExplDoc mirrors BaseExplanationTestCase.createDoc(index).
func createExplDoc(t *testing.T, idx int) *document.Document {
	t.Helper()
	doc := document.NewDocument()

	key, err := document.NewStringField(explKey, intToStr(idx), false)
	if err != nil {
		t.Fatalf("NewStringField(KEY): %v", err)
	}
	doc.Add(key)

	dv, err := document.NewSortedDocValuesField(explKey, []byte(intToStr(idx)))
	if err != nil {
		t.Fatalf("NewSortedDocValuesField(KEY): %v", err)
	}
	doc.Add(dv)

	f, err := document.NewTextField(explField, explDocFields[idx], false)
	if err != nil {
		t.Fatalf("NewTextField(field): %v", err)
	}
	doc.Add(f)

	alt, err := document.NewTextField(explAltField, explDocFields[idx], false)
	if err != nil {
		t.Fatalf("NewTextField(alt): %v", err)
	}
	doc.Add(alt)

	return doc
}

// qtest mirrors BaseExplanationTestCase.qtest: it checks that the expected docs
// match and that their scores agree with the explanations. The query may be
// randomly wrapped in a BooleanQuery alongside a never-matching term, exactly
// like the Java original.
func (tc *explanationTestCase) qtest(q search.Query, expDocNrs []int) {
	tc.t.Helper()
	if tc.nonMatches {
		// TestSimpleExplanationsOfNonMatches overrides qtest to verify that the
		// explanation for every NON-matching document is a non-match.
		testutil.CheckNoMatchExplanations(tc.t, q, explField, tc.searcher, expDocNrs)
		return
	}
	if tc.rng.Intn(2) == 0 {
		bq := search.NewBooleanQuery()
		bq.Add(q, search.SHOULD)
		bq.Add(search.NewTermQuery(index.NewTerm("NEVER", "MATCH")), search.SHOULD)
		q = bq
	}
	testutil.CheckHitCollector(tc.t, q, explField, tc.searcher, expDocNrs)
	// QueryUtils.check (run by Lucene's checkHitCollector) validates the
	// explanation tree; Gocene's CheckHitCollector does not, so do it here.
	testutil.CheckExplanations(tc.t, q, explField, tc.searcher, true)
}

// bqtest mirrors BaseExplanationTestCase.bqtest: qtest the query wrapped both
// as required (reqB) and as optional (optB).
func (tc *explanationTestCase) bqtest(q search.Query, expDocNrs []int) {
	tc.t.Helper()
	tc.qtest(tc.reqB(q), expDocNrs)
	tc.qtest(tc.optB(q), expDocNrs)
}

// matchTheseItems mirrors BaseExplanationTestCase.matchTheseItems: a SHOULD
// BooleanQuery over the KEY field for each supplied document number.
func (tc *explanationTestCase) matchTheseItems(terms []int) search.Query {
	query := search.NewBooleanQuery()
	for _, term := range terms {
		query.Add(search.NewTermQuery(index.NewTerm(explKey, intToStr(term))), search.SHOULD)
	}
	return query
}

// optB mirrors BaseExplanationTestCase.optB: wrap q as SHOULD alongside a
// never-matching MUST_NOT clause.
func (tc *explanationTestCase) optB(q search.Query) search.Query {
	bq := search.NewBooleanQuery()
	bq.Add(q, search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("NEVER", "MATCH")), search.MUST_NOT)
	return bq
}

// reqB mirrors BaseExplanationTestCase.reqB: wrap q as MUST alongside a SHOULD
// clause that matches every document (FIELD:w1).
func (tc *explanationTestCase) reqB(q search.Query) search.Query {
	bq := search.NewBooleanQuery()
	bq.Add(q, search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm(explField, "w1")), search.SHOULD)
	return bq
}

// explTerms mirrors BaseExplanationTestCase.ta: build FIELD terms from strings.
func explTerms(words []string) []*index.Term {
	t := make([]*index.Term, len(words))
	for i, w := range words {
		t[i] = index.NewTerm(explField, w)
	}
	return t
}

// intToStr renders an int the way "" + index does in Java.
func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}