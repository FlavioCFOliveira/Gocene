// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

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

func (m *memDataOutput) WriteByte(b byte) error    { return m.buf.WriteByte(b) }
func (m *memDataOutput) WriteBytes(p []byte) error { _, err := m.buf.Write(p); return err }
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

func (m *memDataInput) ReadByte() (byte, error)  { return m.buf.ReadByte() }
func (m *memDataInput) ReadBytes(p []byte) error { _, err := io.ReadFull(m.buf, p); return err }
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
