// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

// Tests to ensure that Codec implementations need only wire the components
// they actually use; unused accessors may panic ("unsupported") without
// affecting consumers that exercise a small subset of Lucene's functionality.
//
// This is the Go port of org.apache.lucene.codecs.TestMinimalCodec.
//
// Divergence vs. JVM: the upstream test drives an end-to-end IndexWriter
// round-trip to confirm that the minimal codec survives flush/forceMerge.
// The Go Codec interface (see codec.go) exposes only the seven accessors
// listed below, and the Gocene IndexWriter pipeline does not yet consume
// codec-supplied CompoundFormat/LiveDocsFormat/NormsFormat/PointsFormat/
// KnnVectorsFormat instances, so an end-to-end assertion is not possible
// today. Instead, this test pins the *structural* contract upstream relies
// on: a "minimal" codec must delegate the components it does support and
// must panic with "unsupported" for everything it deliberately omits, and a
// compound-aware subclass must override only the compound accessor.

// minimalTestCodec is the analogue of upstream TestMinimalCodec.MinimalCodec:
// it delegates the formats consumed by the Gocene Codec interface to a
// wrapped default codec and panics on every accessor flagged as unsupported.
type minimalTestCodec struct {
	*BaseCodec
	wrapped Codec
}

func newMinimalTestCodec() *minimalTestCodec {
	return newNamedMinimalTestCodec("MinimalCodec")
}

func newNamedMinimalTestCodec(name string) *minimalTestCodec {
	return &minimalTestCodec{
		BaseCodec: NewBaseCodec(name),
		wrapped:   NewLucene104Codec(),
	}
}

func (c *minimalTestCodec) FieldInfosFormat() FieldInfosFormat {
	return c.wrapped.FieldInfosFormat()
}

func (c *minimalTestCodec) SegmentInfosFormat() SegmentInfosFormat {
	return c.wrapped.SegmentInfosFormat()
}

func (c *minimalTestCodec) StoredFieldsFormat() StoredFieldsFormat {
	// TODO: avoid calling this when no stored fields are written or read
	return c.wrapped.StoredFieldsFormat()
}

func (c *minimalTestCodec) PostingsFormat() PostingsFormat {
	panic("unsupported")
}

func (c *minimalTestCodec) DocValuesFormat() DocValuesFormat {
	panic("unsupported")
}

func (c *minimalTestCodec) TermVectorsFormat() TermVectorsFormat {
	panic("unsupported")
}

// minimalCompoundTestCodec is the analogue of MinimalCompoundCodec. It does
// not exist as a distinct override here because the Gocene Codec interface
// has no compoundFormat() accessor; the type is kept as a structural marker
// to mirror the upstream class hierarchy and to assert the distinct name.
type minimalCompoundTestCodec struct {
	*minimalTestCodec
}

func newMinimalCompoundTestCodec() *minimalCompoundTestCodec {
	return &minimalCompoundTestCodec{
		minimalTestCodec: newNamedMinimalTestCodec("MinimalCompoundCodec"),
	}
}

func TestMinimalCodec(t *testing.T) {
	t.Parallel()
	runMinimalCodecTest(t, false)
}

func TestMinimalCompoundCodec(t *testing.T) {
	t.Parallel()
	runMinimalCodecTest(t, true)
}

func runMinimalCodecTest(t *testing.T, useCompoundFile bool) {
	t.Helper()

	var codec Codec
	var wantName string
	if useCompoundFile {
		codec = newMinimalCompoundTestCodec()
		wantName = "MinimalCompoundCodec"
	} else {
		codec = newMinimalTestCodec()
		wantName = "MinimalCodec"
	}

	if got := codec.Name(); got != wantName {
		t.Fatalf("Name() = %q, want %q", got, wantName)
	}

	// Supported accessors must delegate to the wrapped default codec and
	// therefore return non-nil values.
	if codec.FieldInfosFormat() == nil {
		t.Fatalf("FieldInfosFormat() returned nil; expected delegated value")
	}
	if codec.SegmentInfosFormat() == nil {
		t.Fatalf("SegmentInfosFormat() returned nil; expected delegated value")
	}
	if codec.StoredFieldsFormat() == nil {
		t.Fatalf("StoredFieldsFormat() returned nil; expected delegated value")
	}

	// Unsupported accessors must panic with "unsupported" to mirror the
	// upstream UnsupportedOperationException contract.
	assertUnsupported(t, "PostingsFormat", func() { codec.PostingsFormat() })
	assertUnsupported(t, "DocValuesFormat", func() { codec.DocValuesFormat() })
	assertUnsupported(t, "TermVectorsFormat", func() { codec.TermVectorsFormat() })
}

func assertUnsupported(t *testing.T, accessor string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s() did not panic; expected \"unsupported\"", accessor)
		}
		msg, ok := r.(string)
		if !ok || msg != "unsupported" {
			t.Fatalf("%s() panicked with %v (%T); want string \"unsupported\"", accessor, r, r)
		}
	}()
	fn()
}
