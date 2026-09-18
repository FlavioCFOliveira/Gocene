// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SuggestFieldTYPE is the byte marker stamped on a plain suggest field.
// Mirrors org.apache.lucene.search.suggest.document.SuggestField.TYPE
// (SuggestField.java:64), which Java writes as SuggestField.TYPE at every use
// site. ContextSuggestField declares a constant of the same simple name, so
// each carries its owning class here — Go has no class-scoped constants.
const SuggestFieldTYPE byte = 0

// SuggestField indexes a string value and a weight as a weighted completion against a named suggester.
//
// This is the Go port of org.apache.lucene.search.suggest.document.SuggestField from Apache Lucene 10.5.0.
type SuggestField struct {
	*document.Field

	surfaceForm util.BytesRef
	weight      int
}

// FIELD_TYPE is the default field type for suggest fields.
var FIELD_TYPE = func() *document.FieldType {
	ft := document.NewFieldType()
	ft.SetTokenized(true)
	ft.SetStored(false)
	ft.SetStoreTermVectors(false)
	ft.SetOmitNorms(false)
	ft.SetIndexOptions(document.IndexOptionsDocsAndFreqsAndPositions)
	ft.Freeze()
	return ft
}()

// NewSuggestField creates a new SuggestField.
func NewSuggestField(name, value string, weight int) *SuggestField {
	if weight < 0 {
		panic("weight must be >= 0")
	}
	if len(value) == 0 {
		panic("value must have a length > 0")
	}
	for _, r := range value {
		if isReserved(r) {
			panic(fmt.Sprintf("Illegal input [%s] reserved character %q", value, r))
		}
	}

	f := &SuggestField{
		Field:       document.NewField(name, value, FIELD_TYPE),
		surfaceForm: util.NewBytesRef(value),
		weight:      weight,
	}
	return f
}

// TokenStream wraps the base token stream with a CompletionTokenStream and sets the payload.
func (f *SuggestField) TokenStream(analyzer analysis.Analyzer, reuse analysis.TokenStream) analysis.TokenStream {
	ts := f.wrapTokenStream(f.Field.TokenStream(analyzer, reuse))
	cts := ts.(*CompletionTokenStream)
	cts.SetPayload(f.buildSuggestPayload())
	return ts
}

func (f *SuggestField) wrapTokenStream(stream analysis.TokenStream) analysis.TokenStream {
	if cts, ok := stream.(*CompletionTokenStream); ok {
		return cts
	}
	return NewCompletionTokenStream(stream)
}

func (f *SuggestField) buildSuggestPayload() []byte {
	out := store.NewByteArrayDataOutput(len(f.surfaceForm.Bytes) + 10)
	_ = out.WriteVInt(int32(len(f.surfaceForm.Bytes)))
	_ = out.WriteBytes(f.surfaceForm.Bytes, 0, len(f.surfaceForm.Bytes))
	_ = out.WriteVInt(int32(f.weight + 1))
	_ = out.WriteByte(f.Type())
	return out.ToArrayCopy()
}

func isReserved(r rune) bool {
	return r == analysis.SepLabel || r == analysis.HOLE || r == 0 // SEP_LABEL, HOLE, END_BYTE
}

// Type returns the type of the field.
func (f *SuggestField) Type() byte {
	return SuggestFieldTYPE
}
