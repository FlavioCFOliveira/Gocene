// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AcceptStatus is the result of FilteredTermsEnum.Accept. Mirrors
// FilteredTermsEnum.AcceptStatus from Apache Lucene 10.4.0.
type AcceptStatus int

const (
	// AcceptYes accepts the current term.
	AcceptYes AcceptStatus = iota
	// AcceptYesAndSeek accepts the current term and asks the enumerator to
	// produce the next seek term via NextSeekTerm before resuming iteration.
	AcceptYesAndSeek
	// AcceptNo skips the current term and continues iteration.
	AcceptNo
	// AcceptNoAndSeek skips the current term and asks the enumerator to seek.
	AcceptNoAndSeek
	// AcceptEnd halts iteration.
	AcceptEnd
)

// FilteredTermsEnumAcceptor is the predicate consumed by FilteredTermsEnum.
// Mirrors the abstract FilteredTermsEnum.accept(BytesRef) hook.
type FilteredTermsEnumAcceptor interface {
	// Accept inspects the candidate term and returns one of AcceptStatus.
	Accept(term *Term) (AcceptStatus, error)

	// NextSeekTerm is called when Accept returns *_AND_SEEK. The default
	// (returning nil) is interpreted as "no specific seek target — advance
	// normally". Implementations may override to skip ahead by ord.
	NextSeekTerm(current *Term) (*Term, error)
}

// FilteredTermsEnum wraps a delegate TermsEnum and yields only the terms
// approved by an FilteredTermsEnumAcceptor. Mirrors
// org.apache.lucene.index.FilteredTermsEnum (Apache Lucene 10.4.0).
//
// Gocene models the Java abstract class as a value type that takes its
// Accept and NextSeekTerm callbacks via FilteredTermsEnumAcceptor. The
// delegate must already be positioned at the beginning of the enumeration.
type FilteredTermsEnum struct {
	TermsEnumBase

	delegate TermsEnum
	acceptor FilteredTermsEnumAcceptor

	// initialSeek mirrors the private BytesRef initialSeekTerm of the Java
	// class: the term the default nextSeekTerm hands out exactly once.
	initialSeek *Term
	// doSeek mirrors the private boolean doSeek of the Java class.
	doSeek bool
	// actualTerm mirrors the protected BytesRef actualTerm of the Java class:
	// which term the enum is currently positioned to.
	actualTerm *Term
}

// NewFilteredTermsEnum wraps delegate with the given acceptor, reproducing
// the one-argument constructor of org.apache.lucene.index.FilteredTermsEnum:
//
//	protected FilteredTermsEnum(final TermsEnum tenum) { this(tenum, true); }
//
// The enumeration therefore starts with a seek, so a subclass must either set
// an initial seek term or override NextSeekTerm; with neither, Lucene
// documents the enum as empty.
func NewFilteredTermsEnum(delegate TermsEnum, acceptor FilteredTermsEnumAcceptor) *FilteredTermsEnum {
	return NewFilteredTermsEnumWithSeek(delegate, acceptor, true)
}

// NewFilteredTermsEnumWithSeek wraps delegate with the given acceptor,
// reproducing the two-argument constructor of
// org.apache.lucene.index.FilteredTermsEnum:
//
//	protected FilteredTermsEnum(final TermsEnum tenum, final boolean startWithSeek) {
//	  this.tenum = tenum;
//	  doSeek = startWithSeek;
//	}
func NewFilteredTermsEnumWithSeek(delegate TermsEnum, acceptor FilteredTermsEnumAcceptor, startWithSeek bool) *FilteredTermsEnum {
	return &FilteredTermsEnum{
		delegate: delegate,
		acceptor: acceptor,
		doSeek:   startWithSeek,
	}
}

// SetInitialSeekTerm sets the initial seek term. Equivalent to Lucene's
// setInitialSeekTerm(BytesRef). Gocene's TermsEnum is keyed on *Term rather
// than a bare BytesRef, so the seek key carries its field alongside the bytes.
func (f *FilteredTermsEnum) SetInitialSeekTerm(term *Term) {
	f.initialSeek = term
}

// nextSeekTerm reproduces org.apache.lucene.index.FilteredTermsEnum#nextSeekTerm
// together with the way a Java subclass overrides it.
//
// The Java default body is:
//
//	protected BytesRef nextSeekTerm(final BytesRef currentTerm) throws IOException {
//	  final BytesRef t = initialSeekTerm;
//	  initialSeekTerm = null;
//	  return t;
//	}
//
// A subclass that overrides the method (AutomatonTermsEnum, TermInSetQuery's
// SetEnum) replaces that default entirely and never observes initialSeekTerm.
// Gocene renders the override as FilteredTermsEnumAcceptor.NextSeekTerm, so an
// acceptor that produces a term wins and the Java default runs otherwise —
// which is exactly the Java arrangement, because a subclass that keeps the
// default is the only one that may call SetInitialSeekTerm.
func (f *FilteredTermsEnum) nextSeekTerm(current *Term) (*Term, error) {
	t, err := f.acceptor.NextSeekTerm(current)
	if err != nil {
		return nil, err
	}
	if t != nil {
		return t, nil
	}
	t = f.initialSeek
	f.initialSeek = nil
	return t, nil
}

// setActualTerm assigns the Java field `actualTerm` and keeps the term cached
// by TermsEnumBase in step with it, so that Term() reports what Java's
// term() — which forwards to tenum.term() — would report.
func (f *FilteredTermsEnum) setActualTerm(t *Term) {
	f.actualTerm = t
	f.SetCurrentTerm(t)
}

// Next advances to the next accepted term. Returns nil when the enumeration
// is exhausted.
//
// This is the body of org.apache.lucene.index.FilteredTermsEnum#next() in
// Apache Lucene 10.5.0:
//
//	for (;;) {
//	  // Seek or forward the iterator
//	  if (doSeek) {
//	    doSeek = false;
//	    final BytesRef t = nextSeekTerm(actualTerm);
//	    if (t == null || tenum.seekCeil(t) == SeekStatus.END) {
//	      return null;                       // no more terms to seek to or enum exhausted
//	    }
//	    actualTerm = tenum.term();
//	  } else {
//	    actualTerm = tenum.next();
//	    if (actualTerm == null) {
//	      return null;                       // enum exhausted
//	    }
//	  }
//	  switch (accept(actualTerm)) {
//	    case YES_AND_SEEK: doSeek = true;    // falls through
//	    case YES:          return actualTerm;
//	    case NO_AND_SEEK:  doSeek = true; break;
//	    case END:          return null;
//	    case NO:           break;
//	  }
//	}
//
// Note in particular that on the seek branch Java accepts the term it landed
// on (`actualTerm = tenum.term()`), and that YES_AND_SEEK only *arms* the seek
// for the following call — the delegate stays parked on the term just
// returned, so term(), docFreq() and postings() still describe it.
func (f *FilteredTermsEnum) Next() (*Term, error) {
	for {
		// Seek or forward the iterator.
		if f.doSeek {
			f.doSeek = false
			t, err := f.nextSeekTerm(f.actualTerm)
			if err != nil {
				return nil, err
			}
			if t == nil {
				// no more terms to seek to
				f.setActualTerm(nil)
				return nil, nil
			}
			got, err := f.delegate.SeekCeil(t)
			if err != nil {
				return nil, err
			}
			if got == nil {
				// SeekStatus.END: enum exhausted
				f.setActualTerm(nil)
				return nil, nil
			}
			// actualTerm = tenum.term(): SeekCeil hands back exactly the term
			// the delegate is now positioned on.
			f.setActualTerm(got)
		} else {
			t, err := f.delegate.Next()
			if err != nil {
				return nil, err
			}
			f.setActualTerm(t)
			if t == nil {
				// enum exhausted
				return nil, nil
			}
		}

		// check if term is accepted
		status, err := f.acceptor.Accept(f.actualTerm)
		if err != nil {
			return nil, err
		}
		switch status {
		case AcceptYesAndSeek:
			// term accepted, but we need to seek next time (Java falls through
			// to the YES arm without seeking now)
			f.doSeek = true
			return f.actualTerm, nil
		case AcceptYes:
			// term accepted
			return f.actualTerm, nil
		case AcceptNoAndSeek:
			// invalid term, seek next time
			f.doSeek = true
		case AcceptEnd:
			// we are supposed to end the enum
			return nil, nil
		case AcceptNo:
			// we just iterate again
		}
	}
}

// SeekCeil delegates to the underlying iterator and applies acceptance.
func (f *FilteredTermsEnum) SeekCeil(term *Term) (*Term, error) {
	got, err := f.delegate.SeekCeil(term)
	if err != nil {
		return nil, err
	}
	if got == nil {
		f.SetCurrentTerm(nil)
		return nil, nil
	}
	status, err := f.acceptor.Accept(got)
	if err != nil {
		return nil, err
	}
	if status == AcceptYes || status == AcceptYesAndSeek {
		f.SetCurrentTerm(got)
		return got, nil
	}
	// fall through to Next() until something is accepted
	return f.Next()
}

// SeekExact returns true if delegate found an exact-matching accepted term.
func (f *FilteredTermsEnum) SeekExact(term *Term) (bool, error) {
	got, err := f.delegate.SeekCeil(term)
	if err != nil {
		return false, err
	}
	if got == nil || !got.Equals(term) {
		f.SetCurrentTerm(nil)
		return false, nil
	}
	status, err := f.acceptor.Accept(got)
	if err != nil {
		return false, err
	}
	if status == AcceptYes || status == AcceptYesAndSeek {
		f.SetCurrentTerm(got)
		return true, nil
	}
	return false, nil
}

// Ord passes through, mirroring FilteredTermsEnum.ord() which delegates to
// the wrapped enumerator.
func (f *FilteredTermsEnum) Ord() int64 { return f.delegate.Ord() }

// Impacts passes through, mirroring FilteredTermsEnum.impacts(int) which
// delegates to the wrapped enumerator.
func (f *FilteredTermsEnum) Impacts(flags int) (spi.ImpactsEnum, error) {
	return f.delegate.Impacts(flags)
}

// DocFreq passes through.
func (f *FilteredTermsEnum) DocFreq() (int, error) { return f.delegate.DocFreq() }

// TotalTermFreq passes through.
func (f *FilteredTermsEnum) TotalTermFreq() (int64, error) { return f.delegate.TotalTermFreq() }

// Postings passes through.
func (f *FilteredTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return f.delegate.Postings(flags)
}

// PostingsWithLiveDocs passes through.
func (f *FilteredTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return f.delegate.PostingsWithLiveDocs(liveDocs, flags)
}
