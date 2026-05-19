// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TestStringsToAutomaton from
// Apache Lucene 10.4.0. Tests requiring MinimizationOperations.minimize
// (testRandomMinimized, checkMinimized) are deferred until that primitive
// is ported; coverage here matches testBasic, testBasicBinary,
// testLargeTerms, testRandomUnicodeOnly, and testRandomBinary using
// per-term acceptance plus iterator-side enumeration equivalence.

package automaton

import (
	"errors"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// basicTerms mirrors the Java fixture in TestStringsToAutomaton.basicTerms.
func basicTerms() []*util.BytesRef {
	return []*util.BytesRef{
		util.NewBytesRef([]byte("dog")),
		util.NewBytesRef([]byte("day")),
		util.NewBytesRef([]byte("dad")),
		util.NewBytesRef([]byte("cats")),
		util.NewBytesRef([]byte("cat")),
	}
}

func sortBytesRefs(terms []*util.BytesRef) {
	sort.Slice(terms, func(i, j int) bool {
		return util.BytesRefCompare(terms[i], terms[j]) < 0
	})
}

func TestStringsToAutomaton_Basic(t *testing.T) {
	t.Parallel()
	terms := basicTerms()
	sortBytesRefs(terms)
	a, err := BuildStringUnion(terms, false)
	if err != nil {
		t.Fatalf("BuildStringUnion returned error: %v", err)
	}
	checkAcceptsAllAndOnly(t, terms, a, false)
}

func TestStringsToAutomaton_BasicBinary(t *testing.T) {
	t.Parallel()
	terms := basicTerms()
	sortBytesRefs(terms)
	a, err := BuildStringUnion(terms, true)
	if err != nil {
		t.Fatalf("BuildStringUnion returned error: %v", err)
	}
	checkAcceptsAllAndOnly(t, terms, a, true)
}

func TestStringsToAutomaton_LargeTerms(t *testing.T) {
	t.Parallel()
	b10k := make([]byte, 10_000)
	for i := range b10k {
		b10k[i] = 'a'
	}
	_, err := BuildStringUnion([]*util.BytesRef{util.NewBytesRef(b10k)}, false)
	if err == nil {
		t.Fatalf("BuildStringUnion: expected error for 10k-byte term, got nil")
	}
	const wantPrefix = "automaton: StringsToAutomaton does not allow terms larger than 1000 UTF-8 bytes"
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Fatalf("BuildStringUnion error = %q, want prefix %q", err.Error(), wantPrefix)
	}

	b1k := make([]byte, 1000)
	for i := range b1k {
		b1k[i] = 'a'
	}
	if _, err := BuildStringUnion([]*util.BytesRef{util.NewBytesRef(b1k)}, false); err != nil {
		t.Fatalf("BuildStringUnion: unexpected error for 1k-byte term: %v", err)
	}
}

func TestStringsToAutomaton_RejectsUnsortedInput(t *testing.T) {
	t.Parallel()
	terms := []*util.BytesRef{
		util.NewBytesRef([]byte("b")),
		util.NewBytesRef([]byte("a")),
	}
	_, err := BuildStringUnion(terms, true)
	if err == nil {
		t.Fatalf("BuildStringUnion: expected sort-order error, got nil")
	}
	if !strings.Contains(err.Error(), "must be in sorted UTF-8 order") {
		t.Fatalf("BuildStringUnion: unexpected error = %q", err.Error())
	}
}

func TestStringsToAutomaton_EmptyInput(t *testing.T) {
	t.Parallel()
	a, err := BuildStringUnion(nil, false)
	if err != nil {
		t.Fatalf("BuildStringUnion(nil) error = %v", err)
	}
	if a == nil {
		t.Fatalf("BuildStringUnion(nil) returned nil automaton")
	}
	// An empty union accepts nothing; matching even an empty input should
	// only succeed if the empty string was in the set, which it was not.
	runner := NewByteRunAutomatonBinary(a, true)
	if runner.Run([]byte("x"), 0, 1) {
		t.Errorf("empty union accepted 'x'")
	}
}

func TestStringsToAutomaton_IncludesEmptyString(t *testing.T) {
	t.Parallel()
	terms := []*util.BytesRef{
		util.NewBytesRef(nil),
		util.NewBytesRef([]byte("a")),
	}
	sortBytesRefs(terms)
	a, err := BuildStringUnion(terms, true)
	if err != nil {
		t.Fatalf("BuildStringUnion error = %v", err)
	}
	runner := NewByteRunAutomatonBinary(a, true)
	if !runner.Run(nil, 0, 0) {
		t.Errorf("expected automaton to accept empty string")
	}
	if !runner.Run([]byte("a"), 0, 1) {
		t.Errorf("expected automaton to accept 'a'")
	}
	if runner.Run([]byte("b"), 0, 1) {
		t.Errorf("automaton accepted unexpected term 'b'")
	}
}

func TestStringsToAutomaton_IteratorEntrypoint(t *testing.T) {
	t.Parallel()
	terms := basicTerms()
	sortBytesRefs(terms)
	it := bytesRefSliceIterator(terms)
	a, err := BuildStringUnionFromIterator(it, false)
	if err != nil {
		t.Fatalf("BuildStringUnionFromIterator error = %v", err)
	}
	checkAcceptsAllAndOnly(t, terms, a, false)
}

func TestStringsToAutomaton_IteratorPropagatesError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	it := util.BytesRefIteratorFunc(func() (*util.BytesRef, error) {
		return nil, sentinel
	})
	_, err := BuildStringUnionFromIterator(it, false)
	if !errors.Is(err, sentinel) {
		t.Fatalf("BuildStringUnionFromIterator error = %v, want %v", err, sentinel)
	}
}

func TestStringsToAutomaton_RandomBinary(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(0xC0FFEE, 0xBADC0DE))
	for iter := 0; iter < 10; iter++ {
		size := 500 + r.IntN(1500)
		terms := randomBinaryTerms(r, size)
		a, err := BuildStringUnion(terms, true)
		if err != nil {
			t.Fatalf("iter %d: BuildStringUnion error = %v", iter, err)
		}
		checkAcceptsAllAndOnly(t, terms, a, true)
	}
}

func TestStringsToAutomaton_RandomUnicode(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(0xFEEDFACE, 0xCAFEBABE))
	for iter := 0; iter < 10; iter++ {
		size := 500 + r.IntN(1500)
		terms := randomUnicodeTerms(r, size)
		a, err := BuildStringUnion(terms, false)
		if err != nil {
			t.Fatalf("iter %d: BuildStringUnion error = %v", iter, err)
		}
		checkAcceptsAllAndOnly(t, terms, a, false)
	}
}

// --- helpers ---

// checkAcceptsAllAndOnly mirrors TestStringsToAutomaton.checkAutomaton:
// every expected term must be accepted, and FiniteStringsIterator must
// enumerate exactly the expected set.
func checkAcceptsAllAndOnly(t *testing.T, expected []*util.BytesRef, a *Automaton, isBinary bool) {
	t.Helper()
	runner := newRunAutomaton(a, isBinary)
	for _, term := range expected {
		if !runner.Run(term.Bytes, term.Offset, term.Length) {
			t.Errorf("expected automaton to accept %q (binary=%v)", term, isBinary)
		}
	}

	// Iterate every finite string the automaton accepts and ensure it is in
	// the expected set. We need the byte-converted automaton for non-binary
	// input so that the iterator yields one int per byte (matching the
	// expected BytesRef encoding).
	var iterable *Automaton
	if isBinary {
		iterable = a
	} else {
		det, err := Determinize(NewUTF32ToUTF8().Convert(a), DefaultDeterminizeWorkLimit*100)
		if err != nil {
			t.Fatalf("Determinize: %v", err)
		}
		iterable = det
	}
	want := make(map[string]struct{}, len(expected))
	for _, term := range expected {
		want[string(term.Bytes[term.Offset:term.Offset+term.Length])] = struct{}{}
	}
	it := NewFiniteStringsIterator(iterable)
	for {
		ir, err := it.Next()
		if err != nil {
			t.Fatalf("FiniteStringsIterator.Next: %v", err)
		}
		if ir == nil {
			break
		}
		buf := make([]byte, ir.Length)
		for i := 0; i < ir.Length; i++ {
			buf[i] = byte(ir.Ints[ir.Offset+i])
		}
		if _, ok := want[string(buf)]; !ok {
			t.Errorf("automaton produced unexpected term %q", buf)
		}
	}
}

// newRunAutomaton wraps the Lucene byte-level run automaton. The non-binary
// case requires byte-encoding the codepoint automaton, which is exactly
// what NewByteRunAutomaton does internally.
func newRunAutomaton(a *Automaton, isBinary bool) *ByteRunAutomaton {
	if isBinary {
		return NewByteRunAutomatonBinary(a, true)
	}
	return NewByteRunAutomaton(a)
}

// bytesRefSliceIterator wraps a slice in the BytesRefIterator interface.
func bytesRefSliceIterator(terms []*util.BytesRef) util.BytesRefIterator {
	idx := 0
	return util.BytesRefIteratorFunc(func() (*util.BytesRef, error) {
		if idx >= len(terms) {
			return nil, nil
		}
		out := terms[idx]
		idx++
		return out, nil
	})
}

// randomBinaryTerms returns a sorted, deduplicated set of random binary
// terms whose individual length stays below MaxStringUnionTermLength.
func randomBinaryTerms(r *rand.Rand, size int) []*util.BytesRef {
	seen := make(map[string]struct{}, size)
	out := make([]*util.BytesRef, 0, size)
	for len(out) < size {
		n := 1 + r.IntN(16)
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(r.IntN(256))
		}
		if _, dup := seen[string(b)]; dup {
			continue
		}
		seen[string(b)] = struct{}{}
		out = append(out, util.NewBytesRef(b))
	}
	sortBytesRefs(out)
	return out
}

// randomUnicodeTerms returns a sorted, deduplicated set of random terms
// encoded as valid UTF-8. Each term has at most 16 codepoints.
func randomUnicodeTerms(r *rand.Rand, size int) []*util.BytesRef {
	seen := make(map[string]struct{}, size)
	out := make([]*util.BytesRef, 0, size)
	for len(out) < size {
		n := 1 + r.IntN(16)
		buf := make([]byte, 0, 4*n)
		for i := 0; i < n; i++ {
			// Pick from BMP, skipping the surrogate range so the result is
			// always valid UTF-8.
			var cp rune
			for {
				cp = rune(r.IntN(0xFFFE) + 1)
				if cp < 0xD800 || cp > 0xDFFF {
					break
				}
			}
			tmp := make([]byte, utf8.UTFMax)
			w := utf8.EncodeRune(tmp, cp)
			buf = append(buf, tmp[:w]...)
		}
		if _, dup := seen[string(buf)]; dup {
			continue
		}
		seen[string(buf)] = struct{}{}
		out = append(out, util.NewBytesRef(buf))
	}
	sortBytesRefs(out)
	return out
}
