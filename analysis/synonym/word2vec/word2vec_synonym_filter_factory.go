// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package word2vec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DefaultMaxSynonymsPerTerm is the default maximum number of synonyms
// returned per token, matching Lucene's DEFAULT_MAX_SYNONYMS_PER_TERM.
const DefaultMaxSynonymsPerTerm = 5

// DefaultMinAcceptedSimilarity is the default minimum cosine similarity
// threshold, matching Lucene's DEFAULT_MIN_ACCEPTED_SIMILARITY.
const DefaultMinAcceptedSimilarity float32 = 0.8

// Word2VecSynonymFilterFactory is a TokenFilterFactory that creates
// Word2VecSynonymFilter instances. It implements util.ResourceLoaderAware
// so that the model is loaded from the ResourceLoader after construction.
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymFilterFactory
// from Apache Lucene 10.4.0.
//
// Deviation: Lucene's factory extends AbstractAnalysisFactory which
// provides getInt / getFloat / require / get helpers via a Map<String,String>.
// Gocene does not yet ship that base class; param extraction is done
// inline here.
//
// Deviation: ResourceLoaderAware is from util; Java imports it from
// org.apache.lucene.util.
type Word2VecSynonymFilterFactory struct {
	maxSynonymsPerTerm    int
	minAcceptedSimilarity float32
	format                Word2VecSupportedFormats
	word2vecModelFileName string

	synonymProvider *Word2VecSynonymProvider
}

// NewWord2VecSynonymFilterFactory constructs the factory from the given
// parameter map. Required parameter: "model". Optional: "maxSynonymsPerTerm"
// (default 5), "minAcceptedSimilarity" (default 0.8), "format" (default
// "dl4j"). Unknown parameters cause an error.
func NewWord2VecSynonymFilterFactory(args map[string]string) (*Word2VecSynonymFilterFactory, error) {
	f := &Word2VecSynonymFilterFactory{
		maxSynonymsPerTerm:    DefaultMaxSynonymsPerTerm,
		minAcceptedSimilarity: DefaultMinAcceptedSimilarity,
		format:                Word2VecFormatDL4J,
	}

	remaining := make(map[string]string, len(args))
	for k, v := range args {
		remaining[k] = v
	}

	if v, ok := remaining["maxSynonymsPerTerm"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("word2vec: invalid maxSynonymsPerTerm %q: %w", v, err)
		}
		f.maxSynonymsPerTerm = n
		delete(remaining, "maxSynonymsPerTerm")
	}

	if v, ok := remaining["minAcceptedSimilarity"]; ok {
		fv, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return nil, fmt.Errorf("word2vec: invalid minAcceptedSimilarity %q: %w", v, err)
		}
		f.minAcceptedSimilarity = float32(fv)
		delete(remaining, "minAcceptedSimilarity")
	}

	modelName, ok := remaining["model"]
	if !ok {
		return nil, fmt.Errorf("word2vec: required parameter 'model' is missing")
	}
	f.word2vecModelFileName = modelName
	delete(remaining, "model")

	if v, ok := remaining["format"]; ok {
		switch strings.ToUpper(v) {
		case "DL4J":
			f.format = Word2VecFormatDL4J
		default:
			return nil, fmt.Errorf("word2vec: unsupported model format %q", v)
		}
		delete(remaining, "format")
	}

	if len(remaining) > 0 {
		return nil, fmt.Errorf("word2vec: unknown parameters: %v", remaining)
	}

	if f.minAcceptedSimilarity <= 0 || f.minAcceptedSimilarity > 1 {
		return nil, fmt.Errorf(
			"word2vec: minAcceptedSimilarity must be in the range (0, 1]. Found: %v",
			f.minAcceptedSimilarity,
		)
	}
	if f.maxSynonymsPerTerm <= 0 {
		return nil, fmt.Errorf(
			"word2vec: maxSynonymsPerTerm must be a positive integer greater than 0. Found: %d",
			f.maxSynonymsPerTerm,
		)
	}

	return f, nil
}

// GetSynonymProvider returns the loaded synonym provider, or nil if
// Inform has not been called yet.
func (f *Word2VecSynonymFilterFactory) GetSynonymProvider() *Word2VecSynonymProvider {
	return f.synonymProvider
}

// Create wraps input in a Word2VecSynonymFilter. If Inform has not yet
// been called (synonymProvider == nil), the input stream is returned
// unchanged.
func (f *Word2VecSynonymFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	if f.synonymProvider == nil {
		// Passthrough until Inform is called.
		return analysis.NewBaseTokenFilter(input)
	}
	filter, err := NewWord2VecSynonymFilter(
		input,
		f.synonymProvider,
		f.maxSynonymsPerTerm,
		f.minAcceptedSimilarity,
	)
	if err != nil {
		// synonymProvider is non-nil here so the only possible error is
		// the nil-provider check, which cannot fire. Panic to expose bugs.
		panic(fmt.Sprintf("word2vec: NewWord2VecSynonymFilter: %v", err))
	}
	return filter
}

// Inform loads the Word2Vec model from the ResourceLoader and constructs
// the synonym provider. Implements util.ResourceLoaderAware.
func (f *Word2VecSynonymFilterFactory) Inform(loader util.ResourceLoader) error {
	provider, err := GetSynonymProvider(loader, f.word2vecModelFileName, f.format)
	if err != nil {
		return fmt.Errorf("word2vec: inform: %w", err)
	}
	f.synonymProvider = provider
	return nil
}

// Compile-time interface assertions.
var _ analysis.TokenFilterFactory = (*Word2VecSynonymFilterFactory)(nil)
var _ util.ResourceLoaderAware = (*Word2VecSynonymFilterFactory)(nil)
