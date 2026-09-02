package uhighlight

// BreakIterator is an interface for dividing text into passages.
type BreakIterator interface {
	Next() (int, bool)
	Preceding(offset int) int
	Reset()
}

// SplittingBreakIterator is a wrapper that splits passages by a separator.
type SplittingBreakIterator struct {
	inner    BreakIterator
	separator rune
}

func NewSplittingBreakIterator(inner BreakIterator, separator rune) *SplittingBreakIterator {
	return &SplittingBreakIterator{
		inner:    inner,
		separator: separator,
	}
}

func (bi *SplittingBreakIterator) Next() (int, bool) {
	for {
		next, ok := bi.inner.Next()
		if !ok {
			return 0, false
		}
		// In real Lucene this checks for the separator
		return next, true
	}
}

func (bi *SplittingBreakIterator) Preceding(offset int) int {
	return bi.inner.Preceding(offset)
}

func (bi *SplittingBreakIterator) Reset() {
	bi.inner.Reset()
}

// LengthGoalBreakIterator implements a break iterator with a length goal.
type LengthGoalBreakIterator struct {
	inner BreakIterator
	goal  int
}

func (bi *LengthGoalBreakIterator) Next() (int, bool) {
	return bi.inner.Next()
}

func (bi *LengthGoalBreakIterator) Preceding(offset int) int {
	return bi.inner.Preceding(offset)
}

func (bi *LengthGoalBreakIterator) Reset() {
	bi.inner.Reset()
}

// CustomSeparatorBreakIterator implements a break iterator with a custom separator.
type CustomSeparatorBreakIterator struct {
	inner BreakIterator
}

func (bi *CustomSeparatorBreakIterator) Next() (int, bool) {
	return bi.inner.Next()
}

func (bi *CustomSeparatorBreakIterator) Preceding(offset int) int {
	return bi.inner.Preceding(offset)
}

func (bi *CustomSeparatorBreakIterator) Reset() {
	bi.inner.Reset()
}

// WholeBreakIterator implements a break iterator that treats the whole text as one passage.
type WholeBreakIterator struct {
	contentLen int
}

func (bi *WholeBreakIterator) Next() (int, bool) {
	return bi.contentLen, false
}

func (bi *WholeBreakIterator) Preceding(offset int) int {
	return 0
}

func (bi *WholeBreakIterator) Reset() {}
