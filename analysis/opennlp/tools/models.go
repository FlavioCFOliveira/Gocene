// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package tools provides OpenNLP NLP operation wrappers used by the
// analysis/opennlp package.
//
// Go port of org.apache.lucene.analysis.opennlp.tools (Apache Lucene 10.4.0).
//
// Deviation: The Java implementation depends on the OpenNLP library
// (opennlp.tools.*). Go has no CGO-free equivalent. All NLP model types
// and the Span type are modelled as Go interfaces so that callers can
// provide concrete implementations backed by any NLP engine.
package tools

// Span represents a character span [Start, End) in a string, equivalent to
// opennlp.tools.util.Span.
type Span struct {
	Start int
	End   int
}

// Length returns the length of the span (End - Start).
func (s Span) Length() int {
	return s.End - s.Start
}

// SentenceModel is the model interface used by NLPSentenceDetectorOp.
// Implement this interface to supply an actual sentence detection model.
type SentenceModel interface {
	// SplitSentences splits the text into sentence spans.
	SplitSentences(text string) []Span
}

// TokenizerModel is the model interface used by NLPTokenizerOp.
// Implement this interface to supply an actual tokenization model.
type TokenizerModel interface {
	// TokenizePos returns the token spans within sentence.
	TokenizePos(sentence string) []Span
}

// POSModel is the model interface used by NLPPOSTaggerOp.
// Implement this interface to supply an actual POS tagging model.
type POSModel interface {
	// Tag returns POS tags for the given words.
	Tag(words []string) []string
}

// ChunkerModel is the model interface used by NLPChunkerOp.
// Implement this interface to supply an actual chunking model.
type ChunkerModel interface {
	// Chunk returns chunk tags for the given words and their POS tags.
	Chunk(words, tags []string) []string
	// Probs fills probs with the probability of each chunk tag.
	Probs(probs []float64)
}

// TokenNameFinderModel is the model interface used by NLPNERTaggerOp.
// Implement this interface to supply an actual NER model.
type TokenNameFinderModel interface {
	// Find returns named-entity spans in the given words.
	Find(words []string) []Span
	// ClearAdaptiveData clears the adaptive data after each document.
	ClearAdaptiveData()
}

// LemmatizerModel is the model interface used by NLPLemmatizerOp.
// Implement this interface to supply a MaxEnt lemmatizer model.
type LemmatizerModel interface {
	// Lemmatize returns lemmas for the given words and POS tags.
	Lemmatize(words, postags []string) []string
}

// DictionaryLemmatizer is the interface for a dictionary-based lemmatizer.
// Implement this to supply a tab-separated word/lemma/POS dictionary.
type DictionaryLemmatizer interface {
	// Lemmatize returns lemmas; unknown tokens are represented as "O".
	Lemmatize(words, postags []string) []string
}
