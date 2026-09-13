// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analyzing

import (
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest"
)

const (
	linearCoef       = 0.10
	defaultNumFactor = 10
)

type BlenderType int

const (
	BlenderCustom BlenderType = iota
	BlenderPositionLinear
	BlenderPositionReciprocal
	BlenderPositionExponentialReciprocal
)

// BlendedInfixSuggester is an extension of AnalyzingInfixSuggester that transforms the weight
// based on the position of the searched term.
type BlendedInfixSuggester struct {
	*AnalyzingInfixSuggester
	blenderType BlenderType
	numFactor   int
	exponent    float64
}

// NewBlendedInfixSuggester creates a new instance with default blending (Position Linear).
func NewBlendedInfixSuggester(dir store.Directory, analyzer analysis.Analyzer) (*BlendedInfixSuggester, error) {
	return NewBlendedInfixSuggesterAdvanced(
		dir,
		analyzer,
		analyzer,
		defaultMinPrefixChars,
		BlenderPositionLinear,
		defaultNumFactor,
		2.0,
		false,
		defaultAllTermsReq,
		defaultHighlight,
	)
}

func NewBlendedInfixSuggesterAdvanced(
	dir store.Directory,
	indexAnalyzer, queryAnalyzer analysis.Analyzer,
	minPrefixChars int,
	blenderType BlenderType,
	numFactor int,
	exponent float64,
	commitOnBuild bool,
	allTermsRequired bool,
	highlight bool,
) (*BlendedInfixSuggester, error) {
	ais, err := NewAnalyzingInfixSuggesterAdvanced(
		dir,
		indexAnalyzer,
		queryAnalyzer,
		minPrefixChars,
		commitOnBuild,
		allTermsRequired,
		highlight,
		defaultCloseIWOnBuild,
	)
	if err != nil {
		return nil, err
	}

	bis := &BlendedInfixSuggester{
		AnalyzingInfixSuggester: ais,
		blenderType:             blenderType,
		numFactor:               numFactor,
		exponent:                exponent,
	}

	// Override the results creator to apply blending
	bis.resultsCreator = bis.createBlendedResults
	// Override the text field type to enable term vectors
	bis.fieldTypeCreator = bis.getBlendedTextFieldType

	return bis, nil
}

func (s *BlendedInfixSuggester) getBlendedTextFieldType() *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(spi.IndexOptionsDocsAndFreqsAndPositions)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetOmitNorms(true)
	return ft
}

func (s *BlendedInfixSuggester) LookupResults(key string, contexts [][]byte, onlyMorePopular bool, num int) ([]*suggest.LookupResult, error) {
	// BlendedInfixSuggester overrides lookup to multiply num by numFactor
	// We have to call the internal lookup logic of AnalyzingInfixSuggester, but with num * numFactor.
	// Since AnalyzingInfixSuggester.lookup is private and doesn't take num as a direct param in some variants,
	// we'll implement the call here.
	return s.lookupInternal(key, contexts, num*s.numFactor, s.allTermsRequired, s.highlight)
}

// lookupInternal is a helper to access the internal lookup logic of AnalyzingInfixSuggester
// but with the blended num.
func (s *BlendedInfixSuggester) lookupInternal(key string, contexts [][]byte, num int, allTermsRequired bool, doHighlight bool) ([]*suggest.LookupResult, error) {
	// We can't call the private lookup method of the embedded struct from outside the package
	// but we are in the same package, so we can.
	return s.AnalyzingInfixSuggester.lookup(key, contexts, num, allTermsRequired, doHighlight)
}

func (s *BlendedInfixSuggester) createBlendedResults(
	searcher *search.IndexSearcher,
	hits *search.TopFieldDocs,
	num int,
	key string,
	doHighlight bool,
	matchedTokens []string,
	prefixToken string) []*suggest.LookupResult {

	actualNum := num / s.numFactor
	results := make([]*suggest.LookupResult, 0, len(hits.FieldDocs))

	reader := searcher.GetIndexReader()
	termVectors, err := reader.TermVectors()
	if err != nil {
		return nil
	}
	textDV, err := index.MultiDocValuesGetBinaryValues(reader, textFieldName)
	if err != nil {
		return nil
	}
	payloadsDV, err := index.MultiDocValuesGetBinaryValues(reader, payloadsFieldName)
	if err != nil {
		return nil
	}

	for _, fd := range hits.FieldDocs {
		if _, err := textDV.Advance(fd.Doc); err != nil {
			return nil
		}
		raw, err := textDV.BinaryValue()
		if err != nil {
			return nil
		}
		text := string(raw)

		var weight int64
		if len(fd.Fields) > 0 {
			if w, ok := fd.Fields[0].(int64); ok {
				weight = w
			}
		}

		var payload []byte
		if payloadsDV != nil {
			target, err := payloadsDV.Advance(fd.Doc)
			if err != nil {
				return nil
			}
			if target == fd.Doc {
				pv, err := payloadsDV.BinaryValue()
				if err != nil {
					return nil
				}
				payload = append([]byte(nil), pv...)
			}
		}

		var coefficient float64
		if strings.HasPrefix(text, key) {
			coefficient = 1.0
		} else {
			coefficient = s.createCoefficient(termVectors, fd.Doc, matchedTokens, prefixToken)
		}

		if weight == 0 {
			weight = 1
		}
		// Lucene: if (weight < 1 / LINEAR_COEF && weight > -1 / LINEAR_COEF) { weight *= 1 / LINEAR_COEF; }
		// This is weird, but let's be faithful.
		if weight < int64(1/linearCoef) && weight > int64(-1/linearCoef) {
			weight *= int64(1 / linearCoef)
		}
		score := int64(float64(weight) * coefficient)

		var resultKey string
		if doHighlight {
			resultKey = s.doHighlight(text, matchedTokens, prefixToken)
		} else {
			resultKey = text
		}

		results = append(results, &suggest.LookupResult{
			Key:     resultKey,
			Value:   score,
			Payload: payload,
		})
	}

	// Sort by score descending, then key ascending
	sort.Slice(results, func(i, j int) bool {
		if results[i].Value != results[j].Value {
			return results[i].Value > results[j].Value
		}
		return results[i].Key < results[j].Key
	})

	if len(results) > actualNum {
		results = results[:actualNum]
	}

	return results
}

func (s *BlendedInfixSuggester) createCoefficient(
	termVectors index.TermVectors,
	doc int,
	matchedTokens []string,
	prefixToken string) float64 {

	tv, err := termVectors.GetField(doc, textFieldName)
	if err != nil || tv == nil {
		return s.calculateCoefficient(math.MaxInt32)
	}
	it, err := tv.GetIterator()
	if err != nil {
		return s.calculateCoefficient(math.MaxInt32)
	}

	position := math.MaxInt32
	for {
		term, err := it.Next()
		if err != nil {
			return s.calculateCoefficient(math.MaxInt32)
		}
		if term == nil {
			break
		}
		docTerm := term.Text()
		isMatch := false
		for _, mt := range matchedTokens {
			if mt == docTerm {
				isMatch = true
				break
			}
		}
		if !isMatch && prefixToken != "" && strings.HasPrefix(docTerm, prefixToken) {
			isMatch = true
		}

		if isMatch {
			posEnum, err := it.Postings(spi.PostingsFlagOffsets)
			if err != nil {
				return s.calculateCoefficient(math.MaxInt32)
			}
			if _, err := posEnum.NextDoc(); err != nil {
				return s.calculateCoefficient(math.MaxInt32)
			}
			p, err := posEnum.NextPosition()
			if err != nil {
				return s.calculateCoefficient(math.MaxInt32)
			}
			if p < position {
				position = p
			}
		}
	}

	return s.calculateCoefficient(position)
}

func (s *BlendedInfixSuggester) calculateCoefficient(position int) float64 {
	switch s.blenderType {
	case BlenderPositionLinear:
		return 1.0 - linearCoef*float64(position)
	case BlenderPositionReciprocal:
		return 1.0 / float64(position+1)
	case BlenderPositionExponentialReciprocal:
		return 1.0 / math.Pow(float64(position+1), s.exponent)
	case BlenderCustom:
		fallthrough
	default:
		return 1.0
	}
}
