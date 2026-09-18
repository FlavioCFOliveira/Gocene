// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import "github.com/FlavioCFOliveira/Gocene/codecs"

// init registers the KnnVectorsFormats that the backward-codecs
// META-INF/services/org.apache.lucene.codecs.KnnVectorsFormat file of Apache
// Lucene 10.5.0 lists for this package.
func init() {
	// Mirrors: org.apache.lucene.backward_codecs.lucene102.Lucene102BinaryQuantizedVectorsFormat
	codecs.RegisterKnnVectorsFormat(NewLucene102BinaryQuantizedVectorsFormat())

	// Mirrors: org.apache.lucene.backward_codecs.lucene102.Lucene102HnswBinaryQuantizedVectorsFormat
	codecs.RegisterKnnVectorsFormat(NewLucene102HnswBinaryQuantizedVectorsFormat())
}
