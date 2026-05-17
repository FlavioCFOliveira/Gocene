// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"fmt"
	"net"
)

// InetAddressPoint is an indexed 128-bit IP address point, suitable for
// both IPv4 and IPv6. IPv4 addresses are stored as IPv4-mapped IPv6
// addresses (12-byte prefix of 0x00...0x00 0xFF 0xFF followed by the 4
// IPv4 bytes), matching RFC 4291.
//
// This is the Go port of Lucene 10.4.0's
// org.apache.lucene.document.InetAddressPoint.
//
// Static query factories are deferred (depend on search.PointRangeQuery /
// PointInSetQuery) — backlog #2695.
type InetAddressPoint struct {
	*Field
}

// InetAddressPointBytes is the encoded width of an InetAddressPoint
// (matches Lucene's InetAddressPoint.BYTES = 16).
const InetAddressPointBytes = 16

// InetAddressMinValue is the encoded value of "::" (all zero bytes).
var InetAddressMinValue = make([]byte, InetAddressPointBytes)

// InetAddressMaxValue is the encoded value of the all-ones IPv6 address.
var InetAddressMaxValue = func() []byte {
	b := make([]byte, InetAddressPointBytes)
	for i := range b {
		b[i] = 0xFF
	}
	return b
}()

var (
	// InetAddressPointType is the FieldType for an InetAddressPoint
	// (dimensionCount=1, numBytes=16). Mirrors Lucene's static TYPE.
	InetAddressPointType *FieldType

	// InetAddressPointTYPE is the Lucene-canonical alias.
	InetAddressPointTYPE *FieldType
)

func init() {
	InetAddressPointType = NewFieldType()
	InetAddressPointType.SetIndexed(true)
	InetAddressPointType.SetDimensions(1, InetAddressPointBytes)
	InetAddressPointType.Freeze()
	InetAddressPointTYPE = InetAddressPointType
}

// NewInetAddressPoint creates a new InetAddressPoint for the supplied
// IP address.
func NewInetAddressPoint(name string, addr net.IP) (*InetAddressPoint, error) {
	if addr == nil {
		return nil, fmt.Errorf("InetAddressPoint requires a non-nil address")
	}
	encoded := EncodeInetAddress(addr)
	field, err := NewField(name, encoded, InetAddressPointType)
	if err != nil {
		return nil, err
	}
	return &InetAddressPoint{Field: field}, nil
}

// EncodeInetAddress encodes an IPv4 or IPv6 address into the
// 16-byte representation expected by Lucene. IPv4 addresses are mapped to
// IPv4-in-IPv6 form (RFC 4291 §2.5.5.2).
func EncodeInetAddress(addr net.IP) []byte {
	out := make([]byte, InetAddressPointBytes)
	if v4 := addr.To4(); v4 != nil {
		// IPv4-mapped IPv6: ::ffff:a.b.c.d
		out[10], out[11] = 0xFF, 0xFF
		copy(out[12:], v4)
		return out
	}
	copy(out, addr.To16())
	return out
}

// DecodeInetAddress decodes a 16-byte Lucene-encoded address back to a
// net.IP. The returned IP is in IPv4 form when the encoded value is an
// IPv4-mapped IPv6 address (matching Java's InetAddress.getByAddress
// behaviour).
func DecodeInetAddress(encoded []byte) (net.IP, error) {
	if len(encoded) != InetAddressPointBytes {
		return nil, fmt.Errorf("encoded inet address must be %d bytes, got %d", InetAddressPointBytes, len(encoded))
	}
	// IPv4-mapped IPv6 prefix: 10 zeros + 0xFF 0xFF.
	if bytes.HasPrefix(encoded, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF}) {
		return net.IPv4(encoded[12], encoded[13], encoded[14], encoded[15]).To4(), nil
	}
	out := make([]byte, 16)
	copy(out, encoded)
	return out, nil
}
