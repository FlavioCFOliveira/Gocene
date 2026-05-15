// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.RegExp from Apache Lucene 10.4.0
// (Apache License 2.0, derived from dk.brics.automaton).
//
// Differences from Lucene's RegExp:
//   - Case-insensitive matching flags (ASCII_CASE_INSENSITIVE / CASE_INSENSITIVE
//     / CASE_INSENSITIVE_RANGE) are accepted at construction time but return
//     ErrCaseFoldingUnsupported when used during ToAutomaton; the CaseFolding
//     tables (Unicode 16.0 SpecialCasing/CaseFolding) have not yet been ported.
//   - Errors are returned via the error channel rather than panicking.

package automaton

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// RegExp syntax flag values match Lucene's constants.
const (
	// RegExpIntersection enables the '&' intersection operator.
	RegExpIntersection = 0x0001
	// RegExpEmpty enables '#' as the empty language.
	RegExpEmpty = 0x0004
	// RegExpAnyString enables '@' as the any-string match.
	RegExpAnyString = 0x0008
	// RegExpAutomaton enables named '<name>' automaton lookup.
	RegExpAutomaton = 0x0010
	// RegExpInterval enables numerical '<n-m>' interval matching.
	RegExpInterval = 0x0020
	// RegExpAll enables all optional syntax constructs.
	RegExpAll = 0xff
	// RegExpNone disables all optional syntax constructs.
	RegExpNone = 0x0000

	// RegExpASCIICaseInsensitive (deprecated) — alias for RegExpCaseInsensitive.
	RegExpASCIICaseInsensitive = 0x0100
	// RegExpCaseInsensitive enables Unicode-aware case-folding.
	RegExpCaseInsensitive = 0x0200
	// RegExpCaseInsensitiveRange enables case folding within character ranges.
	RegExpCaseInsensitiveRange = 0x0400
	// RegExpDeprecatedComplement (deprecated) enables the '~' general complement.
	RegExpDeprecatedComplement = 0x10000
)

// ErrCaseFoldingUnsupported is returned when a RegExp uses case-insensitive
// flags but CaseFolding is not yet implemented.
var ErrCaseFoldingUnsupported = errors.New("automaton/regexp: CASE_INSENSITIVE flags require CaseFolding tables (Sprint follow-up)")

// RegExpKind enumerates the node kinds in the parsed RegExp AST.
type RegExpKind int

// RegExpKind values mirror Lucene's RegExp.Kind enum.
const (
	RegExpKindUnion RegExpKind = iota
	RegExpKindConcatenation
	RegExpKindIntersection
	RegExpKindOptional
	RegExpKindRepeat
	RegExpKindRepeatMin
	RegExpKindRepeatMinMax
	RegExpKindComplement
	RegExpKindChar
	RegExpKindCharRange
	RegExpKindCharClass
	RegExpKindAnyChar
	RegExpKindEmpty
	RegExpKindString
	RegExpKindAnyString
	RegExpKindAutomaton
	RegExpKindInterval
	RegExpKindDeprecatedComplement
)

// RegExp is the parsed AST of a Lucene RegExp expression.
type RegExp struct {
	Kind RegExpKind

	Exp1, Exp2 *RegExp
	S          string
	C          int
	Min, Max   int
	Digits     int
	From, To   []int

	originalString string
	flags          int
	pos            int
}

// NewRegExp parses s as a RegExp using the full RegExpAll syntax.
func NewRegExp(s string) (*RegExp, error) {
	return NewRegExpFlags(s, RegExpAll, 0)
}

// NewRegExpSyntax parses s using a custom syntax flag mask.
func NewRegExpSyntax(s string, syntaxFlags int) (*RegExp, error) {
	return NewRegExpFlags(s, syntaxFlags, 0)
}

// NewRegExpFlags parses s under the given syntax and match flags.
func NewRegExpFlags(s string, syntaxFlags, matchFlags int) (*RegExp, error) {
	if (syntaxFlags & ^RegExpDeprecatedComplement) > RegExpAll {
		return nil, fmt.Errorf("regexp: illegal syntax flag 0x%x", syntaxFlags)
	}
	if matchFlags > 0 && matchFlags <= RegExpAll {
		return nil, fmt.Errorf("regexp: illegal match flag 0x%x", matchFlags)
	}
	r := &RegExp{originalString: s, flags: syntaxFlags | matchFlags}
	if len(s) == 0 {
		r.assignFrom(makeRegexpString(r.flags, ""))
		return r, nil
	}
	parsed, err := r.parseUnionExp()
	if err != nil {
		return nil, err
	}
	if r.pos < len(r.originalString) {
		return nil, fmt.Errorf("regexp: end-of-string expected at position %d", r.pos)
	}
	r.assignFrom(parsed)
	return r, nil
}

func (r *RegExp) assignFrom(other *RegExp) {
	r.Kind = other.Kind
	r.Exp1 = other.Exp1
	r.Exp2 = other.Exp2
	r.S = other.S
	r.C = other.C
	r.Min = other.Min
	r.Max = other.Max
	r.Digits = other.Digits
	r.From = other.From
	r.To = other.To
}

// GetOriginalString returns the original regex source.
func (r *RegExp) GetOriginalString() string { return r.originalString }

// ToAutomaton constructs an Automaton from the parsed RegExp with no named
// automata bound. Equivalent to ToAutomatonWith(nil, nil).
func (r *RegExp) ToAutomaton() (*Automaton, error) {
	return r.ToAutomatonWith(nil, nil)
}

// ToAutomatonWith constructs an Automaton consulting the named-automaton map
// first, falling back to provider when unset.
func (r *RegExp) ToAutomatonWith(automata map[string]*Automaton, provider AutomatonProvider) (*Automaton, error) {
	return r.toAutomatonInternal(automata, provider)
}

func (r *RegExp) toAutomatonInternal(automata map[string]*Automaton, provider AutomatonProvider) (*Automaton, error) {
	switch r.Kind {
	case RegExpKindUnion:
		list, err := r.gatherLeaves(RegExpKindUnion, automata, provider)
		if err != nil {
			return nil, err
		}
		return Union(list), nil
	case RegExpKindConcatenation:
		list, err := r.gatherLeaves(RegExpKindConcatenation, automata, provider)
		if err != nil {
			return nil, err
		}
		return Concatenate(list), nil
	case RegExpKindIntersection:
		a1, err := r.Exp1.toAutomatonInternal(automata, provider)
		if err != nil {
			return nil, err
		}
		a2, err := r.Exp2.toAutomatonInternal(automata, provider)
		if err != nil {
			return nil, err
		}
		return Intersection(a1, a2), nil
	case RegExpKindOptional:
		a, err := r.Exp1.toAutomatonInternal(automata, provider)
		if err != nil {
			return nil, err
		}
		return Optional(a), nil
	case RegExpKindRepeat:
		a, err := r.Exp1.toAutomatonInternal(automata, provider)
		if err != nil {
			return nil, err
		}
		return Repeat(a), nil
	case RegExpKindRepeatMin:
		a, err := r.Exp1.toAutomatonInternal(automata, provider)
		if err != nil {
			return nil, err
		}
		return RepeatN(a, r.Min), nil
	case RegExpKindRepeatMinMax:
		a, err := r.Exp1.toAutomatonInternal(automata, provider)
		if err != nil {
			return nil, err
		}
		return RepeatRange(a, r.Min, r.Max), nil
	case RegExpKindComplement:
		a, err := r.Exp1.toAutomatonInternal(automata, provider)
		if err != nil {
			return nil, err
		}
		return Complement(a, int(^uint(0)>>1))
	case RegExpKindDeprecatedComplement:
		a, err := r.Exp1.toAutomatonInternal(automata, provider)
		if err != nil {
			return nil, err
		}
		return Complement(a, DefaultDeterminizeWorkLimit)
	case RegExpKindChar:
		if r.checkAny(RegExpASCIICaseInsensitive | RegExpCaseInsensitive) {
			return nil, ErrCaseFoldingUnsupported
		}
		return MakeChar(r.C), nil
	case RegExpKindCharRange:
		return MakeCharRange(r.From[0], r.To[0]), nil
	case RegExpKindCharClass:
		return MakeCharClass(r.From, r.To), nil
	case RegExpKindAnyChar:
		return MakeAnyChar(), nil
	case RegExpKindEmpty:
		return MakeEmpty(), nil
	case RegExpKindString:
		if r.checkAny(RegExpASCIICaseInsensitive | RegExpCaseInsensitive) {
			return nil, ErrCaseFoldingUnsupported
		}
		return MakeString(r.S), nil
	case RegExpKindAnyString:
		return MakeAnyString(), nil
	case RegExpKindAutomaton:
		var aa *Automaton
		if automata != nil {
			aa = automata[r.S]
		}
		if aa == nil && provider != nil {
			got, err := provider.GetAutomaton(r.S)
			if err != nil {
				return nil, err
			}
			aa = got
		}
		if aa == nil {
			return nil, fmt.Errorf("regexp: named automaton %q not found", r.S)
		}
		return aa, nil
	case RegExpKindInterval:
		return MakeDecimalInterval(r.Min, r.Max, r.Digits)
	}
	return nil, fmt.Errorf("regexp: unhandled kind %d", r.Kind)
}

func (r *RegExp) gatherLeaves(kind RegExpKind, automata map[string]*Automaton, provider AutomatonProvider) ([]*Automaton, error) {
	var list []*Automaton
	var walk func(*RegExp) error
	walk = func(node *RegExp) error {
		if node.Kind == kind {
			if err := walk(node.Exp1); err != nil {
				return err
			}
			return walk(node.Exp2)
		}
		a, err := node.toAutomatonInternal(automata, provider)
		if err != nil {
			return err
		}
		list = append(list, a)
		return nil
	}
	if err := walk(r.Exp1); err != nil {
		return nil, err
	}
	if err := walk(r.Exp2); err != nil {
		return nil, err
	}
	return list, nil
}

// String renders the RegExp back into Lucene's canonical textual form.
func (r *RegExp) String() string {
	var sb strings.Builder
	r.toStringBuilder(&sb)
	return sb.String()
}

func (r *RegExp) toStringBuilder(b *strings.Builder) {
	switch r.Kind {
	case RegExpKindUnion:
		b.WriteByte('(')
		r.Exp1.toStringBuilder(b)
		b.WriteByte('|')
		r.Exp2.toStringBuilder(b)
		b.WriteByte(')')
	case RegExpKindConcatenation:
		r.Exp1.toStringBuilder(b)
		r.Exp2.toStringBuilder(b)
	case RegExpKindIntersection:
		b.WriteByte('(')
		r.Exp1.toStringBuilder(b)
		b.WriteByte('&')
		r.Exp2.toStringBuilder(b)
		b.WriteByte(')')
	case RegExpKindOptional:
		b.WriteByte('(')
		r.Exp1.toStringBuilder(b)
		b.WriteString(")?")
	case RegExpKindRepeat:
		b.WriteByte('(')
		r.Exp1.toStringBuilder(b)
		b.WriteString(")*")
	case RegExpKindRepeatMin:
		b.WriteByte('(')
		r.Exp1.toStringBuilder(b)
		b.WriteByte(')')
		b.WriteByte('{')
		b.WriteString(strconv.Itoa(r.Min))
		b.WriteString(",}")
	case RegExpKindRepeatMinMax:
		b.WriteByte('(')
		r.Exp1.toStringBuilder(b)
		b.WriteByte(')')
		b.WriteByte('{')
		b.WriteString(strconv.Itoa(r.Min))
		b.WriteByte(',')
		b.WriteString(strconv.Itoa(r.Max))
		b.WriteByte('}')
	case RegExpKindComplement, RegExpKindDeprecatedComplement:
		b.WriteString("~(")
		r.Exp1.toStringBuilder(b)
		b.WriteByte(')')
	case RegExpKindChar:
		b.WriteByte('\\')
		b.WriteRune(rune(r.C))
	case RegExpKindCharRange:
		b.WriteString("[\\")
		b.WriteRune(rune(r.From[0]))
		b.WriteString("-\\")
		b.WriteRune(rune(r.To[0]))
		b.WriteByte(']')
	case RegExpKindCharClass:
		b.WriteByte('[')
		for i := range r.From {
			b.WriteByte('\\')
			b.WriteRune(rune(r.From[i]))
			if r.From[i] != r.To[i] {
				b.WriteString("-\\")
				b.WriteRune(rune(r.To[i]))
			}
		}
		b.WriteByte(']')
	case RegExpKindAnyChar:
		b.WriteByte('.')
	case RegExpKindEmpty:
		b.WriteByte('#')
	case RegExpKindString:
		b.WriteByte('"')
		b.WriteString(r.S)
		b.WriteByte('"')
	case RegExpKindAnyString:
		b.WriteByte('@')
	case RegExpKindAutomaton:
		b.WriteByte('<')
		b.WriteString(r.S)
		b.WriteByte('>')
	case RegExpKindInterval:
		s1 := strconv.Itoa(r.Min)
		s2 := strconv.Itoa(r.Max)
		b.WriteByte('<')
		if r.Digits > 0 {
			for i := len(s1); i < r.Digits; i++ {
				b.WriteByte('0')
			}
		}
		b.WriteString(s1)
		b.WriteByte('-')
		if r.Digits > 0 {
			for i := len(s2); i < r.Digits; i++ {
				b.WriteByte('0')
			}
		}
		b.WriteString(s2)
		b.WriteByte('>')
	}
}

// GetIdentifiers returns the set of named-automaton identifiers referenced.
func (r *RegExp) GetIdentifiers() map[string]struct{} {
	out := make(map[string]struct{})
	var walk func(*RegExp)
	walk = func(node *RegExp) {
		if node == nil {
			return
		}
		switch node.Kind {
		case RegExpKindUnion, RegExpKindConcatenation, RegExpKindIntersection:
			walk(node.Exp1)
			walk(node.Exp2)
		case RegExpKindOptional, RegExpKindRepeat, RegExpKindRepeatMin, RegExpKindRepeatMinMax,
			RegExpKindComplement, RegExpKindDeprecatedComplement:
			walk(node.Exp1)
		case RegExpKindAutomaton:
			out[node.S] = struct{}{}
		}
	}
	walk(r)
	return out
}

// --- node constructors (mirroring Lucene's static helpers) ---

func newContainerNode(flags int, kind RegExpKind, exp1, exp2 *RegExp) *RegExp {
	return &RegExp{flags: flags, Kind: kind, Exp1: exp1, Exp2: exp2}
}

func newRepeatingNode(flags int, kind RegExpKind, exp *RegExp, min, max int) *RegExp {
	return &RegExp{flags: flags, Kind: kind, Exp1: exp, Min: min, Max: max}
}

func newLeafNode(flags int, kind RegExpKind, s string, c, min, max, digits int, from, to []int) *RegExp {
	return &RegExp{flags: flags, Kind: kind, S: s, C: c, Min: min, Max: max, Digits: digits, From: from, To: to}
}

func makeUnion(flags int, e1, e2 *RegExp) *RegExp {
	return newContainerNode(flags, RegExpKindUnion, e1, e2)
}

func makeConcatenation(flags int, e1, e2 *RegExp) *RegExp {
	if (e1.Kind == RegExpKindChar || e1.Kind == RegExpKindString) &&
		(e2.Kind == RegExpKindChar || e2.Kind == RegExpKindString) {
		return makeStringPair(flags, e1, e2)
	}
	var r1, r2 *RegExp
	switch {
	case e1.Kind == RegExpKindConcatenation &&
		(e1.Exp2.Kind == RegExpKindChar || e1.Exp2.Kind == RegExpKindString) &&
		(e2.Kind == RegExpKindChar || e2.Kind == RegExpKindString):
		r1 = e1.Exp1
		r2 = makeStringPair(flags, e1.Exp2, e2)
	case (e1.Kind == RegExpKindChar || e1.Kind == RegExpKindString) &&
		e2.Kind == RegExpKindConcatenation &&
		(e2.Exp1.Kind == RegExpKindChar || e2.Exp1.Kind == RegExpKindString):
		r1 = makeStringPair(flags, e1, e2.Exp1)
		r2 = e2.Exp2
	default:
		r1 = e1
		r2 = e2
	}
	return newContainerNode(flags, RegExpKindConcatenation, r1, r2)
}

func makeStringPair(flags int, e1, e2 *RegExp) *RegExp {
	var b strings.Builder
	if e1.Kind == RegExpKindString {
		b.WriteString(e1.S)
	} else {
		b.WriteRune(rune(e1.C))
	}
	if e2.Kind == RegExpKindString {
		b.WriteString(e2.S)
	} else {
		b.WriteRune(rune(e2.C))
	}
	return makeRegexpString(flags, b.String())
}

func makeIntersection(flags int, e1, e2 *RegExp) *RegExp {
	return newContainerNode(flags, RegExpKindIntersection, e1, e2)
}

func makeOptional(flags int, e *RegExp) *RegExp {
	return newContainerNode(flags, RegExpKindOptional, e, nil)
}

func makeRepeatNode(flags int, e *RegExp) *RegExp {
	return newContainerNode(flags, RegExpKindRepeat, e, nil)
}

func makeRepeatMinNode(flags int, e *RegExp, min int) *RegExp {
	return newRepeatingNode(flags, RegExpKindRepeatMin, e, min, 0)
}

func makeRepeatMinMaxNode(flags int, e *RegExp, min, max int) *RegExp {
	return newRepeatingNode(flags, RegExpKindRepeatMinMax, e, min, max)
}

func makeComplement(flags int, e *RegExp) *RegExp {
	return newContainerNode(flags, RegExpKindComplement, e, nil)
}

func makeDeprecatedComplement(flags int, e *RegExp) *RegExp {
	return newContainerNode(flags, RegExpKindDeprecatedComplement, e, nil)
}

func makeCharRegexp(flags, c int) *RegExp {
	return newLeafNode(flags, RegExpKindChar, "", c, 0, 0, 0, nil, nil)
}

func makeCharRangeRegexp(flags, from, to int) (*RegExp, error) {
	if from > to {
		return nil, fmt.Errorf("regexp: invalid range %d > %d", from, to)
	}
	return newLeafNode(flags, RegExpKindCharRange, "", 0, 0, 0, 0, []int{from}, []int{to}), nil
}

func makeCharClassRegexp(flags int, from, to []int) (*RegExp, error) {
	if len(from) != len(to) {
		return nil, fmt.Errorf("regexp: from/to length mismatch")
	}
	for i := range from {
		if from[i] > to[i] {
			return nil, fmt.Errorf("regexp: invalid range %d > %d", from[i], to[i])
		}
	}
	return newLeafNode(flags, RegExpKindCharClass, "", 0, 0, 0, 0, from, to), nil
}

func makeAnyCharRegexp(flags int) *RegExp {
	return newContainerNode(flags, RegExpKindAnyChar, nil, nil)
}

func makeEmptyRegexp(flags int) *RegExp {
	return newContainerNode(flags, RegExpKindEmpty, nil, nil)
}

func makeRegexpString(flags int, s string) *RegExp {
	return newLeafNode(flags, RegExpKindString, s, 0, 0, 0, 0, nil, nil)
}

func makeAnyStringRegexp(flags int) *RegExp {
	return newContainerNode(flags, RegExpKindAnyString, nil, nil)
}

func makeAutomatonRegexp(flags int, s string) *RegExp {
	return newLeafNode(flags, RegExpKindAutomaton, s, 0, 0, 0, 0, nil, nil)
}

func makeIntervalRegexp(flags, min, max, digits int) *RegExp {
	return newLeafNode(flags, RegExpKindInterval, "", 0, min, max, digits, nil, nil)
}

// --- parser state ---

func (r *RegExp) check(flag int) bool    { return r.flags&flag != 0 }
func (r *RegExp) checkAny(mask int) bool { return r.flags&mask != 0 }

func (r *RegExp) more() bool { return r.pos < len(r.originalString) }

func (r *RegExp) peek(chars string) bool {
	if !r.more() {
		return false
	}
	cp, _ := utf8.DecodeRuneInString(r.originalString[r.pos:])
	for _, c := range chars {
		if c == cp {
			return true
		}
	}
	return false
}

func (r *RegExp) match(c int) bool {
	if r.pos >= len(r.originalString) {
		return false
	}
	cp, size := utf8.DecodeRuneInString(r.originalString[r.pos:])
	if int(cp) == c {
		r.pos += size
		return true
	}
	return false
}

func (r *RegExp) next() (int, error) {
	if !r.more() {
		return 0, fmt.Errorf("regexp: unexpected end-of-string")
	}
	cp, size := utf8.DecodeRuneInString(r.originalString[r.pos:])
	r.pos += size
	return int(cp), nil
}

func (r *RegExp) parseUnionExp() (*RegExp, error) {
	return r.iterativeParse(r.parseInterExp, func() bool { return r.match('|') }, makeUnion)
}

func (r *RegExp) parseInterExp() (*RegExp, error) {
	return r.iterativeParse(r.parseConcatExp, func() bool {
		return r.check(RegExpIntersection) && r.match('&')
	}, makeIntersection)
}

func (r *RegExp) parseConcatExp() (*RegExp, error) {
	return r.iterativeParse(r.parseRepeatExp, func() bool {
		return r.more() && !r.peek(")|") && (!r.check(RegExpIntersection) || !r.peek("&"))
	}, makeConcatenation)
}

func (r *RegExp) iterativeParse(
	gather func() (*RegExp, error),
	stop func() bool,
	reduce func(int, *RegExp, *RegExp) *RegExp,
) (*RegExp, error) {
	result, err := gather()
	if err != nil {
		return nil, err
	}
	for stop() {
		next, err := gather()
		if err != nil {
			return nil, err
		}
		result = reduce(r.flags, result, next)
	}
	return result, nil
}

func (r *RegExp) parseRepeatExp() (*RegExp, error) {
	e, err := r.parseComplExp()
	if err != nil {
		return nil, err
	}
	for r.peek("?*+{") {
		switch {
		case r.match('?'):
			e = makeOptional(r.flags, e)
		case r.match('*'):
			e = makeRepeatNode(r.flags, e)
		case r.match('+'):
			e = makeRepeatMinNode(r.flags, e, 1)
		case r.match('{'):
			start := r.pos
			for r.peek("0123456789") {
				if _, err := r.next(); err != nil {
					return nil, err
				}
			}
			if start == r.pos {
				return nil, fmt.Errorf("regexp: integer expected at position %d", r.pos)
			}
			n, err := strconv.Atoi(r.originalString[start:r.pos])
			if err != nil {
				return nil, err
			}
			m := -1
			if r.match(',') {
				start = r.pos
				for r.peek("0123456789") {
					if _, err := r.next(); err != nil {
						return nil, err
					}
				}
				if start != r.pos {
					m, err = strconv.Atoi(r.originalString[start:r.pos])
					if err != nil {
						return nil, err
					}
				}
			} else {
				m = n
			}
			if !r.match('}') {
				return nil, fmt.Errorf("regexp: expected '}' at position %d", r.pos)
			}
			if m != -1 && n > m {
				return nil, fmt.Errorf("regexp: invalid repetition range %d..%d", n, m)
			}
			if m == -1 {
				e = makeRepeatMinNode(r.flags, e, n)
			} else {
				e = makeRepeatMinMaxNode(r.flags, e, n, m)
			}
		}
	}
	return e, nil
}

func (r *RegExp) parseComplExp() (*RegExp, error) {
	if r.check(RegExpDeprecatedComplement) && r.match('~') {
		inner, err := r.parseComplExp()
		if err != nil {
			return nil, err
		}
		return makeDeprecatedComplement(r.flags, inner), nil
	}
	return r.parseCharClassExp()
}

func (r *RegExp) parseCharClassExp() (*RegExp, error) {
	if r.match('[') {
		negate := r.match('^')
		e, err := r.parseCharClasses()
		if err != nil {
			return nil, err
		}
		if negate {
			e = makeIntersection(r.flags, makeAnyCharRegexp(r.flags), makeComplement(r.flags, e))
		}
		if !r.match(']') {
			return nil, fmt.Errorf("regexp: expected ']' at position %d", r.pos)
		}
		return e, nil
	}
	return r.parseSimpleExp()
}

func (r *RegExp) parseCharClasses() (*RegExp, error) {
	var starts, ends []int
	for {
		if r.match('\\') {
			if r.peek("\\ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz") {
				if err := r.expandPreDefined(&starts, &ends); err != nil {
					return nil, err
				}
			} else {
				c, err := r.next()
				if err != nil {
					return nil, err
				}
				starts = append(starts, c)
				ends = append(ends, c)
			}
		} else {
			c, err := r.parseCharExp()
			if err != nil {
				return nil, err
			}
			if r.match('-') {
				if r.check(RegExpCaseInsensitiveRange) {
					return nil, ErrCaseFoldingUnsupported
				}
				to, err := r.parseCharExp()
				if err != nil {
					return nil, err
				}
				starts = append(starts, c)
				ends = append(ends, to)
			} else if r.checkAny(RegExpASCIICaseInsensitive | RegExpCaseInsensitive) {
				return nil, ErrCaseFoldingUnsupported
			} else {
				starts = append(starts, c)
				ends = append(ends, c)
			}
		}
		if !(r.more() && !r.peek("]")) {
			break
		}
	}
	if len(starts) == 1 {
		if starts[0] == ends[0] {
			return makeCharRegexp(r.flags, starts[0]), nil
		}
		return makeCharRangeRegexp(r.flags, starts[0], ends[0])
	}
	return makeCharClassRegexp(r.flags, starts, ends)
}

func (r *RegExp) expandPreDefined(starts, ends *[]int) error {
	switch {
	case r.peek("\\"):
		*starts = append(*starts, '\\')
		*ends = append(*ends, '\\')
		_, err := r.next()
		return err
	case r.peek("d"):
		*starts = append(*starts, '0')
		*ends = append(*ends, '9')
		_, err := r.next()
		return err
	case r.peek("D"):
		*starts = append(*starts, MinCodePoint, '9'+1)
		*ends = append(*ends, '0'-1, MaxCodePoint)
		_, err := r.next()
		return err
	case r.peek("s"):
		*starts = append(*starts, '\t', '\r', ' ')
		*ends = append(*ends, '\n', '\r', ' ')
		_, err := r.next()
		return err
	case r.peek("S"):
		*starts = append(*starts, MinCodePoint, '\n'+1, '\r'+1, ' '+1)
		*ends = append(*ends, '\t'-1, '\r'-1, ' '-1, MaxCodePoint)
		_, err := r.next()
		return err
	case r.peek("w"):
		*starts = append(*starts, '0', 'A', '_', 'a')
		*ends = append(*ends, '9', 'Z', '_', 'z')
		_, err := r.next()
		return err
	case r.peek("W"):
		*starts = append(*starts, MinCodePoint, '9'+1, 'Z'+1, '_'+1, 'z'+1)
		*ends = append(*ends, '0'-1, 'A'-1, '_'-1, 'a'-1, MaxCodePoint)
		_, err := r.next()
		return err
	case r.peek("abcefghijklmnopqrtuvxyz") || r.peek("ABCEFGHIJKLMNOPQRTUVXYZ"):
		ch, err := r.next()
		if err != nil {
			return err
		}
		return fmt.Errorf("regexp: invalid character class \\%c", rune(ch))
	}
	return nil
}

func (r *RegExp) matchPredefinedCharacterClass() (*RegExp, error) {
	if r.match('\\') && r.peek("\\ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz") {
		var starts, ends []int
		if err := r.expandPreDefined(&starts, &ends); err != nil {
			return nil, err
		}
		return makeCharClassRegexp(r.flags, starts, ends)
	}
	return nil, nil
}

func (r *RegExp) parseSimpleExp() (*RegExp, error) {
	switch {
	case r.match('.'):
		return makeAnyCharRegexp(r.flags), nil
	case r.check(RegExpEmpty) && r.match('#'):
		return makeEmptyRegexp(r.flags), nil
	case r.check(RegExpAnyString) && r.match('@'):
		return makeAnyStringRegexp(r.flags), nil
	case r.match('"'):
		start := r.pos
		for r.more() && !r.peek("\"") {
			if _, err := r.next(); err != nil {
				return nil, err
			}
		}
		if !r.match('"') {
			return nil, fmt.Errorf("regexp: expected '\"' at position %d", r.pos)
		}
		return makeRegexpString(r.flags, r.originalString[start:r.pos-1]), nil
	case r.match('('):
		if r.match(')') {
			return makeRegexpString(r.flags, ""), nil
		}
		e, err := r.parseUnionExp()
		if err != nil {
			return nil, err
		}
		if !r.match(')') {
			return nil, fmt.Errorf("regexp: expected ')' at position %d", r.pos)
		}
		return e, nil
	case (r.check(RegExpAutomaton) || r.check(RegExpInterval)) && r.match('<'):
		start := r.pos
		for r.more() && !r.peek(">") {
			if _, err := r.next(); err != nil {
				return nil, err
			}
		}
		if !r.match('>') {
			return nil, fmt.Errorf("regexp: expected '>' at position %d", r.pos)
		}
		s := r.originalString[start : r.pos-1]
		i := strings.Index(s, "-")
		if i == -1 {
			if !r.check(RegExpAutomaton) {
				return nil, fmt.Errorf("regexp: interval syntax error at position %d", r.pos-1)
			}
			return makeAutomatonRegexp(r.flags, s), nil
		}
		if !r.check(RegExpInterval) {
			return nil, fmt.Errorf("regexp: illegal identifier at position %d", r.pos-1)
		}
		if i == 0 || i == len(s)-1 || i != strings.LastIndex(s, "-") {
			return nil, fmt.Errorf("regexp: interval syntax error at position %d", r.pos-1)
		}
		minStr := s[:i]
		maxStr := s[i+1:]
		minVal, err := strconv.Atoi(minStr)
		if err != nil {
			return nil, fmt.Errorf("regexp: interval syntax error at position %d", r.pos-1)
		}
		maxVal, err := strconv.Atoi(maxStr)
		if err != nil {
			return nil, fmt.Errorf("regexp: interval syntax error at position %d", r.pos-1)
		}
		digits := 0
		if len(minStr) == len(maxStr) {
			digits = len(minStr)
		}
		if minVal > maxVal {
			minVal, maxVal = maxVal, minVal
		}
		return makeIntervalRegexp(r.flags, minVal, maxVal, digits), nil
	}
	pre, err := r.matchPredefinedCharacterClass()
	if err != nil {
		return nil, err
	}
	if pre != nil {
		return pre, nil
	}
	c, err := r.parseCharExp()
	if err != nil {
		return nil, err
	}
	return makeCharRegexp(r.flags, c), nil
}

func (r *RegExp) parseCharExp() (int, error) {
	r.match('\\')
	return r.next()
}
