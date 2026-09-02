package uhighlight

// WeightedTerm is a lightweight class to hold a term and a weight value used for scoring this term.
// Mirrors org.apache.lucene.search.highlight.WeightedTerm.
type WeightedTerm struct {
	Weight float32 // multiplier
	Term   string  // stemmed form
}

func NewWeightedTerm(weight float32, term string) *WeightedTerm {
	return &WeightedTerm{
		Weight: weight,
		Term:   term,
	}
}

// PositionSpan is a utility class to record Position Spans.
// Mirrors org.apache.lucene.search.highlight.PositionSpan.
type PositionSpan struct {
	Start int
	End   int
}

func NewPositionSpan(start, end int) *PositionSpan {
	return &PositionSpan{
		Start: start,
		End:   end,
	}
}

// WeightedSpanTerm is a lightweight class to hold term, weight, and positions used for scoring this term.
// Mirrors org.apache.lucene.search.highlight.WeightedSpanTerm.
type WeightedSpanTerm struct {
	WeightedTerm
	PositionSensitive bool
	PositionSpans     []*PositionSpan
}

func NewWeightedSpanTerm(weight float32, term string) *WeightedSpanTerm {
	return &WeightedSpanTerm{
		WeightedTerm:  WeightedTerm{Weight: weight, Term: term},
		PositionSpans: make([]*PositionSpan, 0),
	}
}

func NewWeightedSpanTermWithSensitivity(weight float32, term string, positionSensitive bool) *WeightedSpanTerm {
	return &WeightedSpanTerm{
		WeightedTerm:      WeightedTerm{Weight: weight, Term: term},
		PositionSensitive: positionSensitive,
		PositionSpans:     make([]*PositionSpan, 0),
	}
}

// CheckPosition checks to see if this term is valid at position.
func (ws *WeightedSpanTerm) CheckPosition(position int) bool {
	for _, posSpan := range ws.PositionSpans {
		if position >= posSpan.Start && position <= posSpan.End {
			return true
		}
	}
	return false
}

// AddPositionSpans adds the given position spans to this term.
func (ws *WeightedSpanTerm) AddPositionSpans(spans []*PositionSpan) {
	ws.PositionSpans = append(ws.PositionSpans, spans...)
}
