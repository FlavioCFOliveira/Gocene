// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis_test

// Port of lucene/core/src/test/org/apache/lucene/analysis/TestGraphTokenFilter.java
// (Apache Lucene 10.5.0), with the nested test class
// TestGraphTokenizers.GraphTokenizer it depends on
// (lucene/core/src/test/org/apache/lucene/analysis/TestGraphTokenizers.java).

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	testsanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// graphTestFilter is the port of TestGraphTokenFilter.TestFilter.
type graphTestFilter struct {
	*analysis.GraphTokenFilter
}

func newGraphTestFilter(input analysis.TokenStream) *graphTestFilter {
	return &graphTestFilter{GraphTokenFilter: analysis.NewGraphTokenFilter(input)}
}

func (f *graphTestFilter) IncrementToken() (bool, error) {
	return f.IncrementBaseToken()
}

// graphTokenizerToken holds the fields of the test-framework Token that
// GraphTokenizer uses.
type graphTokenizerToken struct {
	term        string
	startOffset int
	endOffset   int
	posInc      int
	posLength   int
}

// graphTokenizer is the port of TestGraphTokenizers.GraphTokenizer.
type graphTokenizer struct {
	*analysis.BaseTokenizer

	tokens      []graphTokenizerToken
	tokensSet   bool
	upto        int
	inputLength int

	termAtt      analysis.CharTermAttribute
	offsetAtt    analysis.OffsetAttribute
	posIncrAtt   tokenattributes.PositionIncrementAttribute
	posLengthAtt analysis.PositionLengthAttribute
}

func newGraphTokenizer() *graphTokenizer {
	t := &graphTokenizer{BaseTokenizer: analysis.NewBaseTokenizer()}
	t.termAtt = t.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	t.offsetAtt = t.AddAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	t.posIncrAtt = t.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	t.posLengthAtt = t.AddAttribute(analysis.PositionLengthAttributeType).(analysis.PositionLengthAttribute)
	return t
}

func (t *graphTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	t.tokens = nil
	t.tokensSet = false
	t.upto = 0
	return nil
}

func (t *graphTokenizer) IncrementToken() (bool, error) {
	if !t.tokensSet {
		if err := t.fillTokens(); err != nil {
			return false, err
		}
	}
	if t.upto == len(t.tokens) {
		return false, nil
	}
	tok := t.tokens[t.upto]
	t.upto++
	t.ClearAttributes()
	t.termAtt.AppendString(tok.term)
	t.offsetAtt.SetOffset(tok.startOffset, tok.endOffset)
	t.posIncrAtt.SetPositionIncrement(tok.posInc)
	t.posLengthAtt.SetPositionLength(tok.posLength)
	return true, nil
}

func (t *graphTokenizer) End() error {
	if err := t.BaseTokenizer.End(); err != nil {
		return err
	}
	// NOTE: somewhat... hackish, but we need this to
	// satisfy BTSTC:
	lastOffset := 0
	if t.tokensSet && len(t.tokens) > 0 {
		lastOffset = t.tokens[len(t.tokens)-1].endOffset
	}
	t.offsetAtt.SetOffset(t.CorrectOffset(lastOffset), t.CorrectOffset(t.inputLength))
	return nil
}

func (t *graphTokenizer) fillTokens() error {
	b, err := io.ReadAll(t.GetReader())
	if err != nil {
		return err
	}
	s := string(b)
	t.inputLength = len(s)

	parts := strings.Split(s, " ")

	t.tokens = t.tokens[:0]
	t.tokensSet = true
	pos := 0
	maxPos := -1
	offset := 0
	for _, part := range parts {
		overlapped := strings.Split(part, "/")
		firstAtPos := true
		minPosLength := int(^uint32(0) >> 1) // Integer.MAX_VALUE
		for _, part2 := range overlapped {
			colonIndex := strings.IndexByte(part2, ':')
			var token string
			var posLength int
			if colonIndex != -1 {
				token = part2[:colonIndex]
				posLength, err = strconv.Atoi(part2[1+colonIndex:])
				if err != nil {
					return err
				}
			} else {
				token = part2
				posLength = 1
			}
			maxPos = max(maxPos, pos+posLength)
			minPosLength = min(minPosLength, posLength)
			tok := graphTokenizerToken{
				term:        token,
				startOffset: offset,
				endOffset:   offset + 2*posLength - 1,
				posLength:   posLength,
			}
			if firstAtPos {
				tok.posInc = 1
			} else {
				tok.posInc = 0
			}
			firstAtPos = false
			t.tokens = append(t.tokens, tok)
		}
		pos += minPosLength
		offset = 2 * pos
	}
	if util.AssertsEnabled() && !(maxPos <= pos) {
		panic(util.NewAssertionError("input string mal-formed: posLength>1 tokens hang over the end"))
	}
	return nil
}

func mustBool(t *testing.T, what string, got bool, err error, want bool) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if got != want {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

func TestGraphTokenFilter_GraphTokenStream(t *testing.T) {
	tok := newGraphTokenizer()
	graph := newGraphTestFilter(tok)

	termAtt := graph.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	posIncAtt := graph.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)

	tok.SetReader(strings.NewReader("a b/c d e/f:3 g/h i j k"))
	if err := tok.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	assertTrue := func(what string) func(bool, error) {
		return func(got bool, err error) { t.Helper(); mustBool(t, what, got, err, true) }
	}
	assertFalse := func(what string) func(bool, error) {
		return func(got bool, err error) { t.Helper(); mustBool(t, what, got, err, false) }
	}
	assertTerm := func(want string) {
		t.Helper()
		if got := termAtt.String(); got != want {
			t.Fatalf("term: got %q, want %q", got, want)
		}
	}
	assertCached := func(want int) {
		t.Helper()
		if got := graph.CachedTokenCount(); got != want {
			t.Fatalf("cachedTokenCount: got %d, want %d", got, want)
		}
	}

	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(0)

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("a")
	if got := posIncAtt.GetPositionIncrement(); got != 1 {
		t.Fatalf("posInc: got %d, want 1", got)
	}
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("b")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("d")
	assertTrue("incrementGraph")(graph.IncrementGraph())
	assertTerm("a")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("c")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("d")
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(5)

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("b")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("d")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("e")
	assertTrue("incrementGraph")(graph.IncrementGraph())
	assertTerm("b")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("d")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("f")
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(6)

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("c")
	if got := posIncAtt.GetPositionIncrement(); got != 0 {
		t.Fatalf("posInc: got %d, want 0", got)
	}
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("d")
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(6)

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("d")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("e")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("g")
	assertTrue("incrementGraph")(graph.IncrementGraph())
	assertTerm("d")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("e")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("h")
	assertTrue("incrementGraph")(graph.IncrementGraph())
	assertTerm("d")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("f")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("j")
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(8)

	// tok.setReader(new StringReader("a b/c d e/f:3 g/h i j k"));

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("e")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("g")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("i")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("j")
	assertTrue("incrementGraph")(graph.IncrementGraph())
	assertTerm("e")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("h")
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(8)

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("f")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("j")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("k")
	assertFalse("incrementGraphToken")(graph.IncrementGraphToken())
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(8)

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("g")
	assertTrue("incrementGraphToken")(graph.IncrementGraphToken())
	assertTerm("i")
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(8)

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("h")
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertCached(8)

	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTrue("incrementBaseToken")(graph.IncrementBaseToken())
	assertTerm("k")
	assertFalse("incrementGraphToken")(graph.IncrementGraphToken())
	if got := graph.GetTrailingPositions(); got != 0 {
		t.Fatalf("trailingPositions: got %d, want 0", got)
	}
	assertFalse("incrementGraph")(graph.IncrementGraph())
	assertFalse("incrementBaseToken")(graph.IncrementBaseToken())
	assertCached(8)
}

func TestGraphTokenFilter_TrailingPositions(t *testing.T) {
	// a/b:2 c _
	cts := testsanalysis.NewCannedTokenStreamWithFinal(
		1, 5,
		testsanalysis.NewToken("a", 0, 1),
		testsanalysis.NewTokenWithPosIncAndLength("b", 0, 0, 1, 2),
		testsanalysis.NewTokenWithPosInc("c", 1, 2, 3))

	gts := newGraphTestFilter(cts)
	mustBool(t, "incrementGraph", mustCall(gts.IncrementGraph), nil, false)
	mustBool(t, "incrementBaseToken", mustCall(gts.IncrementBaseToken), nil, true)
	mustBool(t, "incrementGraphToken", mustCall(gts.IncrementGraphToken), nil, true)
	mustBool(t, "incrementGraphToken", mustCall(gts.IncrementGraphToken), nil, false)
	if got := gts.GetTrailingPositions(); got != 1 {
		t.Fatalf("trailingPositions: got %d, want 1", got)
	}
	mustBool(t, "incrementGraph", mustCall(gts.IncrementGraph), nil, false)
	mustBool(t, "incrementBaseToken", mustCall(gts.IncrementBaseToken), nil, true)
	mustBool(t, "incrementGraphToken", mustCall(gts.IncrementGraphToken), nil, false)
	if got := gts.GetTrailingPositions(); got != 1 {
		t.Fatalf("trailingPositions: got %d, want 1", got)
	}
	mustBool(t, "incrementGraph", mustCall(gts.IncrementGraph), nil, false)
}

// mustCall runs one of the IncrementX methods and panics on an I/O error,
// which the canned streams of these tests never produce.
func mustCall(fn func() (bool, error)) bool {
	ok, err := fn()
	if err != nil {
		panic(err)
	}
	return ok
}

// expectIllegalState runs fn and returns the message of the
// IllegalStateException (a panic) it must raise.
func expectIllegalState(t *testing.T, fn func()) (msg string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected IllegalStateException")
		}
		msg = fmt.Sprint(r)
	}()
	fn()
	return ""
}

func TestGraphTokenFilter_MaximumGraphCacheSize(t *testing.T) {
	tokens := make([]testsanalysis.Token, analysis.MaxTokenCacheSize+5)
	for i := 0; i < analysis.MaxTokenCacheSize+5; i++ {
		tokens[i] = testsanalysis.NewTokenWithPosInc("a", 1, i*2, i*2+1)
	}

	gts := newGraphTestFilter(testsanalysis.NewCannedTokenStream(tokens...))
	msg := expectIllegalState(t, func() {
		mustNoErr(t, gts.Reset())
		mustCall(gts.IncrementBaseToken)
		for {
			mustCall(gts.IncrementGraphToken)
		}
	})
	if msg != "Too many cached tokens (> 100)" {
		t.Fatalf("message: got %q", msg)
	}

	mustNoErr(t, gts.Reset())
	// after reset, the cache should be cleared and so we can read ahead once more
	mustCall(gts.IncrementBaseToken)
	mustCall(gts.IncrementGraphToken)
}

func TestGraphTokenFilter_GraphPathCountLimits(t *testing.T) {
	tokens := make([]testsanalysis.Token, 50)
	tokens[0] = testsanalysis.NewTokenWithPosInc("term", 1, 0, 1)
	tokens[1] = testsanalysis.NewTokenWithPosInc("term1", 1, 2, 3)
	for i := 2; i < 50; i++ {
		tokens[i] = testsanalysis.NewTokenWithPosInc("term"+strconv.Itoa(i), i%2, 2, 3)
	}

	msg := expectIllegalState(t, func() {
		graph := newGraphTestFilter(testsanalysis.NewCannedTokenStream(tokens...))
		mustNoErr(t, graph.Reset())
		mustCall(graph.IncrementBaseToken)
		for i := 0; i < 10; i++ {
			mustCall(graph.IncrementGraphToken)
		}
		for mustCall(graph.IncrementGraph) {
			for i := 0; i < 10; i++ {
				mustCall(graph.IncrementGraphToken)
			}
		}
	})
	if msg != "Too many graph paths (> 1000)" {
		t.Fatalf("message: got %q", msg)
	}
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
