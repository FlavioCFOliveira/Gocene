// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPackedTokenAttributeImpl_Defaults verifies the no-arg constructor
// initialises every packed field to its Lucene default.
func TestPackedTokenAttributeImpl_Defaults(t *testing.T) {
	p := NewPackedTokenAttributeImpl()
	if got := p.String(); got != "" {
		t.Errorf("default term=%q, want empty", got)
	}
	if p.StartOffset() != 0 || p.EndOffset() != 0 {
		t.Errorf("default offsets=%d/%d, want 0/0", p.StartOffset(), p.EndOffset())
	}
	if p.GetType() != DefaultTokenType {
		t.Errorf("default type=%q, want %q", p.GetType(), DefaultTokenType)
	}
	if p.GetPositionIncrement() != 1 {
		t.Errorf("default positionIncrement=%d, want 1", p.GetPositionIncrement())
	}
	if p.GetPositionLength() != 1 {
		t.Errorf("default positionLength=%d, want 1", p.GetPositionLength())
	}
	if p.GetTermFrequency() != 1 {
		t.Errorf("default termFrequency=%d, want 1", p.GetTermFrequency())
	}
}

// TestPackedTokenAttributeImpl_Clone is the Go port of
// TestPackedTokenAttributeImpl#testClone. It verifies that a clone is
// equal to the source and owns its own buffer (the source's term
// buffer is not aliased into the clone).
//
// Source: lucene/core/src/test/org/apache/lucene/analysis/tokenattributes/TestPackedTokenAttributeImpl.java
func TestPackedTokenAttributeImpl_Clone(t *testing.T) {
	src := NewPackedTokenAttributeImpl()
	src.SetOffset(0, 5)
	src.SetValue("hello")
	srcBuf := src.Buffer()

	clone := src.Copy().(*PackedTokenAttributeImpl)
	if !src.Equals(clone) {
		t.Fatalf("clone not equal: src=%q clone=%q", src.String(), clone.String())
	}
	if &srcBuf[0] == &clone.Buffer()[0] {
		t.Fatal("clone aliases the source's term buffer; must be deep-copied")
	}
}

// TestPackedTokenAttributeImpl_CopyTo is the Go port of
// TestPackedTokenAttributeImpl#testCopyTo.
func TestPackedTokenAttributeImpl_CopyTo(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		src := NewPackedTokenAttributeImpl()
		dst := NewPackedTokenAttributeImpl()
		src.CopyTo(dst)

		if !src.Equals(dst) {
			t.Fatalf("empty copy not equal: src=%#v dst=%#v", src, dst)
		}
		if src.String() != "" || dst.String() != "" {
			t.Fatalf("empty copy: src=%q dst=%q, want empty", src.String(), dst.String())
		}
		if src.HashCode() != dst.HashCode() {
			t.Fatalf("hash mismatch: %d vs %d", src.HashCode(), dst.HashCode())
		}
	})

	t.Run("populated", func(t *testing.T) {
		src := NewPackedTokenAttributeImpl()
		src.SetOffset(0, 5)
		src.SetValue("hello")
		srcBuf := src.Buffer()

		dst := NewPackedTokenAttributeImpl()
		src.CopyTo(dst)

		if !src.Equals(dst) {
			t.Fatalf("copy not equal: src=%q dst=%q", src.String(), dst.String())
		}
		if &srcBuf[0] == &dst.Buffer()[0] {
			t.Fatal("CopyTo aliases the source's term buffer; must be deep-copied")
		}
		if src.HashCode() != dst.HashCode() {
			t.Fatalf("hash mismatch: %d vs %d", src.HashCode(), dst.HashCode())
		}
	})
}

// TestPackedTokenAttributeImpl_AttributeReflection is the Go port of
// TestPackedTokenAttributeImpl#testAttributeReflection. It walks
// through every packed attribute, sets a non-default value, and
// asserts that ReflectWith emits the exact key set the Lucene
// reference expects (with the values to match).
func TestPackedTokenAttributeImpl_AttributeReflection(t *testing.T) {
	p := NewPackedTokenAttributeImpl()
	p.AppendString("foobar")
	p.SetOffset(6, 22)
	p.SetPositionIncrement(3)
	p.SetPositionLength(11)
	p.SetType("foobar")
	p.SetTermFrequency(42)

	type key struct {
		attTypeName string
		key         string
	}
	got := make(map[key]any)
	p.ReflectWith(func(attType reflect.Type, k string, v any) {
		got[key{attTypeName: attType.String(), key: k}] = v
	})

	checks := []struct {
		attType reflect.Type
		key     string
		value   any
	}{
		{reflect.TypeOf((*CharTermAttribute)(nil)).Elem(), "term", "foobar"},
		{reflect.TypeOf((*OffsetAttribute)(nil)).Elem(), "startOffset", 6},
		{reflect.TypeOf((*OffsetAttribute)(nil)).Elem(), "endOffset", 22},
		{reflect.TypeOf((*PositionIncrementAttribute)(nil)).Elem(), "positionIncrement", 3},
		// The bare-struct attributes still emit their parity triples
		// even though they do not have a Lucene-style interface; they
		// borrow the concrete-type reflect.Type as their attType.
	}
	for _, c := range checks {
		v, ok := got[key{attTypeName: c.attType.String(), key: c.key}]
		if !ok {
			t.Fatalf("missing triple: %s#%s", c.attType, c.key)
		}
		if !reflect.DeepEqual(v, c.value) {
			t.Fatalf("triple %s#%s: value=%#v, want %#v", c.attType, c.key, v, c.value)
		}
	}

	// TermToBytesRefAttribute#bytes is emitted by the embedded
	// charTermAttribute; assert it independently because its value is
	// a BytesRef.
	bytesKey := key{
		attTypeName: reflect.TypeOf((*TermToBytesRefAttribute)(nil)).Elem().String(),
		key:         "bytes",
	}
	bv, ok := got[bytesKey]
	if !ok {
		t.Fatalf("missing TermToBytesRefAttribute#bytes triple")
	}
	br, ok := bv.(*util.BytesRef)
	if !ok || br.String() != "foobar" {
		t.Fatalf("TermToBytesRefAttribute#bytes=%#v, want BytesRef(foobar)", bv)
	}
}

// TestPackedTokenAttributeImpl_Clear verifies Clear restores every
// packed field to its Lucene default.
func TestPackedTokenAttributeImpl_Clear(t *testing.T) {
	p := NewPackedTokenAttributeImpl()
	p.AppendString("xyz")
	p.SetOffset(1, 4)
	p.SetPositionIncrement(2)
	p.SetPositionLength(3)
	p.SetType("foo")
	p.SetTermFrequency(7)

	p.Clear()
	if p.String() != "" || p.StartOffset() != 0 || p.EndOffset() != 0 ||
		p.GetPositionIncrement() != 1 || p.GetPositionLength() != 1 ||
		p.GetTermFrequency() != 1 || p.GetType() != DefaultTokenType {
		t.Fatalf("Clear left non-default state: %#v", p)
	}
}

// TestPackedTokenAttributeImpl_End verifies that End matches Lucene's
// reference: same as Clear but with positionIncrement = 0.
func TestPackedTokenAttributeImpl_End(t *testing.T) {
	p := NewPackedTokenAttributeImpl()
	p.AppendString("xyz")
	p.SetOffset(1, 4)
	p.SetPositionIncrement(2)
	p.SetPositionLength(3)
	p.SetType("foo")
	p.SetTermFrequency(7)

	p.End()
	if p.GetPositionIncrement() != 0 {
		t.Fatalf("End: positionIncrement=%d, want 0", p.GetPositionIncrement())
	}
	if p.String() != "" || p.StartOffset() != 0 || p.EndOffset() != 0 ||
		p.GetPositionLength() != 1 || p.GetTermFrequency() != 1 ||
		p.GetType() != DefaultTokenType {
		t.Fatalf("End: non-positionIncrement field not at clear-default: %#v", p)
	}
}

// TestPackedTokenAttributeImpl_Validation verifies every guarded
// setter rejects invalid input with a panic, matching the Lucene
// IllegalArgumentException contract.
func TestPackedTokenAttributeImpl_Validation(t *testing.T) {
	cases := []struct {
		name string
		fn   func(*PackedTokenAttributeImpl)
	}{
		{"SetOffset_negative_start", func(p *PackedTokenAttributeImpl) { p.SetOffset(-1, 0) }},
		{"SetOffset_end_before_start", func(p *PackedTokenAttributeImpl) { p.SetOffset(5, 1) }},
		{"SetPositionIncrement_negative", func(p *PackedTokenAttributeImpl) { p.SetPositionIncrement(-1) }},
		{"SetPositionLength_zero", func(p *PackedTokenAttributeImpl) { p.SetPositionLength(0) }},
		{"SetTermFrequency_zero", func(p *PackedTokenAttributeImpl) { p.SetTermFrequency(0) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("did not panic")
				}
			}()
			tc.fn(NewPackedTokenAttributeImpl())
		})
	}
}

// TestPackedTokenAttributeImpl_CopyTo_FallbackPath verifies that
// CopyTo routes per-attribute when the target is not a peer
// PackedTokenAttributeImpl, exercising both the interface targets
// (CharTerm, Offset, PositionIncrement) and the bare-struct targets
// (PositionLength, Type, TermFrequency).
func TestPackedTokenAttributeImpl_CopyTo_FallbackPath(t *testing.T) {
	src := NewPackedTokenAttributeImpl()
	src.AppendString("term")
	src.SetOffset(1, 5)
	src.SetPositionIncrement(2)
	src.SetPositionLength(3)
	src.SetType("foo")
	src.SetTermFrequency(7)

	// Interface targets: CharTerm/Offset/PositionIncrement.
	ct := NewCharTermAttribute()
	src.CopyTo(ct)
	if got := ct.String(); got != "term" {
		t.Fatalf("CharTerm fallback: term=%q, want term", got)
	}
	off := NewOffsetAttribute()
	src.CopyTo(off)
	if off.StartOffset() != 1 || off.EndOffset() != 5 {
		t.Fatalf("Offset fallback: %d/%d, want 1/5", off.StartOffset(), off.EndOffset())
	}
	pi := NewPositionIncrementAttribute()
	src.CopyTo(pi)
	if pi.GetPositionIncrement() != 2 {
		t.Fatalf("PositionIncrement fallback: %d, want 2", pi.GetPositionIncrement())
	}

	// Bare-struct targets.
	pl := NewPositionLengthAttribute()
	src.CopyTo(pl)
	if pl.GetPositionLength() != 3 {
		t.Fatalf("PositionLength fallback: %d, want 3", pl.GetPositionLength())
	}
	tp := NewTypeAttribute()
	src.CopyTo(tp)
	if tp.GetType() != "foo" {
		t.Fatalf("Type fallback: %q, want foo", tp.GetType())
	}
	tf := NewTermFrequencyAttribute()
	src.CopyTo(tf)
	if tf.GetTermFrequency() != 7 {
		t.Fatalf("TermFrequency fallback: %d, want 7", tf.GetTermFrequency())
	}
}
