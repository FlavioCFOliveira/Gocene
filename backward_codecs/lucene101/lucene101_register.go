// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene101

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene101PostingsFormat is a backward-compatibility read-only stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene101.Lucene101PostingsFormat
	codecs.RegisterPostingsFormat(codecs.NewReadOnlyPostingsFormat("Lucene101PostingsFormat"))
}
