// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// DocValuesProducer is an abstract API that produces numeric, binary, sorted,
// sortedset, and sortednumeric docvalues.
//
// Mirrors org.apache.lucene.codecs.DocValuesProducer in Apache Lucene 10.5.0.
//
// The declaration itself lives in the spi package: index names this contract
// (index.DocValuesProducer) and index is imported by codecs, so hosting it here
// directly would close an index <-> codecs import cycle. spi.DocValuesFormat
// .FieldsProducer likewise traffics in spi.DocValuesProducer, which is why this
// is an alias rather than a wider codecs-side interface — a wider interface
// could never be satisfied by the value the format API returns.
type DocValuesProducer = spi.DocValuesProducer
