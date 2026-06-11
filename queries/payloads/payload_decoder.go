// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/PayloadDecoder.java

package payloads

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PayloadDecoder defines a way of converting payloads to float values,
// for use by PayloadScoreQuery.
//
// Mirrors org.apache.lucene.queries.payloads.PayloadDecoder.
type PayloadDecoder interface {
	// ComputePayloadFactor returns a float value for the given payload.
	ComputePayloadFactor(payload *util.BytesRef) float32
}

// PayloadDecoderFunc is a function adapter that implements PayloadDecoder.
type PayloadDecoderFunc func(payload *util.BytesRef) float32

// ComputePayloadFactor implements PayloadDecoder.
func (f PayloadDecoderFunc) ComputePayloadFactor(payload *util.BytesRef) float32 {
	return f(payload)
}

// FloatDecoder is a PayloadDecoder that interprets the first byte of a
// payload as a float. If the payload is nil or empty, returns 1.
//
// Mirrors PayloadDecoder.FLOAT_DECODER in Java (lambda: bytes == null ? 1 : bytes.bytes[bytes.offset]).
var FloatDecoder PayloadDecoder = PayloadDecoderFunc(func(payload *util.BytesRef) float32 {
	if payload == nil || payload.Length == 0 {
		return 1
	}
	return float32(payload.Bytes[payload.Offset])
})
