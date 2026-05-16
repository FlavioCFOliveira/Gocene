// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"fmt"
	"reflect"
)

// This file aligns the bare-struct token attributes
// (TypeAttribute, PayloadAttribute, FlagsAttribute, KeywordAttribute,
// PositionLengthAttribute, TermFrequencyAttribute) with the Lucene
// 10.4.0 reference. Sprint 12 was scoped under option (d), which keeps
// the existing concrete-struct layout (no interface+impl split) to
// avoid touching the ~396 consumer references that depend on it.
//
// The original implementations in token_attributes.go already cover
// Clear / CopyTo / Copy / type-specific helpers. The methods below
// add Lucene-faithful parity:
//
//   - Equals + HashCode for every bare struct.
//   - ReflectWith opt-in (AttributeReflectable) for every bare struct.
//   - Validation panics on the setters whose Lucene counterparts throw
//     IllegalArgumentException (PositionLengthAttribute,
//     TermFrequencyAttribute).
//   - End opt-in (AttributeEnder) where the Lucene reference overrides
//     end() with a value distinct from Clear.
//
// The full Lucene-faithful interface+impl rewrite is tracked in the
// backlog task created at the end of Sprint 12.

// --- TypeAttribute ---------------------------------------------------

// Compile-time assertions for the opt-in surface.
var (
	_ AttributeReflectable = (*TypeAttribute)(nil)
)

// ReflectWith emits the single (TypeAttribute, "type", value) triple
// expected by Lucene's reference reflectWith. Because TypeAttribute is
// a bare struct (no Lucene-style interface yet under Sprint 12 option
// d), the attType passed to reflector is the concrete struct type.
func (ta *TypeAttribute) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf(ta), "type", ta.Type)
}

// Equals returns true if other is a [TypeAttribute] with the same
// Type string, matching Lucene's instance-of guard.
func (ta *TypeAttribute) Equals(other any) bool {
	if ta == other {
		return true
	}
	o, ok := other.(*TypeAttribute)
	if !ok {
		return false
	}
	return ta.Type == o.Type
}

// HashCode returns the Java-style hash of the Type string (or 0 when
// empty), matching {@code TypeAttributeImpl#hashCode()}.
func (ta *TypeAttribute) HashCode() int {
	return javaStringHash(ta.Type)
}

// --- PayloadAttribute ------------------------------------------------

var (
	_ AttributeReflectable = (*PayloadAttribute)(nil)
)

// ReflectWith emits the single (PayloadAttribute, "payload", value)
// triple. The Lucene reference uses BytesRef; the Gocene port keeps
// the existing []byte field for back-compat (see Sprint 12 deferral
// note).
func (pa *PayloadAttribute) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf(pa), "payload", pa.Payload)
}

// Equals returns true if other is a [PayloadAttribute] whose Payload
// is byte-wise equal (nil and empty are considered equal to nil,
// matching the Lucene fast-path).
func (pa *PayloadAttribute) Equals(other any) bool {
	if pa == other {
		return true
	}
	o, ok := other.(*PayloadAttribute)
	if !ok {
		return false
	}
	if pa.Payload == nil || o.Payload == nil {
		return pa.Payload == nil && o.Payload == nil
	}
	return bytes.Equal(pa.Payload, o.Payload)
}

// HashCode returns the Java-style byte-array hash of the Payload, or
// 0 when nil — matching {@code Objects.hashCode(payload)} for a nil
// reference and {@code Arrays.hashCode} for a populated one.
func (pa *PayloadAttribute) HashCode() int {
	if pa.Payload == nil {
		return 0
	}
	code := 1
	for _, b := range pa.Payload {
		code = code*31 + int(int8(b))
	}
	return code
}

// --- FlagsAttribute --------------------------------------------------

var (
	_ AttributeReflectable = (*FlagsAttribute)(nil)
)

// ReflectWith emits the single (FlagsAttribute, "flags", value) triple
// expected by Lucene's reference reflectWith.
func (fa *FlagsAttribute) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf(fa), "flags", fa.Flags)
}

// Equals returns true if other is a [FlagsAttribute] with the same
// flags, matching Lucene's instance-of guard.
func (fa *FlagsAttribute) Equals(other any) bool {
	if fa == other {
		return true
	}
	o, ok := other.(*FlagsAttribute)
	if !ok {
		return false
	}
	return fa.Flags == o.Flags
}

// HashCode returns flags itself, matching Lucene's
// {@code FlagsAttributeImpl#hashCode()}.
func (fa *FlagsAttribute) HashCode() int { return fa.Flags }

// --- KeywordAttribute ------------------------------------------------

var (
	_ AttributeReflectable = (*KeywordAttribute)(nil)
)

// ReflectWith emits the single (KeywordAttribute, "keyword", value)
// triple expected by Lucene's reference reflectWith.
func (ka *KeywordAttribute) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf(ka), "keyword", ka.IsKeyword)
}

// Equals returns true if other is a [KeywordAttribute] whose IsKeyword
// flag matches, mirroring the Lucene equals contract.
func (ka *KeywordAttribute) Equals(other any) bool {
	if ka == other {
		return true
	}
	o, ok := other.(*KeywordAttribute)
	if !ok {
		return false
	}
	return ka.IsKeyword == o.IsKeyword
}

// HashCode mirrors Lucene's {@code KeywordAttributeImpl#hashCode()}:
// 31 when keyword=true, 37 otherwise.
func (ka *KeywordAttribute) HashCode() int {
	if ka.IsKeyword {
		return 31
	}
	return 37
}

// --- PositionLengthAttribute -----------------------------------------

var (
	_ AttributeReflectable = (*PositionLengthAttribute)(nil)
)

// ReflectWith emits the single (PositionLengthAttribute,
// "positionLength", value) triple expected by Lucene.
func (pla *PositionLengthAttribute) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf(pla), "positionLength", pla.PositionLength)
}

// SetPositionLengthValidated wraps SetPositionLength with the Lucene
// validation rule: positionLength < 1 panics, mirroring the
// IllegalArgumentException thrown by
// PositionLengthAttributeImpl#setPositionLength. The legacy unchecked
// SetPositionLength setter is retained for back-compat (see Sprint 12
// option d) but new code should prefer this variant.
func (pla *PositionLengthAttribute) SetPositionLengthValidated(length int) {
	if length < 1 {
		panic(fmt.Sprintf(
			"PositionLengthAttribute.SetPositionLengthValidated: position length must be 1 or greater; got %d",
			length))
	}
	pla.PositionLength = length
}

// Equals returns true if other is a [PositionLengthAttribute] with the
// same positionLength, matching Lucene's instance-of guard.
func (pla *PositionLengthAttribute) Equals(other any) bool {
	if pla == other {
		return true
	}
	o, ok := other.(*PositionLengthAttribute)
	if !ok {
		return false
	}
	return pla.PositionLength == o.PositionLength
}

// HashCode returns positionLength itself, matching Lucene's
// {@code PositionLengthAttributeImpl#hashCode()}.
func (pla *PositionLengthAttribute) HashCode() int { return pla.PositionLength }

// --- TermFrequencyAttribute ------------------------------------------

var (
	_ AttributeReflectable = (*TermFrequencyAttribute)(nil)
	_ AttributeEnder       = (*TermFrequencyAttribute)(nil)
)

// ReflectWith emits the single (TermFrequencyAttribute,
// "termFrequency", value) triple expected by Lucene.
func (tfa *TermFrequencyAttribute) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf(tfa), "termFrequency", tfa.TermFrequency)
}

// SetTermFrequencyValidated wraps SetTermFrequency with the Lucene
// validation rule: termFrequency < 1 panics, mirroring the
// IllegalArgumentException thrown by
// TermFrequencyAttributeImpl#setTermFrequency. The legacy unchecked
// SetTermFrequency setter is retained for back-compat.
func (tfa *TermFrequencyAttribute) SetTermFrequencyValidated(freq int) {
	if freq < 1 {
		panic(fmt.Sprintf(
			"TermFrequencyAttribute.SetTermFrequencyValidated: term frequency must be 1 or greater; got %d",
			freq))
	}
	tfa.TermFrequency = freq
}

// End mirrors Lucene's {@code TermFrequencyAttributeImpl#end()}: reset
// to 1, identical to Clear. Lucene declares end() explicitly to
// document the intent, so the Go port follows suit.
func (tfa *TermFrequencyAttribute) End() {
	tfa.TermFrequency = 1
}

// Equals returns true if other is a [TermFrequencyAttribute] with the
// same termFrequency, matching Lucene's instance-of guard.
func (tfa *TermFrequencyAttribute) Equals(other any) bool {
	if tfa == other {
		return true
	}
	o, ok := other.(*TermFrequencyAttribute)
	if !ok {
		return false
	}
	return tfa.TermFrequency == o.TermFrequency
}

// HashCode returns termFrequency itself, matching Lucene's
// {@code Integer.hashCode(termFrequency)} (which is the value itself
// for non-negative ints).
func (tfa *TermFrequencyAttribute) HashCode() int { return tfa.TermFrequency }

// --- helpers ---------------------------------------------------------

// javaStringHash computes the Java {@code String.hashCode()} of s.
// Implementation: starts at 0 and iterates each UTF-16 code unit; for
// pure ASCII strings (the common Lucene case for type tokens like
// "word") the value is identical to iterating bytes.
func javaStringHash(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = h*31 + int(int8(s[i]))
	}
	return h
}
