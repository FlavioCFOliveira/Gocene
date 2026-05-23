// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// Lucene80Codec implements the Lucene 8.0 index format.
//
// The Gocene Codec interface covers six components. Of those, only
// DocValuesFormat is fully wired in this release; the remaining five
// (PostingsFormat, StoredFieldsFormat, FieldInfosFormat, SegmentInfosFormat,
// TermVectorsFormat) delegate to the default nil provided by BaseCodec until
// the corresponding lucene50/lucene60/lucene70 sub-packages grow real
// interface implementations.
//
// Additionally exposes NormsFormat() as a struct-level method, consistent
// with the Java codec; it will become part of the Gocene Codec interface when
// that interface is extended.
//
// Port of org.apache.lucene.backward_codecs.lucene80.Lucene80Codec (Lucene 10.4.0).
type Lucene80Codec struct {
	*codecs.BaseCodec
	docValuesFormat codecs.DocValuesFormat
	normsFormat     codecs.NormsFormat
}

// NewLucene80Codec creates a new Lucene80Codec.
//
// Port of Lucene80Codec() (Lucene 10.4.0).
func NewLucene80Codec() *Lucene80Codec {
	return &Lucene80Codec{
		BaseCodec:       codecs.NewBaseCodec("Lucene80"),
		docValuesFormat: NewLucene80DocValuesFormat(),
		normsFormat:     NewLucene80NormsFormat(),
	}
}

// DocValuesFormat returns the Lucene 8.0 doc values format.
func (c *Lucene80Codec) DocValuesFormat() codecs.DocValuesFormat {
	return c.docValuesFormat
}

// NormsFormat returns the Lucene 8.0 norms format.
//
// This method is not yet part of the Gocene Codec interface; it is exposed
// as a struct method for callers that know the concrete type.
func (c *Lucene80Codec) NormsFormat() codecs.NormsFormat {
	return c.normsFormat
}

// compile-time assertion
var _ codecs.Codec = (*Lucene80Codec)(nil)
