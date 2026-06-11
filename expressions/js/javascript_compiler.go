package js

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/expressions"
)

// JavascriptCompiler compiles a simple subset of JavaScript expression
// syntax into an expressions.Expression. The supported grammar is
// "term ((+|-|*|/) term)*" where a term is a number, an identifier, a
// parenthesised expression, or a function call (sqrt, abs, ...). Custom
// functions can be registered via SetFunctions so that expression calls
// resolve through the registry before falling back to built-in functions.
//
// Mirrors the public surface of
// org.apache.lucene.expressions.js.JavascriptCompiler.
type JavascriptCompiler struct {
	functions *FunctionRegistry
}

// Compile parses source into an Expression. Custom functions registered via
// SetFunctions are resolved at compile time and take precedence over built-in
// functions of the same name.
func (c JavascriptCompiler) Compile(source string) (*expressions.Expression, error) {
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
	// Resolve function names to builtins or custom functions now, so that
	// eval() never does a string-based dispatch.
	if err := resolveFunctions(root, c.functions); err != nil {
		return nil, err
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
			// Match Lucene's behavior: division by zero returns +Inf
			// for floating-point division (not 0).
			if l == 0 {
				return 0, nil // 0/0 returns 0 per Lucene convention
			}
			return math.Inf(int(l)), nil
		}
		return l / r, nil
	}
	return 0, fmt.Errorf("javascript: unsupported operator %q", b.op)
}
func (b *binOp) collectVariables() []string {
	return mergeUnique(b.lhs.collectVariables(), b.rhs.collectVariables())
}

type funcCall struct {
	name      string
	args      []node
	builtin   func([]float64) (float64, error)
	customFn  CustomFunction
}

func (f *funcCall) eval(values map[string]float64) (float64, error) {
	if len(f.args) == 0 {
		return 0, fmt.Errorf("expression: %s() requires at least 1 argument (got 0)", f.name)
	}
	resolved := make([]float64, len(f.args))
	for i, a := range f.args {
		v, err := a.eval(values)
		if err != nil {
			return 0, err
		}
		resolved[i] = v
	}

	// Custom function registered at compile time takes precedence over builtins.
	if f.customFn != nil {
		return f.customFn(resolved...)
	}
	if f.builtin != nil {
		return f.builtin(resolved)
	}
	return 0, ErrUnknownFunction(f.name)
}
func (f *funcCall) collectVariables() []string {
	var out []string
	for _, a := range f.args {
		out = mergeUnique(out, a.collectVariables())
	}
	return out
}

// builtinFunctions maps lowercase function names to their implementations.
var builtinFunctions = map[string]func([]float64) (float64, error){
	"sqrt":     func(a []float64) (float64, error) { return Sqrt(a[0]), nil },
	"abs":      func(a []float64) (float64, error) { return Abs(a[0]), nil },
	"log10":    func(a []float64) (float64, error) { return Log10(a[0]), nil },
	"log2":     func(a []float64) (float64, error) { return Log2(a[0]), nil },
	"ln":       func(a []float64) (float64, error) { return Ln(a[0]), nil },
	"logn":     func(a []float64) (float64, error) { return Logn(a[0], a[1]), nil },
	"exp":      func(a []float64) (float64, error) { return Exp(a[0]), nil },
	"sin":      func(a []float64) (float64, error) { return Sin(a[0]), nil },
	"cos":      func(a []float64) (float64, error) { return Cos(a[0]), nil },
	"tan":      func(a []float64) (float64, error) { return Tan(a[0]), nil },
	"asin":     func(a []float64) (float64, error) { return Asin(a[0]), nil },
	"acos":     func(a []float64) (float64, error) { return Acos(a[0]), nil },
	"atan":     func(a []float64) (float64, error) { return Atan(a[0]), nil },
	"atan2":    func(a []float64) (float64, error) { return Atan2(a[0], a[1]), nil },
	"sinh":     func(a []float64) (float64, error) { return Sinh(a[0]), nil },
	"cosh":     func(a []float64) (float64, error) { return Cosh(a[0]), nil },
	"tanh":     func(a []float64) (float64, error) { return Tanh(a[0]), nil },
	"asinh":    func(a []float64) (float64, error) { return Asinh(a[0]), nil },
	"acosh":    func(a []float64) (float64, error) { return Acosh(a[0]), nil },
	"atanh":    func(a []float64) (float64, error) { return Atanh(a[0]), nil },
	"ceil":     func(a []float64) (float64, error) { return Ceil(a[0]), nil },
	"floor":    func(a []float64) (float64, error) { return Floor(a[0]), nil },
	"round":    func(a []float64) (float64, error) { return Round(a[0]), nil },
	"haversin": func(a []float64) (float64, error) { return Haversin(a[0], a[1], a[2], a[3]), nil },
	"pow":      func(a []float64) (float64, error) { return Pow(a[0], a[1]), nil },
	"max":      func(a []float64) (float64, error) { return Max(a[0], a[1]), nil },
	"min":      func(a []float64) (float64, error) { return Min(a[0], a[1]), nil },
}

// resolveFunctions walks the AST and resolves function calls to either a
// custom function (from reg, taking precedence) or a built-in function.
func resolveFunctions(n node, reg *FunctionRegistry) error {
	switch t := n.(type) {
	case *numLit, *ident:
		return nil
	case *binOp:
		if err := resolveFunctions(t.lhs, reg); err != nil {
			return err
		}
		return resolveFunctions(t.rhs, reg)
	case *funcCall:
		if reg != nil {
			if fn := reg.Lookup(t.name); fn != nil {
				t.customFn = fn
			}
		}
		if t.customFn == nil {
			t.builtin = builtinFunctions[t.name]
		}
		if t.customFn == nil && t.builtin == nil {
			return ErrUnknownFunction(t.name)
		}
		for _, a := range t.args {
			if err := resolveFunctions(a, reg); err != nil {
				return err
			}
		}
	}
	return nil
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
