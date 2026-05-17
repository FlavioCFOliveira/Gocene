// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"net"
	"testing"
)

func TestInetAddressPoint_IPv4(t *testing.T) {
	p, err := NewInetAddressPoint("ip", net.IPv4(127, 0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	enc := p.BinaryValue()
	if len(enc) != 16 {
		t.Fatalf("encoded length = %d, want 16", len(enc))
	}
	// IPv4-mapped IPv6 prefix
	wantPrefix := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF}
	for i, v := range wantPrefix {
		if enc[i] != v {
			t.Fatalf("byte %d = %02x, want %02x", i, enc[i], v)
		}
	}
	if enc[12] != 127 || enc[15] != 1 {
		t.Fatalf("IPv4 suffix = %v", enc[12:])
	}
}

func TestInetAddressPoint_IPv6(t *testing.T) {
	addr := net.ParseIP("2001:db8::1")
	p, err := NewInetAddressPoint("ip", addr)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.BinaryValue()) != 16 {
		t.Fatalf("encoded length = %d", len(p.BinaryValue()))
	}
	decoded, err := DecodeInetAddress(p.BinaryValue())
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(addr) {
		t.Fatalf("decoded %v != original %v", decoded, addr)
	}
}

func TestInetAddressPoint_Nil(t *testing.T) {
	if _, err := NewInetAddressPoint("ip", nil); err == nil {
		t.Fatalf("expected error for nil IP")
	}
}

func TestInetAddressRange_Basic(t *testing.T) {
	min := net.IPv4(10, 0, 0, 1)
	max := net.IPv4(10, 0, 0, 100)
	r, err := NewInetAddressRange("r", min, max)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Min().Equal(min) || !r.Max().Equal(max) {
		t.Fatalf("min/max mismatch")
	}
	if len(r.BinaryValue()) != 32 {
		t.Fatalf("packed length = %d", len(r.BinaryValue()))
	}
}

func TestInetAddressRange_MinGreaterMaxErrors(t *testing.T) {
	min := net.IPv4(10, 0, 0, 100)
	max := net.IPv4(10, 0, 0, 1)
	if _, err := NewInetAddressRange("r", min, max); err == nil {
		t.Fatalf("expected error for min > max")
	}
}

func TestInetAddressMinMaxValues(t *testing.T) {
	if len(InetAddressMinValue) != 16 || len(InetAddressMaxValue) != 16 {
		t.Fatalf("min/max constants wrong size")
	}
	for _, b := range InetAddressMinValue {
		if b != 0 {
			t.Fatalf("min must be all zeros")
		}
	}
	for _, b := range InetAddressMaxValue {
		if b != 0xFF {
			t.Fatalf("max must be all 0xFF")
		}
	}
}
