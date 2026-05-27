// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// --- OrdTermState ------------------------------------------------------------

func TestOrdTermState_CopyFromSame(t *testing.T) {
	src := &OrdTermState{Ord: 42}
	dst := NewOrdTermState()
	if err := dst.CopyFrom(src); err != nil {
		t.Fatalf("CopyFrom: %v", err)
	}
	if dst.Ord != 42 {
		t.Errorf("Ord=%d, want 42", dst.Ord)
	}
}

type incompatibleTermState struct{}

func (incompatibleTermState) CopyFrom(_ TermState) error { return nil }

func TestOrdTermState_CopyFromDifferent(t *testing.T) {
	dst := NewOrdTermState()
	err := dst.CopyFrom(incompatibleTermState{})
	if err == nil {
		t.Errorf("expected error for incompatible source")
	}
}

func TestOrdTermState_String(t *testing.T) {
	if got := (&OrdTermState{Ord: 9}).String(); got != "OrdTermState ord=9" {
		t.Errorf("String=%q", got)
	}
}

// --- FreqAndNormBuffer --------------------------------------------------------

func TestFreqAndNormBuffer_Add(t *testing.T) {
	b := NewFreqAndNormBuffer()
	for i := 0; i < 20; i++ {
		b.Add(i, int64(i)*2)
	}
	if b.Size != 20 {
		t.Errorf("Size=%d", b.Size)
	}
	for i := 0; i < 20; i++ {
		if b.Freqs[i] != i || b.Norms[i] != int64(i)*2 {
			t.Errorf("entry %d mismatch: freq=%d norm=%d", i, b.Freqs[i], b.Norms[i])
		}
	}
}

func TestFreqAndNormBuffer_GrowNoCopy(t *testing.T) {
	b := NewFreqAndNormBuffer()
	b.GrowNoCopy(100)
	if len(b.Freqs) < 100 || len(b.Norms) < 100 {
		t.Errorf("GrowNoCopy did not grow: freqs=%d norms=%d", len(b.Freqs), len(b.Norms))
	}
	if b.Size != 0 {
		t.Errorf("Size should remain 0 after GrowNoCopy")
	}
}

// --- DocsWithFieldSet, EmptyDocValuesProducer covered in earlier files --------

// --- FilteredTermsEnum -------------------------------------------------------

type acceptOnlyA struct{}

func (acceptOnlyA) Accept(t *Term) (AcceptStatus, error) {
	if t.Text() == "a" {
		return AcceptYes, nil
	}
	return AcceptNo, nil
}
func (acceptOnlyA) NextSeekTerm(_ *Term) (*Term, error) { return nil, nil }

func TestFilteredTermsEnum_AcceptOnlyA(t *testing.T) {
	// Build a delegate over three terms.
	delegate := newFakeTermsEnum([]string{"a", "b", "c"})
	fe := NewFilteredTermsEnum(delegate, acceptOnlyA{})
	got, err := fe.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got == nil || got.Text() != "a" {
		t.Errorf("Next #1=%v, want term 'a'", got)
	}
	got2, err := fe.Next()
	if err != nil {
		t.Fatalf("Next #2: %v", err)
	}
	if got2 != nil {
		t.Errorf("Next #2 should be nil, got %v", got2)
	}
}

// --- ImpactsEnum interface ----------------------------------------------------

// stubImpactsEnum verifies ImpactsEnum is a satisfiable composite interface.
type stubImpactsEnum struct{}

func (stubImpactsEnum) DocID() int                         { return -1 }
func (stubImpactsEnum) Advance(int) (int, error)           { return NO_MORE_DOCS, nil }
func (stubImpactsEnum) NextDoc() (int, error)              { return NO_MORE_DOCS, nil }
func (stubImpactsEnum) Cost() int64                        { return 0 }
func (stubImpactsEnum) Freq() (int, error)                 { return 0, nil }
func (stubImpactsEnum) NextPosition() (int, error)         { return -1, nil }
func (stubImpactsEnum) StartOffset() (int, error)          { return -1, nil }
func (stubImpactsEnum) EndOffset() (int, error)            { return -1, nil }
func (stubImpactsEnum) GetPayload() ([]byte, error)        { return nil, nil }
func (stubImpactsEnum) AdvanceShallow(_ int) error         { return nil }
func (stubImpactsEnum) GetImpacts() (Impacts, error)       { return nil, errors.New("stub") }

func TestImpactsEnum_SatisfiableComposite(t *testing.T) {
	var ie ImpactsEnum = stubImpactsEnum{}
	if _, err := ie.GetImpacts(); err == nil {
		t.Errorf("expected stub error")
	}
}

// --- FieldInvertState ---------------------------------------------------------

func TestFieldInvertState_Accessors(t *testing.T) {
	s := NewFieldInvertStateFull(10, "title", IndexOptionsDocsAndFreqs, 1, 2, 0, 5, 7, 3)
	if s.Name() != "title" || s.IndexOptions() != IndexOptionsDocsAndFreqs {
		t.Errorf("name/options mismatch")
	}
	if s.Position() != 1 || s.Length() != 2 || s.Offset() != 5 || s.MaxTermFrequency() != 7 ||
		s.UniqueTermCount() != 3 || s.NumOverlap() != 0 || s.IndexCreatedVersionMajor() != 10 {
		t.Errorf("accessor mismatch: %+v", s)
	}
	s.SetPosition(11)
	if s.Position() != 11 {
		t.Errorf("SetPosition not honoured")
	}
}

// --- PrefixCodedTerms --------------------------------------------------------

func TestPrefixCodedTerms_BuilderAndIterator(t *testing.T) {
	b := NewPrefixCodedTermsBuilder()
	b.Add(NewTerm("f1", "foo"))
	b.AddFieldBytes("f1", []byte("bar"))
	b.AddFieldBytes("f2", []byte("baz"))
	p := b.Finish()
	p.SetDelGen(7)
	if p.Size() != 3 {
		t.Errorf("Size=%d", p.Size())
	}
	it := p.Iterator()
	want := []struct {
		field, term string
	}{
		{"f1", "bar"},
		{"f1", "foo"},
		{"f2", "baz"},
	}
	for i := 0; i < 3; i++ {
		bytes := it.Next()
		if bytes == nil {
			t.Fatalf("iterator exhausted at %d", i)
		}
		if it.Field() != want[i].field || string(bytes) != want[i].term {
			t.Errorf("entry %d: got (%q,%q), want (%q,%q)", i, it.Field(), bytes, want[i].field, want[i].term)
		}
	}
	if it.DelGen() != 7 {
		t.Errorf("DelGen=%d", it.DelGen())
	}
	if it.Next() != nil {
		t.Errorf("iterator should be exhausted")
	}
}

// --- MultiTerms --------------------------------------------------------------

func TestMultiTerms_ConstructionInvariants(t *testing.T) {
	if _, err := NewMultiTerms(nil, []ReaderSlice{{0, 1, 0}}); err == nil {
		t.Errorf("expected length-mismatch error")
	}
	mt, err := NewMultiTerms([]Terms{}, []ReaderSlice{})
	if err != nil {
		t.Fatal(err)
	}
	if mt.Size() != -1 {
		t.Errorf("MultiTerms.Size() should be -1 per Lucene contract")
	}
	if _, err := mt.Iterator(); !errors.Is(err, ErrMultiTermsEnumNotImplemented) {
		t.Errorf("Iterator should return ErrMultiTermsEnumNotImplemented, got %v", err)
	}
}

// --- fakeTermsEnum used by FilteredTermsEnum tests ----------------------------

type fakeTermsEnum struct {
	TermsEnumBase
	items []string
	pos   int
}

func newFakeTermsEnum(items []string) *fakeTermsEnum {
	return &fakeTermsEnum{items: items, pos: -1}
}

func (f *fakeTermsEnum) Next() (*Term, error) {
	f.pos++
	if f.pos >= len(f.items) {
		return nil, nil
	}
	t := NewTerm("f", f.items[f.pos])
	f.SetCurrentTerm(t)
	return t, nil
}

func (f *fakeTermsEnum) SeekCeil(t *Term) (*Term, error) {
	for i, it := range f.items {
		if it >= t.Text() {
			f.pos = i
			term := NewTerm("f", it)
			f.SetCurrentTerm(term)
			return term, nil
		}
	}
	f.pos = len(f.items)
	return nil, nil
}

func (f *fakeTermsEnum) SeekExact(t *Term) (bool, error) {
	for i, it := range f.items {
		if it == t.Text() {
			f.pos = i
			term := NewTerm("f", it)
			f.SetCurrentTerm(term)
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeTermsEnum) DocFreq() (int, error)         { return 0, nil }
func (f *fakeTermsEnum) TotalTermFreq() (int64, error) { return 0, nil }
func (f *fakeTermsEnum) Postings(int) (PostingsEnum, error) {
	return nil, nil
}
func (f *fakeTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (PostingsEnum, error) {
	return nil, nil
}
