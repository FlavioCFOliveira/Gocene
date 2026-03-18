// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Ensure QueryParserBase implements the base functionality for query parsers

// QueryParserBase is the base class for query parsers.
// This is the Go port of Lucene's org.apache.lucene.queryparser.QueryParserBase.
type QueryParserBase struct {
	// The default field for query terms
	defaultField string

	// The analyzer to use for tokenizing query text
	analyzer analysis.Analyzer

	// Configuration flags
	allowLeadingWildcard     bool
	enablePositionIncrements bool
	lowercaseExpandedTerms   bool
	allowFuzzyAndWildcard    bool

	// Default values
	fuzzyMinSim       float64
	fuzzyPrefixLength int
	phraseSlop        int

	// Locale and timezone for date parsing
	locale   string
	timeZone string
}

// NewQueryParserBase creates a new QueryParserBase.
func NewQueryParserBase(defaultField string, analyzer analysis.Analyzer) *QueryParserBase {
	return &QueryParserBase{
		defaultField:             defaultField,
		analyzer:                 analyzer,
		allowLeadingWildcard:     false,
		enablePositionIncrements: true,
		lowercaseExpandedTerms:   true,
		allowFuzzyAndWildcard:    true,
		fuzzyMinSim:              2.0,
		fuzzyPrefixLength:        0,
		phraseSlop:               0,
		locale:                   "en",
		timeZone:                 "UTC",
	}
}

// GetFieldQuery creates a query for the specified field and term.
func (qpb *QueryParserBase) GetFieldQuery(field, term string) (search.Query, error) {
	return qpb.newTermQuery(field, term), nil
}

// GetRangeQuery creates a range query for the specified field.
func (qpb *QueryParserBase) GetRangeQuery(field, lower, upper string, includeLower, includeUpper bool) (search.Query, error) {
	return search.NewTermRangeQuery(field, []byte(lower), []byte(upper), includeLower, includeUpper), nil
}

// GetWildcardQuery creates a wildcard query for the specified field and pattern.
func (qpb *QueryParserBase) GetWildcardQuery(field, pattern string) (search.Query, error) {
	if !qpb.allowLeadingWildcard {
		if strings.HasPrefix(pattern, "*") || strings.HasPrefix(pattern, "?") {
			return nil, fmt.Errorf("leading wildcard not allowed: %s", pattern)
		}
	}

	term := index.NewTerm(field, pattern)
	return search.NewWildcardQuery(term), nil
}

// GetFuzzyQuery creates a fuzzy query for the specified field and term.
func (qpb *QueryParserBase) GetFuzzyQuery(field, term string, minSimilarity float64) (search.Query, error) {
	if !qpb.allowFuzzyAndWildcard {
		return nil, fmt.Errorf("fuzzy queries not allowed")
	}

	maxEdits := int(minSimilarity)
	if maxEdits < 1 {
		maxEdits = 2
	}
	if maxEdits > 2 {
		maxEdits = 2
	}

	return search.NewFuzzyQueryWithMaxEdits(index.NewTerm(field, term), maxEdits), nil
}

// GetPrefixQuery creates a prefix query for the specified field and prefix.
func (qpb *QueryParserBase) GetPrefixQuery(field, prefix string) (search.Query, error) {
	return search.NewPrefixQuery(index.NewTerm(field, prefix)), nil
}

// GetBooleanQuery creates a boolean query from a list of clauses.
func (qpb *QueryParserBase) GetBooleanQuery(clauses []*search.BooleanClause) (search.Query, error) {
	if len(clauses) == 0 {
		return nil, fmt.Errorf("empty boolean query")
	}

	bq := search.NewBooleanQuery()
	for _, clause := range clauses {
		bq.Add(clause.Query, clause.Occur)
	}
	return bq, nil
}

// GetPhraseQuery creates a phrase query for the specified field and terms.
func (qpb *QueryParserBase) GetPhraseQuery(field string, terms []string) (search.Query, error) {
	if len(terms) == 0 {
		return nil, fmt.Errorf("empty phrase query")
	}

	termPtrs := make([]*index.Term, len(terms))
	for i, text := range terms {
		termPtrs[i] = index.NewTerm(field, text)
	}

	return search.NewPhraseQueryWithSlop(qpb.phraseSlop, field, termPtrs...), nil
}

// GetMatchAllDocsQuery returns a query that matches all documents.
func (qpb *QueryParserBase) GetMatchAllDocsQuery() search.Query {
	return search.NewMatchAllDocsQuery()
}

// GetMatchNoDocsQuery returns a query that matches no documents.
func (qpb *QueryParserBase) GetMatchNoDocsQuery() search.Query {
	return search.NewMatchNoDocsQuery()
}

// Analyze analyzes the given text using the configured analyzer.
func (qpb *QueryParserBase) Analyze(field, text string) []string {
	if qpb.analyzer == nil {
		return []string{text}
	}

	// Create a token stream from the text
	reader := strings.NewReader(text)
	tokenStream, err := qpb.analyzer.TokenStream(field, reader)
	if err != nil {
		return []string{text}
	}
	defer tokenStream.Close()

	var terms []string
	for {
		hasToken, err := tokenStream.IncrementToken()
		if err != nil {
			return []string{text}
		}
		if !hasToken {
			break
		}

		// Get the term attribute using the attribute source
		if baseTs, ok := tokenStream.(*analysis.BaseTokenStream); ok {
			attrSource := baseTs.GetAttributeSource()
			if termAttr := attrSource.GetAttribute("CharTermAttribute"); termAttr != nil {
				if cta, ok := termAttr.(analysis.CharTermAttribute); ok {
					term := cta.String()
					if term != "" {
						terms = append(terms, term)
					}
				}
			}
		}
	}

	return terms
}

// NewTermQuery creates a new term query.
func (qpb *QueryParserBase) newTermQuery(field, text string) search.Query {
	term := index.NewTerm(field, text)
	return search.NewTermQuery(term)
}

// GetDefaultField returns the default field.
func (qpb *QueryParserBase) GetDefaultField() string {
	return qpb.defaultField
}

// SetDefaultField sets the default field.
func (qpb *QueryParserBase) SetDefaultField(field string) {
	qpb.defaultField = field
}

// GetAnalyzer returns the analyzer.
func (qpb *QueryParserBase) GetAnalyzer() analysis.Analyzer {
	return qpb.analyzer
}

// SetAnalyzer sets the analyzer.
func (qpb *QueryParserBase) SetAnalyzer(analyzer analysis.Analyzer) {
	qpb.analyzer = analyzer
}

// GetAllowLeadingWildcard returns whether leading wildcards are allowed.
func (qpb *QueryParserBase) GetAllowLeadingWildcard() bool {
	return qpb.allowLeadingWildcard
}

// SetAllowLeadingWildcard sets whether leading wildcards are allowed.
func (qpb *QueryParserBase) SetAllowLeadingWildcard(allow bool) {
	qpb.allowLeadingWildcard = allow
}

// GetEnablePositionIncrements returns whether position increments are enabled.
func (qpb *QueryParserBase) GetEnablePositionIncrements() bool {
	return qpb.enablePositionIncrements
}

// SetEnablePositionIncrements sets whether position increments are enabled.
func (qpb *QueryParserBase) SetEnablePositionIncrements(enable bool) {
	qpb.enablePositionIncrements = enable
}

// GetLowercaseExpandedTerms returns whether expanded terms should be lowercased.
func (qpb *QueryParserBase) GetLowercaseExpandedTerms() bool {
	return qpb.lowercaseExpandedTerms
}

// SetLowercaseExpandedTerms sets whether expanded terms should be lowercased.
func (qpb *QueryParserBase) SetLowercaseExpandedTerms(lowercase bool) {
	qpb.lowercaseExpandedTerms = lowercase
}

// GetFuzzyMinSim returns the minimum similarity for fuzzy queries.
func (qpb *QueryParserBase) GetFuzzyMinSim() float64 {
	return qpb.fuzzyMinSim
}

// SetFuzzyMinSim sets the minimum similarity for fuzzy queries.
func (qpb *QueryParserBase) SetFuzzyMinSim(minSim float64) {
	qpb.fuzzyMinSim = minSim
}

// GetFuzzyPrefixLength returns the prefix length for fuzzy queries.
func (qpb *QueryParserBase) GetFuzzyPrefixLength() int {
	return qpb.fuzzyPrefixLength
}

// SetFuzzyPrefixLength sets the prefix length for fuzzy queries.
func (qpb *QueryParserBase) SetFuzzyPrefixLength(length int) {
	qpb.fuzzyPrefixLength = length
}

// GetPhraseSlop returns the default phrase slop.
func (qpb *QueryParserBase) GetPhraseSlop() int {
	return qpb.phraseSlop
}

// SetPhraseSlop sets the default phrase slop.
func (qpb *QueryParserBase) SetPhraseSlop(slop int) {
	qpb.phraseSlop = slop
}

// GetLocale returns the locale.
func (qpb *QueryParserBase) GetLocale() string {
	return qpb.locale
}

// SetLocale sets the locale.
func (qpb *QueryParserBase) SetLocale(locale string) {
	qpb.locale = locale
}

// GetTimeZone returns the time zone.
func (qpb *QueryParserBase) GetTimeZone() string {
	return qpb.timeZone
}

// SetTimeZone sets the time zone.
func (qpb *QueryParserBase) SetTimeZone(timeZone string) {
	qpb.timeZone = timeZone
}

// ParseFloat parses a float from a string.
func (qpb *QueryParserBase) ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// Escape escapes special characters in a query string.
func (qpb *QueryParserBase) Escape(s string) string {
	var sb strings.Builder
	for _, ch := range s {
		switch ch {
		case '+', '-', '&', '|', '!', '(', ')', '{', '}', '[', ']',
			'^', '"', '~', '*', '?', ':', '\\':
			sb.WriteByte('\\')
			sb.WriteRune(ch)
		default:
			sb.WriteRune(ch)
		}
	}
	return sb.String()
}

// DiscardEscapeChar removes escape characters from a string.
func (qpb *QueryParserBase) DiscardEscapeChar(s string) string {
	var sb strings.Builder
	escaped := false
	for _, ch := range s {
		if escaped {
			sb.WriteRune(ch)
			escaped = false
		} else if ch == '\\' {
			escaped = true
		} else {
			sb.WriteRune(ch)
		}
	}
	return sb.String()
}
