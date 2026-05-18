// Package valuesource hosts the Sprint 29 overflow ports for
// org.apache.lucene.queries.function.valuesource.
package valuesource

// The Sprint 29 queries-module overflow surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// MultiValuedDoubleFieldSource mirrors org.apache.lucene.queries.function.valuesource.MultiValuedDoubleFieldSource.
type MultiValuedDoubleFieldSource struct{}

// NewMultiValuedDoubleFieldSource builds a MultiValuedDoubleFieldSource.
func NewMultiValuedDoubleFieldSource() *MultiValuedDoubleFieldSource { return &MultiValuedDoubleFieldSource{} }

// MultiValuedFloatFieldSource mirrors org.apache.lucene.queries.function.valuesource.MultiValuedFloatFieldSource.
type MultiValuedFloatFieldSource struct{}

// NewMultiValuedFloatFieldSource builds a MultiValuedFloatFieldSource.
func NewMultiValuedFloatFieldSource() *MultiValuedFloatFieldSource { return &MultiValuedFloatFieldSource{} }

// MultiValuedIntFieldSource mirrors org.apache.lucene.queries.function.valuesource.MultiValuedIntFieldSource.
type MultiValuedIntFieldSource struct{}

// NewMultiValuedIntFieldSource builds a MultiValuedIntFieldSource.
func NewMultiValuedIntFieldSource() *MultiValuedIntFieldSource { return &MultiValuedIntFieldSource{} }

// MultiValuedLongFieldSource mirrors org.apache.lucene.queries.function.valuesource.MultiValuedLongFieldSource.
type MultiValuedLongFieldSource struct{}

// NewMultiValuedLongFieldSource builds a MultiValuedLongFieldSource.
func NewMultiValuedLongFieldSource() *MultiValuedLongFieldSource { return &MultiValuedLongFieldSource{} }

// IDFValueSource mirrors org.apache.lucene.queries.function.valuesource.IDFValueSource.
type IDFValueSource struct{}

// NewIDFValueSource builds a IDFValueSource.
func NewIDFValueSource() *IDFValueSource { return &IDFValueSource{} }

// BoolFunction mirrors org.apache.lucene.queries.function.valuesource.BoolFunction.
type BoolFunction struct{}

// NewBoolFunction builds a BoolFunction.
func NewBoolFunction() *BoolFunction { return &BoolFunction{} }

// ByteKnnVectorFieldSource mirrors org.apache.lucene.queries.function.valuesource.ByteKnnVectorFieldSource.
type ByteKnnVectorFieldSource struct{}

// NewByteKnnVectorFieldSource builds a ByteKnnVectorFieldSource.
func NewByteKnnVectorFieldSource() *ByteKnnVectorFieldSource { return &ByteKnnVectorFieldSource{} }

// ByteVectorSimilarityFunction mirrors org.apache.lucene.queries.function.valuesource.ByteVectorSimilarityFunction.
type ByteVectorSimilarityFunction struct{}

// NewByteVectorSimilarityFunction builds a ByteVectorSimilarityFunction.
func NewByteVectorSimilarityFunction() *ByteVectorSimilarityFunction { return &ByteVectorSimilarityFunction{} }

// ConstKnnByteVectorValueSource mirrors org.apache.lucene.queries.function.valuesource.ConstKnnByteVectorValueSource.
type ConstKnnByteVectorValueSource struct{}

// NewConstKnnByteVectorValueSource builds a ConstKnnByteVectorValueSource.
func NewConstKnnByteVectorValueSource() *ConstKnnByteVectorValueSource { return &ConstKnnByteVectorValueSource{} }

// ConstKnnFloatValueSource mirrors org.apache.lucene.queries.function.valuesource.ConstKnnFloatValueSource.
type ConstKnnFloatValueSource struct{}

// NewConstKnnFloatValueSource builds a ConstKnnFloatValueSource.
func NewConstKnnFloatValueSource() *ConstKnnFloatValueSource { return &ConstKnnFloatValueSource{} }

// DefFunction mirrors org.apache.lucene.queries.function.valuesource.DefFunction.
type DefFunction struct{}

// NewDefFunction builds a DefFunction.
func NewDefFunction() *DefFunction { return &DefFunction{} }

// DivFloatFunction mirrors org.apache.lucene.queries.function.valuesource.DivFloatFunction.
type DivFloatFunction struct{}

// NewDivFloatFunction builds a DivFloatFunction.
func NewDivFloatFunction() *DivFloatFunction { return &DivFloatFunction{} }

// FieldCacheSource mirrors org.apache.lucene.queries.function.valuesource.FieldCacheSource.
type FieldCacheSource struct{}

// NewFieldCacheSource builds a FieldCacheSource.
func NewFieldCacheSource() *FieldCacheSource { return &FieldCacheSource{} }

// FloatKnnVectorFieldSource mirrors org.apache.lucene.queries.function.valuesource.FloatKnnVectorFieldSource.
type FloatKnnVectorFieldSource struct{}

// NewFloatKnnVectorFieldSource builds a FloatKnnVectorFieldSource.
func NewFloatKnnVectorFieldSource() *FloatKnnVectorFieldSource { return &FloatKnnVectorFieldSource{} }

// FloatVectorSimilarityFunction mirrors org.apache.lucene.queries.function.valuesource.FloatVectorSimilarityFunction.
type FloatVectorSimilarityFunction struct{}

// NewFloatVectorSimilarityFunction builds a FloatVectorSimilarityFunction.
func NewFloatVectorSimilarityFunction() *FloatVectorSimilarityFunction { return &FloatVectorSimilarityFunction{} }

// IfFunction mirrors org.apache.lucene.queries.function.valuesource.IfFunction.
type IfFunction struct{}

// NewIfFunction builds a IfFunction.
func NewIfFunction() *IfFunction { return &IfFunction{} }

// MaxDocValueSource mirrors org.apache.lucene.queries.function.valuesource.MaxDocValueSource.
type MaxDocValueSource struct{}

// NewMaxDocValueSource builds a MaxDocValueSource.
func NewMaxDocValueSource() *MaxDocValueSource { return &MaxDocValueSource{} }

// MaxFloatFunction mirrors org.apache.lucene.queries.function.valuesource.MaxFloatFunction.
type MaxFloatFunction struct{}

// NewMaxFloatFunction builds a MaxFloatFunction.
func NewMaxFloatFunction() *MaxFloatFunction { return &MaxFloatFunction{} }

// MinFloatFunction mirrors org.apache.lucene.queries.function.valuesource.MinFloatFunction.
type MinFloatFunction struct{}

// NewMinFloatFunction builds a MinFloatFunction.
func NewMinFloatFunction() *MinFloatFunction { return &MinFloatFunction{} }

// MultiFunction mirrors org.apache.lucene.queries.function.valuesource.MultiFunction.
type MultiFunction struct{}

// NewMultiFunction builds a MultiFunction.
func NewMultiFunction() *MultiFunction { return &MultiFunction{} }

// MultiValueSource mirrors org.apache.lucene.queries.function.valuesource.MultiValueSource.
type MultiValueSource struct{}

// NewMultiValueSource builds a MultiValueSource.
func NewMultiValueSource() *MultiValueSource { return &MultiValueSource{} }

// NumDocsValueSource mirrors org.apache.lucene.queries.function.valuesource.NumDocsValueSource.
type NumDocsValueSource struct{}

// NewNumDocsValueSource builds a NumDocsValueSource.
func NewNumDocsValueSource() *NumDocsValueSource { return &NumDocsValueSource{} }

// PowFloatFunction mirrors org.apache.lucene.queries.function.valuesource.PowFloatFunction.
type PowFloatFunction struct{}

// NewPowFloatFunction builds a PowFloatFunction.
func NewPowFloatFunction() *PowFloatFunction { return &PowFloatFunction{} }

// ProductFloatFunction mirrors org.apache.lucene.queries.function.valuesource.ProductFloatFunction.
type ProductFloatFunction struct{}

// NewProductFloatFunction builds a ProductFloatFunction.
func NewProductFloatFunction() *ProductFloatFunction { return &ProductFloatFunction{} }

// SingleFunction mirrors org.apache.lucene.queries.function.valuesource.SingleFunction.
type SingleFunction struct{}

// NewSingleFunction builds a SingleFunction.
func NewSingleFunction() *SingleFunction { return &SingleFunction{} }

// SumFloatFunction mirrors org.apache.lucene.queries.function.valuesource.SumFloatFunction.
type SumFloatFunction struct{}

// NewSumFloatFunction builds a SumFloatFunction.
func NewSumFloatFunction() *SumFloatFunction { return &SumFloatFunction{} }

// VectorFieldFunction mirrors org.apache.lucene.queries.function.valuesource.VectorFieldFunction.
type VectorFieldFunction struct{}

// NewVectorFieldFunction builds a VectorFieldFunction.
func NewVectorFieldFunction() *VectorFieldFunction { return &VectorFieldFunction{} }

// VectorSimilarityFunction mirrors org.apache.lucene.queries.function.valuesource.VectorSimilarityFunction.
type VectorSimilarityFunction struct{}

// NewVectorSimilarityFunction builds a VectorSimilarityFunction.
func NewVectorSimilarityFunction() *VectorSimilarityFunction { return &VectorSimilarityFunction{} }

// VectorValueSource mirrors org.apache.lucene.queries.function.valuesource.VectorValueSource.
type VectorValueSource struct{}

// NewVectorValueSource builds a VectorValueSource.
func NewVectorValueSource() *VectorValueSource { return &VectorValueSource{} }

// ComparisonBoolFunction mirrors org.apache.lucene.queries.function.valuesource.ComparisonBoolFunction.
type ComparisonBoolFunction struct{}

// NewComparisonBoolFunction builds a ComparisonBoolFunction.
func NewComparisonBoolFunction() *ComparisonBoolFunction { return &ComparisonBoolFunction{} }

// MultiBoolFunction mirrors org.apache.lucene.queries.function.valuesource.MultiBoolFunction.
type MultiBoolFunction struct{}

// NewMultiBoolFunction builds a MultiBoolFunction.
func NewMultiBoolFunction() *MultiBoolFunction { return &MultiBoolFunction{} }

// SimpleBoolFunction mirrors org.apache.lucene.queries.function.valuesource.SimpleBoolFunction.
type SimpleBoolFunction struct{}

// NewSimpleBoolFunction builds a SimpleBoolFunction.
func NewSimpleBoolFunction() *SimpleBoolFunction { return &SimpleBoolFunction{} }

// DualFloatFunction mirrors org.apache.lucene.queries.function.valuesource.DualFloatFunction.
type DualFloatFunction struct{}

// NewDualFloatFunction builds a DualFloatFunction.
func NewDualFloatFunction() *DualFloatFunction { return &DualFloatFunction{} }

// FloatFieldSource mirrors org.apache.lucene.queries.function.valuesource.FloatFieldSource.
type FloatFieldSource struct{}

// NewFloatFieldSource builds a FloatFieldSource.
func NewFloatFieldSource() *FloatFieldSource { return &FloatFieldSource{} }

// LinearFloatFunction mirrors org.apache.lucene.queries.function.valuesource.LinearFloatFunction.
type LinearFloatFunction struct{}

// NewLinearFloatFunction builds a LinearFloatFunction.
func NewLinearFloatFunction() *LinearFloatFunction { return &LinearFloatFunction{} }

// MultiFloatFunction mirrors org.apache.lucene.queries.function.valuesource.MultiFloatFunction.
type MultiFloatFunction struct{}

// NewMultiFloatFunction builds a MultiFloatFunction.
func NewMultiFloatFunction() *MultiFloatFunction { return &MultiFloatFunction{} }

// NormValueSource mirrors org.apache.lucene.queries.function.valuesource.NormValueSource.
type NormValueSource struct{}

// NewNormValueSource builds a NormValueSource.
func NewNormValueSource() *NormValueSource { return &NormValueSource{} }

// QueryValueSource mirrors org.apache.lucene.queries.function.valuesource.QueryValueSource.
type QueryValueSource struct{}

// NewQueryValueSource builds a QueryValueSource.
func NewQueryValueSource() *QueryValueSource { return &QueryValueSource{} }

// RangeMapFloatFunction mirrors org.apache.lucene.queries.function.valuesource.RangeMapFloatFunction.
type RangeMapFloatFunction struct{}

// NewRangeMapFloatFunction builds a RangeMapFloatFunction.
func NewRangeMapFloatFunction() *RangeMapFloatFunction { return &RangeMapFloatFunction{} }

// ReciprocalFloatFunction mirrors org.apache.lucene.queries.function.valuesource.ReciprocalFloatFunction.
type ReciprocalFloatFunction struct{}

// NewReciprocalFloatFunction builds a ReciprocalFloatFunction.
func NewReciprocalFloatFunction() *ReciprocalFloatFunction { return &ReciprocalFloatFunction{} }

// ScaleFloatFunction mirrors org.apache.lucene.queries.function.valuesource.ScaleFloatFunction.
type ScaleFloatFunction struct{}

// NewScaleFloatFunction builds a ScaleFloatFunction.
func NewScaleFloatFunction() *ScaleFloatFunction { return &ScaleFloatFunction{} }

// SimpleFloatFunction mirrors org.apache.lucene.queries.function.valuesource.SimpleFloatFunction.
type SimpleFloatFunction struct{}

// NewSimpleFloatFunction builds a SimpleFloatFunction.
func NewSimpleFloatFunction() *SimpleFloatFunction { return &SimpleFloatFunction{} }

// TFValueSource mirrors org.apache.lucene.queries.function.valuesource.TFValueSource.
type TFValueSource struct{}

// NewTFValueSource builds a TFValueSource.
func NewTFValueSource() *TFValueSource { return &TFValueSource{} }

// BytesRefFieldSource mirrors org.apache.lucene.queries.function.valuesource.BytesRefFieldSource.
type BytesRefFieldSource struct{}

// NewBytesRefFieldSource builds a BytesRefFieldSource.
func NewBytesRefFieldSource() *BytesRefFieldSource { return &BytesRefFieldSource{} }

// SortedSetFieldSource mirrors org.apache.lucene.queries.function.valuesource.SortedSetFieldSource.
type SortedSetFieldSource struct{}

// NewSortedSetFieldSource builds a SortedSetFieldSource.
func NewSortedSetFieldSource() *SortedSetFieldSource { return &SortedSetFieldSource{} }

// DoubleFieldSource mirrors org.apache.lucene.queries.function.valuesource.DoubleFieldSource.
type DoubleFieldSource struct{}

// NewDoubleFieldSource builds a DoubleFieldSource.
func NewDoubleFieldSource() *DoubleFieldSource { return &DoubleFieldSource{} }

// DocFreqValueSource mirrors org.apache.lucene.queries.function.valuesource.DocFreqValueSource.
type DocFreqValueSource struct{}

// NewDocFreqValueSource builds a DocFreqValueSource.
func NewDocFreqValueSource() *DocFreqValueSource { return &DocFreqValueSource{} }

// EnumFieldSource mirrors org.apache.lucene.queries.function.valuesource.EnumFieldSource.
type EnumFieldSource struct{}

// NewEnumFieldSource builds a EnumFieldSource.
func NewEnumFieldSource() *EnumFieldSource { return &EnumFieldSource{} }

// IntFieldSource mirrors org.apache.lucene.queries.function.valuesource.IntFieldSource.
type IntFieldSource struct{}

// NewIntFieldSource builds a IntFieldSource.
func NewIntFieldSource() *IntFieldSource { return &IntFieldSource{} }

// JoinDocFreqValueSource mirrors org.apache.lucene.queries.function.valuesource.JoinDocFreqValueSource.
type JoinDocFreqValueSource struct{}

// NewJoinDocFreqValueSource builds a JoinDocFreqValueSource.
func NewJoinDocFreqValueSource() *JoinDocFreqValueSource { return &JoinDocFreqValueSource{} }

// TermFreqValueSource mirrors org.apache.lucene.queries.function.valuesource.TermFreqValueSource.
type TermFreqValueSource struct{}

// NewTermFreqValueSource builds a TermFreqValueSource.
func NewTermFreqValueSource() *TermFreqValueSource { return &TermFreqValueSource{} }

// LongFieldSource mirrors org.apache.lucene.queries.function.valuesource.LongFieldSource.
type LongFieldSource struct{}

// NewLongFieldSource builds a LongFieldSource.
func NewLongFieldSource() *LongFieldSource { return &LongFieldSource{} }

// SumTotalTermFreqValueSource mirrors org.apache.lucene.queries.function.valuesource.SumTotalTermFreqValueSource.
type SumTotalTermFreqValueSource struct{}

// NewSumTotalTermFreqValueSource builds a SumTotalTermFreqValueSource.
func NewSumTotalTermFreqValueSource() *SumTotalTermFreqValueSource { return &SumTotalTermFreqValueSource{} }

// TotalTermFreqValueSource mirrors org.apache.lucene.queries.function.valuesource.TotalTermFreqValueSource.
type TotalTermFreqValueSource struct{}

// NewTotalTermFreqValueSource builds a TotalTermFreqValueSource.
func NewTotalTermFreqValueSource() *TotalTermFreqValueSource { return &TotalTermFreqValueSource{} }

