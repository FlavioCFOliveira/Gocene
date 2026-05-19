// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubFieldTermIterator drives a deterministic sequence of (field, term,
// delGen) tuples and reuses the same string instance for each distinct
// field, mirroring the contract the Java abstract class imposes on its
// subclasses.
type stubFieldTermIterator struct {
	fields  []string // canonical interned string per tuple
	terms   [][]byte // term bytes per tuple
	delGens []int64  // del gen per tuple
	idx     int      // -1 before first Next, len(...) after exhaustion
	out     util.BytesRef
}

func newStubFieldTermIterator(fields []string, terms [][]byte, delGens []int64) *stubFieldTermIterator {
	return &stubFieldTermIterator{
		fields:  fields,
		terms:   terms,
		delGens: delGens,
		idx:     -1,
	}
}

func (s *stubFieldTermIterator) Next() (*util.BytesRef, error) {
	s.idx++
	if s.idx >= len(s.terms) {
		return nil, nil
	}
	s.out = util.BytesRef{Bytes: s.terms[s.idx], Length: len(s.terms[s.idx])}
	return &s.out, nil
}

func (s *stubFieldTermIterator) field() string { return s.fields[s.idx] }

func (s *stubFieldTermIterator) delGen() int64 { return s.delGens[s.idx] }

// Compile-time assertion that the stub satisfies the contract.
var _ fieldTermIterator = (*stubFieldTermIterator)(nil)

func TestFieldTermIterator_InterfaceContract(t *testing.T) {
	t.Parallel()

	fieldA := "body"
	fieldB := "title"

	stub := newStubFieldTermIterator(
		[]string{fieldA, fieldA, fieldB},
		[][]byte{[]byte("alpha"), []byte("beta"), []byte("gamma")},
		[]int64{0, 0, 7},
	)

	var it fieldTermIterator = stub

	type want struct {
		term   string
		field  string
		fieldP *string // pointer used to assert string-instance reuse via ==
		delGen int64
	}
	cases := []want{
		{term: "alpha", field: fieldA, fieldP: &fieldA, delGen: 0},
		{term: "beta", field: fieldA, fieldP: &fieldA, delGen: 0},
		{term: "gamma", field: fieldB, fieldP: &fieldB, delGen: 7},
	}

	for i, c := range cases {
		got, err := it.Next()
		if err != nil {
			t.Fatalf("Next #%d: unexpected error: %v", i, err)
		}
		if got == nil {
			t.Fatalf("Next #%d: unexpected nil BytesRef", i)
		}
		if string(got.Bytes[:got.Length]) != c.term {
			t.Errorf("Next #%d: term = %q, want %q", i, got.Bytes[:got.Length], c.term)
		}
		if it.field() != c.field {
			t.Errorf("Next #%d: field = %q, want %q", i, it.field(), c.field)
		}
		// The Java contract guarantees that consecutive terms in the same
		// field share the same string instance (==). The Go equivalent is
		// that the returned string equals the canonical value held by the
		// underlying iterator state.
		if got, want := it.field(), *c.fieldP; got != want {
			t.Errorf("Next #%d: field identity mismatch: got %q, want %q", i, got, want)
		}
		if it.delGen() != c.delGen {
			t.Errorf("Next #%d: delGen = %d, want %d", i, it.delGen(), c.delGen)
		}
	}

	end, err := it.Next()
	if err != nil {
		t.Fatalf("trailing Next: unexpected error: %v", err)
	}
	if end != nil {
		t.Fatalf("trailing Next: expected nil BytesRef, got %v", end)
	}
}

// TestFieldTermIterator_EmbedsBytesRefIterator pins the interface
// composition: any fieldTermIterator must also satisfy
// util.BytesRefIterator. If this assertion stops compiling, the Lucene
// contract has been broken.
func TestFieldTermIterator_EmbedsBytesRefIterator(t *testing.T) {
	t.Parallel()

	var it fieldTermIterator = newStubFieldTermIterator(nil, nil, nil)
	var _ util.BytesRefIterator = it
}
