package matchhighlight

// MatchHighlighter coordinates the matches API to render passages for a
// single field. Mirrors
// org.apache.lucene.search.matchhighlight.MatchHighlighter.
type MatchHighlighter struct {
	Selector  *PassageSelector
	Adjuster  PassageAdjuster
	Renderer  FieldValueHighlighter
}

// NewMatchHighlighter builds a highlighter with a selector, adjuster and
// renderer. Any of these may be nil — sensible defaults are substituted.
func NewMatchHighlighter(selector *PassageSelector, adjuster PassageAdjuster, renderer FieldValueHighlighter) *MatchHighlighter {
	if selector == nil {
		selector = NewPassageSelector(3, 200)
	}
	if adjuster == nil {
		adjuster = IdentityPassageAdjuster{}
	}
	if renderer == nil {
		renderer = HighlightAllFieldValueHighlighter{Pre: "<em>", Post: "</em>"}
	}
	return &MatchHighlighter{Selector: selector, Adjuster: adjuster, Renderer: renderer}
}

// Highlight selects passages from regions, adjusts them, and renders the
// supplied field value.
func (m *MatchHighlighter) Highlight(value string, regions []OffsetRange) string {
	selected := m.Selector.Select(regions)
	adjusted := make([]OffsetRange, len(selected))
	for i, r := range selected {
		adjusted[i] = m.Adjuster.Adjust(value, r)
	}
	return m.Renderer.Highlight(value, adjusted)
}
