// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"fmt"
	"net"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// InetAddressPoint is an indexed 128-bit IP address point, suitable for
// both IPv4 and IPv6. IPv4 addresses are stored as IPv4-mapped IPv6
// addresses (12-byte prefix of 0x00...0x00 0xFF 0xFF followed by the 4
// IPv4 bytes), matching RFC 4291.
//
// This is the Go port of Lucene 10.5.0's
// org.apache.lucene.document.InetAddressPoint.
type InetAddressPoint struct {
	*Field
}

// InetAddressPointBytes is the encoded width of an InetAddressPoint
// (matches Lucene's InetAddressPoint.BYTES = 16).
const InetAddressPointBytes = 16

// ipv4Prefix is the RFC 4291 prefix for IPv4-mapped IPv6 addresses.
var ipv4Prefix = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF}

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

// NextUp returns the net.IP that compares immediately greater than the given address.
func NextUp(addr net.IP) (net.IP, error) {
	encoded := EncodeInetAddress(addr)
	if bytes.Equal(encoded, InetAddressMaxValue) {
		return nil, fmt.Errorf("overflow: there is no greater InetAddress than %s", addr.String())
	}

	delta := make([]byte, InetAddressPointBytes)
	delta[InetAddressPointBytes-1] = 1

	result := make([]byte, InetAddressPointBytes)
	if err := util.Add(InetAddressPointBytes, 0, encoded, delta, result); err != nil {
		return nil, err
	}

	return DecodeInetAddress(result)
}

// NextDown returns the net.IP that compares immediately less than the given address.
func NextDown(addr net.IP) (net.IP, error) {
	encoded := EncodeInetAddress(addr)
	if bytes.Equal(encoded, InetAddressMinValue) {
		return nil, fmt.Errorf("underflow: there is no smaller InetAddress than %s", addr.String())
	}

	delta := make([]byte, InetAddressPointBytes)
	delta[InetAddressPointBytes-1] = 1

	result := make([]byte, InetAddressPointBytes)
	if err := util.Subtract(InetAddressPointBytes, 0, encoded, delta, result); err != nil {
		return nil, err
	}

	return DecodeInetAddress(result)
}

// EncodeInetAddress encodes an IPv4 or IPv6 address into the
// 16-byte representation expected by Lucene. IPv4 addresses are mapped to
// IPv4-in-IPv6 form (RFC 4291 §2.5.5.2).
func EncodeInetAddress(addr net.IP) []byte {
	if addr == nil {
		return nil
	}
	if v4 := addr.To4(); v4 != nil {
		out := make([]byte, InetAddressPointBytes)
		copy(out, ipv4Prefix)
		copy(out[12:], v4)
		return out
	}
	out := make([]byte, InetAddressPointBytes)
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
	if bytes.HasPrefix(encoded, ipv4Prefix) {
		return net.IPv4(encoded[12], encoded[13], encoded[14], encoded[15]), nil
	}
	return net.IP(encoded), nil
}

// String returns a string representation of the InetAddressPoint.
func (p *InetAddressPoint) String() string {
	addr, err := DecodeInetAddress(p.Field.Data.Bytes())
	if err != nil {
		return fmt.Sprintf("InetAddressPoint <%s:error>", p.Field.Name)
	}

	host := addr.String()
	if addr.To4() == nil {
		host = "[" + host + "]"
	}

	return fmt.Sprintf("InetAddressPoint <%s:%s>", p.Field.Name, host)
}
