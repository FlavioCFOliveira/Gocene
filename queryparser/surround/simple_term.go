package surround

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SimpleTerm is the marker interface implemented by surround nodes that
// represent a single (possibly wildcard) term. It mirrors
// org.apache.lucene.queryparser.surround.query.SimpleTerm.
type SimpleTerm interface {
	SrndQuery
	DistanceSubQuery

	// GetTermText returns the original term text as parsed (including any
	// trailing truncation/prefix sentinel).
	GetTermText() string

	// IsQuoted reports whether the term was quoted in the source query.
	IsQuoted() bool
}

// SrndTermQuery is the surround node for an exact term. Mirrors
// org.apache.lucene.queryparser.surround.query.SrndTermQuery.
type SrndTermQuery struct {
	SrndQueryBase
	termText string
	quoted   bool
}

// NewSrndTermQuery builds a term query node.
func NewSrndTermQuery(termText string, quoted bool) *SrndTermQuery {
	return &SrndTermQuery{termText: termText, quoted: quoted}
}

func (q *SrndTermQuery) GetTermText() string { return q.termText }
func (q *SrndTermQuery) IsQuoted() bool      { return q.quoted }
func (q *SrndTermQuery) DistanceSubQueryNotAllowed() string {
	return ""
}

// MakeLuceneQueryField produces a TermQuery on the supplied field.
func (q *SrndTermQuery) MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error) {
	return factory.MakeBasicTermQuery(field, q.termText)
}

// AddSpanQueries adds a single SpanTermQuery to the factory.
func (q *SrndTermQuery) AddSpanQueries(factory *SpanNearClauseFactory) error {
	return factory.AddTermWeighted(q.termText, q.GetWeight())
}

var _ SimpleTerm = (*SrndTermQuery)(nil)

// SrndPrefixQuery is the surround node for `prefix*` queries. Mirrors
// org.apache.lucene.queryparser.surround.query.SrndPrefixQuery.
type SrndPrefixQuery struct {
	SrndQueryBase
	prefix    string
	truncator rune
	quoted    bool
}

// NewSrndPrefixQuery builds a prefix-truncated term query.
func NewSrndPrefixQuery(prefix string, quoted bool, truncator rune) *SrndPrefixQuery {
	return &SrndPrefixQuery{prefix: prefix, truncator: truncator, quoted: quoted}
}

func (q *SrndPrefixQuery) GetPrefix() string   { return q.prefix }
func (q *SrndPrefixQuery) GetTruncator() rune  { return q.truncator }
func (q *SrndPrefixQuery) GetTermText() string { return q.prefix }
func (q *SrndPrefixQuery) IsQuoted() bool      { return q.quoted }
func (q *SrndPrefixQuery) DistanceSubQueryNotAllowed() string {
	return ""
}

// MakeLuceneQueryField expands the prefix into the corresponding PrefixQuery.
func (q *SrndPrefixQuery) MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error) {
	if err := factory.tickBasicQueryBudget(); err != nil {
		return nil, err
	}
	return search.NewPrefixQuery(index.NewTerm(field, q.prefix)), nil
}

// AddSpanQueries enumerates the matching terms via the BasicQueryFactory and
// records each as a span-term clause. The current Go port does not perform a
// reader-side enumeration; the prefix itself is queued so callers that resolve
// against an IndexReader can expand it. Callers may also choose to inject the
// expanded terms directly.
func (q *SrndPrefixQuery) AddSpanQueries(factory *SpanNearClauseFactory) error {
	return factory.AddTermWeighted(q.prefix, q.GetWeight())
}

var _ SimpleTerm = (*SrndPrefixQuery)(nil)

// SrndTruncQuery is the surround node for wildcard queries (e.g. `f?o*bar`).
// Mirrors org.apache.lucene.queryparser.surround.query.SrndTruncQuery.
type SrndTruncQuery struct {
	SrndQueryBase
	truncated string
	truncator rune
	anyChar   rune
}

// NewSrndTruncQuery builds a truncated wildcard query.
func NewSrndTruncQuery(truncated string, truncator, anyChar rune) *SrndTruncQuery {
	return &SrndTruncQuery{truncated: truncated, truncator: truncator, anyChar: anyChar}
}

func (q *SrndTruncQuery) GetTruncated() string { return q.truncated }
func (q *SrndTruncQuery) GetTruncator() rune   { return q.truncator }
func (q *SrndTruncQuery) GetAnyChar() rune     { return q.anyChar }
func (q *SrndTruncQuery) GetTermText() string  { return q.truncated }
func (q *SrndTruncQuery) IsQuoted() bool       { return false }
func (q *SrndTruncQuery) DistanceSubQueryNotAllowed() string {
	return ""
}

// MakeLuceneQueryField builds the corresponding WildcardQuery, mapping the
// surround truncators to Lucene's '*' / '?' tokens.
func (q *SrndTruncQuery) MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error) {
	if err := factory.tickBasicQueryBudget(); err != nil {
		return nil, err
	}
	pattern := convertWildcardPattern(q.truncated, q.truncator, q.anyChar)
	return search.NewWildcardQuery(index.NewTerm(field, pattern)), nil
}

// AddSpanQueries queues the truncation pattern; a future enhancement may
// resolve it against the IndexReader to enumerate concrete terms.
func (q *SrndTruncQuery) AddSpanQueries(factory *SpanNearClauseFactory) error {
	return factory.AddTermWeighted(q.truncated, q.GetWeight())
}

var _ SimpleTerm = (*SrndTruncQuery)(nil)

func convertWildcardPattern(s string, truncator, anyChar rune) string {
	if truncator == '*' && anyChar == '?' {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case truncator:
			b.WriteRune('*')
		case anyChar:
			b.WriteRune('?')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
