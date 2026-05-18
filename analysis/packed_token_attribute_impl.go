// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PackedTokenAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.PackedTokenAttributeImpl.
//
// It packs the most common token attributes into a single struct:
//
//   - [CharTermAttribute] (the term text and its TermToBytesRef view)
//   - [TypeAttribute]
//   - [PositionIncrementAttribute]
//   - [PositionLengthAttribute]
//   - [OffsetAttribute]
//   - [TermFrequencyAttribute]
//
// The Java reference extends CharTermAttributeImpl; the Go port
// composes a *charTermAttribute by embedding so the impl inherits the
// CharTerm surface area without inheritance.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/PackedTokenAttributeImpl.java
type PackedTokenAttributeImpl struct {
	*charTermAttribute

	startOffset       int
	endOffset         int
	tokenType         string
	positionIncrement int
	positionLength    int
	termFrequency     int
}

// Compile-time assertions to lock in every interface contract this
// impl participates in. The Sprint 12 option (d) snapshot keeps
// PositionLengthAttribute, TypeAttribute and TermFrequencyAttribute as
// bare structs (no interface+impl split), so contract enforcement for
// those is method-set-driven rather than interface-driven; the bundle
// is verified by the [PackedTokenAttributeImpl_BareStructParity] test.
var (
	_ AttributeImpl              = (*PackedTokenAttributeImpl)(nil)
	_ CharTermAttribute          = (*PackedTokenAttributeImpl)(nil)
	_ TermToBytesRefAttribute    = (*PackedTokenAttributeImpl)(nil)
	_ OffsetAttribute            = (*PackedTokenAttributeImpl)(nil)
	_ PositionIncrementAttribute = (*PackedTokenAttributeImpl)(nil)
	_ AttributeReflectable       = (*PackedTokenAttributeImpl)(nil)
	_ AttributeEnder             = (*PackedTokenAttributeImpl)(nil)
)

// NewPackedTokenAttributeImpl initialises this impl with the Lucene
// defaults: empty term, type "word", positionIncrement 1,
// positionLength 1, termFrequency 1, offsets 0/0.
func NewPackedTokenAttributeImpl() *PackedTokenAttributeImpl {
	return &PackedTokenAttributeImpl{
		charTermAttribute: NewCharTermAttribute().(*charTermAttribute),
		tokenType:         DefaultTokenType,
		positionIncrement: 1,
		positionLength:    1,
		termFrequency:     1,
	}
}

// DefaultTokenType mirrors {@code TypeAttribute#DEFAULT_TYPE} ("word").
const DefaultTokenType = "word"

// --- OffsetAttribute -------------------------------------------------

// StartOffset returns the inclusive start offset.
func (p *PackedTokenAttributeImpl) StartOffset() int { return p.startOffset }

// EndOffset returns the exclusive end offset.
func (p *PackedTokenAttributeImpl) EndOffset() int { return p.endOffset }

// SetStartOffset is the legacy per-field setter retained for back-compat.
func (p *PackedTokenAttributeImpl) SetStartOffset(offset int) { p.startOffset = offset }

// SetEndOffset is the legacy per-field setter retained for back-compat.
func (p *PackedTokenAttributeImpl) SetEndOffset(offset int) { p.endOffset = offset }

// SetOffset is the Lucene-faithful combined setter. It panics on
// invalid input, matching the IllegalArgumentException thrown by
// PackedTokenAttributeImpl#setOffset.
func (p *PackedTokenAttributeImpl) SetOffset(startOffset, endOffset int) {
	if startOffset < 0 || endOffset < startOffset {
		panic(fmt.Sprintf(
			"PackedTokenAttributeImpl.SetOffset: startOffset must be non-negative and endOffset must be >= startOffset; got startOffset=%d, endOffset=%d",
			startOffset, endOffset))
	}
	p.startOffset = startOffset
	p.endOffset = endOffset
}

// --- TypeAttribute ---------------------------------------------------

// GetType returns the lexical type (defaults to "word").
func (p *PackedTokenAttributeImpl) GetType() string { return p.tokenType }

// SetType sets the lexical type.
func (p *PackedTokenAttributeImpl) SetType(tokenType string) { p.tokenType = tokenType }

// --- PositionIncrementAttribute -------------------------------------

// GetPositionIncrement returns the position increment.
func (p *PackedTokenAttributeImpl) GetPositionIncrement() int { return p.positionIncrement }

// SetPositionIncrement panics on negative input, matching Lucene.
func (p *PackedTokenAttributeImpl) SetPositionIncrement(positionIncrement int) {
	if positionIncrement < 0 {
		panic(fmt.Sprintf(
			"PackedTokenAttributeImpl.SetPositionIncrement: increment must be zero or greater; got %d",
			positionIncrement))
	}
	p.positionIncrement = positionIncrement
}

// --- PositionLengthAttribute ----------------------------------------

// GetPositionLength returns the position length.
func (p *PackedTokenAttributeImpl) GetPositionLength() int { return p.positionLength }

// SetPositionLength panics when positionLength < 1, matching Lucene.
func (p *PackedTokenAttributeImpl) SetPositionLength(positionLength int) {
	if positionLength < 1 {
		panic(fmt.Sprintf(
			"PackedTokenAttributeImpl.SetPositionLength: position length must be 1 or greater; got %d",
			positionLength))
	}
	p.positionLength = positionLength
}

// --- TermFrequencyAttribute -----------------------------------------

// GetTermFrequency returns the term frequency.
func (p *PackedTokenAttributeImpl) GetTermFrequency() int { return p.termFrequency }

// SetTermFrequency panics when termFrequency < 1, matching Lucene.
func (p *PackedTokenAttributeImpl) SetTermFrequency(termFrequency int) {
	if termFrequency < 1 {
		panic(fmt.Sprintf(
			"PackedTokenAttributeImpl.SetTermFrequency: term frequency must be 1 or greater; got %d",
			termFrequency))
	}
	p.termFrequency = termFrequency
}

// --- AttributeImpl ---------------------------------------------------

// Clear resets every packed attribute to its default value: empty term,
// type "word", positionIncrement = positionLength = termFrequency = 1,
// offsets = 0/0.
func (p *PackedTokenAttributeImpl) Clear() {
	p.charTermAttribute.Clear()
	p.positionIncrement = 1
	p.positionLength = 1
	p.termFrequency = 1
	p.startOffset = 0
	p.endOffset = 0
	p.tokenType = DefaultTokenType
}

// End mirrors Lucene's {@code end()}: clear all packed fields, then
// set positionIncrement to 0 (the only value that differs from the
// Clear default at end-of-field).
func (p *PackedTokenAttributeImpl) End() {
	p.Clear()
	p.positionIncrement = 0
}

// CopyTo copies this impl's packed state onto target. When target is
// itself a [PackedTokenAttributeImpl] the copy is fast-pathed
// (single-shot field copy); otherwise CopyTo dispatches against the
// individual attribute interfaces, matching the Lucene fallback path.
func (p *PackedTokenAttributeImpl) CopyTo(target AttributeImpl) {
	if to, ok := target.(*PackedTokenAttributeImpl); ok {
		// Mirror the fast path in PackedTokenAttributeImpl#copyTo: copy
		// the term buffer and every packed field directly.
		to.charTermAttribute.SetValue(p.charTermAttribute.String())
		to.positionIncrement = p.positionIncrement
		to.positionLength = p.positionLength
		to.startOffset = p.startOffset
		to.endOffset = p.endOffset
		to.tokenType = p.tokenType
		to.termFrequency = p.termFrequency
		return
	}
	// Fallback: defer to CharTermAttributeImpl#copyTo for the term
	// buffer, then forward each remaining attribute. Interface targets
	// are routed through the interface (Lucene parity); bare-struct
	// attributes (PositionLengthAttribute, TypeAttribute and
	// TermFrequencyAttribute under Sprint 12 option d) are matched as
	// pointer-to-struct, since they ship no Lucene-style interface yet.
	p.charTermAttribute.CopyTo(target)
	if t, ok := target.(OffsetAttribute); ok {
		t.SetOffset(p.startOffset, p.endOffset)
	}
	if t, ok := target.(PositionIncrementAttribute); ok {
		t.SetPositionIncrement(p.positionIncrement)
	}
	if t, ok := target.(*PositionLengthAttribute); ok {
		t.SetPositionLength(p.positionLength)
	}
	if t, ok := target.(*TypeAttribute); ok {
		t.SetType(p.tokenType)
	}
	if t, ok := target.(*TermFrequencyAttribute); ok {
		t.SetTermFrequency(p.termFrequency)
	}
}

// Copy returns a deep clone of this impl.
func (p *PackedTokenAttributeImpl) Copy() AttributeImpl {
	clone := NewPackedTokenAttributeImpl()
	p.CopyTo(clone)
	return clone
}

// CloneAttribute implements util.AttributeImpl.CloneAttribute. Returns
// a deep copy as util.AttributeImpl. Delegates to the existing Copy().
func (p *PackedTokenAttributeImpl) CloneAttribute() util.AttributeImpl { return p.Copy() }

// ReflectWith emits the exact triple set required by the Lucene
// reference: the CharTermAttributeImpl reflection (term + bytes
// triples) plus startOffset/endOffset/positionIncrement/positionLength/
// type/termFrequency.
func (p *PackedTokenAttributeImpl) ReflectWith(reflector AttributeReflector) {
	p.charTermAttribute.ReflectWith(reflector)
	offsetType := reflect.TypeOf((*OffsetAttribute)(nil)).Elem()
	reflector(offsetType, "startOffset", p.startOffset)
	reflector(offsetType, "endOffset", p.endOffset)
	reflector(reflect.TypeOf((*PositionIncrementAttribute)(nil)).Elem(),
		"positionIncrement", p.positionIncrement)
	reflector(reflect.TypeOf((*PositionLengthAttribute)(nil)).Elem(),
		"positionLength", p.positionLength)
	reflector(reflect.TypeOf((*TypeAttribute)(nil)).Elem(), "type", p.tokenType)
	reflector(reflect.TypeOf((*TermFrequencyAttribute)(nil)).Elem(),
		"termFrequency", p.termFrequency)
}

// Equals returns true if other is a [PackedTokenAttributeImpl] whose
// packed fields and embedded term content compare equal, matching the
// Lucene reference.
func (p *PackedTokenAttributeImpl) Equals(other any) bool {
	if p == other {
		return true
	}
	o, ok := other.(*PackedTokenAttributeImpl)
	if !ok {
		return false
	}
	if p.startOffset != o.startOffset ||
		p.endOffset != o.endOffset ||
		p.positionIncrement != o.positionIncrement ||
		p.positionLength != o.positionLength ||
		p.termFrequency != o.termFrequency ||
		p.tokenType != o.tokenType {
		return false
	}
	return p.charTermAttribute.Equals(o.charTermAttribute)
}

// HashCode mirrors Lucene's hash composition: super.hashCode() then
// 31 * h + field for each packed integer/string field.
func (p *PackedTokenAttributeImpl) HashCode() int {
	code := p.charTermAttribute.HashCode()
	code = code*31 + p.startOffset
	code = code*31 + p.endOffset
	code = code*31 + p.positionIncrement
	code = code*31 + p.positionLength
	if p.tokenType != "" {
		hash := 0
		for i := 0; i < len(p.tokenType); i++ {
			hash = hash*31 + int(int8(p.tokenType[i]))
		}
		code = code*31 + hash
	}
	code = code*31 + p.termFrequency
	return code
}
