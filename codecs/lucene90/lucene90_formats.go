// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"strings"

	codecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	lucene90compressing "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
)

// The Sprint 48 lucene90 port surfaces the top-level format types as
// typed stubs alongside the pre-existing concrete helpers
// (indexed_disi.go, the LZ4/Deflate compression-mode pairs). Concrete
// behaviour ports land progressively in follow-up deep-port sprints.
//
// Lucene90StoredFieldsFormat lives in lucene90_stored_fields_format.go.

// Lucene90TermVectorsFormat is the Lucene 9.0 term vectors format: a
// Lucene90CompressingTermVectorsFormat with the parameters
// ("Lucene90TermVectorsData", "", CompressionMode.FAST, 1 << 12, 128, 10).
//
// This is the Go port of org.apache.lucene.codecs.lucene90.Lucene90TermVectorsFormat
// of Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsFormat.
type Lucene90TermVectorsFormat struct {
	*lucene90compressing.Lucene90CompressingTermVectorsFormat
}

// NewLucene90TermVectorsFormat is the sole constructor of
// Lucene90TermVectorsFormat.
func NewLucene90TermVectorsFormat() *Lucene90TermVectorsFormat {
	return &Lucene90TermVectorsFormat{
		Lucene90CompressingTermVectorsFormat: lucene90compressing.NewLucene90CompressingTermVectorsFormat(
			"Lucene90TermVectorsData", "", compressing.FAST, 1<<12, 128, 10),
	}
}

// Name returns the simple class name (see
// Lucene90CompressingTermVectorsFormat.Name).
func (f *Lucene90TermVectorsFormat) Name() string {
	return "Lucene90TermVectorsFormat"
}

// String renders the inherited Lucene90CompressingTermVectorsFormat.toString(),
// which prints getClass().getSimpleName() of the runtime class.
func (f *Lucene90TermVectorsFormat) String() string {
	return strings.Replace(f.Lucene90CompressingTermVectorsFormat.String(),
		"Lucene90CompressingTermVectorsFormat", "Lucene90TermVectorsFormat", 1)
}

func init() {
	// Arms codecs.NewLucene90TermVectorsFormat, the spelling of
	// `new Lucene90TermVectorsFormat()` that Lucene104Codec needs but cannot
	// reach directly (codecs cannot import this package).
	codecs.RegisterLucene90TermVectorsFormat(func() codecs.TermVectorsFormat {
		return NewLucene90TermVectorsFormat()
	})
}

var _ codecs.TermVectorsFormat = (*Lucene90TermVectorsFormat)(nil)
