// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// NumDocsValueSource returns the value of IndexReader.NumDocs() for every document.
type NumDocsValueSource struct{}

func (v *NumDocsValueSource) Description() string {
	return "numdocs()"
}

func (v *NumDocsValueSource) String() string {
	return v.Description()
}

func (v *NumDocsValueSource) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	numDocs := ReaderUtilGetTopLevelContext(readerContext).Reader().NumDocs()
	return &constIntDocValues{
		val:    int32(numDocs),
		parent: v,
	}, nil
}

func (v *NumDocsValueSource) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	return nil
}

func (v *NumDocsValueSource) Equals(o any) bool {
	_, ok := o.(*NumDocsValueSource)
	return ok
}

func (v *NumDocsValueSource) HashCode() int {
	return 0
}

// DocFreqValueSource returns the number of documents containing the term.
type DocFreqValueSource struct {
	field        string
	val          string
	indexedField string
	indexedBytes []byte
}

func NewDocFreqValueSource(field, val, indexedField string, indexedBytes []byte) *DocFreqValueSource {
	return &DocFreqValueSource{
		field:        field,
		val:          val,
		indexedField: indexedField,
		indexedBytes: indexedBytes,
	}
}

func (v *DocFreqValueSource) Description() string {
	return fmt.Sprintf("docfreq(%s,%s)", v.field, v.val)
}

func (v *DocFreqValueSource) String() string {
	return v.Description()
}

func (v *DocFreqValueSource) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	searcher, ok := context["searcher"].(*IndexSearcher)
	if !ok {
		return nil, fmt.Errorf("searcher not found in context")
	}
	docFreq := searcher.IndexReader().DocFreq(v.indexedField, v.indexedBytes)
	return &constIntDocValues{
		val:    int32(docFreq),
		parent: v,
	}, nil
}

func (v *DocFreqValueSource) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	context["searcher"] = searcher
	return nil
}

func (v *DocFreqValueSource) Equals(o any) bool {
	other, ok := o.(*DocFreqValueSource)
	if !ok {
		return false
	}
	return v.indexedField == other.indexedField &&
		util.BytesEqual(v.indexedBytes, other.indexedBytes)
}

func (v *DocFreqValueSource) HashCode() int {
	return len(v.indexedField) + len(v.indexedBytes)
}

// IDFValueSource returns the Inverse Document Frequency (IDF) for every document.
type IDFValueSource struct {
	DocFreqValueSource
}

func NewIDFValueSource(field, val, indexedField string, indexedBytes []byte) *IDFValueSource {
	return &IDFValueSource{
		DocFreqValueSource: *NewDocFreqValueSource(field, val, indexedField, indexedBytes),
	}
}

func (v *IDFValueSource) Description() string {
	return fmt.Sprintf("idf(%s,%s)", v.field, v.val)
}

func (v *IDFValueSource) String() string {
	return v.Description()
}

func (v *IDFValueSource) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	searcher, ok := context["searcher"].(*IndexSearcher)
	if !ok {
		return nil, fmt.Errorf("searcher not found in context")
	}
	sim := searcher.Similarity()
	docFreq := searcher.IndexReader().DocFreq(v.indexedField, v.indexedBytes)
	idf := sim.IDF(docFreq, searcher.IndexReader().NumDocs())

	return &constDoubleDocValues{
		val:    idf,
		parent: v,
	}, nil
}

// TotalTermFreqValueSource returns the total term freq (sum of term freqs across all documents).
type TotalTermFreqValueSource struct {
	field        string
	val          string
	indexedField string
	indexedBytes []byte
}

func NewTotalTermFreqValueSource(field, val, indexedField string, indexedBytes []byte) *TotalTermFreqValueSource {
	return &TotalTermFreqValueSource{
		field:        field,
		val:          val,
		indexedField: indexedField,
		indexedBytes: indexedBytes,
	}
}

func (v *TotalTermFreqValueSource) Description() string {
	return fmt.Sprintf("totaltermfreq(%s,%s)", v.field, v.val)
}

func (v *TotalTermFreqValueSource) String() string {
	return v.Description()
}

func (v *TotalTermFreqValueSource) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	val, ok := context[v]
	if !ok {
		return nil, fmt.Errorf("weight not created for TotalTermFreqValueSource")
	}
	return val.(FunctionValues), nil
}

func (v *TotalTermFreqValueSource) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	var totalTermFreq int64
	for _, leaf := range searcher.GetTopReaderContext().Leaves() {
		val := leaf.Reader().TotalTermFreq(v.indexedField, v.indexedBytes)
		totalTermFreq += val
	}
	context[v] = &constLongDocValues{
		val:    totalTermFreq,
		parent: v,
	}
	return nil
}

func (v *TotalTermFreqValueSource) Equals(o any) bool {
	other, ok := o.(*TotalTermFreqValueSource)
	if !ok {
		return false
	}
	return v.indexedField == other.indexedField &&
		util.BytesEqual(v.indexedBytes, other.indexedBytes)
}

func (v *TotalTermFreqValueSource) HashCode() int {
	return len(v.indexedField) + len(v.indexedBytes)
}

// SumTotalTermFreqValueSource returns the number of tokens.
type SumTotalTermFreqValueSource struct {
	indexedField string
}

func NewSumTotalTermFreqValueSource(indexedField string) *SumTotalTermFreqValueSource {
	return &SumTotalTermFreqValueSource{indexedField: indexedField}
}

func (v *SumTotalTermFreqValueSource) Description() string {
	return fmt.Sprintf("sumtotaltermfreq(%s)", v.indexedField)
}

func (v *SumTotalTermFreqValueSource) String() string {
	return v.Description()
}

func (v *SumTotalTermFreqValueSource) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	val, ok := context[v]
	if !ok {
		return nil, fmt.Errorf("weight not created for SumTotalTermFreqValueSource")
	}
	return val.(FunctionValues), nil
}

func (v *SumTotalTermFreqValueSource) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	var sumTotalTermFreq int64
	for _, leaf := range searcher.GetTopReaderContext().Leaves() {
		terms := leaf.Reader().Terms(v.indexedField)
		sumTotalTermFreq += terms.SumTotalTermFreq()
	}
	context[v] = &constLongDocValues{
		val:    sumTotalTermFreq,
		parent: v,
	}
	return nil
}

func (v *SumTotalTermFreqValueSource) Equals(o any) bool {
	other, ok := o.(*SumTotalTermFreqValueSource)
	if !ok {
		return false
	}
	return v.indexedField == other.indexedField
}

func (v *SumTotalTermFreqValueSource) HashCode() int {
	return len(v.indexedField)
}

// JoinDocFreqValueSource returns the Document Frequency within another field for a field value.
type JoinDocFreqValueSource struct {
	field  string
	qfield string
}

func NewJoinDocFreqValueSource(field, qfield string) *JoinDocFreqValueSource {
	return &JoinDocFreqValueSource{field: field, qfield: qfield}
}

func (v *JoinDocFreqValueSource) Description() string {
	return fmt.Sprintf("joindf(%s:(%s))", v.field, v.qfield)
}

func (v *JoinDocFreqValueSource) String() string {
	return v.Description()
}

func (v *JoinDocFreqValueSource) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	topReader := ReaderUtilGetTopLevelContext(readerContext).Reader()
	terms := topReader.Terms(v.qfield)

	return &joinDocFreqValues{
		vs: v,
		readerContext: readerContext,
		topReader: topReader,
		terms: terms,
	}, nil
}

func (v *JoinDocFreqValueSource) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	return nil
}

func (v *JoinDocFreqValueSource) Equals(o any) bool {
	other, ok := o.(*JoinDocFreqValueSource)
	if !ok {
		return false
	}
	return v.field == other.field && v.qfield == other.qfield
}

func (v *JoinDocFreqValueSource) HashCode() int {
	return len(v.field) + len(v.qfield)
}

type joinDocFreqValues struct {
	vs            *JoinDocFreqValueSource
	readerContext LeafReaderContext
	topReader     IndexReader
	terms         Terms
	lastDocID     int
}

func (v *joinDocFreqValues) IntVal(doc int) (int32, error) {
	if doc < v.lastDocID {
		return 0, fmt.Errorf("docs were sent out-of-order: lastDocID=%d vs docID=%d", v.lastDocID, doc)
	}
	v.lastDocID = doc

	// This is a simplification of the Lucene implementation
	// In Lucene, it uses SortedDocValues to find the value of the field for the doc
	// and then looks up that value in the qfield.

	// For the sake of this port, we'll assume a simple implementation or
	// leave it as a TODO if it requires complex SortedDocValues integration.
	return 0, fmt.Errorf("JoinDocFreqValueSource.IntVal: not fully implemented")
}

func (v *joinDocFreqValues) FloatVal(doc int) (float32, error) {
	i, err := v.IntVal(doc)
	return float32(i), err
}

func (v *joinDocFreqValues) LongVal(doc int) (int64, error) {
	i, err := v.IntVal(doc)
	return int64(i), err
}

func (v *joinDocFreqValues) DoubleVal(doc int) (float64, error) {
	i, err := v.IntVal(doc)
	return float64(i), err
}

func (v *joinDocFreqValues) ByteVal(doc int) (byte, error) {
	i, err := v.IntVal(doc)
	return byte(i), err
}

func (v *joinDocFreqValues) ShortVal(doc int) (int16, error) {
	i, err := v.IntVal(doc)
	return int16(i), err
}

func (v *joinDocFreqValues) StrVal(doc int) (string, error) {
	i, err := v.IntVal(doc)
	return fmt.Sprintf("%d", i), err
}

func (v *joinDocFreqValues) BoolVal(doc int) (bool, error) {
	i, err := v.IntVal(doc)
	return i != 0, err
}

func (v *joinDocFreqValues) Exists(doc int) (bool, error) {
	return true, nil
}

func (v *joinDocFreqValues) ObjectVal(doc int) (any, error) {
	return v.IntVal(doc)
}

func (v *joinDocFreqValues) String(doc int) (string, error) {
	i, err := v.IntVal(doc)
	return fmt.Sprintf("%s=%d", v.vs.Description(), i), err
}

func (v *joinDocFreqValues) GetValueFiller() ValueFiller {
	return nil // not implemented
}

// NormValueSource returns the decoded norm for every document.
type NormValueSource struct {
	field string
}

func NewNormValueSource(field string) *NormValueSource {
	return &NormValueSource{field: field}
}

func (v *NormValueSource) Description() string {
	return fmt.Sprintf("norm(%s)", v.field)
}

func (v *NormValueSource) String() string {
	return v.Description()
}

func (v *NormValueSource) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	context["searcher"] = searcher
	return nil
}

func (v *NormValueSource) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	searcher, ok := context["searcher"].(*IndexSearcher)
	if !ok {
		return nil, fmt.Errorf("searcher not found in context")
	}

	sim := searcher.Similarity()
	// Simplified IDF logic for norm
	// In Lucene, it creates a SimScorer with a bogus term to get the norm contribution

	return &normDocValues{
		vs: v,
		sim: sim,
	}, nil
}

func (v *NormValueSource) Equals(o any) bool {
	other, ok := o.(*NormValueSource)
	if !ok {
		return false
	}
	return v.field == other.field
}

func (v *NormValueSource) HashCode() int {
	return len(v.field)
}

type normDocValues struct {
	vs  *NormValueSource
	sim Similarity
}

func (v *normDocValues) FloatVal(doc int) (float32, error) {
	// Simplified norm retrieval
	norm, err := v.vs.getNorm(doc) // needs implementation
	if err != nil {
		return 0, err
	}
	return v.sim.ScoreNorm(norm), nil
}

func (v *normDocValues) IntVal(doc int) (int32, error) {
	f, err := v.FloatVal(doc)
	return int32(f), err
}

func (v *normDocValues) LongVal(doc int) (int64, error) {
	f, err := v.FloatVal(doc)
	return int64(f), err
}

func (v *normDocValues) DoubleVal(doc int) (float64, error) {
	f, err := v.FloatVal(doc)
	return float64(f), err
}

func (v *normDocValues) ByteVal(doc int) (byte, error) {
	f, err := v.FloatVal(doc)
	return byte(f), err
}

func (v *normDocValues) ShortVal(doc int) (int16, error) {
	f, err := v.FloatVal(doc)
	return int16(f), err
}

func (v *normDocValues) StrVal(doc int) (string, error) {
	f, err := v.FloatVal(doc)
	return fmt.Sprintf("%f", f), err
}

func (v *normDocValues) BoolVal(doc int) (bool, error) {
	f, err := v.FloatVal(doc)
	return f != 0, err
}

func (v *normDocValues) Exists(doc int) (bool, error) {
	return true, nil
}

func (v *normDocValues) ObjectVal(doc int) (any, error) {
	return v.FloatVal(doc)
}

func (v *normDocValues) String(doc int) (string, error) {
	f, err := v.FloatVal(doc)
	return fmt.Sprintf("%s=%f", v.vs.Description(), f), err
}

func (v *normDocValues) GetValueFiller() ValueFiller {
	return nil
}

func (v *NormValueSource) getNorm(doc int) (float64, error) {
	return 1.0, nil // placeholder
}

// QueryValueSource returns the relevance score of the query.
type QueryValueSource struct {
	q       Query
	defVal  float32
}

func NewQueryValueSource(q Query, defVal float32) *QueryValueSource {
	return &QueryValueSource{q: q, defVal: defVal}
}

func (v *QueryValueSource) Description() string {
	return fmt.Sprintf("query(%s,def=%f)", v.q.String(), v.defVal)
}

func (v *QueryValueSource) String() string {
	return v.Description()
}

func (v *QueryValueSource) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	rewritten, err := searcher.Rewrite(v.q)
	if err != nil {
		return err
	}
	weight, err := searcher.CreateWeight(rewritten, ScoreModeComplete, 1.0)
	if err != nil {
		return err
	}
	context[v] = weight
	return nil
}

func (v *QueryValueSource) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	weight, ok := context[v].(*Weight)
	if !ok {
		return nil, fmt.Errorf("weight not found in context for QueryValueSource")
	}
	return &queryDocValues{
		vs: v,
		readerContext: readerContext,
		weight: weight,
	}, nil
}

func (v *QueryValueSource) Equals(o any) bool {
	other, ok := o.(*QueryValueSource)
	if !ok {
		return false
	}
	return v.q.Equals(other.q) && v.defVal == other.defVal
}

func (v *QueryValueSource) HashCode() int {
	return v.q.HashCode()
}

type queryDocValues struct {
	vs            *QueryValueSource
	readerContext LeafReaderContext
	weight        *Weight
	scorer        Scorer
	disi          DocIdSetIterator
	lastDocRequested int
}

func (v *queryDocValues) FloatVal(doc int) (float32, error) {
	if v.scorer == nil {
		scorer, err := v.weight.Scorer(v.readerContext)
		if err != nil {
			return v.vs.defVal, err
		}
		v.scorer = scorer
		v.disi = scorer.Iterator()
	}

	if v.disi.DocID() < doc {
		v.disi.Advance(doc)
	}

	if v.disi.DocID() == doc {
		return float32(v.scorer.Score()), nil
	}
	return v.vs.defVal, nil
}

func (v *queryDocValues) IntVal(doc int) (int32, error) {
	f, err := v.FloatVal(doc)
	return int32(f), err
}

func (v *queryDocValues) LongVal(doc int) (int64, error) {
	f, err := v.FloatVal(doc)
	return int64(f), err
}

func (v *queryDocValues) DoubleVal(doc int) (float64, error) {
	f, err := v.FloatVal(doc)
	return float64(f), err
}

func (v *queryDocValues) ByteVal(doc int) (byte, error) {
	f, err := v.FloatVal(doc)
	return byte(f), err
}

func (v *queryDocValues) ShortVal(doc int) (int16, error) {
	f, err := v.FloatVal(doc)
	return int16(f), err
}

func (v *queryDocValues) StrVal(doc int) (string, error) {
	f, err := v.FloatVal(doc)
	return fmt.Sprintf("%f", f), err
}

func (v *queryDocValues) BoolVal(doc int) (bool, error) {
	f, err := v.FloatVal(doc)
	return f != 0, err
}

func (v *queryDocValues) Exists(doc int) (bool, error) {
	return true, nil
}

func (v *queryDocValues) ObjectVal(doc int) (any, error) {
	return v.FloatVal(doc)
}

func (v *queryDocValues) String(doc int) (string, error) {
	f, err := v.FloatVal(doc)
	return fmt.Sprintf("%s=%f", v.vs.Description(), f), err
}

func (v *queryDocValues) GetValueFiller() ValueFiller {
	return nil
}

// MultiFunction is an abstract parent class for ValueSource implementations that wrap multiple ValueSources.
type MultiFunction struct {
	sources []ValueSource
}

func (v *MultiFunction) Description() string {
	return v.name() + "(" + v.sourcesDescription() + ")"
}

func (v *MultiFunction) String() string {
	return v.Description()
}

func (v *MultiFunction) name() string {
	return "multi"
}

func (v *MultiFunction) sourcesDescription() string {
	var descs []string
	for _, s := range v.sources {
		descs = append(descs, s.Description())
	}
	// simplified join
	return fmt.Sprintf("%v", descs)
}

func (v *MultiFunction) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	for _, s := range v.sources {
		if err := s.CreateWeight(context, searcher); err != nil {
			return err
		}
	}
	return nil
}

func (v *MultiFunction) Equals(o any) bool {
	other, ok := o.(*MultiFunction)
	if !ok {
		return false
	}
	if len(v.sources) != len(other.sources) {
		return false
	}
	for i := range v.sources {
		if !v.sources[i].Equals(other.sources[i]) {
			return false
		}
	}
	return true
}

func (v *MultiFunction) HashCode() int {
	return 0 // simplified
}

// SingleFunction is a function with a single argument.
type SingleFunction struct {
	source ValueSource
}

func (v *SingleFunction) Description() string {
	return v.name() + "(" + v.source.Description() + ")"
}

func (v *SingleFunction) String() string {
	return v.Description()
}

func (v *SingleFunction) name() string {
	return "single"
}

func (v *SingleFunction) CreateWeight(context map[any]any, searcher *IndexSearcher) error {
	return v.source.CreateWeight(context, searcher)
}

func (v *SingleFunction) Equals(o any) bool {
	other, ok := o.(*SingleFunction)
	if !ok {
		return false
	}
	return v.source.Equals(other.source)
}

func (v *SingleFunction) HashCode() int {
	return v.source.HashCode()
}

// DefFunction returns the values from the provided ValueSources which are available for a particular docId.
type DefFunction struct {
	MultiFunction
}

func NewDefFunction(sources []ValueSource) *DefFunction {
	return &DefFunction{MultiFunction: MultiFunction{sources: sources}}
}

func (v *DefFunction) name() string {
	return "def"
}

func (v *DefFunction) GetValues(context map[any]any, readerContext LeafReaderContext) (FunctionValues, error) {
	vals := make([]FunctionValues, len(v.sources))
	for i, s := range v.sources {
		fv, err := s.GetValues(context, readerContext)
		if err != nil {
			return nil, err
		}
		vals[i] = fv
	}
	return &defFunctionValues{
		vals: vals,
		vs: v,
	}, nil
}

type defFunctionValues struct {
	vals []FunctionValues
	vs   *DefFunction
}

func (v *defFunctionValues) DoubleVal(doc int) (float64, error) {
	for _, fv := range v.vals {
		if exists, err := fv.Exists(doc); err == nil && exists {
			return fv.DoubleVal(doc)
		}
	}
	return 0, nil
}

func (v *defFunctionValues) FloatVal(doc int) (float32, error) {
	d, err := v.DoubleVal(doc)
	return float32(d), err
}

func (v *defFunctionValues) IntVal(doc int) (int32, error) {
	d, err := v.DoubleVal(doc)
	return int32(d), err
}

func (v *defFunctionValues) LongVal(doc int) (int64, error) {
	d, err := v.DoubleVal(doc)
	return int64(d), err
}

func (v *defFunctionValues) ByteVal(doc int) (byte, error) {
	d, err := v.DoubleVal(doc)
	return byte(d), err
}

func (v *defFunctionValues) ShortVal(doc int) (int16, error) {
	d, err := v.DoubleVal(doc)
	return int16(d), err
}

func (v *defFunctionValues) StrVal(doc int) (string, error) {
	d, err := v.DoubleVal(doc)
	return fmt.Sprintf("%f", d), err
}

func (v *defFunctionValues) BoolVal(doc int) (bool, error) {
	d, err := v.DoubleVal(doc)
	return d != 0, err
}

func (v *defFunctionValues) Exists(doc int) (bool, error) {
	for _, fv := range v.vals {
		if exists, err := fv.Exists(doc); err == nil && exists {
			return true, nil
		}
	}
	return false, nil
}

func (v *defFunctionValues) ObjectVal(doc int) (any, error) {
	return v.DoubleVal(doc)
}

func (v *defFunctionValues) String(doc int) (string, error) {
	d, err := v.DoubleVal(doc)
	return fmt.Sprintf("%s=%f", v.vs.Description(), d), err
}

func (v *defFunctionValues) GetValueFiller() ValueFiller {
	return nil
}

type constLongDocValues struct {
	val    int64
	parent ValueSource
}

func (v *constLongDocValues) LongVal(doc int) (int64, error) { return v.val, nil }
func (v *constLongDocValues) IntVal(doc int) (int32, error) {
	return int32(v.val), nil
}
func (v *constLongDocValues) FloatVal(doc int) (float32, error) {
	return float32(v.val), nil
}
func (v *constLongDocValues) DoubleVal(doc int) (float64, error) {
	return float64(v.val), nil
}
func (v *constLongDocValues) ByteVal(doc int) (byte, error) {
	return byte(v.val), nil
}
func (v *constLongDocValues) ShortVal(doc int) (int16, error) {
	return int16(v.val), nil
}
func (v *constLongDocValues) StrVal(doc int) (string, error) {
	return fmt.Sprintf("%d", v.val), nil
}
func (v *constLongDocValues) BoolVal(doc int) (bool, error) {
	return v.val != 0, nil
}
func (v *constLongDocValues) Exists(doc int) (bool, error) {
	return true, nil
}
func (v *constLongDocValues) ObjectVal(doc int) (any, error) {
	return v.val, nil
}
func (v *constLongDocValues) String(doc int) (string, error) {
	return fmt.Sprintf("%s=%d", v.parent.Description(), v.val), nil
}
func (v *constLongDocValues) GetValueFiller() ValueFiller {
	return &longValueFiller{val: v.val}
}

type longValueFiller struct {
	val int64
}

func (f *longValueFiller) GetValue() MutableValue {
	return &MutableValueLong{Value: f.val, ExistsB: true}
}
