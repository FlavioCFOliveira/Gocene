// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// LongFieldSource obtains long field values from NumericDocValues and
// makes those values available as other numeric types, casting as needed.
type LongFieldSource struct {
	FieldCacheSource
}

func NewLongFieldSource(field string) *LongFieldSource {
	return &LongFieldSource{
		FieldCacheSource: FieldCacheSource{Field: field},
	}
}

func (f *LongFieldSource) Description() string {
	return fmt.Sprintf("long(%s)", f.Field)
}

func (f *LongFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	ndv, err := readerContext.Reader().GetNumericDocValues(f.Field)
	if err != nil {
		return nil, err
	}

	fv := &longDocValues{
		source: f,
		ndv:    ndv,
	}
	fv.SetSelf(fv)
	return fv, nil
}

type longDocValues struct {
	function.BaseFunctionValues
	source   *LongFieldSource
	ndv      index.NumericDocValues
	lastDocID int
}

func (f *longDocValues) LongVal(doc int) (int64, error) {
	exists, err := f.Exists(doc)
	if err != nil {
		return 0, err
	}
	if exists {
		return f.ndv.LongValue()
	}
	return 0, nil
}

func (f *longDocValues) StrVal(doc int) (string, error) {
	val, err := f.LongVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", val), nil
}

func (f *longDocValues) Exists(doc int) (bool, error) {
	if doc < f.lastDocID {
		return false, fmt.Errorf("docs were sent out-of-order: lastDocID=%d vs docID=%d", f.lastDocID, doc)
	}
	f.lastDocID = doc
	curDocID := f.ndv.DocID()
	if doc > curDocID {
		next, err := f.ndv.Advance(doc)
		if err != nil {
			return false, err
		}
		curDocID = next
	}
	return doc == curDocID, nil
}

func (f *longDocValues) ObjectVal(doc int) (any, error) {
	exists, err := f.Exists(doc)
	if err != nil {
		return nil, err
	}
	if exists {
		return f.LongVal(doc)
	}
	return nil, nil
}

func (f *longDocValues) ToString(doc int) (string, error) {
	return fmt.Sprintf("long(%s)=%d", f.source.Field, f.LongVal(doc)), nil
}
