package search

import (
	"bytes"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
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
		MultiTermQuery:   NewMultiTermQuery(field, rewriteMethod),
		field:            field,
		termData:         packed,
		termDataHashCode: computePrefixCodedTermsHash(packed),
	}
	q.MultiTermQuery.SetOwner(q)

	return q
}

// NewTermInSetIndexOrDocValuesQuery creates a new IndexOrDocValuesQuery
// combining two TermInSetQuery with the same terms.
//
// Mirrors the static factory TermInSetQuery.newIndexOrDocValuesQuery. It is a
// different artefact from the IndexOrDocValuesQuery constructor, which Go's
// flat package namespace would otherwise collide with, so it carries its
// declaring class in the name.
func NewTermInSetIndexOrDocValuesQuery(indexRewriteMethod RewriteMethod, field string, terms []*util.BytesRef) *IndexOrDocValuesQuery {
	packed := packTerms(field, terms)

	indexQuery := newTermInSetQueryFromPacked(indexRewriteMethod, field, packed)
	dvQuery := newTermInSetQueryFromPacked(DefaultDocValuesRewriteMethod, field, packed)

	return NewIndexOrDocValuesQuery(indexQuery, dvQuery)
}

func newTermInSetQueryFromPacked(rewrite RewriteMethod, field string, packed *index.PrefixCodedTerms) *TermInSetQuery {
	q := &TermInSetQuery{
		MultiTermQuery:   NewMultiTermQuery(field, rewrite),
		field:            field,
		termData:         packed,
		termDataHashCode: computePrefixCodedTermsHash(packed),
	}
	q.MultiTermQuery.SetOwner(q)

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

		t := spi.NewTermFromBytes(field, term.ValidBytes())
		builder.Add(t)
		previous = term
	}

	return builder.Finish()
}

func (q *TermInSetQuery) GetTermsCount() int64 {
	return q.termData.Size()
}

// GetBytesRefIterator returns an iterator over the encoded terms for query
// inspection.
//
// Mirrors TermInSetQuery.getBytesRefIterator(), whose body is
// `final TermIterator iterator = this.termData.iterator(); return () -> iterator.next();`.
func (q *TermInSetQuery) GetBytesRefIterator() util.BytesRefIterator {
	iterator := q.termData.Iterator()
	return util.BytesRefIteratorFunc(func() (*util.BytesRef, error) {
		b := iterator.Next()
		if b == nil {
			return nil, nil
		}
		return util.NewBytesRef(b), nil
	})
}

func (q *TermInSetQuery) Visit(visitor QueryVisitor) {
	if !visitor.AcceptField(q.field) {
		return
	}
	if q.termData.Size() == 1 {
		termBytes := q.termData.Iterator().Next()
		visitor.ConsumeTerms(q, spi.NewTermFromBytes(q.field, termBytes))
	}
	if q.termData.Size() > 1 {
		visitor.ConsumeTermsMatching(q, q.field, q.asByteRunAutomaton)
	}
}

// asByteRunAutomaton builds a binary ByteRunAutomaton accepting exactly the
// query's terms.
//
// Mirrors TermInSetQuery.asByteRunAutomaton(), whose body builds
// Automata.makeBinaryStringUnion(termData.iterator()) and returns
// new ByteRunAutomaton(a, true), wrapped in a catch that rethrows the
// IOException as UncheckedIOException because termData.iterator() never throws.
//
// PORT NOTE. util/automaton does not expose the Automata facade method
// MakeBinaryStringUnion; its body, StringsToAutomaton.build(BytesRefIterator,
// true), is available as automaton.BuildStringUnionFromIterator, and is what is
// called here. The empty-input branch of Automata.makeBinaryStringUnion does
// not apply: this method is only reached when termData.Size() > 1.
func (q *TermInSetQuery) asByteRunAutomaton() ByteRunAutomaton {
	a, err := automaton.BuildStringUnionFromIterator(q.GetBytesRefIterator(), true)
	if err != nil {
		// Shouldn't happen since GetBytesRefIterator provides an iterator
		// implementation that never fails.
		panic(err)
	}
	return byteRunAutomatonAdapter{a: automaton.NewByteRunAutomatonBinary(a, true)}
}

// byteRunAutomatonAdapter adapts *automaton.ByteRunAutomaton, whose Run carries
// the offset/length parameters of Java's ByteRunAutomaton.run(byte[], int, int),
// to the narrower search.ByteRunAutomaton contract that QueryVisitor consumes.
type byteRunAutomatonAdapter struct {
	a *automaton.ByteRunAutomaton
}

// Run reports whether the automaton accepts the whole of input.
func (b byteRunAutomatonAdapter) Run(input []byte) bool {
	return b.a.Run(input, 0, len(input))
}

func (q *TermInSetQuery) Equals(other spi.Query) bool {
	if otherQuery, ok := other.(*TermInSetQuery); ok {
		// no need to check 'field' explicitly since it is encoded in 'termData'
		// termData might be heavy to compare so check the hash code first
		return q.termDataHashCode == otherQuery.termDataHashCode &&
			q.termData.Equals(otherQuery.termData)
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

// termInSetQueryBaseRAMBytesUsed is a rough estimate of
// RamUsageEstimator.shallowSizeOfInstance(TermInSetQuery.class).
//
// Mirrors TermInSetQuery.BASE_RAM_BYTES_USED.
const termInSetQueryBaseRAMBytesUsed int64 = 32

// RamBytesUsed mirrors TermInSetQuery.ramBytesUsed(), which is
// BASE_RAM_BYTES_USED + termData.ramBytesUsed().
//
// PORT NOTE. index.PrefixCodedTerms does not expose ramBytesUsed(), so the
// term-content term is estimated here.
func (q *TermInSetQuery) RamBytesUsed() int64 {
	return termInSetQueryBaseRAMBytesUsed + prefixCodedTermsRAMBytesUsed(q.termData)
}

// prefixCodedTermsRAMBytesUsed estimates PrefixCodedTerms.ramBytesUsed().
func prefixCodedTermsRAMBytesUsed(p *index.PrefixCodedTerms) int64 {
	if p == nil {
		return 0
	}
	return p.Size() * 16
}

func (q *TermInSetQuery) GetChildResources() []util.Accountable {
	return nil
}

func (q *TermInSetQuery) GetTermsEnumWithAttributes(terms index.Terms, atts *util.AttributeSource) (index.TermsEnum, error) {
	se := &setEnum{
		query:    q,
		iterator: q.termData.Iterator(),
	}
	se.seekTerm = se.iterator.Next()
	it, err := terms.Iterator()
	if err != nil {
		return nil, err
	}
	se.FilteredTermsEnum = index.NewFilteredTermsEnum(it, se)
	return se, nil
}

type setEnum struct {
	*index.FilteredTermsEnum
	query    *TermInSetQuery
	iterator *index.PrefixCodedTermIterator
	seekTerm []byte
}

// Accept next()s the encoded-term iterator until it is >= the incoming term; an
// exact match is a hit, otherwise it is a miss.
//
// Mirrors SetEnum.accept(BytesRef).
func (s *setEnum) Accept(term *spi.Term) (index.AcceptStatus, error) {
	termBytes := term.Bytes.ValidBytes()

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

// NextSeekTerm next()s the encoded-term iterator until it is > currentTerm; it
// must always make progress.
//
// Mirrors SetEnum.nextSeekTerm(BytesRef).
func (s *setEnum) NextSeekTerm(current *spi.Term) (*spi.Term, error) {
	if current == nil {
		if s.seekTerm == nil {
			return nil, nil
		}
		return spi.NewTermFromBytes(s.query.field, s.seekTerm), nil
	}
	currentBytes := current.Bytes.ValidBytes()
	for s.seekTerm != nil && bytes.Compare(s.seekTerm, currentBytes) <= 0 {
		s.seekTerm = s.iterator.Next()
	}
	if s.seekTerm == nil {
		return nil, nil
	}
	return spi.NewTermFromBytes(s.query.field, s.seekTerm), nil
}

func computePrefixCodedTermsHash(p *index.PrefixCodedTerms) int {
	h := 0
	it := p.Iterator()
	for term := it.Next(); term != nil; term = it.Next() {
		h = 31*h + util.MurmurHash3_x86_32(term, 0, len(term), util.GoodFastHashSeed)
	}
	return h
}

// Compile-time assertion that TermInSetQuery supplies the abstract
// MultiTermQuery#getTermsEnum(Terms, AttributeSource) body.
var _ MultiTermQueryOwner = (*TermInSetQuery)(nil)
