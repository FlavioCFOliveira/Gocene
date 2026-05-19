// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPrefix8ToComparableUnsignedLong_FullLong verifies that an 8-byte term
// round-trips through the prefix encoder as the same big-endian unsigned
// value bit-for-bit.
func TestPrefix8ToComparableUnsignedLong_FullLong(t *testing.T) {
	in := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	got := Prefix8ToComparableUnsignedLong(util.NewBytesRef(in))
	want := binary.BigEndian.Uint64(in)
	if got != want {
		t.Fatalf("got %#x, want %#x", got, want)
	}
}

// TestPrefix8ToComparableUnsignedLong_LongerThan8 verifies that bytes past
// the first eight are ignored.
func TestPrefix8ToComparableUnsignedLong_LongerThan8(t *testing.T) {
	in := []byte{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66}
	got := Prefix8ToComparableUnsignedLong(util.NewBytesRef(in))
	want := binary.BigEndian.Uint64(in[:8])
	if got != want {
		t.Fatalf("got %#x, want %#x", got, want)
	}
}

// TestPrefix8ToComparableUnsignedLong_ShortTermsPadRight checks that all
// short-term branches (4..7 bytes) are right-padded with zeros so that a
// shorter prefix-sharing term sorts before a longer one — the contract the
// Lucene comment spells out.
func TestPrefix8ToComparableUnsignedLong_ShortTermsPadRight(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want uint64
	}{
		{"empty", []byte{}, 0x0},
		{"one", []byte{0xab}, 0xab00000000000000},
		{"two", []byte{0xab, 0xcd}, 0xabcd000000000000},
		{"three", []byte{0xab, 0xcd, 0xef}, 0xabcdef0000000000},
		{"four", []byte{0xde, 0xad, 0xbe, 0xef}, 0xdeadbeef00000000},
		{"five", []byte{0xde, 0xad, 0xbe, 0xef, 0x01}, 0xdeadbeef01000000},
		{"six", []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02}, 0xdeadbeef01020000},
		{"seven", []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02, 0x03}, 0xdeadbeef01020300},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Prefix8ToComparableUnsignedLong(util.NewBytesRef(tc.in))
			if got != tc.want {
				t.Fatalf("got %#x, want %#x", got, tc.want)
			}
		})
	}
}

// TestPrefix8ToComparableUnsignedLong_HonoursOffset ensures the helper only
// reads bytes inside [Offset, Offset+Length).
func TestPrefix8ToComparableUnsignedLong_HonoursOffset(t *testing.T) {
	backing := []byte{0x00, 0x00, 0xde, 0xad, 0xbe, 0xef}
	br := &util.BytesRef{Bytes: backing, Offset: 2, Length: 4}
	got := Prefix8ToComparableUnsignedLong(br)
	want := uint64(0xdeadbeef00000000)
	if got != want {
		t.Fatalf("got %#x, want %#x", got, want)
	}
}

// TestPrefix8ToComparableUnsignedLong_PreservesUnsignedOrder checks the
// invariant the cache depends on: BytesRef ordering must agree with the
// uint64 ordering whenever the cached prefixes differ.
func TestPrefix8ToComparableUnsignedLong_PreservesUnsignedOrder(t *testing.T) {
	cases := [][2][]byte{
		{{0x00}, {0x01}},
		{{0x7f, 0xff, 0xff, 0xff}, {0x80, 0x00, 0x00, 0x00}},
		{{0xfe}, {0xff}},
		{{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, {0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x00}},
	}
	for _, c := range cases {
		l1 := Prefix8ToComparableUnsignedLong(util.NewBytesRef(c[0]))
		l2 := Prefix8ToComparableUnsignedLong(util.NewBytesRef(c[1]))
		byteCmp := bytes.Compare(c[0], c[1])
		if byteCmp < 0 && !(l1 <= l2) {
			t.Fatalf("byte order says %x<%x but prefix says %#x>%#x", c[0], c[1], l1, l2)
		}
		if byteCmp > 0 && !(l1 >= l2) {
			t.Fatalf("byte order says %x>%x but prefix says %#x<%#x", c[0], c[1], l1, l2)
		}
	}
}

// tieFakeTermsEnum is a deterministic in-memory TermsEnum used to exercise
// TermsEnumIndex. It implements the full TermsEnum interface plus the
// optional ordinalSeeker contract.
type tieFakeTermsEnum struct {
	field string
	terms [][]byte
	pos   int  // -1 before first call, len(terms) after end
	live  bool // true once Next/Seek positioned the cursor
}

func newTieFakeTermsEnum(field string, terms ...string) *tieFakeTermsEnum {
	bs := make([][]byte, len(terms))
	for i, s := range terms {
		bs[i] = []byte(s)
	}
	return &tieFakeTermsEnum{field: field, terms: bs, pos: -1}
}

func (f *tieFakeTermsEnum) Next() (*Term, error) {
	f.pos++
	if f.pos >= len(f.terms) {
		f.live = false
		return nil, nil
	}
	f.live = true
	return NewTermFromBytes(f.field, f.terms[f.pos]), nil
}

func (f *tieFakeTermsEnum) SeekCeil(target *Term) (*Term, error) {
	tgt := target.GetBytesRef().ValidBytes()
	for i, b := range f.terms {
		if bytes.Compare(b, tgt) >= 0 {
			f.pos = i
			f.live = true
			return NewTermFromBytes(f.field, b), nil
		}
	}
	f.pos = len(f.terms)
	f.live = false
	return nil, nil
}

func (f *tieFakeTermsEnum) SeekExact(target *Term) (bool, error) {
	tgt := target.GetBytesRef().ValidBytes()
	for i, b := range f.terms {
		if bytes.Equal(b, tgt) {
			f.pos = i
			f.live = true
			return true, nil
		}
	}
	f.live = false
	return false, nil
}

func (f *tieFakeTermsEnum) SeekExactOrd(ord int64) error {
	if ord < 0 || int(ord) >= len(f.terms) {
		return errors.New("ord out of range")
	}
	f.pos = int(ord)
	f.live = true
	return nil
}

func (f *tieFakeTermsEnum) Term() *Term {
	if !f.live || f.pos < 0 || f.pos >= len(f.terms) {
		return nil
	}
	return NewTermFromBytes(f.field, f.terms[f.pos])
}

func (f *tieFakeTermsEnum) DocFreq() (int, error)              { return 0, nil }
func (f *tieFakeTermsEnum) TotalTermFreq() (int64, error)      { return 0, nil }
func (f *tieFakeTermsEnum) Postings(int) (PostingsEnum, error) { return &EmptyPostingsEnum{}, nil }
func (f *tieFakeTermsEnum) PostingsWithLiveDocs(util.Bits, int) (PostingsEnum, error) {
	return &EmptyPostingsEnum{}, nil
}

// plainTermsEnum is a TermsEnum that intentionally does not implement
// ordinalSeeker, so SeekExactOrd on the wrapper must fail.
type plainTermsEnum struct {
	field string
	cur   *Term
}

func (p *plainTermsEnum) Next() (*Term, error)               { return nil, nil }
func (p *plainTermsEnum) SeekCeil(*Term) (*Term, error)      { return nil, nil }
func (p *plainTermsEnum) SeekExact(*Term) (bool, error)      { return false, nil }
func (p *plainTermsEnum) Term() *Term                        { return p.cur }
func (p *plainTermsEnum) DocFreq() (int, error)              { return 0, nil }
func (p *plainTermsEnum) TotalTermFreq() (int64, error)      { return 0, nil }
func (p *plainTermsEnum) Postings(int) (PostingsEnum, error) { return &EmptyPostingsEnum{}, nil }
func (p *plainTermsEnum) PostingsWithLiveDocs(util.Bits, int) (PostingsEnum, error) {
	return &EmptyPostingsEnum{}, nil
}

// TestTermsEnumIndex_NextWalksAndUpdatesCache walks the wrapped enumerator
// to exhaustion and verifies Term, prefix cache, and end-of-stream semantics.
func TestTermsEnumIndex_NextWalksAndUpdatesCache(t *testing.T) {
	te := newTieFakeTermsEnum("f", "alpha", "bravo", "charlie")
	idx := NewTermsEnumIndex(te, 7)

	if idx.SubIndex != 7 {
		t.Fatalf("SubIndex: got %d, want 7", idx.SubIndex)
	}
	if idx.Term() != nil {
		t.Fatal("Term must be nil before first Next")
	}

	want := []string{"alpha", "bravo", "charlie"}
	for i, w := range want {
		got, err := idx.Next()
		if err != nil {
			t.Fatalf("Next #%d: %v", i, err)
		}
		if got == nil {
			t.Fatalf("Next #%d returned nil", i)
		}
		if string(got.ValidBytes()) != w {
			t.Fatalf("Next #%d: got %q, want %q", i, got.ValidBytes(), w)
		}
		if idx.currentTermPrefix8 != Prefix8ToComparableUnsignedLong(got) {
			t.Fatalf("cache mismatch at %q", w)
		}
	}

	end, err := idx.Next()
	if err != nil {
		t.Fatalf("Next at end: %v", err)
	}
	if end != nil {
		t.Fatal("Next past last must return nil")
	}
	if idx.Term() != nil {
		t.Fatal("Term must be nil after end-of-stream")
	}
	if idx.currentTermPrefix8 != 0 {
		t.Fatalf("prefix cache must reset to 0, got %#x", idx.currentTermPrefix8)
	}
}

// TestTermsEnumIndex_SeekCeilStatuses covers all three SeekStatus branches.
func TestTermsEnumIndex_SeekCeilStatuses(t *testing.T) {
	te := newTieFakeTermsEnum("f", "apple", "kiwi", "orange")
	idx := NewTermsEnumIndex(te, 0)

	status, err := idx.SeekCeil(util.NewBytesRef([]byte("kiwi")))
	if err != nil {
		t.Fatalf("SeekCeil exact: %v", err)
	}
	if status != SeekStatusFound {
		t.Fatalf("SeekCeil exact: got %v, want FOUND", status)
	}
	if string(idx.Term().ValidBytes()) != "kiwi" {
		t.Fatalf("Term after exact ceil: %q", idx.Term().ValidBytes())
	}

	status, err = idx.SeekCeil(util.NewBytesRef([]byte("ban")))
	if err != nil {
		t.Fatalf("SeekCeil notfound: %v", err)
	}
	if status != SeekStatusNotFound {
		t.Fatalf("SeekCeil notfound: got %v, want NOT_FOUND", status)
	}
	if string(idx.Term().ValidBytes()) != "kiwi" {
		t.Fatalf("Term after ceil-to-next: %q", idx.Term().ValidBytes())
	}

	status, err = idx.SeekCeil(util.NewBytesRef([]byte("zebra")))
	if err != nil {
		t.Fatalf("SeekCeil end: %v", err)
	}
	if status != SeekStatusEnd {
		t.Fatalf("SeekCeil end: got %v, want END", status)
	}
	if idx.Term() != nil {
		t.Fatalf("Term after END must be nil, got %v", idx.Term())
	}
}

// TestTermsEnumIndex_SeekExact covers the hit / miss paths.
func TestTermsEnumIndex_SeekExact(t *testing.T) {
	te := newTieFakeTermsEnum("f", "alpha", "beta", "gamma")
	idx := NewTermsEnumIndex(te, 0)

	found, err := idx.SeekExact(util.NewBytesRef([]byte("beta")))
	if err != nil {
		t.Fatalf("SeekExact hit: %v", err)
	}
	if !found {
		t.Fatal("SeekExact: hit expected")
	}
	if string(idx.Term().ValidBytes()) != "beta" {
		t.Fatalf("Term: %q", idx.Term().ValidBytes())
	}

	found, err = idx.SeekExact(util.NewBytesRef([]byte("delta")))
	if err != nil {
		t.Fatalf("SeekExact miss: %v", err)
	}
	if found {
		t.Fatal("SeekExact: miss expected")
	}
	if idx.Term() != nil {
		t.Fatalf("Term after miss must be nil, got %v", idx.Term())
	}
}

// TestTermsEnumIndex_SeekExactOrd_Supported uses the fake enumerator which
// implements ordinalSeeker.
func TestTermsEnumIndex_SeekExactOrd_Supported(t *testing.T) {
	te := newTieFakeTermsEnum("f", "a", "b", "c", "d")
	idx := NewTermsEnumIndex(te, 0)

	if err := idx.SeekExactOrd(2); err != nil {
		t.Fatalf("SeekExactOrd: %v", err)
	}
	if string(idx.Term().ValidBytes()) != "c" {
		t.Fatalf("Term: %q", idx.Term().ValidBytes())
	}
}

// TestTermsEnumIndex_SeekExactOrd_Unsupported verifies that an enumerator
// without ordinalSeeker support yields the documented sentinel.
func TestTermsEnumIndex_SeekExactOrd_Unsupported(t *testing.T) {
	idx := NewTermsEnumIndex(&plainTermsEnum{field: "f"}, 0)
	err := idx.SeekExactOrd(0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errOrdinalSeekUnsupported) {
		t.Fatalf("got %v, want errOrdinalSeekUnsupported", err)
	}
}

// TestTermsEnumIndex_CompareTermTo_FastAndSlowPaths exercises both the
// cached-prefix shortcut and the byte-by-byte fallback.
func TestTermsEnumIndex_CompareTermTo_FastAndSlowPaths(t *testing.T) {
	a := NewTermsEnumIndex(newTieFakeTermsEnum("f", "alpha"), 0)
	b := NewTermsEnumIndex(newTieFakeTermsEnum("f", "bravo"), 1)
	if _, err := a.Next(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Next(); err != nil {
		t.Fatal(err)
	}
	if a.CompareTermTo(b) >= 0 {
		t.Fatal("alpha must sort before bravo")
	}
	if b.CompareTermTo(a) <= 0 {
		t.Fatal("bravo must sort after alpha")
	}

	// Slow path: identical 8-byte prefixes, different tail.
	long1 := NewTermsEnumIndex(newTieFakeTermsEnum("f", "abcdefghAAAA"), 0)
	long2 := NewTermsEnumIndex(newTieFakeTermsEnum("f", "abcdefghBBBB"), 1)
	if _, err := long1.Next(); err != nil {
		t.Fatal(err)
	}
	if _, err := long2.Next(); err != nil {
		t.Fatal(err)
	}
	if long1.currentTermPrefix8 != long2.currentTermPrefix8 {
		t.Fatal("setup error: prefixes should match to exercise slow path")
	}
	if long1.CompareTermTo(long2) >= 0 {
		t.Fatal("AAAA tail must sort before BBBB tail")
	}
}

// TestTermsEnumIndex_Reset propagates state from one wrapper into another.
func TestTermsEnumIndex_Reset(t *testing.T) {
	src := NewTermsEnumIndex(newTieFakeTermsEnum("f", "x", "y"), 5)
	if _, err := src.Next(); err != nil {
		t.Fatal(err)
	}

	dst := NewTermsEnumIndex(newTieFakeTermsEnum("g", "u"), 9)
	dst.Reset(src)

	if dst.TermsEnum != src.TermsEnum {
		t.Fatal("Reset must copy TermsEnum reference")
	}
	if !bytes.Equal(dst.Term().ValidBytes(), src.Term().ValidBytes()) {
		t.Fatal("Reset must copy current term")
	}
	if dst.currentTermPrefix8 != src.currentTermPrefix8 {
		t.Fatal("Reset must copy cached prefix")
	}
}

// TestTermsEnumIndex_TermStateEquality covers the snapshot + termEquals
// fast/slow paths.
func TestTermsEnumIndex_TermStateEquality(t *testing.T) {
	idx := NewTermsEnumIndex(newTieFakeTermsEnum("f", "hello"), 0)
	if _, err := idx.Next(); err != nil {
		t.Fatal(err)
	}

	state := NewTermsEnumTermState()
	state.CopyFrom(idx)

	if !idx.TermEquals(state) {
		t.Fatal("self-equality must hold")
	}

	other := NewTermsEnumIndex(newTieFakeTermsEnum("f", "world"), 0)
	if _, err := other.Next(); err != nil {
		t.Fatal(err)
	}
	if other.TermEquals(state) {
		t.Fatal("different term must not match state")
	}
}

// TestEmptyTermsEnumIndexArray is a tiny sanity check on the canonical
// empty slice.
func TestEmptyTermsEnumIndexArray(t *testing.T) {
	if EmptyTermsEnumIndexArray == nil {
		t.Fatal("must be non-nil empty slice")
	}
	if len(EmptyTermsEnumIndexArray) != 0 {
		t.Fatalf("len: got %d, want 0", len(EmptyTermsEnumIndexArray))
	}
}
