// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TypeAttribute provides a way to store the token type.
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.TypeAttribute.
type TypeAttribute struct {
	// Type is the token type (e.g., "word", "acronym", "<ALPHANUM>", etc.)
	Type string
}

// NewTypeAttribute creates a new TypeAttribute with the default type "word".
func NewTypeAttribute() *TypeAttribute {
	return &TypeAttribute{Type: "word"}
}

// SetType sets the token type.
func (ta *TypeAttribute) SetType(tokenType string) {
	ta.Type = tokenType
}

// GetType returns the token type.
func (ta *TypeAttribute) GetType() string {
	return ta.Type
}

// Clone returns a copy of this attribute.
func (ta *TypeAttribute) Clone() *TypeAttribute {
	return &TypeAttribute{Type: ta.Type}
}

// Clear resets this attribute to its default value.
func (ta *TypeAttribute) Clear() {
	ta.Type = "word"
}

// PayloadAttribute provides a way to store a payload for a token.
// Payloads are arbitrary byte arrays that can be associated with tokens.
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.PayloadAttribute.
type PayloadAttribute struct {
	// Payload is the byte slice payload for this token.
	// A nil payload means no payload.
	Payload []byte
}

// NewPayloadAttribute creates a new empty PayloadAttribute.
func NewPayloadAttribute() *PayloadAttribute {
	return &PayloadAttribute{}
}

// NewPayloadAttributeWithPayload creates a new PayloadAttribute with the given payload.
func NewPayloadAttributeWithPayload(payload []byte) *PayloadAttribute {
	copied := make([]byte, len(payload))
	copy(copied, payload)
	return &PayloadAttribute{Payload: copied}
}

// SetPayload sets the payload.
func (pa *PayloadAttribute) SetPayload(payload []byte) {
	if payload == nil {
		pa.Payload = nil
		return
	}
	pa.Payload = make([]byte, len(payload))
	copy(pa.Payload, payload)
}

// GetPayload returns the payload.
func (pa *PayloadAttribute) GetPayload() []byte {
	return pa.Payload
}

// Clone returns a copy of this attribute.
func (pa *PayloadAttribute) Clone() *PayloadAttribute {
	if pa.Payload == nil {
		return &PayloadAttribute{}
	}
	copied := make([]byte, len(pa.Payload))
	copy(copied, pa.Payload)
	return &PayloadAttribute{Payload: copied}
}

// Clear resets this attribute.
func (pa *PayloadAttribute) Clear() {
	pa.Payload = nil
}

// HasPayload returns true if this attribute has a non-nil payload.
func (pa *PayloadAttribute) HasPayload() bool {
	return pa.Payload != nil && len(pa.Payload) > 0
}

// FlagsAttribute provides a way to store custom flags for a token.
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.FlagsAttribute.
type FlagsAttribute struct {
	// Flags contains custom flags for this token.
	Flags int
}

// NewFlagsAttribute creates a new FlagsAttribute with flags set to 0.
func NewFlagsAttribute() *FlagsAttribute {
	return &FlagsAttribute{Flags: 0}
}

// NewFlagsAttributeWithFlags creates a new FlagsAttribute with the given flags.
func NewFlagsAttributeWithFlags(flags int) *FlagsAttribute {
	return &FlagsAttribute{Flags: flags}
}

// SetFlags sets the flags.
func (fa *FlagsAttribute) SetFlags(flags int) {
	fa.Flags = flags
}

// GetFlags returns the flags.
func (fa *FlagsAttribute) GetFlags() int {
	return fa.Flags
}

// Clone returns a copy of this attribute.
func (fa *FlagsAttribute) Clone() *FlagsAttribute {
	return &FlagsAttribute{Flags: fa.Flags}
}

// Clear resets this attribute.
func (fa *FlagsAttribute) Clear() {
	fa.Flags = 0
}

// IsFlagSet returns true if the given flag bit is set.
func (fa *FlagsAttribute) IsFlagSet(flag int) bool {
	return fa.Flags&flag != 0
}

// SetFlag sets or clears the given flag bit.
func (fa *FlagsAttribute) SetFlag(flag int, set bool) {
	if set {
		fa.Flags |= flag
	} else {
		fa.Flags &= ^flag
	}
}

// KeywordAttribute marks a token as a keyword.
// Keyword tokens are typically not modified by subsequent filters
// (e.g., not lowercased, not stemmed).
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.KeywordAttribute.
type KeywordAttribute struct {
	// IsKeyword is true if this token is a keyword.
	IsKeyword bool
}

// NewKeywordAttribute creates a new KeywordAttribute with IsKeyword set to false.
func NewKeywordAttribute() *KeywordAttribute {
	return &KeywordAttribute{IsKeyword: false}
}

// NewKeywordAttributeWithValue creates a new KeywordAttribute with the given value.
func NewKeywordAttributeWithValue(isKeyword bool) *KeywordAttribute {
	return &KeywordAttribute{IsKeyword: isKeyword}
}

// SetKeyword sets whether this token is a keyword.
func (ka *KeywordAttribute) SetKeyword(isKeyword bool) {
	ka.IsKeyword = isKeyword
}

// IsKeyword returns true if this token is a keyword.
func (ka *KeywordAttribute) IsKeywordToken() bool {
	return ka.IsKeyword
}

// Clone returns a copy of this attribute.
func (ka *KeywordAttribute) Clone() *KeywordAttribute {
	return &KeywordAttribute{IsKeyword: ka.IsKeyword}
}

// Clear resets this attribute.
func (ka *KeywordAttribute) Clear() {
	ka.IsKeyword = false
}

// PositionLengthAttribute provides the position length of a token.
// The position length indicates how many positions this token spans.
// For most tokens, this is 1. For tokens that represent multiple words
// (like those produced by a shingle filter), this can be greater than 1.
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.PositionLengthAttribute.
type PositionLengthAttribute struct {
	// PositionLength is the number of positions this token spans.
	// Default is 1.
	PositionLength int
}

// NewPositionLengthAttribute creates a new PositionLengthAttribute with length 1.
func NewPositionLengthAttribute() *PositionLengthAttribute {
	return &PositionLengthAttribute{PositionLength: 1}
}

// NewPositionLengthAttributeWithLength creates a new PositionLengthAttribute with the given length.
func NewPositionLengthAttributeWithLength(length int) *PositionLengthAttribute {
	return &PositionLengthAttribute{PositionLength: length}
}

// SetPositionLength sets the position length.
func (pla *PositionLengthAttribute) SetPositionLength(length int) {
	pla.PositionLength = length
}

// GetPositionLength returns the position length.
func (pla *PositionLengthAttribute) GetPositionLength() int {
	return pla.PositionLength
}

// Clone returns a copy of this attribute.
func (pla *PositionLengthAttribute) Clone() *PositionLengthAttribute {
	return &PositionLengthAttribute{PositionLength: pla.PositionLength}
}

// Clear resets this attribute.
func (pla *PositionLengthAttribute) Clear() {
	pla.PositionLength = 1
}

// TermFrequencyAttribute provides the term frequency for a token.
// This can be used to encode term frequencies in the token stream.
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.TermFrequencyAttribute.
type TermFrequencyAttribute struct {
	// TermFrequency is the frequency of this term.
	// Default is 1.
	TermFrequency int
}

// NewTermFrequencyAttribute creates a new TermFrequencyAttribute with frequency 1.
func NewTermFrequencyAttribute() *TermFrequencyAttribute {
	return &TermFrequencyAttribute{TermFrequency: 1}
}

// NewTermFrequencyAttributeWithFrequency creates a new TermFrequencyAttribute with the given frequency.
func NewTermFrequencyAttributeWithFrequency(freq int) *TermFrequencyAttribute {
	return &TermFrequencyAttribute{TermFrequency: freq}
}

// SetTermFrequency sets the term frequency.
func (tfa *TermFrequencyAttribute) SetTermFrequency(freq int) {
	tfa.TermFrequency = freq
}

// GetTermFrequency returns the term frequency.
func (tfa *TermFrequencyAttribute) GetTermFrequency() int {
	return tfa.TermFrequency
}

// Clone returns a copy of this attribute.
func (tfa *TermFrequencyAttribute) Clone() *TermFrequencyAttribute {
	return &TermFrequencyAttribute{TermFrequency: tfa.TermFrequency}
}

// Clear resets this attribute.
func (tfa *TermFrequencyAttribute) Clear() {
	tfa.TermFrequency = 1
}
