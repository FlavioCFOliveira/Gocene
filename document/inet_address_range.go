// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"fmt"
	"net"
)

// InetAddressRange is an indexed [min, max] IP-address range. Both
// endpoints are encoded as 16-byte InetAddressPoint values; the
// underlying FieldType uses dimensionCount=2 and numBytes=16 so the
// per-range payload is 32 bytes.
//
// Go port of Lucene 10.4.0's org.apache.lucene.document.InetAddressRange.
//
// Static query factories (NewIntersectsQuery / NewContainsQuery /
// NewWithinQuery / NewCrossesQuery) deferred — see backlog #2695.
type InetAddressRange struct {
	*Field
	min net.IP
	max net.IP
}

var (
	// InetAddressRangeType is the FieldType for an InetAddressRange:
	// dimensionCount=2, numBytes=16.
	InetAddressRangeType *FieldType

	// InetAddressRangeTYPE is the Lucene-canonical alias.
	InetAddressRangeTYPE *FieldType
)

func init() {
	InetAddressRangeType = NewFieldType()
	InetAddressRangeType.SetIndexed(true)
	InetAddressRangeType.SetDimensions(2, InetAddressPointBytes)
	InetAddressRangeType.Freeze()
	InetAddressRangeTYPE = InetAddressRangeType
}

// NewInetAddressRange creates an InetAddressRange covering [min, max].
// Both endpoints are required to be non-nil; min must be <= max (in
// the unsigned-byte ordering of the encoded form).
func NewInetAddressRange(name string, min, max net.IP) (*InetAddressRange, error) {
	if min == nil || max == nil {
		return nil, fmt.Errorf("InetAddressRange endpoints must be non-nil")
	}
	encMin := EncodeInetAddress(min)
	encMax := EncodeInetAddress(max)
	if bytes.Compare(encMin, encMax) > 0 {
		return nil, fmt.Errorf("min %v > max %v", min, max)
	}
	packed := make([]byte, 2*InetAddressPointBytes)
	copy(packed[:InetAddressPointBytes], encMin)
	copy(packed[InetAddressPointBytes:], encMax)
	field, err := NewField(name, packed, InetAddressRangeType)
	if err != nil {
		return nil, err
	}
	dupMin := make(net.IP, len(min))
	copy(dupMin, min)
	dupMax := make(net.IP, len(max))
	copy(dupMax, max)
	return &InetAddressRange{Field: field, min: dupMin, max: dupMax}, nil
}

// Min returns a copy of the minimum address.
func (r *InetAddressRange) Min() net.IP {
	out := make(net.IP, len(r.min))
	copy(out, r.min)
	return out
}

// Max returns a copy of the maximum address.
func (r *InetAddressRange) Max() net.IP {
	out := make(net.IP, len(r.max))
	copy(out, r.max)
	return out
}
