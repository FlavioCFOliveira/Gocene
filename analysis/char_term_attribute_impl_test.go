// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Tests ported from Apache Lucene 10.4.0:
// lucene/core/src/test/org/apache/lucene/analysis/tokenattributes/TestCharTermAttributeImpl.java

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// assertCharTermCloneIsEqual is the Go equivalent of
// TestCharTermAttributeImpl.assertCloneIsEqual.
func assertCharTermCloneIsEqual(t *testing.T, att *CharTermAttributeImpl) *CharTermAttributeImpl {
	t.Helper()
	clone, ok := att.CloneAttribute().(*CharTermAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *CharTermAttributeImpl")
	}
	if !att.Equals(clone) {
		t.Fatal("clone must be equal to original")
	}
	if att.HashCode() != clone.HashCode() {
		t.Fatal("clone hashcode must equal original hashcode")
	}
	return clone
}

// assertCharTermCopyIsEqual is the Go equivalent of
// TestCharTermAttributeImpl.assertCopyIsEqual.
func assertCharTermCopyIsEqual(t *testing.T, att *CharTermAttributeImpl) *CharTermAttributeImpl {
	t.Helper()
	cp := NewCharTermAttributeImpl()
	att.CopyTo(cp)
	if !att.Equals(cp) {
		t.Fatal("copied instance must be equal to original")
	}
	if att.HashCode() != cp.HashCode() {
		t.Fatal("copied instance hashcode must equal original hashcode")
	}
	return cp
}

// TestCharTermAttributeImpl_Resize mirrors testResize.
func TestCharTermAttributeImpl_Resize(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.SetValue("hello")
	for i := 0; i < 2000; i++ {
		attr.ResizeBuffer(i)
		if i > len(attr.Buffer()) {
			t.Fatalf("i=%d > buffer length %d", i, len(attr.Buffer()))
		}
		if attr.String() != "hello" {
			t.Fatalf("unexpected term after ResizeBuffer(%d): %q", i, attr.String())
		}
	}
}

// TestCharTermAttributeImpl_SetLength mirrors testSetLength.
func TestCharTermAttributeImpl_SetLength(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.SetValue("hello")
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetLength(-1) must panic")
		}
	}()
	attr.SetLength(-1)
}

// TestCharTermAttributeImpl_Grow mirrors testGrow.
func TestCharTermAttributeImpl_Grow(t *testing.T) {
	// First variant: repeated copyBuffer (SetValue) doubling.
	attr := NewCharTermAttributeImpl()
	buf := "ab"
	for i := 0; i < 20; i++ {
		attr.SetValue(buf)
		if attr.Length() != len(buf) {
			t.Fatalf("iteration %d: want length %d, got %d", i, len(buf), attr.Length())
		}
		if attr.String() != buf {
			t.Fatalf("iteration %d: want %q, got %q", i, buf, attr.String())
		}
		buf += buf
	}
	if attr.Length() != 1<<20 {
		t.Fatalf("want length 1048576, got %d", attr.Length())
	}

	// Second variant: setEmpty().append(buf) doubling.
	attr = NewCharTermAttributeImpl()
	buf = "ab"
	for i := 0; i < 20; i++ {
		attr.SetEmpty()
		attr.AppendString(buf)
		if attr.Length() != len(buf) {
			t.Fatalf("iteration %d: want length %d, got %d", i, len(buf), attr.Length())
		}
		if attr.String() != buf {
			t.Fatalf("iteration %d: want %q, got %q", i, buf, attr.String())
		}
		buf += attr.String()
	}
	if attr.Length() != 1<<20 {
		t.Fatalf("want length 1048576, got %d", attr.Length())
	}

	// Third variant: slow growth to long term.
	attr = NewCharTermAttributeImpl()
	singleChar := "a"
	for i := 0; i < 20000; i++ {
		attr.SetEmpty()
		attr.AppendString(singleChar)
		if attr.Length() != len(singleChar) {
			t.Fatalf("iteration %d: want length %d, got %d", i, len(singleChar), attr.Length())
		}
		if attr.String() != singleChar {
			t.Fatalf("iteration %d: want %q, got %q", i, singleChar, attr.String())
		}
		singleChar += "a"
	}
	if attr.Length() != 20000 {
		t.Fatalf("want length 20000, got %d", attr.Length())
	}
}

// TestCharTermAttributeImpl_ToString mirrors testToString.
func TestCharTermAttributeImpl_ToString(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.SetValue("aloha")
	if attr.String() != "aloha" {
		t.Fatalf("want %q, got %q", "aloha", attr.String())
	}
	attr.SetEmpty()
	attr.AppendString("hi there")
	if attr.String() != "hi there" {
		t.Fatalf("want %q, got %q", "hi there", attr.String())
	}
}

// TestCharTermAttributeImpl_Clone mirrors testClone.
func TestCharTermAttributeImpl_Clone(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.SetValue("hello")
	origBuf := attr.Buffer()
	clone := assertCharTermCloneIsEqual(t, attr)
	if attr.String() != clone.String() {
		t.Fatal("clone string must match original")
	}
	// Buffers must be distinct slices.
	if &origBuf[0] == &clone.Buffer()[0] {
		t.Fatal("clone must not share backing buffer with original")
	}
}

// TestCharTermAttributeImpl_Equals mirrors testEquals.
func TestCharTermAttributeImpl_Equals(t *testing.T) {
	a := NewCharTermAttributeImpl()
	a.SetValue("hello")
	b := NewCharTermAttributeImpl()
	b.SetValue("hello")
	c := NewCharTermAttributeImpl()
	c.SetValue("hello2")

	if !a.Equals(b) {
		t.Fatal("equal terms must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different terms must not be equal")
	}
	if c.Equals(b) {
		t.Fatal("different terms must not be equal (reverse)")
	}
}

// TestCharTermAttributeImpl_CopyTo mirrors testCopyTo.
func TestCharTermAttributeImpl_CopyTo(t *testing.T) {
	// Empty attribute.
	attr := NewCharTermAttributeImpl()
	cp := assertCharTermCopyIsEqual(t, attr)
	if attr.String() != "" || cp.String() != "" {
		t.Fatal("both must be empty")
	}

	// Non-empty attribute.
	attr = NewCharTermAttributeImpl()
	attr.SetValue("hello")
	origBuf := attr.Buffer()
	cp = assertCharTermCopyIsEqual(t, attr)
	if attr.String() != cp.String() {
		t.Fatal("copied string must match original")
	}
	if len(origBuf) > 0 && len(cp.Buffer()) > 0 && &origBuf[0] == &cp.Buffer()[0] {
		t.Fatal("copy must not share backing buffer with original")
	}
}

// TestCharTermAttributeImpl_AppendString covers string appends.
func TestCharTermAttributeImpl_AppendString(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.AppendString("0123456789")
	attr.AppendString("0123456789")
	if attr.String() != "01234567890123456789" {
		t.Fatalf("want %q, got %q", "01234567890123456789", attr.String())
	}
}

// TestCharTermAttributeImpl_AppendNull covers nil/empty appends do not
// panic and the Java "null" string behaviour is not applicable in Go
// (we simply skip nil for strings).
func TestCharTermAttributeImpl_AppendNull(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.AppendString("test")
	attr.AppendString("")
	if attr.String() != "test" {
		t.Fatalf("appending empty string must not change content; got %q", attr.String())
	}
}

// TestCharTermAttributeImpl_SubSequence verifies SubSequence via String slicing.
func TestCharTermAttributeImpl_SubSequence(t *testing.T) {
	s := "0123456789"
	attr := NewCharTermAttributeImpl()
	attr.SetValue(s)
	if attr.Length() != len(s) {
		t.Fatalf("want length %d, got %d", len(s), attr.Length())
	}
	sub := attr.String()[1:3]
	if sub != "12" {
		t.Fatalf("want %q, got %q", "12", sub)
	}
}

// TestCharTermAttributeImpl_Clear verifies Clear resets the length.
func TestCharTermAttributeImpl_Clear(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.SetValue("hello")
	attr.Clear()
	if attr.Length() != 0 {
		t.Fatalf("after Clear want length 0, got %d", attr.Length())
	}
	if attr.String() != "" {
		t.Fatalf("after Clear want empty string, got %q", attr.String())
	}
}

// TestCharTermAttributeImpl_AttributeInterfaces verifies the declared
// interface types match expectations.
func TestCharTermAttributeImpl_AttributeInterfaces(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	ifaces := attr.AttributeInterfaces()
	if len(ifaces) != 2 {
		t.Fatalf("want 2 interface types, got %d", len(ifaces))
	}
	found := map[string]bool{}
	for _, tp := range ifaces {
		found[tp.Name()] = true
	}
	if !found["CharTermAttribute"] {
		t.Fatal("CharTermAttributeType not declared")
	}
	if !found["TermToBytesRefAttribute"] {
		t.Fatal("TermToBytesRefAttributeType not declared")
	}
}

// TestCharTermAttributeImpl_GetBytesRef verifies the live BytesRef
// contract: GetBytesRef returns the current term bytes.
func TestCharTermAttributeImpl_GetBytesRef(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.SetValue("foobar")
	ref := attr.GetBytesRef()
	if ref == nil {
		t.Fatal("GetBytesRef must not return nil")
	}
	if ref.Length != len("foobar") {
		t.Fatalf("want length %d, got %d", len("foobar"), ref.Length)
	}
	if string(ref.Bytes[ref.Offset:ref.Offset+ref.Length]) != "foobar" {
		t.Fatal("GetBytesRef bytes do not match term")
	}
}

// TestCharTermAttributeImpl_LargeAppend mirrors
// testAppendableInterfaceWithLongSequences for ASCII sequences.
func TestCharTermAttributeImpl_LargeAppend(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	s1 := "01234567890123456789012345678901234567890123456789"
	attr.AppendString(s1)
	// Append a slice of s1 bytes [3:50].
	attr.Append([]byte(s1)[3:50])
	want := s1 + s1[3:50]
	if attr.String() != want {
		t.Fatalf("want %q, got %q", want, attr.String())
	}

	attr.SetEmpty()
	sub := "01234567890123456789"
	attr.Append([]byte(sub)[5:17])
	if attr.String() != "567890123456" {
		t.Fatalf("want %q, got %q", "567890123456", attr.String())
	}
}

// TestCharTermAttributeImpl_ReflectWith covers the two reflect triples.
func TestCharTermAttributeImpl_ReflectWith(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.AppendString("foobar")

	type entry struct {
		key string
		val any
	}
	var got []entry
	attr.ReflectWith(func(tp reflect.Type, key string, val any) {
		got = append(got, entry{key, val})
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 reflect entries, got %d", len(got))
	}
	if got[0].key != "term" || got[0].val != "foobar" {
		t.Fatalf("unexpected first entry: key=%q val=%v", got[0].key, got[0].val)
	}
	if got[1].key != "bytes" {
		t.Fatalf("unexpected second entry key: %q", got[1].key)
	}
}

// TestCharTermAttributeImpl_AppendSelf mirrors the Java append(t2)
// for CharTermAttribute to CharTermAttribute.
func TestCharTermAttributeImpl_AppendSelf(t *testing.T) {
	a := NewCharTermAttributeImpl()
	a.AppendString("hello")
	b := NewCharTermAttributeImpl()
	b.AppendString("world")
	a.AppendChars(b.Bytes())
	if a.String() != "helloworld" {
		t.Fatalf("want %q, got %q", "helloworld", a.String())
	}
}

// TestCharTermAttributeImpl_HashCodeParity checks that equal attributes
// produce the same hash code.
func TestCharTermAttributeImpl_HashCodeParity(t *testing.T) {
	a := NewCharTermAttributeImpl()
	a.SetValue("test")
	b := NewCharTermAttributeImpl()
	b.SetValue("test")
	if a.HashCode() != b.HashCode() {
		t.Fatalf("equal attributes must have equal hash codes: %d vs %d",
			a.HashCode(), b.HashCode())
	}
}

// TestCharTermAttributeImpl_Bytes covers the Bytes() deep-copy.
func TestCharTermAttributeImpl_Bytes(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.SetValue("hello")
	b := attr.Bytes()
	if string(b) != "hello" {
		t.Fatalf("want %q, got %q", "hello", string(b))
	}
	// Mutation of returned slice must not affect attribute.
	b[0] = 'X'
	if attr.String() != "hello" {
		t.Fatal("Bytes() must return a copy")
	}
}

// TestCharTermAttributeImpl_AppendChar covers single-byte append.
func TestCharTermAttributeImpl_AppendChar(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.AppendString("hell")
	attr.AppendChar('o')
	if attr.String() != "hello" {
		t.Fatalf("want %q, got %q", "hello", attr.String())
	}
}

// TestCharTermAttributeImpl_CopyTo_WrongTarget verifies the panic path.
func TestCharTermAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type must panic")
		}
	}()
	attr := NewCharTermAttributeImpl()
	attr.CopyTo(NewSentenceAttributeImpl())
}

// TestCharTermAttributeImpl_AttributeReflection mirrors
// testAttributeReflection (keys are interface-name#field).
func TestCharTermAttributeImpl_AttributeReflection(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.AppendString("foobar")

	reflected := map[string]any{}
	attr.ReflectWith(func(tp reflect.Type, key string, val any) {
		reflected[tp.String()+"#"+key] = val
	})
	if reflected["analysis.CharTermAttribute#term"] != "foobar" {
		t.Fatalf("unexpected CharTermAttribute#term value: %v",
			reflected["analysis.CharTermAttribute#term"])
	}
	ref, ok := reflected["analysis.TermToBytesRefAttribute#bytes"]
	if !ok {
		t.Fatal("TermToBytesRefAttribute#bytes not reflected")
	}
	if ref == nil {
		t.Fatal("reflected bytes must not be nil")
	}
}

// TestCharTermAttributeImpl_End verifies that End resets the length.
func TestCharTermAttributeImpl_End(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	attr.SetValue("hello")
	attr.End()
	if attr.Length() != 0 {
		t.Fatalf("End must reset length; got %d", attr.Length())
	}
}

// TestCharTermAttributeImpl_SetValue_Chaining verifies SetValue returns
// the attribute for chaining.
func TestCharTermAttributeImpl_SetValue_Chaining(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	result := attr.SetValue("chain")
	if result != attr {
		t.Fatal("SetValue must return the same attribute")
	}
	if attr.String() != "chain" {
		t.Fatalf("want %q, got %q", "chain", attr.String())
	}
}

// TestCharTermAttributeImpl_LongGrowth confirms the buffer stays
// consistent across many doublings.
func TestCharTermAttributeImpl_LongGrowth(t *testing.T) {
	attr := NewCharTermAttributeImpl()
	want := strings.Repeat("a", 1<<16)
	attr.SetValue(want)
	if attr.String() != want {
		t.Fatal("long growth: term mismatch")
	}
}
