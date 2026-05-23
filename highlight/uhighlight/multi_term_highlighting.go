package uhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ExtractAutomata extracts MultiTermQuery matchers from query that match the
// provided fieldMatcher predicate.  When lookInSpan is false, span-query
// sub-trees are skipped.
//
// Mirrors org.apache.lucene.search.uhighlight.MultiTermHighlighting.extractAutomata.
func ExtractAutomata(query search.Query, fieldMatcher func(string) bool, lookInSpan bool) []*LabelledCharArrayMatcher {
	collector := &automataCollector{
		lookInSpan:   lookInSpan,
		fieldMatcher: fieldMatcher,
	}
	visitQuery(query, collector)
	return collector.matchers
}

// CanExtractAutomataFromLeafQuery reports whether the given query is a type
// from which an automaton can be extracted (AutomatonQuery or FuzzyQuery).
//
// Mirrors
// org.apache.lucene.search.uhighlight.MultiTermHighlighting.canExtractAutomataFromLeafQuery.
func CanExtractAutomataFromLeafQuery(query search.Query) bool {
	switch query.(type) {
	case *search.AutomatonQuery, *search.FuzzyQuery:
		return true
	default:
		return false
	}
}

// visitQuery drives a recursive query-tree walk, routing each node through
// the supplied visitor.  It mirrors the recursive descent performed by
// Query.visit(QueryVisitor) in Java.
func visitQuery(q search.Query, v search.QueryVisitor) {
	if q == nil {
		return
	}
	// Queries that implement the Visit hook (the majority of the ported types)
	type visitable interface{ Visit(search.QueryVisitor) }
	if vq, ok := q.(visitable); ok {
		vq.Visit(v)
		return
	}
	// Fall back: treat the query as an opaque leaf.
	v.VisitLeaf(q)
}

// automataCollector is the internal QueryVisitor that accumulates
// LabelledCharArrayMatchers from MultiTermQuery nodes.
type automataCollector struct {
	search.EmptyQueryVisitorBase
	lookInSpan   bool
	fieldMatcher func(string) bool
	matchers     []*LabelledCharArrayMatcher
}

// AcceptField returns true only when the field matches the predicate.
func (c *automataCollector) AcceptField(field string) bool {
	if c.fieldMatcher == nil {
		return true
	}
	return c.fieldMatcher(field)
}

// GetSubVisitor skips span-query sub-trees when lookInSpan is false.
func (c *automataCollector) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	if !c.lookInSpan {
		if _, ok := parent.(search.SpanQuery); ok {
			return search.EmptyQueryVisitor
		}
	}
	return c
}

// ConsumeTermsMatching captures automaton-based matchers.
func (c *automataCollector) ConsumeTermsMatching(query search.Query, field string, automaton func() search.ByteRunAutomaton) {
	label := queryLabel(query)
	matcher := NewRunAutomatonMatcher(automaton())
	c.matchers = append(c.matchers, NewLabelledCharArrayMatcher(label, matcher))
}

// queryLabel returns a human-readable label for a query (its string form
// mirrors query.toString() in Java).
func queryLabel(q search.Query) string {
	type stringer interface{ String() string }
	if s, ok := q.(stringer); ok {
		return s.String()
	}
	return "unknown"
}

// RunAutomatonMatcher adapts a ByteRunAutomaton to the CharArrayMatcher
// interface by encoding each rune slice to UTF-8 bytes and running the
// automaton over them.
type RunAutomatonMatcher struct {
	automaton search.ByteRunAutomaton
}

// NewRunAutomatonMatcher builds the adapter.
func NewRunAutomatonMatcher(a search.ByteRunAutomaton) *RunAutomatonMatcher {
	return &RunAutomatonMatcher{automaton: a}
}

// Match returns true if the rune slice chars[start:start+length] matches the
// underlying automaton after UTF-8 encoding.
func (m *RunAutomatonMatcher) Match(chars []rune, start, length int) bool {
	if m.automaton == nil {
		return false
	}
	// Encode the rune slice to UTF-8 bytes.
	b := runeSliceToUTF8(chars, start, length)
	return m.automaton.Run(b)
}

// runeSliceToUTF8 converts chars[start:start+length] to a UTF-8 byte slice.
func runeSliceToUTF8(chars []rune, start, length int) []byte {
	end := start + length
	if start < 0 || end > len(chars) || length < 0 {
		return nil
	}
	// Pre-allocate assuming 3 bytes per rune on average.
	buf := make([]byte, 0, length*3)
	for _, r := range chars[start:end] {
		// Encode each rune manually to stay zero-alloc in the common ASCII path.
		switch {
		case r < 0x80:
			buf = append(buf, byte(r))
		case r < 0x800:
			buf = append(buf, byte(0xC0|(r>>6)), byte(0x80|(r&0x3F)))
		case r < 0x10000:
			buf = append(buf, byte(0xE0|(r>>12)), byte(0x80|((r>>6)&0x3F)), byte(0x80|(r&0x3F)))
		default:
			r -= 0x10000
			buf = append(buf,
				byte(0xF0|(r>>18)),
				byte(0x80|((r>>12)&0x3F)),
				byte(0x80|((r>>6)&0x3F)),
				byte(0x80|(r&0x3F)),
			)
		}
	}
	return buf
}

var _ CharArrayMatcher = (*RunAutomatonMatcher)(nil)
