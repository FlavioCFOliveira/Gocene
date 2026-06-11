// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene99HnswScalarQuantizedVectorsFormat backward-compatibility stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene99.Lucene99HnswScalarQuantizedVectorsFormat
	codecs.RegisterKnnVectorsFormat(codecs.NewReadOnlyKnnVectorsFormat("Lucene99HnswScalarQuantizedVectorsFormat"))

	// Lucene99ScalarQuantizedVectorsFormat backward-compatibility stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsFormat
	codecs.RegisterKnnVectorsFormat(codecs.NewReadOnlyKnnVectorsFormat("Lucene99ScalarQuantizedVectorsFormat"))
}
