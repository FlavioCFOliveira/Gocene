// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// singleTermsEnumAcceptor is the predicate for SingleTermsEnum.
type singleTermsEnumAcceptor struct {
	singleRef *Term
}

func (a *singleTermsEnumAcceptor) Accept(term *Term) (AcceptStatus, error) {
	if term.Equals(a.singleRef) {
		return AcceptYes, nil
	}
	return AcceptEnd, nil
}

func (a *singleTermsEnumAcceptor) NextSeekTerm(current *Term) (*Term, error) {
	return nil, nil
}

// SingleTermsEnum is a FilteredTermsEnum for enumerating a single term.
// Mirrors org.apache.lucene.index.SingleTermsEnum from Apache Lucene 10.5.0.
type SingleTermsEnum struct {
	*FilteredTermsEnum
}

// NewSingleTermsEnum creates a new SingleTermsEnum.
// After calling the constructor the enumeration is already pointing to the term, if it exists.
func NewSingleTermsEnum(tenum TermsEnum, termText *Term) *SingleTermsEnum {
	acceptor := &singleTermsEnumAcceptor{singleRef: termText}
	fte := NewFilteredTermsEnum(tenum, acceptor)
	fte.SetInitialSeekTerm(termText)
	return &SingleTermsEnum{FilteredTermsEnum: fte}
}
