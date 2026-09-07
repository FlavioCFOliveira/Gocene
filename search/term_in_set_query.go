package search

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermInSetQuery is a specialization for a disjunction over many terms.
// It behaves like a ConstantScoreQuery over a BooleanQuery containing only SHOULD clauses.
type TermInSetQuery struct {
	*MultiTermQuery
	field            string
	termData         *index.PrefixCodedTerms
	termDataHashCode int
}

// NewTermInSetQuery creates a new TermInSetQuery from the given collection of terms.
func NewTermInSetQuery(field string, terms []*util.BytesRef) *TermInSetQuery {
	return NewTermInSetQueryWithRewrite(ConstantScoreBlendedRewrite, field, terms)
}

// NewTermInSetQueryWithRewrite creates a new TermInSetQuery from the given collection of terms.
func NewTermInSetQueryWithRewrite(rewriteMethod RewriteMethod, field string, terms []*util.BytesRef) *TermInSetQuery {
	packed := packTerms(field, terms)

	q := &TermInSetQuery{
		MultiTermQuery:   NewMultiTermQuery(field, rewriteMethod, nil),
		field:            field,
		termData:         packed,
		termDataHashCode: computePrefixCodedTermsHash(packed),
	}

	q.MultiTermQuery.termsEnumFunc = func(t index.Terms) (index.TermsEnum, error) {
		return q.getTermsEnum(t, nil)
	}

	return q
}

// NewIndexOrDocValuesQuery creates a new IndexOrDocValuesQuery combining two TermInSetQuery.
func NewIndexOrDocValuesQuery(indexRewriteMethod RewriteMethod, field string, terms []*util.BytesRef) *IndexOrDocValuesQuery {
	packed := packTerms(field, terms)

	indexQuery := newTermInSetQueryFromPacked(indexRewriteMethod, field, packed)
	dvQuery := newTermInSetQueryFromPacked(DocValuesRewrite, field, packed)

	return &IndexOrDocValuesQuery{
		IndexQuery: indexQuery,
		DVQuery:    dvQuery,
	}
}

func newTermInSetQueryFromPacked(rewrite RewriteMethod, field string, packed *index.PrefixCodedTerms) *TermInSetQuery {
	q := &TermInSetQuery{
		MultiTermQuery:   NewMultiTermQuery(field, rewrite, nil),
		field:            field,
		termData:         packed,
		termDataHashCode: computePrefixCodedTermsHash(packed),
	}

	q.MultiTermQuery.termsEnumFunc = func(t index.Terms) (index.TermsEnum, error) {
		return q.getTermsEnum(t, nil)
	}

	return q
}

func packTerms(field string, terms []*util.BytesRef) *index.PrefixCodedTerms {
	if len(terms) == 0 {
		return &index.PrefixCodedTerms{}
	}

	sortedTerms := make([]*util.BytesRef, len(terms))
	copy(sortedTerms, terms)
	sort.Slice(sortedTerms, func(i, j int) bool {
		return util.BytesRefCompare(sortedTerms[i], sortedTerms[j]) < 0
	})

	builder := index.NewPrefixCodedTermsBuilder()
	var previous *util.BytesRef
	for _, term := range sortedTerms {
		if previous != nil && util.BytesRefEquals(previous, term) {
			continue // deduplicate
		}

		t := schema.NewTerm(field, term.ValidBytes())
		builder.Add(t)
		previous = term
	}

	return builder.Finish()
}

func (q *TermInSetQuery) GetTermsCount() int64 {
	return q.termData.Size()
}

func (q *TermInSetQuery) GetBytesRefIterator() util.BytesRefIterator {
	iterator := q.termData.Iterator()
	return func() (*util.BytesRef, error) {
		bytes := iterator.Next()
		if bytes == nil {
			return nil, nil
		}
		return util.NewBytesRef(bytes), nil
	}
}

func (q *TermInSetQuery) Visit(visitor QueryVisitor) {
	if !visitor.AcceptField(q.field) {
		return
	}
	if q.termData.Size() == 1 {
		termBytes := q.termData.Iterator().Next()
		visitor.ConsumeTerms(q, schema.NewTerm(q.field, termBytes))
	}
	if q.termData.Size() > 1 {
		visitor.ConsumeTermsMatching(q, q.field, q.asByteRunAutomaton)
	}
}

func (q *TermInSetQuery) asByteRunAutomaton() util.ByteRunAutomaton {
	// In Lucene, this uses Automata.makeBinaryStringUnion(termData.iterator())
	// and returns a ByteRunAutomaton.
	// This is currently not implemented in Gocene.
	return nil
}

func (q *TermInSetQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*TermInSetQuery); ok {
		return q.termDataHashCode == otherQuery.termDataHashCode &&
			prefixCodedTermsEquals(q.termData, otherQuery.termData)
	}
	return false
}

func (q *TermInSetQuery) HashCode() int {
	return 31*12345 + q.termDataHashCode
}

func (q *TermInSetQuery) ToString(defaultField string) string {
	var builder strings.Builder
	builder.WriteString(q.field)
	builder.WriteString(":(")

	iterator := q.termData.Iterator()
	first := true
	for term := iterator.Next(); term != nil; term = iterator.Next() {
		if !first {
			builder.WriteByte(' ')
		}
		first = false
		builder.WriteString(string(term))
	}
	builder.WriteString(")")

	return builder.String()
}

func (q *TermInSetQuery) RamBytesUsed() int64 {
	// 32 is a rough estimate for BASE_RAM_BYTES_USED
	return 32 + q.termData.ramBytesUsed()
}

func (q *TermInSetQuery) GetChildResources() []util.Accountable {
	return nil
}

func (q *TermInSetQuery) getTermsEnum(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	se := &setEnum{
		query:    q,
		iterator: q.termData.Iterator(),
	}
	se.seekTerm = se.iterator.Next()
	se.FilteredTermsEnum = index.NewFilteredTermsEnum(terms.Iterator(), se)
	return se, nil
}

type setEnum struct {
	*index.FilteredTermsEnum
	query    *TermInSetQuery
	iterator *index.PrefixCodedTermIterator
	seekTerm []byte
}

func (s *setEnum) Accept(term *schema.Term) (index.AcceptStatus, error) {
	termBytes := term.Text()

	var cmp int
	for s.seekTerm != nil {
		cmp = bytes.Compare(s.seekTerm, termBytes)
		if cmp >= 0 {
			break
		}
		s.seekTerm = s.iterator.Next()
	}

	if s.seekTerm == nil {
		return index.AcceptEnd, nil
	} else if cmp == 0 {
		return index.AcceptYesAndSeek, nil
	} else {
		return index.AcceptNoAndSeek, nil
	}
}

func (s *setEnum) NextSeekTerm(current *schema.Term) (*schema.Term, error) {
	if current == nil {
		return schema.NewTerm(s.query.field, s.seekTerm), nil
	}
	currentBytes := current.Text()
	for s.seekTerm != nil && bytes.Compare(s.seekTerm, currentBytes) <= 0 {
		s.seekTerm = s.iterator.Next()
	}
	if s.seekTerm == nil {
		return nil, nil
	}
	return schema.NewTerm(s.query.field, s.seekTerm), nil
}

func computePrefixCodedTermsHash(p *index.PrefixCodedTerms) int {
	h := 0
	it := p.Iterator()
	for term := it.Next(); term != nil; term = it.Next() {
		h = 31*h + util.MurmurHash3_x86_32_Bytes(term, util.GoodFastHashSeed)
	}
	return h
}

func prefixCodedTermsEquals(a, b *index.PrefixCodedTerms) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Size() != b.Size() {
		return false
	}
	itA := a.Iterator()
	itB := b.Iterator()
	for {
		termA := itA.Next()
		termB := itB.Next()
		if termA == nil || termB == nil {
			return termA == termB
		}
		if !bytes.Equal(termA, termB) {
			return false
		}
	}
}

func (p *index.PrefixCodedTerms) ramBytesUsed() int64 {
	return p.Size() * 16
}
