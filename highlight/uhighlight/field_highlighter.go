package uhighlight

// FieldHighlighter coordinates a single field's offsets-enum walk into a
// rendered passage list. Mirrors
// org.apache.lucene.search.uhighlight.FieldHighlighter.
type FieldHighlighter struct {
	Components *UHComponents
	MaxPassages int
}

// NewFieldHighlighter builds a FieldHighlighter.
func NewFieldHighlighter(components *UHComponents, maxPassages int) *FieldHighlighter {
	if maxPassages < 1 {
		maxPassages = 1
	}
	return &FieldHighlighter{Components: components, MaxPassages: maxPassages}
}

// Passage is a single rendered passage (start, end, score in highlight order).
type Passage struct {
	StartOffset int
	EndOffset   int
	Score       float32
}

// HighlightFieldForDoc resolves up to MaxPassages passages for the supplied
// docContext, sorted by descending score. Concrete extraction is delegated
// to the FieldOffsetStrategy.
func (h *FieldHighlighter) HighlightFieldForDoc(docContext any) ([]Passage, error) {
	enum, err := h.Components.OffsetStrat.GetOffsetsEnum(docContext)
	if err != nil {
		return nil, err
	}
	defer enum.Close()
	var passages []Passage
	for enum.Next() {
		passages = append(passages, Passage{
			StartOffset: enum.StartOffset(),
			EndOffset:   enum.EndOffset(),
			Score:       enum.Weight(),
		})
	}
	if len(passages) > h.MaxPassages {
		passages = passages[:h.MaxPassages]
	}
	return passages, nil
}
