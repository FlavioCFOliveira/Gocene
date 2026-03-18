// Package flexible provides the flexible query parser framework for Lucene-compatible query parsing.
package flexible

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// StandardQueryConfigHandler handles configuration for the standard query parser.
type StandardQueryConfigHandler struct {
	// Analyzer is the analyzer to use for text analysis
	analyzer analysis.Analyzer
	// DefaultField is the default field to search
	defaultField string
	// DefaultOperator is the default boolean operator (AND or OR)
	defaultOperator string
	// PhraseSlop is the default slop for phrase queries
	phraseSlop int
	// AllowLeadingWildcard allows wildcards at the beginning of terms
	allowLeadingWildcard bool
	// LowercaseExpandedTerms converts expanded terms to lowercase
	lowercaseExpandedTerms bool
	// MultiTermRewriteMethod is the rewrite method for multi-term queries
	multiTermRewriteMethod string
}

// NewStandardQueryConfigHandler creates a new StandardQueryConfigHandler.
func NewStandardQueryConfigHandler() *StandardQueryConfigHandler {
	return &StandardQueryConfigHandler{
		analyzer:               nil,
		defaultField:           "",
		defaultOperator:        "OR",
		phraseSlop:             0,
		allowLeadingWildcard:   false,
		lowercaseExpandedTerms: true,
		multiTermRewriteMethod: "constant_score",
	}
}

// GetAnalyzer returns the analyzer.
func (h *StandardQueryConfigHandler) GetAnalyzer() analysis.Analyzer {
	return h.analyzer
}

// SetAnalyzer sets the analyzer.
func (h *StandardQueryConfigHandler) SetAnalyzer(analyzer analysis.Analyzer) {
	h.analyzer = analyzer
}

// GetDefaultField returns the default field.
func (h *StandardQueryConfigHandler) GetDefaultField() string {
	return h.defaultField
}

// SetDefaultField sets the default field.
func (h *StandardQueryConfigHandler) SetDefaultField(field string) {
	h.defaultField = field
}

// GetDefaultOperator returns the default operator.
func (h *StandardQueryConfigHandler) GetDefaultOperator() string {
	return h.defaultOperator
}

// SetDefaultOperator sets the default operator (AND or OR).
func (h *StandardQueryConfigHandler) SetDefaultOperator(operator string) {
	if operator == "AND" || operator == "OR" {
		h.defaultOperator = operator
	}
}

// GetPhraseSlop returns the phrase slop.
func (h *StandardQueryConfigHandler) GetPhraseSlop() int {
	return h.phraseSlop
}

// SetPhraseSlop sets the phrase slop.
func (h *StandardQueryConfigHandler) SetPhraseSlop(slop int) {
	h.phraseSlop = slop
}

// IsAllowLeadingWildcard returns true if leading wildcards are allowed.
func (h *StandardQueryConfigHandler) IsAllowLeadingWildcard() bool {
	return h.allowLeadingWildcard
}

// SetAllowLeadingWildcard sets whether leading wildcards are allowed.
func (h *StandardQueryConfigHandler) SetAllowLeadingWildcard(allow bool) {
	h.allowLeadingWildcard = allow
}

// IsLowercaseExpandedTerms returns true if expanded terms should be lowercased.
func (h *StandardQueryConfigHandler) IsLowercaseExpandedTerms() bool {
	return h.lowercaseExpandedTerms
}

// SetLowercaseExpandedTerms sets whether expanded terms should be lowercased.
func (h *StandardQueryConfigHandler) SetLowercaseExpandedTerms(lowercase bool) {
	h.lowercaseExpandedTerms = lowercase
}

// StandardSyntaxParser parses query strings into QueryNode trees.
type StandardSyntaxParser struct {
	config *StandardQueryConfigHandler
}

// NewStandardSyntaxParser creates a new StandardSyntaxParser.
func NewStandardSyntaxParser(config *StandardQueryConfigHandler) *StandardSyntaxParser {
	return &StandardSyntaxParser{
		config: config,
	}
}

// Parse parses a query string into a QueryNode tree.
func (p *StandardSyntaxParser) Parse(query string) (QueryNode, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query string")
	}

	parser := newQueryStringParser(query, p.config)
	return parser.parse()
}

// queryStringParser is a recursive descent parser for query strings.
type queryStringParser struct {
	input  string
	pos    int
	config *StandardQueryConfigHandler
}

// newQueryStringParser creates a new queryStringParser.
func newQueryStringParser(input string, config *StandardQueryConfigHandler) *queryStringParser {
	return &queryStringParser{
		input:  input,
		pos:    0,
		config: config,
	}
}

// parse parses the entire query string.
func (p *queryStringParser) parse() (QueryNode, error) {
	p.skipWhitespace()
	if p.isAtEnd() {
		return nil, fmt.Errorf("empty query")
	}

	return p.parseOrExpression()
}

// parseOrExpression parses OR expressions (lowest precedence).
func (p *queryStringParser) parseOrExpression() (QueryNode, error) {
	left, err := p.parseAndExpression()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.matchString("OR") || p.matchString("||") {
			right, err := p.parseAndExpression()
			if err != nil {
				return nil, err
			}

			// Create OR node
			orNode := NewOrQueryNode([]QueryNode{left, right})
			left = orNode
		} else {
			break
		}
	}

	return left, nil
}

// parseAndExpression parses AND expressions.
func (p *queryStringParser) parseAndExpression() (QueryNode, error) {
	left, err := p.parseNotExpression()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.matchString("AND") || p.matchString("&&") {
			right, err := p.parseNotExpression()
			if err != nil {
				return nil, err
			}

			// Create AND node
			andNode := NewAndQueryNode([]QueryNode{left, right})
			left = andNode
		} else if p.peek() != ')' && p.peek() != 0 {
			// Implicit AND (whitespace between terms)
			if p.config.GetDefaultOperator() == "AND" {
				right, err := p.parseNotExpression()
				if err != nil {
					return nil, err
				}
				andNode := NewAndQueryNode([]QueryNode{left, right})
				left = andNode
			} else {
				break
			}
		} else {
			break
		}
	}

	return left, nil
}

// parseNotExpression parses NOT expressions.
func (p *queryStringParser) parseNotExpression() (QueryNode, error) {
	p.skipWhitespace()

	// Check for NOT operator
	if p.matchString("NOT") || p.matchString("!") {
		child, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}

		// Create modifier node with prohibited modifier
		modifierNode := NewModifierQueryNode(child, ModifierProhibited)
		return modifierNode, nil
	}

	// Check for + (required) and - (prohibited) modifiers
	if p.peek() == '+' {
		p.advance()
		child, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return NewModifierQueryNode(child, ModifierRequired), nil
	}

	if p.peek() == '-' {
		p.advance()
		child, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return NewModifierQueryNode(child, ModifierProhibited), nil
	}

	return p.parsePrimary()
}

// parsePrimary parses primary expressions (terms, phrases, groups, etc.).
func (p *queryStringParser) parsePrimary() (QueryNode, error) {
	p.skipWhitespace()

	if p.isAtEnd() {
		return nil, fmt.Errorf("unexpected end of query")
	}

	ch := p.peek()

	// Grouped expression
	if ch == '(' {
		return p.parseGroup()
	}

	// Phrase (quoted)
	if ch == '"' {
		return p.parsePhrase()
	}

	// Range query
	if ch == '[' || ch == '{' {
		return p.parseRange()
	}

	// Fielded query (field:value)
	return p.parseFieldOrTerm()
}

// parseGroup parses a grouped expression.
func (p *queryStringParser) parseGroup() (QueryNode, error) {
	if !p.match('(') {
		return nil, fmt.Errorf("expected '('")
	}

	p.skipWhitespace()
	child, err := p.parseOrExpression()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()
	if !p.match(')') {
		return nil, fmt.Errorf("expected ')'")
	}

	return NewGroupQueryNode(child), nil
}

// parsePhrase parses a quoted phrase.
func (p *queryStringParser) parsePhrase() (QueryNode, error) {
	if !p.match('"') {
		return nil, fmt.Errorf("expected '\"'")
	}

	start := p.pos
	var text strings.Builder

	for !p.isAtEnd() && p.peek() != '"' {
		text.WriteRune(p.advance())
	}

	if !p.match('"') {
		return nil, fmt.Errorf("unterminated phrase")
	}

	end := p.pos

	// Check for slop (~N)
	slop := p.config.GetPhraseSlop()
	p.skipWhitespace()
	if p.match('~') {
		slopStr := p.parseNumber()
		if slopStr != "" {
			s, err := strconv.Atoi(slopStr)
			if err == nil {
				slop = s
			}
		}
	}

	return NewPhraseSlopQueryNode(p.config.GetDefaultField(), text.String(), slop, start, end), nil
}

// parseRange parses a range query [a TO b] or {a TO b}.
func (p *queryStringParser) parseRange() (QueryNode, error) {
	// Get lower bound type
	lowerBound := BoundExclusive
	if p.match('[') {
		lowerBound = BoundInclusive
	} else if p.match('{') {
		lowerBound = BoundExclusive
	} else {
		return nil, fmt.Errorf("expected '[' or '{'")
	}

	// Parse lower term
	p.skipWhitespace()
	lower := p.parseTerm()
	if lower == "" {
		return nil, fmt.Errorf("expected lower bound term")
	}

	// Parse TO
	p.skipWhitespace()
	if !p.matchString("TO") {
		return nil, fmt.Errorf("expected 'TO'")
	}

	// Parse upper term
	p.skipWhitespace()
	upper := p.parseTerm()
	if upper == "" {
		return nil, fmt.Errorf("expected upper bound term")
	}

	p.skipWhitespace()

	// Get upper bound type
	upperBound := BoundExclusive
	if p.match(']') {
		upperBound = BoundInclusive
	} else if p.match('}') {
		upperBound = BoundExclusive
	} else {
		return nil, fmt.Errorf("expected ']' or '}'")
	}

	return NewRangeQueryNode(p.config.GetDefaultField(), lower, upper, lowerBound, upperBound), nil
}

// parseFieldOrTerm parses a fielded query or a simple term.
func (p *queryStringParser) parseFieldOrTerm() (QueryNode, error) {
	start := p.pos
	field := p.config.GetDefaultField()

	// Parse first part
	part1 := p.parseTerm()
	if part1 == "" {
		return nil, fmt.Errorf("expected term")
	}

	// Check for field separator
	p.skipWhitespace()
	if p.match(':') {
		// This is a fielded query
		field = part1
		p.skipWhitespace()

		// Check for special query types
		if p.peek() == '(' {
			// Subquery
			return p.parseGroupWithField(field)
		}

		if p.peek() == '"' {
			// Phrase
			return p.parsePhraseWithField(field)
		}

		if p.peek() == '[' || p.peek() == '{' {
			// Range
			return p.parseRangeWithField(field)
		}

		// Regular term
		term := p.parseTerm()
		if term == "" {
			return nil, fmt.Errorf("expected term after field:")
		}

		end := p.pos
		return p.createTermNode(field, term, start, end)
	}

	// This is a simple term in the default field
	end := p.pos
	return p.createTermNode(field, part1, start, end)
}

// parseGroupWithField parses a grouped expression with a specified field.
func (p *queryStringParser) parseGroupWithField(field string) (QueryNode, error) {
	groupNode, err := p.parseGroup()
	if err != nil {
		return nil, err
	}

	// Set field on all children
	// This is a simplified approach - in practice, you'd recursively set the field
	_ = field
	return groupNode, nil
}

// parsePhraseWithField parses a phrase with a specified field.
func (p *queryStringParser) parsePhraseWithField(field string) (QueryNode, error) {
	phraseNode, err := p.parsePhrase()
	if err != nil {
		return nil, err
	}

	// Update the field
	if psn, ok := phraseNode.(*PhraseSlopQueryNode); ok {
		psn.SetField(field)
	}

	return phraseNode, nil
}

// parseRangeWithField parses a range with a specified field.
func (p *queryStringParser) parseRangeWithField(field string) (QueryNode, error) {
	rangeNode, err := p.parseRange()
	if err != nil {
		return nil, err
	}

	// Update the field
	if rn, ok := rangeNode.(*RangeQueryNode); ok {
		rn.SetField(field)
	}

	return rangeNode, nil
}

// createTermNode creates a term node, handling wildcards and fuzzy.
func (p *queryStringParser) createTermNode(field, term string, start, end int) (QueryNode, error) {
	// Check for wildcard
	if strings.ContainsAny(term, "*?") {
		return NewFieldQueryNode(field, term, start, end), nil
	}

	// Check for fuzzy (~)
	if strings.HasSuffix(term, "~") {
		baseTerm := term[:len(term)-1]
		return NewFuzzyQueryNode(field, baseTerm, 0.5, 0, start, end), nil
	}

	// Check for fuzzy with similarity (~N)
	if idx := strings.LastIndex(term, "~"); idx > 0 && idx < len(term)-1 {
		baseTerm := term[:idx]
		simStr := term[idx+1:]
		if sim, err := strconv.ParseFloat(simStr, 64); err == nil {
			return NewFuzzyQueryNode(field, baseTerm, sim, 0, start, end), nil
		}
	}

	// Check for boost (^)
	if idx := strings.LastIndex(term, "^"); idx > 0 {
		baseTerm := term[:idx]
		boostStr := term[idx+1:]
		if boost, err := strconv.ParseFloat(boostStr, 64); err == nil {
			child := NewFieldQueryNode(field, baseTerm, start, end)
			return NewBoostQueryNode(child, boost), nil
		}
	}

	// Regular term
	return NewFieldQueryNode(field, term, start, end), nil
}

// parseTerm parses a single term.
func (p *queryStringParser) parseTerm() string {
	var term strings.Builder

	for !p.isAtEnd() {
		ch := p.peek()
		if unicode.IsSpace(rune(ch)) || ch == ':' || ch == ')' || ch == ']' || ch == '}' ||
			ch == '(' || ch == '[' || ch == '{' || ch == '"' || ch == '+' || ch == '-' ||
			ch == '~' || ch == '^' {
			break
		}
		term.WriteRune(p.advance())
	}

	return term.String()
}

// parseNumber parses a number.
func (p *queryStringParser) parseNumber() string {
	var num strings.Builder

	for !p.isAtEnd() && unicode.IsDigit(rune(p.peek())) {
		num.WriteRune(p.advance())
	}

	return num.String()
}

// skipWhitespace skips whitespace characters.
func (p *queryStringParser) skipWhitespace() {
	for !p.isAtEnd() && unicode.IsSpace(rune(p.peek())) {
		p.advance()
	}
}

// peek returns the current character without consuming it.
func (p *queryStringParser) peek() rune {
	if p.isAtEnd() {
		return 0
	}
	return rune(p.input[p.pos])
}

// advance consumes and returns the current character.
func (p *queryStringParser) advance() rune {
	if p.isAtEnd() {
		return 0
	}
	ch := p.input[p.pos]
	p.pos++
	return rune(ch)
}

// isAtEnd returns true if we've reached the end of input.
func (p *queryStringParser) isAtEnd() bool {
	return p.pos >= len(p.input)
}

// match consumes the expected character if it matches.
func (p *queryStringParser) match(expected rune) bool {
	if p.isAtEnd() {
		return false
	}
	if rune(p.input[p.pos]) != expected {
		return false
	}
	p.pos++
	return true
}

// matchString consumes the expected string if it matches.
func (p *queryStringParser) matchString(expected string) bool {
	if p.pos+len(expected) > len(p.input) {
		return false
	}
	if p.input[p.pos:p.pos+len(expected)] != expected {
		return false
	}
	// Make sure it's a complete token (followed by whitespace or end)
	nextPos := p.pos + len(expected)
	if nextPos < len(p.input) {
		nextChar := p.input[nextPos]
		if !unicode.IsSpace(rune(nextChar)) && nextChar != ':' && nextChar != ')' &&
			nextChar != ']' && nextChar != '}' {
			return false
		}
	}
	p.pos = nextPos
	return true
}

// StandardQueryNodeProcessorPipeline creates the standard processor pipeline.
type StandardQueryNodeProcessorPipeline struct {
	*QueryNodeProcessorPipeline
}

// NewStandardQueryNodeProcessorPipeline creates a new StandardQueryNodeProcessorPipeline.
func NewStandardQueryNodeProcessorPipeline() *StandardQueryNodeProcessorPipeline {
	processors := []QueryNodeProcessor{
		NewPhraseSlopQueryNodeProcessor(),
		NewBoostQueryNodeProcessor(),
		NewNoChildOptimizationProcessor(),
	}

	return &StandardQueryNodeProcessorPipeline{
		QueryNodeProcessorPipeline: NewQueryNodeProcessorPipeline(processors),
	}
}

// StandardQueryTreeBuilder creates the standard query tree builder.
type StandardQueryTreeBuilder struct {
	*QueryTreeBuilder
}

// NewStandardQueryTreeBuilder creates a new StandardQueryTreeBuilder.
func NewStandardQueryTreeBuilder() *StandardQueryTreeBuilder {
	treeBuilder := NewQueryTreeBuilder()

	// Register all builders
	treeBuilder.SetBuilder("FieldQueryNode", NewFieldQueryNodeBuilder())
	treeBuilder.SetBuilder("BooleanQueryNode", NewBooleanQueryNodeBuilder(treeBuilder))
	treeBuilder.SetBuilder("AndQueryNode", NewBooleanQueryNodeBuilder(treeBuilder))
	treeBuilder.SetBuilder("OrQueryNode", NewBooleanQueryNodeBuilder(treeBuilder))
	treeBuilder.SetBuilder("ModifierQueryNode", NewBooleanQueryNodeBuilder(treeBuilder))
	treeBuilder.SetBuilder("BoostQueryNode", NewBoostQueryNodeBuilder(treeBuilder))
	treeBuilder.SetBuilder("FuzzyQueryNode", NewFuzzyQueryNodeBuilder())
	treeBuilder.SetBuilder("RangeQueryNode", NewRangeQueryNodeBuilder())
	treeBuilder.SetBuilder("PhraseSlopQueryNode", NewPhraseQueryNodeBuilder())
	treeBuilder.SetBuilder("GroupQueryNode", NewGroupQueryNodeBuilder(treeBuilder))
	treeBuilder.SetBuilder("MatchAllDocsQueryNode", NewMatchAllDocsQueryNodeBuilder())
	treeBuilder.SetBuilder("MatchNoDocsQueryNode", NewMatchNoDocsQueryNodeBuilder())

	return &StandardQueryTreeBuilder{
		QueryTreeBuilder: treeBuilder,
	}
}

// StandardQueryParser is the main entry point for the flexible query parser.
type StandardQueryParser struct {
	config     *StandardQueryConfigHandler
	syntax     *StandardSyntaxParser
	processors *StandardQueryNodeProcessorPipeline
	builder    *StandardQueryTreeBuilder
}

// NewStandardQueryParser creates a new StandardQueryParser.
func NewStandardQueryParser() *StandardQueryParser {
	config := NewStandardQueryConfigHandler()

	return &StandardQueryParser{
		config:     config,
		syntax:     NewStandardSyntaxParser(config),
		processors: NewStandardQueryNodeProcessorPipeline(),
		builder:    NewStandardQueryTreeBuilder(),
	}
}

// Parse parses a query string and returns a Lucene Query.
func (p *StandardQueryParser) Parse(query string) (search.Query, error) {
	// Parse the query string into a QueryNode tree
	queryTree, err := p.syntax.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	if queryTree == nil {
		return search.NewMatchNoDocsQuery(), nil
	}

	// Process the query tree
	processedTree, err := p.processors.Process(queryTree)
	if err != nil {
		return nil, fmt.Errorf("processing error: %w", err)
	}

	if processedTree == nil {
		return search.NewMatchNoDocsQuery(), nil
	}

	// Build the Lucene Query
	luceneQuery, err := p.builder.Build(processedTree)
	if err != nil {
		return nil, fmt.Errorf("build error: %w", err)
	}

	return luceneQuery, nil
}

// GetConfig returns the configuration handler.
func (p *StandardQueryParser) GetConfig() *StandardQueryConfigHandler {
	return p.config
}

// SetAnalyzer sets the analyzer.
func (p *StandardQueryParser) SetAnalyzer(analyzer analysis.Analyzer) {
	p.config.SetAnalyzer(analyzer)
}

// SetDefaultField sets the default field.
func (p *StandardQueryParser) SetDefaultField(field string) {
	p.config.SetDefaultField(field)
}

// SetDefaultOperator sets the default operator.
func (p *StandardQueryParser) SetDefaultOperator(operator string) {
	p.config.SetDefaultOperator(operator)
}
