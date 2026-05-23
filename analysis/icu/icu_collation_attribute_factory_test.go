// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu_test

import (
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/icu"
	"github.com/FlavioCFOliveira/Gocene/analysis/icu/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// lexicographicCollator sorts by the UTF-8 bytes of the string — a simple
// test collator that is deterministic.
type lexicographicCollator struct{}

func (l *lexicographicCollator) GetRawCollationKey(s string) []byte {
	return []byte(s)
}

// TestICUCollationAttributeFactory_CreateAttributeInstance verifies that the
// factory returns an ICUCollatedTermAttributeImpl for the CharTermAttribute
// interface type (mirrors the Java TestICUCollationKeyAnalyzer structural
// setup).
func TestICUCollationAttributeFactory_CreateAttributeInstance(t *testing.T) {
	collator := &lexicographicCollator{}
	factory := icu.NewICUCollationAttributeFactory(collator)

	impl := factory.CreateAttributeInstance(analysis.CharTermAttributeType)
	if impl == nil {
		t.Fatal("factory returned nil for CharTermAttributeType")
	}
	if _, ok := impl.(*tokenattributes.ICUCollatedTermAttributeImpl); !ok {
		t.Errorf("expected *ICUCollatedTermAttributeImpl, got %T", impl)
	}
}

// TestICUCollationAttributeFactory_Delegation verifies that the
// ICUCollatedTermAttributeImpl is not produced for an unrelated registered
// attribute type.
func TestICUCollationAttributeFactory_Delegation(t *testing.T) {
	collator := &lexicographicCollator{}
	factory := icu.NewICUCollationAttributeFactory(collator)

	// CharTermAttribute must resolve to ICUCollatedTermAttributeImpl.
	ctImpl := factory.CreateAttributeInstance(analysis.CharTermAttributeType)
	if _, ok := ctImpl.(*tokenattributes.ICUCollatedTermAttributeImpl); !ok {
		t.Errorf("CharTermAttribute: expected *ICUCollatedTermAttributeImpl, got %T", ctImpl)
	}

	// TermToBytesRefAttribute is also satisfied by ICUCollatedTermAttributeImpl
	// because it embeds CharTermAttributeImpl.
	tbrType := reflect.TypeOf((*analysis.TermToBytesRefAttribute)(nil)).Elem()
	tbrImpl := factory.CreateAttributeInstance(tbrType)
	if _, ok := tbrImpl.(*tokenattributes.ICUCollatedTermAttributeImpl); !ok {
		t.Errorf("TermToBytesRefAttribute: expected *ICUCollatedTermAttributeImpl, got %T", tbrImpl)
	}
}

// TestICUCollationAttributeFactory_WithDelegate verifies that a custom
// delegate is honoured for non-collation attributes.
func TestICUCollationAttributeFactory_WithDelegate(t *testing.T) {
	collator := &lexicographicCollator{}
	factory := icu.NewICUCollationAttributeFactoryWithDelegate(
		util.DefaultAttributeFactoryInstance,
		collator,
	)

	impl := factory.CreateAttributeInstance(analysis.CharTermAttributeType)
	if impl == nil {
		t.Fatal("factory returned nil")
	}
	if _, ok := impl.(*tokenattributes.ICUCollatedTermAttributeImpl); !ok {
		t.Errorf("expected *ICUCollatedTermAttributeImpl, got %T", impl)
	}
}

// TestICUCollationAttributeFactory_GetBytesRef verifies that the produced
// ICUCollatedTermAttributeImpl encodes the term via the collator when
// GetBytesRef is called.
func TestICUCollationAttributeFactory_GetBytesRef(t *testing.T) {
	collator := &lexicographicCollator{}
	factory := icu.NewICUCollationAttributeFactory(collator)

	rawImpl := factory.CreateAttributeInstance(analysis.CharTermAttributeType)
	attr := rawImpl.(*tokenattributes.ICUCollatedTermAttributeImpl)
	attr.SetValue("hello")

	ref := attr.GetBytesRef()
	want := []byte("hello")
	if ref == nil {
		t.Fatal("GetBytesRef returned nil")
	}
	if string(ref.Bytes[:ref.Length]) != string(want) {
		t.Errorf("got %q, want %q", ref.Bytes[:ref.Length], want)
	}
}
