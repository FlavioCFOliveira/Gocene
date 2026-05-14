// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"reflect"
	"strings"
	"testing"
)

// charTermAttribute is a fake Lucene-style Attribute used to exercise
// the AttributeImpl contract. Mirrors CharTermAttribute in shape.
type charTermAttribute interface {
	Attribute
	Term() string
	SetTerm(string)
}

type offsetAttribute interface {
	Attribute
	StartOffset() int
	EndOffset() int
	SetOffset(start, end int)
}

// combinedAttributeImpl satisfies both charTermAttribute and
// offsetAttribute, exercising the multi-interface AttributeImpl shape
// from Lucene's docs.
type combinedAttributeImpl struct {
	BaseAttributeImpl
	term  string
	start int
	end   int
}

func (c *combinedAttributeImpl) Term() string                  { return c.term }
func (c *combinedAttributeImpl) SetTerm(s string)              { c.term = s }
func (c *combinedAttributeImpl) StartOffset() int              { return c.start }
func (c *combinedAttributeImpl) EndOffset() int                { return c.end }
func (c *combinedAttributeImpl) SetOffset(start, end int)      { c.start = start; c.end = end }
func (c *combinedAttributeImpl) Clear()                        { c.term = ""; c.start = 0; c.end = 0 }
func (c *combinedAttributeImpl) End()                          { c.Clear() }
func (c *combinedAttributeImpl) CloneAttribute() AttributeImpl { cc := *c; return &cc }
func (c *combinedAttributeImpl) CopyTo(target AttributeImpl) {
	t, ok := target.(*combinedAttributeImpl)
	if !ok {
		panic("CopyTo target must be *combinedAttributeImpl")
	}
	t.term, t.start, t.end = c.term, c.start, c.end
}
func (c *combinedAttributeImpl) ReflectWith(r AttributeReflector) {
	r(charTermAttributeType, "term", c.term)
	r(offsetAttributeType, "startOffset", c.start)
	r(offsetAttributeType, "endOffset", c.end)
}

var (
	charTermAttributeType = reflect.TypeOf((*charTermAttribute)(nil)).Elem()
	offsetAttributeType   = reflect.TypeOf((*offsetAttribute)(nil)).Elem()
)

func TestAttributeImpl_ClearAndEnd(t *testing.T) {
	impl := &combinedAttributeImpl{term: "abc", start: 1, end: 4}
	impl.Clear()
	if impl.term != "" || impl.start != 0 || impl.end != 0 {
		t.Errorf("Clear() left state = %+v, want zero", impl)
	}

	impl.term = "xyz"
	impl.End()
	if impl.term != "" {
		t.Errorf("End() did not delegate to Clear: term = %q", impl.term)
	}
}

func TestAttributeImpl_CopyTo(t *testing.T) {
	src := &combinedAttributeImpl{term: "hello", start: 2, end: 7}
	dst := &combinedAttributeImpl{}
	src.CopyTo(dst)
	if dst.term != "hello" || dst.start != 2 || dst.end != 7 {
		t.Errorf("CopyTo result = %+v, want term=hello start=2 end=7", dst)
	}
}

func TestAttributeImpl_CloneIsDeep(t *testing.T) {
	src := &combinedAttributeImpl{term: "src", start: 1, end: 2}
	cloned := src.CloneAttribute().(*combinedAttributeImpl)
	if cloned == src {
		t.Fatal("clone returned same pointer")
	}
	cloned.term = "mutated"
	if src.term != "src" {
		t.Errorf("mutation of clone leaked to source: src.term = %q", src.term)
	}
}

func TestReflectAsString_NoClassPrefix(t *testing.T) {
	impl := &combinedAttributeImpl{term: "lucene", start: 3, end: 9}
	got := ReflectAsString(impl, false)
	want := "term=lucene,startOffset=3,endOffset=9"
	if got != want {
		t.Errorf("ReflectAsString(false) = %q, want %q", got, want)
	}
}

func TestReflectAsString_WithClassPrefix(t *testing.T) {
	// Register a canonical Java-style name so the output is
	// byte-for-byte parity with the Lucene reference.
	RegisterAttributeClassName(charTermAttributeType, "org.apache.lucene.analysis.tokenattributes.CharTermAttribute")
	RegisterAttributeClassName(offsetAttributeType, "org.apache.lucene.analysis.tokenattributes.OffsetAttribute")

	impl := &combinedAttributeImpl{term: "lucene", start: 3, end: 9}
	got := ReflectAsString(impl, true)
	want := "org.apache.lucene.analysis.tokenattributes.CharTermAttribute#term=lucene," +
		"org.apache.lucene.analysis.tokenattributes.OffsetAttribute#startOffset=3," +
		"org.apache.lucene.analysis.tokenattributes.OffsetAttribute#endOffset=9"
	if got != want {
		t.Errorf("ReflectAsString(true) = %q, want %q", got, want)
	}
}

func TestReflectAsString_NilValueRendersAsNull(t *testing.T) {
	impl := nilValueImpl{}
	got := ReflectAsString(impl, false)
	if !strings.Contains(got, "key=null") {
		t.Errorf("ReflectAsString = %q, want substring %q", got, "key=null")
	}
}

type nilValueImpl struct{ BaseAttributeImpl }

func (nilValueImpl) Clear()                        {}
func (nilValueImpl) End()                          {}
func (nilValueImpl) CopyTo(AttributeImpl)          {}
func (nilValueImpl) CloneAttribute() AttributeImpl { return nilValueImpl{} }
func (nilValueImpl) ReflectWith(r AttributeReflector) {
	r(charTermAttributeType, "key", nil)
}

func TestAttributeClassName_FallbackToReflect(t *testing.T) {
	type unregisteredAttribute interface{ Attribute }
	tp := reflect.TypeOf((*unregisteredAttribute)(nil)).Elem()
	got := AttributeClassName(tp)
	if got == "" {
		t.Error("AttributeClassName should never return an empty string")
	}
}
