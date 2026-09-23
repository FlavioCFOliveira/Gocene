// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestFieldCacheRewriteMethod.java
// (Apache Lucene 10.5.0).
//
// Tests the FieldcacheRewriteMethod with random regular expressions. The class
// extends TestRegexpRandom2 (test_regexp_random2_test.go): it inherits
// testRegexps and overrides assertSame.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// nullAutomatonProvider renders the lambda `name -> null`.
type nullAutomatonProvider struct{}

func (nullAutomatonProvider) GetAutomaton(name string) (*automaton.Automaton, error) {
	return nil, nil
}

// fcrmSetUp renders the inherited setUp() with the overridden assertSame
// installed.
func fcrmSetUp(t *testing.T) (*regexpRandom2TestCase, func()) {
	t.Helper()
	tc, tearDown := rr2SetUp(t)
	tc.assertSameFn = func(t *testing.T, regexp string) { fcrmAssertSame(t, tc, regexp) }
	return tc, tearDown
}

// fcrmAssertSame renders the assertSame(String) override: test fieldcache
// rewrite against filter rewrite.
func fcrmAssertSame(t *testing.T, tc *regexpRandom2TestCase, regexp string) {
	t.Helper()
	fieldCache := search.NewRegexpQueryFull(
		index.NewTerm(tc.fieldName, regexp),
		automaton.RegExpNone,
		0,
		nullAutomatonProvider{},
		automaton.DefaultDeterminizeWorkLimit,
		search.NewDocValuesRewriteMethod(),
		true)

	filter := search.NewRegexpQueryFull(
		index.NewTerm(tc.fieldName, regexp),
		automaton.RegExpNone,
		0,
		nullAutomatonProvider{},
		automaton.DefaultDeterminizeWorkLimit,
		search.ConstantScoreRewrite,
		true)
	filter2 := search.NewRegexpQueryFull(
		index.NewTerm(tc.fieldName, regexp),
		automaton.RegExpNone,
		0,
		nullAutomatonProvider{},
		automaton.DefaultDeterminizeWorkLimit,
		search.ConstantScoreBlendedRewrite,
		true)

	fieldCacheDocs := mustSearch(t, tc.searcher1, fieldCache, 25)
	filterDocs := mustSearch(t, tc.searcher2, filter, 25)
	filter2Docs := mustSearch(t, tc.searcher2, filter2, 25)

	testsearch.CheckEqual(t, fieldCache, fieldCacheDocs.ScoreDocs, filterDocs.ScoreDocs)
	testsearch.CheckEqual(t, fieldCache, fieldCacheDocs.ScoreDocs, filter2Docs.ScoreDocs)
}

// TestFieldCacheRewriteMethodRegexps renders the testRegexps() test method
// inherited from TestRegexpRandom2.
func TestFieldCacheRewriteMethodRegexps(t *testing.T) {
	tc, tearDown := fcrmSetUp(t)
	defer tearDown()
	tc.testRegexps(t)
}

func TestFieldCacheRewriteMethodEquals(t *testing.T) {
	tc, tearDown := fcrmSetUp(t)
	defer tearDown()
	fieldName := tc.fieldName
	{
		a1 := search.NewRegexpQueryWithFlags(index.NewTerm(fieldName, "[aA]"), automaton.RegExpNone)
		a2 := search.NewRegexpQueryWithFlags(index.NewTerm(fieldName, "[aA]"), automaton.RegExpNone)
		b := search.NewRegexpQueryWithFlags(index.NewTerm(fieldName, "[bB]"), automaton.RegExpNone)
		if !a1.Equals(a2) {
			t.Fatal("assertEquals(a1, a2)")
		}
		if a1.Equals(b) {
			t.Fatal("assertFalse(a1.equals(b))")
		}
		queryUtilsCheck(t, a1)
	}

	{
		a1 := search.NewRegexpQueryFull(
			index.NewTerm(fieldName, "[aA]"),
			automaton.RegExpNone,
			0,
			nullAutomatonProvider{},
			automaton.DefaultDeterminizeWorkLimit,
			search.NewDocValuesRewriteMethod(),
			true)
		a2 := search.NewRegexpQueryFull(
			index.NewTerm(fieldName, "[aA]"),
			automaton.RegExpNone,
			0,
			nullAutomatonProvider{},
			automaton.DefaultDeterminizeWorkLimit,
			search.NewDocValuesRewriteMethod(),
			true)
		b := search.NewRegexpQueryFull(
			index.NewTerm(fieldName, "[bB]"),
			automaton.RegExpNone,
			0,
			nullAutomatonProvider{},
			automaton.DefaultDeterminizeWorkLimit,
			search.NewDocValuesRewriteMethod(),
			true)
		if !a1.Equals(a2) {
			t.Fatal("assertEquals(a1, a2)")
		}
		if a1.Equals(b) {
			t.Fatal("assertFalse(a1.equals(b))")
		}
		queryUtilsCheck(t, a1)
	}
}
