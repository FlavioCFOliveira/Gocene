// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Tests for the monitor package.
//
// Deviations from Java:
//   - Tests that require IndexSearcher, RandomIndexWriter, Monitor.match(),
//     Analyzer, WhitespaceTokenizer, or assertAnalyzesTo are deferred to
//     backlog #2693 when the full Gocene search pipeline is available.
//   - The present tests exercise pure data-structure and algorithmic logic
//     (QueryMatch, QueryTree, TermWeightor, MatchingQueries,
//     MultiMatchingQueries, PresearcherMatch, SlowLog, MonitorQuery,
//     MonitorConfiguration, QueryDecomposer, QueryCacheEntry,
//     WritableQueryIndex, ReadonlyQueryIndex, ConcurrentQueryLoader).
package monitor

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// QueryMatch tests (tasks 3550 / 4374)
// ---------------------------------------------------------------------------

func TestQueryMatch_GetQueryID(t *testing.T) {
	m := NewQueryMatch("q1")
	if got := m.GetQueryID(); got != "q1" {
		t.Errorf("GetQueryID() = %q; want q1", got)
	}
}

func TestQueryMatch_Equals(t *testing.T) {
	a := NewQueryMatch("q1")
	b := NewQueryMatch("q1")
	c := NewQueryMatch("q2")
	if !a.Equals(b) {
		t.Error("equal query IDs should be equal")
	}
	if a.Equals(c) {
		t.Error("different query IDs should not be equal")
	}
}

func TestQueryMatch_String(t *testing.T) {
	m := NewQueryMatch("testQ")
	want := "Match(query=testQ)"
	if got := m.String(); got != want {
		t.Errorf("String() = %q; want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// TermWeightor tests (task 3567 / 4388)
// ---------------------------------------------------------------------------

func TestTermWeightor_Default(t *testing.T) {
	w := DefaultTermWeightor
	term := index.NewTerm("f", "hello")
	weight := w.ApplyAsDouble(term)
	if weight <= 0 {
		t.Errorf("DefaultTermWeightor weight = %g; want > 0", weight)
	}
}

func TestTermWeightor_LengthWeightor(t *testing.T) {
	w := LengthWeightor(3, 0.3)
	// Longer terms should weigh more (4 - norms[len]).
	short := w.ApplyAsDouble(index.NewTerm("f", "ab"))
	long := w.ApplyAsDouble(index.NewTerm("f", "abcdefgh"))
	if short >= long {
		t.Errorf("short(%g) should be < long(%g)", short, long)
	}
}

func TestTermWeightor_FieldWeightor(t *testing.T) {
	w := FieldWeightor(1.5, "g")
	if got := w.ApplyAsDouble(index.NewTerm("f", "foo")); got != 1.0 {
		t.Errorf("non-matching field weight = %g; want 1.0", got)
	}
	if got := w.ApplyAsDouble(index.NewTerm("g", "foo")); got != 1.5 {
		t.Errorf("matching field weight = %g; want 1.5", got)
	}
}

func TestTermWeightor_TermWeightor(t *testing.T) {
	w := TermWeightorByBytes(0.01, util.NewBytesRef([]byte("START")))
	if got := w.ApplyAsDouble(index.NewTerm("f", "START")); got != 0.01 {
		t.Errorf("TermWeightorByBytes(START) = %g; want 0.01", got)
	}
	if got := w.ApplyAsDouble(index.NewTerm("f", "OTHER")); got != 1.0 {
		t.Errorf("TermWeightorByBytes(OTHER) = %g; want 1.0", got)
	}
}

func TestTermWeightor_TermFreqWeightor(t *testing.T) {
	freqs := map[string]int{"france": 31635, "s": 47088}
	w := TermFreqWeightor(freqs, 100, 0.8)
	france := w.ApplyAsDouble(index.NewTerm("f", "france"))
	s := w.ApplyAsDouble(index.NewTerm("f", "s"))
	if france <= s {
		t.Errorf("france(%g) should weigh more than s(%g) due to lower freq", france, s)
	}
}

func TestTermWeightor_TermAndFieldWeightor(t *testing.T) {
	w := TermAndFieldWeightor(0.1,
		index.NewTerm("field1", "f"),
		index.NewTerm("field1", "g"),
	)
	if got := w.ApplyAsDouble(index.NewTerm("field1", "f")); got != 0.1 {
		t.Errorf("TermAndFieldWeightor(field1,f) = %g; want 0.1", got)
	}
	if got := w.ApplyAsDouble(index.NewTerm("field2", "f")); got != 1.0 {
		t.Errorf("TermAndFieldWeightor(field2,f) = %g; want 1.0", got)
	}
}

func TestTermWeightor_Combine(t *testing.T) {
	w1 := FieldWeightor(2.0, "a")
	w2 := FieldWeightor(3.0, "a")
	w := CombineWeightors(w1, w2)
	if got := w.ApplyAsDouble(index.NewTerm("a", "x")); got != 6.0 {
		t.Errorf("combined weight = %g; want 6.0", got)
	}
}

// ---------------------------------------------------------------------------
// QueryTree tests (task 3573 / 4388)
// ---------------------------------------------------------------------------

func TestQueryTree_TermLeaf(t *testing.T) {
	term := index.NewTerm("f", "foo")
	node := NewTermQueryTreeFromTerm(term, DefaultTermWeightor)
	if node.Weight() <= 0 {
		t.Errorf("leaf weight = %g; want > 0", node.Weight())
	}

	var collected []string
	node.CollectTerms(func(field string, b *util.BytesRef) {
		collected = append(collected, field+":"+b.String())
	})
	if len(collected) != 1 || collected[0] != "f:foo" {
		t.Errorf("CollectTerms() = %v; want [f:foo]", collected)
	}

	if node.AdvancePhase(0) {
		t.Error("leaf AdvancePhase should return false")
	}
}

func TestQueryTree_AnyTerm(t *testing.T) {
	node := NewAnyTermQueryTree("reason")
	if node.Weight() != 0 {
		t.Errorf("ANY weight = %g; want 0", node.Weight())
	}

	var collected []string
	node.CollectTerms(func(f string, b *util.BytesRef) {
		collected = append(collected, f+":"+b.String())
	})
	want := AnyTokenField + ":" + AnyToken
	if len(collected) != 1 || collected[0] != want {
		t.Errorf("ANY CollectTerms = %v; want [%s]", collected, want)
	}
}

func TestQueryTree_AnyTokensAreNotPreferred(t *testing.T) {
	// From Java TestQueryTermComparators.testAnyTokensAreNotPreferred
	node1 := NewTermQueryTree("f", util.NewBytesRef([]byte("foo")), 1.0)
	node2 := NewAnyTermQueryTree("*:*")

	// Conjunction picks the highest-weight child; ANY (weight 0) should not win.
	// Build manually using newConjunctionFromSlice.
	conj := newConjunctionFromSlice([]QueryTree{node1, node2})

	var terms []string
	conj.CollectTerms(func(f string, b *util.BytesRef) {
		terms = append(terms, f+":"+b.String())
	})
	// The conjunction should collect from node1 (f:foo), not from the ANY node.
	if len(terms) != 1 || terms[0] != "f:foo" {
		t.Errorf("conjunction terms = %v; want [f:foo]", terms)
	}
}

func TestQueryTree_HigherWeightsArePreferred(t *testing.T) {
	// From Java TestQueryTermComparators.testHigherWeightsArePreferred
	node1 := NewTermQueryTree("f", util.NewBytesRef([]byte("foo")), 1.0)
	node2 := NewTermQueryTree("f", util.NewBytesRef([]byte("foobar")), 1.5)

	conj := newConjunctionFromSlice([]QueryTree{node1, node2})

	var terms []string
	conj.CollectTerms(func(f string, b *util.BytesRef) {
		terms = append(terms, f+":"+b.String())
	})
	if len(terms) != 1 || terms[0] != "f:foobar" {
		t.Errorf("conjunction terms = %v; want [f:foobar]", terms)
	}
}

func TestQueryTree_Conjunction_AdvancePhase(t *testing.T) {
	node1 := NewTermQueryTree("f", util.NewBytesRef([]byte("a")), 2.0)
	node2 := NewTermQueryTree("f", util.NewBytesRef([]byte("b")), 1.0)
	conj := newConjunctionFromSlice([]QueryTree{node1, node2})

	// First phase: highest-weight child (a).
	var first []string
	conj.CollectTerms(func(f string, b *util.BytesRef) { first = append(first, b.String()) })
	if len(first) != 1 || first[0] != "a" {
		t.Errorf("phase 0 = %v; want [a]", first)
	}

	// Advance: should move to child b.
	if !conj.AdvancePhase(0) {
		t.Error("AdvancePhase should return true")
	}
	var second []string
	conj.CollectTerms(func(f string, b *util.BytesRef) { second = append(second, b.String()) })
	if len(second) != 1 || second[0] != "b" {
		t.Errorf("phase 1 = %v; want [b]", second)
	}

	// No more phases.
	if conj.AdvancePhase(0) {
		t.Error("final AdvancePhase should return false")
	}
}

func TestQueryTree_Disjunction_CollectsAll(t *testing.T) {
	node1 := NewTermQueryTree("f", util.NewBytesRef([]byte("x")), 1.0)
	node2 := NewTermQueryTree("f", util.NewBytesRef([]byte("y")), 2.0)
	disj := newDisjunctionFromSlice([]QueryTree{node1, node2})

	var terms []string
	disj.CollectTerms(func(f string, b *util.BytesRef) { terms = append(terms, b.String()) })
	if len(terms) != 2 {
		t.Errorf("disjunction CollectTerms = %v; want 2 terms", terms)
	}
}

// ---------------------------------------------------------------------------
// MonitorQuery tests (task 3542)
// ---------------------------------------------------------------------------

func TestMonitorQuery_Fields(t *testing.T) {
	mq := NewMonitorQuery("id1", nil, "hello world", map[string]string{"k": "v"})
	if mq.GetID() != "id1" {
		t.Errorf("GetID() = %q; want id1", mq.GetID())
	}
	if mq.GetQueryString() != "hello world" {
		t.Errorf("GetQueryString() = %q; want 'hello world'", mq.GetQueryString())
	}
	if mq.GetMetadata()["k"] != "v" {
		t.Errorf("GetMetadata()[k] = %q; want v", mq.GetMetadata()["k"])
	}
}

func TestMonitorQuery_Equality(t *testing.T) {
	mq1 := NewMonitorQuery("id1", nil, "", nil)
	mq2 := NewMonitorQuery("id1", nil, "", nil)
	mq3 := NewMonitorQuery("id2", nil, "", nil)
	if !mq1.Equals(mq2) {
		t.Error("same IDs should be equal")
	}
	if mq1.Equals(mq3) {
		t.Error("different IDs should not be equal")
	}
}

func TestMonitorQuery_MetadataIsDefensive(t *testing.T) {
	orig := map[string]string{"a": "1"}
	mq := NewMonitorQuery("id", nil, "", orig)
	orig["b"] = "2"
	if _, has := mq.GetMetadata()["b"]; has {
		t.Error("mutating the original map should not affect MonitorQuery metadata")
	}
}

// ---------------------------------------------------------------------------
// MatchingQueries tests (task 3551)
// ---------------------------------------------------------------------------

func TestMatchingQueries_Matches(t *testing.T) {
	m := NewQueryMatch("q1")
	mq := newMatchingQueries(
		map[string]*QueryMatch{"q1": m},
		nil, 0, 0, 1,
	)
	got, ok := mq.Matches("q1")
	if !ok || got != m {
		t.Error("Matches(q1) should return the match")
	}
	_, ok = mq.Matches("q99")
	if ok {
		t.Error("Matches(q99) should return false")
	}
	if mq.GetMatchCount() != 1 {
		t.Errorf("GetMatchCount() = %d; want 1", mq.GetMatchCount())
	}
}

// ---------------------------------------------------------------------------
// MultiMatchingQueries tests (task 3571)
// ---------------------------------------------------------------------------

func TestMultiMatchingQueries_Matches(t *testing.T) {
	m := NewQueryMatch("q1")
	mmq := newMultiMatchingQueries(
		[]map[string]*QueryMatch{{"q1": m}},
		nil, 0, 0, 1, 1,
	)
	got, ok := mmq.Matches("q1", 0)
	if !ok || got != m {
		t.Error("Matches(q1, 0) should return the match")
	}
	_, ok = mmq.Matches("q1", 1) // out of range
	if ok {
		t.Error("Matches(q1, 1) should return false (out of range)")
	}
	if mmq.GetMatchCount(0) != 1 {
		t.Errorf("GetMatchCount(0) = %d; want 1", mmq.GetMatchCount(0))
	}
	if mmq.GetBatchSize() != 1 {
		t.Errorf("GetBatchSize() = %d; want 1", mmq.GetBatchSize())
	}
}

func TestMultiMatchingQueries_Singleton(t *testing.T) {
	m := NewQueryMatch("q1")
	mmq := newMultiMatchingQueries(
		[]map[string]*QueryMatch{{"q1": m}},
		nil, 10, 5, 3, 1,
	)
	single := mmq.singleton()
	got, ok := single.Matches("q1")
	if !ok || got != m {
		t.Error("singleton Matches(q1) should work")
	}
	if single.GetQueryBuildTime() != 10 {
		t.Errorf("GetQueryBuildTime() = %d; want 10", single.GetQueryBuildTime())
	}
}

// ---------------------------------------------------------------------------
// PresearcherMatch tests (task 3540 / 3556)
// ---------------------------------------------------------------------------

func TestPresearcherMatch_Fields(t *testing.T) {
	m := NewQueryMatch("q1")
	pm := newPresearcherMatch("q1", "presearch_terms", m)
	if pm.QueryID != "q1" {
		t.Errorf("QueryID = %q; want q1", pm.QueryID)
	}
	if pm.PresearcherMatches != "presearch_terms" {
		t.Errorf("PresearcherMatches = %q; want presearch_terms", pm.PresearcherMatches)
	}
	if pm.QueryMatch != m {
		t.Error("QueryMatch should match the provided match")
	}
}

// ---------------------------------------------------------------------------
// SlowLog tests (task 3547)
// ---------------------------------------------------------------------------

func TestSlowLog_AddAndIterate(t *testing.T) {
	s := NewSlowLog()
	s.AddQuery("q1", 100)
	s.AddQuery("q2", 200)
	entries := s.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries count = %d; want 2", len(entries))
	}
	if entries[0].QueryID != "q1" || entries[0].Time != 100 {
		t.Errorf("entries[0] = %+v; want {q1, 100}", entries[0])
	}
}

func TestSlowLog_AddAll(t *testing.T) {
	s := NewSlowLog()
	s.AddAll([]SlowLogEntry{{"a", 1}, {"b", 2}})
	if len(s.Entries()) != 2 {
		t.Errorf("entries count = %d; want 2", len(s.Entries()))
	}
}

func TestSlowLog_String(t *testing.T) {
	s := NewSlowLog()
	s.AddQuery("q1", 50)
	out := s.String()
	if out == "" {
		t.Error("String() should not be empty")
	}
}

// ---------------------------------------------------------------------------
// ScoringMatch tests (task 3538)
// ---------------------------------------------------------------------------

func TestScoringMatch_Fields(t *testing.T) {
	m := NewScoringMatch("q1", 3.14)
	if m.GetQueryID() != "q1" {
		t.Errorf("GetQueryID() = %q; want q1", m.GetQueryID())
	}
	if m.GetScore() != 3.14 {
		t.Errorf("GetScore() = %g; want 3.14", m.GetScore())
	}
}

func TestScoringMatch_Equals(t *testing.T) {
	a := NewScoringMatch("q1", 1.0)
	b := NewScoringMatch("q1", 1.0)
	c := NewScoringMatch("q1", 2.0)
	if !a.Equals(b) {
		t.Error("same queryID+score should be equal")
	}
	if a.Equals(c) {
		t.Error("different scores should not be equal")
	}
}

// ---------------------------------------------------------------------------
// ExplainingMatch tests (task 3539)
// ---------------------------------------------------------------------------

func TestExplainingMatch_Fields(t *testing.T) {
	exp := &Explanation{IsMatch: true, Description: "score explains", Value: 5.0}
	m := NewExplainingMatch("q1", exp)
	if m.GetQueryID() != "q1" {
		t.Errorf("GetQueryID() = %q; want q1", m.GetQueryID())
	}
	if m.GetExplanation() != exp {
		t.Error("GetExplanation() should return the provided explanation")
	}
}

// ---------------------------------------------------------------------------
// HighlightsMatch tests (task 3546)
// ---------------------------------------------------------------------------

func TestHighlightsMatch_AddHit(t *testing.T) {
	m := NewHighlightsMatch("q1")
	m.AddHit("body", 0, 1, 0, 5)
	m.AddHit("body", 2, 3, 6, 11)
	m.AddHit("title", 0, 1, 0, 4)

	if got := m.GetHitCount(); got != 3 {
		t.Errorf("GetHitCount() = %d; want 3", got)
	}
	fields := m.GetFields()
	if len(fields) != 2 {
		t.Errorf("GetFields() len = %d; want 2", len(fields))
	}
	bodyHits := m.GetFieldHits("body")
	if len(bodyHits) != 2 {
		t.Errorf("body hits = %d; want 2", len(bodyHits))
	}
}

func TestHighlightsMatch_Merge(t *testing.T) {
	a := NewHighlightsMatch("q1")
	a.AddHit("f", 0, 1, 0, 3)
	b := NewHighlightsMatch("q1")
	b.AddHit("f", 2, 3, 4, 7)

	merged := MergeHighlights("q1", a, b)
	if got := merged.GetHitCount(); got != 2 {
		t.Errorf("merged hit count = %d; want 2", got)
	}
}

// ---------------------------------------------------------------------------
// MonitorConfiguration tests (task 3563)
// ---------------------------------------------------------------------------

func TestMonitorConfiguration_Defaults(t *testing.T) {
	cfg := NewMonitorConfiguration()
	if cfg.QueryUpdateBufferSize != 5000 {
		t.Errorf("QueryUpdateBufferSize = %d; want 5000", cfg.QueryUpdateBufferSize)
	}
	if cfg.PurgeFrequency == 0 {
		t.Error("PurgeFrequency should not be zero")
	}
	if cfg.QueryDecomposer == nil {
		t.Error("QueryDecomposer should not be nil")
	}
}

func TestMonitorConfiguration_Setters(t *testing.T) {
	cfg := NewMonitorConfiguration().
		SetQueryUpdateBufferSize(100).
		SetReadOnly(true)
	if cfg.QueryUpdateBufferSize != 100 {
		t.Errorf("QueryUpdateBufferSize = %d; want 100", cfg.QueryUpdateBufferSize)
	}
	if !cfg.ReadOnly {
		t.Error("ReadOnly should be true")
	}
}

// ---------------------------------------------------------------------------
// QueryDecomposer tests (task 3559)
// ---------------------------------------------------------------------------

func TestQueryDecomposer_SingleQuery(t *testing.T) {
	qd := NewQueryDecomposer()
	// With no BooleanQuery support yet, decompose returns the query as-is.
	q := &stubQuery{name: "test"}
	results := qd.Decompose(q)
	if len(results) != 1 {
		t.Errorf("Decompose = %d results; want 1", len(results))
	}
}

func TestQueryDecomposer_NilQuery(t *testing.T) {
	qd := NewQueryDecomposer()
	if results := qd.Decompose(nil); results != nil {
		t.Errorf("Decompose(nil) = %v; want nil", results)
	}
}

// ---------------------------------------------------------------------------
// QueryCacheEntry tests (task 3564)
// ---------------------------------------------------------------------------

func TestQueryCacheEntry_Decompose(t *testing.T) {
	q := &stubQuery{name: "q"}
	mq := NewMonitorQuery("myID", q, "", nil)
	qd := NewQueryDecomposer()
	entries := DecomposeMonitorQuery(mq, qd)
	if len(entries) != 1 {
		t.Fatalf("entries = %d; want 1", len(entries))
	}
	if entries[0].QueryID != "myID" {
		t.Errorf("QueryID = %q; want myID", entries[0].QueryID)
	}
	if entries[0].CacheID != "myID_0" {
		t.Errorf("CacheID = %q; want myID_0", entries[0].CacheID)
	}
}

// ---------------------------------------------------------------------------
// WritableQueryIndex tests (task 3549)
// ---------------------------------------------------------------------------

func TestWritableQueryIndex_CommitAndGet(t *testing.T) {
	idx := NewWritableQueryIndex(nil, &NoFilteringPresearcher{})
	mq := NewMonitorQuery("q1", nil, "", nil)
	if err := idx.Commit([]*MonitorQuery{mq}); err != nil {
		t.Fatalf("Commit error: %v", err)
	}
	got, err := idx.GetQuery("q1")
	if err != nil || got == nil {
		t.Fatalf("GetQuery error: %v, got nil: %v", err, got == nil)
	}
	if got.GetID() != "q1" {
		t.Errorf("GetQuery id = %q; want q1", got.GetID())
	}
}

func TestWritableQueryIndex_NumDocs(t *testing.T) {
	idx := NewWritableQueryIndex(nil, &NoFilteringPresearcher{})
	idx.Commit([]*MonitorQuery{NewMonitorQuery("a", nil, "", nil)})
	idx.Commit([]*MonitorQuery{NewMonitorQuery("b", nil, "", nil)})
	n, err := idx.NumDocs()
	if err != nil || n != 2 {
		t.Errorf("NumDocs() = %d, %v; want 2, nil", n, err)
	}
}

func TestWritableQueryIndex_Delete(t *testing.T) {
	idx := NewWritableQueryIndex(nil, &NoFilteringPresearcher{})
	idx.Commit([]*MonitorQuery{NewMonitorQuery("q1", nil, "", nil)})
	idx.DeleteQueries([]string{"q1"})
	n, _ := idx.NumDocs()
	if n != 0 {
		t.Errorf("NumDocs after delete = %d; want 0", n)
	}
}

func TestWritableQueryIndex_Clear(t *testing.T) {
	idx := NewWritableQueryIndex(nil, &NoFilteringPresearcher{})
	idx.Commit([]*MonitorQuery{NewMonitorQuery("q1", nil, "", nil)})
	idx.Clear()
	n, _ := idx.NumDocs()
	if n != 0 {
		t.Errorf("NumDocs after clear = %d; want 0", n)
	}
}

// ---------------------------------------------------------------------------
// ReadonlyQueryIndex tests (task 3561)
// ---------------------------------------------------------------------------

func TestReadonlyQueryIndex_MutationsPanic(t *testing.T) {
	inner := NewWritableQueryIndex(nil, &NoFilteringPresearcher{})
	ro := NewReadonlyQueryIndex(inner)

	assertPanics(t, "Commit", func() { ro.Commit(nil) })
	assertPanics(t, "DeleteQueries", func() { ro.DeleteQueries(nil) })
	assertPanics(t, "Clear", func() { ro.Clear() })
}

func TestReadonlyQueryIndex_Reads(t *testing.T) {
	inner := NewWritableQueryIndex(nil, &NoFilteringPresearcher{})
	inner.Commit([]*MonitorQuery{NewMonitorQuery("q1", nil, "", nil)})
	ro := NewReadonlyQueryIndex(inner)

	got, err := ro.GetQuery("q1")
	if err != nil || got == nil {
		t.Fatalf("GetQuery error: %v", err)
	}
	n, err := ro.NumDocs()
	if err != nil || n != 1 {
		t.Errorf("NumDocs() = %d, %v; want 1, nil", n, err)
	}
}

// ---------------------------------------------------------------------------
// Monitor tests (task 3555)
// ---------------------------------------------------------------------------

func TestMonitor_RegisterAndGet(t *testing.T) {
	mon, err := NewMonitor(nil)
	if err != nil {
		t.Fatalf("NewMonitor: %v", err)
	}
	defer mon.Close()

	mq := NewMonitorQuery("q1", nil, "", nil)
	if err := mon.Register([]*MonitorQuery{mq}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := mon.GetQuery("q1")
	if err != nil || got == nil {
		t.Fatalf("GetQuery: %v, got nil: %v", err, got == nil)
	}
	n, err := mon.GetQueryCount()
	if err != nil || n != 1 {
		t.Errorf("GetQueryCount() = %d, %v; want 1, nil", n, err)
	}
}

func TestMonitor_DeleteAndClear(t *testing.T) {
	mon, _ := NewMonitor(nil)
	defer mon.Close()
	mon.Register([]*MonitorQuery{
		NewMonitorQuery("q1", nil, "", nil),
		NewMonitorQuery("q2", nil, "", nil),
	})
	mon.Delete("q1")
	n, _ := mon.GetQueryCount()
	if n != 1 {
		t.Errorf("count after delete = %d; want 1", n)
	}
	mon.Clear()
	n, _ = mon.GetQueryCount()
	if n != 0 {
		t.Errorf("count after clear = %d; want 0", n)
	}
}

func TestMonitor_ReadOnly(t *testing.T) {
	cfg := NewMonitorConfiguration().SetReadOnly(true)
	mon, err := NewMonitorWithConfig(nil, cfg)
	if err != nil {
		t.Fatalf("NewMonitorWithConfig: %v", err)
	}
	defer mon.Close()

	assertPanics(t, "Register on readonly Monitor", func() {
		mon.Register([]*MonitorQuery{NewMonitorQuery("q1", nil, "", nil)})
	})
}

func TestMonitor_UpdateListener(t *testing.T) {
	var updates []*MonitorQuery
	listener := &testListener{onUpdate: func(mqs []*MonitorQuery) { updates = append(updates, mqs...) }}
	mon, _ := NewMonitor(nil)
	defer mon.Close()
	mon.AddUpdateListener(listener)
	mon.Register([]*MonitorQuery{NewMonitorQuery("q1", nil, "", nil)})
	if len(updates) != 1 || updates[0].GetID() != "q1" {
		t.Errorf("listener updates = %v; want [q1]", updates)
	}
}

// ---------------------------------------------------------------------------
// ConcurrentQueryLoader tests (task 3543)
// ---------------------------------------------------------------------------

func TestConcurrentQueryLoader_LoadsAll(t *testing.T) {
	mon, _ := NewMonitor(nil)
	defer mon.Close()
	loader := NewConcurrentQueryLoader(mon, 2, 100)
	for i := 0; i < 10; i++ {
		id := string(rune('a' + i))
		if err := loader.Add(NewMonitorQuery(id, nil, "", nil)); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := loader.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	n, _ := mon.GetQueryCount()
	if n != 10 {
		t.Errorf("query count = %d; want 10", n)
	}
}

func TestConcurrentQueryLoader_AfterShutdownReturnsError(t *testing.T) {
	mon, _ := NewMonitor(nil)
	defer mon.Close()
	loader := NewConcurrentQueryLoader(mon, 1, 10)
	loader.Close()
	err := loader.Add(NewMonitorQuery("q", nil, "", nil))
	if err == nil {
		t.Error("Add after Close should return error")
	}
}

// ---------------------------------------------------------------------------
// MonitorUpdateListener tests (task 3560)
// ---------------------------------------------------------------------------

func TestMonitorUpdateListener_NoopEmbedding(t *testing.T) {
	// Verifies that embedding NoopMonitorUpdateListener compiles and no-ops.
	var l NoopMonitorUpdateListener
	l.AfterUpdate(nil)
	l.AfterDelete(nil)
	l.AfterClear()
	l.OnPurge()
	l.OnPurgeError(nil)
}

// ---------------------------------------------------------------------------
// TermWeightor (combine/field/freq) tests (task 3567 / 4388 extended)
// ---------------------------------------------------------------------------

func TestTermWeightor_LengthWeightor32Threshold(t *testing.T) {
	w := LengthWeightor(3, 0.3)
	long32 := string(make([]byte, 32))
	long33 := string(make([]byte, 33))
	w32 := w.ApplyAsDouble(index.NewTerm("f", long32))
	w33 := w.ApplyAsDouble(index.NewTerm("f", long33))
	if w32 != w33 {
		t.Errorf("length ≥ 32 should produce same weight: w32=%g w33=%g", w32, w33)
	}
}

// ---------------------------------------------------------------------------
// QueryTree.StringAt tests (task 3573)
// ---------------------------------------------------------------------------

func TestQueryTree_StringAt(t *testing.T) {
	node := NewTermQueryTree("f", util.NewBytesRef([]byte("hello")), 1.5)
	s := node.StringAt(0)
	if s == "" {
		t.Error("StringAt should return non-empty string")
	}
	any := NewAnyTermQueryTree("reason")
	s2 := any.StringAt(2)
	if s2 == "" {
		t.Error("AnyTermQueryTree StringAt should return non-empty string")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// stubQuery is a minimal search.Query for testing.
type stubQuery struct {
	search.BaseQuery
	name string
}

// testListener is a MonitorUpdateListener for testing.
type testListener struct {
	NoopMonitorUpdateListener
	onUpdate func([]*MonitorQuery)
}

func (l *testListener) AfterUpdate(mqs []*MonitorQuery) {
	if l.onUpdate != nil {
		l.onUpdate(mqs)
	}
}

// assertPanics verifies that f() panics.
func assertPanics(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s should have panicked", name)
		}
	}()
	f()
}
