package tokenattributes

import (
	"fmt"
	"reflect"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/util"
)

var CharTermAttributeType = reflect.TypeOf((*CharTermAttribute)(nil)).Elem()

// CharTermAttribute represents the term text of a Token.
type CharTermAttribute interface {
	util.AttributeImpl
	TermToBytesRefAttribute

	// CopyBuffer copies the contents of buffer, starting at offset for length characters,
	// into the termBuffer array.
	CopyBuffer(buffer []rune, offset, length int)

	// Buffer returns the internal termBuffer rune array.
	Buffer() []rune

	// ResizeBuffer grows the termBuffer to at least size newSize, preserving the existing content.
	ResizeBuffer(int) []rune

	// SetLength sets the number of valid characters in the termBuffer array.
	SetLength(int) CharTermAttribute

	// SetEmpty sets the length of the termBuffer to zero.
	SetEmpty() CharTermAttribute

	// Length returns the number of valid characters in the term.
	Length() int

	// CharAt returns the character at the specified index.
	CharAt(int) rune

	// SubSequence returns a subsequence of the term.
	SubSequence(start, end int) string

	// AppendString appends the specified string to this character sequence.
	AppendString(string) CharTermAttribute

	// AppendRunes appends the specified runes to this character sequence.
	AppendRunes([]rune, int, int) CharTermAttribute

	// AppendRune appends the specified rune to this character sequence.
	AppendRune(rune) CharTermAttribute

	// AppendTerm appends the contents of the other CharTermAttribute to this character sequence.
	AppendTerm(CharTermAttribute) CharTermAttribute
}

type charTermAttribute struct {
	util.BaseAttributeImpl
	termBuffer []rune
	termLength int
	builder    *util.BytesRefBuilder
}

const minBufferSize = 10

// NewCharTermAttribute creates a new CharTermAttribute with an empty term.
func NewCharTermAttribute() CharTermAttribute {
	return &charTermAttribute{
		termBuffer: make([]rune, 0, minBufferSize),
		termLength: 0,
		builder:    util.NewBytesRefBuilder(),
	}
}

func (c *charTermAttribute) CopyBuffer(buffer []rune, offset, length int) {
	c.growTermBuffer(length)
	copy(c.termBuffer, buffer[offset:offset+length])
	c.termLength = length
}

func (c *charTermAttribute) Buffer() []rune {
	return c.termBuffer
}

func (c *charTermAttribute) ResizeBuffer(newSize int) []rune {
	if len(c.termBuffer) < newSize {
		newBuffer := make([]rune, newSize)
		copy(newBuffer, c.termBuffer)
		c.termBuffer = newBuffer
	}
	return c.termBuffer
}

func (c *charTermAttribute) growTermBuffer(newSize int) {
	if len(c.termBuffer) < newSize {
		c.termBuffer = make([]rune, newSize)
	}
}

func (c *charTermAttribute) SetLength(length int) CharTermAttribute {
	if length < 0 || length > len(c.termBuffer) {
		panic(fmt.Sprintf("length must be non-negative and <= buffer length; got %d", length))
	}
	c.termLength = length
	return c
}

func (c *charTermAttribute) SetEmpty() CharTermAttribute {
	c.termLength = 0
	return c
}

func (c *charTermAttribute) Length() int {
	return c.termLength
}

func (c *charTermAttribute) CharAt(index int) rune {
	if index < 0 || index >= c.termLength {
		panic(fmt.Sprintf("index must be within [0, %d); got %d", c.termLength, index))
	}
	return c.termBuffer[index]
}

func (c *charTermAttribute) SubSequence(start, end int) string {
	if start < 0 || end > c.termLength || start > end {
		panic(fmt.Sprintf("bounds [%d, %d) must be within [0, %d)", start, end, c.termLength))
	}
	return string(c.termBuffer[start:end])
}

func (c *charTermAttribute) AppendString(s string) CharTermAttribute {
	if s == "" {
		return c
	}
	runes := []rune(s)
	return c.AppendRunes(runes, 0, len(runes))
}

func (c *charTermAttribute) AppendRunes(runes []rune, start, end int) CharTermAttribute {
	if runes == nil {
		return c.appendNull()
	}
	if start < 0 || end > len(runes) || start > end {
		panic(fmt.Sprintf("bounds [%d, %d) must be within [0, %d)", start, end, len(runes)))
	}
	lenToAppend := end - start
	if lenToAppend == 0 {
		return c
	}
	c.ResizeBuffer(c.termLength + lenToAppend)
	copy(c.termBuffer[c.termLength:], runes[start:end])
	c.termLength += lenToAppend
	return c
}

func (c *charTermAttribute) AppendRune(r rune) CharTermAttribute {
	c.ResizeBuffer(c.termLength + 1)
	c.termBuffer[c.termLength] = r
	c.termLength++
	return c
}

func (c *charTermAttribute) AppendTerm(ta CharTermAttribute) CharTermAttribute {
	if ta == nil {
		return c.appendNull()
	}
	termLen := ta.Length()
	if termLen == 0 {
		return c
	}
	c.ResizeBuffer(c.termLength + termLen)
	copy(c.termBuffer[c.termLength:], ta.Buffer()[:termLen])
	c.termLength += termLen
	return c
}

func (c *charTermAttribute) appendNull() CharTermAttribute {
	null := []rune("null")
	return c.AppendRunes(null, 0, len(null))
}

func (c *charTermAttribute) GetBytesRef() *util.BytesRef {
	buf := make([]byte, 0, c.termLength*utf8.UTFMax)
	for i := 0; i < c.termLength; i++ {
		r := c.termBuffer[i]
		var b [utf8.UTFMax]byte
		n := utf8.EncodeRune(b[:], r)
		buf = append(buf, b[:n]...)
	}
	return util.NewBytesRef(buf)
}

func (c *charTermAttribute) Clear() {
	c.termLength = 0
}

func (c *charTermAttribute) CloneAttribute() util.AttributeImpl {
	clone := &charTermAttribute{
		termBuffer: make([]rune, len(c.termBuffer)),
		termLength: c.termLength,
		builder:    util.NewBytesRefBuilder(),
	}
	copy(clone.termBuffer, c.termBuffer)
	return clone
}

func (c *charTermAttribute) CopyTo(target util.AttributeImpl) {
	t, ok := target.(CharTermAttribute)
	if !ok {
		panic("target must implement CharTermAttribute")
	}
	t.CopyBuffer(c.termBuffer, 0, c.termLength)
}

func (c *charTermAttribute) ReflectWith(reflector util.AttributeReflector) {
	reflector(CharTermAttributeType, "term", string(c.termBuffer[:c.termLength]))
	reflector(reflect.TypeOf((*TermToBytesRefAttribute)(nil)).Elem(), "bytes", c.GetBytesRef())
}

func (c *charTermAttribute) String() string {
	return string(c.termBuffer[:c.termLength])
}
