// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"bytes"
	"fmt"
	"math"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DocFreqValueSource returns the number of documents containing the term.
//
// Mirrors org.apache.lucene.queries.function.valuesource.DocFreqValueSource
// (@lucene.internal) of Apache Lucene 10.5.0.
type DocFreqValueSource struct {
	function.BaseValueSource
	field        string
	indexedField string
	val          string
	indexedBytes []byte
}

var _ function.ValueSource = (*DocFreqValueSource)(nil)

// NewDocFreqValueSource mirrors
// DocFreqValueSource(String, String, String, BytesRef).
func NewDocFreqValueSource(field, val, indexedField string, indexedBytes []byte) *DocFreqValueSource {
	return &DocFreqValueSource{
		field:        field,
		indexedField: indexedField,
		val:          val,
		indexedBytes: indexedBytes,
	}
}

// Name mirrors name().
func (v *DocFreqValueSource) Name() string { return "docfreq" }

// Description mirrors description().
func (v *DocFreqValueSource) Description() string {
	return v.Name() + "(" + v.field + "," + v.val + ")"
}

// GetValues mirrors getValues(Map, LeafReaderContext).
func (v *DocFreqValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	searcher, err := searcherFromContext(ctx)
	if err != nil {
		return nil, err
	}
	docfreq, err := index.DocFreq(searcher.GetIndexReader(), index.NewTermFromBytes(v.indexedField, v.indexedBytes))
	if err != nil {
		return nil, err
	}
	return newConstIntDocValues(int32(docfreq), v), nil
}

// CreateWeight mirrors createWeight(Map, IndexSearcher), which stashes the
// searcher in the context under the literal key "searcher".
func (v *DocFreqValueSource) CreateWeight(ctx function.Context, searcher any) error {
	ctx["searcher"] = searcher
	return nil
}

// HashCode mirrors
// getClass().hashCode() + indexedField.hashCode()*29 + indexedBytes.hashCode().
func (v *DocFreqValueSource) HashCode() int32 {
	return hashString("DocFreqValueSource") + hashString(v.indexedField)*29 + hashBytes(v.indexedBytes)
}

// Equals mirrors equals(Object).
func (v *DocFreqValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*DocFreqValueSource)
	if !ok {
		return false
	}
	return v.indexedField == o.indexedField && bytes.Equal(v.indexedBytes, o.indexedBytes)
}

// searcherFromContext renders the `(IndexSearcher) context.get("searcher")`
// cast that DocFreqValueSource, IDFValueSource and NormValueSource all perform.
func searcherFromContext(ctx function.Context) (*search.IndexSearcher, error) {
	if ctx == nil {
		return nil, fmt.Errorf("valuesource: no searcher in context")
	}
	searcher, ok := ctx["searcher"].(*search.IndexSearcher)
	if !ok || searcher == nil {
		return nil, fmt.Errorf("valuesource: no searcher in context")
	}
	return searcher, nil
}

// constIntDocValues renders the package-private static nested class
// DocFreqValueSource.ConstIntDocValues.
type constIntDocValues struct {
	*docvalues.IntDocValues
	sval   string
	parent function.ValueSource
}

func newConstIntDocValues(val int32, parent function.ValueSource) *constIntDocValues {
	c := &constIntDocValues{
		sval:   strconv.FormatInt(int64(val), 10),
		parent: parent,
	}
	c.IntDocValues = docvalues.NewIntDocValues(parent, func(int) (int32, error) { return val, nil })
	c.SetSelf(c)
	return c
}

// ToString mirrors toString(int): parent.description() + '=' + sval.
func (c *constIntDocValues) ToString(doc int) (string, error) {
	return c.parent.Description() + "=" + c.sval, nil
}

// constDoubleDocValues renders the package-private static nested class
// DocFreqValueSource.ConstDoubleDocValues.
type constDoubleDocValues struct {
	*docvalues.DoubleDocValues
	sval   string
	parent function.ValueSource
}

func newConstDoubleDocValues(val float64, parent function.ValueSource) *constDoubleDocValues {
	c := &constDoubleDocValues{
		sval:   formatJavaDouble(val),
		parent: parent,
	}
	c.DoubleDocValues = docvalues.NewDoubleDocValues(parent, func(int) (float64, error) { return val, nil })
	c.SetSelf(c)
	return c
}

// ToString mirrors toString(int): parent.description() + '=' + sval.
func (c *constDoubleDocValues) ToString(doc int) (string, error) {
	return c.parent.Description() + "=" + c.sval, nil
}

// formatJavaDouble renders a float64 the way java.lang.Double.toString does,
// which is what ConstDoubleDocValues stores in sval.
func formatJavaDouble(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 1) {
		return "Infinity"
	}
	if math.IsInf(v, -1) {
		return "-Infinity"
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e16 {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%v", v)
}
