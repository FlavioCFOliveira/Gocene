// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Mirrors lucene/core/src/test/org/apache/lucene/util/graph/TestGraphTokenStreamFiniteStrings.java
// from Apache Lucene 10.4.0.

package graph

import (
	"errors"
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// cannedToken is a local stand-in for org.apache.lucene.tests.analysis.Token.
// It carries the three attribute values exercised by the reference tests.
type cannedToken struct {
	term    string
	posIncr int
	posLen  int
}

func tk(term string, posIncr, posLen int) cannedToken {
	return cannedToken{term: term, posIncr: posIncr, posLen: posLen}
}

// cannedTokenStream is a minimal Go port of
// org.apache.lucene.tests.analysis.CannedTokenStream covering only the
// attributes consumed by GraphTokenStreamFiniteStrings.
type cannedTokenStream struct {
	*analysis.BaseTokenStream
	tokens     []cannedToken
	cursor     int
	termAtt    analysis.CharTermAttribute
	posIncrAtt analysis.PositionIncrementAttribute
	posLenAtt  *analysis.PositionLengthAttribute
}

func newCannedTokenStream(tokens ...cannedToken) *cannedTokenStream {
	s := &cannedTokenStream{
		BaseTokenStream: analysis.NewBaseTokenStream(),
		tokens:          tokens,
	}
	s.termAtt = analysis.NewCharTermAttribute()
	s.posIncrAtt = analysis.NewPositionIncrementAttribute()
	s.posLenAtt = analysis.NewPositionLengthAttribute()
	s.AddAttribute(s.termAtt)
	s.AddAttribute(s.posIncrAtt)
	s.AddAttribute(s.posLenAtt)
	return s
}

func (s *cannedTokenStream) IncrementToken() (bool, error) {
	if s.cursor >= len(s.tokens) {
		return false, nil
	}
	s.ClearAttributes()
	t := s.tokens[s.cursor]
	s.termAtt.SetValue(t.term)
	s.posIncrAtt.SetPositionIncrement(t.posIncr)
	s.posLenAtt.SetPositionLength(t.posLen)
	s.cursor++
	return true, nil
}

func (s *cannedTokenStream) End() error   { return nil }
func (s *cannedTokenStream) Close() error { return nil }

// assertTokenStream mirrors the Java test helper of the same name.
func assertTokenStream(t *testing.T, ts analysis.TokenStream, terms []string, increments []int) {
	t.Helper()
	if ts == nil {
		t.Fatal("ts is nil")
	}
	if len(terms) != len(increments) {
		t.Fatalf("test bug: terms (%d) and increments (%d) must have equal length", len(terms), len(increments))
	}
	termAtt, ok := ts.(interface {
		GetAttribute(string) analysis.AttributeImpl
	})
	if !ok {
		t.Fatalf("ts %T does not expose GetAttribute", ts)
	}
	tA := termAtt.GetAttribute("CharTermAttribute").(analysis.CharTermAttribute)
	iA := termAtt.GetAttribute("PositionIncrementAttribute").(analysis.PositionIncrementAttribute)
	lA := termAtt.GetAttribute("PositionLengthAttribute").(*analysis.PositionLengthAttribute)
	offset := 0
	for {
		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		if offset >= len(terms) {
			t.Fatalf("stream produced more tokens than expected (>%d)", len(terms))
		}
		if got, want := tA.String(), terms[offset]; got != want {
			t.Errorf("token[%d] term: got %q want %q", offset, got, want)
		}
		if got, want := iA.GetPositionIncrement(), increments[offset]; got != want {
			t.Errorf("token[%d] posIncr: got %d want %d", offset, got, want)
		}
		if got := lA.GetPositionLength(); got != 1 {
			t.Errorf("token[%d] posLen: got %d want 1 (linear)", offset, got)
		}
		offset++
	}
	if offset != len(terms) {
		t.Errorf("token count: got %d want %d", offset, len(terms))
	}
}

// assertTerms mirrors the assertion patterns of the Java tests for
// GetTerms(field, state).
func assertTerms(t *testing.T, got []*index.Term, wantField string, wantTexts ...string) {
	t.Helper()
	if len(got) != len(wantTexts) {
		t.Fatalf("terms count: got %d want %d", len(got), len(wantTexts))
	}
	for i, term := range got {
		if term.Field != wantField {
			t.Errorf("terms[%d].Field: got %q want %q", i, term.Field, wantField)
		}
		if term.Text() != wantTexts[i] {
			t.Errorf("terms[%d].Text: got %q want %q", i, term.Text(), wantTexts[i])
		}
	}
}

// --- Direct ports of the Java test peers. ---

func TestIllegalState(t *testing.T) {
	ts := newCannedTokenStream(tk("a", 0, 1), tk("b", 1, 1))
	_, err := NewGraphTokenStreamFiniteStrings(ts)
	if !errors.Is(err, ErrMalformedStartToken) {
		t.Fatalf("expected ErrMalformedStartToken, got %v", err)
	}
}

func TestEmpty(t *testing.T) {
	ts := newCannedTokenStream()
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if it.HasNext() {
		t.Fatal("expected no paths in empty graph")
	}
	pts, err := g.ArticulationPoints()
	if err != nil {
		t.Fatalf("ArticulationPoints: %v", err)
	}
	if len(pts) != 0 {
		t.Fatalf("expected empty articulation points, got %v", pts)
	}
}

func TestSingleGraph(t *testing.T) {
	ts := newCannedTokenStream(
		tk("fast", 1, 1),
		tk("wi", 1, 1),
		tk("wifi", 0, 2),
		tk("fi", 1, 1),
		tk("network", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}

	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal("path 1 missing")
	}
	assertTokenStream(t, it.Next(), []string{"fast", "wi", "fi", "network"}, []int{1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal("path 2 missing")
	}
	assertTokenStream(t, it.Next(), []string{"fast", "wifi", "network"}, []int{1, 1, 1})
	if it.HasNext() {
		t.Fatal("unexpected extra path")
	}

	pts, err := g.ArticulationPoints()
	if err != nil {
		t.Fatalf("ArticulationPoints: %v", err)
	}
	if !reflect.DeepEqual(pts, []int{1, 3}) {
		t.Fatalf("articulation points: got %v want [1 3]", pts)
	}

	if g.HasSidePath(0) {
		t.Error("HasSidePath(0): got true want false")
	}
	it = g.GetFiniteStringsRange(0, 1)
	if !it.HasNext() {
		t.Fatal("0->1 missing")
	}
	assertTokenStream(t, it.Next(), []string{"fast"}, []int{1})
	if it.HasNext() {
		t.Fatal("unexpected extra 0->1")
	}
	assertTerms(t, g.GetTerms("field", 0), "field", "fast")

	if !g.HasSidePath(1) {
		t.Error("HasSidePath(1): got false want true")
	}
	it = g.GetFiniteStringsRange(1, 3)
	if !it.HasNext() {
		t.Fatal("1->3 path 1 missing")
	}
	assertTokenStream(t, it.Next(), []string{"wi", "fi"}, []int{1, 1})
	if !it.HasNext() {
		t.Fatal("1->3 path 2 missing")
	}
	assertTokenStream(t, it.Next(), []string{"wifi"}, []int{1})
	if it.HasNext() {
		t.Fatal("unexpected extra 1->3")
	}

	if g.HasSidePath(3) {
		t.Error("HasSidePath(3): got true want false")
	}
	it = g.GetFiniteStringsRange(3, -1)
	if !it.HasNext() {
		t.Fatal("3-> missing")
	}
	assertTokenStream(t, it.Next(), []string{"network"}, []int{1})
	if it.HasNext() {
		t.Fatal("unexpected extra 3->")
	}
	assertTerms(t, g.GetTerms("field", 3), "field", "network")
}

func TestSingleGraphWithGap(t *testing.T) {
	ts := newCannedTokenStream(
		tk("hey", 1, 1),
		tk("fast", 2, 1),
		tk("wi", 1, 1),
		tk("wifi", 0, 2),
		tk("fi", 1, 1),
		tk("network", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal("path 1 missing")
	}
	assertTokenStream(t, it.Next(),
		[]string{"hey", "fast", "wi", "fi", "network"}, []int{1, 2, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal("path 2 missing")
	}
	assertTokenStream(t, it.Next(),
		[]string{"hey", "fast", "wifi", "network"}, []int{1, 2, 1, 1})
	if it.HasNext() {
		t.Fatal("unexpected extra")
	}

	pts, err := g.ArticulationPoints()
	if err != nil {
		t.Fatalf("ArticulationPoints: %v", err)
	}
	if !reflect.DeepEqual(pts, []int{1, 2, 4}) {
		t.Fatalf("articulation points: got %v want [1 2 4]", pts)
	}

	if g.HasSidePath(0) {
		t.Error("HasSidePath(0): true")
	}
	it = g.GetFiniteStringsRange(0, 1)
	if !it.HasNext() {
		t.Fatal("0->1 missing")
	}
	assertTokenStream(t, it.Next(), []string{"hey"}, []int{1})
	if it.HasNext() {
		t.Fatal("extra 0->1")
	}
	assertTerms(t, g.GetTerms("field", 0), "field", "hey")

	if g.HasSidePath(1) {
		t.Error("HasSidePath(1): true")
	}
	it = g.GetFiniteStringsRange(1, 2)
	if !it.HasNext() {
		t.Fatal("1->2 missing")
	}
	assertTokenStream(t, it.Next(), []string{"fast"}, []int{2})
	if it.HasNext() {
		t.Fatal("extra 1->2")
	}
	assertTerms(t, g.GetTerms("field", 1), "field", "fast")

	if !g.HasSidePath(2) {
		t.Error("HasSidePath(2): false")
	}
	it = g.GetFiniteStringsRange(2, 4)
	if !it.HasNext() {
		t.Fatal("2->4 path 1 missing")
	}
	assertTokenStream(t, it.Next(), []string{"wi", "fi"}, []int{1, 1})
	if !it.HasNext() {
		t.Fatal("2->4 path 2 missing")
	}
	assertTokenStream(t, it.Next(), []string{"wifi"}, []int{1})
	if it.HasNext() {
		t.Fatal("extra 2->4")
	}

	if g.HasSidePath(4) {
		t.Error("HasSidePath(4): true")
	}
	it = g.GetFiniteStringsRange(4, -1)
	if !it.HasNext() {
		t.Fatal("4-> missing")
	}
	assertTokenStream(t, it.Next(), []string{"network"}, []int{1})
	if it.HasNext() {
		t.Fatal("extra 4->")
	}
	assertTerms(t, g.GetTerms("field", 4), "field", "network")
}

func TestGraphAndGapSameToken(t *testing.T) {
	ts := newCannedTokenStream(
		tk("fast", 1, 1),
		tk("wi", 2, 1),
		tk("wifi", 0, 2),
		tk("fi", 1, 1),
		tk("network", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal("path 1 missing")
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wi", "fi", "network"}, []int{1, 2, 1, 1})
	if !it.HasNext() {
		t.Fatal("path 2 missing")
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wifi", "network"}, []int{1, 2, 1})
	if it.HasNext() {
		t.Fatal("extra")
	}

	pts, _ := g.ArticulationPoints()
	if !reflect.DeepEqual(pts, []int{1, 3}) {
		t.Fatalf("articulation points: got %v want [1 3]", pts)
	}

	if g.HasSidePath(0) {
		t.Error("HasSidePath(0): true")
	}
	it = g.GetFiniteStringsRange(0, 1)
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"fast"}, []int{1})
	if it.HasNext() {
		t.Fatal()
	}
	assertTerms(t, g.GetTerms("field", 0), "field", "fast")

	if !g.HasSidePath(1) {
		t.Error("HasSidePath(1): false")
	}
	it = g.GetFiniteStringsRange(1, 3)
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"wi", "fi"}, []int{2, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"wifi"}, []int{2})
	if it.HasNext() {
		t.Fatal()
	}

	if g.HasSidePath(3) {
		t.Error("HasSidePath(3): true")
	}
	it = g.GetFiniteStringsRange(3, -1)
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"network"}, []int{1})
	if it.HasNext() {
		t.Fatal()
	}
	assertTerms(t, g.GetTerms("field", 3), "field", "network")
}

func TestGraphAndGapSameTokenTerm(t *testing.T) {
	ts := newCannedTokenStream(
		tk("a", 1, 1),
		tk("b", 1, 1),
		tk("c", 2, 1),
		tk("a", 0, 2),
		tk("d", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"a", "b", "c", "d"}, []int{1, 1, 2, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"a", "b", "a"}, []int{1, 1, 2})
	if it.HasNext() {
		t.Fatal()
	}

	pts, _ := g.ArticulationPoints()
	if !reflect.DeepEqual(pts, []int{1, 2}) {
		t.Fatalf("articulation points: got %v want [1 2]", pts)
	}

	if g.HasSidePath(0) {
		t.Error()
	}
	it = g.GetFiniteStringsRange(0, 1)
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"a"}, []int{1})
	if it.HasNext() {
		t.Fatal()
	}
	assertTerms(t, g.GetTerms("field", 0), "field", "a")

	if g.HasSidePath(1) {
		t.Error()
	}
	it = g.GetFiniteStringsRange(1, 2)
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"b"}, []int{1})
	if it.HasNext() {
		t.Fatal()
	}
	assertTerms(t, g.GetTerms("field", 1), "field", "b")

	if !g.HasSidePath(2) {
		t.Error()
	}
	it = g.GetFiniteStringsRange(2, -1)
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"c", "d"}, []int{2, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"a"}, []int{2})
	if it.HasNext() {
		t.Fatal()
	}
}

func TestStackedGraph(t *testing.T) {
	ts := newCannedTokenStream(
		tk("fast", 1, 1),
		tk("wi", 1, 1),
		tk("wifi", 0, 2),
		tk("wireless", 0, 2),
		tk("fi", 1, 1),
		tk("network", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wi", "fi", "network"}, []int{1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wifi", "network"}, []int{1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wireless", "network"}, []int{1, 1, 1})
	if it.HasNext() {
		t.Fatal()
	}

	pts, _ := g.ArticulationPoints()
	if !reflect.DeepEqual(pts, []int{1, 3}) {
		t.Fatalf("articulation points: got %v want [1 3]", pts)
	}
}

func TestStackedGraphWithGap(t *testing.T) {
	ts := newCannedTokenStream(
		tk("fast", 1, 1),
		tk("wi", 2, 1),
		tk("wifi", 0, 2),
		tk("wireless", 0, 2),
		tk("fi", 1, 1),
		tk("network", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wi", "fi", "network"}, []int{1, 2, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wifi", "network"}, []int{1, 2, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wireless", "network"}, []int{1, 2, 1})
	if it.HasNext() {
		t.Fatal()
	}
}

func TestStackedGraphWithRepeat(t *testing.T) {
	ts := newCannedTokenStream(
		tk("ny", 1, 4),
		tk("new", 0, 1),
		tk("new", 0, 3),
		tk("york", 1, 1),
		tk("city", 1, 2),
		tk("york", 1, 1),
		tk("is", 1, 1),
		tk("great", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"ny", "is", "great"}, []int{1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"new", "york", "city", "is", "great"}, []int{1, 1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"new", "york", "is", "great"}, []int{1, 1, 1, 1})
	if it.HasNext() {
		t.Fatal()
	}

	pts, _ := g.ArticulationPoints()
	if !reflect.DeepEqual(pts, []int{4, 5}) {
		t.Fatalf("articulation points: got %v want [4 5]", pts)
	}
}

func TestGraphWithRegularSynonym(t *testing.T) {
	ts := newCannedTokenStream(
		tk("fast", 1, 1),
		tk("speedy", 0, 1),
		tk("wi", 1, 1),
		tk("wifi", 0, 2),
		tk("fi", 1, 1),
		tk("network", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wi", "fi", "network"}, []int{1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wifi", "network"}, []int{1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"speedy", "wi", "fi", "network"}, []int{1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"speedy", "wifi", "network"}, []int{1, 1, 1})
	if it.HasNext() {
		t.Fatal()
	}

	pts, _ := g.ArticulationPoints()
	if !reflect.DeepEqual(pts, []int{1, 3}) {
		t.Fatalf("articulation points: got %v want [1 3]", pts)
	}

	assertTerms(t, g.GetTerms("field", 0), "field", "fast", "speedy")
}

func TestMultiGraph(t *testing.T) {
	ts := newCannedTokenStream(
		tk("turbo", 1, 1),
		tk("fast", 0, 2),
		tk("charged", 1, 1),
		tk("wi", 1, 1),
		tk("wifi", 0, 2),
		tk("fi", 1, 1),
		tk("network", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"turbo", "charged", "wi", "fi", "network"}, []int{1, 1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"turbo", "charged", "wifi", "network"}, []int{1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wi", "fi", "network"}, []int{1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"fast", "wifi", "network"}, []int{1, 1, 1})
	if it.HasNext() {
		t.Fatal()
	}

	pts, _ := g.ArticulationPoints()
	if !reflect.DeepEqual(pts, []int{2, 4}) {
		t.Fatalf("articulation points: got %v want [2 4]", pts)
	}
}

func TestMultipleSidePaths(t *testing.T) {
	ts := newCannedTokenStream(
		tk("the", 1, 1),
		tk("ny", 1, 4),
		tk("new", 0, 1),
		tk("york", 1, 1),
		tk("wifi", 1, 5),
		tk("wi", 0, 1),
		tk("fi", 1, 4),
		tk("wifi", 2, 2),
		tk("wi", 0, 1),
		tk("fi", 1, 1),
		tk("network", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"the", "ny", "wifi", "network"}, []int{1, 1, 2, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"the", "ny", "wi", "fi", "network"}, []int{1, 1, 2, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"the", "new", "york", "wifi", "network"}, []int{1, 1, 1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(),
		[]string{"the", "new", "york", "wi", "fi", "network"}, []int{1, 1, 1, 1, 1, 1})
	if it.HasNext() {
		t.Fatal()
	}

	pts, _ := g.ArticulationPoints()
	if !reflect.DeepEqual(pts, []int{1, 7}) {
		t.Fatalf("articulation points: got %v want [1 7]", pts)
	}
}

func TestSidePathWithGap(t *testing.T) {
	ts := newCannedTokenStream(
		tk("king", 1, 1),
		tk("alfred", 1, 4),
		tk("alfred", 0, 1),
		tk("great", 3, 1),
		tk("awesome", 0, 1),
		tk("ruled", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"king", "alfred", "ruled"}, []int{1, 1, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"king", "alfred", "great", "ruled"}, []int{1, 1, 3, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"king", "alfred", "awesome", "ruled"}, []int{1, 1, 3, 1})
	if it.HasNext() {
		t.Fatal()
	}
}

func TestMultipleSidePathsWithGaps(t *testing.T) {
	ts := newCannedTokenStream(
		tk("king", 1, 1),
		tk("alfred", 1, 4),
		tk("alfred", 0, 1),
		tk("saxons", 3, 3),
		tk("wessex", 2, 1),
		tk("ruled", 1, 1),
	)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	it := g.GetFiniteStrings()
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"king", "alfred", "wessex", "ruled"}, []int{1, 1, 2, 1})
	if !it.HasNext() {
		t.Fatal()
	}
	assertTokenStream(t, it.Next(), []string{"king", "alfred", "saxons", "ruled"}, []int{1, 1, 3, 1})
	if it.HasNext() {
		t.Fatal()
	}
}

func TestLongTokenStreamRecursionGuard(t *testing.T) {
	tokens := []cannedToken{
		tk("fast", 1, 1),
		tk("wi", 1, 1),
		tk("wifi", 0, 2),
		tk("fi", 1, 1),
	}
	for i := 0; i < 1024+1; i++ {
		tokens = append(tokens, tk("network", 1, 1))
	}
	ts := newCannedTokenStream(tokens...)
	g, err := NewGraphTokenStreamFiniteStrings(ts)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	_, err = g.ArticulationPoints()
	if !errors.Is(err, ErrRecursionLimitExceeded) {
		t.Fatalf("expected ErrRecursionLimitExceeded, got %v", err)
	}
}
