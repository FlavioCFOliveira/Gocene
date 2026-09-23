// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// -----------------------------------------------------------------------------
// Test doubles
// -----------------------------------------------------------------------------

// fakeSortField is a minimal SortField stand-in that also satisfies
// [SortFieldNamer]. The Gocene built-in providers will wire in
// `*search.SortField` once Sprint 22 follow-ups port them; for now the
// SortFieldProvider contract is exercised against this local fake.
type fakeSortField struct {
	provider string
	payload  byte
}

func (f *fakeSortField) ProviderName() string { return f.provider }

// memDataOutput is a minimal [store.DataOutput] backed by bytes.Buffer.
// Only the methods the round-trip test exercises are functional; the
// rest panic to surface accidental dependencies.
type memDataOutput struct{ buf bytes.Buffer }

func (m *memDataOutput) WriteByte(b byte) error { return m.buf.WriteByte(b) }
func (m *memDataOutput) WriteBytes(p []byte, _ int, _ int) error {
	_, err := m.buf.Write(p)
	return err
}
func (m *memDataOutput) WriteBytesN(p []byte, n int) error {
	_, err := m.buf.Write(p[:n])
	return err
}
func (m *memDataOutput) WriteShort(v int16) error {
	return binary.Write(&m.buf, binary.BigEndian, v)
}
func (m *memDataOutput) WriteInt(v int32) error {
	return binary.Write(&m.buf, binary.BigEndian, v)
}
func (m *memDataOutput) WriteLong(v int64) error {
	return binary.Write(&m.buf, binary.BigEndian, v)
}
func (m *memDataOutput) WriteString(s string) error { _, err := m.buf.WriteString(s); return err }

// memDataInput is the read counterpart of [memDataOutput].
type memDataInput struct{ buf *bytes.Buffer }

func (m *memDataInput) ReadByte() (byte, error) { return m.buf.ReadByte() }
func (m *memDataInput) ReadBytes(pBuf []byte, offset, length int) error {
	p := pBuf[offset : offset+length]
	_, err := io.ReadFull(m.buf, p)
	return err
}
func (m *memDataInput) ReadBytesN(n int) ([]byte, error) {
	out := make([]byte, n)
	_, err := io.ReadFull(m.buf, out)
	return out, err
}
func (m *memDataInput) ReadShort() (int16, error) {
	var v int16
	err := binary.Read(m.buf, binary.BigEndian, &v)
	return v, err
}
func (m *memDataInput) ReadInt() (int32, error) {
	var v int32
	err := binary.Read(m.buf, binary.BigEndian, &v)
	return v, err
}
func (m *memDataInput) ReadLong() (int64, error) {
	var v int64
	err := binary.Read(m.buf, binary.BigEndian, &v)
	return v, err
}
func (m *memDataInput) ReadString() (string, error) { return m.buf.String(), nil }

// ReadFloats carries the default body Lucene gives DataInput.ReadFloats.
func (m *memDataInput) ReadFloats(dst []float32, offset int, len int) error {
	for i := 0; i < len; i++ {
		v, err := m.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = math.Float32frombits(uint32(v))
	}
	return nil
}

// ReadInts carries the default body Lucene gives DataInput.ReadInts.
func (m *memDataInput) ReadInts(dst []int32, offset int, length int) error {
	for i := 0; i < length; i++ {
		v, err := m.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

// ReadLongs carries the default body Lucene gives DataInput.ReadLongs.
func (m *memDataInput) ReadLongs(dst []int64, offset int, length int) error {
	for i := 0; i < length; i++ {
		v, err := m.ReadLong()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

// ReadMapOfStrings carries the default body Lucene gives DataInput.ReadMapOfStrings.
func (m *memDataInput) ReadMapOfStrings() (map[string]string, error) {
	count, err := m.ReadVInt()
	if err != nil {
		return nil, err
	}
	res := make(map[string]string, count)
	for i := 0; i < int(count); i++ {
		k, err := m.ReadString()
		if err != nil {
			return nil, err
		}
		v, err := m.ReadString()
		if err != nil {
			return nil, err
		}
		res[k] = v
	}
	return res, nil
}

// ReadSetOfStrings carries the default body Lucene gives DataInput.ReadSetOfStrings.
func (m *memDataInput) ReadSetOfStrings() ([]string, error) {
	count, err := m.ReadVInt()
	if err != nil {
		return nil, err
	}
	res := make([]string, 0, count)
	for i := 0; i < int(count); i++ {
		v, err := m.ReadString()
		if err != nil {
			return nil, err
		}
		res = append(res, v)
	}
	return res, nil
}

// ReadVInt carries the default body Lucene gives DataInput.ReadVInt.
func (m *memDataInput) ReadVInt() (int32, error) {
	var v int32
	for shift := 0; ; shift += 7 {
		b, err := m.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= int32(b&0x7F) << shift
		if b&0x80 == 0 {
			return v, nil
		}
	}
}

// ReadVLong carries the default body Lucene gives DataInput.ReadVLong.
func (m *memDataInput) ReadVLong() (int64, error) {
	var v int64
	for shift := 0; ; shift += 7 {
		b, err := m.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= int64(b&0x7F) << shift
		if b&0x80 == 0 {
			return v, nil
		}
	}
}

// ReadZInt carries the default body Lucene gives DataInput.ReadZInt.
func (m *memDataInput) ReadZInt() (int32, error) {
	v, err := m.ReadVInt()
	if err != nil {
		return 0, err
	}
	return int32(uint32(v)>>1) ^ -(v & 1), nil
}

// ReadZLong carries the default body Lucene gives DataInput.ReadZLong.
func (m *memDataInput) ReadZLong() (int64, error) {
	v, err := m.ReadVLong()
	if err != nil {
		return 0, err
	}
	return int64(uint64(v)>>1) ^ -(v & 1), nil
}

// SkipBytes carries the default body Lucene gives DataInput.SkipBytes.
func (m *memDataInput) SkipBytes(p0 int64) error {
	for i := int64(0); i < p0; i++ {
		if _, err := m.ReadByte(); err != nil {
			return err
		}
	}
	return nil
}

// CopyBytes carries the default body Lucene gives DataOutput.CopyBytes.
func (m *memDataOutput) CopyBytes(input spi.DataInput, numBytes int64) error {
	buf := make([]byte, 16384)
	for left := numBytes; left > 0; {
		n := len(buf)
		if left < int64(n) {
			n = int(left)
		}
		if err := input.ReadBytes(buf, 0, n); err != nil {
			return err
		}
		if err := m.WriteBytes(buf, 0, n); err != nil {
			return err
		}
		left -= int64(n)
	}
	return nil
}

// WriteGroupVInts is abstract in Lucene's DataOutput; this double does not support it.
func (m *memDataOutput) WriteGroupVInts(values []int32, limit int) error {
	return errors.New("memDataOutput.WriteGroupVInts: unsupported operation")
}

// WriteMapOfStrings carries the default body Lucene gives DataOutput.WriteMapOfStrings.
func (m *memDataOutput) WriteMapOfStrings(p0 map[string]string) error {
	if err := m.WriteVInt(int32(len(p0))); err != nil {
		return err
	}
	keys := make([]string, 0, len(p0))
	for k := range p0 {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := m.WriteString(k); err != nil {
			return err
		}
		if err := m.WriteString(p0[k]); err != nil {
			return err
		}
	}
	return nil
}

// WriteSetOfStrings carries the default body Lucene gives DataOutput.WriteSetOfStrings.
func (m *memDataOutput) WriteSetOfStrings(s []string) error {
	if err := m.WriteVInt(int32(len(s))); err != nil {
		return err
	}
	for _, v := range s {
		if err := m.WriteString(v); err != nil {
			return err
		}
	}
	return nil
}

// WriteVInt carries the default body Lucene gives DataOutput.WriteVInt.
func (m *memDataOutput) WriteVInt(i int32) error {
	v := uint32(i)
	for v >= 0x80 {
		if err := m.WriteByte(byte(v&0x7F) | 0x80); err != nil {
			return err
		}
		v >>= 7
	}
	return m.WriteByte(byte(v))
}

// WriteVLong carries the default body Lucene gives DataOutput.WriteVLong.
func (m *memDataOutput) WriteVLong(i int64) error {
	v := uint64(i)
	for v >= 0x80 {
		if err := m.WriteByte(byte(v&0x7F) | 0x80); err != nil {
			return err
		}
		v >>= 7
	}
	return m.WriteByte(byte(v))
}

// WriteZInt carries the default body Lucene gives DataOutput.WriteZInt.
func (m *memDataOutput) WriteZInt(i int32) error {
	return m.WriteVInt((i >> 31) ^ (i << 1))
}

// WriteZLong carries the default body Lucene gives DataOutput.WriteZLong.
func (m *memDataOutput) WriteZLong(i int64) error {
	return m.WriteVLong((i >> 63) ^ (i << 1))
}

// Compile-time assertions that the local adapters honour the store
// contract that SortFieldProvider builds on.
var (
	_ store.DataOutput = (*memDataOutput)(nil)
	_ store.DataInput  = (*memDataInput)(nil)
)

// roundtripProvider serialises a single byte payload. Sufficient to
// prove the round-trip contract and SPI dispatch end-to-end.
type roundtripProvider struct {
	name string
}

func (r *roundtripProvider) Name() string { return r.name }

func (r *roundtripProvider) ReadSortField(in store.DataInput) (SortFieldValue, error) {
	b, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	return &fakeSortField{provider: r.name, payload: b}, nil
}

func (r *roundtripProvider) WriteSortField(sf SortFieldValue, out store.DataOutput) error {
	f, ok := sf.(*fakeSortField)
	if !ok {
		return errors.New("WriteSortField: not a fakeSortField")
	}
	return out.WriteByte(f.payload)
}

// -----------------------------------------------------------------------------
// SPI loader-level tests — exercise NewNamedSPILoader directly so the
// behaviour can be asserted without polluting the package-level
// registry.
// -----------------------------------------------------------------------------

func TestSortFieldProvider_LoaderLookupUnknown(t *testing.T) {
	loader := util.NewNamedSPILoader[SortFieldProvider]("SortFieldProvider")
	if _, err := loader.Lookup("missing"); !errors.Is(err, util.ErrSPINotFound) {
		t.Fatalf("expected ErrSPINotFound, got %v", err)
	}
}

func TestSortFieldProvider_LoaderAvailableEmpty(t *testing.T) {
	loader := util.NewNamedSPILoader[SortFieldProvider]("SortFieldProvider")
	if got := loader.AvailableServices(); len(got) != 0 {
		t.Fatalf("expected no providers, got %v", got)
	}
}

// -----------------------------------------------------------------------------
// Package-level registry tests — register a sentinel provider once and
// exercise the public helpers against it.
// -----------------------------------------------------------------------------

const testProviderName = "gocenesortfieldprovidertest"

func init() {
	if err := RegisterSortFieldProvider(&roundtripProvider{name: testProviderName}); err != nil {
		panic("test setup: RegisterSortFieldProvider failed: " + err.Error())
	}
}

func TestRegisterSortFieldProvider_InvalidName(t *testing.T) {
	bad := &roundtripProvider{name: "has space"}
	err := RegisterSortFieldProvider(bad)
	if !errors.Is(err, util.ErrInvalidSPIName) {
		t.Fatalf("expected ErrInvalidSPIName, got %v", err)
	}
}

func TestLookupSortFieldProvider_Known(t *testing.T) {
	p, err := LookupSortFieldProvider(testProviderName)
	if err != nil {
		t.Fatalf("LookupSortFieldProvider: %v", err)
	}
	if p.Name() != testProviderName {
		t.Fatalf("expected name %q, got %q", testProviderName, p.Name())
	}
}

func TestLookupSortFieldProvider_Unknown(t *testing.T) {
	if _, err := LookupSortFieldProvider("doesnotexist"); !errors.Is(err, util.ErrSPINotFound) {
		t.Fatalf("expected ErrSPINotFound, got %v", err)
	}
}

func TestAvailableSortFieldProviders_ContainsRegistered(t *testing.T) {
	avail := AvailableSortFieldProviders()
	for _, n := range avail {
		if n == testProviderName {
			return
		}
	}
	t.Fatalf("expected %q in available providers, got %v", testProviderName, avail)
}

func TestReloadSortFieldProviders_NoOp(t *testing.T) {
	before := AvailableSortFieldProviders()
	ReloadSortFieldProviders()
	after := AvailableSortFieldProviders()
	if len(before) != len(after) {
		t.Fatalf("reload mutated registry: %d -> %d", len(before), len(after))
	}
}

// -----------------------------------------------------------------------------
// WriteSortField helper tests.
// -----------------------------------------------------------------------------

func TestWriteSortField_Nil(t *testing.T) {
	out := &memDataOutput{}
	if err := WriteSortField(nil, out); !errors.Is(err, ErrSortFieldNotSerializable) {
		t.Fatalf("expected ErrSortFieldNotSerializable, got %v", err)
	}
}

func TestWriteSortField_NoNamer(t *testing.T) {
	out := &memDataOutput{}
	val := struct{ x int }{x: 1}
	if err := WriteSortField(val, out); !errors.Is(err, ErrSortFieldNotSerializable) {
		t.Fatalf("expected ErrSortFieldNotSerializable, got %v", err)
	}
}

func TestWriteSortField_EmptyProviderName(t *testing.T) {
	out := &memDataOutput{}
	sf := &fakeSortField{provider: "", payload: 0xAB}
	if err := WriteSortField(sf, out); !errors.Is(err, ErrSortFieldNotSerializable) {
		t.Fatalf("expected ErrSortFieldNotSerializable, got %v", err)
	}
}

func TestWriteSortField_UnknownProvider(t *testing.T) {
	out := &memDataOutput{}
	sf := &fakeSortField{provider: "totallyunregistered", payload: 0xCD}
	err := WriteSortField(sf, out)
	if !errors.Is(err, ErrSortFieldNotSerializable) {
		t.Fatalf("expected ErrSortFieldNotSerializable, got %v", err)
	}
	if !errors.Is(err, util.ErrSPINotFound) {
		t.Fatalf("expected wrapped ErrSPINotFound, got %v", err)
	}
}

func TestWriteSortField_DispatchesToProvider(t *testing.T) {
	out := &memDataOutput{}
	sf := &fakeSortField{provider: testProviderName, payload: 0x42}
	if err := WriteSortField(sf, out); err != nil {
		t.Fatalf("WriteSortField: %v", err)
	}
	if got, want := out.buf.Bytes(), []byte{0x42}; !bytes.Equal(got, want) {
		t.Fatalf("wire bytes: got %v, want %v", got, want)
	}

	provider, err := LookupSortFieldProvider(testProviderName)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	got, err := provider.ReadSortField(&memDataInput{buf: &out.buf})
	if err != nil {
		t.Fatalf("ReadSortField: %v", err)
	}
	gotSF, ok := got.(*fakeSortField)
	if !ok {
		t.Fatalf("expected *fakeSortField, got %T", got)
	}
	if gotSF.payload != sf.payload {
		t.Fatalf("payload round-trip: got 0x%X, want 0x%X", gotSF.payload, sf.payload)
	}
	if gotSF.provider != sf.provider {
		t.Fatalf("provider round-trip: got %q, want %q", gotSF.provider, sf.provider)
	}
}
