package queryparser

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// StandardQueryParser is a query parser that supports Lucene's standard query syntax.
// This includes fielded queries, boolean operators, phrase queries, range queries,
// wildcard queries, and more.
//
// This is the Go port of Lucene's org.apache.lucene.queryparser.standard.StandardQueryParser.
type StandardQueryParser struct {
	// defaultField is the default field to search
	defaultField string

	// defaultOperator is the default boolean operator (AND or OR)
	defaultOperator BooleanOperator

	// allowLeadingWildcard allows wildcards at the beginning of terms
	allowLeadingWildcard bool

	// enablePositionIncrements enables position increments in phrase queries
	enablePositionIncrements bool

	// fuzzyPrefixLength is the prefix length for fuzzy queries
	fuzzyPrefixLength int

	// fuzzyMinSim is the minimum similarity for fuzzy queries
	fuzzyMinSim float64

	// phraseSlop is the default slop for phrase queries
	phraseSlop int

	// locale for parsing
	locale string

	// timeZone for date parsing
	timeZone string
}

// BooleanOperator represents the default boolean operator.
type BooleanOperator int

const (
	// AND means all terms must match
	AND BooleanOperator = iota
	// OR means any term can match
	OR
)

// String returns the string representation of the BooleanOperator.
func (op BooleanOperator) String() string {
	switch op {
	case AND:
		return "AND"
	case OR:
		return "OR"
	default:
		return "UNKNOWN"
	}
}

// NewStandardQueryParser creates a new StandardQueryParser.
func NewStandardQueryParser() *StandardQueryParser {
	return &StandardQueryParser{
		defaultField:             "",
		defaultOperator:          OR,
		allowLeadingWildcard:     false,
		enablePositionIncrements: true,
		fuzzyPrefixLength:        0,
		fuzzyMinSim:              2.0,
		phraseSlop:               0,
		locale:                   "en",
		timeZone:                 "UTC",
	}
}

// SetDefaultField sets the default field for queries.
func (p *StandardQueryParser) SetDefaultField(field string) {
	p.defaultField = field
}

// GetDefaultField returns the default field.
func (p *StandardQueryParser) GetDefaultField() string {
	return p.defaultField
}

// SetDefaultOperator sets the default boolean operator.
func (p *StandardQueryParser) SetDefaultOperator(op BooleanOperator) {
	p.defaultOperator = op
}

// GetDefaultOperator returns the default boolean operator.
func (p *StandardQueryParser) GetDefaultOperator() BooleanOperator {
	return p.defaultOperator
}

// SetAllowLeadingWildcard sets whether to allow leading wildcards.
func (p *StandardQueryParser) SetAllowLeadingWildcard(allow bool) {
	p.allowLeadingWildcard = allow
}

// GetAllowLeadingWildcard returns whether leading wildcards are allowed.
func (p *StandardQueryParser) GetAllowLeadingWildcard() bool {
	return p.allowLeadingWildcard
}

// SetPhraseSlop sets the default phrase slop.
func (p *StandardQueryParser) SetPhraseSlop(slop int) {
	p.phraseSlop = slop
}

// GetPhraseSlop returns the default phrase slop.
func (p *StandardQueryParser) GetPhraseSlop() int {
	return p.phraseSlop
}

// Parse parses a query string and returns a Query.
func (p *StandardQueryParser) Parse(queryText string) (search.Query, error) {
	if queryText == "" {
		return nil, fmt.Errorf("query text cannot be empty")
	}

	// Create a parser state
	state := &parserState{
		parser: p,
		text:   queryText,
		pos:    0,
	}

	// Parse the query
	query, err := state.parseQuery()
	if err != nil {
		return nil, fmt.Errorf("parse error at position %d: %w", state.pos, err)
	}

	return query, nil
}

// ParseWithField parses a query string with a specific field.
func (p *StandardQueryParser) ParseWithField(field string, queryText string) (search.Query, error) {
	oldField := p.defaultField
	p.defaultField = field
	defer func() { p.defaultField = oldField }()

	return p.Parse(queryText)
}

// parserState holds the state during parsing.
type parserState struct {
	parser *StandardQueryParser
	text   string
	pos    int
}

// parseQuery parses a complete query.
func (s *parserState) parseQuery() (search.Query, error) {
	s.skipWhitespace()

	if s.pos >= len(s.text) {
		return nil, fmt.Errorf("empty query")
	}

	// Parse the first clause
	query, err := s.parseClause()
	if err != nil {
		return nil, err
	}

	// Parse additional clauses with boolean operators
	for s.pos < len(s.text) {
		s.skipWhitespace()
		if s.pos >= len(s.text) {
			break
		}

		// Check for boolean operators
		op := s.parser.defaultOperator
		if s.match("AND") {
			op = AND
		} else if s.match("OR") {
			op = OR
		} else if s.match("NOT") {
			// NOT is handled specially
			clause, err := s.parseClause()
			if err != nil {
				return nil, err
			}
			// Create a boolean query with MUST_NOT
			boolQuery := search.NewBooleanQuery()
			boolQuery.Add(query, search.MUST)
			boolQuery.Add(clause, search.MUST_NOT)
			query = boolQuery
			continue
		}

		clause, err := s.parseClause()
		if err != nil {
			return nil, err
		}

		// Combine queries based on operator
		if op == AND {
			boolQuery := search.NewBooleanQuery()
			boolQuery.Add(query, search.MUST)
			boolQuery.Add(clause, search.MUST)
			query = boolQuery
		} else {
			boolQuery := search.NewBooleanQuery()
			boolQuery.Add(query, search.SHOULD)
			boolQuery.Add(clause, search.SHOULD)
			query = boolQuery
		}
	}

	return query, nil
}

// parseClause parses a single clause (fielded term, phrase, group, etc.).
func (s *parserState) parseClause() (search.Query, error) {
	s.skipWhitespace()

	if s.pos >= len(s.text) {
		return nil, fmt.Errorf("unexpected end of query")
	}

	// Check for grouped query
	if s.peek() == '(' {
		return s.parseGroup()
	}

	// Check for fielded query
	field := s.parser.defaultField
	if s.peek() != '"' && s.peek() != '+' && s.peek() != '-' && s.peek() != '!' {
		// Try to parse a field name
		possibleField := s.parseIdentifier()
		if s.peek() == ':' {
			s.pos++ // consume ':'
			field = possibleField
		} else {
			// Not a field, treat as term
			s.pos -= len(possibleField)
		}
	}

	// Check for modifiers
	occur := search.SHOULD
	if s.peek() == '+' {
		occur = search.MUST
		s.pos++
	} else if s.peek() == '-' || s.peek() == '!' {
		occur = search.MUST_NOT
		s.pos++
	}

	s.skipWhitespace()

	// Parse the term or phrase
	var query search.Query
	var err error

	if s.peek() == '"' {
		query, err = s.parsePhrase(field)
	} else if s.peek() == '[' || s.peek() == '{' {
		query, err = s.parseRange(field)
	} else {
		query, err = s.parseTerm(field)
	}

	if err != nil {
		return nil, err
	}

	// Apply occurrence modifier
	if occur != search.SHOULD {
		boolQuery := search.NewBooleanQuery()
		boolQuery.Add(query, occur)
		query = boolQuery
	}

	return query, nil
}

// parseGroup parses a grouped query.
func (s *parserState) parseGroup() (search.Query, error) {
	s.pos++ // consume '('

	query, err := s.parseQuery()
	if err != nil {
		return nil, err
	}

	s.skipWhitespace()
	if s.pos >= len(s.text) || s.text[s.pos] != ')' {
		return nil, fmt.Errorf("expected ')'")
	}
	s.pos++ // consume ')'

	return query, nil
}

// parsePhrase parses a phrase query.
func (s *parserState) parsePhrase(field string) (search.Query, error) {
	s.pos++ // consume opening quote

	start := s.pos
	for s.pos < len(s.text) && s.text[s.pos] != '"' {
		s.pos++
	}

	if s.pos >= len(s.text) {
		return nil, fmt.Errorf("unterminated phrase")
	}

	phrase := s.text[start:s.pos]
	s.pos++ // consume closing quote

	// Check for slop
	slop := s.parser.phraseSlop
	if s.peek() == '~' {
		s.pos++
		// For now, skip slop value parsing
	}

	// Create terms from phrase words
	words := strings.Fields(phrase)
	terms := make([]*index.Term, len(words))
	for i, word := range words {
		terms[i] = index.NewTerm(field, word)
	}

	return search.NewPhraseQueryWithSlop(slop, field, terms...), nil
}

// parseRange parses a range query.
func (s *parserState) parseRange(field string) (search.Query, error) {
	s.pos++ // consume '[' or '{'

	start := s.pos
	for s.pos < len(s.text) && s.text[s.pos] != ' ' && s.text[s.pos] != ']' && s.text[s.pos] != '}' {
		s.pos++
	}

	lower := s.text[start:s.pos]

	s.skipWhitespace()
	if s.pos >= len(s.text) || (s.text[s.pos] != ']' && s.text[s.pos] != '}') {
		return nil, fmt.Errorf("expected ']' or '}'")
	}

	s.pos++ // consume ']' or '}'

	// For now, create a simple term query
	// In a full implementation, this would create a range query
	return search.NewTermQuery(index.NewTerm(field, lower)), nil
}

// parseTerm parses a single term.
func (s *parserState) parseTerm(field string) (search.Query, error) {
	start := s.pos

	// Parse the term
	for s.pos < len(s.text) {
		c := s.text[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
			c == '(' || c == ')' || c == '[' || c == ']' ||
			c == '{' || c == '}' || c == '"' || c == ':' {
			break
		}
		s.pos++
	}

	if start == s.pos {
		return nil, fmt.Errorf("expected term")
	}

	term := s.text[start:s.pos]

	// Check for fuzzy
	if s.peek() == '~' {
		s.pos++
		// For now, just create a term query
		// In a full implementation, this would create a fuzzy query
		return search.NewTermQuery(index.NewTerm(field, term)), nil
	}

	// Check for wildcard
	if strings.Contains(term, "*") || strings.Contains(term, "?") {
		// For now, just create a term query
		// In a full implementation, this would create a wildcard query
		return search.NewTermQuery(index.NewTerm(field, term)), nil
	}

	return search.NewTermQuery(index.NewTerm(field, term)), nil
}

// parseIdentifier parses an identifier.
func (s *parserState) parseIdentifier() string {
	start := s.pos
	for s.pos < len(s.text) {
		c := s.text[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
			c == '(' || c == ')' || c == '[' || c == ']' ||
			c == '{' || c == '}' || c == '"' || c == ':' {
			break
		}
		s.pos++
	}
	return s.text[start:s.pos]
}

// skipWhitespace skips whitespace characters.
func (s *parserState) skipWhitespace() {
	for s.pos < len(s.text) {
		c := s.text[s.pos]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		s.pos++
	}
}

// peek returns the current character without consuming it.
func (s *parserState) peek() byte {
	if s.pos >= len(s.text) {
		return 0
	}
	return s.text[s.pos]
}

// match tries to match a string at the current position.
func (s *parserState) match(str string) bool {
	if s.pos+len(str) > len(s.text) {
		return false
	}
	if s.text[s.pos:s.pos+len(str)] == str {
		s.pos += len(str)
		return true
	}
	return false
}
