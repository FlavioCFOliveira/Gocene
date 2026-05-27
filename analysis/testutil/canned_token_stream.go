// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package testutil holds analysis-side test helpers ported from
// Apache Lucene 10.4.0's lucene-test-framework. Sprint 116 T4688
// introduces [CannedTokenStream] (and the value-type [Token]) as the
// smallest viable slice of the broader test-framework port (see
// sibling tasks T4689 BaseTokenStreamTestCase, T4690
// MockDirectoryWrapper, T4691 RandomIndexWriter).
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/analysis/CannedTokenStream.java
//	lucene/test-framework/src/java/org/apache/lucene/tests/analysis/Token.java
package testutil

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// Token is a pre-built, value-shaped token used to seed a
// [CannedTokenStream]. It mirrors the field set of Lucene 10.4.0's
// org.apache.lucene.tests.analysis.Token (which extends
// PackedTokenAttributeImpl plus FlagsAttribute/PayloadAttribute):
// term text, character offsets, lexical type, position increment,
// position length, flags, and an optional payload.
//
// Token is intentionally a plain value type rather than an
// AttributeImpl: the only consumer is [CannedTokenStream], which
// copies each Token's fields into the live attributes registered on
// the stream. This keeps the test-only Token out of the production
// AttributeSource hot path while preserving Lucene-faithful semantics.
//
// The zero value yields an empty term, offsets 0/0, type "word",
// position increment 1, position length 1, no flags, no payload —
// matching the defaults of PackedTokenAttributeImpl.
type Token struct {
	// Text is the term text written into CharTermAttribute.
	Text string

	// StartOffset is the inclusive character start offset.
	StartOffset int

	// EndOffset is the exclusive character end offset.
	EndOffset int

	// Type is the lexical type (defaults to "word" when empty).
	Type string

	// PositionIncrement is the gap to the previous token (default 1).
	PositionIncrement int

	// PositionLength is the span this token occupies (default 1).
	PositionLength int

	// Flags is the raw flag bitfield (default 0).
	Flags int

	// Payload is the optional metadata bytes (default nil).
	// A non-nil payload is copied into the stream's PayloadAttribute
	// by value; mutating the slice after construction is undefined.
	Payload []byte

	// typeSet tracks whether Type was explicitly set, so that an
	// empty string maps to the Lucene-faithful default "word"
	// rather than overwriting the attribute with "".
	typeSet bool

	// positionIncrementSet / positionLengthSet preserve the
	// distinction between "use the default" and "explicitly 0/N",
	// matching Lucene's per-field setter semantics.
	positionIncrementSet bool
	positionLengthSet    bool
}

// NewToken builds a [Token] with term text and start/end offsets,
// mirroring Lucene's Token(CharSequence text, int start, int end).
// All other fields take Lucene defaults.
func NewToken(text string, start, end int) Token {
	return Token{
		Text:        text,
		StartOffset: start,
		EndOffset:   end,
	}
}

// NewTokenWithPosInc builds a [Token] with term text, position
// increment, and start/end offsets, mirroring Lucene's
// Token(CharSequence text, int posInc, int start, int end).
func NewTokenWithPosInc(text string, posInc, start, end int) Token {
	return Token{
		Text:                 text,
		StartOffset:          start,
		EndOffset:            end,
		PositionIncrement:    posInc,
		positionIncrementSet: true,
	}
}

// NewTokenWithPosIncAndLength builds a [Token] with term text,
// position increment, start/end offsets, and position length,
// mirroring Lucene's
// Token(CharSequence text, int posInc, int start, int end, int posLength).
func NewTokenWithPosIncAndLength(text string, posInc, start, end, posLength int) Token {
	return Token{
		Text:                 text,
		StartOffset:          start,
		EndOffset:            end,
		PositionIncrement:    posInc,
		positionIncrementSet: true,
		PositionLength:       posLength,
		positionLengthSet:    true,
	}
}

// WithType returns a copy of t with its lexical Type set to the given
// value. An explicit empty string is honoured rather than coerced.
func (t Token) WithType(typeName string) Token {
	t.Type = typeName
	t.typeSet = true
	return t
}

// WithFlags returns a copy of t with the raw Flags bitfield set.
func (t Token) WithFlags(flags int) Token {
	t.Flags = flags
	return t
}

// WithPayload returns a copy of t with its Payload set. A nil payload
// clears any previous payload.
func (t Token) WithPayload(payload []byte) Token {
	t.Payload = payload
	return t
}

// CannedTokenStream is a [analysis.TokenStream] that emits a
// pre-built slice of [Token] values in order, then signals end of
// stream. It is the Go port of Lucene 10.4.0's
// org.apache.lucene.tests.analysis.CannedTokenStream.
//
// The stream registers the same set of attributes that Lucene's
// CannedTokenStream produces (via PackedTokenAttributeImpl plus
// FlagsAttribute and PayloadAttribute): CharTerm, Offset, Type,
// PositionIncrement, PositionLength, Flags, Payload. Each call to
// [CannedTokenStream.IncrementToken] clears the attributes and copies
// the next [Token] into them. After the last token, the stream's
// final End() applies the trailing finalOffset and finalPosInc set
// at construction time (defaults: 0 and 0, matching Lucene).
//
// CannedTokenStream is intended for tests only; it must not be used
// inside the production analysis pipeline.
type CannedTokenStream struct {
	*analysis.BaseTokenStream

	tokens      []Token
	upto        int
	finalOffset int
	finalPosInc int
	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	typeAttr    analysis.TypeAttribute
	posIncrAttr analysis.PositionIncrementAttribute
	posLenAttr  analysis.PositionLengthAttribute
	flagsAttr   *analysis.FlagsAttributeImpl
	payloadAttr *analysis.PayloadAttributeImpl
}

// Compile-time interface assertion.
var _ analysis.TokenStream = (*CannedTokenStream)(nil)

// NewCannedTokenStream constructs a [CannedTokenStream] over the
// given tokens, with finalPosInc and finalOffset both 0 — matching
// the Lucene zero-argument-besides-tokens constructor.
func NewCannedTokenStream(tokens ...Token) *CannedTokenStream {
	return NewCannedTokenStreamWithFinal(0, 0, tokens...)
}

// NewCannedTokenStreamWithFinal constructs a [CannedTokenStream] over
// the given tokens, with a custom finalPosInc and finalOffset applied
// by [CannedTokenStream.End]. Mirrors Lucene's
// CannedTokenStream(int finalPosInc, int finalOffset, Token... tokens).
func NewCannedTokenStreamWithFinal(finalPosInc, finalOffset int, tokens ...Token) *CannedTokenStream {
	cts := &CannedTokenStream{
		BaseTokenStream: analysis.NewBaseTokenStream(),
		tokens:          tokens,
		finalOffset:     finalOffset,
		finalPosInc:     finalPosInc,
	}

	// Register the attribute set that Lucene's CannedTokenStream
	// exposes via Token.TOKEN_ATTRIBUTE_FACTORY. We use the
	// individual concrete impls rather than PackedTokenAttributeImpl
	// to keep the stream's surface easy to inspect from tests.
	cts.termAttr = analysis.NewCharTermAttribute()
	cts.offsetAttr = analysis.NewOffsetAttribute()
	cts.typeAttr = analysis.NewTypeAttributeImpl()
	cts.posIncrAttr = analysis.NewPositionIncrementAttribute()
	cts.posLenAttr = analysis.NewPositionLengthAttributeImpl()
	cts.flagsAttr = analysis.NewFlagsAttributeImpl()
	cts.payloadAttr = analysis.NewPayloadAttributeImpl()

	cts.AddAttribute(cts.termAttr)
	cts.AddAttribute(cts.offsetAttr)
	cts.AddAttribute(cts.typeAttr)
	cts.AddAttribute(cts.posIncrAttr)
	cts.AddAttribute(cts.posLenAttr)
	cts.AddAttribute(cts.flagsAttr)
	cts.AddAttribute(cts.payloadAttr)

	return cts
}

// IncrementToken advances to the next canned token. Returns
// (true, nil) when a token was emitted, or (false, nil) at end of
// stream. Never returns an error — the canned source cannot fail.
//
// Each call clears all attributes and copies the next Token's
// fields into the live attribute impls, applying the Lucene-faithful
// defaults (type "word", posInc 1, posLen 1) when the Token did not
// explicitly set them.
func (cts *CannedTokenStream) IncrementToken() (bool, error) {
	if cts.upto >= len(cts.tokens) {
		return false, nil
	}

	cts.ClearAttributes()
	tok := cts.tokens[cts.upto]
	cts.upto++

	cts.termAttr.SetValue(tok.Text)
	cts.offsetAttr.SetOffset(tok.StartOffset, tok.EndOffset)

	if tok.typeSet {
		cts.typeAttr.SetType(tok.Type)
	} else {
		cts.typeAttr.SetType(analysis.DefaultTokenType)
	}

	if tok.positionIncrementSet {
		cts.posIncrAttr.SetPositionIncrement(tok.PositionIncrement)
	} else {
		cts.posIncrAttr.SetPositionIncrement(1)
	}

	if tok.positionLengthSet {
		cts.posLenAttr.SetPositionLength(tok.PositionLength)
	} else {
		cts.posLenAttr.SetPositionLength(1)
	}

	cts.flagsAttr.SetFlags(tok.Flags)

	if tok.Payload != nil {
		// Copy the payload so subsequent mutations to the source
		// slice do not leak into the stream's attribute state.
		dup := make([]byte, len(tok.Payload))
		copy(dup, tok.Payload)
		cts.payloadAttr.SetPayload(dup)
	} else {
		cts.payloadAttr.SetPayload(nil)
	}

	return true, nil
}

// Reset rewinds the stream to the beginning of the canned sequence.
func (cts *CannedTokenStream) Reset() error {
	cts.upto = 0
	return nil
}

// End applies the trailing finalPosInc and finalOffset, mirroring
// Lucene's CannedTokenStream.end().
func (cts *CannedTokenStream) End() error {
	cts.posIncrAttr.SetPositionIncrement(cts.finalPosInc)
	cts.offsetAttr.SetOffset(cts.finalOffset, cts.finalOffset)
	return nil
}

// Close releases resources. The canned stream owns no external
// handles, so Close is a no-op that exists to satisfy the
// [analysis.TokenStream] contract.
func (cts *CannedTokenStream) Close() error { return nil }

// CharTermAttribute exposes the live CharTermAttribute for callers
// that want to inspect the current token's text without going through
// the AttributeSource lookup.
func (cts *CannedTokenStream) CharTermAttribute() analysis.CharTermAttribute {
	return cts.termAttr
}

// OffsetAttribute exposes the live OffsetAttribute.
func (cts *CannedTokenStream) OffsetAttribute() analysis.OffsetAttribute {
	return cts.offsetAttr
}

// TypeAttribute exposes the live TypeAttribute.
func (cts *CannedTokenStream) TypeAttribute() analysis.TypeAttribute {
	return cts.typeAttr
}

// PositionIncrementAttribute exposes the live PositionIncrementAttribute.
func (cts *CannedTokenStream) PositionIncrementAttribute() analysis.PositionIncrementAttribute {
	return cts.posIncrAttr
}

// PositionLengthAttribute exposes the live PositionLengthAttribute.
func (cts *CannedTokenStream) PositionLengthAttribute() analysis.PositionLengthAttribute {
	return cts.posLenAttr
}

// FlagsAttribute exposes the live FlagsAttribute.
func (cts *CannedTokenStream) FlagsAttribute() *analysis.FlagsAttributeImpl {
	return cts.flagsAttr
}

// PayloadAttribute exposes the live PayloadAttribute.
func (cts *CannedTokenStream) PayloadAttribute() *analysis.PayloadAttributeImpl {
	return cts.payloadAttr
}
