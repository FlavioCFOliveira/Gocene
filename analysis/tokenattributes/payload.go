package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

var PayloadAttributeType = reflect.TypeOf((*PayloadAttribute)(nil)).Elem()

// PayloadAttribute represents the payload of a Token.
type PayloadAttribute interface {
	util.AttributeImpl

	// GetPayload returns this Token's payload.
	GetPayload() *util.BytesRef

	// SetPayload sets this Token's payload.
	SetPayload(*util.BytesRef)
}

type payloadAttribute struct {
	util.BaseAttributeImpl
	payload *util.BytesRef
}

// NewPayloadAttribute initializes a new PayloadAttributeImpl with no payload.
func NewPayloadAttribute() PayloadAttribute {
	return &payloadAttribute{
		payload: nil,
	}
}

// NewPayloadAttributeWithPayload initializes a new PayloadAttributeImpl with the given payload.
func NewPayloadAttributeWithPayload(payload *util.BytesRef) PayloadAttribute {
	return &payloadAttribute{
		payload: payload,
	}
}

func (p *payloadAttribute) GetPayload() *util.BytesRef {
	return p.payload
}

func (p *payloadAttribute) SetPayload(payload *util.BytesRef) {
	p.payload = payload
}

func (p *payloadAttribute) Clear() {
	p.payload = nil
}

func (p *payloadAttribute) CloneAttribute() util.AttributeImpl {
	clone := &payloadAttribute{}
	if p.payload != nil {
		clone.payload = p.payload.Clone()
	}
	return clone
}

func (p *payloadAttribute) CopyTo(target util.AttributeImpl) {
	t, ok := target.(PayloadAttribute)
	if !ok {
		panic("target must implement PayloadAttribute")
	}
	if p.payload == nil {
		t.SetPayload(nil)
	} else {
		t.SetPayload(p.payload.Clone())
	}
}

func (p *payloadAttribute) ReflectWith(reflector util.AttributeReflector) {
	reflector(PayloadAttributeType, "payload", p.payload)
}
