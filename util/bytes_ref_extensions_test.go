// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestEmptyBytesIsZeroLength(t *testing.T) {
	if len(EmptyBytes) != 0 {
		t.Errorf("EmptyBytes should be empty, got len %d", len(EmptyBytes))
	}
}

// TestWrapBytes_ZeroCopy verifies WrapBytes does not copy the input.
// Mutating the input must be visible through the returned BytesRef.
func TestWrapBytes_ZeroCopy(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	br := WrapBytes(data)
	if br.Length != 4 {
		t.Errorf("Length = %d, want 4", br.Length)
	}
	data[0] = 99
	if br.Bytes[0] != 99 {
		t.Error("WrapBytes should reference the input slice without copying")
	}
}

// TestNewBytesRefRange_ZeroCopy mirrors the Java constructor
// {@code BytesRef(byte[], int, int)}.
func TestNewBytesRefRange_ZeroCopy(t *testing.T) {
	data := []byte("xxhellox")
	br := NewBytesRefRange(data, 2, 5)
	if got := br.Utf8ToString(); got != "hello" {
		t.Errorf("Utf8ToString = %q, want %q", got, "hello")
	}
	data[2] = 'H'
	if br.Bytes[2] != 'H' {
		t.Error("NewBytesRefRange should reference the input slice without copying")
	}
}

// TestBytesRefFromString_RoundTrip verifies the CharSequence-style
// constructor.
func TestBytesRefFromString_RoundTrip(t *testing.T) {
	br := BytesRefFromString("hello")
	if got := br.Utf8ToString(); got != "hello" {
		t.Errorf("Utf8ToString = %q, want %q", got, "hello")
	}
	// Multibyte UTF-8.
	br = BytesRefFromString("héllo")
	if got := br.Utf8ToString(); got != "héllo" {
		t.Errorf("Utf8ToString = %q, want %q", got, "héllo")
	}
}

// TestShallowClone_SharesBytes confirms the Lucene-matching shallow
// semantics: the underlying slice is shared.
func TestShallowClone_SharesBytes(t *testing.T) {
	src := NewBytesRef([]byte("hello"))
	clone := src.ShallowClone()
	if &clone.Bytes[0] != &src.Bytes[0] {
		t.Error("ShallowClone must share the underlying slice")
	}
	src.Bytes[0] = 'H'
	if clone.Bytes[0] != 'H' {
		t.Error("mutation through src should be visible to ShallowClone (shared bytes)")
	}
}

// TestBytesRefDeepCopyOf_Independent confirms the static deep copy
// helper returns a BytesRef with its own backing slice.
func TestBytesRefDeepCopyOf_Independent(t *testing.T) {
	src := NewBytesRef([]byte("hello"))
	dst := BytesRefDeepCopyOf(src)
	if &dst.Bytes[0] == &src.Bytes[0] {
		t.Error("BytesRefDeepCopyOf must allocate a fresh slice")
	}
	src.Bytes[0] = 'H'
	if dst.Bytes[0] != 'h' {
		t.Errorf("dst.Bytes[0] = %c, want unchanged 'h'", dst.Bytes[0])
	}
	if dst.Offset != 0 || dst.Length != 5 {
		t.Errorf("dst offset/length = (%d,%d), want (0,5)", dst.Offset, dst.Length)
	}
}

// TestUtf8ToString verifies UTF-8 decoding mirrors Java behaviour on
// well-formed input.
func TestUtf8ToString(t *testing.T) {
	br := NewBytesRef([]byte("café"))
	if got := br.Utf8ToString(); got != "café" {
		t.Errorf("Utf8ToString = %q, want %q", got, "café")
	}
}

// TestToHexString_MatchesLuceneFormat verifies the
// "[6c 75 63 65 6e 65]" Lucene toString format.
func TestToHexString_MatchesLuceneFormat(t *testing.T) {
	br := NewBytesRef([]byte("lucene"))
	want := "[6c 75 63 65 6e 65]"
	if got := br.ToHexString(); got != want {
		t.Errorf("ToHexString = %q, want %q", got, want)
	}
	// Empty.
	empty := NewBytesRefEmpty()
	if got := empty.ToHexString(); got != "[]" {
		t.Errorf("empty ToHexString = %q, want %q", got, "[]")
	}
}

// TestHashCode_UsesMurmurHash3 verifies the hash is now derived from
// MurmurHash3 (not the old String#hashCode style). The exact value
// depends on GoodFastHashSeed; the contract is that equal inputs have
// equal hashes and different inputs have different hashes.
func TestHashCode_UsesMurmurHash3(t *testing.T) {
	a := NewBytesRef([]byte("hello"))
	b := NewBytesRef([]byte("hello"))
	if a.HashCode() != b.HashCode() {
		t.Error("equal content must have equal hash codes")
	}
	c := NewBytesRef([]byte("world"))
	if a.HashCode() == c.HashCode() {
		t.Error("different content should have different hash codes")
	}
	// The pre-port "String#hashCode" of "hello" was 99162322; under
	// MurmurHash3 it differs. Just confirm we are no longer using the
	// trivial formula by checking the value is not the Java string
	// hash code computed via 31*acc+byte.
	javaStyleHash := 0
	for _, b := range []byte("hello") {
		javaStyleHash = 31*javaStyleHash + int(b)
	}
	if a.HashCode() == javaStyleHash {
		t.Error("HashCode should now use MurmurHash3, not the old Java string hash formula")
	}
}

// TestBytesEqualsRange exercises the BytesRef-rooted alias for
// [BytesRefEquals].
func TestBytesEqualsRange(t *testing.T) {
	a := NewBytesRef([]byte("xyz"))
	b := NewBytesRef([]byte("xyz"))
	if !a.BytesEqualsRange(b) {
		t.Error("equal content should compare equal")
	}
	c := NewBytesRef([]byte("xy"))
	if a.BytesEqualsRange(c) {
		t.Error("different lengths must not compare equal")
	}
}
