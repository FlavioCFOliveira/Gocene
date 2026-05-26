// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene102BinaryQuantizedVectorsFormat backward-compatibility stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene102.Lucene102BinaryQuantizedVectorsFormat
	codecs.RegisterKnnVectorsFormat(codecs.NewReadOnlyKnnVectorsFormat("Lucene102BinaryQuantizedVectorsFormat"))

	// Lucene102HnswBinaryQuantizedVectorsFormat backward-compatibility stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene102.Lucene102HnswBinaryQuantizedVectorsFormat
	codecs.RegisterKnnVectorsFormat(codecs.NewReadOnlyKnnVectorsFormat("Lucene102HnswBinaryQuantizedVectorsFormat"))
}
