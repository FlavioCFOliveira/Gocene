// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/TestFilterDirectory.java
// (Lucene 10.4.0 has no dedicated TestFilterIndexInput.java; the unwrap
// behaviour is exercised through TestFilterDirectory and indirectly via tests
// that instrument I/O. The cases below cover the documented contract:
// constructor, delegate forwarding, unwrap.)

package store

import "testing"

func TestFilterIndexInput_DelegateForwarding(t *testing.T) {
	base := &fakeIndexInput{
		BaseIndexInput: NewBaseIndexInput("base", 4),
		data:           []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	filter := NewFilterIndexInput("filter", base)
	got, err := filter.ReadByte()
	if err != nil || got != 0xDE {
		t.Fatalf("ReadByte = (%#x, %v)", got, err)
	}
	if filter.GetFilePointer() != 1 {
		t.Fatalf("GetFilePointer = %d, want 1", filter.GetFilePointer())
	}
	if filter.Length() != 4 {
		t.Fatalf("Length = %d, want 4", filter.Length())
	}
	if filter.GetDelegate() != base {
		t.Fatalf("GetDelegate did not return the wrapped input")
	}
}

func TestFilterIndexInput_UnwrapAll(t *testing.T) {
	base := &fakeIndexInput{
		BaseIndexInput: NewBaseIndexInput("base", 0),
	}
	f1 := NewFilterIndexInput("f1", base)
	f2 := NewFilterIndexInput("f2", f1)
	f3 := NewFilterIndexInput("f3", f2)
	if got := UnwrapFilterIndexInput(f3); got != base {
		t.Fatalf("UnwrapFilterIndexInput did not reach the base delegate")
	}
}
