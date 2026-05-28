// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsCollectorSV collects all single-valued terms from a specified field.
// One term per document is collected (the doc's sorted ordinal).
//
// Mirrors org.apache.lucene.search.join.TermsCollector.SV.
type TermsCollectorSV struct {
	*DocValuesTermsCollector[index.SortedDocValues]
	collectedTerms *util.BytesRefHash
}

// NewTermsCollectorSV creates a single-value TermsCollector for the given field.
func NewTermsCollectorSV(field string) *TermsCollectorSV {
	hash := util.NewBytesRefHash()
	c := &TermsCollectorSV{collectedTerms: hash}

	dvFunc := func(lr *index.LeafReader) (index.SortedDocValues, error) {
		return lr.GetSortedDocValues(field)
	}
	// Migrated to AdvanceExact + OrdValue (rmp #4709). The collect
	// callback is invoked in monotonic doc order by the search collector
	// framework.
	collectFn := func(dv index.SortedDocValues, doc int) error {
		if dv == nil {
			_, _ = hash.Add(util.NewBytesRefEmpty())
			return nil
		}
		ok, err := dv.AdvanceExact(doc)
		if err != nil {
			return err
		}
		var term []byte
		if ok {
			ord, err := dv.OrdValue()
			if err != nil {
				return err
			}
			if ord >= 0 {
				term, err = dv.LookupOrd(ord)
				if err != nil {
					return err
				}
			}
		}
		_, err = hash.Add(util.NewBytesRef(term))
		return err
	}

	c.DocValuesTermsCollector = newDocValuesTermsCollector(dvFunc, collectFn, search.COMPLETE_NO_SCORES)
	return c
}

// GetCollectorTerms returns the collected BytesRefHash.
func (c *TermsCollectorSV) GetCollectorTerms() *util.BytesRefHash { return c.collectedTerms }

// TermsCollectorMV collects all multi-valued terms from a specified field.
// All ordinals for each document are collected.
//
// Mirrors org.apache.lucene.search.join.TermsCollector.MV.
type TermsCollectorMV struct {
	*DocValuesTermsCollector[index.SortedSetDocValues]
	collectedTerms *util.BytesRefHash
}

// NewTermsCollectorMV creates a multi-value TermsCollector for the given field.
func NewTermsCollectorMV(field string) *TermsCollectorMV {
	hash := util.NewBytesRefHash()
	c := &TermsCollectorMV{collectedTerms: hash}

	dvFunc := func(lr *index.LeafReader) (index.SortedSetDocValues, error) {
		return lr.GetSortedSetDocValues(field)
	}
	// Migrated to AdvanceExact + NextOrd (rmp #4709). The collect
	// callback runs in monotonic doc order; NextOrd yields the current
	// doc's ords until -1.
	collectFn := func(dv index.SortedSetDocValues, doc int) error {
		if dv == nil {
			return nil
		}
		ok, err := dv.AdvanceExact(doc)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		for {
			ord, err := dv.NextOrd()
			if err != nil {
				return err
			}
			if ord < 0 {
				return nil
			}
			term, err := dv.LookupOrd(ord)
			if err != nil {
				return err
			}
			if _, err = hash.Add(util.NewBytesRef(term)); err != nil {
				return err
			}
		}
	}

	c.DocValuesTermsCollector = newDocValuesTermsCollector(dvFunc, collectFn, search.COMPLETE_NO_SCORES)
	return c
}

// GetCollectorTerms returns the collected BytesRefHash.
func (c *TermsCollectorMV) GetCollectorTerms() *util.BytesRefHash { return c.collectedTerms }

// CreateTermsCollector is a convenience factory that chooses SV or MV based on
// the multipleValuesPerDocument flag.
func CreateTermsCollector(field string, multipleValuesPerDocument bool) search.Collector {
	if multipleValuesPerDocument {
		return NewTermsCollectorMV(field)
	}
	return NewTermsCollectorSV(field)
}

// interface compliance
var _ search.Collector = (*TermsCollectorSV)(nil)
var _ search.Collector = (*TermsCollectorMV)(nil)
