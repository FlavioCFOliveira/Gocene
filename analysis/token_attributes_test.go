// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"testing"
)

// TypeAttribute Tests

func TestNewTypeAttribute(t *testing.T) {
	ta := NewTypeAttribute()
	if ta.GetType() != "word" {
		t.Errorf("Expected default type 'word', got '%s'", ta.GetType())
	}
}

func TestTypeAttributeSetGet(t *testing.T) {
	ta := NewTypeAttribute()
	ta.SetType("<ALPHANUM>")
	if ta.GetType() != "<ALPHANUM>" {
		t.Errorf("Expected type '<ALPHANUM>', got '%s'", ta.GetType())
	}
}

func TestTypeAttributeClone(t *testing.T) {
	ta := NewTypeAttribute()
	ta.SetType("custom")
	clone := ta.CloneAttribute().(TypeAttribute)
	if clone.GetType() != "custom" {
		t.Error("Clone should have same type")
	}
	// Modify original should not affect clone
	ta.SetType("modified")
	if clone.GetType() != "custom" {
		t.Error("Clone should be independent")
	}
}

func TestTypeAttributeClear(t *testing.T) {
	ta := NewTypeAttribute()
	ta.SetType("custom")
	ta.Clear()
	if ta.GetType() != "word" {
		t.Error("Clear should reset to default")
	}
}

// PayloadAttribute Tests

func TestNewPayloadAttribute(t *testing.T) {
	pa := NewPayloadAttribute()
	if pa.GetPayload() != nil {
		t.Error("Expected nil payload")
	}
}

func TestNewPayloadAttributeWithPayload(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	pa := NewPayloadAttributeWithPayload(data)
	if !bytes.Equal(pa.GetPayload(), data) {
		t.Error("Payload should match input")
	}
	// Modifying original should not affect attribute
	data[0] = 0xFF
	if pa.GetPayload()[0] != 0x01 {
		t.Error("Payload should be a copy")
	}
}

func TestPayloadAttributeSetGet(t *testing.T) {
	pa := NewPayloadAttribute()
	data := []byte{0x01, 0x02, 0x03}
	pa.SetPayload(data)
	if !bytes.Equal(pa.GetPayload(), data) {
		t.Error("GetPayload should return set payload")
	}
}

func TestPayloadAttributeSetNil(t *testing.T) {
	pa := NewPayloadAttributeWithPayload([]byte{0x01})
	pa.SetPayload(nil)
	if pa.GetPayload() != nil {
		t.Error("SetPayload(nil) should set payload to nil")
	}
}

func TestPayloadAttributeHasPayload(t *testing.T) {
	pa := NewPayloadAttribute()
	if pa.HasPayload() {
		t.Error("Empty payload should return false")
	}
	pa.SetPayload([]byte{0x01})
	if !pa.HasPayload() {
		t.Error("Non-empty payload should return true")
	}
	pa.SetPayload([]byte{})
	if pa.HasPayload() {
		t.Error("Empty byte slice should return false")
	}
}

func TestPayloadAttributeClone(t *testing.T) {
	pa := NewPayloadAttributeWithPayload([]byte{0x01, 0x02})
	clone := pa.CloneAttribute().(PayloadAttribute)
	if !bytes.Equal(clone.GetPayload(), pa.GetPayload()) {
		t.Error("Clone should have same payload")
	}
	// Modify original
	pa.GetPayload()[0] = 0xFF
	if clone.GetPayload()[0] != 0x01 {
		t.Error("Clone should be independent")
	}
}

func TestPayloadAttributeClear(t *testing.T) {
	pa := NewPayloadAttributeWithPayload([]byte{0x01})
	pa.Clear()
	if pa.GetPayload() != nil {
		t.Error("Clear should set payload to nil")
	}
}

// FlagsAttribute Tests

func TestNewFlagsAttribute(t *testing.T) {
	fa := NewFlagsAttribute()
	if fa.GetFlags() != 0 {
		t.Errorf("Expected flags 0, got %d", fa.GetFlags())
	}
}

func TestNewFlagsAttributeWithFlags(t *testing.T) {
	fa := NewFlagsAttributeWithFlags(0x0F)
	if fa.GetFlags() != 0x0F {
		t.Errorf("Expected flags 0x0F, got %d", fa.GetFlags())
	}
}

func TestFlagsAttributeSetGet(t *testing.T) {
	fa := NewFlagsAttribute()
	fa.SetFlags(0xFF)
	if fa.GetFlags() != 0xFF {
		t.Errorf("Expected flags 0xFF, got %d", fa.GetFlags())
	}
}

func TestFlagsAttributeIsFlagSet(t *testing.T) {
	fa := NewFlagsAttributeWithFlags(0x05) // Binary: 0101
	if !fa.IsFlagSet(0x01) {
		t.Error("Flag 0x01 should be set")
	}
	if !fa.IsFlagSet(0x04) {
		t.Error("Flag 0x04 should be set")
	}
	if fa.IsFlagSet(0x02) {
		t.Error("Flag 0x02 should not be set")
	}
}

func TestFlagsAttributeSetFlag(t *testing.T) {
	fa := NewFlagsAttribute()
	fa.SetFlag(0x01, true)
	if !fa.IsFlagSet(0x01) {
		t.Error("Flag should be set")
	}
	fa.SetFlag(0x01, false)
	if fa.IsFlagSet(0x01) {
		t.Error("Flag should be cleared")
	}
}

func TestFlagsAttributeClone(t *testing.T) {
	fa := NewFlagsAttributeWithFlags(0xFF)
	clone := fa.CloneAttribute().(FlagsAttribute)
	if clone.GetFlags() != 0xFF {
		t.Error("Clone should have same flags")
	}
}

func TestFlagsAttributeClear(t *testing.T) {
	fa := NewFlagsAttributeWithFlags(0xFF)
	fa.Clear()
	if fa.GetFlags() != 0 {
		t.Error("Clear should reset flags to 0")
	}
}

// KeywordAttribute Tests

func TestNewKeywordAttribute(t *testing.T) {
	ka := NewKeywordAttribute()
	if ka.IsKeywordToken() {
		t.Error("Expected IsKeywordToken to be false by default")
	}
}

func TestNewKeywordAttributeWithValue(t *testing.T) {
	ka := NewKeywordAttributeWithValue(true)
	if !ka.IsKeywordToken() {
		t.Error("Expected IsKeywordToken to be true")
	}
}

func TestKeywordAttributeSetGet(t *testing.T) {
	ka := NewKeywordAttribute()
	ka.SetKeyword(true)
	if !ka.IsKeywordToken() {
		t.Error("IsKeywordToken should return true")
	}
	ka.SetKeyword(false)
	if ka.IsKeywordToken() {
		t.Error("IsKeywordToken should return false")
	}
}

func TestKeywordAttributeClone(t *testing.T) {
	ka := NewKeywordAttributeWithValue(true)
	clone := ka.CloneAttribute().(KeywordAttribute)
	if !clone.IsKeywordToken() {
		t.Error("Clone should have same value")
	}
}

func TestKeywordAttributeClear(t *testing.T) {
	ka := NewKeywordAttributeWithValue(true)
	ka.Clear()
	if ka.IsKeywordToken() {
		t.Error("Clear should reset IsKeyword to false")
	}
}

// PositionLengthAttribute Tests

func TestNewPositionLengthAttribute(t *testing.T) {
	pla := NewPositionLengthAttribute()
	if pla.GetPositionLength() != 1 {
		t.Errorf("Expected default length 1, got %d", pla.GetPositionLength())
	}
}

func TestNewPositionLengthAttributeWithLength(t *testing.T) {
	pla := NewPositionLengthAttributeWithLength(3)
	if pla.GetPositionLength() != 3 {
		t.Errorf("Expected length 3, got %d", pla.GetPositionLength())
	}
}

func TestPositionLengthAttributeSetGet(t *testing.T) {
	pla := NewPositionLengthAttribute()
	pla.SetPositionLength(5)
	if pla.GetPositionLength() != 5 {
		t.Errorf("Expected length 5, got %d", pla.GetPositionLength())
	}
}

func TestPositionLengthAttributeClone(t *testing.T) {
	pla := NewPositionLengthAttributeWithLength(3)
	clone := pla.CloneAttribute().(PositionLengthAttribute)
	if clone.GetPositionLength() != 3 {
		t.Error("Clone should have same length")
	}
}

func TestPositionLengthAttributeClear(t *testing.T) {
	pla := NewPositionLengthAttributeWithLength(5)
	pla.Clear()
	if pla.GetPositionLength() != 1 {
		t.Error("Clear should reset length to 1")
	}
}

// TermFrequencyAttribute Tests

func TestNewTermFrequencyAttribute(t *testing.T) {
	tfa := NewTermFrequencyAttribute()
	if tfa.GetTermFrequency() != 1 {
		t.Errorf("Expected default frequency 1, got %d", tfa.GetTermFrequency())
	}
}

func TestNewTermFrequencyAttributeWithFrequency(t *testing.T) {
	tfa := NewTermFrequencyAttributeWithFrequency(5)
	if tfa.GetTermFrequency() != 5 {
		t.Errorf("Expected frequency 5, got %d", tfa.GetTermFrequency())
	}
}

func TestTermFrequencyAttributeSetGet(t *testing.T) {
	tfa := NewTermFrequencyAttribute()
	tfa.SetTermFrequency(10)
	if tfa.GetTermFrequency() != 10 {
		t.Errorf("Expected frequency 10, got %d", tfa.GetTermFrequency())
	}
}

func TestTermFrequencyAttributeClone(t *testing.T) {
	tfa := NewTermFrequencyAttributeWithFrequency(5)
	clone := tfa.CloneAttribute().(TermFrequencyAttribute)
	if clone.GetTermFrequency() != 5 {
		t.Error("Clone should have same frequency")
	}
}

func TestTermFrequencyAttributeClear(t *testing.T) {
	tfa := NewTermFrequencyAttributeWithFrequency(10)
	tfa.Clear()
	if tfa.GetTermFrequency() != 1 {
		t.Error("Clear should reset frequency to 1")
	}
}
