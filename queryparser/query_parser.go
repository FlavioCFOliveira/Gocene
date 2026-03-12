// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
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

// parseExpression parses an expression (handles OR).
func (p *QueryParser) parseExpression() (search.Query, error) {
	left, err := p.parseAndExpression()
	if err != nil {
		return nil, err
	}

	for p.match(TokenTypeOR) {
		p.nextToken()
		right, err := p.parseAndExpression()
		if err != nil {
			return nil, err
		}
		left = search.NewBooleanQueryOrWithQueries(left, right)
	}

	return left, nil
}

// parseAndExpression parses an AND expression.
func (p *QueryParser) parseAndExpression() (search.Query, error) {
	left, err := p.parseNotExpression()
	if err != nil {
		return nil, err
	}

	for p.match(TokenTypeAND) || p.isImplicitAnd() {
		if p.match(TokenTypeAND) {
			p.nextToken()
		}
		right, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		left = search.NewBooleanQueryAndWithQueries(left, right)
	}

	return left, nil
}

// isImplicitAnd checks if we should treat this as an implicit AND.
func (p *QueryParser) isImplicitAnd() bool {
	// Check if next token starts a new term/field/group
	return p.lookAhead.Type == TokenTypeTerm ||
		p.lookAhead.Type == TokenTypeField ||
		p.lookAhead.Type == TokenTypeLParen ||
		p.lookAhead.Type == TokenTypePlus ||
		p.lookAhead.Type == TokenTypeMinus
}

// parseNotExpression parses a NOT expression.
func (p *QueryParser) parseNotExpression() (search.Query, error) {
	if p.match(TokenTypeNOT) {
		p.nextToken()
		query, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		return search.NewBooleanQueryNotWithQuery(query), nil
	}
	return p.parseModifierExpression()
}

// parseModifierExpression parses expressions with + or - modifiers.
func (p *QueryParser) parseModifierExpression() (search.Query, error) {
	modifier := search.SHOULD // default

	if p.match(TokenTypePlus) {
		modifier = search.MUST
		p.nextToken()
	} else if p.match(TokenTypeMinus) {
		modifier = search.MUST_NOT
		p.nextToken()
	}

	query, err := p.parsePrimaryExpression()
	if err != nil {
		return nil, err
	}

	// Apply modifier if specified
	if modifier != search.SHOULD {
		bq := search.NewBooleanQuery()
		bq.Add(query, modifier)
		return bq, nil
	}

	return query, nil
}

// parsePrimaryExpression parses primary expressions (terms, fields, groups, etc.).
func (p *QueryParser) parsePrimaryExpression() (search.Query, error) {
	switch p.currentToken.Type {
	case TokenTypeLParen:
		return p.parseGroup()
	case TokenTypeField:
		return p.parseFieldQuery()
	case TokenTypeTerm:
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

	// Parse lower bound
	if !p.match(TokenTypeTerm) {
		return nil, fmt.Errorf("expected term at position %d", p.currentToken.Pos)
	}
	lowerTerm := p.currentToken.Value
	p.nextToken()

	// Expect TO
	if !p.match(TokenTypeTO) {
		return nil, fmt.Errorf("expected TO at position %d", p.currentToken.Pos)
	}
	p.nextToken()

	// Parse upper bound
	if !p.match(TokenTypeTerm) {
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

	// Extract terms from phrase query and create new one with slop
	if pq, ok := query.(*search.PhraseQuery); ok {
		// For now, we can't modify the existing phrase query
		// In a full implementation, we would create a new PhraseQuery with slop
		_ = slop
		return pq, nil
	}

	return query, nil
}

// createTermQuery creates a term query.
func (p *QueryParser) createTermQuery(field, text string) search.Query {
	term := index.NewTerm(field, text)
	return search.NewTermQuery(term)
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

// applyFieldToQuery applies a field to a query (for field:(expression) syntax).
func (p *QueryParser) applyFieldToQuery(field string, query search.Query) (search.Query, error) {
	// This is a simplified implementation
	// In a full implementation, we would recursively apply the field to all sub-queries
	return query, nil
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
