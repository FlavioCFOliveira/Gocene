// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene84

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene84PostingsFormat is a backward-compatibility read-only stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene84.Lucene84PostingsFormat
	codecs.RegisterPostingsFormat(codecs.NewReadOnlyPostingsFormat("Lucene84PostingsFormat"))
}
