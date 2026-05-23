package matchhighlight

// CharSequenceIterator iterates over the runes of a string, mimicking the
// contract of java.text.CharacterIterator as used within the match-highlight
// pipeline.  Only the subset of methods actually consumed by the Gocene
// highlighter is exposed; the full CharacterIterator interface is not
// represented in Go.
//
// Mirrors org.apache.lucene.search.matchhighlight.CharSequenceIterator.
type CharSequenceIterator struct {
	text  []rune
	begin int
	end   int
	pos   int
}

// Done is the sentinel rune returned when the iterator is exhausted
// (mirrors CharacterIterator.DONE = '￿').
const Done rune = '￿'

// NewCharSequenceIterator creates a new CharSequenceIterator over text.
func NewCharSequenceIterator(text string) *CharSequenceIterator {
	r := []rune(text)
	return &CharSequenceIterator{
		text:  r,
		begin: 0,
		end:   len(r),
		pos:   0,
	}
}

// First sets the position to the beginning and returns the first character.
func (c *CharSequenceIterator) First() rune {
	c.pos = c.begin
	return c.Current()
}

// Last sets the position to the last character and returns it.
func (c *CharSequenceIterator) Last() rune {
	if c.end != c.begin {
		c.pos = c.end - 1
	} else {
		c.pos = c.end
	}
	return c.Current()
}

// SetIndex sets the iterator position to p and returns the character at that
// index. Returns Done if p equals the end index.
func (c *CharSequenceIterator) SetIndex(p int) rune {
	if p < c.begin || p > c.end {
		panic("matchhighlight: CharSequenceIterator: invalid index")
	}
	c.pos = p
	return c.Current()
}

// Current returns the character at the current position, or Done if the
// position is outside the valid range.
func (c *CharSequenceIterator) Current() rune {
	if c.pos >= c.begin && c.pos < c.end {
		return c.text[c.pos]
	}
	return Done
}

// Next advances the position by one and returns the character at the new
// position, or Done if the end has been reached.
func (c *CharSequenceIterator) Next() rune {
	if c.pos < c.end-1 {
		c.pos++
		return c.text[c.pos]
	}
	c.pos = c.end
	return Done
}

// Previous moves the position back by one and returns the character at the
// new position, or Done if already at the beginning.
func (c *CharSequenceIterator) Previous() rune {
	if c.pos > c.begin {
		c.pos--
		return c.text[c.pos]
	}
	return Done
}

// BeginIndex returns the start index of the text.
func (c *CharSequenceIterator) BeginIndex() int { return c.begin }

// EndIndex returns the end index (exclusive) of the text.
func (c *CharSequenceIterator) EndIndex() int { return c.end }

// Index returns the current position.
func (c *CharSequenceIterator) Index() int { return c.pos }

// Clone returns a shallow copy of this iterator.
func (c *CharSequenceIterator) Clone() *CharSequenceIterator {
	cp := *c
	return &cp
}
