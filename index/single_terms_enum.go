package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

type singleTermsEnumAcceptor struct {
	singleRef *util.BytesRef
}

func (a *singleTermsEnumAcceptor) Accept(term *Term) (AcceptStatus, error) {
	if term.Bytes.Equals(a.singleRef) {
		return AcceptYes, nil
	}
	return AcceptEnd, nil
}

func (a *singleTermsEnumAcceptor) NextSeekTerm(current *Term) (*Term, error) {
	return nil, nil
}

// SingleTermsEnum is a subclass of FilteredTermsEnum for enumerating a single term.
// This is the Go port of Lucene's org.apache.lucene.index.SingleTermsEnum.
type SingleTermsEnum struct {
	*FilteredTermsEnum
}

// NewSingleTermsEnum creates a new SingleTermsEnum.
func NewSingleTermsEnum(tenum TermsEnum, termText *util.BytesRef) *SingleTermsEnum {
	acceptor := &singleTermsEnumAcceptor{singleRef: termText}
	fte := NewFilteredTermsEnum(tenum, acceptor)
	fte.SetInitialSeekTerm(termText)
	return &SingleTermsEnum{FilteredTermsEnum: fte}
}
