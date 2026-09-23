package tokenattributes

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

var PositionIncrementAttributeType = reflect.TypeOf((*PositionIncrementAttribute)(nil)).Elem()

// PositionIncrementAttribute determines the position of this token relative to the previous
// Token in a TokenStream, used in phrase searching.
type PositionIncrementAttribute interface {
	util.AttributeImpl

	// GetPositionIncrement returns the position increment of this Token.
	GetPositionIncrement() int

	// SetPositionIncrement sets the position increment.
	SetPositionIncrement(int)
}

type positionIncrementAttribute struct {
	util.BaseAttributeImpl
	positionIncrement int
}

// NewPositionIncrementAttribute initializes a new PositionIncrementAttribute with default increment of 1.
func NewPositionIncrementAttribute() PositionIncrementAttribute {
	return &positionIncrementAttribute{
		positionIncrement: 1,
	}
}

func (p *positionIncrementAttribute) GetPositionIncrement() int {
	return p.positionIncrement
}

func (p *positionIncrementAttribute) SetPositionIncrement(positionIncrement int) {
	if positionIncrement < 0 {
		panic(fmt.Sprintf("Position increment must be zero or greater; got %d", positionIncrement))
	}
	p.positionIncrement = positionIncrement
}

func (p *positionIncrementAttribute) Clear() {
	p.positionIncrement = 1
}

func (p *positionIncrementAttribute) End() {
	p.positionIncrement = 0
}

// Equals reports whether other is a PositionIncrementAttributeImpl with the
// same position increment (PositionIncrementAttributeImpl.equals(Object)).
func (p *positionIncrementAttribute) Equals(other any) bool {
	if o, ok := other.(*positionIncrementAttribute); ok {
		if o == p {
			return true
		}
		return p.positionIncrement == o.positionIncrement
	}
	return false
}

// HashCode returns the position increment
// (PositionIncrementAttributeImpl.hashCode()).
func (p *positionIncrementAttribute) HashCode() int {
	return p.positionIncrement
}

func (p *positionIncrementAttribute) CloneAttribute() util.AttributeImpl {
	return &positionIncrementAttribute{
		positionIncrement: p.positionIncrement,
	}
}

func (p *positionIncrementAttribute) CopyTo(target util.AttributeImpl) {
	t, ok := target.(PositionIncrementAttribute)
	if !ok {
		panic("target must implement PositionIncrementAttribute")
	}
	t.SetPositionIncrement(p.positionIncrement)
}

func (p *positionIncrementAttribute) ReflectWith(reflector util.AttributeReflector) {
	reflector(PositionIncrementAttributeType, "positionIncrement", p.positionIncrement)
}
