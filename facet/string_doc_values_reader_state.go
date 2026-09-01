// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// StringDocValuesReaderState stores an OrdinalMap created for a specific IndexReader and field.
//
// This is the Go port of Lucene's org.apache.lucene.facet.StringDocValuesReaderState.
type StringDocValuesReaderState struct {
	Reader    *index.IndexReader
	Field     string
	OrdinalMap *index.OrdinalMap
}

// NewStringDocValuesReaderState constructs state specific to a reader + field.
func NewStringDocValuesReaderState(reader *index.IndexReader, field string) (*StringDocValuesReaderState, error) {
	ordinalMap, err := buildOrdinalMap(reader, field)
	if err != nil {
		return nil, err
	}
	return &StringDocValuesReaderState{
		Reader:     reader,
		Field:      field,
		OrdinalMap: ordinalMap,
	}, nil
}

func buildOrdinalMap(reader *index.IndexReader, field string) (*index.OrdinalMap, error) {
	leaves := reader.Leaves()
	leafCount := len(leaves)

	if leafCount <= 1 {
		return nil, nil
	}

	docValues := make([]index.SortedSetDocValues, leafCount)
	for i := 0; i < leafCount; i++ {
		context := leaves[i]
		dv, err := index.GetSortedSet(context.Reader(), field)
		if err != nil {
			return nil, err
		}
		docValues[i] = dv
	}

	return index.BuildOrdinalMap(docValues)
}

func (s *StringDocValuesReaderState) String() string {
	return fmt.Sprintf("StringDocValuesReaderState(field=%s reader=%p)", s.Field, s.Reader)
}
