// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func newTestLeaf(t *testing.T, name string, docs int, fieldNames ...string) *LeafReader {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	si := NewSegmentInfo(name, docs, dir)
	fi := NewFieldInfos()
	for idx, fn := range fieldNames {
		field := NewFieldInfo(fn, idx, DefaultFieldInfoOptions())
		if err := fi.Add(field); err != nil {
			t.Fatal(err)
		}
	}
	return NewLeafReaderWithFieldInfos(si, fi)
}

func TestParallelLeafReader_FieldDispatch(t *testing.T) {
	a := newTestLeaf(t, "_a", 5, "title", "body")
	b := newTestLeaf(t, "_b", 5, "rank")
	p, err := NewParallelLeafReader(a, b)
	if err != nil {
		t.Fatalf("NewParallelLeafReader: %v", err)
	}
	if len(p.GetParallelReaders()) != 2 {
		t.Errorf("GetParallelReaders: got %d", len(p.GetParallelReaders()))
	}
	if r := p.readerFor("title"); r != a {
		t.Errorf("readerFor(title) = %v, want a", r)
	}
	if r := p.readerFor("rank"); r != b {
		t.Errorf("readerFor(rank) = %v, want b", r)
	}
	if r := p.readerFor("missing"); r != nil {
		t.Errorf("readerFor(missing) = %v, want nil", r)
	}
}

func TestParallelLeafReader_DuplicateFieldRejected(t *testing.T) {
	a := newTestLeaf(t, "_a", 5, "f1")
	b := newTestLeaf(t, "_b", 5, "f1")
	if _, err := NewParallelLeafReader(a, b); err == nil {
		t.Errorf("expected duplicate-field error")
	}
}

func TestParallelLeafReader_MaxDocMismatchRejected(t *testing.T) {
	a := newTestLeaf(t, "_a", 5)
	b := newTestLeaf(t, "_b", 6)
	if _, err := NewParallelLeafReader(a, b); err == nil {
		t.Errorf("expected MaxDoc-mismatch error")
	}
}

func TestParallelLeafReader_NoReadersRejected(t *testing.T) {
	if _, err := NewParallelLeafReader(); err == nil {
		t.Errorf("expected error for empty reader list")
	}
}

func TestParallelLeafReader_DoCloseIdempotent(t *testing.T) {
	a := newTestLeaf(t, "_a", 1, "f")
	p, err := NewParallelLeafReaderWithClose(false, a)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.DoClose(); err != nil {
		t.Errorf("first DoClose: %v", err)
	}
	if err := p.DoClose(); err != nil {
		t.Errorf("second DoClose: %v", err)
	}
}
