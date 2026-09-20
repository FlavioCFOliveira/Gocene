// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/spi"

// TotalHitsRelation says how the total hit count should be interpreted. It
// renders the nested enum org.apache.lucene.search.TotalHits.Relation; Go has
// no nested types, so the enclosing type's name is folded into it, leaving the
// plain name Relation for the other nested enum Lucene declares,
// org.apache.lucene.index.PointValues.Relation ([index.Relation]).
type TotalHitsRelation = spi.TotalHitsRelation

const (
	// EQUAL_TO means the value is exact.
	EQUAL_TO = spi.EQUAL_TO
	// GREATER_THAN_OR_EQUAL_TO means the value is at least the given value.
	GREATER_THAN_OR_EQUAL_TO = spi.GREATER_THAN_OR_EQUAL_TO
)

// TotalHits represents the total number of hits.
type TotalHits = spi.TotalHits

// NewTotalHits creates a new TotalHits.
func NewTotalHits(value int64, relation TotalHitsRelation) *TotalHits {
	return spi.NewTotalHits(value, relation)
}

// IsExact is declared on spi.TotalHits, the type TotalHits aliases, and cannot
// be re-declared here: Go allows methods only on a package's own types. The
// duplicate that used to sit here had no Lucene counterpart either — Apache
// Lucene 10.5.0 declares TotalHits as a record with value() and relation()
// only.
