// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification

import (
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// readerStatProvider is duck-typed against readers that expose the per-field
// aggregate statistics used by BooleanPerceptronClassifier to compute bias.
type readerStatProvider interface {
	GetSumTotalTermFreq(field string) (int64, error)
	GetDocCount(field string) (int, error)
}

// BooleanPerceptronClassifier classifies text with a binary (true/false)
// perceptron model.  During construction it:
//  1. Reads every term in the text field and initialises its weight from the
//     global total-term-frequency.
//  2. Searches the index for training documents; for each misclassified
//     document it updates the term weights using the perceptron update rule.
//  3. Builds an FST<Long> mapping term → weight for fast O(|term|) scoring.
//
// At inference time AssignClass tokenises the input text, sums the weights of
// the observed terms, and compares the sum against the bias.  The confidence
// score is 1 − exp(−|bias − sum| / bias).
//
// Port of org.apache.lucene.classification.BooleanPerceptronClassifier.
type BooleanPerceptronClassifier struct {
	analyzer      analysis.Analyzer
	textFieldName string
	bias          float64
	weights       *fst.FST[int64]
}

// NewBooleanPerceptronClassifier creates and trains the classifier.
//
//   - reader: an index.IndexReaderInterface over the training index.
//   - analyzer: the Analyzer used to tokenize text.
//   - query: an optional filter query (pass nil to use all indexed docs).
//   - batchSize: number of documents per weight-update batch (nil defaults to
//     all documents processed as one batch).
//   - bias: the decision boundary; when nil or 0 it is computed automatically
//     as the average total-term-frequency for textFieldName.
//   - classFieldName: the field holding the Boolean class label ("true"/"false").
//   - textFieldName: the field holding the text to classify.
func NewBooleanPerceptronClassifier(
	reader interface{},
	analyzer analysis.Analyzer,
	query interface{},
	bias *float64,
	classFieldName string,
	textFieldName string,
) *BooleanPerceptronClassifier {
	c := &BooleanPerceptronClassifier{
		analyzer:      analyzer,
		textFieldName: textFieldName,
	}

	ri, ok := reader.(index.IndexReaderInterface)
	if !ok {
		return c
	}
	tp, ok := reader.(termsProvider)
	if !ok {
		return c
	}

	// Resolve bias.
	effectiveBias := 0.0
	if bias != nil && *bias != 0 {
		effectiveBias = *bias
	} else {
		// Compute average total-term-frequency for the text field.
		if sp, ok := reader.(readerStatProvider); ok {
			sumTTF, err := sp.GetSumTotalTermFreq(textFieldName)
			if err == nil && sumTTF >= 0 {
				docCount, err := sp.GetDocCount(textFieldName)
				if err == nil && docCount > 0 {
					effectiveBias = float64(sumTTF) / float64(docCount)
				}
			}
		}
	}
	if effectiveBias == 0 {
		// Unable to determine bias; return untrained classifier.
		return c
	}
	c.bias = effectiveBias

	// Phase 1: initialise weights from global term statistics.
	textTerms, err := tp.Terms(textFieldName)
	if err != nil || textTerms == nil {
		return c
	}
	weights := make(map[string]int64)
	termsIt, err := textTerms.GetIterator()
	if err != nil {
		return c
	}
	for {
		t, err := termsIt.Next()
		if err != nil || t == nil {
			break
		}
		ttf, err := termsIt.TotalTermFreq()
		if err != nil {
			break
		}
		weights[t.Text()] = ttf
	}

	// Phase 2: perceptron training loop.
	var filterQuery search.Query
	if q, ok := query.(search.Query); ok {
		filterQuery = q
	}
	searcher := search.NewIndexSearcher(ri)

	trainingBQ := search.NewBooleanQuery()
	trainingBQ.Add(search.NewWildcardQuery(index.NewTerm(classFieldName, "*")), search.MUST)
	if filterQuery != nil {
		trainingBQ.Add(filterQuery, search.MUST)
	}
	topDocs, err := searcher.Search(trainingBQ, ri.MaxDoc())
	if err != nil {
		return c
	}

	batchSize := len(topDocs.ScoreDocs) // treat entire training set as one batch by default
	if batchSize == 0 {
		batchSize = 1
	}

	tvProvider, _ := reader.(index.IndexReaderInterface)
	tvs, _ := tvProvider.TermVectors()

	batchCount := 0
	for _, sd := range topDocs.ScoreDocs {
		doc, err := searcher.Doc(sd.Doc)
		if err != nil || doc == nil {
			continue
		}
		textField := doc.GetField(textFieldName)
		classField := doc.GetField(classFieldName)
		if textField == nil || classField == nil {
			continue
		}

		// Temporarily assign class for the current weight vector.
		result := c.assignWithWeights(textField.StringValue(), weights)
		if result == nil {
			continue
		}
		assigned := result.AssignedClass
		correct := classField.StringValue() == "true"
		modifier := 0.0
		if correct && !assigned {
			modifier = 1.0
		} else if !correct && assigned {
			modifier = -1.0
		}

		if modifier != 0 && tvs != nil {
			docTermVectors, err := tvs.GetField(sd.Doc, textFieldName)
			if err == nil && docTermVectors != nil {
				docIt, err := docTermVectors.GetIterator()
				if err == nil {
					for {
						t, err := docIt.Next()
						if err != nil || t == nil {
							break
						}
						termFreq, _ := docIt.TotalTermFreq()
						prev := weights[t.Text()]
						newVal := prev + int64(modifier*float64(termFreq))
						if newVal < 0 {
							newVal = 0
						}
						weights[t.Text()] = newVal
					}
				}
			}
		}

		batchCount++
		if batchCount%batchSize == 0 {
			c.weights = buildWeightFST(weights)
		}
	}
	// Final FST build.
	c.weights = buildWeightFST(weights)
	return c
}

// AssignClass returns true/false and a confidence score for the input text.
func (c *BooleanPerceptronClassifier) AssignClass(text string) (*ClassificationResult[*util.BytesRef], error) {
	if c.analyzer == nil || c.weights == nil {
		return nil, nil
	}
	result := c.assignWithWeights(text, nil)
	if result == nil {
		return nil, nil
	}
	label := "false"
	if result.AssignedClass {
		label = "true"
	}
	return &ClassificationResult[*util.BytesRef]{
		AssignedClass: util.NewBytesRef([]byte(label)),
		Score:         result.Score,
	}, nil
}

// GetClasses returns [true, false] or [false, true] sorted by descending score.
func (c *BooleanPerceptronClassifier) GetClasses(text string) ([]*ClassificationResult[*util.BytesRef], error) {
	best, err := c.AssignClass(text)
	if err != nil || best == nil {
		return nil, err
	}
	other := "true"
	if best.AssignedClass.String() == "true" {
		other = "false"
	}
	list := []*ClassificationResult[*util.BytesRef]{
		best,
		{AssignedClass: util.NewBytesRef([]byte(other)), Score: 1.0 - best.Score},
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	return list, nil
}

// GetClassesMax returns the top max classes.
func (c *BooleanPerceptronClassifier) GetClassesMax(text string, max int) ([]*ClassificationResult[*util.BytesRef], error) {
	list, err := c.GetClasses(text)
	if err != nil {
		return nil, err
	}
	if max > len(list) {
		max = len(list)
	}
	return list[:max], nil
}

// assignWithWeights is a helper for both training-time (weights map) and
// inference-time (FST) scoring.  When weights is non-nil it consults the map
// directly; otherwise it falls back to the built FST.
func (c *BooleanPerceptronClassifier) assignWithWeights(text string, weights map[string]int64) *ClassificationResult[bool] {
	if c.analyzer == nil {
		return nil
	}
	ts, err := c.analyzer.TokenStream(c.textFieldName, strings.NewReader(text))
	if err != nil {
		return nil
	}
	src := ts.(interface {
		GetAttributeSource() *util.AttributeSource
	}).GetAttributeSource()
	var termAttr analysis.CharTermAttribute
	if raw := src.GetAttribute(analysis.CharTermAttributeType); raw != nil {
		termAttr, _ = raw.(analysis.CharTermAttribute)
	}

	var output int64
	for {
		ok, err := ts.IncrementToken()
		if err != nil || !ok {
			break
		}
		if termAttr == nil {
			continue
		}
		word := termAttr.String()
		if weights != nil {
			output += weights[word]
		} else if c.weights != nil {
			scratchInts := util.NewIntsRefBuilder()
			scratchBytes := util.NewBytesRefBuilder()
			scratchBytes.CopyChars(word)
			intsRef := fst.ToIntsRef(scratchBytes.Get(), scratchInts)
			if val, found, err := fst.Get(c.weights, intsRef); err == nil && found {
				output += val
			}
		}
	}
	_ = ts.End()
	_ = ts.Close()

	score := 1.0 - math.Exp(-math.Abs(c.bias-float64(output))/c.bias)
	return &ClassificationResult[bool]{
		AssignedClass: output >= int64(c.bias),
		Score:         score,
	}
}

// buildWeightFST compiles a sorted map of term → weight into an FST<Long>.
// The FST is rebuilt from scratch because the current FSTCompiler requires
// inputs to be added in sorted order.  A nil weights map or compile failure
// returns nil (untrained classifier).
func buildWeightFST(weights map[string]int64) *fst.FST[int64] {
	if len(weights) == 0 {
		return nil
	}
	// Sort keys lexicographically to satisfy FSTCompiler's ordering constraint.
	keys := make([]string, 0, len(weights))
	for k := range weights {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	compiler := fst.NewFSTCompilerBuilder[int64](fst.InputTypeByte1, fst.PositiveIntOutputs()).Build()
	scratchInts := util.NewIntsRefBuilder()
	scratchBytes := util.NewBytesRefBuilder()

	for _, k := range keys {
		v := weights[k]
		if v < 0 {
			v = 0
		}
		scratchBytes.CopyChars(k)
		intsRef := fst.ToIntsRef(scratchBytes.Get(), scratchInts)
		if err := compiler.Add(intsRef, v); err != nil {
			return nil
		}
	}
	metadata, err := compiler.Compile()
	if err != nil || metadata == nil {
		return nil
	}
	built, err := fst.FromFSTReader(metadata, compiler.GetFSTReader())
	if err != nil {
		return nil
	}
	return built
}

