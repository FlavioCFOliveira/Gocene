// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// NormsProducer is an abstract API that produces field normalization values.
//
// Mirrors org.apache.lucene.codecs.NormsProducer in Apache Lucene 10.5.0.
//
// The declaration itself lives in the spi package: index names this contract
// and index is imported by codecs, so hosting it here directly would close an
// index <-> codecs import cycle. spi.NormsFormat.NormsProducer likewise
// traffics in spi.NormsProducer, which is why this is an alias rather than a
// wider codecs-side interface.
type NormsProducer = spi.NormsProducer
