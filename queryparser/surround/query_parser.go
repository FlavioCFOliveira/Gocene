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
	tok, err := p.tokens.NextToken()
	if err != nil {
		return err
	}
	p.current = tok
	return nil
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
		if err := p.advance(); err != nil {
			return nil, err
		}
		prefix := strings.TrimSuffix(text, "*")
		return NewSrndPrefixQuery(prefix, false, '*'), nil
	case Suffixterm:
		text := p.current.Image
		if err := p.advance(); err != nil {
			return nil, err
		}
		return NewSrndTruncQuery(text, '*', '?'), nil
	case QuotedToken:
		raw := p.current.Image
		text := strings.Trim(raw, "\"")
		if err := p.advance(); err != nil {
			return nil, err
		}
		return NewSrndTermQuery(text, true), nil
	default:
		return nil, NewParseExceptionFromToken(p.current, [][]int{{Term, Truncterm, Suffixterm, QuotedToken, OpenParen}}, TokenImage)
	}
}

func (p *QueryParser) parseTermOrFieldQuery() (SrndQuery, error) {
	first := p.current.Image
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.current.Kind != Colon && p.current.Kind != Comma {
		return p.applyOptionalBoost(NewSrndTermQuery(first, false))
	}
	fields := []string{first}
	for p.current.Kind == Comma {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.current.Kind != Term {
			return nil, NewParseExceptionFromToken(p.current, [][]int{{Term}}, TokenImage)
		}
		fields = append(fields, p.current.Image)
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if p.current.Kind != Colon {
		return nil, NewParseExceptionFromToken(p.current, [][]int{{Colon}}, TokenImage)
	}
	if err := p.advance(); err != nil {
		return nil, err
	}
	inner, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	return NewFieldsQuery(inner, fields, ','), nil
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
