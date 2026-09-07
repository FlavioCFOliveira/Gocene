// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"fmt"
	"io"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TokenStreamComponents encapsulates the outer components of a token stream.
//
// This is the Go port of Lucene's Analyzer.TokenStreamComponents.
type TokenStreamComponents struct {
	// source is the function to set the reader on the tokenizer.
	source func(io.Reader) error
	// sink is the resulting token stream.
	sink api.TokenStream
	// reusableStringReader is an internal cache used by TokenStreamFromString.
	reusableStringReader *ReusableStringReader
}

// GetTokenStream returns the sink TokenStream.
func (tsc *TokenStreamComponents) GetTokenStream() api.TokenStream {
	return tsc.sink
}

// SetReader resets the encapsulated components with the given reader.
func (tsc *TokenStreamComponents) SetReader(reader io.Reader) error {
	return tsc.source(reader)
}

// ReuseStrategy defines how TokenStreamComponents are reused per call to TokenStream.
type ReuseStrategy interface {
	// GetReusableComponents gets the reusable TokenStreamComponents for the field with the given name.
	GetReusableComponents(a *Analyzer, fieldName string) *TokenStreamComponents
	// SetReusableComponents stores the given TokenStreamComponents as the reusable components for the field.
	SetReusableComponents(a *Analyzer, fieldName string, components *TokenStreamComponents)
}

type globalReuseStrategy struct{}

func (s *globalReuseStrategy) GetReusableComponents(a *Analyzer, fieldName string) *TokenStreamComponents {
	val, ok := a.storedValue.Load("global")
	if !ok {
		return nil
	}
	return val.(*TokenStreamComponents)
}

func (s *globalReuseStrategy) SetReusableComponents(a *Analyzer, fieldName string, components *TokenStreamComponents) {
	a.storedValue.Store("global", components)
}

type perFieldReuseStrategy struct{}

func (s *perFieldReuseStrategy) GetReusableComponents(a *Analyzer, fieldName string) *TokenStreamComponents {
	val, ok := a.storedValue.Load("per-field")
	if !ok {
		return nil
	}
	m := val.(map[string]*TokenStreamComponents)
	return m[fieldName]
}

func (s *perFieldReuseStrategy) SetReusableComponents(a *Analyzer, fieldName string, components *TokenStreamComponents) {
	val, ok := a.storedValue.Load("per-field")
	var m map[string]*TokenStreamComponents
	if !ok {
		m = make(map[string]*TokenStreamComponents)
		a.storedValue.Store("per-field", m)
	} else {
		m = val.(map[string]*TokenStreamComponents)
	}
	m[fieldName] = components
}

var (
	// GlobalReuseStrategy reuses the same components for every field.
	GlobalReuseStrategy = &globalReuseStrategy{}
	// PerFieldReuseStrategy reuses components per-field.
	PerFieldReuseStrategy = &perFieldReuseStrategy{}
)

// Analyzer builds TokenStreams, which analyze text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Analyzer.
type Analyzer struct {
	reuseStrategy ReuseStrategy
	storedValue   sync.Map

	// CreateComponents creates a new TokenStreamComponents instance for this analyzer.
	CreateComponents func(fieldName string) *TokenStreamComponents
	// normalizeFilter wraps the given TokenStream in order to apply normalization filters.
	normalizeFilter func(fieldName string, in api.TokenStream) api.TokenStream
	// InitReader adds a CharFilter chain.
	InitReader func(fieldName string, reader io.Reader) io.Reader
	// InitReaderForNormalization wraps the given Reader with CharFilters that make sense for normalization.
	InitReaderForNormalization func(fieldName string, reader io.Reader) io.Reader
	// AttributeFactory returns the AttributeFactory to be used for analysis and normalization.
	AttributeFactory func(fieldName string) util.AttributeFactory
}

// BaseAnalyzer is an alias for Analyzer to support legacy implementations.
type BaseAnalyzer = Analyzer

// NewAnalyzer creates a new Analyzer with the given ReuseStrategy.
func NewAnalyzer(strategy ReuseStrategy) *Analyzer {
	if strategy == nil {
		strategy = GlobalReuseStrategy
	}
	a := &Analyzer{
		reuseStrategy: strategy,
	}
	// Default implementations
	a.normalizeFilter = func(fieldName string, in api.TokenStream) api.TokenStream {
		return in
	}
	a.InitReader = func(fieldName string, reader io.Reader) io.Reader {
		return reader
	}
	a.InitReaderForNormalization = func(fieldName string, reader io.Reader) io.Reader {
		return reader
	}
	a.AttributeFactory = func(fieldName string) util.AttributeFactory {
		return util.DefaultAttributeFactoryInstance
	}
	return a
}

// TokenStream returns a TokenStream suitable for fieldName, tokenizing the contents of reader.
func (a *Analyzer) TokenStream(fieldName string, reader io.Reader) (api.TokenStream, error) {
	components := a.reuseStrategy.GetReusableComponents(a, fieldName)
	r := a.InitReader(fieldName, reader)
	if components == nil {
		components = a.CreateComponents(fieldName)
		a.reuseStrategy.SetReusableComponents(a, fieldName, components)
	}
	if err := components.SetReader(r); err != nil {
		return nil, err
	}
	return components.GetTokenStream(), nil
}

// TokenStreamFromString returns a TokenStream suitable for fieldName, tokenizing the contents of text.
func (a *Analyzer) TokenStreamFromString(fieldName string, text string) (api.TokenStream, error) {
	components := a.reuseStrategy.GetReusableComponents(a, fieldName)
	var strReader *ReusableStringReader
	if components == nil || components.reusableStringReader == nil {
		strReader = NewReusableStringReader()
	} else {
		strReader = components.reusableStringReader
	}
	strReader.SetValue(text)
	r := a.InitReader(fieldName, strReader)
	if components == nil {
		components = a.CreateComponents(fieldName)
		a.reuseStrategy.SetReusableComponents(a, fieldName, components)
	}
	if err := components.SetReader(r); err != nil {
		return nil, err
	}
	components.reusableStringReader = strReader
	return components.GetTokenStream(), nil
}

func (a *Analyzer) Normalize(fieldName string) api.TokenStream {
	components := a.CreateComponents(fieldName)
	return a.normalizeFilter(fieldName, components.GetTokenStream())
}

// NormalizeText normalizes a string down to the representation that it would have in the index.
func (a *Analyzer) NormalizeText(fieldName string, text string) (*util.BytesRef, error) {
	// Apply char filters
	var filteredText string
	reader := NewReusableStringReader()
	reader.SetValue(text)
	filterReader := a.InitReaderForNormalization(fieldName, reader)

	var buf []byte
	br := bufio.NewReader(filterReader)
	for {
		b, err := br.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("normalization threw an unexpected exception: %w", err)
		}
		buf = append(buf, b)
	}
	filteredText = string(buf)

	attrFactory := a.AttributeFactory(fieldName)
	ts := NewStringTokenStream(attrFactory, filteredText, len(text))
	normTs := a.normalizeFilter(fieldName, ts)
	defer normTs.Close()

	ts.Reset()
	ok, err := normTs.IncrementToken()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("the normalization token stream is expected to produce exactly 1 token, but got 0 for analyzer and input %q", text)
	}

	termAtt := normTs.GetAttribute(TermToBytesRefAttributeType).(*TermToBytesRefAttribute)
	term := util.DeepCopyOfBytesRef(termAtt.GetBytesRef())

	if ok, err := normTs.IncrementToken(); err != nil || ok {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("the normalization token stream is expected to produce exactly 1 token, but got 2+ for analyzer and input %q", text)
	}

	normTs.End()
	return term, nil
}

// GetPositionIncrementGap returns the position increment gap.
func (a *Analyzer) GetPositionIncrementGap(fieldName string) int {
	return 0
}

// GetOffsetGap returns the offset gap.
func (a *Analyzer) GetOffsetGap(fieldName string) int {
	return 1
}

// Close releases persistent resources used by this Analyzer.
func (a *Analyzer) Close() error {
	return nil
}

type stringTokenStream struct {
	BaseTokenStream
	value  string
	length int
	used   bool
	termAtt *CharTermAttribute
	offAtt  *OffsetAttribute
}

func NewStringTokenStream(factory util.AttributeFactory, value string, length int) api.TokenStream {
	ts := &stringTokenStream{
		BaseTokenStream: *NewBaseTokenStreamWithFactory(factory),
		value:           value,
		length:          length,
		used:            true,
	}
	ts.termAtt = ts.GetAttribute(CharTermAttributeType).(*CharTermAttribute)
	ts.offAtt = ts.GetAttribute(OffsetAttributeType).(*OffsetAttribute)
	return ts
}

func (ts *stringTokenStream) Reset() error {
	ts.used = false
	return nil
}

func (ts *stringTokenStream) IncrementToken() (bool, error) {
	if ts.used {
		return false, nil
	}
	ts.ClearAttributes()
	ts.termAtt.Append(ts.value)
	ts.offAtt.SetOffset(0, ts.length)
	ts.used = true
	return true, nil
}

func (ts *stringTokenStream) End() error {
	ts.offAtt.SetOffset(ts.length, ts.length)
	return nil
}
