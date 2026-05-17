// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
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

	delegate      TermsEnum
	acceptor      FilteredTermsEnumAcceptor
	initialSeek   *Term
	startWithSeek bool
}

// NewFilteredTermsEnum wraps delegate with the given acceptor (default:
// no initial seek).
func NewFilteredTermsEnum(delegate TermsEnum, acceptor FilteredTermsEnumAcceptor) *FilteredTermsEnum {
	return NewFilteredTermsEnumWithSeek(delegate, acceptor, false)
}

// NewFilteredTermsEnumWithSeek wraps delegate with the given acceptor.
// If startWithSeek is true, the first Next() will call NextSeekTerm(nil)
// to obtain a starting term and seek to it.
func NewFilteredTermsEnumWithSeek(delegate TermsEnum, acceptor FilteredTermsEnumAcceptor, startWithSeek bool) *FilteredTermsEnum {
	return &FilteredTermsEnum{
		delegate:      delegate,
		acceptor:      acceptor,
		startWithSeek: startWithSeek,
	}
}

// SetInitialSeekTerm sets the initial seek term. Equivalent to Lucene's
// setInitialSeekTerm.
func (f *FilteredTermsEnum) SetInitialSeekTerm(term *Term) {
	f.initialSeek = term
}

// Next advances to the next accepted term. Returns nil when the enumeration
// is exhausted.
func (f *FilteredTermsEnum) Next() (*Term, error) {
	// Honour an explicit initial seek if one is set.
	if f.initialSeek != nil {
		seek, err := f.delegate.SeekCeil(f.initialSeek)
		f.initialSeek = nil
		if err != nil {
			return nil, err
		}
		if seek == nil {
			f.currentTerm = nil
			return nil, nil
		}
	} else if f.startWithSeek {
		// Honour startWithSeek by asking the acceptor for an initial seek term.
		seek, err := f.acceptor.NextSeekTerm(nil)
		f.startWithSeek = false
		if err != nil {
			return nil, err
		}
		if seek != nil {
			if _, err := f.delegate.SeekCeil(seek); err != nil {
				return nil, err
			}
		}
	}

	for {
		var term *Term
		if f.currentTerm == nil && f.initialSeek == nil {
			t, err := f.delegate.Next()
			if err != nil {
				return nil, err
			}
			term = t
		} else {
			t, err := f.delegate.Next()
			if err != nil {
				return nil, err
			}
			term = t
		}
		if term == nil {
			f.currentTerm = nil
			return nil, nil
		}
		status, err := f.acceptor.Accept(term)
		if err != nil {
			return nil, err
		}
		switch status {
		case AcceptYes, AcceptYesAndSeek:
			f.currentTerm = term
			if status == AcceptYesAndSeek {
				if seek, serr := f.acceptor.NextSeekTerm(term); serr != nil {
					return term, serr
				} else if seek != nil {
					if _, derr := f.delegate.SeekCeil(seek); derr != nil {
						return term, derr
					}
				}
			}
			return term, nil
		case AcceptNo:
			continue
		case AcceptNoAndSeek:
			seek, serr := f.acceptor.NextSeekTerm(term)
			if serr != nil {
				return nil, serr
			}
			if seek != nil {
				if _, derr := f.delegate.SeekCeil(seek); derr != nil {
					return nil, derr
				}
			}
			continue
		case AcceptEnd:
			f.currentTerm = nil
			return nil, nil
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
		f.currentTerm = nil
		return nil, nil
	}
	status, err := f.acceptor.Accept(got)
	if err != nil {
		return nil, err
	}
	if status == AcceptYes || status == AcceptYesAndSeek {
		f.currentTerm = got
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
		f.currentTerm = nil
		return false, nil
	}
	status, err := f.acceptor.Accept(got)
	if err != nil {
		return false, err
	}
	if status == AcceptYes || status == AcceptYesAndSeek {
		f.currentTerm = got
		return true, nil
	}
	return false, nil
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
