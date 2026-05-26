// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene912 is the real backward-compatibility PostingsFormat implementation
	// for the Lucene 9.12 wire layout. It rejects writes and will serve reads
	// once the full FieldsProducer port lands.
	// Mirrors: org.apache.lucene.backward_codecs.lucene912.Lucene912PostingsFormat
	codecs.RegisterPostingsFormat(NewLucene912PostingsFormat())
}
