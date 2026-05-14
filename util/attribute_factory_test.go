// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"reflect"
	"testing"
)

// fakeIntAttribute is a domain Attribute interface for tests in this
// package. Equivalent to {@code interface FooAttribute extends
// Attribute} in Lucene's pattern.
type fakeIntAttribute interface {
	Attribute
	Int() int
	SetInt(int)
}

// fakeIntAttributeImpl is the matching AttributeImpl. It satisfies
// Clear/End/ReflectWith/CopyTo/CloneAttribute plus the
// fakeIntAttribute sub-interface.
type fakeIntAttributeImpl struct {
	BaseAttributeImpl
	value int
}

func (a *fakeIntAttributeImpl) Int() int     { return a.value }
func (a *fakeIntAttributeImpl) SetInt(v int) { a.value = v }
func (a *fakeIntAttributeImpl) Clear()       { a.value = 0 }
func (a *fakeIntAttributeImpl) End()         { a.Clear() }
func (a *fakeIntAttributeImpl) ReflectWith(r AttributeReflector) {
	r(fakeIntAttributeType, "int", a.value)
}
func (a *fakeIntAttributeImpl) CopyTo(target AttributeImpl) {
	t, ok := target.(*fakeIntAttributeImpl)
	if !ok {
		panic("CopyTo target must be *fakeIntAttributeImpl")
	}
	t.value = a.value
}
func (a *fakeIntAttributeImpl) CloneAttribute() AttributeImpl {
	c := *a
	return &c
}

var fakeIntAttributeType = reflect.TypeOf((*fakeIntAttribute)(nil)).Elem()

func newFakeIntAttributeImpl() AttributeImpl { return &fakeIntAttributeImpl{} }

func TestDefaultAttributeFactory_RegisteredType(t *testing.T) {
	// Register and resolve.
	RegisterAttributeImpl(fakeIntAttributeType, newFakeIntAttributeImpl)

	impl := DefaultAttributeFactoryInstance.CreateAttributeInstance(fakeIntAttributeType)
	if impl == nil {
		t.Fatal("CreateAttributeInstance returned nil for registered type")
	}
	concrete, ok := impl.(*fakeIntAttributeImpl)
	if !ok {
		t.Fatalf("CreateAttributeInstance returned %T, want *fakeIntAttributeImpl", impl)
	}
	if concrete.Int() != 0 {
		t.Errorf("freshly created impl has Int() = %d, want 0", concrete.Int())
	}
}

func TestDefaultAttributeFactory_UnregisteredTypePanics(t *testing.T) {
	type otherAttribute interface {
		Attribute
		other()
	}
	t.Cleanup(func() { _ = recover() })

	defer func() {
		if r := recover(); r == nil {
			t.Error("CreateAttributeInstance should panic for unregistered type")
		}
	}()
	DefaultAttributeFactoryInstance.CreateAttributeInstance(reflect.TypeOf((*otherAttribute)(nil)).Elem())
}

func TestRegisterAttributeImpl_RejectsNonInterface(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("RegisterAttributeImpl should panic for non-interface attType")
		}
	}()
	RegisterAttributeImpl(reflect.TypeOf(0), newFakeIntAttributeImpl)
}

func TestRegisterAttributeImpl_RejectsNilFactory(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("RegisterAttributeImpl should panic for nil factory")
		}
	}()
	RegisterAttributeImpl(fakeIntAttributeType, nil)
}

func TestStaticImplementationAttributeFactory_PrefersOwnImpl(t *testing.T) {
	RegisterAttributeImpl(fakeIntAttributeType, newFakeIntAttributeImpl)
	delegate := DefaultAttributeFactoryInstance
	saf := NewStaticImplementationAttributeFactory(delegate, newFakeIntAttributeImpl)

	got := saf.CreateAttributeInstance(fakeIntAttributeType)
	if _, ok := got.(*fakeIntAttributeImpl); !ok {
		t.Fatalf("got %T, want *fakeIntAttributeImpl", got)
	}
}

// stubAttributeImpl is unrelated to fakeIntAttribute; used to verify the
// delegate path of StaticImplementationAttributeFactory.
type stubAttributeImpl struct{ BaseAttributeImpl }

func (s *stubAttributeImpl) Clear()                         {}
func (s *stubAttributeImpl) End()                           {}
func (s *stubAttributeImpl) ReflectWith(AttributeReflector) {}
func (s *stubAttributeImpl) CopyTo(AttributeImpl)           {}
func (s *stubAttributeImpl) CloneAttribute() AttributeImpl  { return &stubAttributeImpl{} }

func TestStaticImplementationAttributeFactory_DelegatesUnknownTypes(t *testing.T) {
	RegisterAttributeImpl(fakeIntAttributeType, newFakeIntAttributeImpl)

	// stub impl does not satisfy fakeIntAttribute (no Int/SetInt), so
	// the static factory must delegate.
	saf := NewStaticImplementationAttributeFactory(
		DefaultAttributeFactoryInstance,
		func() AttributeImpl { return &stubAttributeImpl{} },
	)

	got := saf.CreateAttributeInstance(fakeIntAttributeType)
	if _, ok := got.(*fakeIntAttributeImpl); !ok {
		t.Fatalf("got %T, want delegate to produce *fakeIntAttributeImpl", got)
	}
}

func TestStaticImplementationAttributeFactory_Equals(t *testing.T) {
	RegisterAttributeImpl(fakeIntAttributeType, newFakeIntAttributeImpl)
	a := NewStaticImplementationAttributeFactory(DefaultAttributeFactoryInstance, newFakeIntAttributeImpl)
	b := NewStaticImplementationAttributeFactory(DefaultAttributeFactoryInstance, newFakeIntAttributeImpl)
	if !a.Equals(b) {
		t.Error("two factories with same delegate and impl should be equal")
	}
	if a.Equals(nil) {
		t.Error("factory should not be equal to nil")
	}
}
