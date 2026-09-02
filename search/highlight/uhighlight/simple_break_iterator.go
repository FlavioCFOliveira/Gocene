package uhighlight

import (
	"unicode"
)

// SimpleBreakIterator implements a basic sentence-level break iterator.
type SimpleBreakIterator struct {
	content string
	pos     int
}

func NewSimpleBreakIterator(content string) *SimpleBreakIterator {
	return &SimpleBreakIterator{
		content: content,
		pos:     0,
	}
}

func (bi *SimpleBreakIterator) Next() (int, bool) {
	if bi.pos >= len(bi.content) {
		return 0, false
	}
	for i := bi.pos; i < len(bi.content); i++ {
		r := rune(bi.content[i])
		if r == '.' || r == '!' || r == '?' {
			bi.pos = i + 1
			return bi.pos, true
		}
	}
	bi.pos = len(bi.content)
	return bi.pos, true
}

func (bi *SimpleBreakIterator) Preceding(offset int) int {
	if offset <= 0 {
		return 0
	}
	for i := offset - 1; i >= 0; i-- {
		r := rune(bi.content[i])
		if r == '.' || r == '!' || r == '?' {
			return i + 1
		}
	}
	return 0
}

func (bi *SimpleBreakIterator) Reset() {
	bi.pos = 0
}
