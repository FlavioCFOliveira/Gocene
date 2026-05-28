// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// lengthPrefixCollator is a deterministic test Collator that prefixes the term
// with its byte length. It is a faithful, total implementation of the
// one-method Collator contract and yields easily-asserted key bytes.
type lengthPrefixCollator struct{}

func (lengthPrefixCollator) CollationKey(s string) []byte {
	return append([]byte{byte(len(s))}, s...)
}

var _ Collator = lengthPrefixCollator{}

// TestGetBytesRefUsesCollator verifies the attribute overrides GetBytesRef to
// emit the collation key of the current term rather than its raw UTF-8.
func TestGetBytesRefUsesCollator(t *testing.T) {
	t.Parallel()

	attr := NewCollatedTermAttributeImpl(lengthPrefixCollator{})
	attr.SetEmpty()
	attr.AppendString("abc")

	ref := attr.GetBytesRef()
	if ref == nil {
		t.Fatal("GetBytesRef() = nil")
	}
	want := []byte{3, 'a', 'b', 'c'} // length prefix 3 + "abc"
	if !bytes.Equal(ref.Bytes, want) {
		t.Errorf("GetBytesRef().Bytes = % x, want % x", ref.Bytes, want)
	}
	if ref.Offset != 0 || ref.Length != len(want) {
		t.Errorf("GetBytesRef() offset/len = (%d,%d), want (0,%d)", ref.Offset, ref.Length, len(want))
	}
}

// TestGetBytesRefTracksCurrentTerm verifies the key reflects whatever term is
// currently set, so the same attribute can be reused across tokens.
func TestGetBytesRefTracksCurrentTerm(t *testing.T) {
	t.Parallel()

	attr := NewCollatedTermAttributeImpl(lengthPrefixCollator{})

	attr.SetEmpty()
	attr.AppendString("x")
	if got := attr.GetBytesRef().Bytes; !bytes.Equal(got, []byte{1, 'x'}) {
		t.Errorf("first term key = % x, want 01 78", got)
	}

	attr.SetEmpty()
	attr.AppendString("yy")
	if got := attr.GetBytesRef().Bytes; !bytes.Equal(got, []byte{2, 'y', 'y'}) {
		t.Errorf("second term key = % x, want 02 79 79", got)
	}
}

// TestAttributeInterfacesDelegates verifies the impl advertises the same
// attribute interfaces as the embedded CharTermAttributeImpl, so attribute
// lookup treats it as a CharTermAttribute / TermToBytesRefAttribute.
func TestAttributeInterfacesDelegates(t *testing.T) {
	t.Parallel()

	attr := NewCollatedTermAttributeImpl(lengthPrefixCollator{})
	got := attr.AttributeInterfaces()
	want := analysis.NewCharTermAttributeImpl().AttributeInterfaces()

	if len(got) != len(want) {
		t.Fatalf("AttributeInterfaces() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AttributeInterfaces()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
