package matchhighlight

import "fmt"

// FieldValueHighlighters bundles the bundled per-field-value highlighter
// strategies. Mirrors org.apache.lucene.search.matchhighlight.FieldValueHighlighters.

// SkipFieldValueHighlighter returns field values unchanged.
type SkipFieldValueHighlighter struct{}

// Highlight returns value unchanged.
func (SkipFieldValueHighlighter) Highlight(value string, _ []OffsetRange) string { return value }

// HighlightAllFieldValueHighlighter wraps every regions slice in <em>.
type HighlightAllFieldValueHighlighter struct {
	Pre  string
	Post string
}

// Highlight inserts Pre/Post around every region in value.
func (h HighlightAllFieldValueHighlighter) Highlight(value string, regions []OffsetRange) string {
	if len(regions) == 0 {
		return value
	}
	out := ""
	last := 0
	for _, r := range regions {
		from := r.From
		to := r.To
		if from < last {
			from = last
		}
		if from > len(value) {
			from = len(value)
		}
		if to > len(value) {
			to = len(value)
		}
		if to <= from {
			continue
		}
		out += value[last:from] + h.Pre + value[from:to] + h.Post
		last = to
	}
	out += value[last:]
	return out
}

// FieldValueHighlighter is the contract supported by the helpers above.
type FieldValueHighlighter interface {
	Highlight(value string, regions []OffsetRange) string
}

// Sprintf-style debug rendering helper used by some tests.
func describeRegions(rs []OffsetRange) string {
	out := ""
	for i, r := range rs {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("[%d,%d)", r.From, r.To)
	}
	return out
}

var (
	_ FieldValueHighlighter = SkipFieldValueHighlighter{}
	_ FieldValueHighlighter = HighlightAllFieldValueHighlighter{}
)
