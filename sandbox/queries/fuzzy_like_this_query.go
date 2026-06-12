// Package queries implements org.apache.lucene.sandbox.queries.
package queries

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FuzzyLikeThisQuery synthesises a Boolean query of FuzzyQuery clauses for
// the supplied terms. Mirrors
// org.apache.lucene.sandbox.queries.FuzzyLikeThisQuery.
type FuzzyLikeThisQuery struct {
	*search.BaseQuery
	FieldVals   []FieldVals
	Analyzer    analysis.Analyzer
	MaxNumTerms int
	IgnoreTF    bool
}

// FieldVals holds the per-field configuration for a FuzzyLikeThisQuery.
type FieldVals struct {
	FieldName    string
	MaxEdits     int
	PrefixLength int
	QueryString  string
}

// NewFuzzyLikeThisQuery builds the query.
func NewFuzzyLikeThisQuery(maxNumTerms int, analyzer analysis.Analyzer) *FuzzyLikeThisQuery {
	return &FuzzyLikeThisQuery{
		BaseQuery:   &search.BaseQuery{},
		MaxNumTerms: maxNumTerms,
		Analyzer:    analyzer,
	}
}

// AddTerms registers a (field, queryString) tuple with fuzzy parameters.
// minSimilarity must be an integer value between 0 and 2 (inclusive).
func (q *FuzzyLikeThisQuery) AddTerms(queryString, fieldName string, minSimilarity float32, prefixLength int) {
	maxEdits := int(minSimilarity)
	if float32(maxEdits) != minSimilarity || maxEdits < 0 || maxEdits > 2 {
		panic(fmt.Sprintf("minSimilarity must be integer value between 0 and 2, inclusive; got %f", minSimilarity))
	}
	q.FieldVals = append(q.FieldVals, FieldVals{
		FieldName:    fieldName,
		MaxEdits:     maxEdits,
		PrefixLength: prefixLength,
		QueryString:  queryString,
	})
}

// SetIgnoreTF sets whether term frequency should be ignored.
func (q *FuzzyLikeThisQuery) SetIgnoreTF(ignoreTF bool) {
	q.IgnoreTF = ignoreTF
}

// IsIgnoreTF returns true if term frequency is ignored.
func (q *FuzzyLikeThisQuery) IsIgnoreTF() bool {
	return q.IgnoreTF
}

// Rewrite expands this query into a BooleanQuery of FuzzyQuery clauses.
func (q *FuzzyLikeThisQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	bq := search.NewBooleanQuery()
	globalSeen := make(map[string]bool)

	for _, f := range q.FieldVals {
		if f.QueryString == "" {
			continue
		}
		// Skip fields that do not exist in the reader, mirroring Java's
		// MultiTerms.getTerms null check.
		if tp, ok := reader.(interface{ Terms(field string) (index.Terms, error) }); ok {
			terms, err := tp.Terms(f.FieldName)
			if err != nil {
				return nil, err
			}
			if terms == nil {
				continue
			}
		}
		stream, err := q.Analyzer.TokenStream(f.FieldName, strings.NewReader(f.QueryString))
		if err != nil {
			return nil, err
		}
		if stream == nil {
			continue
		}

		// Reset the stream if supported.
		if r, ok := stream.(interface{ Reset() error }); ok {
			if err := r.Reset(); err != nil {
				return nil, err
			}
		}

		var attrSrc *util.AttributeSource
		if s, ok := stream.(interface{ GetAttributeSource() *util.AttributeSource }); ok {
			attrSrc = s.GetAttributeSource()
		}
		if attrSrc == nil {
			continue
		}

		termAttrImpl := attrSrc.GetAttribute(analysis.CharTermAttributeType)
		if termAttrImpl == nil {
			continue
		}
		termAttr, ok := termAttrImpl.(analysis.CharTermAttribute)
		if !ok {
			continue
		}

		for {
			hasToken, err := stream.IncrementToken()
			if err != nil {
				return nil, err
			}
			if !hasToken {
				break
			}

			term := termAttr.String()
			if term == "" {
				continue
			}

			// Deduplicate globally per (field, term) pair.
			key := f.FieldName + ":" + term
			if globalSeen[key] {
				continue
			}
			globalSeen[key] = true

			var clause search.Query = search.NewFuzzyQueryFull(
				index.NewTerm(f.FieldName, term),
				f.MaxEdits,
				f.PrefixLength,
				50,
				true,
			)
			if q.IgnoreTF {
				clause = search.NewConstantScoreQuery(clause)
			}
			bq.Add(clause, search.SHOULD)
		}

		if e, ok := stream.(interface{ End() error }); ok {
			_ = e.End()
		}
		if c, ok := stream.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}

	if len(bq.Clauses()) == 0 {
		return search.NewMatchNoDocsQuery(), nil
	}
	if len(bq.Clauses()) == 1 {
		return bq.Clauses()[0].Query, nil
	}
	return bq, nil
}

// Clone creates a copy of this query.
func (q *FuzzyLikeThisQuery) Clone() search.Query {
	clonedFieldVals := make([]FieldVals, len(q.FieldVals))
	copy(clonedFieldVals, q.FieldVals)
	return &FuzzyLikeThisQuery{
		BaseQuery:   &search.BaseQuery{},
		FieldVals:   clonedFieldVals,
		Analyzer:    q.Analyzer,
		MaxNumTerms: q.MaxNumTerms,
		IgnoreTF:    q.IgnoreTF,
	}
}

// Equals checks if this query equals another.
func (q *FuzzyLikeThisQuery) Equals(other search.Query) bool {
	o, ok := other.(*FuzzyLikeThisQuery)
	if !ok {
		return false
	}
	if q.MaxNumTerms != o.MaxNumTerms || q.IgnoreTF != o.IgnoreTF {
		return false
	}
	if len(q.FieldVals) != len(o.FieldVals) {
		return false
	}
	if q.Analyzer != o.Analyzer {
		return false
	}
	for i := range q.FieldVals {
		if q.FieldVals[i] != o.FieldVals[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *FuzzyLikeThisQuery) HashCode() int {
	hash := q.MaxNumTerms
	if q.IgnoreTF {
		hash = hash*31 + 1
	}
	for _, f := range q.FieldVals {
		hash = hash*31 + stringHash(f.FieldName)
		hash = hash*31 + f.MaxEdits
		hash = hash*31 + f.PrefixLength
		hash = hash*31 + stringHash(f.QueryString)
	}
	return hash
}

func stringHash(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = h*31 + int(s[i])
	}
	return h
}

// String returns a textual representation of this query.
func (q *FuzzyLikeThisQuery) String() string {
	var sb strings.Builder
	sb.WriteString("FuzzyLikeThisQuery(")
	for i, f := range q.FieldVals {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s:%s~%d/%d", f.FieldName, f.QueryString, f.MaxEdits, f.PrefixLength))
	}
	sb.WriteString(")")
	return sb.String()
}

// Visit dispatches to the visitor.
func (q *FuzzyLikeThisQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

// CreateWeight creates a Weight for this query by rewriting first.
func (q *FuzzyLikeThisQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	reader := searcher.GetReader()
	rewritten, err := q.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, needsScores, boost)
}

// Ensure FuzzyLikeThisQuery implements search.Query.
var _ search.Query = (*FuzzyLikeThisQuery)(nil)
