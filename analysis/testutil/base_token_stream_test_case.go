// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"bytes"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TokenStreamExpectations describes the expected sequence of tokens
// emitted by an [analysis.TokenStream] together with the optional
// trailing state applied by End(). It is the Go-idiomatic counterpart
// of Lucene 10.4.0's
// org.apache.lucene.tests.analysis.BaseTokenStreamTestCase
// assertTokenStreamContents parameter list.
//
// Each per-token slice must, when non-nil, have the same length as
// Terms. A nil slice means "do not assert this attribute". Trailing
// fields use *int / *float32 to preserve the Lucene Integer/Float
// "null" semantics (don't assert) versus "explicit zero".
//
// FinalOffset / FinalPositionIncrement, when set, are checked after
// the stream's End() call.
//
// GraphOffsetsAreCorrect enables Lucene's graph-offset validation:
// every token leaving from a given position must have the same start
// offset, and every token arriving at a given end position must have
// the same end offset. Defaults to true for matrix-of-positions
// streams; disable for streams that legitimately break the graph
// invariant.
type TokenStreamExpectations struct {
	// Terms is the expected term-text sequence. Required; the length
	// of this slice drives the expected token count.
	Terms []string

	// StartOffsets / EndOffsets are per-token character offsets. A
	// nil slice skips offset assertions for that side.
	StartOffsets []int
	EndOffsets   []int

	// Types is the per-token lexical type ("word", "<NUM>", ...). A
	// nil slice skips type assertions.
	Types []string

	// PositionIncrements / PositionLengths are per-token position
	// metadata. Nil slices skip the matching assertions.
	PositionIncrements []int
	PositionLengths    []int

	// FinalOffset is the expected offsetAtt.endOffset() reading
	// after End(). Nil means "do not assert".
	FinalOffset *int

	// FinalPositionIncrement is the expected
	// posIncrAtt.getPositionIncrement() reading after End(). Nil
	// means "do not assert".
	FinalPositionIncrement *int

	// KeywordAtts is the per-token expected KeywordAttribute state.
	// Nil skips the assertion.
	KeywordAtts []bool

	// Payloads is the per-token expected payload bytes. A nil
	// outer slice skips payload assertions entirely; a non-nil
	// outer slice with a nil entry asserts that the corresponding
	// token has a nil payload.
	Payloads [][]byte

	// Flags is the per-token expected FlagsAttribute bitfield. Nil
	// skips the assertion.
	Flags []int

	// Boost is the per-token expected BoostAttribute value. Nil
	// skips the assertion. Float comparisons use a 1e-3 tolerance
	// matching Lucene's reference.
	Boost []float32

	// GraphOffsetsAreCorrect, when true, enables the Lucene
	// graph-offset invariants described in the type docstring.
	// Defaults to true via [AssertTokenStreamContents].
	GraphOffsetsAreCorrect bool

	// graphOffsetsAreCorrectSet tracks whether the field was set
	// explicitly so the default-to-true wrapper can detect it.
	graphOffsetsAreCorrectSet bool
}

// WithGraphOffsetsAreCorrect returns a copy of e with the flag set
// to the given value (and the explicit-set marker recorded).
func (e TokenStreamExpectations) WithGraphOffsetsAreCorrect(b bool) TokenStreamExpectations {
	e.GraphOffsetsAreCorrect = b
	e.graphOffsetsAreCorrectSet = true
	return e
}

// TestingT is the subset of *testing.T (and compatible recorders)
// that [AssertTokenStreamContents] requires. Accepting an interface
// keeps the helper unit-testable by allowing recording fakes to
// observe Errorf / Fatalf without aborting the outer test.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// AssertTokenStreamContents drives ts through one Reset / N
// IncrementToken / End / Close cycle and asserts that each emitted
// token matches the corresponding entry in want.
//
// It mirrors the canonical Lucene 10.4.0 assertTokenStreamContents
// helper: it pre-populates each registered attribute with a bogus
// value before every IncrementToken, then verifies that the stream
// cleared and rewrote the attributes; it enforces posInc >= 1 on the
// first token and >= 0 thereafter; it enforces posLength >= 1; it
// checks offsets never go backwards; and it applies the graph-offset
// invariants when enabled.
//
// On any mismatch the helper calls t.Errorf so the test continues
// and reports every failure, except for absent attributes (e.g.
// posIncrements requested but the stream has no PositionIncrement
// attribute) which call t.Fatalf because subsequent assertions
// cannot run.
//
// The helper does not consume an error budget: ts itself is allowed
// to return errors from IncrementToken / Reset / End / Close; those
// errors are reported via t.Fatalf because they invalidate the rest
// of the test.
func AssertTokenStreamContents(t TestingT, ts analysis.TokenStream, want TokenStreamExpectations) {
	t.Helper()

	if !want.graphOffsetsAreCorrectSet {
		want.GraphOffsetsAreCorrect = true
	}

	if want.Terms == nil {
		t.Fatalf("AssertTokenStreamContents: want.Terms must not be nil")
	}
	if err := validateSliceLengths(want); err != nil {
		t.Fatalf("AssertTokenStreamContents: %v", err)
	}

	src := getAttributeSource(t, ts)

	var termAtt analysis.CharTermAttribute
	if len(want.Terms) > 0 {
		ta := src.GetAttribute(analysis.CharTermAttributeType)
		if ta == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no CharTermAttribute")
		}
		var ok bool
		termAtt, ok = ta.(analysis.CharTermAttribute)
		if !ok {
			t.Fatalf("AssertTokenStreamContents: CharTermAttribute impl type %T does not satisfy interface", ta)
		}
	}

	var offsetAtt analysis.OffsetAttribute
	if want.StartOffsets != nil || want.EndOffsets != nil || want.FinalOffset != nil {
		oa := src.GetAttribute(analysis.OffsetAttributeType)
		if oa == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no OffsetAttribute")
		}
		offsetAtt, _ = oa.(analysis.OffsetAttribute)
	}

	var typeAtt analysis.TypeAttribute
	if want.Types != nil {
		ta := src.GetAttribute(analysis.TypeAttributeType)
		if ta == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no TypeAttribute")
		}
		typeAtt, _ = ta.(analysis.TypeAttribute)
	}

	var posIncrAtt analysis.PositionIncrementAttribute
	if want.PositionIncrements != nil || want.FinalPositionIncrement != nil {
		pa := src.GetAttribute(analysis.PositionIncrementAttributeType)
		if pa == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no PositionIncrementAttribute")
		}
		posIncrAtt, _ = pa.(analysis.PositionIncrementAttribute)
	}

	var posLenAtt analysis.PositionLengthAttribute
	if want.PositionLengths != nil {
		pa := src.GetAttribute(analysis.PositionLengthAttributeType)
		if pa == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no PositionLengthAttribute")
		}
		posLenAtt, _ = pa.(analysis.PositionLengthAttribute)
	}

	var keywordAtt analysis.KeywordAttribute
	if want.KeywordAtts != nil {
		ka := src.GetAttribute(analysis.KeywordAttributeType)
		if ka == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no KeywordAttribute")
		}
		keywordAtt, _ = ka.(analysis.KeywordAttribute)
	}

	var payloadAtt *analysis.PayloadAttributeImpl
	if want.Payloads != nil {
		pa := src.GetAttribute(analysis.PayloadAttributeType)
		if pa == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no PayloadAttribute")
		}
		payloadAtt, _ = pa.(*analysis.PayloadAttributeImpl)
		if payloadAtt == nil {
			t.Fatalf("AssertTokenStreamContents: PayloadAttribute impl type %T not supported by helper", pa)
		}
	}

	var flagsAtt *analysis.FlagsAttributeImpl
	if want.Flags != nil {
		fa := src.GetAttribute(analysis.FlagsAttributeType)
		if fa == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no FlagsAttribute")
		}
		flagsAtt, _ = fa.(*analysis.FlagsAttributeImpl)
		if flagsAtt == nil {
			t.Fatalf("AssertTokenStreamContents: FlagsAttribute impl type %T not supported by helper", fa)
		}
	}

	var boostAtt analysis.BoostAttribute
	if want.Boost != nil {
		ba := src.GetAttribute(analysis.BoostAttributeType)
		if ba == nil {
			t.Fatalf("AssertTokenStreamContents: stream has no BoostAttribute")
		}
		boostAtt, _ = ba.(analysis.BoostAttribute)
	}

	// Reset before iteration.
	if err := resetTokenStream(ts); err != nil {
		t.Fatalf("AssertTokenStreamContents: Reset error: %v", err)
	}

	posToStart := make(map[int]int)
	posToEnd := make(map[int]int)
	pos := -1
	lastStartOffset := 0

	for i, expectedTerm := range want.Terms {
		// Pre-populate attributes with bogus values so we can
		// detect a missing clearAttributes() in the stream.
		src.ClearAttributes()
		if termAtt != nil {
			termAtt.SetValue("bogusTerm")
		}
		if offsetAtt != nil {
			offsetAtt.SetOffset(14584724, 24683243)
		}
		if typeAtt != nil {
			typeAtt.SetType("bogusType")
		}
		if posIncrAtt != nil {
			posIncrAtt.SetPositionIncrement(45987657)
		}
		if posLenAtt != nil {
			posLenAtt.SetPositionLength(45987653)
		}
		if keywordAtt != nil {
			keywordAtt.SetKeyword((i & 1) == 0)
		}
		if payloadAtt != nil {
			payloadAtt.SetPayload([]byte{0x00, 0xdf, 0x12, 0xbd, 0x24})
		}
		if flagsAtt != nil {
			flagsAtt.SetFlags(^0)
		}
		if boostAtt != nil {
			boostAtt.SetBoost(-1)
		}

		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("token #%d: IncrementToken error: %v", i, err)
		}
		if !ok {
			t.Fatalf("token #%d (%q): IncrementToken returned false; stream exhausted early", i, expectedTerm)
		}

		if got := termAtt.String(); got != expectedTerm {
			t.Errorf("token #%d term: got %q, want %q", i, got, expectedTerm)
		}
		if want.StartOffsets != nil {
			if got := offsetAtt.StartOffset(); got != want.StartOffsets[i] {
				t.Errorf("token #%d startOffset term=%q: got %d, want %d", i, termAtt.String(), got, want.StartOffsets[i])
			}
		}
		if want.EndOffsets != nil {
			if got := offsetAtt.EndOffset(); got != want.EndOffsets[i] {
				t.Errorf("token #%d endOffset term=%q: got %d, want %d", i, termAtt.String(), got, want.EndOffsets[i])
			}
		}
		if want.Types != nil {
			if got := typeAtt.GetType(); got != want.Types[i] {
				t.Errorf("token #%d type term=%q: got %q, want %q", i, termAtt.String(), got, want.Types[i])
			}
		}
		if want.PositionIncrements != nil {
			if got := posIncrAtt.GetPositionIncrement(); got != want.PositionIncrements[i] {
				t.Errorf("token #%d posIncrement term=%q: got %d, want %d", i, termAtt.String(), got, want.PositionIncrements[i])
			}
		}
		if want.PositionLengths != nil {
			if got := posLenAtt.GetPositionLength(); got != want.PositionLengths[i] {
				t.Errorf("token #%d posLength term=%q: got %d, want %d", i, termAtt.String(), got, want.PositionLengths[i])
			}
		}
		if want.KeywordAtts != nil {
			if got := keywordAtt.IsKeywordToken(); got != want.KeywordAtts[i] {
				t.Errorf("token #%d keywordAtt term=%q: got %v, want %v", i, termAtt.String(), got, want.KeywordAtts[i])
			}
		}
		if want.Flags != nil {
			if got := flagsAtt.GetFlags(); got != want.Flags[i] {
				t.Errorf("token #%d flagsAtt term=%q: got %d, want %d", i, termAtt.String(), got, want.Flags[i])
			}
		}
		if want.Boost != nil {
			if got := boostAtt.GetBoost(); !float32Equal(got, want.Boost[i], 0.001) {
				t.Errorf("token #%d boostAtt term=%q: got %g, want %g", i, termAtt.String(), got, want.Boost[i])
			}
		}
		if want.Payloads != nil {
			got := payloadAtt.GetPayload()
			expected := want.Payloads[i]
			switch {
			case expected == nil && got != nil:
				t.Errorf("token #%d payload: got %v, want nil", i, got)
			case expected != nil && !bytes.Equal(got, expected):
				t.Errorf("token #%d payload: got %v, want %v", i, got, expected)
			}
		}

		// Invariants Lucene enforces even when the caller doesn't.
		if posIncrAtt != nil {
			inc := posIncrAtt.GetPositionIncrement()
			if i == 0 && inc < 1 {
				t.Errorf("token #%d: first posIncrement must be >= 1, got %d", i, inc)
			} else if i > 0 && inc < 0 {
				t.Errorf("token #%d: posIncrement must be >= 0, got %d", i, inc)
			}
		}
		if posLenAtt != nil {
			if l := posLenAtt.GetPositionLength(); l < 1 {
				t.Errorf("token #%d: posLength must be >= 1, got %d", i, l)
			}
		}
		if offsetAtt != nil {
			startOffset := offsetAtt.StartOffset()
			endOffset := offsetAtt.EndOffset()
			if want.FinalOffset != nil {
				if startOffset > *want.FinalOffset {
					t.Errorf("token #%d: startOffset %d > finalOffset %d term=%q", i, startOffset, *want.FinalOffset, termAtt.String())
				}
				if endOffset > *want.FinalOffset {
					t.Errorf("token #%d: endOffset %d > finalOffset %d term=%q", i, endOffset, *want.FinalOffset, termAtt.String())
				}
			}
			if startOffset < lastStartOffset {
				t.Errorf("token #%d: offsets went backwards startOffset=%d < lastStartOffset=%d term=%q", i, startOffset, lastStartOffset, termAtt.String())
			}
			lastStartOffset = startOffset

			if want.GraphOffsetsAreCorrect && posLenAtt != nil && posIncrAtt != nil {
				posInc := posIncrAtt.GetPositionIncrement()
				pos += posInc
				posLength := posLenAtt.GetPositionLength()

				if s, seen := posToStart[pos]; !seen {
					posToStart[pos] = startOffset
				} else if s != startOffset {
					t.Errorf("token #%d: inconsistent startOffset at pos=%d posLen=%d term=%q: got %d, previously %d", i, pos, posLength, termAtt.String(), startOffset, s)
				}

				endPos := pos + posLength
				if e, seen := posToEnd[endPos]; !seen {
					posToEnd[endPos] = endOffset
				} else if e != endOffset {
					t.Errorf("token #%d: inconsistent endOffset at endPos=%d posLen=%d term=%q: got %d, previously %d", i, endPos, posLength, termAtt.String(), endOffset, e)
				}
			}
		}
	}

	// Trailing tokens must not exist.
	if ok, err := ts.IncrementToken(); err != nil {
		t.Fatalf("trailing IncrementToken error: %v", err)
	} else if ok {
		extra := ""
		if termAtt != nil {
			extra = termAtt.String()
		}
		t.Errorf("TokenStream emitted more tokens than expected (count=%d); extra term=%q", len(want.Terms), extra)
	}

	// Bogus values again so we can verify End() also clears.
	src.ClearAttributes()
	if termAtt != nil {
		termAtt.SetValue("bogusTerm")
	}
	if offsetAtt != nil {
		offsetAtt.SetOffset(14584724, 24683243)
	}
	if typeAtt != nil {
		typeAtt.SetType("bogusType")
	}
	if posIncrAtt != nil {
		posIncrAtt.SetPositionIncrement(45987657)
	}
	if posLenAtt != nil {
		posLenAtt.SetPositionLength(45987653)
	}

	if err := ts.End(); err != nil {
		t.Fatalf("End error: %v", err)
	}

	if want.FinalOffset != nil {
		if got := offsetAtt.EndOffset(); got != *want.FinalOffset {
			t.Errorf("finalOffset: got %d, want %d", got, *want.FinalOffset)
		}
	}
	if offsetAtt != nil {
		if got := offsetAtt.EndOffset(); got < 0 {
			t.Errorf("finalOffset must be >= 0, got %d", got)
		}
	}
	if want.FinalPositionIncrement != nil {
		if got := posIncrAtt.GetPositionIncrement(); got != *want.FinalPositionIncrement {
			t.Errorf("finalPosInc: got %d, want %d", got, *want.FinalPositionIncrement)
		}
	}

	if err := ts.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

// AssertTokenStreamContentsSimple is a convenience wrapper that only
// asserts term text — the equivalent of Lucene's two-arg
// assertTokenStreamContents(TokenStream, String[]).
func AssertTokenStreamContentsSimple(t TestingT, ts analysis.TokenStream, terms []string) {
	t.Helper()
	AssertTokenStreamContents(t, ts, TokenStreamExpectations{Terms: terms})
}

// AssertTokenStreamContentsTypes wraps [AssertTokenStreamContents] for
// the (terms, types) shape.
func AssertTokenStreamContentsTypes(t TestingT, ts analysis.TokenStream, terms []string, types []string) {
	t.Helper()
	AssertTokenStreamContents(t, ts, TokenStreamExpectations{Terms: terms, Types: types})
}

// AssertTokenStreamContentsPosInc wraps [AssertTokenStreamContents] for
// the (terms, posIncrements) shape.
func AssertTokenStreamContentsPosInc(t TestingT, ts analysis.TokenStream, terms []string, posIncrements []int) {
	t.Helper()
	AssertTokenStreamContents(t, ts, TokenStreamExpectations{Terms: terms, PositionIncrements: posIncrements})
}

// AssertTokenStreamContentsOffsets wraps [AssertTokenStreamContents]
// for the (terms, startOffsets, endOffsets) shape.
func AssertTokenStreamContentsOffsets(t TestingT, ts analysis.TokenStream, terms []string, startOffsets, endOffsets []int) {
	t.Helper()
	AssertTokenStreamContents(t, ts, TokenStreamExpectations{
		Terms:        terms,
		StartOffsets: startOffsets,
		EndOffsets:   endOffsets,
	})
}

// AssertTokenStreamContentsOffsetsFinal wraps
// [AssertTokenStreamContents] for the (terms, startOffsets,
// endOffsets, finalOffset) shape.
func AssertTokenStreamContentsOffsetsFinal(t TestingT, ts analysis.TokenStream, terms []string, startOffsets, endOffsets []int, finalOffset int) {
	t.Helper()
	fo := finalOffset
	AssertTokenStreamContents(t, ts, TokenStreamExpectations{
		Terms:        terms,
		StartOffsets: startOffsets,
		EndOffsets:   endOffsets,
		FinalOffset:  &fo,
	})
}

// IntPtr returns a pointer to v, useful when populating the optional
// *int fields of [TokenStreamExpectations] inline.
func IntPtr(v int) *int { return &v }

// Float32Ptr returns a pointer to v, useful when populating the
// optional *float32 fields of [TokenStreamExpectations] inline.
func Float32Ptr(v float32) *float32 { return &v }

// --- helpers ---------------------------------------------------------

// attributeSourceCarrier is satisfied by token-stream impls that
// expose their underlying util.AttributeSource via
// GetAttributeSource. [analysis.BaseTokenStream] and every concrete
// stream that embeds it implement this contract.
type attributeSourceCarrier interface {
	GetAttributeSource() *attrSrc
}

// We avoid importing util.AttributeSource by name in the carrier so
// the assertion stays in this file; use type-assertions on the
// runtime value instead.
type attrSrc = util.AttributeSource

func getAttributeSource(t TestingT, ts analysis.TokenStream) *attrSrc {
	t.Helper()
	if c, ok := ts.(attributeSourceCarrier); ok {
		return c.GetAttributeSource()
	}
	t.Fatalf("AssertTokenStreamContents: TokenStream impl %T does not expose GetAttributeSource", ts)
	return nil
}

// resetTokenStream invokes a Reset() method when the stream exposes
// one (mirroring Lucene's TokenStream.reset()). Streams without an
// explicit Reset (e.g. trivially canned ones) are accepted as-is.
func resetTokenStream(ts analysis.TokenStream) error {
	if r, ok := ts.(interface{ Reset() error }); ok {
		return r.Reset()
	}
	return nil
}

func validateSliceLengths(e TokenStreamExpectations) error {
	n := len(e.Terms)
	check := func(name string, length int) error {
		if length != n {
			return errLengthMismatch(name, length, n)
		}
		return nil
	}
	if e.StartOffsets != nil {
		if err := check("StartOffsets", len(e.StartOffsets)); err != nil {
			return err
		}
	}
	if e.EndOffsets != nil {
		if err := check("EndOffsets", len(e.EndOffsets)); err != nil {
			return err
		}
	}
	if e.Types != nil {
		if err := check("Types", len(e.Types)); err != nil {
			return err
		}
	}
	if e.PositionIncrements != nil {
		if err := check("PositionIncrements", len(e.PositionIncrements)); err != nil {
			return err
		}
	}
	if e.PositionLengths != nil {
		if err := check("PositionLengths", len(e.PositionLengths)); err != nil {
			return err
		}
	}
	if e.KeywordAtts != nil {
		if err := check("KeywordAtts", len(e.KeywordAtts)); err != nil {
			return err
		}
	}
	if e.Payloads != nil {
		if err := check("Payloads", len(e.Payloads)); err != nil {
			return err
		}
	}
	if e.Flags != nil {
		if err := check("Flags", len(e.Flags)); err != nil {
			return err
		}
	}
	if e.Boost != nil {
		if err := check("Boost", len(e.Boost)); err != nil {
			return err
		}
	}
	return nil
}

type lengthMismatchErr struct {
	name string
	got  int
	want int
}

func (e *lengthMismatchErr) Error() string {
	return e.name + ": length mismatch (got " + itoa(e.got) + ", want " + itoa(e.want) + ")"
}

func errLengthMismatch(name string, got, want int) error {
	return &lengthMismatchErr{name: name, got: got, want: want}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func float32Equal(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
