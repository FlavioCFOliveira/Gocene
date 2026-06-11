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

// binOp covers +, -, *, /, %, <<, >>, >>>, &, |, ^.
type binOp struct {
	op       byte
	op2      byte // second char for 2-char operators (<<, >>, <=, >=, ==, !=, &&, ||)
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
	switch {
	case b.op == '+':
		return l + r, nil
	case b.op == '-':
		return l - r, nil
	case b.op == '*':
		return l * r, nil
	case b.op == '/':
		if r == 0 {
			if isIntegerOperand(l) && isIntegerOperand(r) {
				if l == 0 {
					return 0, nil
				}
				if l > 0 {
					return float64(MaxInt64), nil
				}
				return float64(-MaxInt64), nil
			}
			if l == 0 {
				return 0, nil
			}
			return math.Inf(int(l)), nil
		}
		return l / r, nil
	case b.op == '%':
		if r == 0 {
			return 0, nil
		}
		return math.Mod(l, r), nil
	case b.op == '<' && b.op2 == '<':
		return ToInt32Float64(float64(ToInt32(l) << uint32(ToUint32(r)))), nil
	case b.op == '>' && b.op2 == '>':
		return ToInt32Float64(float64(ToInt32(l) >> uint32(ToUint32(r)))), nil
	case b.op == '>' && b.op2 == 0:
		return boolToFloat(l > r), nil
	case b.op == '<' && b.op2 == 0:
		return boolToFloat(l < r), nil
	case b.op == '>' && b.op2 == '=':
		return boolToFloat(l >= r), nil
	case b.op == '<' && b.op2 == '=':
		return boolToFloat(l <= r), nil
	case b.op == '=' && b.op2 == '=':
		return boolToFloat(l == r), nil
	case b.op == '!' && b.op2 == '=':
		return boolToFloat(l != r), nil
	case b.op == '&' && b.op2 == '&':
		return boolToFloat(l != 0 && r != 0), nil
	case b.op == '|' && b.op2 == '|':
		return boolToFloat(l != 0 || r != 0), nil
	case b.op == '&':
		return ToInt32Float64(float64(ToInt32(l) & ToInt32(r))), nil
	case b.op == '|':
		return ToInt32Float64(float64(ToInt32(l) | ToInt32(r))), nil
	case b.op == '^':
		return ToInt32Float64(float64(ToInt32(l) ^ ToInt32(r))), nil
	}
	return 0, fmt.Errorf("javascript: unsupported operator %q%c", b.op, b.op2)
}
func (b *binOp) collectVariables() []string {
	return mergeUnique(b.lhs.collectVariables(), b.rhs.collectVariables())
}

// unaryOp covers !, -, +, ~.
type unaryOp struct {
	op  byte
	arg node
}

func (u *unaryOp) eval(values map[string]float64) (float64, error) {
	v, err := u.arg.eval(values)
	if err != nil {
		return 0, err
	}
	switch u.op {
	case '-':
		return -v, nil
	case '!':
		if v == 0 {
			return 1, nil
		}
		return 0, nil
	case '~':
		return ToInt32Float64(float64(^ToInt32(v))), nil
	case '+':
		return v, nil
	}
	return 0, fmt.Errorf("javascript: unsupported unary operator %q", u.op)
}
func (u *unaryOp) collectVariables() []string { return u.arg.collectVariables() }

// condOp covers the ternary conditional (?:).
type condOp struct {
	cond, trueBranch, falseBranch node
}

func (c *condOp) eval(values map[string]float64) (float64, error) {
	cond, err := c.cond.eval(values)
	if err != nil {
		return 0, err
	}
	if cond != 0 {
		return c.trueBranch.eval(values)
	}
	return c.falseBranch.eval(values)
}
func (c *condOp) collectVariables() []string {
	return mergeUnique(
		mergeUnique(c.cond.collectVariables(), c.trueBranch.collectVariables()),
		c.falseBranch.collectVariables(),
	)
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
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
	case *unaryOp:
		return resolveFunctions(t.arg, reg)
	case *condOp:
		if err := resolveFunctions(t.cond, reg); err != nil {
			return err
		}
		if err := resolveFunctions(t.trueBranch, reg); err != nil {
			return err
		}
		return resolveFunctions(t.falseBranch, reg)
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

// peek returns the byte at position pos, or 0 if at end.
func (p *parser) peek() byte {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

// peek2 returns the two-byte prefix at pos, or empty string.
func (p *parser) peek2() string {
	if p.pos+1 >= len(p.src) {
		return ""
	}
	return p.src[p.pos : p.pos+2]
}

// consume advances pos if the current char matches c.
func (p *parser) consume(c byte) bool {
	if p.peek() == c {
		p.pos++
		return true
	}
	return false
}

// parseExpression is the top-level entry: conditional expression.
func (p *parser) parseExpression() (node, error) {
	return p.parseConditional()
}

// parseConditional handles the ternary ?: operator (right-associative).
func (p *parser) parseConditional() (node, error) {
	cond, err := p.parseBooleanOr()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()
	if !p.consume('?') {
		return cond, nil
	}
	trueBranch, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()
	if !p.consume(':') {
		return nil, fmt.Errorf("expected ':' in conditional expression at position %d", p.pos)
	}
	falseBranch, err := p.parseConditional()
	if err != nil {
		return nil, err
	}
	return &condOp{cond: cond, trueBranch: trueBranch, falseBranch: falseBranch}, nil
}

// parseBooleanOr handles || (lowest binary precedence).
func (p *parser) parseBooleanOr() (node, error) {
	left, err := p.parseBooleanAnd()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.peek2() != "||" {
			break
		}
		p.pos += 2
		right, err := p.parseBooleanAnd()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: '|', op2: '|', lhs: left, rhs: right}
	}
	return left, nil
}

// parseBooleanAnd handles &&.
func (p *parser) parseBooleanAnd() (node, error) {
	left, err := p.parseBitwiseOr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.peek2() != "&&" {
			break
		}
		p.pos += 2
		right, err := p.parseBitwiseOr()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: '&', op2: '&', lhs: left, rhs: right}
	}
	return left, nil
}

// parseBitwiseOr handles |.
func (p *parser) parseBitwiseOr() (node, error) {
	left, err := p.parseBitwiseXor()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.peek() != '|' || p.peek2() == "||" {
			break
		}
		p.pos++
		right, err := p.parseBitwiseXor()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: '|', lhs: left, rhs: right}
	}
	return left, nil
}

// parseBitwiseXor handles ^.
func (p *parser) parseBitwiseXor() (node, error) {
	left, err := p.parseBitwiseAnd()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if !p.consume('^') {
			break
		}
		right, err := p.parseBitwiseAnd()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: '^', lhs: left, rhs: right}
	}
	return left, nil
}

// parseBitwiseAnd handles &.
func (p *parser) parseBitwiseAnd() (node, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.peek() != '&' || p.peek2() == "&&" {
			break
		}
		p.pos++
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: '&', lhs: left, rhs: right}
	}
	return left, nil
}

// parseEquality handles == and !=.
func (p *parser) parseEquality() (node, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		op2 := p.peek2()
		if op2 == "==" {
			p.pos += 2
			right, err := p.parseComparison()
			if err != nil {
				return nil, err
			}
			left = &binOp{op: '=', op2: '=', lhs: left, rhs: right}
		} else if op2 == "!=" {
			p.pos += 2
			right, err := p.parseComparison()
			if err != nil {
				return nil, err
			}
			left = &binOp{op: '!', op2: '=', lhs: left, rhs: right}
		} else {
			break
		}
	}
	return left, nil
}

// parseComparison handles <, >, <=, >=.
func (p *parser) parseComparison() (node, error) {
	left, err := p.parseShift()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		op2 := p.peek2()
		switch op2 {
		case "<=", ">=":
			p.pos += 2
			right, err := p.parseShift()
			if err != nil {
				return nil, err
			}
			left = &binOp{op: op2[0], op2: op2[1], lhs: left, rhs: right}
		default:
			if p.peek() == '<' || p.peek() == '>' {
				op := p.peek()
				p.pos++
				right, err := p.parseShift()
				if err != nil {
					return nil, err
				}
				left = &binOp{op: op, lhs: left, rhs: right}
			} else {
				return left, nil
			}
		}
	}
}

// parseShift handles << and >>.
func (p *parser) parseShift() (node, error) {
	left, err := p.parseAddition()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		op2 := p.peek2()
		if op2 == "<<" || op2 == ">>" {
			p.pos += 2
			right, err := p.parseAddition()
			if err != nil {
				return nil, err
			}
			left = &binOp{op: op2[0], op2: op2[1], lhs: left, rhs: right}
		} else {
			break
		}
	}
	return left, nil
}

// parseAddition handles + and -.
func (p *parser) parseAddition() (node, error) {
	left, err := p.parseMultiplication()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		c := p.peek()
		if c != '+' && c != '-' {
			break
		}
		p.pos++
		right, err := p.parseMultiplication()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: c, lhs: left, rhs: right}
	}
	return left, nil
}

// parseMultiplication handles *, /, %.
func (p *parser) parseMultiplication() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		c := p.peek()
		if c != '*' && c != '/' && c != '%' {
			break
		}
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &binOp{op: c, lhs: left, rhs: right}
	}
	return left, nil
}

// parseUnary handles prefix -, +, !, ~.
func (p *parser) parseUnary() (node, error) {
	p.skipWhitespace()
	c := p.peek()
	if c == '-' || c == '+' || c == '!' || c == '~' {
		p.pos++
		arg, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &unaryOp{op: c, arg: arg}, nil
	}
	return p.parsePrimary()
}

// parsePrimary handles numbers, identifiers, function calls, and parenthesised exprs.
func (p *parser) parsePrimary() (node, error) {
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

// isIntegerOperand returns true when x has no fractional part.
func isIntegerOperand(x float64) bool {
	return x == math.Trunc(x)
}
