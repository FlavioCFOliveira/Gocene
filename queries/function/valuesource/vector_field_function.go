// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// VectorFieldFunction is a base implementation for retrieving FunctionValues
// instances for knn vectors fields.
type VectorFieldFunction struct {
	function.BaseFunctionValues
	valueSource function.ValueSource
	lastDocID   int
}

// NewVectorFieldFunction creates a new VectorFieldFunction.
func NewVectorFieldFunction(valueSource function.ValueSource) *VectorFieldFunction {
	return &VectorFieldFunction{
		valueSource: valueSource,
		lastDocID:   -1,
	}
}

// GetVectorIterator is implemented by concrete subtypes.
type vectorIteratorProvider interface {
	getVectorIterator() search.DocIdSetIterator
}

// Exists reports whether the document has a vector.
func (v *VectorFieldFunction) Exists(doc int) (bool, error) {
	if doc < v.lastDocID {
		return false, fmt.Errorf("docs were sent out-of-order: lastDocID=%d vs docID=%d", v.lastDocID, doc)
	}

	v.lastDocID = doc

	provider, ok := v.Self.(vectorIteratorProvider)
	if !ok {
		return false, fmt.Errorf("FunctionValues does not provide a vector iterator")
	}

	it := provider.getVectorIterator()
	curDocID := it.DocID()
	if doc > curDocID {
		advanced, err := it.Advance(doc)
		if err != nil {
			return false, err
		}
		curDocID = advanced
	}
	return doc == curDocID, nil
}

// ToString renders a representation of the value.
func (v *VectorFieldFunction) ToString(doc int) (string, error) {
	// In a real implementation, this would use the specific vector value.
	// For now, we follow Lucene's pattern of combining description and value.
	return v.valueSource.Description() + " (value omitted)", nil
}

// CheckField verifies the Vector Encoding of a field.
func CheckField(reader index.LeafReader, field string, expectedEncoding index.VectorEncoding) error {
	fi := reader.GetFieldInfos().FieldInfo(field)
	if fi != nil {
		var actual index.VectorEncoding
		if fi.HasVectorValues() {
			actual = fi.GetVectorEncoding()
		} else {
			actual = -1 // Not a vector field
		}
		if expectedEncoding != actual {
			return fmt.Errorf("unexpected vector encoding (%v) for field %s (expected=%v)", actual, field, expectedEncoding)
		}
	}
	return nil
}
