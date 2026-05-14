// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"testing"
)

func TestEmptyBytesRefIterator(t *testing.T) {
	t.Parallel()

	ref, err := EmptyBytesRefIterator.Next()
	if err != nil {
		t.Fatalf("EMPTY.Next() returned error: %v", err)
	}
	if ref != nil {
		t.Fatalf("EMPTY.Next() returned non-nil ref %v", ref)
	}
}

func TestBytesRefIteratorFunc_Adapter(t *testing.T) {
	t.Parallel()

	values := [][]byte{[]byte("a"), []byte("bb"), []byte("ccc")}
	i := 0
	iter := BytesRefIteratorFunc(func() (*BytesRef, error) {
		if i >= len(values) {
			return nil, nil
		}
		v := values[i]
		i++
		return &BytesRef{Bytes: v, Offset: 0, Length: len(v)}, nil
	})

	var got []string
	for {
		ref, err := iter.Next()
		if err != nil {
			t.Fatalf("Next() error: %v", err)
		}
		if ref == nil {
			break
		}
		got = append(got, string(ref.ValidBytes()))
	}
	if len(got) != 3 || got[0] != "a" || got[1] != "bb" || got[2] != "ccc" {
		t.Fatalf("collected %v, want [a bb ccc]", got)
	}
}

func TestBytesRefIterator_PropagatesError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("io: oops")
	iter := BytesRefIteratorFunc(func() (*BytesRef, error) {
		return nil, sentinel
	})
	ref, err := iter.Next()
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if ref != nil {
		t.Fatalf("expected nil ref on error, got %v", ref)
	}
}

// Static type check: BytesRefIteratorFunc satisfies BytesRefIterator.
var _ BytesRefIterator = BytesRefIteratorFunc(nil)
