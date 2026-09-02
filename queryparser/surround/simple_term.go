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

	// Visit enumerates matching terms in the index and adds them to the visitor.
	Visit(visitor *MatchingTermVisitor, reader search.IndexReader, field string) error

	// WrapWithBoost wraps the query with the term's boost.
	WrapWithBoost(q search.Query) search.Query
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
	query, err := factory.MakeBasicTermQuery(field, q.termText)
	if err != nil {
		return nil, err
	}
	return q.WrapWithBoost(query), nil
}

func (q *SrndTermQuery) Visit(visitor *MatchingTermVisitor, reader search.IndexReader, field string) error {
	terms, err := reader.Terms(field)
	if err != nil {
		return err
	}
	iter, err := terms.GetIteratorWithSeek(index.NewTerm(field, q.termText))
	if err != nil {
		return err
	}
	if iter != nil {
		term, err := iter.Next()
		if err != nil {
			return err
		}
		if term != nil && term.Text() == q.termText {
			visitor.AddTerm(*term)
		}
	}
	return nil
}

func (q *SrndTermQuery) String() string {
	var sb strings.Builder
	if q.quoted {
		sb.WriteByte('"')
	}
	sb.WriteString(q.termText)
	if q.quoted {
		sb.WriteByte('"')
	}
	q.WeightToString(&sb)
	return sb.String()
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
	query := search.NewPrefixQuery(index.NewTerm(field, q.prefix))
	return q.WrapWithBoost(query), nil
}

func (q *SrndPrefixQuery) String() string {
	var sb strings.Builder
	if q.quoted {
		sb.WriteByte('"')
	}
	sb.WriteString(q.prefix)
	if q.quoted {
		sb.WriteByte('"')
	}
	sb.WriteRune(q.truncator)
	q.WeightToString(&sb)
	return sb.String()
}

func (q *SrndPrefixQuery) Visit(visitor *MatchingTermVisitor, reader search.IndexReader, field string) error {
	terms, err := reader.Terms(field)
	if err != nil {
		return err
	}
	iter, err := terms.GetIteratorWithSeek(index.NewTerm(field, q.prefix))
	if err != nil {
		return err
	}
	if iter == nil {
		return nil
	}
	for {
		term, err := iter.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		if !term.StartsWith(q.prefix) {
			break
		}
		visitor.AddTerm(*term)
	}
	return nil
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
	query := search.NewWildcardQuery(index.NewTerm(field, pattern))
	return q.WrapWithBoost(query), nil
}

func (q *SrndTruncQuery) String() string {
	var sb strings.Builder
	sb.WriteString(q.truncated)
	q.WeightToString(&sb)
	return sb.String()
}

// AddSpanQueries queues the truncation pattern; a future enhancement may
// resolve it against the IndexReader to enumerate concrete terms.
func (q *SrndTruncQuery) AddSpanQueries(factory *SpanNearClauseFactory) error {
	return factory.AddTermWeighted(q.truncated, q.GetWeight())
}

func (q *SrndTruncQuery) Visit(visitor *MatchingTermVisitor, reader search.IndexReader, field string) error {
	terms, err := reader.Terms(field)
	if err != nil {
		return err
	}
	iter, err := terms.GetIterator()
	if err != nil {
		return err
	}
	if iter == nil {
		return nil
	}
	for {
		term, err := iter.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		if matchesWildcard(term.Text(), q.truncated, q.truncator, q.anyChar) {
			visitor.AddTerm(*term)
		}
	}
	return nil
}

var _ SimpleTerm = (*SrndTruncQuery)(nil)

func matchesWildcard(text string, pattern string, truncator, anyChar rune) bool {
	t := []rune(text)
	p := []rune(pattern)

	n, m := len(t), len(p)
	dp := make([][]bool, n+1)
	for i := range dp {
		dp[i] = make([]bool, m+1)
	}

	dp[0][0] = true
	for j := 1; j <= m; j++ {
		if p[j-1] == truncator {
			dp[0][j] = dp[0][j-1]
		}
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if p[j-1] == truncator {
				dp[i][j] = dp[i-1][j] || dp[i][j-1]
			} else if p[j-1] == anyChar {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = dp[i-1][j-1] && t[i-1] == p[j-1]
			}
		}
	}
	return dp[n][m]
}

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
