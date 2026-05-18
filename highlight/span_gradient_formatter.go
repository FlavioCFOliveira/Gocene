package highlight

import "fmt"

// SpanGradientFormatter wraps matched tokens in a <span> with a color
// derived from the token's score. Mirrors
// org.apache.lucene.search.highlight.SpanGradientFormatter.
type SpanGradientFormatter struct {
	MaxScore float32
	MinFg    string // "rgb(...)" for the weakest score
	MaxFg    string // "rgb(...)" for the strongest score
	MinBg    string
	MaxBg    string
}

// NewSpanGradientFormatter builds a formatter.
func NewSpanGradientFormatter(maxScore float32, minFg, maxFg, minBg, maxBg string) *SpanGradientFormatter {
	if maxScore <= 0 {
		maxScore = 1.0
	}
	return &SpanGradientFormatter{MaxScore: maxScore, MinFg: minFg, MaxFg: maxFg, MinBg: minBg, MaxBg: maxBg}
}

// HighlightTerm renders originalText wrapped in a <span> tag whose style
// reflects score.
func (f *SpanGradientFormatter) HighlightTerm(originalText string, score float32) string {
	if score <= 0 {
		return originalText
	}
	if score > f.MaxScore {
		score = f.MaxScore
	}
	return fmt.Sprintf("<span style=\"color: %s; background: %s\">%s</span>", f.MaxFg, f.MaxBg, originalText)
}
