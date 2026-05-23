// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tools

import (
	"io"
	"sync"
)

// ModelLoader loads a model by name from a resource source, returning the
// raw bytes. It is the Go analogue of Lucene's ResourceLoader combined with
// each model constructor.
//
// The caller is responsible for closing the returned ReadCloser. Returning
// (nil, nil) for an unknown name is acceptable; callers interpret nil as
// "model not found".
type ModelLoader interface {
	// Open opens the named resource for reading.
	Open(name string) (io.ReadCloser, error)
}

// ModelRegistry holds the model creation functions registered for each
// model type. Applications register constructors so that OpenNLPOpsFactory
// can load models on demand.
type ModelRegistry struct {
	sentenceModels    sync.Map // string → SentenceModel
	tokenizerModels   sync.Map // string → TokenizerModel
	posModels         sync.Map // string → POSModel
	chunkerModels     sync.Map // string → ChunkerModel
	nerModels         sync.Map // string → TokenNameFinderModel
	lemmatizerModels  sync.Map // string → LemmatizerModel
	lemmaDictionaries sync.Map // string → DictionaryLemmatizer
}

// OpenNLPOpsFactory caches NLP model objects and creates Op wrappers.
//
// Go port of org.apache.lucene.analysis.opennlp.tools.OpenNLPOpsFactory
// (Apache Lucene 10.4.0).
//
// Deviation: The Java class uses opennlp.tools.* model types loaded from
// binary files via ResourceLoader. In Go, models are injected directly
// through Put* methods. There is no binary deserialization layer because
// Go has no OpenNLP library.
var defaultRegistry = &ModelRegistry{}

// PutSentenceModel stores a SentenceModel under name in the global cache.
func PutSentenceModel(name string, model SentenceModel) {
	defaultRegistry.sentenceModels.Store(name, model)
}

// PutTokenizerModel stores a TokenizerModel under name in the global cache.
func PutTokenizerModel(name string, model TokenizerModel) {
	defaultRegistry.tokenizerModels.Store(name, model)
}

// PutPOSModel stores a POSModel under name in the global cache.
func PutPOSModel(name string, model POSModel) {
	defaultRegistry.posModels.Store(name, model)
}

// PutChunkerModel stores a ChunkerModel under name in the global cache.
func PutChunkerModel(name string, model ChunkerModel) {
	defaultRegistry.chunkerModels.Store(name, model)
}

// PutNERModel stores a TokenNameFinderModel under name in the global cache.
func PutNERModel(name string, model TokenNameFinderModel) {
	defaultRegistry.nerModels.Store(name, model)
}

// PutLemmatizerModel stores a LemmatizerModel under name in the global cache.
func PutLemmatizerModel(name string, model LemmatizerModel) {
	defaultRegistry.lemmatizerModels.Store(name, model)
}

// PutDictionaryLemmatizer stores a DictionaryLemmatizer under name in the
// global cache.
func PutDictionaryLemmatizer(name string, lemmatizer DictionaryLemmatizer) {
	defaultRegistry.lemmaDictionaries.Store(name, lemmatizer)
}

// GetSentenceDetector returns an NLPSentenceDetectorOp for the named model.
// If name is empty, returns an op that treats the full input as one sentence.
func GetSentenceDetector(name string) *NLPSentenceDetectorOp {
	if name == "" {
		return NewNLPSentenceDetectorOp()
	}
	v, _ := defaultRegistry.sentenceModels.Load(name)
	if v == nil {
		return NewNLPSentenceDetectorOp()
	}
	return NewNLPSentenceDetectorOpWithModel(v.(SentenceModel))
}

// GetTokenizer returns an NLPTokenizerOp for the named model.
// If name is empty, returns an op that treats each sentence as one token.
func GetTokenizer(name string) *NLPTokenizerOp {
	if name == "" {
		return NewNLPTokenizerOp()
	}
	v, _ := defaultRegistry.tokenizerModels.Load(name)
	if v == nil {
		return NewNLPTokenizerOp()
	}
	return NewNLPTokenizerOpWithModel(v.(TokenizerModel))
}

// GetPOSTagger returns an NLPPOSTaggerOp for the named model.
// Panics if the model is not found in the cache.
func GetPOSTagger(name string) *NLPPOSTaggerOp {
	v, ok := defaultRegistry.posModels.Load(name)
	if !ok {
		panic("OpenNLPOpsFactory: POS model not registered: " + name)
	}
	return NewNLPPOSTaggerOp(v.(POSModel))
}

// GetChunker returns an NLPChunkerOp for the named model.
// Panics if the model is not found in the cache.
func GetChunker(name string) *NLPChunkerOp {
	v, ok := defaultRegistry.chunkerModels.Load(name)
	if !ok {
		panic("OpenNLPOpsFactory: chunker model not registered: " + name)
	}
	return NewNLPChunkerOp(v.(ChunkerModel))
}

// GetNERTagger returns an NLPNERTaggerOp for the named model.
// Panics if the model is not found in the cache.
func GetNERTagger(name string) *NLPNERTaggerOp {
	v, ok := defaultRegistry.nerModels.Load(name)
	if !ok {
		panic("OpenNLPOpsFactory: NER model not registered: " + name)
	}
	return NewNLPNERTaggerOp(v.(TokenNameFinderModel))
}

// GetLemmatizer returns an NLPLemmatizerOp backed by the named dictionary
// and/or model. At least one of dictionaryName and modelName must be non-empty.
func GetLemmatizer(dictionaryName, modelName string) *NLPLemmatizerOp {
	var dict DictionaryLemmatizer
	var model LemmatizerModel

	if dictionaryName != "" {
		if v, ok := defaultRegistry.lemmaDictionaries.Load(dictionaryName); ok {
			dict = v.(DictionaryLemmatizer)
		}
	}
	if modelName != "" {
		if v, ok := defaultRegistry.lemmatizerModels.Load(modelName); ok {
			model = v.(LemmatizerModel)
		}
	}
	if dict == nil && model == nil {
		panic("OpenNLPOpsFactory: at least one of dictionary or lemmatizer model must be registered")
	}
	return NewNLPLemmatizerOp(dict, model)
}

// ClearModels removes all cached models. Intended for use in tests.
func ClearModels() {
	defaultRegistry.sentenceModels.Range(func(k, _ any) bool {
		defaultRegistry.sentenceModels.Delete(k)
		return true
	})
	defaultRegistry.tokenizerModels.Range(func(k, _ any) bool {
		defaultRegistry.tokenizerModels.Delete(k)
		return true
	})
	defaultRegistry.posModels.Range(func(k, _ any) bool {
		defaultRegistry.posModels.Delete(k)
		return true
	})
	defaultRegistry.chunkerModels.Range(func(k, _ any) bool {
		defaultRegistry.chunkerModels.Delete(k)
		return true
	})
	defaultRegistry.nerModels.Range(func(k, _ any) bool {
		defaultRegistry.nerModels.Delete(k)
		return true
	})
	defaultRegistry.lemmatizerModels.Range(func(k, _ any) bool {
		defaultRegistry.lemmatizerModels.Delete(k)
		return true
	})
	defaultRegistry.lemmaDictionaries.Range(func(k, _ any) bool {
		defaultRegistry.lemmaDictionaries.Delete(k)
		return true
	})
}
