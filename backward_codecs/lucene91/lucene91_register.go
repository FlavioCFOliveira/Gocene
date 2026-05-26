// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene91

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene91HnswVectorsFormat is a backward-compatibility KNN vectors stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene91.Lucene91HnswVectorsFormat
	codecs.RegisterKnnVectorsFormat(codecs.NewReadOnlyKnnVectorsFormat("Lucene91HnswVectorsFormat"))
}
