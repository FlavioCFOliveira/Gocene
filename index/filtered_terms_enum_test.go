// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Regression tests for org.apache.lucene.index.FilteredTermsEnum#next().
//
// Gocene used to seek and then call the delegate's Next(), which skipped the
// very term it had just sought; and it performed the YES_AND_SEEK seek eagerly,
// which moved the delegate off the term it was returning. Java does neither:
//
//	if (doSeek) {
//	  doSeek = false;
//	  final BytesRef t = nextSeekTerm(actualTerm);
//	  if (t == null || tenum.seekCeil(t) == SeekStatus.END) return null;
//	  actualTerm = tenum.term();          // <- the sought term IS the candidate
//	} else {
//	  actualTerm = tenum.next();
//	  ...
//	}
//	switch (accept(actualTerm)) {
//	  case YES_AND_SEEK: doSeek = true;   // <- only ARMS the seek, falls through
//	  case YES: return actualTerm;
//	  ...
//	}

// drainFiltered collects the whole enumeration as text.
func drainFiltered(t *testing.T, fe *FilteredTermsEnum) []string {
	t.Helper()
	var got []string
	for {
		term, err := fe.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if term == nil {
			return got
		}
		got = append(got, term.Text())
	}
}

// TestFilteredTermsEnum_SeekBranchAcceptsSoughtTerm pins the defect directly:
// SingleTermsEnum seeds setInitialSeekTerm and its accept() only ever says YES
// to that one term, so if next() skips the term it sought the enumeration is
// empty. Java yields [lucene]; Gocene used to yield [].
func TestFilteredTermsEnum_SeekBranchAcceptsSoughtTerm(t *testing.T) {
	delegate := newFilteredFakeTermsEnum([]string{"alpha", "lucene", "zebra"})
	fe := NewSingleTermFilteredEnum(delegate, NewTerm("f", "lucene"))

	got := drainFiltered(t, fe)
	if len(got) != 1 || got[0] != "lucene" {
		t.Fatalf("SingleTermsEnum over [alpha lucene zebra] seeking \"lucene\" = %v, want [lucene]", got)
	}
}

// TestFilteredTermsEnum_SeekBranchMissingTermIsEmpty is the negative half of the
// test above: a term that is not in the dictionary must yield nothing, because
// seekCeil lands on the next term and accept() answers END for it.
func TestFilteredTermsEnum_SeekBranchMissingTermIsEmpty(t *testing.T) {
	delegate := newFilteredFakeTermsEnum([]string{"alpha", "lucene", "zebra"})
	fe := NewSingleTermFilteredEnum(delegate, NewTerm("f", "gocene"))

	if got := drainFiltered(t, fe); len(got) != 0 {
		t.Fatalf("SingleTermsEnum seeking absent \"gocene\" = %v, want []", got)
	}
}

// setLikeAcceptor reproduces the accept/nextSeekTerm pair of
// org.apache.lucene.search.TermInSetQuery.SetEnum, the other caller that drives
// next() through its seek branch — this time via YES_AND_SEEK / NO_AND_SEEK
// rather than an initial seek term.
type setLikeAcceptor struct {
	want []string
	i    int
}

func (s *setLikeAcceptor) Accept(term *Term) (AcceptStatus, error) {
	cmp := 0
	for s.i < len(s.want) {
		cmp = strings.Compare(s.want[s.i], term.Text())
		if cmp >= 0 {
			break
		}
		s.i++
	}
	if s.i >= len(s.want) {
		return AcceptEnd, nil
	}
	if cmp == 0 {
		return AcceptYesAndSeek, nil
	}
	return AcceptNoAndSeek, nil
}

func (s *setLikeAcceptor) NextSeekTerm(current *Term) (*Term, error) {
	if current != nil {
		for s.i < len(s.want) && s.want[s.i] <= current.Text() {
			s.i++
		}
	}
	if s.i >= len(s.want) {
		return nil, nil
	}
	return NewTerm("f", s.want[s.i]), nil
}

// TestFilteredTermsEnum_SeekBranchYieldsEverySoughtTerm drives the seek branch
// repeatedly: every one of the sought terms must be handed to accept() and
// returned. With the old body each seek was followed by delegate.Next(), so the
// sought terms were consumed without ever being offered to accept().
func TestFilteredTermsEnum_SeekBranchYieldsEverySoughtTerm(t *testing.T) {
	delegate := newFilteredFakeTermsEnum([]string{"a", "b", "c", "d", "e"})
	fe := NewFilteredTermsEnum(delegate, &setLikeAcceptor{want: []string{"c", "e"}})

	got := drainFiltered(t, fe)
	if len(got) != 2 || got[0] != "c" || got[1] != "e" {
		t.Fatalf("set-like enum over [a b c d e] wanting [c e] = %v, want [c e]", got)
	}
}

// TestFilteredTermsEnum_YesAndSeekLeavesDelegateOnTerm pins the second half of
// the defect. Java's YES_AND_SEEK sets doSeek and falls through to `return
// actualTerm` — it does NOT seek yet — so the delegate stays parked on the term
// just returned and term(), docFreq() and postings() still describe it. Gocene
// used to seek immediately, which handed callers postings for a later term.
func TestFilteredTermsEnum_YesAndSeekLeavesDelegateOnTerm(t *testing.T) {
	delegate := newFilteredFakeTermsEnum([]string{"a", "b", "c", "d", "e"})
	fe := NewFilteredTermsEnum(delegate, &setLikeAcceptor{want: []string{"c", "e"}})

	term, err := fe.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if term == nil || term.Text() != "c" {
		t.Fatalf("Next = %v, want term c", term)
	}
	if got := delegate.Term(); got == nil || got.Text() != "c" {
		t.Fatalf("delegate parked on %v after YES_AND_SEEK, want c (the seek is only armed, not performed)", got)
	}
	if got := fe.Term(); got == nil || got.Text() != "c" {
		t.Fatalf("FilteredTermsEnum.Term() = %v after YES_AND_SEEK, want c", got)
	}
}

// endlessAcceptor never accepts and never proposes a seek target.
type endlessAcceptor struct{}

func (endlessAcceptor) Accept(*Term) (AcceptStatus, error) { return AcceptNo, nil }
func (endlessAcceptor) NextSeekTerm(*Term) (*Term, error)  { return nil, nil }

// TestFilteredTermsEnum_NilSeekTermEndsEnumeration pins the branch
// `if (t == null || tenum.seekCeil(t) == SeekStatus.END) return null;`, which is
// what makes Lucene's documented statement true: "If the initial seek term is
// null (default), the enum is empty." The default constructor starts with
// doSeek == true, exactly as Java's `FilteredTermsEnum(tenum)` does via
// `this(tenum, true)`.
func TestFilteredTermsEnum_NilSeekTermEndsEnumeration(t *testing.T) {
	delegate := newFilteredFakeTermsEnum([]string{"a", "b", "c"})
	fe := NewFilteredTermsEnum(delegate, endlessAcceptor{})

	if got := drainFiltered(t, fe); len(got) != 0 {
		t.Fatalf("default-constructed enum with no seek term = %v, want [] (Lucene: \"the enum is empty\")", got)
	}
}

// TestFilteredTermsEnum_StartWithSeekFalseScansFromTheStart is the companion of
// the test above for `FilteredTermsEnum(tenum, false)`: with doSeek initially
// false the first iteration takes the tenum.next() branch and the enumeration
// is a plain filtered scan.
func TestFilteredTermsEnum_StartWithSeekFalseScansFromTheStart(t *testing.T) {
	delegate := newFilteredFakeTermsEnum([]string{"a", "b", "c"})
	fe := NewFilteredTermsEnumWithSeek(delegate, endlessAcceptor{}, false)

	if got := drainFiltered(t, fe); len(got) != 0 {
		t.Fatalf("scan with an always-NO acceptor = %v, want []", got)
	}
	// The delegate must have been walked to exhaustion, not seeked past.
	if got := delegate.Term(); got != nil {
		t.Fatalf("delegate term after exhaustion = %v, want nil", got)
	}
}

// filteredFakeTermsEnum is a sorted in-memory TermsEnum used as the delegate of
// the enumerators under test. It is self-contained on purpose: package index's
// older fake (phase5_test.go) predates TermsEnum.Impacts and no longer
// satisfies the interface.
type filteredFakeTermsEnum struct {
	TermsEnumBase
	items []string
	pos   int
}

func newFilteredFakeTermsEnum(items []string) *filteredFakeTermsEnum {
	return &filteredFakeTermsEnum{items: items, pos: -1}
}

func (f *filteredFakeTermsEnum) Next() (*Term, error) {
	f.pos++
	if f.pos >= len(f.items) {
		f.SetCurrentTerm(nil)
		return nil, nil
	}
	t := NewTerm("f", f.items[f.pos])
	f.SetCurrentTerm(t)
	return t, nil
}

// SeekCeil positions on the least term >= t, returning nil for SeekStatus.END.
func (f *filteredFakeTermsEnum) SeekCeil(t *Term) (*Term, error) {
	for i, it := range f.items {
		if it >= t.Text() {
			f.pos = i
			term := NewTerm("f", it)
			f.SetCurrentTerm(term)
			return term, nil
		}
	}
	f.pos = len(f.items)
	f.SetCurrentTerm(nil)
	return nil, nil
}

func (f *filteredFakeTermsEnum) SeekExact(t *Term) (bool, error) {
	for i, it := range f.items {
		if it == t.Text() {
			f.pos = i
			f.SetCurrentTerm(NewTerm("f", it))
			return true, nil
		}
	}
	return false, nil
}

func (f *filteredFakeTermsEnum) Ord() int64                    { return int64(f.pos) }
func (f *filteredFakeTermsEnum) DocFreq() (int, error)         { return 1, nil }
func (f *filteredFakeTermsEnum) TotalTermFreq() (int64, error) { return 1, nil }
func (f *filteredFakeTermsEnum) Postings(int) (PostingsEnum, error) {
	return nil, nil
}
func (f *filteredFakeTermsEnum) Impacts(int) (spi.ImpactsEnum, error) {
	return nil, nil
}
func (f *filteredFakeTermsEnum) PostingsWithLiveDocs(util.Bits, int) (PostingsEnum, error) {
	return nil, nil
}
