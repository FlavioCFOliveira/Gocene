// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tools

// NLPLemmatizerOp wraps a dictionary-based lemmatizer and/or a MaxEnt
// lemmatizer model to produce lemmas for each word in a sentence.
//
// Go port of org.apache.lucene.analysis.opennlp.tools.NLPLemmatizerOp
// (Apache Lucene 10.4.0).
//
// At least one of dictionaryLemmatizer or lemmatizerModel must be non-nil.
// When both are provided the dictionary is tried first; for out-of-vocabulary
// tokens (signalled by "O") the MaxEnt model is used as a fallback.
type NLPLemmatizerOp struct {
	dictionary DictionaryLemmatizer
	model      LemmatizerModel
}

// NewNLPLemmatizerOp constructs a lemmatizer op. At least one parameter must
// be non-nil; otherwise the function panics.
func NewNLPLemmatizerOp(dictionary DictionaryLemmatizer, model LemmatizerModel) *NLPLemmatizerOp {
	if dictionary == nil && model == nil {
		panic("NLPLemmatizerOp: at least one of dictionary or model must be non-nil")
	}
	return &NLPLemmatizerOp{dictionary: dictionary, model: model}
}

// Lemmatize returns lemmas for each (word, postag) pair. Unknown words are
// replaced with the original word.
func (op *NLPLemmatizerOp) Lemmatize(words, postags []string) []string {
	if op.dictionary != nil {
		lemmas := op.dictionary.Lemmatize(words, postags)
		var maxEntLemmas []string
		for i := range lemmas {
			if lemmas[i] == "O" { // word not in dictionary
				if op.model != nil {
					if maxEntLemmas == nil {
						maxEntLemmas = op.model.Lemmatize(words, postags)
					}
					if maxEntLemmas[i] == "_" {
						lemmas[i] = words[i]
					} else {
						lemmas[i] = maxEntLemmas[i]
					}
				} else {
					lemmas[i] = words[i]
				}
			}
		}
		return lemmas
	}
	// Only a MaxEnt lemmatizer.
	maxEntLemmas := op.model.Lemmatize(words, postags)
	for i := range maxEntLemmas {
		if maxEntLemmas[i] == "_" {
			maxEntLemmas[i] = words[i]
		}
	}
	return maxEntLemmas
}
