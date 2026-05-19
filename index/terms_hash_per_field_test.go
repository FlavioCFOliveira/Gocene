// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// wirePerField wires a freshly-allocated TermsHashPerField with hooks that
// produce a ParallelPostingsArray big enough to hold the standard side
// arrays. The returned slice freqs[termID] is updated by the AddTerm hook
// so tests can assert that the chain dispatched correctly.
func wirePerField(t *testing.T, streamCount int, opts IndexOptions, next *TermsHashPerField) (*TermsHashPerField, *[]int) {
	t.Helper()
	intPool := util.NewIntBlockPool()
	bytePool := util.NewByteBlockPool(util.NewDirectTrackingAllocator(util.NewCounter()))
	bytePool.NextBuffer()
	termPool := util.NewByteBlockPool(util.NewDirectTrackingAllocator(util.NewCounter()))
	termPool.NextBuffer()

	freqs := []int{}
	var field *TermsHashPerField
	hooks := TermsHashPerFieldHooks{
		CreatePostingsArray: func(size int) *ParallelPostingsArray {
			return NewParallelPostingsArray(size)
		},
		NewPostingsArray: func() {
			if field == nil || field.PostingsArray == nil {
				return
			}
			if len(freqs) < field.PostingsArray.Size {
				grown := make([]int, field.PostingsArray.Size)
				copy(grown, freqs)
				freqs = grown
			}
		},
		NewTerm: func(termID, docID int) error {
			if termID >= len(freqs) {
				grown := make([]int, termID+1)
				copy(grown, freqs)
				freqs = grown
			}
			freqs[termID] = 1
			field.WriteStreamVInt(0, int32(docID))
			return nil
		},
		AddTerm: func(termID, docID int) error {
			freqs[termID]++
			field.WriteStreamVInt(0, int32(docID))
			return nil
		},
	}
	var err error
	field, err = NewTermsHashPerField(streamCount, intPool, bytePool, termPool, util.NewCounter(), next, "f", opts, hooks)
	if err != nil {
		t.Fatalf("NewTermsHashPerField: %v", err)
	}
	return field, &freqs
}

func TestParallelPostingsArray_GrowAndCopy(t *testing.T) {
	a := NewParallelPostingsArray(4)
	for i := range a.TextStarts {
		a.TextStarts[i] = 10 + i
		a.AddressOffset[i] = 100 + i
		a.ByteStarts[i] = 1000 + i
	}
	b := NewParallelPostingsArray(8)
	a.CopyTo(b, a.Size)
	for i := 0; i < a.Size; i++ {
		if b.TextStarts[i] != a.TextStarts[i] || b.AddressOffset[i] != a.AddressOffset[i] || b.ByteStarts[i] != a.ByteStarts[i] {
			t.Fatalf("CopyTo mismatch at %d", i)
		}
	}
	for i := a.Size; i < b.Size; i++ {
		if b.TextStarts[i] != 0 || b.AddressOffset[i] != 0 || b.ByteStarts[i] != 0 {
			t.Fatalf("CopyTo leaked data past numToCopy at %d", i)
		}
	}
	if got := a.BytesPerPosting(); got != 12 {
		t.Fatalf("BytesPerPosting = %d, want 12", got)
	}
}

func TestNewTermsHashPerField_RejectsNoneAndNilPools(t *testing.T) {
	pool := util.NewByteBlockPool(util.NewDirectTrackingAllocator(util.NewCounter()))
	hooks := TermsHashPerFieldHooks{
		NewTerm:             func(int, int) error { return nil },
		AddTerm:             func(int, int) error { return nil },
		CreatePostingsArray: func(size int) *ParallelPostingsArray { return NewParallelPostingsArray(size) },
	}
	if _, err := NewTermsHashPerField(1, util.NewIntBlockPool(), pool, pool, util.NewCounter(), nil, "f", IndexOptionsNone, hooks); err == nil {
		t.Fatal("expected error for IndexOptionsNone")
	}
	if _, err := NewTermsHashPerField(1, nil, pool, pool, util.NewCounter(), nil, "f", IndexOptionsDocs, hooks); err == nil {
		t.Fatal("expected error for nil intPool")
	}
	if _, err := NewTermsHashPerField(1, util.NewIntBlockPool(), nil, pool, util.NewCounter(), nil, "f", IndexOptionsDocs, hooks); err == nil {
		t.Fatal("expected error for nil bytePool")
	}
	if _, err := NewTermsHashPerField(1, util.NewIntBlockPool(), pool, pool, util.NewCounter(), nil, "f", IndexOptionsDocs, TermsHashPerFieldHooks{}); err == nil {
		t.Fatal("expected error for missing hooks")
	}
}

func TestTermsHashPerField_AddNewAndRepeatedTerms(t *testing.T) {
	field, freqs := wirePerField(t, 1, IndexOptionsDocs, nil)

	terms := [][]byte{[]byte("apple"), []byte("banana"), []byte("apple"), []byte("cherry"), []byte("banana"), []byte("apple")}
	for i, term := range terms {
		if err := field.Add(util.NewBytesRef(term), i); err != nil {
			t.Fatalf("Add[%d]=%q: %v", i, term, err)
		}
	}
	if got, want := field.GetNumTerms(), 3; got != want {
		t.Fatalf("GetNumTerms = %d, want %d", got, want)
	}

	wantFreq := map[string]int{"apple": 3, "banana": 2, "cherry": 1}
	scratch := &util.BytesRef{}
	for id := 0; id < field.GetNumTerms(); id++ {
		field.bytesHash.Get(id, scratch)
		text := string(scratch.ValidBytes())
		if (*freqs)[id] != wantFreq[text] {
			t.Fatalf("freq[%s] = %d, want %d", text, (*freqs)[id], wantFreq[text])
		}
	}
}

func TestTermsHashPerField_AssertDocIDMonotonic(t *testing.T) {
	field, _ := wirePerField(t, 1, IndexOptionsDocs, nil)
	if err := field.Add(util.NewBytesRef([]byte("x")), 5); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := field.Add(util.NewBytesRef([]byte("y")), 4); err == nil {
		t.Fatal("expected out-of-order docID to be rejected")
	}
}

func TestTermsHashPerField_StreamRoundTrip(t *testing.T) {
	field, _ := wirePerField(t, 1, IndexOptionsDocs, nil)
	wantDocs := map[string][]int32{}
	rng := rand.New(rand.NewPCG(1, 2))
	lastDoc := 0
	for i := 0; i < 256; i++ {
		term := []byte("t" + strconv.Itoa(rng.IntN(16)))
		doc := lastDoc + rng.IntN(8)
		if err := field.Add(util.NewBytesRef(term), doc); err != nil {
			t.Fatalf("Add: %v", err)
		}
		wantDocs[string(term)] = append(wantDocs[string(term)], int32(doc))
		lastDoc = doc
	}

	scratch := &util.BytesRef{}
	for id := 0; id < field.GetNumTerms(); id++ {
		field.bytesHash.Get(id, scratch)
		text := string(scratch.ValidBytes())
		var r ByteSliceReader
		if err := field.InitReader(&r, id, 0); err != nil {
			t.Fatalf("InitReader[%s]: %v", text, err)
		}
		got := make([]int32, 0, len(wantDocs[text]))
		for !r.EOF() {
			v, err := readVInt32(&r)
			if err != nil {
				t.Fatalf("read[%s]: %v", text, err)
			}
			got = append(got, v)
		}
		if len(got) != len(wantDocs[text]) {
			t.Fatalf("term %q: got %d docs, want %d", text, len(got), len(wantDocs[text]))
		}
		for i := range got {
			if got[i] != wantDocs[text][i] {
				t.Fatalf("term %q docs[%d] = %d, want %d", text, i, got[i], wantDocs[text][i])
			}
		}
	}
}

func TestTermsHashPerField_WriteBytesSpansSlices(t *testing.T) {
	field, _ := wirePerField(t, 1, IndexOptionsDocs, nil)
	// Force a fresh term so we own a stream and can write directly.
	if err := field.Add(util.NewBytesRef([]byte("spans")), 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	payload := bytes.Repeat([]byte{0x42}, 4096)
	field.WriteStreamBytes(0, payload, 0, len(payload))

	var r ByteSliceReader
	if err := field.InitReader(&r, 0, 0); err != nil {
		t.Fatalf("InitReader: %v", err)
	}
	// First byte is the vint(0) written by NewTerm.
	if _, err := readVInt32(&r); err != nil {
		t.Fatalf("read docID prefix: %v", err)
	}
	got := make([]byte, len(payload))
	if err := r.ReadBytes(got); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %x... want %x...", got[:16], payload[:16])
	}
}

func TestTermsHashPerField_ChainForwardsToNextField(t *testing.T) {
	next, nextFreqs := wirePerField(t, 1, IndexOptionsDocs, nil)
	primary, _ := wirePerField(t, 1, IndexOptionsDocs, next)
	// Force Start so doNextCall becomes true.
	primary.Start(stubField{}, true)

	for i, term := range [][]byte{[]byte("a"), []byte("b"), []byte("a")} {
		if err := primary.Add(util.NewBytesRef(term), i); err != nil {
			t.Fatalf("primary.Add: %v", err)
		}
	}
	if next.GetNumTerms() != 2 {
		t.Fatalf("next.GetNumTerms = %d, want 2", next.GetNumTerms())
	}
	totalFreq := 0
	for _, f := range *nextFreqs {
		totalFreq += f
	}
	if totalFreq != 3 {
		t.Fatalf("forwarded calls = %d, want 3", totalFreq)
	}
}

func TestTermsHashPerField_SortReinitAndReset(t *testing.T) {
	field, _ := wirePerField(t, 1, IndexOptionsDocs, nil)
	for i, term := range [][]byte{[]byte("c"), []byte("a"), []byte("b")} {
		if err := field.Add(util.NewBytesRef(term), i); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	field.SortTerms()
	sorted := field.GetSortedTermIDs()
	scratch := &util.BytesRef{}
	prev := ""
	for _, id := range sorted[:field.GetNumTerms()] {
		field.bytesHash.Get(id, scratch)
		cur := string(scratch.ValidBytes())
		if prev != "" && cur < prev {
			t.Fatalf("sorted order violated: %q before %q", prev, cur)
		}
		prev = cur
	}

	// SortTerms must reject a second call without ReinitHash/Reset.
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected SortTerms to panic on second call")
			}
		}()
		field.SortTerms()
	}()

	field.ReinitHash()
	field.SortTerms() // must not panic after reinit
	field.Reset()
	if field.GetNumTerms() != 0 {
		t.Fatalf("Reset: GetNumTerms = %d, want 0", field.GetNumTerms())
	}
}

func TestTermsHashPerField_CompareToAndFieldName(t *testing.T) {
	a, _ := wirePerField(t, 1, IndexOptionsDocs, nil)
	b, _ := wirePerField(t, 1, IndexOptionsDocs, nil)
	// Both fields have "f" by default; mutate to differentiate.
	a.fieldName = "alpha"
	b.fieldName = "beta"
	if a.CompareTo(b) >= 0 {
		t.Fatalf("CompareTo(alpha, beta) = %d, want < 0", a.CompareTo(b))
	}
	if b.CompareTo(a) <= 0 {
		t.Fatalf("CompareTo(beta, alpha) = %d, want > 0", b.CompareTo(a))
	}
	if a.GetFieldName() != "alpha" {
		t.Fatalf("GetFieldName = %q, want %q", a.GetFieldName(), "alpha")
	}
}

func TestTermsHashPerField_FinishPropagates(t *testing.T) {
	next, _ := wirePerField(t, 1, IndexOptionsDocs, nil)
	primary, _ := wirePerField(t, 1, IndexOptionsDocs, next)
	if err := primary.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

func TestTermsHashPerField_PropagatesHookError(t *testing.T) {
	field, _ := wirePerField(t, 1, IndexOptionsDocs, nil)
	sentinel := errors.New("newterm boom")
	field.NewTerm = func(int, int) error { return sentinel }
	err := field.Add(util.NewBytesRef([]byte("z")), 0)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Add error = %v, want %v", err, sentinel)
	}
}

// readVInt32 decodes a vint from r matching the encoding written by WriteStreamVInt.
func readVInt32(r *ByteSliceReader) (int32, error) {
	var result int32
	shift := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return 0, err
			}
			return 0, fmt.Errorf("readVInt32: %w", err)
		}
		result |= int32(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("readVInt32: too many bytes")
		}
	}
}

// stubField is a minimal IndexableField for Start().
type stubField struct{}

func (stubField) Name() string                  { return "f" }
func (stubField) FieldType() FieldTypeInterface { return nil }
func (stubField) StringValue() string           { return "" }
func (stubField) BinaryValue() []byte           { return nil }
func (stubField) NumericValue() interface{}     { return nil }
