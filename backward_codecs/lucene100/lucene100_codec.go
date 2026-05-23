// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene100

import (
	"github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene912"
	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// Lucene100Mode controls the stored-fields compression strategy used when
// writing new segments with this codec.
//
// Port of org.apache.lucene.backward_codecs.lucene100.Lucene100Codec.Mode.
type Lucene100Mode int

const (
	// Lucene100ModeBestSpeed trades compression ratio for retrieval speed.
	Lucene100ModeBestSpeed Lucene100Mode = iota
	// Lucene100ModeBestCompression trades retrieval speed for compression ratio.
	Lucene100ModeBestCompression
)

// Lucene100Codec implements the Lucene 10.0 index format.
//
// Gocene deviation: like all backward codecs in Gocene, this type wraps
// Lucene104Codec via FilterCodec and overrides only the postings format.
// The stored-fields Mode parameter is accepted for API compatibility but does
// not alter the underlying Lucene104StoredFieldsFormat used at runtime. This
// is the same trade-off documented for Lucene912Codec.
//
// Port of org.apache.lucene.backward_codecs.lucene100.Lucene100Codec
// (Lucene 10.4.0).
type Lucene100Codec struct {
	*codecs.FilterCodec
	postingsFormat codecs.PostingsFormat
	mode           Lucene100Mode
}

// NewLucene100Codec creates a Lucene100Codec using BEST_SPEED mode.
//
// Port of Lucene100Codec() (Lucene 10.4.0).
func NewLucene100Codec() *Lucene100Codec {
	return NewLucene100CodecWithMode(Lucene100ModeBestSpeed)
}

// NewLucene100CodecWithMode creates a Lucene100Codec with the given mode.
//
// Port of Lucene100Codec(Mode) (Lucene 10.4.0).
func NewLucene100CodecWithMode(mode Lucene100Mode) *Lucene100Codec {
	return &Lucene100Codec{
		FilterCodec:    codecs.NewFilterCodec("Lucene100", codecs.NewLucene104Codec()),
		postingsFormat: lucene912.NewLucene912PostingsFormat(),
		mode:           mode,
	}
}

// PostingsFormat returns the Lucene 9.12 postings format used by Lucene 10.0
// segments.
func (c *Lucene100Codec) PostingsFormat() codecs.PostingsFormat {
	return c.postingsFormat
}

// Mode returns the stored-fields compression mode this codec was constructed
// with.
func (c *Lucene100Codec) Mode() Lucene100Mode {
	return c.mode
}

// compile-time assertion
var _ codecs.Codec = (*Lucene100Codec)(nil)
