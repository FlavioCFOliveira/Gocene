package matchhighlight

// BreakIteratorShrinkingAdjuster keeps highlight passages within sentence
// boundaries provided by an external break-iterator function. The supplied
// SentenceBoundary callback returns the [start, end) of the sentence
// containing pos. Mirrors
// org.apache.lucene.search.matchhighlight.BreakIteratorShrinkingAdjuster.
type BreakIteratorShrinkingAdjuster struct {
	SentenceBoundary func(text string, pos int) OffsetRange
}

// NewBreakIteratorShrinkingAdjuster builds the adjuster.
func NewBreakIteratorShrinkingAdjuster(boundary func(text string, pos int) OffsetRange) *BreakIteratorShrinkingAdjuster {
	return &BreakIteratorShrinkingAdjuster{SentenceBoundary: boundary}
}

// Adjust shrinks region so its bounds never leave the surrounding sentence.
func (a *BreakIteratorShrinkingAdjuster) Adjust(text string, region OffsetRange) OffsetRange {
	if a.SentenceBoundary == nil {
		return region
	}
	startSent := a.SentenceBoundary(text, region.From)
	endSent := a.SentenceBoundary(text, region.To-1)
	from := region.From
	if from < startSent.From {
		from = startSent.From
	}
	to := region.To
	if to > endSent.To {
		to = endSent.To
	}
	return NewOffsetRange(from, to)
}

var _ PassageAdjuster = (*BreakIteratorShrinkingAdjuster)(nil)
