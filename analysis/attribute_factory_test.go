// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestNewDefaultAttributeFactory(t *testing.T) {
	factory := NewDefaultAttributeFactory()
	if factory == nil {
		t.Fatal("NewDefaultAttributeFactory() returned nil")
	}
	if factory.creators == nil {
		t.Fatal("factory.creators is nil")
	}
}

func TestDefaultAttributeFactory_CreateAttributeInstance(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	tests := []struct {
		name     string
		attrType string
		wantNil  bool
	}{
		{
			name:     "CharTermAttribute",
			attrType: "CharTermAttribute",
			wantNil:  false,
		},
		{
			name:     "OffsetAttribute",
			attrType: "OffsetAttribute",
			wantNil:  false,
		},
		{
			name:     "PositionIncrementAttribute",
			attrType: "PositionIncrementAttribute",
			wantNil:  false,
		},
		{
			name:     "TypeAttribute",
			attrType: "TypeAttribute",
			wantNil:  false,
		},
		{
			name:     "PayloadAttribute",
			attrType: "PayloadAttribute",
			wantNil:  false,
		},
		{
			name:     "FlagsAttribute",
			attrType: "FlagsAttribute",
			wantNil:  false,
		},
		{
			name:     "KeywordAttribute",
			attrType: "KeywordAttribute",
			wantNil:  false,
		},
		{
			name:     "PositionLengthAttribute",
			attrType: "PositionLengthAttribute",
			wantNil:  false,
		},
		{
			name:     "TermFrequencyAttribute",
			attrType: "TermFrequencyAttribute",
			wantNil:  false,
		},
		{
			name:     "UnknownAttribute",
			attrType: "UnknownAttribute",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := factory.CreateAttributeInstance(tt.attrType)
			if tt.wantNil {
				if got != nil {
					t.Errorf("CreateAttributeInstance() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("CreateAttributeInstance() returned nil for %s", tt.attrType)
			}
		})
	}
}

func TestDefaultAttributeFactory_RegisterCreator(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	customCreated := false
	customCreator := func() AttributeImpl {
		customCreated = true
		return NewCharTermAttribute()
	}

	factory.RegisterCreator("CustomAttribute", customCreator)

	// Create attribute - should use custom creator
	_ = factory.CreateAttributeInstance("CustomAttribute")

	if !customCreated {
		t.Error("Custom creator was not called")
	}
}

func TestStaticAttributeFactory(t *testing.T) {
	factory := NewStaticAttributeFactory()

	// Create a static instance
	staticAttr := NewCharTermAttribute()
	staticAttr.SetEmpty()
	staticAttr.Append([]byte("test"))

	factory.RegisterAttribute("CharTermAttribute", staticAttr)

	// Retrieve the static instance
	got := factory.CreateAttributeInstance("CharTermAttribute")
	if got == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	// Should be the same instance
	if got != staticAttr {
		t.Error("CreateAttributeInstance() did not return the registered static instance")
	}

	// Test unregistered type
	gotUnregistered := factory.CreateAttributeInstance("UnknownAttribute")
	if gotUnregistered != nil {
		t.Error("CreateAttributeInstance() for unregistered type should return nil")
	}
}

func TestDefaultAttributeFactory_CreateAttributeInstance_CreatesNewInstances(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	// Create first attribute and modify it
	attr1 := factory.CreateAttributeInstance("CharTermAttribute")
	if attr1 == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	cta1 := attr1.(*charTermAttribute)
	cta1.Append([]byte("modified"))

	// Create second attribute - should be fresh
	attr2 := factory.CreateAttributeInstance("CharTermAttribute")
	if attr2 == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	cta2 := attr2.(*charTermAttribute)

	// They should be different instances
	if cta1 == cta2 {
		t.Error("CreateAttributeInstance() returned same instance twice")
	}

	// Second instance should be empty (fresh)
	if cta2.Length() != 0 {
		t.Errorf("Second instance has Length() = %d, want 0", cta2.Length())
	}

	// First instance should still have the value
	if cta1.String() != "modified" {
		t.Errorf("First instance has value %s, want 'modified'", cta1.String())
	}
}

func TestDefaultAttributeFactory_TypeAttribute(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	attr := factory.CreateAttributeInstance("TypeAttribute")
	if attr == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	typeAttr, ok := attr.(*TypeAttribute)
	if !ok {
		t.Fatal("CreateAttributeInstance() did not return TypeAttribute")
	}

	if typeAttr.GetType() != "word" {
		t.Errorf("TypeAttribute.Type = %v, want 'word'", typeAttr.GetType())
	}
}

func TestDefaultAttributeFactory_PayloadAttribute(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	attr := factory.CreateAttributeInstance("PayloadAttribute")
	if attr == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	payloadAttr, ok := attr.(*PayloadAttribute)
	if !ok {
		t.Fatal("CreateAttributeInstance() did not return PayloadAttribute")
	}

	if payloadAttr.HasPayload() {
		t.Error("New PayloadAttribute should not have payload")
	}
}

func TestDefaultAttributeFactory_KeywordAttribute(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	attr := factory.CreateAttributeInstance("KeywordAttribute")
	if attr == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	keywordAttr, ok := attr.(*KeywordAttribute)
	if !ok {
		t.Fatal("CreateAttributeInstance() did not return KeywordAttribute")
	}

	if keywordAttr.IsKeywordToken() {
		t.Error("New KeywordAttribute should have IsKeyword = false")
	}
}

func TestDefaultAttributeFactory_PositionLengthAttribute(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	attr := factory.CreateAttributeInstance("PositionLengthAttribute")
	if attr == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	posLenAttr, ok := attr.(*PositionLengthAttribute)
	if !ok {
		t.Fatal("CreateAttributeInstance() did not return PositionLengthAttribute")
	}

	if posLenAttr.GetPositionLength() != 1 {
		t.Errorf("PositionLengthAttribute.PositionLength = %d, want 1", posLenAttr.GetPositionLength())
	}
}

func TestDefaultAttributeFactory_TermFrequencyAttribute(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	attr := factory.CreateAttributeInstance("TermFrequencyAttribute")
	if attr == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	tfAttr, ok := attr.(*TermFrequencyAttribute)
	if !ok {
		t.Fatal("CreateAttributeInstance() did not return TermFrequencyAttribute")
	}

	if tfAttr.GetTermFrequency() != 1 {
		t.Errorf("TermFrequencyAttribute.TermFrequency = %d, want 1", tfAttr.GetTermFrequency())
	}
}

func TestDefaultAttributeFactory_FlagsAttribute(t *testing.T) {
	factory := NewDefaultAttributeFactory()

	attr := factory.CreateAttributeInstance("FlagsAttribute")
	if attr == nil {
		t.Fatal("CreateAttributeInstance() returned nil")
	}

	flagsAttr, ok := attr.(*FlagsAttribute)
	if !ok {
		t.Fatal("CreateAttributeInstance() did not return FlagsAttribute")
	}

	if flagsAttr.GetFlags() != 0 {
		t.Errorf("FlagsAttribute.Flags = %d, want 0", flagsAttr.GetFlags())
	}
}
