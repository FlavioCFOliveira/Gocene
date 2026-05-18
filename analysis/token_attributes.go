// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file defines the six "small" token attributes shipped by the
// Gocene port of Lucene 10.4.0 as interface+impl pairs:
//
//   - TypeAttribute / typeAttributeImpl
//   - PayloadAttribute / payloadAttributeImpl
//   - FlagsAttribute / flagsAttributeImpl
//   - KeywordAttribute / keywordAttributeImpl
//   - PositionLengthAttribute / positionLengthAttributeImpl
//   - TermFrequencyAttribute / termFrequencyAttributeImpl
//
// Sprint 12 originally shipped these as bare structs to avoid the large
// consumer-migration cost; Sprint 54 Phase 3 promotes them to the
// Lucene-faithful interface+impl layout that the rest of Gocene's
// attribute surface (CharTermAttribute, OffsetAttribute,
// PackedTokenAttributeImpl, ...) already follows.
//
// The interface methods use the Go-idiomatic Get*/Set* naming preserved
// throughout Gocene's attribute layer, not Lucene's raw Java getter
// names (e.g. type() / setType(String)). Each concrete impl embeds
// [util.BaseAttributeImpl] and satisfies [util.AttributeImpl], plus the
// optional [AttributeReflectable] / [AttributeEnder] surfaces.

// --- TypeAttribute ---------------------------------------------------

// TypeAttributeType is the reflect.Type of the [TypeAttribute]
// interface, used as the lookup key for an [AttributeSource].
var TypeAttributeType = reflect.TypeOf((*TypeAttribute)(nil)).Elem()

// DefaultTypeAttributeValue mirrors {@code TypeAttribute#DEFAULT_TYPE}
// ("word"), the value to which Clear() resets the token type.
const DefaultTypeAttributeValue = "word"

// TypeAttribute provides a way to store the token type.
//
// This is the Go port of
// org.apache.lucene.analysis.tokenattributes.TypeAttribute. Token types
// are short strings such as "word", "acronym", "<ALPHANUM>", ... emitted
// by tokenizers and consumed by downstream filters.
//
// The interface embeds [AttributeImpl] so concrete values can be used
// uniformly with the rest of Gocene's attribute layer (mirroring the
// CharTermAttribute / OffsetAttribute conventions); [util.Attribute] is
// the empty Lucene marker that AttributeImpl already carries via
// embedding so it is not declared again here.
type TypeAttribute interface {
	AttributeImpl

	// GetType returns the current token type.
	GetType() string

	// SetType replaces the token type.
	SetType(tokenType string)
}

// typeAttributeImpl is the default [TypeAttribute] implementation. It
// embeds [util.BaseAttributeImpl] for the no-op End default and provides
// the full [util.AttributeImpl] surface (Clear, CopyTo, ReflectWith,
// CloneAttribute) plus Equals/HashCode for Lucene parity.
type typeAttributeImpl struct {
	util.BaseAttributeImpl
	tokenType string
}

// Compile-time assertions lock in the contracts this impl participates
// in.
var (
	_ TypeAttribute                   = (*typeAttributeImpl)(nil)
	_ util.AttributeImpl              = (*typeAttributeImpl)(nil)
	_ AttributeImpl                   = (*typeAttributeImpl)(nil)
	_ AttributeReflectable            = (*typeAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*typeAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (ta *typeAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{TypeAttributeType}
}

// NewTypeAttribute creates a new TypeAttribute with the default type
// "word".
func NewTypeAttribute() TypeAttribute {
	return &typeAttributeImpl{tokenType: DefaultTypeAttributeValue}
}

// GetType returns the token type.
func (ta *typeAttributeImpl) GetType() string { return ta.tokenType }

// SetType replaces the token type.
func (ta *typeAttributeImpl) SetType(tokenType string) { ta.tokenType = tokenType }

// Clear resets the type to the Lucene default ("word"), matching
// {@code TypeAttributeImpl#clear()}.
func (ta *typeAttributeImpl) Clear() { ta.tokenType = DefaultTypeAttributeValue }

// CopyTo copies this attribute's state onto target. Any target
// satisfying [TypeAttribute] is supported; mismatched targets are
// silently ignored, mirroring the Lucene fallback.
func (ta *typeAttributeImpl) CopyTo(target AttributeImpl) {
	if t, ok := target.(TypeAttribute); ok {
		t.SetType(ta.tokenType)
	}
}

// CloneAttribute returns a deep clone of this impl as a
// [util.AttributeImpl].
func (ta *typeAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &typeAttributeImpl{tokenType: ta.tokenType}
}

// ReflectWith emits the single (TypeAttribute, "type", value) triple
// expected by Lucene's reference reflectWith.
func (ta *typeAttributeImpl) ReflectWith(reflector AttributeReflector) {
	reflector(TypeAttributeType, "type", ta.tokenType)
}

// Equals returns true if other is a [TypeAttribute] with the same type
// string. Matches both the concrete impl pointer and any other impl
// satisfying the interface, mirroring Lucene's instance-of guard.
func (ta *typeAttributeImpl) Equals(other any) bool {
	if ta == other {
		return true
	}
	o, ok := other.(TypeAttribute)
	if !ok {
		return false
	}
	return ta.tokenType == o.GetType()
}

// HashCode returns the Java-style hash of the type string (or 0 when
// empty), matching {@code TypeAttributeImpl#hashCode()}.
func (ta *typeAttributeImpl) HashCode() int {
	return javaStringHash(ta.tokenType)
}

// --- PayloadAttribute ------------------------------------------------

// PayloadAttributeType is the reflect.Type of the [PayloadAttribute]
// interface, used as the lookup key for an [AttributeSource].
var PayloadAttributeType = reflect.TypeOf((*PayloadAttribute)(nil)).Elem()

// PayloadAttribute provides a way to store a payload for a token.
// Payloads are arbitrary byte arrays that can be associated with
// tokens.
//
// This is the Go port of
// org.apache.lucene.analysis.tokenattributes.PayloadAttribute.
type PayloadAttribute interface {
	AttributeImpl

	// GetPayload returns the current payload byte slice (or nil).
	GetPayload() []byte

	// SetPayload replaces the payload. A nil argument clears the
	// payload; otherwise the bytes are copied to avoid sharing.
	SetPayload(payload []byte)

	// HasPayload reports whether the attribute holds a non-empty
	// payload.
	HasPayload() bool
}

// payloadAttributeImpl is the default [PayloadAttribute] implementation.
type payloadAttributeImpl struct {
	util.BaseAttributeImpl
	payload []byte
}

var (
	_ PayloadAttribute                = (*payloadAttributeImpl)(nil)
	_ util.AttributeImpl              = (*payloadAttributeImpl)(nil)
	_ AttributeImpl                   = (*payloadAttributeImpl)(nil)
	_ AttributeReflectable            = (*payloadAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*payloadAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (pa *payloadAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{PayloadAttributeType}
}

// NewPayloadAttribute creates a new empty PayloadAttribute.
func NewPayloadAttribute() PayloadAttribute {
	return &payloadAttributeImpl{}
}

// NewPayloadAttributeWithPayload creates a new PayloadAttribute with
// the given payload (copied to avoid sharing).
func NewPayloadAttributeWithPayload(payload []byte) PayloadAttribute {
	pa := &payloadAttributeImpl{}
	pa.SetPayload(payload)
	return pa
}

// GetPayload returns the payload.
func (pa *payloadAttributeImpl) GetPayload() []byte { return pa.payload }

// SetPayload replaces the payload. Bytes are copied to prevent the
// caller's slice from aliasing the attribute's state.
func (pa *payloadAttributeImpl) SetPayload(payload []byte) {
	if payload == nil {
		pa.payload = nil
		return
	}
	pa.payload = make([]byte, len(payload))
	copy(pa.payload, payload)
}

// HasPayload returns true if this attribute has a non-empty payload.
func (pa *payloadAttributeImpl) HasPayload() bool {
	return len(pa.payload) > 0
}

// Clear resets the payload to nil.
func (pa *payloadAttributeImpl) Clear() { pa.payload = nil }

// CopyTo copies this attribute's state onto target.
func (pa *payloadAttributeImpl) CopyTo(target AttributeImpl) {
	if t, ok := target.(PayloadAttribute); ok {
		t.SetPayload(pa.payload)
	}
}

// CloneAttribute returns a deep clone of this impl.
func (pa *payloadAttributeImpl) CloneAttribute() util.AttributeImpl {
	cloned := &payloadAttributeImpl{}
	if pa.payload != nil {
		cloned.payload = make([]byte, len(pa.payload))
		copy(cloned.payload, pa.payload)
	}
	return cloned
}

// ReflectWith emits the single (PayloadAttribute, "payload", value)
// triple. The Lucene reference uses BytesRef; Gocene keeps []byte.
func (pa *payloadAttributeImpl) ReflectWith(reflector AttributeReflector) {
	reflector(PayloadAttributeType, "payload", pa.payload)
}

// Equals returns true if other is a [PayloadAttribute] whose payload is
// byte-wise equal. Two nil payloads compare equal (Lucene fast-path).
func (pa *payloadAttributeImpl) Equals(other any) bool {
	if pa == other {
		return true
	}
	o, ok := other.(PayloadAttribute)
	if !ok {
		return false
	}
	op := o.GetPayload()
	if pa.payload == nil || op == nil {
		return pa.payload == nil && op == nil
	}
	return bytes.Equal(pa.payload, op)
}

// HashCode returns the Java-style byte-array hash of the payload, or 0
// when nil (matches {@code Objects.hashCode(payload)} +
// {@code Arrays.hashCode}).
func (pa *payloadAttributeImpl) HashCode() int {
	if pa.payload == nil {
		return 0
	}
	code := 1
	for _, b := range pa.payload {
		code = code*31 + int(int8(b))
	}
	return code
}

// --- FlagsAttribute --------------------------------------------------

// FlagsAttributeType is the reflect.Type of the [FlagsAttribute]
// interface, used as the lookup key for an [AttributeSource].
var FlagsAttributeType = reflect.TypeOf((*FlagsAttribute)(nil)).Elem()

// FlagsAttribute provides a way to store custom flags for a token.
//
// This is the Go port of
// org.apache.lucene.analysis.tokenattributes.FlagsAttribute.
type FlagsAttribute interface {
	AttributeImpl

	// GetFlags returns the current flag bitmask.
	GetFlags() int

	// SetFlags replaces the flag bitmask.
	SetFlags(flags int)

	// IsFlagSet returns true when the given flag bit is present.
	IsFlagSet(flag int) bool

	// SetFlag toggles the given flag bit on or off in place.
	SetFlag(flag int, set bool)
}

// flagsAttributeImpl is the default [FlagsAttribute] implementation.
type flagsAttributeImpl struct {
	util.BaseAttributeImpl
	flags int
}

var (
	_ FlagsAttribute                  = (*flagsAttributeImpl)(nil)
	_ util.AttributeImpl              = (*flagsAttributeImpl)(nil)
	_ AttributeImpl                   = (*flagsAttributeImpl)(nil)
	_ AttributeReflectable            = (*flagsAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*flagsAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (fa *flagsAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{FlagsAttributeType}
}

// NewFlagsAttribute creates a new FlagsAttribute with flags set to 0.
func NewFlagsAttribute() FlagsAttribute {
	return &flagsAttributeImpl{}
}

// NewFlagsAttributeWithFlags creates a new FlagsAttribute with the
// given flags.
func NewFlagsAttributeWithFlags(flags int) FlagsAttribute {
	return &flagsAttributeImpl{flags: flags}
}

// GetFlags returns the flags.
func (fa *flagsAttributeImpl) GetFlags() int { return fa.flags }

// SetFlags replaces the flags.
func (fa *flagsAttributeImpl) SetFlags(flags int) { fa.flags = flags }

// IsFlagSet returns true if the given flag bit is set.
func (fa *flagsAttributeImpl) IsFlagSet(flag int) bool {
	return fa.flags&flag != 0
}

// SetFlag sets or clears the given flag bit.
func (fa *flagsAttributeImpl) SetFlag(flag int, set bool) {
	if set {
		fa.flags |= flag
	} else {
		fa.flags &= ^flag
	}
}

// Clear resets the flags to 0.
func (fa *flagsAttributeImpl) Clear() { fa.flags = 0 }

// CopyTo copies this attribute's state onto target.
func (fa *flagsAttributeImpl) CopyTo(target AttributeImpl) {
	if t, ok := target.(FlagsAttribute); ok {
		t.SetFlags(fa.flags)
	}
}

// CloneAttribute returns a deep clone of this impl.
func (fa *flagsAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &flagsAttributeImpl{flags: fa.flags}
}

// ReflectWith emits the single (FlagsAttribute, "flags", value) triple.
func (fa *flagsAttributeImpl) ReflectWith(reflector AttributeReflector) {
	reflector(FlagsAttributeType, "flags", fa.flags)
}

// Equals returns true if other is a [FlagsAttribute] with the same
// flag bitmask.
func (fa *flagsAttributeImpl) Equals(other any) bool {
	if fa == other {
		return true
	}
	o, ok := other.(FlagsAttribute)
	if !ok {
		return false
	}
	return fa.flags == o.GetFlags()
}

// HashCode returns the flag bitmask itself, matching
// {@code FlagsAttributeImpl#hashCode()}.
func (fa *flagsAttributeImpl) HashCode() int { return fa.flags }

// --- KeywordAttribute ------------------------------------------------

// KeywordAttributeType is the reflect.Type of the [KeywordAttribute]
// interface, used as the lookup key for an [AttributeSource].
var KeywordAttributeType = reflect.TypeOf((*KeywordAttribute)(nil)).Elem()

// KeywordAttribute marks a token as a keyword.
//
// Keyword tokens are typically not modified by subsequent filters
// (e.g., not lowercased, not stemmed).
//
// This is the Go port of
// org.apache.lucene.analysis.tokenattributes.KeywordAttribute.
type KeywordAttribute interface {
	AttributeImpl

	// IsKeywordToken reports whether the current token is flagged as a
	// keyword.
	IsKeywordToken() bool

	// SetKeyword toggles the keyword flag.
	SetKeyword(isKeyword bool)
}

// keywordAttributeImpl is the default [KeywordAttribute] implementation.
type keywordAttributeImpl struct {
	util.BaseAttributeImpl
	isKeyword bool
}

var (
	_ KeywordAttribute                = (*keywordAttributeImpl)(nil)
	_ util.AttributeImpl              = (*keywordAttributeImpl)(nil)
	_ AttributeImpl                   = (*keywordAttributeImpl)(nil)
	_ AttributeReflectable            = (*keywordAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*keywordAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (ka *keywordAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{KeywordAttributeType}
}

// NewKeywordAttribute creates a new KeywordAttribute with the keyword
// flag set to false.
func NewKeywordAttribute() KeywordAttribute {
	return &keywordAttributeImpl{}
}

// NewKeywordAttributeWithValue creates a new KeywordAttribute with the
// given keyword flag.
func NewKeywordAttributeWithValue(isKeyword bool) KeywordAttribute {
	return &keywordAttributeImpl{isKeyword: isKeyword}
}

// IsKeywordToken returns true if this token is a keyword.
func (ka *keywordAttributeImpl) IsKeywordToken() bool { return ka.isKeyword }

// SetKeyword toggles the keyword flag.
func (ka *keywordAttributeImpl) SetKeyword(isKeyword bool) { ka.isKeyword = isKeyword }

// Clear resets the keyword flag to false.
func (ka *keywordAttributeImpl) Clear() { ka.isKeyword = false }

// CopyTo copies this attribute's state onto target.
func (ka *keywordAttributeImpl) CopyTo(target AttributeImpl) {
	if t, ok := target.(KeywordAttribute); ok {
		t.SetKeyword(ka.isKeyword)
	}
}

// CloneAttribute returns a deep clone of this impl.
func (ka *keywordAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &keywordAttributeImpl{isKeyword: ka.isKeyword}
}

// ReflectWith emits the single (KeywordAttribute, "keyword", value)
// triple.
func (ka *keywordAttributeImpl) ReflectWith(reflector AttributeReflector) {
	reflector(KeywordAttributeType, "keyword", ka.isKeyword)
}

// Equals returns true if other is a [KeywordAttribute] whose flag
// matches.
func (ka *keywordAttributeImpl) Equals(other any) bool {
	if ka == other {
		return true
	}
	o, ok := other.(KeywordAttribute)
	if !ok {
		return false
	}
	return ka.isKeyword == o.IsKeywordToken()
}

// HashCode mirrors {@code KeywordAttributeImpl#hashCode()}: 31 when
// keyword=true, 37 otherwise.
func (ka *keywordAttributeImpl) HashCode() int {
	if ka.isKeyword {
		return 31
	}
	return 37
}

// --- PositionLengthAttribute -----------------------------------------

// PositionLengthAttributeType is the reflect.Type of the
// [PositionLengthAttribute] interface, used as the lookup key for an
// [AttributeSource].
var PositionLengthAttributeType = reflect.TypeOf((*PositionLengthAttribute)(nil)).Elem()

// PositionLengthAttribute provides the position length of a token.
//
// The position length indicates how many positions this token spans.
// For most tokens, this is 1. For tokens that represent multiple words
// (like those produced by a shingle filter), this can be greater than 1.
//
// This is the Go port of
// org.apache.lucene.analysis.tokenattributes.PositionLengthAttribute.
type PositionLengthAttribute interface {
	AttributeImpl

	// GetPositionLength returns the position length.
	GetPositionLength() int

	// SetPositionLength replaces the position length. The Lucene
	// reference throws IllegalArgumentException when length < 1; for
	// back-compat with existing Gocene consumers SetPositionLength is
	// permissive and SetPositionLengthValidated provides the Lucene
	// invariant.
	SetPositionLength(length int)

	// SetPositionLengthValidated panics when length < 1, mirroring
	// {@code PositionLengthAttributeImpl#setPositionLength(int)}.
	SetPositionLengthValidated(length int)
}

// positionLengthAttributeImpl is the default [PositionLengthAttribute]
// implementation.
type positionLengthAttributeImpl struct {
	util.BaseAttributeImpl
	positionLength int
}

var (
	_ PositionLengthAttribute         = (*positionLengthAttributeImpl)(nil)
	_ util.AttributeImpl              = (*positionLengthAttributeImpl)(nil)
	_ AttributeImpl                   = (*positionLengthAttributeImpl)(nil)
	_ AttributeReflectable            = (*positionLengthAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*positionLengthAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (pla *positionLengthAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{PositionLengthAttributeType}
}

// NewPositionLengthAttribute creates a new PositionLengthAttribute with
// length 1.
func NewPositionLengthAttribute() PositionLengthAttribute {
	return &positionLengthAttributeImpl{positionLength: 1}
}

// NewPositionLengthAttributeWithLength creates a new
// PositionLengthAttribute with the given length.
func NewPositionLengthAttributeWithLength(length int) PositionLengthAttribute {
	return &positionLengthAttributeImpl{positionLength: length}
}

// GetPositionLength returns the position length.
func (pla *positionLengthAttributeImpl) GetPositionLength() int { return pla.positionLength }

// SetPositionLength replaces the position length without validation.
func (pla *positionLengthAttributeImpl) SetPositionLength(length int) {
	pla.positionLength = length
}

// SetPositionLengthValidated panics when length < 1, mirroring Lucene.
func (pla *positionLengthAttributeImpl) SetPositionLengthValidated(length int) {
	if length < 1 {
		panic(fmt.Sprintf(
			"PositionLengthAttribute.SetPositionLengthValidated: position length must be 1 or greater; got %d",
			length))
	}
	pla.positionLength = length
}

// Clear resets the position length to 1.
func (pla *positionLengthAttributeImpl) Clear() { pla.positionLength = 1 }

// CopyTo copies this attribute's state onto target.
func (pla *positionLengthAttributeImpl) CopyTo(target AttributeImpl) {
	if t, ok := target.(PositionLengthAttribute); ok {
		t.SetPositionLength(pla.positionLength)
	}
}

// CloneAttribute returns a deep clone of this impl.
func (pla *positionLengthAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &positionLengthAttributeImpl{positionLength: pla.positionLength}
}

// ReflectWith emits the single (PositionLengthAttribute,
// "positionLength", value) triple.
func (pla *positionLengthAttributeImpl) ReflectWith(reflector AttributeReflector) {
	reflector(PositionLengthAttributeType, "positionLength", pla.positionLength)
}

// Equals returns true if other is a [PositionLengthAttribute] with the
// same value.
func (pla *positionLengthAttributeImpl) Equals(other any) bool {
	if pla == other {
		return true
	}
	o, ok := other.(PositionLengthAttribute)
	if !ok {
		return false
	}
	return pla.positionLength == o.GetPositionLength()
}

// HashCode returns positionLength itself, matching
// {@code PositionLengthAttributeImpl#hashCode()}.
func (pla *positionLengthAttributeImpl) HashCode() int { return pla.positionLength }

// --- TermFrequencyAttribute ------------------------------------------

// TermFrequencyAttributeType is the reflect.Type of the
// [TermFrequencyAttribute] interface, used as the lookup key for an
// [AttributeSource].
var TermFrequencyAttributeType = reflect.TypeOf((*TermFrequencyAttribute)(nil)).Elem()

// TermFrequencyAttribute provides the term frequency for a token.
//
// This is the Go port of
// org.apache.lucene.analysis.tokenattributes.TermFrequencyAttribute.
type TermFrequencyAttribute interface {
	AttributeImpl

	// GetTermFrequency returns the term frequency.
	GetTermFrequency() int

	// SetTermFrequency replaces the term frequency. The Lucene
	// reference throws IllegalArgumentException when freq < 1; for
	// back-compat with existing Gocene consumers SetTermFrequency is
	// permissive and SetTermFrequencyValidated provides the Lucene
	// invariant.
	SetTermFrequency(freq int)

	// SetTermFrequencyValidated panics when freq < 1, mirroring
	// {@code TermFrequencyAttributeImpl#setTermFrequency(int)}.
	SetTermFrequencyValidated(freq int)
}

// termFrequencyAttributeImpl is the default [TermFrequencyAttribute]
// implementation. Unlike the other small attributes it overrides End
// explicitly to mirror {@code TermFrequencyAttributeImpl#end()} which
// resets to 1 (identical to Clear). The override is kept to document
// the parity with Lucene; the embedding of [util.BaseAttributeImpl]
// would otherwise install the no-op default.
type termFrequencyAttributeImpl struct {
	util.BaseAttributeImpl
	termFrequency int
}

var (
	_ TermFrequencyAttribute          = (*termFrequencyAttributeImpl)(nil)
	_ util.AttributeImpl              = (*termFrequencyAttributeImpl)(nil)
	_ AttributeImpl                   = (*termFrequencyAttributeImpl)(nil)
	_ AttributeReflectable            = (*termFrequencyAttributeImpl)(nil)
	_ AttributeEnder                  = (*termFrequencyAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*termFrequencyAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (tfa *termFrequencyAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{TermFrequencyAttributeType}
}

// NewTermFrequencyAttribute creates a new TermFrequencyAttribute with
// frequency 1.
func NewTermFrequencyAttribute() TermFrequencyAttribute {
	return &termFrequencyAttributeImpl{termFrequency: 1}
}

// NewTermFrequencyAttributeWithFrequency creates a new
// TermFrequencyAttribute with the given frequency.
func NewTermFrequencyAttributeWithFrequency(freq int) TermFrequencyAttribute {
	return &termFrequencyAttributeImpl{termFrequency: freq}
}

// GetTermFrequency returns the term frequency.
func (tfa *termFrequencyAttributeImpl) GetTermFrequency() int { return tfa.termFrequency }

// SetTermFrequency replaces the term frequency without validation.
func (tfa *termFrequencyAttributeImpl) SetTermFrequency(freq int) {
	tfa.termFrequency = freq
}

// SetTermFrequencyValidated panics when freq < 1, mirroring Lucene.
func (tfa *termFrequencyAttributeImpl) SetTermFrequencyValidated(freq int) {
	if freq < 1 {
		panic(fmt.Sprintf(
			"TermFrequencyAttribute.SetTermFrequencyValidated: term frequency must be 1 or greater; got %d",
			freq))
	}
	tfa.termFrequency = freq
}

// Clear resets the term frequency to 1.
func (tfa *termFrequencyAttributeImpl) Clear() { tfa.termFrequency = 1 }

// End mirrors {@code TermFrequencyAttributeImpl#end()}: reset to 1,
// identical to Clear. Lucene declares end() explicitly to document the
// intent, so the Go port overrides the embedded
// [util.BaseAttributeImpl.End] (which would otherwise be a no-op).
func (tfa *termFrequencyAttributeImpl) End() { tfa.termFrequency = 1 }

// CopyTo copies this attribute's state onto target.
func (tfa *termFrequencyAttributeImpl) CopyTo(target AttributeImpl) {
	if t, ok := target.(TermFrequencyAttribute); ok {
		t.SetTermFrequency(tfa.termFrequency)
	}
}

// CloneAttribute returns a deep clone of this impl.
func (tfa *termFrequencyAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &termFrequencyAttributeImpl{termFrequency: tfa.termFrequency}
}

// ReflectWith emits the single (TermFrequencyAttribute,
// "termFrequency", value) triple.
func (tfa *termFrequencyAttributeImpl) ReflectWith(reflector AttributeReflector) {
	reflector(TermFrequencyAttributeType, "termFrequency", tfa.termFrequency)
}

// Equals returns true if other is a [TermFrequencyAttribute] with the
// same value.
func (tfa *termFrequencyAttributeImpl) Equals(other any) bool {
	if tfa == other {
		return true
	}
	o, ok := other.(TermFrequencyAttribute)
	if !ok {
		return false
	}
	return tfa.termFrequency == o.GetTermFrequency()
}

// HashCode returns termFrequency itself, matching
// {@code Integer.hashCode(termFrequency)} for non-negative values.
func (tfa *termFrequencyAttributeImpl) HashCode() int { return tfa.termFrequency }
