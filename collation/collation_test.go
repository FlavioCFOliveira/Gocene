// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package collation

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/collation/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// upperKeyCollator is a deterministic, total test Collator. It maps a string to
// the bytes of its ASCII upper-cased form. This is a faithful implementation of
// the one-method Collator contract (CollationKey(string) []byte) and lets the
// tests assert the produced key bytes exactly. Real collators (ICU, JDK) are
// out of scope here; the package's job is to wire whatever Collator it is given
// into the attribute / doc-values pipeline, which is what these tests verify.
type upperKeyCollator struct{}

func (upperKeyCollator) CollationKey(s string) []byte {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out[i] = c
	}
	return out
}

// Compile-time assertion that the fake satisfies the interface the package
// consumes.
var _ tokenattributes.Collator = upperKeyCollator{}

// TestCollationDocValuesFieldSetStringValue verifies that SetStringValue runs
// the input through the configured Collator and stores the resulting key bytes
// as the field's binary value. This is the load-bearing behaviour of the type:
// the indexed bytes must be the collation key, not the raw UTF-8.
func TestCollationDocValuesFieldSetStringValue(t *testing.T) {
	t.Parallel()

	f, err := NewCollationDocValuesField("sortField", upperKeyCollator{})
	if err != nil {
		t.Fatalf("NewCollationDocValuesField: %v", err)
	}
	if f.Name() != "sortField" {
		t.Errorf("Name() = %q, want sortField", f.Name())
	}

	f.SetStringValue("Hello")
	want := []byte("HELLO")
	if got := f.BinaryValue(); !bytes.Equal(got, want) {
		t.Errorf("BinaryValue() after SetStringValue(Hello) = % x, want % x", got, want)
	}

	// Reusing the same instance for a new document replaces the value, matching
	// the documented "reuse one instance, call SetStringValue per document"
	// contract.
	f.SetStringValue("world")
	want = []byte("WORLD")
	if got := f.BinaryValue(); !bytes.Equal(got, want) {
		t.Errorf("BinaryValue() after reuse SetStringValue(world) = % x, want % x", got, want)
	}
}

// TestCollationDocValuesFieldKeyOrdering checks the property that motivates the
// type: ordering the produced keys reproduces the collator's ordering. With the
// case-folding collator, "apple" and "Apple" collate equal, while "Banana"
// sorts after both.
func TestCollationDocValuesFieldKeyOrdering(t *testing.T) {
	t.Parallel()
	c := upperKeyCollator{}

	keyApple := c.CollationKey("apple")
	keyAppleCap := c.CollationKey("Apple")
	keyBanana := c.CollationKey("Banana")

	if !bytes.Equal(keyApple, keyAppleCap) {
		t.Errorf("case-folding collator keys differ: %q vs %q", keyApple, keyAppleCap)
	}
	if bytes.Compare(keyApple, keyBanana) >= 0 {
		t.Errorf("key(apple) >= key(Banana): % x vs % x", keyApple, keyBanana)
	}
}

// TestCollatedTermAttributeGetBytesRef verifies the attribute returns the
// collation key of its current term via GetBytesRef, overriding the raw-UTF-8
// behaviour of the embedded CharTermAttributeImpl.
func TestCollatedTermAttributeGetBytesRef(t *testing.T) {
	t.Parallel()

	attr := tokenattributes.NewCollatedTermAttributeImpl(upperKeyCollator{})
	// Set the current term via the embedded CharTermAttribute surface.
	attr.SetEmpty()
	attr.AppendString("café")

	ref := attr.GetBytesRef()
	if ref == nil {
		t.Fatal("GetBytesRef() = nil")
	}
	// "café" upper-cased by the ASCII fold leaves the non-ASCII bytes intact.
	want := upperKeyCollator{}.CollationKey("café")
	if !bytes.Equal(ref.Bytes, want) {
		t.Errorf("GetBytesRef().Bytes = % x, want % x", ref.Bytes, want)
	}
	if ref.Length != len(want) {
		t.Errorf("GetBytesRef().Length = %d, want %d", ref.Length, len(want))
	}
}

// TestCollationAttributeFactoryCreateInstance verifies the factory produces a
// CollatedTermAttributeImpl bound to its Collator.
func TestCollationAttributeFactoryCreateInstance(t *testing.T) {
	t.Parallel()

	factory := NewCollationAttributeFactory(upperKeyCollator{})
	inst := factory.CreateInstance()
	if inst == nil {
		t.Fatal("CreateInstance() = nil")
	}

	inst.SetEmpty()
	inst.AppendString("xyz")
	want := []byte("XYZ")
	if got := inst.GetBytesRef().Bytes; !bytes.Equal(got, want) {
		t.Errorf("factory instance GetBytesRef().Bytes = % x, want % x", got, want)
	}
}

// TestCollationKeyAnalyzerConstruction verifies the analyzer is constructed as
// a valid Analyzer and can produce a token stream without error.
func TestCollationKeyAnalyzerConstruction(t *testing.T) {
	t.Parallel()

	a := NewCollationKeyAnalyzer(upperKeyCollator{})
	if a == nil {
		t.Fatal("NewCollationKeyAnalyzer() = nil")
	}
	var _ analysis.Analyzer = a // also enforced by a package-level assertion

	ts, err := a.TokenStream("field", bytes.NewReader([]byte("input")))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	if ts == nil {
		t.Fatal("TokenStream() returned a nil stream")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestCollationKeyAnalyzerEndToEnd verifies that CollationKeyAnalyzer wires
// KeywordTokenizer with CollationAttributeFactory so the emitted token's
// BytesRef is the collation key produced by the configured Collator, not the
// raw UTF-8 input.
func TestCollationKeyAnalyzerEndToEnd(t *testing.T) {
	t.Parallel()

	a := NewCollationKeyAnalyzer(upperKeyCollator{})
	ts, err := a.TokenStream("field", bytes.NewReader([]byte("Hello")))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	if ts == nil {
		t.Fatal("TokenStream() returned nil")
	}

	hasToken, err := ts.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken: %v", err)
	}
	if !hasToken {
		t.Fatal("expected one token, got none")
	}

	attrSrc := ts.(interface {
		GetAttributeSource() *util.AttributeSource
	}).GetAttributeSource()
	termAttr := attrSrc.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	ref := termAttr.GetBytesRef()
	if ref == nil {
		t.Fatal("GetBytesRef() = nil")
	}

	want := []byte("HELLO")
	got := ref.Bytes[ref.Offset : ref.Offset+ref.Length]
	if !bytes.Equal(got, want) {
		t.Errorf("collation key bytes = % x, want % x", got, want)
	}

	// The same analyzer must produce equal keys for strings that the collator
	// deems equivalent (case-folding in this test collator).
	ts2, err := a.TokenStream("field", bytes.NewReader([]byte("hElLo")))
	if err != nil {
		t.Fatalf("TokenStream for hElLo: %v", err)
	}
	if _, err := ts2.IncrementToken(); err != nil {
		t.Fatalf("IncrementToken for hElLo: %v", err)
	}
	attrSrc2 := ts2.(interface {
		GetAttributeSource() *util.AttributeSource
	}).GetAttributeSource()
	termAttr2 := attrSrc2.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	ref2 := termAttr2.GetBytesRef()
	got2 := ref2.Bytes[ref2.Offset : ref2.Offset+ref2.Length]
	if !bytes.Equal(got2, want) {
		t.Errorf("collation key bytes for hElLo = % x, want % x", got2, want)
	}

	// KeywordTokenizer emits exactly one token per input.
	hasMore, err := ts.IncrementToken()
	if err != nil {
		t.Fatalf("second IncrementToken: %v", err)
	}
	if hasMore {
		t.Error("expected exactly one token")
	}
}

// TestCollationKeyAnalyzerLocaleEquivalentStrings checks that two strings the
// configured collator treats as equal yield identical collation key bytes.
func TestCollationKeyAnalyzerLocaleEquivalentStrings(t *testing.T) {
	t.Parallel()

	a := NewCollationKeyAnalyzer(upperKeyCollator{})
	keyFor := func(input string) []byte {
		ts, err := a.TokenStream("field", bytes.NewReader([]byte(input)))
		if err != nil {
			t.Fatalf("TokenStream(%q): %v", input, err)
		}
		if _, err := ts.IncrementToken(); err != nil {
			t.Fatalf("IncrementToken(%q): %v", input, err)
		}
		attrSrc := ts.(interface {
			GetAttributeSource() *util.AttributeSource
		}).GetAttributeSource()
		termAttr := attrSrc.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
		ref := termAttr.GetBytesRef()
		return ref.Bytes[ref.Offset : ref.Offset+ref.Length]
	}

	aKey := keyFor("Apple")
	bKey := keyFor("APPLE")
	if !bytes.Equal(aKey, bKey) {
		t.Errorf("collator-equal strings produced different keys: % x vs % x", aKey, bKey)
	}
}
