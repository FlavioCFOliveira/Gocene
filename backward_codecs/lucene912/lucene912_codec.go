// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// Lucene912Codec implements the Lucene 9.12 index format.
//
// It wraps Lucene104Codec via FilterCodec and overrides only the postings
// format to return Lucene912PostingsFormat. All other component formats
// (stored fields, field infos, segment infos, term vectors, doc values) are
// delegated to the current (Lucene 10.4) implementation — which matches the
// formats actually used for those components by the original Lucene 9.12
// Codec (e.g. Lucene90StoredFieldsFormat, Lucene94FieldInfosFormat, etc.).
//
// Port of org.apache.lucene.backward_codecs.lucene912.Lucene912Codec
// (Lucene 10.4.0).
type Lucene912Codec struct {
	*codecs.FilterCodec
	postingsFormat codecs.PostingsFormat
}

// NewLucene912Codec returns a Lucene912Codec delegating to Lucene104Codec.
func NewLucene912Codec() *Lucene912Codec {
	return &Lucene912Codec{
		FilterCodec:    codecs.NewFilterCodec("Lucene912", codecs.NewLucene104Codec()),
		postingsFormat: NewLucene912PostingsFormat(),
	}
}

// PostingsFormat returns the Lucene 9.12 postings format.
func (c *Lucene912Codec) PostingsFormat() codecs.PostingsFormat {
	return c.postingsFormat
}
