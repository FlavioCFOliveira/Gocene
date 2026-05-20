// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/PayloadAttributeImpl.java

package analysis

import (
	"bytes"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PayloadAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.PayloadAttributeImpl.
//
// It is the exported concrete implementation of [PayloadAttribute].
// The payload is stored as a []byte slice (Gocene uses []byte where
// Lucene uses BytesRef). CopyTo and CloneAttribute deep-copy the
// slice so callers do not share backing arrays.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/PayloadAttributeImpl.java
type PayloadAttributeImpl struct {
	payload []byte
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*PayloadAttributeImpl)(nil)
	_ PayloadAttribute                = (*PayloadAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*PayloadAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (p *PayloadAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{PayloadAttributeType}
}

// NewPayloadAttributeImpl initialises this attribute with no payload,
// matching the Lucene no-arg constructor.
func NewPayloadAttributeImpl() *PayloadAttributeImpl {
	return &PayloadAttributeImpl{}
}

// NewPayloadAttributeImplWithPayload initialises this attribute with a
// deep copy of the given payload, matching the Lucene single-arg
// constructor.
func NewPayloadAttributeImplWithPayload(payload []byte) *PayloadAttributeImpl {
	p := &PayloadAttributeImpl{}
	p.SetPayload(payload)
	return p
}

// GetPayload returns the current payload slice, or nil when no payload
// has been set since the last Clear.
func (p *PayloadAttributeImpl) GetPayload() []byte { return p.payload }

// SetPayload replaces the payload. A nil argument clears the payload.
// The slice is copied to prevent the caller's backing array from being
// shared with the attribute's state.
func (p *PayloadAttributeImpl) SetPayload(payload []byte) {
	if payload == nil {
		p.payload = nil
		return
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	p.payload = cp
}

// HasPayload reports whether the attribute holds a non-empty payload.
func (p *PayloadAttributeImpl) HasPayload() bool { return len(p.payload) > 0 }

// Clear resets this attribute to the empty (no-payload) state,
// matching {@code PayloadAttributeImpl#clear()}.
func (p *PayloadAttributeImpl) Clear() { p.payload = nil }

// End implements util.AttributeImpl.End. The Lucene base calls
// clear() from end(); we replicate that here.
func (p *PayloadAttributeImpl) End() { p.Clear() }

// CloneAttribute returns a deep copy of this impl as [util.AttributeImpl].
func (p *PayloadAttributeImpl) CloneAttribute() util.AttributeImpl {
	clone := NewPayloadAttributeImpl()
	p.CopyTo(clone)
	return clone
}

// CopyTo deep-copies this impl's payload onto target, which must
// implement [PayloadAttribute]; a panic with an explanatory message is
// raised otherwise (matching Lucene's cast contract).
func (p *PayloadAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(PayloadAttribute)
	if !ok {
		panic("PayloadAttributeImpl.CopyTo: target must implement PayloadAttribute")
	}
	t.SetPayload(p.payload)
}

// ReflectWith pushes the (PayloadAttribute, "payload", value) triple
// through reflector, matching the Lucene reference exactly.
func (p *PayloadAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(PayloadAttributeType, "payload", p.payload)
}

// Equals returns true if other is a [PayloadAttributeImpl] whose
// payload is byte-wise equal. Two nil payloads compare equal.
func (p *PayloadAttributeImpl) Equals(other any) bool {
	if p == other {
		return true
	}
	o, ok := other.(*PayloadAttributeImpl)
	if !ok {
		return false
	}
	if p.payload == nil || o.payload == nil {
		return p.payload == nil && o.payload == nil
	}
	return bytes.Equal(p.payload, o.payload)
}

// HashCode mirrors {@code PayloadAttributeImpl#hashCode()}: 0 when the
// payload is nil, otherwise the Java Arrays.hashCode equivalent.
func (p *PayloadAttributeImpl) HashCode() int {
	if p.payload == nil {
		return 0
	}
	code := 1
	for _, b := range p.payload {
		code = code*31 + int(int8(b))
	}
	return code
}
