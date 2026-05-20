// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/tokenattributes/TestBytesRefAttImpl.java
//
// Deviation: assertCopyIsEqual uses Java reflection (getConstructor().newInstance()).
// In Go the equivalent is calling NewBytesTermAttributeImpl() directly. The
// interface-membership check (testLucene9856) is verified via a compile-time
// assertion rather than runtime reflection.

package analysis

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestBytesRefAttImpl_CopyTo mirrors testCopyTo (Lucene 10.4.0).
// Verifies that CopyTo produces an equal but independently-owned copy
// for both the nil and non-nil BytesRef cases.
func TestBytesRefAttImpl_CopyTo(t *testing.T) {
	src := NewBytesTermAttributeImpl()
	dst := NewBytesTermAttributeImpl()

	// Empty (nil bytes) case: copy must be equal and dst.GetBytesRef() must be nil.
	src.CopyTo(dst)
	if !src.Equals(dst) {
		t.Error("empty copy: src and dst must be equal")
	}
	if dst.GetBytesRef() != nil {
		t.Errorf("empty copy: expected nil GetBytesRef, got %v", dst.GetBytesRef())
	}

	// Non-nil bytes case: copy must be equal but a distinct pointer.
	src.SetBytesRef(util.NewBytesRef([]byte("hello")))
	src.CopyTo(dst)
	if !src.Equals(dst) {
		t.Error("non-nil copy: src and dst must be equal")
	}
	if src.GetBytesRef() == dst.GetBytesRef() {
		t.Error("non-nil copy: dst must own an independent BytesRef (deep copy)")
	}
	if !util.BytesRefEquals(src.GetBytesRef(), dst.GetBytesRef()) {
		t.Errorf("non-nil copy: bytes mismatch: src=%v dst=%v",
			src.GetBytesRef(), dst.GetBytesRef())
	}
}

// TestBytesRefAttImpl_ImplementsTermToBytesRef mirrors testLucene9856
// (Lucene 10.4.0). Verifies that BytesTermAttributeImpl explicitly
// implements TermToBytesRefAttribute — enforced here at compile time
// by the compile-time assertions in bytes_term_attribute_impl.go.
func TestBytesRefAttImpl_ImplementsTermToBytesRef(t *testing.T) {
	var _ TermToBytesRefAttribute = (*BytesTermAttributeImpl)(nil)
}
