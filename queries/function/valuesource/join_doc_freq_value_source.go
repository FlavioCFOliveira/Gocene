// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// JoinDocFreqValueSourceName mirrors the public constant
// JoinDocFreqValueSource.NAME.
const JoinDocFreqValueSourceName = "joindf"

// JoinDocFreqValueSource uses a field value and finds the Document Frequency
// within another field.
//
// Mirrors
// org.apache.lucene.queries.function.valuesource.JoinDocFreqValueSource.
type JoinDocFreqValueSource struct {
	FieldCacheSource
	qfield string
}

var _ function.ValueSource = (*JoinDocFreqValueSource)(nil)

// NewJoinDocFreqValueSource mirrors JoinDocFreqValueSource(String, String).
func NewJoinDocFreqValueSource(field, qfield string) *JoinDocFreqValueSource {
	return &JoinDocFreqValueSource{
		FieldCacheSource: FieldCacheSource{Field: field},
		qfield:           qfield,
	}
}

// Description mirrors description().
func (v *JoinDocFreqValueSource) Description() string {
	return JoinDocFreqValueSourceName + "(" + v.Field + ":(" + v.qfield + "))"
}

// GetValues mirrors getValues(Map, LeafReaderContext).
func (v *JoinDocFreqValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	terms, err := index.GetSorted(readerContext.LeafReader(), v.Field)
	if err != nil {
		return nil, err
	}
	top := index.ReaderUtilGetTopLevelContext(readerContext).Reader()
	t, err := index.MultiTermsGetTerms(top, v.qfield)
	if err != nil {
		return nil, err
	}
	var termsEnum index.TermsEnum
	if t == nil {
		termsEnum = &index.EmptyTermsEnum{}
	} else {
		termsEnum, err = t.GetIterator()
		if err != nil {
			return nil, err
		}
	}

	lastDocID := -1
	fv := docvalues.NewIntDocValues(v, func(doc int) (int32, error) {
		if doc < lastDocID {
			return 0, fmt.Errorf("docs were sent out-of-order: lastDocID=%d vs docID=%d", lastDocID, doc)
		}
		lastDocID = doc
		curDocID := terms.DocID()
		if doc > curDocID {
			curDocID, err = terms.Advance(doc)
			if err != nil {
				return 0, err
			}
		}
		if doc == curDocID {
			ord, err := terms.OrdValue()
			if err != nil {
				return 0, err
			}
			term, err := terms.LookupOrd(ord)
			if err != nil {
				return 0, err
			}
			found, err := termsEnum.SeekExact(index.NewTermFromBytes(v.qfield, term))
			if err != nil {
				return 0, err
			}
			if found {
				df, err := termsEnum.DocFreq()
				if err != nil {
					return 0, err
				}
				return int32(df), nil
			}
		}
		return 0, nil
	})
	return fv, nil
}

// Equals mirrors equals(Object).
func (v *JoinDocFreqValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*JoinDocFreqValueSource)
	if !ok {
		return false
	}
	if v.qfield != o.qfield {
		return false
	}
	return v.FieldCacheSource.Equals(o)
}

// HashCode mirrors qfield.hashCode() + super.hashCode().
func (v *JoinDocFreqValueSource) HashCode() int32 {
	return hashString(v.qfield) + v.FieldCacheSource.HashCode()
}
