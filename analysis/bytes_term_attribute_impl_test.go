// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestBytesTermAttributeImpl_CopyTo is the Go port of
// TestBytesRefAttImpl#testCopyTo. It verifies the Lucene Javadoc
// contract: an empty copy round-trips equal, and a populated copy is
// equal (BytesRefEquals) without being the same pointer.
//
// Source: lucene/core/src/test/org/apache/lucene/analysis/tokenattributes/TestBytesRefAttImpl.java
func TestBytesTermAttributeImpl_CopyTo(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		src := NewBytesTermAttributeImpl()
		dst := NewBytesTermAttributeImpl()

		src.CopyTo(dst)

		if !src.Equals(dst) {
			t.Fatalf("empty copy not equal: src=%#v dst=%#v", src, dst)
		}
		if dst.GetBytesRef() != nil {
			t.Fatalf("empty copy BytesRef=%#v, want nil", dst.GetBytesRef())
		}
		if src.HashCode() != dst.HashCode() {
			t.Fatalf("hash mismatch: %d vs %d", src.HashCode(), dst.HashCode())
		}
	})

	t.Run("populated", func(t *testing.T) {
		src := NewBytesTermAttributeImpl()
		src.SetBytesRef(util.NewBytesRef([]byte("hello")))
		dst := NewBytesTermAttributeImpl()

		src.CopyTo(dst)

		if !src.Equals(dst) {
			t.Fatalf("copy not equal: src.bytes=%q dst.bytes=%q",
				src.GetBytesRef().String(), dst.GetBytesRef().String())
		}
		if src.GetBytesRef() == dst.GetBytesRef() {
			t.Fatal("copy must own its own BytesRef pointer (deep copy contract)")
		}
		if src.HashCode() != dst.HashCode() {
			t.Fatalf("hash mismatch: %d vs %d", src.HashCode(), dst.HashCode())
		}
	})
}

// TestBytesTermAttributeImpl_Lucene9856 mirrors
// TestBytesRefAttImpl#testLucene9856: BytesTermAttributeImpl must
// satisfy [TermToBytesRefAttribute] explicitly.
func TestBytesTermAttributeImpl_Lucene9856(t *testing.T) {
	var _ TermToBytesRefAttribute = (*BytesTermAttributeImpl)(nil)
}

// TestBytesTermAttributeImpl_Clear verifies Clear resets bytes to nil
// in line with the Java reference.
func TestBytesTermAttributeImpl_Clear(t *testing.T) {
	impl := NewBytesTermAttributeImpl()
	impl.SetBytesRef(util.NewBytesRef([]byte("data")))
	impl.Clear()
	if got := impl.GetBytesRef(); got != nil {
		t.Fatalf("Clear left bytes=%#v, want nil", got)
	}
}

// TestBytesTermAttributeImpl_Copy verifies Copy returns an independent
// clone whose mutation does not affect the source.
func TestBytesTermAttributeImpl_Copy(t *testing.T) {
	src := NewBytesTermAttributeImpl()
	src.SetBytesRef(util.NewBytesRef([]byte("alpha")))

	clone := src.Copy().(*BytesTermAttributeImpl)
	clone.SetBytesRef(util.NewBytesRef([]byte("beta")))

	if got := src.GetBytesRef().String(); got != "alpha" {
		t.Fatalf("Copy is not independent: src bytes=%q, want %q", got, "alpha")
	}
	if got := clone.GetBytesRef().String(); got != "beta" {
		t.Fatalf("clone bytes=%q, want %q", got, "beta")
	}
}

// TestBytesTermAttributeImpl_CopyTo_WrongTarget verifies the helper
// rejects an unrelated target with an explanatory panic, matching the
// Lucene cast contract.
func TestBytesTermAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type did not panic")
		}
	}()
	impl := NewBytesTermAttributeImpl()
	impl.CopyTo(&MockAttribute{})
}

// TestBytesTermAttributeImpl_ReflectWith verifies the single triple
// emitted matches the Lucene reflectWith output for this attribute.
func TestBytesTermAttributeImpl_ReflectWith(t *testing.T) {
	impl := NewBytesTermAttributeImpl()
	ref := util.NewBytesRef([]byte("xyz"))
	impl.SetBytesRef(ref)

	var keys []string
	var values []any
	var types []reflect.Type
	impl.ReflectWith(func(attType reflect.Type, key string, value any) {
		keys = append(keys, key)
		values = append(values, value)
		types = append(types, attType)
	})

	if len(keys) != 1 || keys[0] != "bytes" {
		t.Fatalf("emitted keys=%v, want [bytes]", keys)
	}
	if got, want := values[0], any(ref); got != want {
		t.Fatalf("emitted value=%#v, want %#v", got, want)
	}
	wantType := reflect.TypeOf((*TermToBytesRefAttribute)(nil)).Elem()
	if types[0] != wantType {
		t.Fatalf("emitted attType=%v, want %v", types[0], wantType)
	}
}
