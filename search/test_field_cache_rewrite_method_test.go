// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestFieldCacheRewriteMethod.java
//   (extends TestRegexpRandom2)
//
// TestFieldCacheRewriteMethod indexes random terms into a field that is BOTH an
// indexed StringField and a SortedDocValuesField (inherited from
// TestRegexpRandom2.setUp), then for many random regexps asserts that a
// RegexpQuery rewritten via DocValuesRewriteMethod returns exactly the same hit
// set as the same RegexpQuery rewritten via the postings-based
// CONSTANT_SCORE_REWRITE and CONSTANT_SCORE_BLENDED_REWRITE methods
// (CheckHits.checkEqual on the score docs).
//
// In Gocene, the DocValues-rewrite scoring path requires the field's
// SortedSetDocValues to expose an ordinal-aware TermsEnum
// (SortedSetDocValuesWithTermsEnum + TermsEnumWithOrd) and the multi-term query
// to supply a term-filtering TermsEnum (MultiTermQueryTermsEnumProvider); none
// of those optional interfaces is satisfied by the production codec's
// SortedDocValues nor by the regexp query, so DocValuesRewriteMethod produces a
// query that matches zero documents. A faithful equality assertion against the
// (working) postings RegexpQuery therefore cannot hold.
//
// This port builds the real index and the real postings reference, computes the
// reference hit set, then drives the DocValuesRewriteMethod path and fails
// honestly when it cannot reproduce that hit set, citing the concrete missing
// wiring (rather than skipping or weakening the assertion).

package search_test

import (
	"math/rand"
	"regexp"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const fieldCacheRewriteField = "field"

// buildFieldCacheRewriteIndex indexes random short terms into both a StringField
// and a SortedDocValuesField on the same field, mirroring
// TestRegexpRandom2.setUp (the parent of TestFieldCacheRewriteMethod).
func buildFieldCacheRewriteIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()))) //nolint:gosec // deterministic test seed
	ix := newIntegrationIndex(t)
	const num = 200
	for i := 0; i < num; i++ {
		s := regexp2RandomTerm(rng)
		doc := document.NewDocument()
		sf, err := document.NewStringField(fieldCacheRewriteField, s, false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)
		dv, err := document.NewSortedDocValuesField(fieldCacheRewriteField, []byte(s))
		if err != nil {
			t.Fatalf("NewSortedDocValuesField: %v", err)
		}
		doc.Add(dv)
		ix.addDoc(doc)
	}
	return ix.searcher()
}

// fieldCacheRewriteReferenceHits runs the postings-based RegexpQuery (the
// analogue of the CONSTANT_SCORE_REWRITE filter the reference compares against)
// and returns the matched doc IDs as a set.
func fieldCacheRewriteReferenceHits(t *testing.T, s *search.IndexSearcher, reg string) (map[int]struct{}, bool) {
	t.Helper()
	if _, rerr := regexp.Compile("^(?:" + reg + ")$"); rerr != nil {
		return nil, false // invalid pattern: skipped identically on both sides
	}
	q, err := search.NewRegexpQuery(fieldCacheRewriteField, reg)
	if err != nil {
		return nil, false
	}
	top, err := s.Search(q, 25)
	if err != nil {
		t.Fatalf("postings RegexpQuery search %q: %v", reg, err)
	}
	hits := make(map[int]struct{}, len(top.ScoreDocs))
	for _, sd := range top.ScoreDocs {
		hits[sd.Doc] = struct{}{}
	}
	return hits, true
}

// fieldCacheRewriteDocValuesHits builds the DocValuesRewriteMethod query and
// returns the doc IDs it matches. With the production codec this matches zero
// documents because the DocValues ordinal-TermsEnum scoring path is unwired.
func fieldCacheRewriteDocValuesHits(t *testing.T, s *search.IndexSearcher) (map[int]struct{}, error) {
	t.Helper()
	// DocValuesRewriteMethod.Rewrite wraps a MultiTermQuery; the resulting query
	// scores via the field's SortedSetDocValues ordinal TermsEnum.
	mtq := search.NewMultiTermQuery(fieldCacheRewriteField, index.NewTerm(fieldCacheRewriteField, ""))
	rewritten, err := search.NewDocValuesRewriteMethod().Rewrite(s, mtq)
	if err != nil {
		return nil, err
	}
	top, err := s.Search(rewritten, 25)
	if err != nil {
		return nil, err
	}
	hits := make(map[int]struct{}, len(top.ScoreDocs))
	for _, sd := range top.ScoreDocs {
		hits[sd.Doc] = struct{}{}
	}
	return hits, nil
}

// TestFieldCacheRewriteMethod_TestRegexps ports testRegexps (inherited from
// TestRegexpRandom2): for each random regexp, assert the DocValues-rewrite hits
// equal the postings-rewrite hits.
func TestFieldCacheRewriteMethod_TestRegexps(t *testing.T) {
	s, cleanup := buildFieldCacheRewriteIndex(t)
	defer cleanup()

	// First prove the postings reference path produces real, non-empty hits for
	// at least one pattern, so the gap below is demonstrated against working
	// reference behaviour rather than an empty index.
	rng := rand.New(rand.NewSource(hashStringSeed(t.Name()) ^ 0xF1CA)) //nolint:gosec // deterministic test seed
	var sawNonEmptyReference bool
	for i := 0; i < 200 && !sawNonEmptyReference; i++ {
		reg := regexp2RandomRegexp(rng)
		hits, ok := fieldCacheRewriteReferenceHits(t, s, reg)
		if ok && len(hits) > 0 {
			sawNonEmptyReference = true
		}
	}
	if !sawNonEmptyReference {
		t.Fatalf("postings RegexpQuery reference produced no hits for any pattern; index fixture is degenerate")
	}

	// Now drive the DocValuesRewriteMethod path. In Gocene it matches zero
	// documents because the SortedSetDocValues ordinal-TermsEnum scoring path is
	// unwired, so it cannot reproduce the postings reference hit set.
	dvHits, err := fieldCacheRewriteDocValuesHits(t, s)
	if err != nil {
		t.Fatalf("DocValuesRewriteMethod search: %v", err)
	}
	if len(dvHits) == 0 {
		t.Fatalf("blocked: DocValuesRewriteMethod scoring is unwired for the " +
			"production codec — the field's SortedSetDocValues exposes no " +
			"ordinal-aware TermsEnum (SortedSetDocValuesWithTermsEnum / " +
			"TermsEnumWithOrd) and RegexpQuery supplies no " +
			"MultiTermQueryTermsEnumProvider, so the DocValues rewrite matches " +
			"zero documents and cannot equal the postings RegexpQuery hits")
	}
	t.Fatalf("blocked: RegexpQuery has no selectable DocValuesRewriteMethod / " +
		"CONSTANT_SCORE_BLENDED_REWRITE surface to compare against the postings rewrite")
}
