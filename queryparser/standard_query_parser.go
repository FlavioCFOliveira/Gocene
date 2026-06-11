package queryparser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

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

	// timeZone for date parsing (default "UTC")
	timeZone string

	// dateResolution controls how dates are resolved to terms.
	// When set, range query bounds that look like dates (ISO 8601 or
	// yyyyMMdd) are parsed and resolved to the configured granularity.
	// nil means date resolution is disabled (default).
	dateResolution *DateResolution
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

// SetTimeZone sets the time zone for date range parsing.
func (p *StandardQueryParser) SetTimeZone(tz string) error {
	_, err := time.LoadLocation(tz)
	if err != nil {
		return fmt.Errorf("invalid timezone %q: %w", tz, err)
	}
	p.timeZone = tz
	return nil
}

// SetDateResolution enables date resolution with the given granularity.
// When set, range query bounds that look like dates are resolved to terms
// at the configured resolution before building the range query.
// Pass nil to disable date resolution.
func (p *StandardQueryParser) SetDateResolution(dr *DateResolution) {
	p.dateResolution = dr
}

// DateResolution mirrors Lucene's DateTools.Resolution: the granularity
// at which a date is rounded when converted to a term.
type DateResolution int

const (
	ResolutionYear  DateResolution = iota
	ResolutionMonth
	ResolutionDay
	ResolutionHour
	ResolutionMinute
	ResolutionSecond
	ResolutionMillisecond
)

// datePatterns lists common date formats in order of specificity.
var datePatterns = []string{
	"2006-01-02T15:04:05.000Z07:00", // ISO 8601 with milliseconds
	"2006-01-02T15:04:05Z07:00",     // ISO 8601
	"2006-01-02T15:04Z07:00",        // ISO 8601 minute precision
	"2006-01-02",                     // Date only
	"20060102",                       // Compact date (yyyyMMdd)
	"2006-01",                        // Year-month
	"2006",                           // Year only
}

// dateTermRE matches ISO 8601 dates and compact yyyyMMdd format.
var dateTermRE = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?|\d{2}\d{2})$`)

// isDateTerm returns true if s looks like a date value.
func isDateTerm(s string) bool {
	return dateTermRE.MatchString(s)
}

// parseDateRangeTerm parses a date string and returns the term bytes
// after applying the configured resolution. Returns nil if s is not a
// recognizable date or resolution is not configured.
func (p *StandardQueryParser) parseDateRangeTerm(s string) ([]byte, error) {
	if p.dateResolution == nil || !isDateTerm(s) {
		return nil, nil
	}
	if s == "*" {
		return nil, nil
	}

	loc := time.UTC
	if p.timeZone != "" {
		if l, err := time.LoadLocation(p.timeZone); err == nil {
			loc = l
		}
	}

	var t time.Time
	var err error
	for _, layout := range datePatterns {
		t, err = time.ParseInLocation(layout, s, loc)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, nil // not a recognizable date, fall back to raw string
	}

	// Apply resolution truncation matching Lucene DateTools.round.
	switch *p.dateResolution {
	case ResolutionYear:
		t = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	case ResolutionMonth:
		t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case ResolutionDay:
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case ResolutionHour:
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case ResolutionMinute:
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
	case ResolutionSecond:
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, t.Location())
	case ResolutionMillisecond:
		// Already at millisecond precision.
	}

	return []byte(t.Format("20060102150405")), nil
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
//
// Simple phrase:  "quick brown fox"
// This generates a PhraseQuery with the words at consecutive positions.
//
// Multi-phrase (synonyms):  "quick|fast brown fox"
// Terms separated by '|' occupy the same position and generate a
// MultiPhraseQuery instead, matching the behavior of analyzers that
// emit synonyms at the same position (e.g., SynonymGraphFilter).
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
		digitStart := s.pos
		for s.pos < len(s.text) && s.text[s.pos] >= '0' && s.text[s.pos] <= '9' {
			s.pos++
		}
		if s.pos > digitStart {
			parsed, err := strconv.Atoi(s.text[digitStart:s.pos])
			if err != nil {
				return nil, fmt.Errorf("invalid phrase slop %q at position %d: %w", s.text[digitStart:s.pos], digitStart, err)
			}
			slop = parsed
		}
	}

	// Split phrase into positional groups. Words separated by spaces
	// occupy different positions; words separated by '|' are synonyms
	// that share the same position.
	words := strings.Fields(phrase)

	// Detect whether any word contains a '|' separator → multi-phrase.
	hasMulti := false
	for _, w := range words {
		if strings.Contains(w, "|") {
			hasMulti = true
			break
		}
	}

	if !hasMulti {
		// Simple phrase: every word at its own position.
		terms := make([]*index.Term, len(words))
		for i, word := range words {
			terms[i] = index.NewTerm(field, word)
		}
		return search.NewPhraseQueryWithSlop(slop, field, terms...), nil
	}

	// Multi-phrase: build positional groups.
	builder := search.NewMultiPhraseQueryBuilder()
	builder.SetSlop(slop)
	position := 0
	for _, word := range words {
		parts := strings.Split(word, "|")
		terms := make([]*index.Term, len(parts))
		for j, p := range parts {
			terms[j] = index.NewTerm(field, p)
		}
		builder.AddTermsAtPosition(terms, position)
		position++
	}
	return builder.Build(), nil
}

// parseRange parses a range query of the form
//
//	[ lower TO upper ]   – inclusive on both ends
//	{ lower TO upper }   – exclusive on both ends
//
// Mixed inclusive/exclusive forms ([a TO b} and {a TO b]) are accepted as
// well, matching the Lucene 10.4.0 StandardQueryParser grammar.
//
// The "TO" separator is case-insensitive and must be surrounded by
// whitespace, just like Lucene's reference parser. Wildcards on the
// bounds ("*") collapse to an open-ended range.
func (s *parserState) parseRange(field string) (search.Query, error) {
	openBracket := s.text[s.pos]
	s.pos++ // consume '[' or '{'
	includeLower := openBracket == '['

	s.skipWhitespace()
	lower, err := s.parseRangeBound()
	if err != nil {
		return nil, err
	}

	s.skipWhitespace()
	if !s.consumeTOSeparator() {
		return nil, fmt.Errorf("expected TO separator in range query at position %d", s.pos)
	}
	s.skipWhitespace()

	upper, err := s.parseRangeBound()
	if err != nil {
		return nil, err
	}
	s.skipWhitespace()

	if s.pos >= len(s.text) || (s.text[s.pos] != ']' && s.text[s.pos] != '}') {
		return nil, fmt.Errorf("expected ']' or '}' at position %d", s.pos)
	}
	closeBracket := s.text[s.pos]
	includeUpper := closeBracket == ']'
	s.pos++ // consume ']' or '}'

	// Lucene treats "*" as an unbounded sentinel on either end. The
	// TermRangeQuery contract represents "no bound" with a nil byte
	// slice, so we map "*" to nil bytes before constructing the query.
	var lowerBytes, upperBytes []byte
	if lower != "*" {
		// Try date resolution first; fall back to raw string.
		dateBytes, err := s.parser.parseDateRangeTerm(lower)
		if err != nil {
			return nil, err
		}
		if dateBytes != nil {
			lowerBytes = dateBytes
		} else {
			lowerBytes = []byte(lower)
		}
	}
	if upper != "*" {
		dateBytes, err := s.parser.parseDateRangeTerm(upper)
		if err != nil {
			return nil, err
		}
		if dateBytes != nil {
			upperBytes = dateBytes
		} else {
			upperBytes = []byte(upper)
		}
	}

	return search.NewTermRangeQuery(field, lowerBytes, upperBytes, includeLower, includeUpper), nil
}

// parseRangeBound consumes a single range bound (lower or upper). A bound
// is a run of non-whitespace, non-bracket characters; the surrounding
// whitespace has already been skipped by the caller.
func (s *parserState) parseRangeBound() (string, error) {
	start := s.pos
	for s.pos < len(s.text) {
		c := s.text[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
			c == ']' || c == '}' {
			break
		}
		s.pos++
	}
	if start == s.pos {
		return "", fmt.Errorf("expected range bound at position %d", s.pos)
	}
	return s.text[start:s.pos], nil
}

// consumeTOSeparator advances past a case-insensitive "TO" keyword
// surrounded by whitespace. Returns false if the cursor is not at a
// valid separator (callers report the position-tagged error).
func (s *parserState) consumeTOSeparator() bool {
	if s.pos+2 > len(s.text) {
		return false
	}
	if (s.text[s.pos] == 'T' || s.text[s.pos] == 't') &&
		(s.text[s.pos+1] == 'O' || s.text[s.pos+1] == 'o') {
		// Require a trailing whitespace to avoid matching "TOuch" or
		// similar identifier-shaped runs.
		if s.pos+2 == len(s.text) || isRangeWhitespace(s.text[s.pos+2]) {
			s.pos += 2
			return true
		}
	}
	return false
}

// isRangeWhitespace mirrors the small whitespace set parseRangeBound
// considers terminator (space, tab, CR, LF). Brackets count as a
// terminator too but cannot legally appear before the upper bound, so
// they are deliberately excluded here.
func isRangeWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// parseTerm parses a single term.
//
// The terminator set includes '~' so a trailing fuzzy / phrase-slop
// marker is left in the stream for the caller to inspect via peek().
func (s *parserState) parseTerm(field string) (search.Query, error) {
	start := s.pos

	// Parse the term
	for s.pos < len(s.text) {
		c := s.text[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
			c == '(' || c == ')' || c == '[' || c == ']' ||
			c == '{' || c == '}' || c == '"' || c == ':' ||
			c == '~' {
			break
		}
		s.pos++
	}

	if start == s.pos {
		return nil, fmt.Errorf("expected term")
	}

	term := s.text[start:s.pos]

	// Check for fuzzy: "term~" or "term~N" where N is the optional
	// maximum edit distance (0, 1, or 2). The grammar matches Lucene's
	// FlexibleQueryParser StandardSyntaxParser: digits immediately after
	// '~' are interpreted as the max-edits override.
	if s.peek() == '~' {
		s.pos++
		maxEdits := -1
		digitStart := s.pos
		for s.pos < len(s.text) && s.text[s.pos] >= '0' && s.text[s.pos] <= '9' {
			s.pos++
		}
		if s.pos > digitStart {
			parsed, err := strconv.Atoi(s.text[digitStart:s.pos])
			if err != nil {
				return nil, fmt.Errorf("invalid fuzzy edit distance %q at position %d: %w", s.text[digitStart:s.pos], digitStart, err)
			}
			maxEdits = parsed
		}
		t := index.NewTerm(field, term)
		if maxEdits < 0 {
			return search.NewFuzzyQuery(t), nil
		}
		return search.NewFuzzyQueryWithMaxEdits(t, maxEdits), nil
	}

	// Check for wildcard: presence of '*' or '?' anywhere in the term
	// triggers a WildcardQuery. Lucene additionally short-circuits a
	// trailing-only '*' into a PrefixQuery; that distinction is a pure
	// optimisation (WildcardQuery handles both forms correctly) and is
	// not required here.
	if strings.Contains(term, "*") || strings.Contains(term, "?") {
		return search.NewWildcardQuery(index.NewTerm(field, term)), nil
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
