// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/PayloadMatcherFactory.java

package payloads

import (
	"encoding/binary"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PayloadType specifies the decoding of the BytesRef for the payload.
//
// Mirrors org.apache.lucene.queries.payloads.SpanPayloadCheckQuery.PayloadType.
type PayloadType int

const (
	// PayloadTypeINT is for a 4-byte payload that is a packed integer (big-endian).
	PayloadTypeINT PayloadType = iota
	// PayloadTypeFLOAT is a 4-byte payload decoded to a float32 (big-endian).
	PayloadTypeFLOAT
	// PayloadTypeSTRING is a UTF-8 encoded string decoded from the byte array.
	PayloadTypeSTRING
)

// MatchOperation specifies the inequality operation for payload matching.
//
// Mirrors org.apache.lucene.queries.payloads.SpanPayloadCheckQuery.MatchOperation.
type MatchOperation int

const (
	// MatchOperationEQ checks for binary equality of the byte array (default).
	MatchOperationEQ MatchOperation = iota
	// MatchOperationLT matches if the payload value is less than the reference.
	MatchOperationLT
	// MatchOperationLTE matches if the payload value is less than or equal to the reference.
	MatchOperationLTE
	// MatchOperationGT matches if the payload value is greater than the reference.
	MatchOperationGT
	// MatchOperationGTE matches if the payload value is greater than or equal to the reference.
	MatchOperationGTE
)

// CreateMatcherForOpAndType returns a PayloadMatcher for the given PayloadType
// and MatchOperation. For EQ (or nil operation) it returns an equality matcher.
// For inequality operations, it returns the appropriate typed matcher.
//
// Mirrors PayloadMatcherFactory.createMatcherForOpAndType.
func CreateMatcherForOpAndType(payloadType PayloadType, op MatchOperation) PayloadMatcher {
	if op == MatchOperationEQ {
		return eqPayloadMatcherSingleton
	}
	switch payloadType {
	case PayloadTypeINT:
		return intMatcherForOp(op)
	case PayloadTypeFLOAT:
		return floatMatcherForOp(op)
	case PayloadTypeSTRING:
		return stringMatcherForOp(op)
	default:
		return eqPayloadMatcherSingleton
	}
}

func intMatcherForOp(op MatchOperation) PayloadMatcher {
	switch op {
	case MatchOperationLT:
		return ltIntPayloadMatcherSingleton
	case MatchOperationLTE:
		return lteIntPayloadMatcherSingleton
	case MatchOperationGT:
		return gtIntPayloadMatcherSingleton
	case MatchOperationGTE:
		return gteIntPayloadMatcherSingleton
	default:
		return eqPayloadMatcherSingleton
	}
}

func floatMatcherForOp(op MatchOperation) PayloadMatcher {
	switch op {
	case MatchOperationLT:
		return ltFloatPayloadMatcherSingleton
	case MatchOperationLTE:
		return lteFloatPayloadMatcherSingleton
	case MatchOperationGT:
		return gtFloatPayloadMatcherSingleton
	case MatchOperationGTE:
		return gteFloatPayloadMatcherSingleton
	default:
		return eqPayloadMatcherSingleton
	}
}

func stringMatcherForOp(op MatchOperation) PayloadMatcher {
	switch op {
	case MatchOperationLT:
		return ltStringPayloadMatcherSingleton
	case MatchOperationLTE:
		return lteStringPayloadMatcherSingleton
	case MatchOperationGT:
		return gtStringPayloadMatcherSingleton
	case MatchOperationGTE:
		return gteStringPayloadMatcherSingleton
	default:
		return eqPayloadMatcherSingleton
	}
}

// --- Equality matcher (works for all payload types) ---

type eqPayloadMatcher struct{}

func (eqPayloadMatcher) ComparePayload(source, payload *util.BytesRef) bool {
	return util.BytesRefEquals(source, payload)
}

var eqPayloadMatcherSingleton = &eqPayloadMatcher{}

// --- String matchers ---

type stringPayloadMatcherFunc func(val, threshold string) bool

type stringPayloadMatcher struct {
	fn stringPayloadMatcherFunc
}

func (m *stringPayloadMatcher) ComparePayload(source, payload *util.BytesRef) bool {
	return m.fn(
		string(payload.ValidBytes()),
		string(source.ValidBytes()),
	)
}

var (
	ltStringPayloadMatcherSingleton  = &stringPayloadMatcher{fn: func(val, thresh string) bool { return val < thresh }}
	lteStringPayloadMatcherSingleton = &stringPayloadMatcher{fn: func(val, thresh string) bool { return val <= thresh }}
	gtStringPayloadMatcherSingleton  = &stringPayloadMatcher{fn: func(val, thresh string) bool { return val > thresh }}
	gteStringPayloadMatcherSingleton = &stringPayloadMatcher{fn: func(val, thresh string) bool { return val >= thresh }}
)

// --- Int matchers ---

type intPayloadMatcherFunc func(val, threshold int) bool

type intPayloadMatcher struct {
	fn intPayloadMatcherFunc
}

func (m *intPayloadMatcher) ComparePayload(source, payload *util.BytesRef) bool {
	// Decode big-endian int32 from 4 bytes at offset.
	payloadVal := int32(binary.BigEndian.Uint32(payload.Bytes[payload.Offset : payload.Offset+4]))
	sourceVal := int32(binary.BigEndian.Uint32(source.Bytes[source.Offset : source.Offset+4]))
	return m.fn(int(payloadVal), int(sourceVal))
}

var (
	ltIntPayloadMatcherSingleton  = &intPayloadMatcher{fn: func(val, thresh int) bool { return val < thresh }}
	lteIntPayloadMatcherSingleton = &intPayloadMatcher{fn: func(val, thresh int) bool { return val <= thresh }}
	gtIntPayloadMatcherSingleton  = &intPayloadMatcher{fn: func(val, thresh int) bool { return val > thresh }}
	gteIntPayloadMatcherSingleton = &intPayloadMatcher{fn: func(val, thresh int) bool { return val >= thresh }}
)

// --- Float matchers ---

type floatPayloadMatcherFunc func(val, threshold float32) bool

type floatPayloadMatcher struct {
	fn floatPayloadMatcherFunc
}

func (m *floatPayloadMatcher) ComparePayload(source, payload *util.BytesRef) bool {
	payloadVal := math.Float32frombits(binary.BigEndian.Uint32(payload.Bytes[payload.Offset : payload.Offset+4]))
	sourceVal := math.Float32frombits(binary.BigEndian.Uint32(source.Bytes[source.Offset : source.Offset+4]))
	return m.fn(payloadVal, sourceVal)
}

var (
	ltFloatPayloadMatcherSingleton  = &floatPayloadMatcher{fn: func(val, thresh float32) bool { return val < thresh }}
	lteFloatPayloadMatcherSingleton = &floatPayloadMatcher{fn: func(val, thresh float32) bool { return val <= thresh }}
	gtFloatPayloadMatcherSingleton  = &floatPayloadMatcher{fn: func(val, thresh float32) bool { return val > thresh }}
	gteFloatPayloadMatcherSingleton = &floatPayloadMatcher{fn: func(val, thresh float32) bool { return val >= thresh }}
)
