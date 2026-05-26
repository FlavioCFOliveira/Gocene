// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene80 is the real backward-compatibility DocValuesFormat implementation
	// for the Lucene 8.0 doc values layout.
	// Mirrors: org.apache.lucene.backward_codecs.lucene80.Lucene80DocValuesFormat
	codecs.RegisterDocValuesFormat(NewLucene80DocValuesFormat())
}
