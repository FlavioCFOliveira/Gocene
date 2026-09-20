// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import "github.com/FlavioCFOliveira/Gocene/spi"

// PointsReader is the Go port of org.apache.lucene.codecs.PointsReader from
// Apache Lucene 10.5.0.
//
// Lucene declares this contract exactly once, in org.apache.lucene.codecs,
// with four members: checkIntegrity(), getValues(String), getMergeInstance()
// and the close() it inherits from Closeable. Gocene had declared it twice —
// a two-member spi.PointsReader plus a wider codecs.PointsReader embedding it
// — which made spi.PointsFormat.FieldsReader and
// codecs.Lucene90PointsFormat.FieldsReader two incompatible signatures for the
// same Lucene method.
//
// The whole contract now lives in spi, where Lucene's PointValues is already
// declared (index.PointValues is an alias of spi.PointValues), so nothing
// forced the split. This alias keeps the Lucene name available in the package
// Lucene declares it in, matching the convention used for CompoundDirectory,
// DocValuesProducer, KnnVectorsReader and NormsProducer.
type PointsReader = spi.PointsReader
