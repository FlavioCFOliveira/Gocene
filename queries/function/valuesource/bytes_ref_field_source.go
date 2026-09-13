// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// BytesRefFieldSource is an implementation for retrieving [function.FunctionValues]
// instances for string based fields.
type BytesRefFieldSource struct {
	FieldCacheSource
}

func NewBytesRefFieldSource(field string) *BytesRefFieldSource {
	return &BytesRefFieldSource{
		FieldCacheSource: FieldCacheSource{Field: field},
	}
}

func (f *BytesRefFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	fieldInfo := readerContext.LeafReader().GetFieldInfos().GetByName(f.Field)

	// To be sorted or not to be sorted, that is the question
	if fieldInfo != nil && fieldInfo.DocValuesType() == index.DocValuesTypeBinary {
		ndv, err := readerContext.LeafReader().GetBinaryDocValues(f.Field)
		if err != nil {
			return nil, err
		}

		fv := &bytesRefDocValues{
			ndv: ndv,
		}
		fv.SetSelf(fv)
		return fv, nil
	}

	// Fallback to SortedDocValues via DocTermsIndexDocValues
	dv, err := docvalues.NewDocTermsIndexDocValues(f, readerContext, f.Field)
	if err != nil {
		return nil, err
	}
	// Override some methods to match Java's anonymous class implementation
	return &bytesRefFallback{
		DocTermsIndexDocValues: dv,
	}, nil
}

type bytesRefDocValues struct {
	function.BaseFunctionValues
	ndv       index.BinaryDocValues
	lastDocID int
}

func (f *bytesRefDocValues) Exists(doc int) (bool, error) {
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

func (f *bytesRefDocValues) BytesVal(doc int, target *[]byte) (bool, error) {
	exists, err := f.Exists(doc)
	if err != nil {
		return false, err
	}
	if !exists {
		if target != nil {
			*target = (*target)[:0]
		}
		return false, nil
	}

	value, err := f.ndv.BinaryValue()
	if err != nil {
		return false, err
	}
	if len(value) == 0 {
		if target != nil {
			*target = (*target)[:0]
		}
		return false, nil
	}

	if target != nil {
		*target = append((*target)[:0], value...)
	}
	return true, nil
}

func (f *bytesRefDocValues) StrVal(doc int) (string, error) {
	var buf []byte
	exists, err := f.BytesVal(doc, &buf)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", nil
	}
	return string(buf), nil
}

func (f *bytesRefDocValues) ObjectVal(doc int) (any, error) {
	return f.StrVal(doc)
}

func (f *bytesRefDocValues) ToString(doc int) (string, error) {
	s, err := f.StrVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("bytesref(%s)=%s", f.Field, s), nil
}

type bytesRefFallback struct {
	*docvalues.DocTermsIndexDocValues
}

func (f *bytesRefFallback) ObjectVal(doc int) (any, error) {
	return f.StrVal(doc)
}

func (f *bytesRefFallback) ToString(doc int) (string, error) {
	s, err := f.StrVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("bytesref(%s)=%s", f.Field, s), nil
}
