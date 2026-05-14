// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"reflect"
	"testing"
)

// TestAttributeReflector_FunctionTypeIsCallable confirms the
// AttributeReflector is a callable function type (the Go equivalent of
// the @FunctionalInterface in Java) and accepts the documented
// (attType, key, value) triple.
func TestAttributeReflector_FunctionTypeIsCallable(t *testing.T) {
	type ar interface{ Attribute }
	tp := reflect.TypeOf((*ar)(nil)).Elem()

	var called bool
	var gotType reflect.Type
	var gotKey string
	var gotValue any

	r := AttributeReflector(func(attType reflect.Type, key string, value any) {
		called = true
		gotType = attType
		gotKey = key
		gotValue = value
	})

	r(tp, "k", 42)

	if !called {
		t.Fatal("reflector was not invoked")
	}
	if gotType != tp {
		t.Errorf("attType = %v, want %v", gotType, tp)
	}
	if gotKey != "k" {
		t.Errorf("key = %q, want %q", gotKey, "k")
	}
	if gotValue != 42 {
		t.Errorf("value = %v, want %d", gotValue, 42)
	}
}

// TestAttributeReflector_AcceptsNilValue confirms reflectors must accept
// nil values (Lucene contract: implementations must not skip null
// properties).
func TestAttributeReflector_AcceptsNilValue(t *testing.T) {
	type ar interface{ Attribute }
	tp := reflect.TypeOf((*ar)(nil)).Elem()

	var gotValue any = "untouched"

	r := AttributeReflector(func(_ reflect.Type, _ string, value any) {
		gotValue = value
	})
	r(tp, "k", nil)

	if gotValue != nil {
		t.Errorf("value should be nil after invocation, got %v", gotValue)
	}
}
