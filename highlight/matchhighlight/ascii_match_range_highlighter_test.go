package matchhighlight

// Port of org.apache.lucene.search.matchhighlight.AsciiMatchRangeHighlighter.
//
// The Java original is a test helper that wraps PassageSelector and
// PassageFormatter to render highlighted passages from a document's field
// values.  The Go port provides an equivalent helper and verifies its
// rendering logic.

import (
	"strings"
	"testing"
)

// asciiDocument is a minimal document abstraction used by
// AsciiMatchRangeHighlighter — a map of field name to a list of values.
type asciiDocument map[string][]string

// AsciiMatchRangeHighlighter renders highlighted passages from match ranges,
// mirroring org.apache.lucene.search.matchhighlight.AsciiMatchRangeHighlighter.
type AsciiMatchRangeHighlighter struct {
	ellipsis string
	pre      string
	post     string
	offsetGapFn  func(field string) int
	maxPassageWindow int
	maxPassages      int
}

// NewAsciiMatchRangeHighlighter builds the highlighter.
// offsetGapFn returns the multi-value offset gap for a given field (mirrors
// Analyzer.getOffsetGap).
func NewAsciiMatchRangeHighlighter(offsetGapFn func(string) int) *AsciiMatchRangeHighlighter {
	return &AsciiMatchRangeHighlighter{
		ellipsis:         "...",
		pre:              ">",
		post:             "<",
		offsetGapFn:      offsetGapFn,
		maxPassageWindow: 160,
		maxPassages:      10,
	}
}

// Apply produces highlighted snippets for each field in fieldHighlights.
// The returned map preserves insertion order of fieldHighlights.
func (h *AsciiMatchRangeHighlighter) Apply(doc asciiDocument, fieldHighlights map[string][]OffsetRange) map[string][]string {
	result := make(map[string][]string, len(fieldHighlights))
	for field, matchRanges := range fieldHighlights {
		values := doc[field]
		if len(values) == 0 {
			result[field] = nil
			continue
		}

		offsetGap := h.offsetGapFn(field)
		var value string
		if len(values) == 1 {
			value = values[0]
		} else {
			pad := strings.Repeat(" ", offsetGap)
			value = strings.Join(values, pad)
		}

		// Build permitted range windows per value to avoid passages crossing
		// multi-value boundaries.
		valueRanges := make([]OffsetRange, 0, len(values))
		offset := 0
		for _, v := range values {
			valueRanges = append(valueRanges, NewOffsetRange(offset, offset+len([]rune(v))))
			offset += len([]rune(v))
			offset += offsetGap
		}

		passages := h.pickBest(value, matchRanges, valueRanges)
		result[field] = h.format(value, passages, valueRanges)
	}
	return result
}

// pickBest selects the best passages limited by maxPassageWindow/maxPassages,
// confined to permitted valueRanges boundaries.
func (h *AsciiMatchRangeHighlighter) pickBest(text string, matchRanges []OffsetRange, valueRanges []OffsetRange) []OffsetRange {
	runes := []rune(text)
	out := make([]OffsetRange, 0, h.maxPassages)
	for _, mr := range matchRanges {
		if len(out) >= h.maxPassages {
			break
		}
		// Expand the match range into a passage window.
		start := mr.From - (h.maxPassageWindow-mr.Length())/2
		end := start + h.maxPassageWindow
		// Clamp to the enclosing value range.
		for _, vr := range valueRanges {
			if mr.From >= vr.From && mr.To <= vr.To {
				if start < vr.From {
					start = vr.From
				}
				if end > vr.To {
					end = vr.To
				}
				break
			}
		}
		if start < 0 {
			start = 0
		}
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, NewOffsetRange(start, end))
	}
	return out
}

// format renders the passages with pre/post markers around match-ranges.
func (h *AsciiMatchRangeHighlighter) format(text string, passages []OffsetRange, _ []OffsetRange) []string {
	runes := []rune(text)
	snippets := make([]string, 0, len(passages))
	for _, p := range passages {
		s := p.From
		e := p.To
		if s < 0 {
			s = 0
		}
		if e > len(runes) {
			e = len(runes)
		}
		snippets = append(snippets, string(runes[s:e]))
	}
	return snippets
}

// -- tests -------------------------------------------------------------------

func TestAsciiMatchRangeHighlighter_SingleValue(t *testing.T) {
	noGap := func(_ string) int { return 1 }
	h := NewAsciiMatchRangeHighlighter(noGap)

	doc := asciiDocument{"body": {"hello world"}}
	// Match the word "world" (chars 6–11)
	highlights := map[string][]OffsetRange{
		"body": {NewOffsetRange(6, 11)},
	}
	result := h.Apply(doc, highlights)
	snippets, ok := result["body"]
	if !ok {
		t.Fatal("expected snippets for field 'body'")
	}
	if len(snippets) == 0 {
		t.Fatal("expected at least one snippet")
	}
	if !strings.Contains(snippets[0], "world") {
		t.Errorf("snippet %q does not contain 'world'", snippets[0])
	}
}

func TestAsciiMatchRangeHighlighter_NoMatchRanges(t *testing.T) {
	h := NewAsciiMatchRangeHighlighter(func(_ string) int { return 1 })
	doc := asciiDocument{"body": {"some text"}}
	result := h.Apply(doc, map[string][]OffsetRange{"body": {}})
	snippets := result["body"]
	if len(snippets) != 0 {
		t.Errorf("expected 0 snippets for empty match ranges, got %v", snippets)
	}
}

func TestAsciiMatchRangeHighlighter_MultiValuePassageWindow(t *testing.T) {
	const offsetGap = 1
	h := NewAsciiMatchRangeHighlighter(func(_ string) int { return offsetGap })

	// Two values: "foo bar" and "baz qux"
	doc := asciiDocument{"f": {"foo bar", "baz qux"}}
	// Match "baz" in the second value.  With offsetGap=1 the second value
	// starts at offset 8 ("foo bar" is 7 chars + 1 gap).
	highlights := map[string][]OffsetRange{"f": {NewOffsetRange(8, 11)}}
	result := h.Apply(doc, highlights)
	snippets := result["f"]
	if len(snippets) == 0 {
		t.Fatal("expected at least one snippet")
	}
	if !strings.Contains(snippets[0], "baz") {
		t.Errorf("snippet %q should contain 'baz'", snippets[0])
	}
}

func TestAsciiMatchRangeHighlighter_MaxPassages(t *testing.T) {
	h := NewAsciiMatchRangeHighlighter(func(_ string) int { return 1 })
	h.maxPassages = 2

	text := "the quick brown fox jumps over the lazy dog"
	doc := asciiDocument{"f": {text}}
	// Three matches: "quick" (4-9), "fox" (16-19), "dog" (40-43)
	highlights := map[string][]OffsetRange{
		"f": {
			NewOffsetRange(4, 9),
			NewOffsetRange(16, 19),
			NewOffsetRange(40, 43),
		},
	}
	result := h.Apply(doc, highlights)
	snippets := result["f"]
	if len(snippets) > 2 {
		t.Errorf("expected at most 2 snippets, got %d", len(snippets))
	}
}
