package js

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/expressions"
)

// JavascriptCompiler compiles a simple subset of JavaScript expression
// syntax into an expressions.Expression. The supported grammar is
// "term ((+|-|*|/) term)*" where a term is a number, an identifier, a
// parenthesised expression, or a function call (sqrt, abs, ...). Mirrors
// the public surface of
// org.apache.lucene.expressions.js.JavascriptCompiler.
type JavascriptCompiler struct{}

// Compile parses source into an Expression.
func (JavascriptCompiler) Compile(source string) (*expressions.Expression, error) {
	p := &parser{src: source, pos: 0}
	root, err := p.parseExpression()
	if err != nil {
		return nil, fmt.Errorf("javascript compile error: %w", err)
	}
	if p.pos < len(p.src) {
		p.skipWhitespace()
		if p.pos < len(p.src) {
			return nil, fmt.Errorf("javascript compile error: unexpected trailing %q", source[p.pos:])
		}
	}
	vars := root.collectVariables()
	expr := expressions.NewExpression(source, vars, func(values map[string]float64) (float64, error) {
		return root.eval(values)
	})
	return expr, nil
}

type node interface {
	eval(values map[string]float64) (float64, error)
	collectVariables() []string
}

type numLit struct{ v float64 }

func (n *numLit) eval(_ map[string]float64) (float64, error)   { return n.v, nil }
func (n *numLit) collectVariables() []string                   { return nil }

type ident struct{ name string }

func (i *ident) eval(values map[string]float64) (float64, error) {
	if v, ok := values[i.name]; ok {
		return v, nil
	}
	return 0, nil
}
func (i *ident) collectVariables() []string { return []string{i.name} }

type binOp struct {
	op       byte
	lhs, rhs node
}

func (b *binOp) eval(values map[string]float64) (float64, error) {
	l, err := b.lhs.eval(values)
	if err != nil {
		return 0, err
	}
	r, err := b.rhs.eval(values)
	if err != nil {
		return 0, err
	}
	switch b.op {
	case '+':
		return l + r, nil
	case '-':
		return l - r, nil
	case '*':
		return l * r, nil
	case '/':
		if r == 0 {
			return 0, nil
		}
		return l / r, nil
	}
	return 0, fmt.Errorf("javascript: unsupported operator %q", b.op)
}
func (b *binOp) collectVariables() []string {
	return mergeUnique(b.lhs.collectVariables(), b.rhs.collectVariables())
}

type funcCall struct {
	name string
	args []node
}

func (f *funcCall) eval(values map[string]float64) (float64, error) {
	resolved := make([]float64, len(f.args))
	for i, a := range f.args {
		v, err := a.eval(values)
		if err != nil {
			return 0, err
		}
		resolved[i] = v
	}
	switch f.name {
	case "sqrt":
		return Sqrt(resolved[0]), nil
	case "abs":
		return Abs(resolved[0]), nil
	case "log10":
		return Log10(resolved[0]), nil
	case "log2":
		return Log2(resolved[0]), nil
	case "ln":
		return Ln(resolved[0]), nil
	case "logn":
		return Logn(resolved[0], resolved[1]), nil
	case "exp":
		return Exp(resolved[0]), nil
	case "sin":
		return Sin(resolved[0]), nil
	case "cos":
		return Cos(resolved[0]), nil
	case "tan":
		return Tan(resolved[0]), nil
	case "asin":
		return Asin(resolved[0]), nil
	case "acos":
		return Acos(resolved[0]), nil
	case "atan":
		return Atan(resolved[0]), nil
	case "atan2":
		return Atan2(resolved[0], resolved[1]), nil
	case "sinh":
		return Sinh(resolved[0]), nil
	case "cosh":
		return Cosh(resolved[0]), nil
	case "tanh":
		return Tanh(resolved[0]), nil
	case "asinh":
		return Asinh(resolved[0]), nil
	case "acosh":
		return Acosh(resolved[0]), nil
	case "atanh":
		return Atanh(resolved[0]), nil
	case "ceil":
		return Ceil(resolved[0]), nil
	case "floor":
		return Floor(resolved[0]), nil
	case "round":
		return Round(resolved[0]), nil
	case "haversin":
		return Haversin(resolved[0], resolved[1], resolved[2], resolved[3]), nil
	case "pow":
		return Pow(resolved[0], resolved[1]), nil
	case "max":
		return Max(resolved[0], resolved[1]), nil
	case "min":
		return Min(resolved[0], resolved[1]), nil
	}
	return 0, fmt.Errorf("javascript: unknown function %q", f.name)
}
func (f *funcCall) collectVariables() []string {
	var out []string
	for _, a := range f.args {
		out = mergeUnique(out, a.collectVariables())
	}
	return out
}

func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

type parser struct {
	src string
	pos int
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
}

func (p *parser) parseExpression() (node, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.pos >= len(p.src) {
			break
		}
		op := p.src[p.pos]
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: op, lhs: left, rhs: right}
	}
	return left, nil
}

func (p *parser) parseTerm() (node, error) {
	left, err := p.parseFactor()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.pos >= len(p.src) {
			break
		}
		op := p.src[p.pos]
		if op != '*' && op != '/' {
			break
		}
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: op, lhs: left, rhs: right}
	}
	return left, nil
}

func (p *parser) parseFactor() (node, error) {
	p.skipWhitespace()
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unexpected end of input")
	}
	c := p.src[p.pos]
	if c == '(' {
		p.pos++
		inner, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if p.pos >= len(p.src) || p.src[p.pos] != ')' {
			return nil, fmt.Errorf("missing ')' near %q", p.src[p.pos:])
		}
		p.pos++
		return inner, nil
	}
	if isDigit(c) || c == '.' {
		return p.parseNumber()
	}
	if isLetter(c) || c == '_' {
		return p.parseIdentOrCall()
	}
	return nil, fmt.Errorf("unexpected character %q", c)
}

func (p *parser) parseNumber() (node, error) {
	start := p.pos
	for p.pos < len(p.src) && (isDigit(p.src[p.pos]) || p.src[p.pos] == '.') {
		p.pos++
	}
	v, err := strconv.ParseFloat(p.src[start:p.pos], 64)
	if err != nil {
		return nil, err
	}
	return &numLit{v: v}, nil
}

func (p *parser) parseIdentOrCall() (node, error) {
	start := p.pos
	for p.pos < len(p.src) && (isLetter(p.src[p.pos]) || isDigit(p.src[p.pos]) || p.src[p.pos] == '_' || p.src[p.pos] == '.') {
		p.pos++
	}
	name := p.src[start:p.pos]
	p.skipWhitespace()
	if p.pos < len(p.src) && p.src[p.pos] == '(' {
		p.pos++
		var args []node
		for {
			p.skipWhitespace()
			if p.pos < len(p.src) && p.src[p.pos] == ')' {
				p.pos++
				break
			}
			a, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			p.skipWhitespace()
			if p.pos < len(p.src) && p.src[p.pos] == ',' {
				p.pos++
				continue
			}
		}
		return &funcCall{name: strings.ToLower(name), args: args}, nil
	}
	return &ident{name: name}, nil
}

func isDigit(b byte) bool  { return b >= '0' && b <= '9' }
func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }
