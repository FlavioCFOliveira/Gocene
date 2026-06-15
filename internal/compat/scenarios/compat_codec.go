// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// compatCodec wraps Lucene104Codec and replaces formats that differ from the
// Apache Lucene 10.4.0 wire format with their Lucene90 equivalents.
// The Lucene 10.4 Java reference still uses the Lucene90 stored-fields and
// term-vectors wire formats, so a byte-compatible Gocene codec must do the
// same.
type compatCodec struct {
	codecs.Codec
	sf codecs.StoredFieldsFormat
	tv codecs.TermVectorsFormat
}

func newCompatCodec() *compatCodec {
	return &compatCodec{
		Codec: codecs.NewLucene104Codec(),
		sf:    lucene90.NewLucene90StoredFieldsFormat(),
		tv:    codecs.NewLucene90TermVectorsFormat(),
	}
}

func (c *compatCodec) StoredFieldsFormat() codecs.StoredFieldsFormat {
	return c.sf
}

func (c *compatCodec) TermVectorsFormat() codecs.TermVectorsFormat {
	return c.tv
}

var _ codecs.Codec = (*compatCodec)(nil)
