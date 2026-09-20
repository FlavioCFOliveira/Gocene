package tokenattributes

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

var OffsetAttributeType = reflect.TypeOf((*OffsetAttribute)(nil)).Elem()

// OffsetAttribute represents the start and end character offset of a Token.
type OffsetAttribute interface {
	util.AttributeImpl

	// StartOffset returns this Token's starting offset.
	StartOffset() int

	// EndOffset returns this Token's ending offset.
	EndOffset() int

	// SetOffset sets the starting and ending offset.
	SetOffset(startOffset, endOffset int)
}

type offsetAttribute struct {
	util.BaseAttributeImpl
	startOffset int
	endOffset   int
}

// NewOffsetAttribute initializes a new OffsetAttribute with offsets of 0.
func NewOffsetAttribute() OffsetAttribute {
	return &offsetAttribute{
		startOffset: 0,
		endOffset:   0,
	}
}

func (o *offsetAttribute) StartOffset() int {
	return o.startOffset
}

func (o *offsetAttribute) EndOffset() int {
	return o.endOffset
}

func (o *offsetAttribute) SetOffset(startOffset, endOffset int) {
	if startOffset < 0 || endOffset < startOffset {
		panic(fmt.Sprintf("startOffset must be non-negative, and endOffset must be >= startOffset; got startOffset=%d, endOffset=%d", startOffset, endOffset))
	}
	o.startOffset = startOffset
	o.endOffset = endOffset
}

func (o *offsetAttribute) Clear() {
	o.startOffset = 0
	o.endOffset = 0
}

func (o *offsetAttribute) CloneAttribute() util.AttributeImpl {
	return &offsetAttribute{
		startOffset: o.startOffset,
		endOffset:   o.endOffset,
	}
}

func (o *offsetAttribute) CopyTo(target util.AttributeImpl) {
	t, ok := target.(OffsetAttribute)
	if !ok {
		panic("target must implement OffsetAttribute")
	}
	t.SetOffset(o.startOffset, o.endOffset)
}

func (o *offsetAttribute) ReflectWith(reflector util.AttributeReflector) {
	reflector(OffsetAttributeType, "startOffset", o.startOffset)
	reflector(OffsetAttributeType, "endOffset", o.endOffset)
}
