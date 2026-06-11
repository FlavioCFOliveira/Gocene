// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import "log"

// init registers every PostingsFormat, DocValuesFormat, and KnnVectorsFormat
// implementation that lives in the codecs package itself.
//
// This mirrors the Java ServiceLoader registrations declared in:
//   - lucene/core/src/resources/META-INF/services/org.apache.lucene.codecs.PostingsFormat
//   - lucene/core/src/resources/META-INF/services/org.apache.lucene.codecs.DocValuesFormat
//   - lucene/core/src/resources/META-INF/services/org.apache.lucene.codecs.KnnVectorsFormat
//
// Backward-compatibility format registrations are performed by the
// backward_codecs sub-packages (each has its own init() in a *_register.go
// file, triggered via blank imports in backward_codecs/backward_codecs.go).
func init() {
	// ---- PostingsFormats ----

	// Lucene104 is the current production postings format (Lucene 10.4.0).
	// Mirrors: org.apache.lucene.codecs.lucene104.Lucene104PostingsFormat
	RegisterPostingsFormat(NewLucene104PostingsFormat())

	// Lucene103PostingsFormat is the previous generation postings format.
	// Mirrors: org.apache.lucene.codecs.lucene103.Lucene103PostingsFormat
	// (the deep-port read path is deferred; the registration ensures
	// PostingsFormatByName resolves the name from PerField attributes).
	RegisterPostingsFormat(NewLucene103PostingsFormat())

	// Lucene99PostingsFormat is the backward-compatibility format for
	// Lucene 9.9 postings (BLOCK_SIZE=128, long-based ForUtil). Gocene
	// extends the Java contract (read-only) to support writing for
	// round-trip testing.
	RegisterPostingsFormat(NewLucene99PostingsFormat())

	// PerField40 is the per-field dispatch wrapper format.
	// Mirrors: org.apache.lucene.codecs.perfield.PerFieldPostingsFormat
	// The default delegate is Lucene104; callers that need a different
	// delegate must construct PerFieldPostingsFormat directly.
	RegisterPostingsFormat(
		NewPerFieldPostingsFormatWithDefault(NewLucene104PostingsFormat()),
	)

	// ---- DocValuesFormats ----

	// Lucene90 is the current production doc values format (Lucene 9.0+).
	// Mirrors: org.apache.lucene.codecs.lucene90.Lucene90DocValuesFormat
	RegisterDocValuesFormat(NewLucene90DocValuesFormat())

	// PerFieldDV40 is the per-field dispatch wrapper for doc values.
	// Mirrors: org.apache.lucene.codecs.perfield.PerFieldDocValuesFormat
	RegisterDocValuesFormat(
		NewPerFieldDocValuesFormatWithDefault(NewLucene90DocValuesFormat()),
	)

	// ---- KnnVectorsFormats ----

	// Lucene99HnswVectorsFormat is the current production HNSW vectors format.
	// Mirrors: org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsFormat
	// NewLucene99HnswVectorsFormat validates its parameters and can return an
	// error; the default parameters are valid, so panic on failure is safe here.
	lucene99Hnsw, err := NewLucene99HnswVectorsFormat()
	if err != nil {
		log.Printf("codecs: WARNING: failed to create default Lucene99HnswVectorsFormat: %v", err)
	}
	RegisterKnnVectorsFormat(lucene99Hnsw)

	// Lucene104ScalarQuantizedVectorsFormat is the scalar-quantized flat
	// vectors format introduced in Lucene 10.4.
	// Mirrors: org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorsFormat
	RegisterKnnVectorsFormat(NewLucene104ScalarQuantizedVectorsFormat())

	// Lucene104HnswScalarQuantizedVectorsFormat combines HNSW graph traversal
	// with scalar quantization. The deep-port implementation is deferred; this
	// stub ensures KnnVectorsFormatByName resolves the canonical name.
	// Mirrors: org.apache.lucene.codecs.lucene104.Lucene104HnswScalarQuantizedVectorsFormat
	RegisterKnnVectorsFormat(NewReadOnlyKnnVectorsFormat("Lucene104HnswScalarQuantizedVectorsFormat"))

	// PerFieldVectors90 is the per-field dispatch wrapper for KNN vectors.
	// Mirrors: org.apache.lucene.codecs.perfield.PerFieldKnnVectorsFormat
	lucene99HnswDefault, err2 := NewLucene99HnswVectorsFormat()
	if err2 != nil {
		panic("codecs: failed to create Lucene99HnswVectorsFormat for PerField default: " + err2.Error())
	}
	RegisterKnnVectorsFormat(
		NewPerFieldKnnVectorsFormatWithDefault(lucene99HnswDefault),
	)
}
