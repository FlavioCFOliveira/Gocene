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
	"github.com/FlavioCFOliveira/Gocene/util"
)

// QueryParser parses query strings into Query objects using the classic Lucene query syntax.
// This is the Go port of Lucene's QueryParser.
//
// Supported syntax:
//   - term: search for a term in the default field
//   - field:term: search for a term in a specific field
//   - term1 AND term2: boolean AND
//   - term1 OR term2: boolean OR
//   - NOT term: boolean NOT
//   - +term: term is required
//   - -term: term is prohibited
//   - "phrase": search for a phrase
//   - term*: wildcard query with *
//   - term?: wildcard query with ?
//   - term~: fuzzy query
//   - term~2: fuzzy query with edit distance
//   - term^boost: boost term
//   - (expression): grouping
//   - [a TO b]: inclusive range
//   - {a TO b}: exclusive range
type QueryParser struct {
	defaultField string
	analyzer     *analysis.StandardAnalyzer
	tokenManager *QueryParserTokenManager
	currentToken Token
	lookAhead    Token
}

// NewQueryParser creates a new QueryParser.
func NewQueryParser(defaultField string, analyzer *analysis.StandardAnalyzer) *QueryParser {
	return &QueryParser{
		defaultField: defaultField,
		analyzer:     analyzer,
	}
}

// NewQueryParserWithDefaultField creates a new QueryParser with default field.
func NewQueryParserWithDefaultField(defaultField string) *QueryParser {
	return NewQueryParser(defaultField, analysis.NewStandardAnalyzer())
}

// Parse parses a query string into a Query.
func (p *QueryParser) Parse(queryString string) (search.Query, error) {
	if queryString == "" {
		return search.NewMatchAllDocsQuery(), nil
	}

	p.tokenManager = NewQueryParserTokenManager(queryString)
	p.nextToken() // Initialize currentToken
	p.nextToken() // Initialize lookAhead

	return p.parseExpression()
}

// nextToken advances to the next token.
func (p *QueryParser) nextToken() {
	p.currentToken = p.lookAhead
	p.lookAhead = p.tokenManager.NextToken()
}

// consume consumes the current token if it matches the expected type.
func (p *QueryParser) consume(expected TokenType) (Token, error) {
	if p.currentToken.Type != expected {
		return Token{}, fmt.Errorf("expected %s but got %s at position %d", expected, p.currentToken.Type, p.currentToken.Pos)
	}
	token := p.currentToken
	p.nextToken()
	return token, nil
}

// match checks if the current token matches the expected type.
func (p *QueryParser) match(expected TokenType) bool {
	return p.currentToken.Type == expected
}

// matchLookAhead checks if the look ahead token matches the expected type.
func (p *QueryParser) matchLookAhead(expected TokenType) bool {
	return p.lookAhead.Type == expected
}

// parseExpression parses a full expression handling OR, AND, and implicit
// sequences.
//
// Precedence (lowest to highest): OR < implicit-sequence/AND < NOT < primary.
func (p *QueryParser) parseExpression() (search.Query, error) {
	first, err := p.parseAndOrSequence()
	if err != nil {
		return nil, err
	}

	if !p.match(TokenTypeOR) {
		return first, nil
	}

	// Collect all OR operands into a single flat BooleanQuery (SHOULD).
	operands := []search.Query{first}
	for p.match(TokenTypeOR) {
		p.nextToken()
		next, err := p.parseAndOrSequence()
		if err != nil {
			return nil, err
		}
		operands = append(operands, next)
	}
	return search.NewBooleanQueryOrWithQueries(operands...), nil
}

// parseAndOrSequence handles explicit AND and implicit sequences (space-separated
// terms). When AND is explicit, all operands become MUST. When the sequence is
// implicit (no AND keyword), each term's Occur is determined by its +/- prefix
// or NOT keyword, matching Lucene's classic query parser behaviour.
func (p *QueryParser) parseAndOrSequence() (search.Query, error) {
	// If the next operator is an explicit AND, use MUST for all operands.
	first, firstOccur, err := p.parseClauseWithOccur()
	if err != nil {
		return nil, err
	}

	if !p.match(TokenTypeAND) && !p.isImplicitSequenceContinuation() {
		// Single clause — unwrap Occur if it is a plain SHOULD (no modifier).
		if firstOccur == search.SHOULD {
			return first, nil
		}
		bq := search.NewBooleanQuery()
		bq.Add(first, firstOccur)
		return bq, nil
	}

	// Multiple clauses.
	type clauseEntry struct {
		q     search.Query
		occur search.Occur
	}
	clauses := []clauseEntry{{first, firstOccur}}
	explicitAnd := false

	for p.match(TokenTypeAND) || p.isImplicitSequenceContinuation() {
		if p.match(TokenTypeAND) {
			explicitAnd = true
			p.nextToken()
		}
		q, occur, err2 := p.parseClauseWithOccur()
		if err2 != nil {
			return nil, err2
		}
		clauses = append(clauses, clauseEntry{q, occur})
	}

	bq := search.NewBooleanQuery()
	for _, c := range clauses {
		if explicitAnd {
			// Explicit AND: all clauses become MUST regardless of prefix.
			bq.Add(c.q, search.MUST)
		} else {
			bq.Add(c.q, c.occur)
		}
	}
	return bq, nil
}

// parseClauseWithOccur parses one clause and returns the query together with
// the Occur derived from its prefix (+, -, NOT) or SHOULD if there is none.
func (p *QueryParser) parseClauseWithOccur() (search.Query, search.Occur, error) {
	occur := search.SHOULD

	switch p.currentToken.Type {
	case TokenTypePlus:
		occur = search.MUST
		p.nextToken()
	case TokenTypeMinus:
		occur = search.MUST_NOT
		p.nextToken()
	case TokenTypeNOT:
		occur = search.MUST_NOT
		p.nextToken()
	}

	q, err := p.parsePrimaryExpression()
	if err != nil {
		return nil, occur, err
	}
	return q, occur, nil
}

// isImplicitSequenceContinuation returns true when the current token begins
// another term/clause that is implicitly adjacent to the previous one (i.e.
// separated only by whitespace, no OR keyword).
func (p *QueryParser) isImplicitSequenceContinuation() bool {
	switch p.currentToken.Type {
	case TokenTypeAND, TokenTypeOR, TokenTypeEOF,
		TokenTypeRParen, TokenTypeRBracket, TokenTypeRBrace:
		return false
	case TokenTypeTerm, TokenTypeWildcard, TokenTypeField,
		TokenTypeLParen, TokenTypeQuote,
		TokenTypeLBracket, TokenTypeLBrace,
		TokenTypePlus, TokenTypeMinus, TokenTypeNOT:
		return true
	}
	return false
}

// parsePrimaryExpression parses primary expressions (terms, fields, groups, etc.).
func (p *QueryParser) parsePrimaryExpression() (search.Query, error) {
	switch p.currentToken.Type {
	case TokenTypeLParen:
		return p.parseGroup()
	case TokenTypeField:
		return p.parseFieldQuery()
	case TokenTypeTerm, TokenTypeWildcard:
		return p.parseTermQuery()
	case TokenTypeQuote:
		return p.parsePhraseQuery()
	case TokenTypeLBracket, TokenTypeLBrace:
		return p.parseRangeQuery()
	default:
		return nil, fmt.Errorf("unexpected token %s at position %d", p.currentToken.Type, p.currentToken.Pos)
	}
}

// parseGroup parses a grouped expression.
func (p *QueryParser) parseGroup() (search.Query, error) {
	if _, err := p.consume(TokenTypeLParen); err != nil {
		return nil, err
	}

	query, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	if _, err := p.consume(TokenTypeRParen); err != nil {
		return nil, err
	}

	// Check for boost
	if p.match(TokenTypeBoost) {
		return p.applyBoost(query)
	}

	return query, nil
}

// parseFieldQuery parses a field:term or field:expression query.
func (p *QueryParser) parseFieldQuery() (search.Query, error) {
	field := p.currentToken.Value
	p.nextToken()

	if _, err := p.consume(TokenTypeColon); err != nil {
		return nil, err
	}

	// Check for special field queries
	switch p.currentToken.Type {
	case TokenTypeLBracket, TokenTypeLBrace:
		return p.parseFieldRangeQuery(field)
	case TokenTypeLParen:
		// Grouped expression after field
		query, err := p.parseGroup()
		if err != nil {
			return nil, err
		}
		// Apply field to the query
		return p.applyFieldToQuery(field, query)
	case TokenTypeQuote:
		return p.parseFieldPhraseQuery(field)
	case TokenTypeTerm, TokenTypeWildcard:
		return p.parseFieldTermQuery(field)
	default:
		return nil, fmt.Errorf("expected term, phrase, or range after field at position %d", p.currentToken.Pos)
	}
}

// parseFieldTermQuery parses a term query for a specific field.
func (p *QueryParser) parseFieldTermQuery(field string) (search.Query, error) {
	term := p.currentToken.Value
	isWildcard := p.currentToken.Type == TokenTypeWildcard
	p.nextToken()

	var query search.Query
	if isWildcard {
		// Wildcard query
		query = p.createWildcardQuery(field, term)
	} else {
		// Regular term query
		query = p.createTermQuery(field, term)
	}

	// Check for fuzzy
	if p.match(TokenTypeFuzzy) {
		return p.applyFuzzy(query)
	}

	// Check for boost
	if p.match(TokenTypeBoost) {
		return p.applyBoost(query)
	}

	return query, nil
}

// parseFieldPhraseQuery parses a phrase query for a specific field.
func (p *QueryParser) parseFieldPhraseQuery(field string) (search.Query, error) {
	// Consume opening quote
	p.nextToken()

	var terms []string
	for !p.match(TokenTypeQuote) && !p.match(TokenTypeEOF) {
		if p.match(TokenTypeTerm) {
			terms = append(terms, p.currentToken.Value)
			p.nextToken()
		} else {
			p.nextToken()
		}
	}

	if _, err := p.consume(TokenTypeQuote); err != nil {
		return nil, err
	}

	// Create phrase query
	query := p.createPhraseQuery(field, terms)

	// Check for proximity
	if p.match(TokenTypeFuzzy) {
		return p.applyProximity(query)
	}

	// Check for boost
	if p.match(TokenTypeBoost) {
		return p.applyBoost(query)
	}

	return query, nil
}

// parseFieldRangeQuery parses a range query for a specific field.
func (p *QueryParser) parseFieldRangeQuery(field string) (search.Query, error) {
	return p.parseRangeQueryWithField(field)
}

// parseTermQuery parses a term query.
func (p *QueryParser) parseTermQuery() (search.Query, error) {
	term := p.currentToken.Value
	isWildcard := p.currentToken.Type == TokenTypeWildcard
	p.nextToken()

	var query search.Query
	if isWildcard {
		query = p.createWildcardQuery(p.defaultField, term)
	} else {
		query = p.createTermQuery(p.defaultField, term)
	}

	// Check for fuzzy
	if p.match(TokenTypeFuzzy) {
		return p.applyFuzzy(query)
	}

	// Check for boost
	if p.match(TokenTypeBoost) {
		return p.applyBoost(query)
	}

	return query, nil
}

// parsePhraseQuery parses a phrase query.
func (p *QueryParser) parsePhraseQuery() (search.Query, error) {
	// Consume opening quote
	p.nextToken()

	var terms []string
	for !p.match(TokenTypeQuote) && !p.match(TokenTypeEOF) {
		if p.match(TokenTypeTerm) {
			terms = append(terms, p.currentToken.Value)
			p.nextToken()
		} else {
			p.nextToken()
		}
	}

	if _, err := p.consume(TokenTypeQuote); err != nil {
		return nil, err
	}

	// Create phrase query
	query := p.createPhraseQuery(p.defaultField, terms)

	// Check for proximity
	if p.match(TokenTypeFuzzy) {
		return p.applyProximity(query)
	}

	// Check for boost
	if p.match(TokenTypeBoost) {
		return p.applyBoost(query)
	}

	return query, nil
}

// parseRangeQuery parses a range query.
func (p *QueryParser) parseRangeQuery() (search.Query, error) {
	return p.parseRangeQueryWithField(p.defaultField)
}

// parseRangeQueryWithField parses a range query for a specific field.
func (p *QueryParser) parseRangeQueryWithField(field string) (search.Query, error) {
	// Determine inclusive/exclusive
	var includeLower, includeUpper bool
	if p.match(TokenTypeLBracket) {
		includeLower = true
	} else if p.match(TokenTypeLBrace) {
		includeLower = false
	} else {
		return nil, fmt.Errorf("expected [ or { at position %d", p.currentToken.Pos)
	}
	p.nextToken()

	// Parse lower bound (can be TERM or NUMBER)
	if !p.match(TokenTypeTerm) && !p.match(TokenTypeNumber) {
		return nil, fmt.Errorf("expected term at position %d", p.currentToken.Pos)
	}
	lowerTerm := p.currentToken.Value
	p.nextToken()

	// Expect TO
	if !p.match(TokenTypeTO) {
		return nil, fmt.Errorf("expected TO at position %d", p.currentToken.Pos)
	}
	p.nextToken()

	// Parse upper bound (can be TERM or NUMBER)
	if !p.match(TokenTypeTerm) && !p.match(TokenTypeNumber) {
		return nil, fmt.Errorf("expected term at position %d", p.currentToken.Pos)
	}
	upperTerm := p.currentToken.Value
	p.nextToken()

	// Determine upper inclusive/exclusive
	if p.match(TokenTypeRBracket) {
		includeUpper = true
	} else if p.match(TokenTypeRBrace) {
		includeUpper = false
	} else {
		return nil, fmt.Errorf("expected ] or } at position %d", p.currentToken.Pos)
	}
	p.nextToken()

	// Create range query
	query := p.createRangeQuery(field, lowerTerm, upperTerm, includeLower, includeUpper)

	// Check for boost
	if p.match(TokenTypeBoost) {
		return p.applyBoost(query)
	}

	return query, nil
}

// applyBoost applies a boost to a query.
func (p *QueryParser) applyBoost(query search.Query) (search.Query, error) {
	if _, err := p.consume(TokenTypeBoost); err != nil {
		return nil, err
	}

	var boost float64 = 1.0
	if p.match(TokenTypeNumber) {
		val, err := strconv.ParseFloat(p.currentToken.Value, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid boost value at position %d", p.currentToken.Pos)
		}
		boost = val
		p.nextToken()
	}

	return search.NewBoostQuery(query, float32(boost)), nil
}

// applyFuzzy applies fuzzy matching to a query.
func (p *QueryParser) applyFuzzy(query search.Query) (search.Query, error) {
	if _, err := p.consume(TokenTypeFuzzy); err != nil {
		return nil, err
	}

	maxEdits := 2 // Default
	if p.match(TokenTypeNumber) {
		val, err := strconv.Atoi(p.currentToken.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid fuzzy value at position %d", p.currentToken.Pos)
		}
		maxEdits = val
		p.nextToken()
	}

	// Extract term from query
	if tq, ok := query.(*search.TermQuery); ok {
		term := tq.Term()
		return search.NewFuzzyQueryWithMaxEdits(term, maxEdits), nil
	}

	// For non-term queries, return as-is with a warning (in real impl)
	return query, nil
}

// applyProximity applies proximity matching to a phrase query.
func (p *QueryParser) applyProximity(query search.Query) (search.Query, error) {
	if _, err := p.consume(TokenTypeFuzzy); err != nil {
		return nil, err
	}

	slop := 0
	if p.match(TokenTypeNumber) {
		val, err := strconv.Atoi(p.currentToken.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid proximity value at position %d", p.currentToken.Pos)
		}
		slop = val
		p.nextToken()
	}

	// Re-emit the phrase query with the parsed slop. Lucene's
	// QueryParser.handleBoost / addSlopToPhrase pattern preserves the
	// existing terms and field and produces a new PhraseQuery so the
	// returned value carries the requested slop. PhraseQuery exposes
	// SetSlop directly in Gocene, so an in-place mutation is sufficient
	// and avoids re-walking the term list.
	if pq, ok := query.(*search.PhraseQuery); ok {
		pq.SetSlop(slop)
		return pq, nil
	}

	return query, nil
}

// createTermQuery creates a term query, running the input text through the
// configured analyzer (so e.g. "Hello" becomes "hello" under StandardAnalyzer).
//
// If the analyzer emits a single token, a plain TermQuery is returned.
// If multiple tokens are produced, they are wrapped in a BooleanQuery (SHOULD).
// If the analyzer emits no tokens (text was filtered out, e.g. a stop word),
// the un-analyzed text is used so the query is not silently dropped.
func (p *QueryParser) createTermQuery(field, text string) search.Query {
	tokens := p.analyzeText(field, text)
	switch len(tokens) {
	case 0:
		// Analyzer dropped the token (e.g. stop word); fall back to raw text
		// so the query still has something to match against.
		return search.NewTermQuery(index.NewTerm(field, text))
	case 1:
		return search.NewTermQuery(index.NewTerm(field, tokens[0]))
	default:
		bq := search.NewBooleanQuery()
		for _, tok := range tokens {
			bq.Add(search.NewTermQuery(index.NewTerm(field, tok)), search.SHOULD)
		}
		return bq
	}
}

// analyzeText runs text through the configured analyzer and returns the
// resulting token terms. If the analyzer is nil or fails, the original text
// is returned unchanged.
func (p *QueryParser) analyzeText(field, text string) []string {
	if p.analyzer == nil {
		return []string{text}
	}
	ts, err := p.analyzer.TokenStream(field, strings.NewReader(text))
	if err != nil || ts == nil {
		return []string{text}
	}
	defer ts.Close()

	var tokens []string
	for {
		hasNext, err := ts.IncrementToken()
		if err != nil || !hasNext {
			break
		}
		if attrSrc, ok := ts.(interface {
			GetAttributeSource() *util.AttributeSource
		}); ok {
			if attr := attrSrc.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType); attr != nil {
				if termAttr, ok := attr.(analysis.CharTermAttribute); ok {
					tokens = append(tokens, termAttr.String())
				}
			}
		}
	}
	return tokens
}

// createWildcardQuery creates a wildcard query.
func (p *QueryParser) createWildcardQuery(field, pattern string) search.Query {
	term := index.NewTerm(field, pattern)
	return search.NewWildcardQuery(term)
}

// createPhraseQuery creates a phrase query.
func (p *QueryParser) createPhraseQuery(field string, terms []string) search.Query {
	termPtrs := make([]*index.Term, len(terms))
	for i, text := range terms {
		termPtrs[i] = index.NewTerm(field, text)
	}
	return search.NewPhraseQuery(field, termPtrs...)
}

// createRangeQuery creates a range query.
func (p *QueryParser) createRangeQuery(field, lower, upper string, includeLower, includeUpper bool) search.Query {
	// Use TermRangeQuery for string ranges
	return search.NewTermRangeQuery(field, []byte(lower), []byte(upper), includeLower, includeUpper)
}

// applyFieldToQuery applies a field to a query (for field:(expression)
// syntax). It mirrors the recursive descent Lucene's QueryParser performs
// when a field-qualified group is parsed: every leaf term query is
// rewritten to target the requested field, and compound queries
// (BooleanQuery, PhraseQuery, FuzzyQuery, WildcardQuery, TermRangeQuery)
// have their constituent terms repointed.
//
// The original input query is left untouched; the function returns a new
// query tree so callers that retain the original reference observe no
// side-effects.
func (p *QueryParser) applyFieldToQuery(field string, query search.Query) (search.Query, error) {
	if query == nil {
		return nil, nil
	}

	switch q := query.(type) {
	case *search.TermQuery:
		return search.NewTermQuery(index.NewTerm(field, q.Term().Text())), nil

	case *search.PhraseQuery:
		terms := q.Terms()
		retargeted := make([]*index.Term, len(terms))
		for i, t := range terms {
			retargeted[i] = index.NewTerm(field, t.Text())
		}
		pq := search.NewPhraseQueryWithSlop(q.GetSlop(), field, retargeted...)
		return pq, nil

	case *search.BooleanQuery:
		nq := search.NewBooleanQuery()
		nq.SetMinimumNumberShouldMatch(q.MinimumNumberShouldMatch())
		for _, clause := range q.Clauses() {
			sub, err := p.applyFieldToQuery(field, clause.Query)
			if err != nil {
				return nil, err
			}
			nq.Add(sub, clause.Occur)
		}
		return nq, nil

	case *search.FuzzyQuery:
		return search.NewFuzzyQuery(index.NewTerm(field, q.Term().Text())), nil

	case *search.WildcardQuery:
		return search.NewWildcardQuery(index.NewTerm(field, q.Term().Text())), nil

	case *search.TermRangeQuery:
		return search.NewTermRangeQuery(
			field,
			q.LowerTerm(), q.UpperTerm(),
			q.IncludesLower(), q.IncludesUpper(),
		), nil

	default:
		// Unknown query type: there is nothing useful we can do without
		// risking semantic corruption. Mirror Lucene's behaviour of
		// returning the original tree untouched.
		return query, nil
	}
}

// GetDefaultField returns the default field for queries.
func (p *QueryParser) GetDefaultField() string {
	return p.defaultField
}

// SetDefaultField sets the default field for queries.
func (p *QueryParser) SetDefaultField(field string) {
	p.defaultField = field
}

// GetAnalyzer returns the analyzer used by this parser.
func (p *QueryParser) GetAnalyzer() *analysis.StandardAnalyzer {
	return p.analyzer
}

// SetAnalyzer sets the analyzer for this parser.
func (p *QueryParser) SetAnalyzer(analyzer *analysis.StandardAnalyzer) {
	p.analyzer = analyzer
}
