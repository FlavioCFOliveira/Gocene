// Package simple implements Lucene's SimpleQueryParser — a hand-written
// parser for a Google-style query language that never throws on invalid input
// (silent fallback to a MatchNoDocsQuery is preferred over surfacing errors
// to the end user). Mirrors org.apache.lucene.queryparser.simple.SimpleQueryParser.
package simple

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Operator flags map exactly to Lucene's static constants on SimpleQueryParser.
const (
	OpAnd        = 1 << 0
	OpNot        = 1 << 1
	OpOr         = 1 << 2
	OpPrefix     = 1 << 3
	OpPhrase     = 1 << 4
	OpPrecedence = 1 << 5
	OpEscape     = 1 << 6
	OpWhitespace = 1 << 7
	OpFuzzy      = 1 << 8
	OpNear       = 1 << 9
	OpAll        = -1
)

// SimpleQueryParser converts a Google-style query string into a search.Query.
type SimpleQueryParser struct {
	Analyzer        analysis.Analyzer
	Fields          []string
	FieldWeights    map[string]float32
	Flags           int
	DefaultOperator search.Occur
}

// NewSimpleQueryParser builds a parser that searches the supplied fields with
// every operator enabled.
func NewSimpleQueryParser(analyzer analysis.Analyzer, fields ...string) *SimpleQueryParser {
	return &SimpleQueryParser{
		Analyzer:        analyzer,
		Fields:          fields,
		Flags:           OpAll,
		DefaultOperator: search.SHOULD,
	}
}

// NewSimpleQueryParserWithFlags lets callers restrict which operators are
// recognised.
func NewSimpleQueryParserWithFlags(analyzer analysis.Analyzer, fields []string, flags int) *SimpleQueryParser {
	p := NewSimpleQueryParser(analyzer, fields...)
	p.Flags = flags
	return p
}

// Parse converts the query string into a Query. Per Lucene's contract the
// parser is forgiving: unbalanced characters and trailing operators are
// silently ignored.
func (p *SimpleQueryParser) Parse(queryText string) search.Query {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return search.NewMatchNoDocsQuery()
	}
	pos := 0
	q := p.parseExpr(queryText, &pos, 0)
	if q == nil {
		return search.NewMatchNoDocsQuery()
	}
	return q
}

func (p *SimpleQueryParser) parseExpr(s string, pos *int, depth int) search.Query {
	bq := search.NewBooleanQuery()
	for *pos < len(s) {
		p.skipWhitespace(s, pos)
		if *pos >= len(s) {
			break
		}
		c := s[*pos]
		if c == ')' && depth > 0 {
			*pos++
			break
		}
		occur := p.DefaultOperator
		if p.flagSet(OpAnd) && c == '+' {
			occur = search.MUST
			*pos++
		} else if p.flagSet(OpNot) && c == '-' {
			occur = search.MUST_NOT
			*pos++
		} else if p.flagSet(OpOr) && c == '|' {
			occur = search.SHOULD
			*pos++
		}
		p.skipWhitespace(s, pos)
		if *pos >= len(s) {
			break
		}
		var clause search.Query
		c = s[*pos]
		switch {
		case p.flagSet(OpPrecedence) && c == '(':
			*pos++
			clause = p.parseExpr(s, pos, depth+1)
		case p.flagSet(OpPhrase) && c == '"':
			clause = p.parsePhrase(s, pos)
		default:
			clause = p.parseTerm(s, pos)
		}
		if clause == nil {
			continue
		}
		bq.Add(clause, occur)
	}
	clauses := bq.Clauses()
	switch len(clauses) {
	case 0:
		return nil
	case 1:
		if clauses[0].Occur == search.MUST_NOT {
			return bq
		}
		return clauses[0].Query
	default:
		return bq
	}
}

func (p *SimpleQueryParser) parsePhrase(s string, pos *int) search.Query {
	*pos++
	start := *pos
	for *pos < len(s) && s[*pos] != '"' {
		*pos++
	}
	text := s[start:*pos]
	if *pos < len(s) && s[*pos] == '"' {
		*pos++
	}
	slop := 0
	if p.flagSet(OpNear) && *pos < len(s) && s[*pos] == '~' {
		*pos++
		slop = p.parseInt(s, pos)
	}
	terms := p.analyze(text)
	if len(terms) == 0 {
		return nil
	}
	return p.buildPhraseAcrossFields(terms, slop)
}

func (p *SimpleQueryParser) parseTerm(s string, pos *int) search.Query {
	start := *pos
	for *pos < len(s) {
		c := s[*pos]
		if unicode.IsSpace(rune(c)) {
			break
		}
		if p.flagSet(OpPrecedence) && (c == '(' || c == ')') {
			break
		}
		if p.flagSet(OpAnd) && c == '+' {
			break
		}
		if p.flagSet(OpNot) && c == '-' {
			break
		}
		if p.flagSet(OpOr) && c == '|' {
			break
		}
		if c == '~' && (p.flagSet(OpFuzzy) || p.flagSet(OpNear)) {
			break
		}
		if c == '*' && p.flagSet(OpPrefix) {
			break
		}
		*pos++
	}
	text := s[start:*pos]
	if text == "" {
		return nil
	}
	prefix := false
	if p.flagSet(OpPrefix) && *pos < len(s) && s[*pos] == '*' {
		prefix = true
		*pos++
	}
	fuzzy := 0
	if p.flagSet(OpFuzzy) && *pos < len(s) && s[*pos] == '~' {
		*pos++
		fuzzy = p.parseInt(s, pos)
		if fuzzy == 0 {
			fuzzy = 2
		}
	}
	if prefix {
		return p.buildPrefixAcrossFields(text)
	}
	if fuzzy > 0 {
		return p.buildFuzzyAcrossFields(text, fuzzy)
	}
	tokens := p.analyze(text)
	if len(tokens) == 0 {
		return nil
	}
	if len(tokens) == 1 {
		return p.buildTermAcrossFields(tokens[0])
	}
	return p.buildPhraseAcrossFields(tokens, 0)
}

func (p *SimpleQueryParser) parseInt(s string, pos *int) int {
	start := *pos
	for *pos < len(s) && s[*pos] >= '0' && s[*pos] <= '9' {
		*pos++
	}
	if start == *pos {
		return 0
	}
	v, err := strconv.Atoi(s[start:*pos])
	if err != nil {
		return 0
	}
	return v
}

func (p *SimpleQueryParser) skipWhitespace(s string, pos *int) {
	for *pos < len(s) && unicode.IsSpace(rune(s[*pos])) {
		*pos++
	}
}

func (p *SimpleQueryParser) flagSet(flag int) bool { return p.Flags&flag != 0 }

func (p *SimpleQueryParser) analyze(text string) []string {
	tokens := strings.Fields(text)
	if p.Analyzer == nil {
		return tokens
	}
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		out = append(out, strings.ToLower(tok))
	}
	return out
}

func (p *SimpleQueryParser) buildTermAcrossFields(text string) search.Query {
	if len(p.Fields) == 0 {
		return nil
	}
	if len(p.Fields) == 1 {
		q := search.NewTermQuery(index.NewTerm(p.Fields[0], text))
		return p.applyFieldBoost(p.Fields[0], q)
	}
	bq := search.NewBooleanQuery()
	for _, f := range p.Fields {
		q := search.NewTermQuery(index.NewTerm(f, text))
		bq.Add(p.applyFieldBoost(f, q), search.SHOULD)
	}
	return bq
}

func (p *SimpleQueryParser) buildPrefixAcrossFields(text string) search.Query {
	if len(p.Fields) == 0 {
		return nil
	}
	if len(p.Fields) == 1 {
		q := search.NewPrefixQuery(index.NewTerm(p.Fields[0], text))
		return p.applyFieldBoost(p.Fields[0], q)
	}
	bq := search.NewBooleanQuery()
	for _, f := range p.Fields {
		q := search.NewPrefixQuery(index.NewTerm(f, text))
		bq.Add(p.applyFieldBoost(f, q), search.SHOULD)
	}
	return bq
}

func (p *SimpleQueryParser) buildFuzzyAcrossFields(text string, maxEdits int) search.Query {
	if len(p.Fields) == 0 {
		return nil
	}
	if len(p.Fields) == 1 {
		q := search.NewFuzzyQueryWithParams(index.NewTerm(p.Fields[0], text), maxEdits, 0, 50)
		return p.applyFieldBoost(p.Fields[0], q)
	}
	bq := search.NewBooleanQuery()
	for _, f := range p.Fields {
		q := search.NewFuzzyQueryWithParams(index.NewTerm(f, text), maxEdits, 0, 50)
		bq.Add(p.applyFieldBoost(f, q), search.SHOULD)
	}
	return bq
}

func (p *SimpleQueryParser) buildPhraseAcrossFields(tokens []string, slop int) search.Query {
	if len(p.Fields) == 0 || len(tokens) == 0 {
		return nil
	}
	build := func(field string) search.Query {
		terms := make([]*index.Term, len(tokens))
		for i, tok := range tokens {
			terms[i] = index.NewTerm(field, tok)
		}
		pq := search.NewPhraseQueryWithSlop(slop, field, terms...)
		return p.applyFieldBoost(field, pq)
	}
	if len(p.Fields) == 1 {
		return build(p.Fields[0])
	}
	bq := search.NewBooleanQuery()
	for _, f := range p.Fields {
		bq.Add(build(f), search.SHOULD)
	}
	return bq
}

func (p *SimpleQueryParser) applyFieldBoost(field string, q search.Query) search.Query {
	if p.FieldWeights == nil {
		return q
	}
	boost, ok := p.FieldWeights[field]
	if !ok || boost == 1.0 {
		return q
	}
	return search.NewBoostQuery(q, boost)
}
