// Package payloads hosts the Sprint 29 overflow ports for
// org.apache.lucene.queries.payloads.
package payloads

// The Sprint 29 queries-module overflow surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// AveragePayloadFunction mirrors org.apache.lucene.queries.payloads.AveragePayloadFunction.
type AveragePayloadFunction struct{}

// NewAveragePayloadFunction builds a AveragePayloadFunction.
func NewAveragePayloadFunction() *AveragePayloadFunction { return &AveragePayloadFunction{} }

// MaxPayloadFunction mirrors org.apache.lucene.queries.payloads.MaxPayloadFunction.
type MaxPayloadFunction struct{}

// NewMaxPayloadFunction builds a MaxPayloadFunction.
func NewMaxPayloadFunction() *MaxPayloadFunction { return &MaxPayloadFunction{} }

// MinPayloadFunction mirrors org.apache.lucene.queries.payloads.MinPayloadFunction.
type MinPayloadFunction struct{}

// NewMinPayloadFunction builds a MinPayloadFunction.
func NewMinPayloadFunction() *MinPayloadFunction { return &MinPayloadFunction{} }

// PayloadDecoder mirrors org.apache.lucene.queries.payloads.PayloadDecoder.
type PayloadDecoder struct{}

// NewPayloadDecoder builds a PayloadDecoder.
func NewPayloadDecoder() *PayloadDecoder { return &PayloadDecoder{} }

// PayloadMatcher mirrors org.apache.lucene.queries.payloads.PayloadMatcher.
type PayloadMatcher struct{}

// NewPayloadMatcher builds a PayloadMatcher.
func NewPayloadMatcher() *PayloadMatcher { return &PayloadMatcher{} }

// SumPayloadFunction mirrors org.apache.lucene.queries.payloads.SumPayloadFunction.
type SumPayloadFunction struct{}

// NewSumPayloadFunction builds a SumPayloadFunction.
func NewSumPayloadFunction() *SumPayloadFunction { return &SumPayloadFunction{} }

// PayloadFunction mirrors org.apache.lucene.queries.payloads.PayloadFunction.
type PayloadFunction struct{}

// NewPayloadFunction builds a PayloadFunction.
func NewPayloadFunction() *PayloadFunction { return &PayloadFunction{} }

// PayloadScoreQuery mirrors org.apache.lucene.queries.payloads.PayloadScoreQuery.
type PayloadScoreQuery struct{}

// NewPayloadScoreQuery builds a PayloadScoreQuery.
func NewPayloadScoreQuery() *PayloadScoreQuery { return &PayloadScoreQuery{} }

// SpanPayloadCheckQuery mirrors org.apache.lucene.queries.payloads.SpanPayloadCheckQuery.
type SpanPayloadCheckQuery struct{}

// NewSpanPayloadCheckQuery builds a SpanPayloadCheckQuery.
func NewSpanPayloadCheckQuery() *SpanPayloadCheckQuery { return &SpanPayloadCheckQuery{} }

// PayloadMatcherFactory mirrors org.apache.lucene.queries.payloads.PayloadMatcherFactory.
type PayloadMatcherFactory struct{}

// NewPayloadMatcherFactory builds a PayloadMatcherFactory.
func NewPayloadMatcherFactory() *PayloadMatcherFactory { return &PayloadMatcherFactory{} }

