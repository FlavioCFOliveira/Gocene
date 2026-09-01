// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// GlobalOrdinalsCollector collects global ordinals from a set of documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.GlobalOrdinalsCollector.
type GlobalOrdinalsCollector struct {
	field      string
	ordinals   map[int]int
	ordinalMap *index.OrdinalMap
}

func NewGlobalOrdinalsCollector(field string, ordinalMap *index.OrdinalMap) *GlobalOrdinalsCollector {
	return &GlobalOrdinalsCollector{
		field:      field,
		ordinals:   make(map[int]int),
		ordinalMap: ordinalMap,
	}
}

// Collect collects the global ordinal for the given document.
func (c *GlobalOrdinalsCollector) Collect(doc int, context *index.LeafReaderContext) {
	dv, err := index.GetSortedSet(context.Reader(), c.field)
	if err != nil {
		return
	}

	// Simplified logic: get the ordinal and map it to global.
	ord := dv.OrdValue()
	globalOrd := c.ordinalMap.GetGlobalOrd(context.Ord, ord)
	c.ordinals[doc] = int(globalOrd)
}

func (c *GlobalOrdinalsCollector) GetOrdinals() map[int]int {
	return c.ordinals
}
