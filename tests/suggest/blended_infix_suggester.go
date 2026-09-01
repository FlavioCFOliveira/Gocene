// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	linearCoef = 0.10
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
	ais, err := NewAnalyzingInfixSuggester(dir, analyzer)
	if err != nil {
		return nil, err
	}
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
		blenderType:            blenderType,
		numFactor:              numFactor,
		exponent:               exponent,
	}

	// Override the results creator to apply blending
	bis.resultsCreator = bis.createBlendedResults
	// Override the text field type to enable term vectors
	bis.fieldTypeCreator = bis.getBlendedTextFieldType

	return bis, nil
}

func (s *BlendedInfixSuggester) getBlendedTextFieldType() *index.FieldType {
	ft := index.NewFieldType(index.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetOmitNorms(true)
	return ft
}

func (s *BlendedInfixSuggester) LookupResults(key string, contexts [][]byte, onlyMorePopular bool, num int) ([]*LookupResult, error) {
	// BlendedInfixSuggester overrides lookup to multiply num by numFactor
	// We have to call the internal lookup logic of AnalyzingInfixSuggester, but with num * numFactor.
	// Since AnalyzingInfixSuggester.lookup is private and doesn't take num as a direct param in some variants,
	// we'll implement the call here.
	return s.lookupInternal(key, contexts, num*s.numFactor, s.allTermsRequired, s.highlight)
}

// lookupInternal is a helper to access the internal lookup logic of AnalyzingInfixSuggester
// but with the blended num.
func (s *BlendedInfixSuggester) lookupInternal(key string, contexts [][]byte, num int, allTermsRequired bool, doHighlight bool) ([]*LookupResult, error) {
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
	prefixToken string) []*LookupResult {

	actualNum := num / s.numFactor
	results := make([]*LookupResult, 0, len(hits.ScoreDocs))

	termVectors := searcher.IndexReader().TermVectors()
	textDV := searcher.IndexReader().MultiDocValues().BinaryValues(textFieldName)
	payloadsDV := searcher.IndexReader().MultiDocValues().BinaryValues(payloadsFieldName)

	for _, sd := range hits.ScoreDocs {
		textDV.Advance(sd.Doc)
		text := string(textDV.BinaryValue())
		weight := sd.Fields[0].(int64)

		var payload []byte
		if payloadsDV != nil {
			if payloadsDV.Advance(sd.Doc) == sd.Doc {
				payload = append([]byte(nil), payloadsDV.BinaryValue()...)
			}
		}

		var coefficient float64
		if strings.HasPrefix(text, key) {
			coefficient = 1.0
		} else {
			coefficient = s.createCoefficient(termVectors, sd.Doc, matchedTokens, prefixToken)
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
			resultKey = s.highlight(text, matchedTokens, prefixToken)
		} else {
			resultKey = text
		}

		results = append(results, &LookupResult{
			Key:      resultKey,
			Value:    score,
			Payload:  payload,
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

	tv := termVectors.Get(doc, textFieldName)
	it := tv.Iterator()

	position := math.MaxInt32
	for {
		term, ok := it.Next()
		if !ok {
			break
		}
		docTerm := string(term)
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
			posEnum := it.Postings(nil, index.PostingsOffsets)
			posEnum.NextDoc()
			p := posEnum.NextPosition()
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
