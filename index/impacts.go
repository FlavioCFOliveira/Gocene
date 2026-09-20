// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Impacts conveys information about upcoming impacts (i.e. (freq, norm)
// pairs that may trigger non-zero scores) within a postings list. Mirrors
// org.apache.lucene.index.Impacts from Apache Lucene 10.5.0.
//
// Lucene declares Impacts once; spi carries the declaration because
// spi.TermsEnum#Impacts must name it, so the index spelling is an alias.
type Impacts = spi.Impacts
