// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecbridge installs the production Lucene 10.4 codec as the
// default Codec for the index package.
//
// # Why this package exists
//
// The index/ package defines an index.Codec interface family that mirrors,
// but is not identical to, the codecs/ package's Codec interface family.
// The two families exist because index/ cannot import codecs/ (codecs/
// already imports index/ to reach concrete types such as *index.SegmentInfo
// and *index.FieldInfos), and structurally unifying the two families is a
// large enough refactor that it has been split out as a follow-up task
// (rmp #4669, "SPI unification"). Until that lands, the bridge defined
// here adapts a concrete codecs.Codec into an index.Codec so the
// IndexWriter can persist documents through real Lucene 10.4 formats by
// default.
//
// # Usage
//
// Production callers (binaries, integration tests, examples) blank-import
// this package to install the bridge as the index-side default:
//
//	import _ "github.com/FlavioCFOliveira/Gocene/internal/codecbridge"
//
// The package's init() calls index.RegisterDefaultCodec with a bridge
// wrapping codecs.NewLucene104Codec(). Tests that do not need a real codec
// (purely structural unit tests) may omit the import and accept that
// IndexWriter.AddDocument + Commit will surface index.ErrNoCodec.
//
// # Scope of adaptation
//
// The bridge adapts every interface method exposed by index.Codec:
// PostingsFormat, StoredFieldsFormat, FieldInfosFormat, SegmentInfosFormat,
// SegmentInfoFormat, and TermVectorsFormat. The only non-trivial conversion
// is StoredFieldsWriter.WriteField, where the inbound field is an
// index.IndexableField (the codec-facing narrow interface) but the
// codecs.StoredFieldsWriter contract expects document.IndexableField (the
// document-facing wider interface). That conversion is documented in
// adaptIndexableField below.
package codecbridge

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// init installs the production Lucene 10.4 codec as the default codec
// resolved by index.NewIndexWriterConfig.
func init() {
	index.RegisterDefaultCodec(BridgeForLucene104())
}

// BridgeForLucene104 returns an index.Codec that delegates to the
// production Lucene104Codec defined in package codecs.
//
// The returned value is safe for concurrent use; the underlying
// codecs.Codec implementations are stateless with respect to format
// dispatch.
func BridgeForLucene104() index.Codec {
	return NewBridge(codecs.NewLucene104Codec())
}

// NewBridge wraps an arbitrary codecs.Codec in an index.Codec adapter.
// Exported to allow tests and future callers to install custom codecs
// (e.g., Asserting or filtering codec variants) without going through
// the package-init default.
func NewBridge(c codecs.Codec) index.Codec {
	return &bridgeCodec{inner: c}
}

// bridgeCodec implements index.Codec by delegating each format accessor
// to a dedicated adapter that translates between the index and codecs
// SPI families.
type bridgeCodec struct {
	inner codecs.Codec
}

// Compile-time guarantee that bridgeCodec satisfies index.Codec.
var _ index.Codec = (*bridgeCodec)(nil)

func (b *bridgeCodec) Name() string {
	return b.inner.Name()
}

func (b *bridgeCodec) PostingsFormat() index.PostingsFormat {
	return &postingsFormatAdapter{inner: b.inner.PostingsFormat()}
}

func (b *bridgeCodec) StoredFieldsFormat() index.StoredFieldsFormat {
	return &storedFieldsFormatAdapter{inner: b.inner.StoredFieldsFormat()}
}

func (b *bridgeCodec) FieldInfosFormat() index.FieldInfosFormat {
	return &fieldInfosFormatAdapter{inner: b.inner.FieldInfosFormat()}
}

func (b *bridgeCodec) SegmentInfosFormat() index.SegmentInfosFormat {
	return &segmentInfosFormatAdapter{inner: b.inner.SegmentInfosFormat()}
}

func (b *bridgeCodec) SegmentInfoFormat() index.SegmentInfoFormat {
	// codecs.Codec does not expose SegmentInfoFormat directly; the .si
	// format is reachable only via the concrete Lucene99SegmentInfoFormat.
	// Mirror Lucene 10.4 wiring by instantiating it here.
	return &segmentInfoFormatAdapter{inner: codecs.NewLucene99SegmentInfoFormat()}
}

func (b *bridgeCodec) TermVectorsFormat() index.TermVectorsFormat {
	return &termVectorsFormatAdapter{inner: b.inner.TermVectorsFormat()}
}
