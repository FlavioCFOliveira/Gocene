// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"regexp"
	"strings"
)

// IntervalFunction is the abstract base for representations of interval
// functions used by the flexible standard query parser.
//
// Each concrete subtype holds the function parameters and implements String()
// to produce the canonical fn:<name>(...) representation. Conversion to an
// actual IntervalsSource is deferred to the intervals execution layer (out of
// scope for the parser module).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.IntervalFunction.
type IntervalFunction interface {
	// String returns the canonical string form of this interval function (e.g.
	// "fn:ordered(a b)"). Required by the Java abstract contract.
	String() string
}

// whitespaceRe matches any whitespace character (used by AnalyzedText).
var whitespaceRe = regexp.MustCompile(`\s`)

// requiresQuotes reports whether a term string must be quoted in an interval
// function expression. Mirrors AnalyzedText.requiresQuotes(String).
func requiresQuotes(term string) bool {
	return whitespaceRe.MatchString(term)
}

// quoteIfNeeded wraps term in double-quotes when it contains whitespace.
func quoteIfNeeded(term string) string {
	if requiresQuotes(term) {
		return `"` + term + `"`
	}
	return term
}

// joinFunctions joins the String() values of a slice of IntervalFunction with
// spaces — the separator used by multi-source functions (Ordered, Unordered,
// Phrase).
func joinFunctions(sources []IntervalFunction) string {
	ss := make([]string, len(sources))
	for i, s := range sources {
		ss[i] = s.String()
	}
	return strings.Join(ss, " ")
}

// NotWithin represents Intervals#notWithin(IntervalsSource, int, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.NotWithin.
type NotWithin struct {
	minuend    IntervalFunction
	positions  int
	subtrahend IntervalFunction
}

// NewNotWithin constructs a NotWithin interval function.
// Both minuend and subtrahend must be non-nil.
func NewNotWithin(minuend IntervalFunction, positions int, subtrahend IntervalFunction) *NotWithin {
	if minuend == nil {
		panic("intervalfn: minuend must not be nil")
	}
	if subtrahend == nil {
		panic("intervalfn: subtrahend must not be nil")
	}
	return &NotWithin{minuend: minuend, positions: positions, subtrahend: subtrahend}
}

// GetMinuend returns the minuend (source) interval function.
func (n *NotWithin) GetMinuend() IntervalFunction { return n.minuend }

// GetPositions returns the position threshold.
func (n *NotWithin) GetPositions() int { return n.positions }

// GetSubtrahend returns the subtrahend (reference) interval function.
func (n *NotWithin) GetSubtrahend() IntervalFunction { return n.subtrahend }

// String returns "fn:notWithin(<minuend> <positions> <subtrahend>)".
func (n *NotWithin) String() string {
	return fmt.Sprintf("fn:notWithin(%s %d %s)", n.minuend, n.positions, n.subtrahend)
}

// Containing represents Intervals#containing(IntervalsSource, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Containing.
type Containing struct {
	big   IntervalFunction
	small IntervalFunction
}

// NewContaining constructs a Containing interval function.
func NewContaining(big, small IntervalFunction) *Containing {
	if big == nil {
		panic("intervalfn: big must not be nil")
	}
	if small == nil {
		panic("intervalfn: small must not be nil")
	}
	return &Containing{big: big, small: small}
}

// GetBig returns the containing (outer) interval function.
func (c *Containing) GetBig() IntervalFunction { return c.big }

// GetSmall returns the contained (inner) interval function.
func (c *Containing) GetSmall() IntervalFunction { return c.small }

// String returns "fn:containing(<big> <small>)".
func (c *Containing) String() string {
	return fmt.Sprintf("fn:containing(%s %s)", c.big, c.small)
}

// MaxGaps represents Intervals#maxgaps(int, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.MaxGaps.
type MaxGaps struct {
	maxGaps int
	source  IntervalFunction
}

// NewMaxGaps constructs a MaxGaps interval function.
func NewMaxGaps(maxGaps int, source IntervalFunction) *MaxGaps {
	if source == nil {
		panic("intervalfn: source must not be nil")
	}
	return &MaxGaps{maxGaps: maxGaps, source: source}
}

// GetMaxGaps returns the maximum number of gaps.
func (m *MaxGaps) GetMaxGaps() int { return m.maxGaps }

// GetSource returns the inner interval function.
func (m *MaxGaps) GetSource() IntervalFunction { return m.source }

// String returns "fn:maxgaps(<maxGaps> <source>)".
func (m *MaxGaps) String() string {
	return fmt.Sprintf("fn:maxgaps(%d %s)", m.maxGaps, m.source)
}

// NonOverlapping represents Intervals#nonOverlapping(IntervalsSource, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.NonOverlapping.
type NonOverlapping struct {
	minuend    IntervalFunction
	subtrahend IntervalFunction
}

// NewNonOverlapping constructs a NonOverlapping interval function.
func NewNonOverlapping(minuend, subtrahend IntervalFunction) *NonOverlapping {
	if minuend == nil {
		panic("intervalfn: minuend must not be nil")
	}
	if subtrahend == nil {
		panic("intervalfn: subtrahend must not be nil")
	}
	return &NonOverlapping{minuend: minuend, subtrahend: subtrahend}
}

// GetMinuend returns the minuend interval function.
func (n *NonOverlapping) GetMinuend() IntervalFunction { return n.minuend }

// GetSubtrahend returns the subtrahend interval function.
func (n *NonOverlapping) GetSubtrahend() IntervalFunction { return n.subtrahend }

// String returns "fn:nonOverlapping(<minuend> <subtrahend>)".
func (n *NonOverlapping) String() string {
	return fmt.Sprintf("fn:nonOverlapping(%s %s)", n.minuend, n.subtrahend)
}

// Overlapping represents Intervals#overlapping(IntervalsSource, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Overlapping.
type Overlapping struct {
	source    IntervalFunction
	reference IntervalFunction
}

// NewOverlapping constructs an Overlapping interval function.
func NewOverlapping(source, reference IntervalFunction) *Overlapping {
	if source == nil {
		panic("intervalfn: source must not be nil")
	}
	if reference == nil {
		panic("intervalfn: reference must not be nil")
	}
	return &Overlapping{source: source, reference: reference}
}

// GetSource returns the source interval function.
func (o *Overlapping) GetSource() IntervalFunction { return o.source }

// GetReference returns the reference interval function.
func (o *Overlapping) GetReference() IntervalFunction { return o.reference }

// String returns "fn:overlapping(<source> <reference>)".
func (o *Overlapping) String() string {
	return fmt.Sprintf("fn:overlapping(%s %s)", o.source, o.reference)
}

// NotContaining represents Intervals#notContaining(IntervalsSource, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.NotContaining.
type NotContaining struct {
	minuend    IntervalFunction
	subtrahend IntervalFunction
}

// NewNotContaining constructs a NotContaining interval function.
func NewNotContaining(minuend, subtrahend IntervalFunction) *NotContaining {
	if minuend == nil {
		panic("intervalfn: minuend must not be nil")
	}
	if subtrahend == nil {
		panic("intervalfn: subtrahend must not be nil")
	}
	return &NotContaining{minuend: minuend, subtrahend: subtrahend}
}

// GetMinuend returns the minuend interval function.
func (n *NotContaining) GetMinuend() IntervalFunction { return n.minuend }

// GetSubtrahend returns the subtrahend interval function.
func (n *NotContaining) GetSubtrahend() IntervalFunction { return n.subtrahend }

// String returns "fn:notContaining(<minuend> <subtrahend>)".
func (n *NotContaining) String() string {
	return fmt.Sprintf("fn:notContaining(%s %s)", n.minuend, n.subtrahend)
}

// MaxWidth represents Intervals#maxwidth(int, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.MaxWidth.
type MaxWidth struct {
	width  int
	source IntervalFunction
}

// NewMaxWidth constructs a MaxWidth interval function.
func NewMaxWidth(width int, source IntervalFunction) *MaxWidth {
	if source == nil {
		panic("intervalfn: source must not be nil")
	}
	return &MaxWidth{width: width, source: source}
}

// GetWidth returns the maximum width.
func (m *MaxWidth) GetWidth() int { return m.width }

// GetSource returns the inner interval function.
func (m *MaxWidth) GetSource() IntervalFunction { return m.source }

// String returns "fn:maxwidth(<width> <source>)".
func (m *MaxWidth) String() string {
	return fmt.Sprintf("fn:maxwidth(%d %s)", m.width, m.source)
}

// ContainedBy represents Intervals#containedBy(IntervalsSource, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.ContainedBy.
type ContainedBy struct {
	small IntervalFunction
	big   IntervalFunction
}

// NewContainedBy constructs a ContainedBy interval function.
func NewContainedBy(small, big IntervalFunction) *ContainedBy {
	if small == nil {
		panic("intervalfn: small must not be nil")
	}
	if big == nil {
		panic("intervalfn: big must not be nil")
	}
	return &ContainedBy{small: small, big: big}
}

// GetSmall returns the inner (contained) interval function.
func (c *ContainedBy) GetSmall() IntervalFunction { return c.small }

// GetBig returns the outer (container) interval function.
func (c *ContainedBy) GetBig() IntervalFunction { return c.big }

// String returns "fn:containedBy(<small> <big>)".
func (c *ContainedBy) String() string {
	return fmt.Sprintf("fn:containedBy(%s %s)", c.small, c.big)
}

// Ordered represents Intervals#ordered(IntervalsSource...).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Ordered.
type Ordered struct {
	sources []IntervalFunction
}

// NewOrdered constructs an Ordered interval function.
func NewOrdered(sources []IntervalFunction) *Ordered {
	if sources == nil {
		panic("intervalfn: sources must not be nil")
	}
	return &Ordered{sources: sources}
}

// GetSources returns the list of source interval functions.
func (o *Ordered) GetSources() []IntervalFunction { return o.sources }

// String returns "fn:ordered(<s1> <s2> ...)".
func (o *Ordered) String() string {
	return fmt.Sprintf("fn:ordered(%s)", joinFunctions(o.sources))
}

// Wildcard represents Intervals#wildcard(BytesRef) or Intervals#wildcard(BytesRef, int).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Wildcard.
type Wildcard struct {
	wildcard      string
	maxExpansions int
}

// NewWildcard constructs a Wildcard interval function.
// maxExpansions=0 means "use the default" (no expansion limit passed).
func NewWildcard(wildcard string, maxExpansions int) *Wildcard {
	return &Wildcard{wildcard: wildcard, maxExpansions: maxExpansions}
}

// GetWildcard returns the wildcard pattern.
func (w *Wildcard) GetWildcard() string { return w.wildcard }

// GetMaxExpansions returns the maximum number of expansions (0 = default).
func (w *Wildcard) GetMaxExpansions() int { return w.maxExpansions }

// String returns "fn:wildcard(<pattern>)" or "fn:wildcard(<pattern> maxExpansions:<n>)".
func (w *Wildcard) String() string {
	suffix := ""
	if w.maxExpansions != 0 {
		suffix = fmt.Sprintf(" maxExpansions:%d", w.maxExpansions)
	}
	return fmt.Sprintf("fn:wildcard(%s%s)", w.wildcard, suffix)
}

// Unordered represents Intervals#unordered(IntervalsSource...).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Unordered.
type Unordered struct {
	sources []IntervalFunction
}

// NewUnordered constructs an Unordered interval function.
func NewUnordered(sources []IntervalFunction) *Unordered {
	if sources == nil {
		panic("intervalfn: sources must not be nil")
	}
	return &Unordered{sources: sources}
}

// GetSources returns the list of source interval functions.
func (u *Unordered) GetSources() []IntervalFunction { return u.sources }

// String returns "fn:unordered(<s1> <s2> ...)".
func (u *Unordered) String() string {
	return fmt.Sprintf("fn:unordered(%s)", joinFunctions(u.sources))
}

// After represents Intervals#after(IntervalsSource, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.After.
type After struct {
	source    IntervalFunction
	reference IntervalFunction
}

// NewAfter constructs an After interval function.
func NewAfter(source, reference IntervalFunction) *After {
	if source == nil {
		panic("intervalfn: source must not be nil")
	}
	if reference == nil {
		panic("intervalfn: reference must not be nil")
	}
	return &After{source: source, reference: reference}
}

// GetSource returns the source interval function.
func (a *After) GetSource() IntervalFunction { return a.source }

// GetReference returns the reference interval function.
func (a *After) GetReference() IntervalFunction { return a.reference }

// String returns "fn:after(<source> <reference>)".
func (a *After) String() string {
	return fmt.Sprintf("fn:after(%s %s)", a.source, a.reference)
}

// FuzzyTerm represents Intervals#fuzzyTerm with the given parameters.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.FuzzyTerm.
//
// Default values: maxEdits defaults to FuzzyQuery.defaultMaxEdits (2) when nil
// is passed; maxExpansions defaults to Intervals.DEFAULT_MAX_EXPANSIONS (128)
// when nil is passed.
const (
	// FuzzyTermDefaultMaxEdits mirrors FuzzyQuery.defaultMaxEdits.
	FuzzyTermDefaultMaxEdits = 2
	// FuzzyTermDefaultMaxExpansions mirrors Intervals.DEFAULT_MAX_EXPANSIONS.
	FuzzyTermDefaultMaxExpansions = 128
)

// FuzzyTerm holds the parameters for a fuzzy term interval function.
type FuzzyTerm struct {
	term          string
	maxEdits      int
	maxExpansions int
}

// NewFuzzyTerm constructs a FuzzyTerm interval function.
// Pass nil for maxEdits or maxExpansions to use the Lucene defaults.
func NewFuzzyTerm(term string, maxEdits, maxExpansions *int) *FuzzyTerm {
	me := FuzzyTermDefaultMaxEdits
	if maxEdits != nil {
		me = *maxEdits
	}
	mexp := FuzzyTermDefaultMaxExpansions
	if maxExpansions != nil {
		mexp = *maxExpansions
	}
	return &FuzzyTerm{term: term, maxEdits: me, maxExpansions: mexp}
}

// GetTerm returns the fuzzy term.
func (f *FuzzyTerm) GetTerm() string { return f.term }

// GetMaxEdits returns the maximum edit distance.
func (f *FuzzyTerm) GetMaxEdits() int { return f.maxEdits }

// GetMaxExpansions returns the maximum number of expansions.
func (f *FuzzyTerm) GetMaxExpansions() int { return f.maxExpansions }

// String returns "fn:fuzzyTerm(<term> <maxEdits><maxExpansions>)".
func (f *FuzzyTerm) String() string {
	return fmt.Sprintf("fn:fuzzyTerm(%s %d%d)", quoteIfNeeded(f.term), f.maxEdits, f.maxExpansions)
}

// UnorderedNoOverlaps represents
// Intervals#unorderedNoOverlaps(IntervalsSource, IntervalsSource).
//
// Mirrors
// org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.UnorderedNoOverlaps.
type UnorderedNoOverlaps struct {
	a IntervalFunction
	b IntervalFunction
}

// NewUnorderedNoOverlaps constructs an UnorderedNoOverlaps interval function.
func NewUnorderedNoOverlaps(a, b IntervalFunction) *UnorderedNoOverlaps {
	if a == nil {
		panic("intervalfn: a must not be nil")
	}
	if b == nil {
		panic("intervalfn: b must not be nil")
	}
	return &UnorderedNoOverlaps{a: a, b: b}
}

// GetA returns the first interval function.
func (u *UnorderedNoOverlaps) GetA() IntervalFunction { return u.a }

// GetB returns the second interval function.
func (u *UnorderedNoOverlaps) GetB() IntervalFunction { return u.b }

// String returns "fn:unorderedNoOverlaps(<a> <b>)".
func (u *UnorderedNoOverlaps) String() string {
	return fmt.Sprintf("fn:unorderedNoOverlaps(%s %s)", u.a, u.b)
}

// Extend represents Intervals#extend(IntervalsSource, int, int).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Extend.
type Extend struct {
	source IntervalFunction
	before int
	after  int
}

// NewExtend constructs an Extend interval function.
func NewExtend(source IntervalFunction, before, after int) *Extend {
	if source == nil {
		panic("intervalfn: source must not be nil")
	}
	return &Extend{source: source, before: before, after: after}
}

// GetSource returns the source interval function.
func (e *Extend) GetSource() IntervalFunction { return e.source }

// GetBefore returns the number of positions to extend before the source.
func (e *Extend) GetBefore() int { return e.before }

// GetAfter returns the number of positions to extend after the source.
func (e *Extend) GetAfter() int { return e.after }

// String returns "fn:extend(<source> <before> <after>)".
func (e *Extend) String() string {
	return fmt.Sprintf("fn:extend(%s %d %d)", e.source, e.before, e.after)
}

// Phrase represents Intervals#phrase(IntervalsSource...).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.Phrase.
type Phrase struct {
	sources []IntervalFunction
}

// NewPhrase constructs a Phrase interval function.
func NewPhrase(sources []IntervalFunction) *Phrase {
	if sources == nil {
		panic("intervalfn: sources must not be nil")
	}
	return &Phrase{sources: sources}
}

// GetSources returns the list of source interval functions.
func (p *Phrase) GetSources() []IntervalFunction { return p.sources }

// String returns "fn:phrase(<s1> <s2> ...)".
func (p *Phrase) String() string {
	return fmt.Sprintf("fn:phrase(%s)", joinFunctions(p.sources))
}

// NotContainedBy represents Intervals#notContainedBy(IntervalsSource, IntervalsSource).
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.NotContainedBy.
type NotContainedBy struct {
	small IntervalFunction
	big   IntervalFunction
}

// NewNotContainedBy constructs a NotContainedBy interval function.
func NewNotContainedBy(small, big IntervalFunction) *NotContainedBy {
	if small == nil {
		panic("intervalfn: small must not be nil")
	}
	if big == nil {
		panic("intervalfn: big must not be nil")
	}
	return &NotContainedBy{small: small, big: big}
}

// GetSmall returns the small (subject) interval function.
func (n *NotContainedBy) GetSmall() IntervalFunction { return n.small }

// GetBig returns the big (container) interval function.
func (n *NotContainedBy) GetBig() IntervalFunction { return n.big }

// String returns "fn:notContainedBy(<small> <big>)".
func (n *NotContainedBy) String() string {
	return fmt.Sprintf("fn:notContainedBy(%s %s)", n.small, n.big)
}

// AnalyzedText represents an analyzed text term for use in interval queries.
// It uses Intervals#analyzedText with gaps=0 and ordered=true.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.intervalfn.AnalyzedText.
type AnalyzedText struct {
	term string
}

// NewAnalyzedText constructs an AnalyzedText interval function.
func NewAnalyzedText(term string) *AnalyzedText {
	return &AnalyzedText{term: term}
}

// GetTerm returns the term string.
func (a *AnalyzedText) GetTerm() string { return a.term }

// RequiresQuotes reports whether the term string contains whitespace and
// must be quoted in the string representation. Mirrors the static
// AnalyzedText.requiresQuotes(String) method.
func RequiresQuotes(term string) bool { return requiresQuotes(term) }

// String returns the term, quoted when it contains whitespace.
func (a *AnalyzedText) String() string {
	return quoteIfNeeded(a.term)
}
