// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// ImpactsSource produces Impacts and supports shallow-advance to allow callers
// to retrieve more precise impact information for upcoming docs. Mirrors
// org.apache.lucene.index.ImpactsSource from Apache Lucene 10.5.0.
//
// Lucene declares ImpactsSource once; spi carries the declaration, so the index
// spelling is an alias.
type ImpactsSource = spi.ImpactsSource
