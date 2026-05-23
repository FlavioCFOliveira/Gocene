package surround

import (
	"strconv"
	"strings"
)

// QueryParser is the hand-written recursive-descent parser for the surround
// query language. It produces an SrndQuery tree from a string input. Mirrors
// org.apache.lucene.queryparser.surround.parser.QueryParser.
type QueryParser struct {
	tokens       *QueryParserTokenManager
	current      *Token
	buf          [2]*Token // ring-buffer for up to two peeked tokens
	bufLen       int       // number of tokens buffered in buf
	defaultField string
}

// NewQueryParser builds a parser that targets defaultField when no field is
// supplied in the query string.
func NewQueryParser(defaultField string) *QueryParser {
	return &QueryParser{defaultField: defaultField}
}

// Parse parses a surround query string and returns the root SrndQuery node.
func (p *QueryParser) Parse(query string) (SrndQuery, error) {
	p.tokens = NewQueryParserTokenManager(query)
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.current.Kind == EOF {
		return nil, NewParseException("empty surround query")
	}
	tree, err := p.parseOrQuery()
	if err != nil {
		return nil, err
	}
	if p.current.Kind != EOF {
		return nil, NewParseExceptionFromToken(p.current, [][]int{{EOF}}, TokenImage)
	}
	return tree, nil
}

func (p *QueryParser) advance() error {
	if p.bufLen > 0 {
		p.current = p.buf[0]
		p.buf[0] = p.buf[1]
		p.buf[1] = nil
		p.bufLen--
		return nil
	}
	tok, err := p.tokens.NextToken()
	if err != nil {
		return err
	}
	p.current = tok
	return nil
}

// peek returns the k-th look-ahead token (k=1 is the token after current,
// k=2 is two tokens ahead).  Tokens are buffered and drained by advance().
func (p *QueryParser) peek(k int) (*Token, error) {
	for p.bufLen < k {
		tok, err := p.tokens.NextToken()
		if err != nil {
			return nil, err
		}
		p.buf[p.bufLen] = tok
		p.bufLen++
	}
	return p.buf[k-1], nil
}

func (p *QueryParser) parseOrQuery() (SrndQuery, error) {
	first, err := p.parseAndQuery()
	if err != nil {
		return nil, err
	}
	if p.current.Kind != OrOp {
		return first, nil
	}
	children := []SrndQuery{first}
	for p.current.Kind == OrOp {
		if err := p.advance(); err != nil {
			return nil, err
		}
		next, err := p.parseAndQuery()
		if err != nil {
			return nil, err
		}
		children = append(children, next)
	}
	return NewOrQuery(children, true, "OR"), nil
}

func (p *QueryParser) parseAndQuery() (SrndQuery, error) {
	first, err := p.parseNotQuery()
	if err != nil {
		return nil, err
	}
	if p.current.Kind != AndOp {
		return first, nil
	}
	children := []SrndQuery{first}
	for p.current.Kind == AndOp {
		if err := p.advance(); err != nil {
			return nil, err
		}
		next, err := p.parseNotQuery()
		if err != nil {
			return nil, err
		}
		children = append(children, next)
	}
	return NewAndQuery(children, true, "AND"), nil
}

func (p *QueryParser) parseNotQuery() (SrndQuery, error) {
	first, err := p.parseDistanceQuery()
	if err != nil {
		return nil, err
	}
	if p.current.Kind != NotOp {
		return first, nil
	}
	children := []SrndQuery{first}
	for p.current.Kind == NotOp {
		if err := p.advance(); err != nil {
			return nil, err
		}
		next, err := p.parseDistanceQuery()
		if err != nil {
			return nil, err
		}
		children = append(children, next)
	}
	return NewNotQuery(children, true, "NOT"), nil
}

func (p *QueryParser) parseDistanceQuery() (SrndQuery, error) {
	first, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if p.current.Kind != W && p.current.Kind != N {
		return first, nil
	}
	children := []SrndQuery{first}
	var lastOp string
	var lastOrdered bool
	lastDistance := 0
	for p.current.Kind == W || p.current.Kind == N {
		op := p.current.Image
		distance, ordered, err := parseDistanceOp(op)
		if err != nil {
			return nil, err
		}
		lastOp = op
		lastDistance = distance
		lastOrdered = ordered
		if err := p.advance(); err != nil {
			return nil, err
		}
		next, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		children = append(children, next)
	}
	return NewDistanceQuery(children, true, lastDistance, lastOp, lastOrdered), nil
}

func (p *QueryParser) parsePrimary() (SrndQuery, error) {
	switch p.current.Kind {
	case OpenParen:
		if err := p.advance(); err != nil {
			return nil, err
		}
		inner, err := p.parseOrQuery()
		if err != nil {
			return nil, err
		}
		if p.current.Kind != CloseParen {
			return nil, NewParseExceptionFromToken(p.current, [][]int{{CloseParen}}, TokenImage)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return inner, nil
	case Term:
		return p.parseTermOrFieldQuery()
	case Truncterm:
		text := p.current.Image
		if !isTruncAcceptable(text) {
			return nil, NewParseException("Too unrestrictive truncation: " + text)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return NewSrndTruncQuery(text, '*', '?'), nil
	case Suffixterm:
		text := p.current.Image
		if !isPrefixAcceptable(text) {
			return nil, NewParseException("Too unrestrictive truncation: " + text)
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		prefix := strings.TrimSuffix(text, "*")
		return NewSrndPrefixQuery(prefix, false, '*'), nil
	case QuotedToken:
		raw := p.current.Image
		text := strings.Trim(raw, "\"")
		if err := p.advance(); err != nil {
			return nil, err
		}
		return NewSrndTermQuery(text, true), nil
	case W, N:
		// Prefix form: W(a, b, ...) or N(a, b, ...)
		op := p.current
		if err := p.advance(); err != nil {
			return nil, err
		}
		children, err := p.parseFieldsQueryList()
		if err != nil {
			return nil, err
		}
		distance, ordered, err := parseDistanceOp(op.Image)
		if err != nil {
			return nil, err
		}
		return NewDistanceQuery(children, false, distance, op.Image, ordered), nil
	case AndOp:
		// Prefix form: AND(a, b, ...)
		if err := p.advance(); err != nil {
			return nil, err
		}
		children, err := p.parseFieldsQueryList()
		if err != nil {
			return nil, err
		}
		return NewAndQuery(children, false, "AND"), nil
	case OrOp:
		// Prefix form: OR(a, b, ...)
		if err := p.advance(); err != nil {
			return nil, err
		}
		children, err := p.parseFieldsQueryList()
		if err != nil {
			return nil, err
		}
		return NewOrQuery(children, false, "OR"), nil
	default:
		return nil, NewParseExceptionFromToken(p.current, [][]int{{Term, Truncterm, Suffixterm, QuotedToken, OpenParen}}, TokenImage)
	}
}

// parseFieldsQueryList parses "( q1 , q2 , ... )" — at least two items.
// Mirrors Java's FieldsQueryList production which requires one or more
// comma-separated additions after the first element (i.e. ≥ 2 items total).
func (p *QueryParser) parseFieldsQueryList() ([]SrndQuery, error) {
	if p.current.Kind != OpenParen {
		return nil, NewParseExceptionFromToken(p.current, [][]int{{OpenParen}}, TokenImage)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	first, err := p.parseOrQuery()
	if err != nil {
		return nil, err
	}
	// Java grammar requires at least one comma (+ quantifier), so single-item
	// lists are a parse error.
	if p.current.Kind != Comma {
		return nil, NewParseExceptionFromToken(p.current, [][]int{{Comma}}, TokenImage)
	}
	children := []SrndQuery{first}
	for p.current.Kind == Comma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		next, err := p.parseOrQuery()
		if err != nil {
			return nil, err
		}
		children = append(children, next)
	}
	if p.current.Kind != CloseParen {
		return nil, NewParseExceptionFromToken(p.current, [][]int{{CloseParen}}, TokenImage)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	return children, nil
}

func (p *QueryParser) parseTermOrFieldQuery() (SrndQuery, error) {
	first := p.current.Image
	if err := p.advance(); err != nil {
		return nil, err
	}

	// Single-field path: "term:query" — mirrors Java OptionalFields LOOKAHEAD(2).
	if p.current.Kind == Colon {
		if err := p.advance(); err != nil {
			return nil, err
		}
		inner, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return NewFieldsQuery(inner, []string{first}, ','), nil
	}

	// Multi-field comma path: "field1,field2,...:query".
	// We only enter this branch when:
	//   current = COMMA
	//   peek(1) = TERM  (the next field name)
	//   peek(2) = COMMA or COLON  (the sequence continues toward ":")
	// This two-token lookahead prevents the parser from greedily consuming
	// commas that are FieldsQueryList operand separators (e.g. inside W(a,b)).
	// In the W(a,b) case: after "a", current=",", peek(1)="b" (Term),
	// peek(2)=")" (CloseParen) — does NOT satisfy the condition, so we fall
	// through to the plain-term path and let FieldsQueryList own the comma.
	if p.current.Kind == Comma {
		p1, err := p.peek(1)
		if err != nil {
			return nil, err
		}
		if p1.Kind == Term {
			p2, err := p.peek(2)
			if err != nil {
				return nil, err
			}
			if p2.Kind == Comma || p2.Kind == Colon {
				fields := []string{first}
				for p.current.Kind == Comma {
					// Peek: next token must be a TERM followed by COMMA or COLON.
					nxt1, err2 := p.peek(1)
					if err2 != nil {
						return nil, err2
					}
					if nxt1.Kind != Term {
						break
					}
					nxt2, err3 := p.peek(2)
					if err3 != nil {
						return nil, err3
					}
					if nxt2.Kind != Comma && nxt2.Kind != Colon {
						break
					}
					// consume comma
					if err := p.advance(); err != nil {
						return nil, err
					}
					// consume term (field name)
					fields = append(fields, p.current.Image)
					if err := p.advance(); err != nil {
						return nil, err
					}
				}
				if p.current.Kind == Colon {
					if err := p.advance(); err != nil {
						return nil, err
					}
					inner, err := p.parsePrimary()
					if err != nil {
						return nil, err
					}
					return NewFieldsQuery(inner, fields, ','), nil
				}
				return nil, NewParseExceptionFromToken(p.current, [][]int{{Colon}}, TokenImage)
			}
		}
	}

	// No field qualifier: plain term with optional boost.
	return p.applyOptionalBoost(NewSrndTermQuery(first, false))
}

func (p *QueryParser) applyOptionalBoost(q SrndQuery) (SrndQuery, error) {
	if p.current.Kind != Caret {
		return q, nil
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.current.Kind != NumberToken {
		return nil, NewParseExceptionFromToken(p.current, [][]int{{NumberToken}}, TokenImage)
	}
	v, err := strconv.ParseFloat(p.current.Image, 32)
	if err != nil {
		return nil, NewParseExceptionWithCause("invalid boost", err)
	}
	q.SetWeight(float32(v))
	if err := p.advance(); err != nil {
		return nil, err
	}
	return q, nil
}

// minimumPrefixLength mirrors Java's MINIMUM_PREFIX_LENGTH = 3.
const minimumPrefixLength = 3

// minimumCharsInTrunc mirrors Java's MINIMUM_CHARS_IN_TRUNC = 3.
const minimumCharsInTrunc = 3

// isPrefixAcceptable reports whether a SUFFIXTERM (e.g. "abc*") has a prefix
// long enough (>= 3 chars). Mirrors isAcceptableAsPrefix in Java.
func isPrefixAcceptable(s string) bool {
	// strip the trailing *
	prefix := strings.TrimSuffix(s, "*")
	return len([]rune(prefix)) >= minimumPrefixLength
}

// isTruncAcceptable reports whether a TRUNCTERM has enough non-wildcard
// characters (>= 3). Mirrors isTruncatedAcceptable in Java.
func isTruncAcceptable(s string) bool {
	count := 0
	for _, c := range s {
		if c != '*' && c != '?' {
			count++
		}
	}
	return count >= minimumCharsInTrunc
}

func parseDistanceOp(op string) (int, bool, error) {
	if len(op) < 2 {
		return 0, false, NewParseException("invalid distance operator: " + op)
	}
	last := op[len(op)-1]
	ordered := last == 'W' || last == 'w'
	n, err := strconv.Atoi(op[:len(op)-1])
	if err != nil {
		return 0, ordered, NewParseExceptionWithCause("invalid distance number", err)
	}
	return n, ordered, nil
}
