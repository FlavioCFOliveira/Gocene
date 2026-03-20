// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// GreekStemmer implements light stemming for Greek language.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.el.GreekStemmer.
type GreekStemmer struct{}

// NewGreekStemmer creates a new GreekStemmer.
func NewGreekStemmer() *GreekStemmer {
	return &GreekStemmer{}
}

// Stem performs light stemming on a Greek word.
func (s *GreekStemmer) Stem(word string) string {
	if len(word) < 4 {
		return word
	}

	runes := []rune(word)
	length := len(runes)

	// Greek has many case and suffix variations
	switch {
	// Remove common case endings
	// -ας, -ες, -ης, -ος (masculine nominative singular/plural)
	case length > 3 && (string(runes[length-3:]) == "ας" ||
		string(runes[length-3:]) == "ες" ||
		string(runes[length-3:]) == "ης" ||
		string(runes[length-3:]) == "ος"):
		return string(runes[:length-2])
	// -α, -η, -ο (neuter/feminine singular)
	case length > 2 && (string(runes[length-2:]) == "α" ||
		string(runes[length-2:]) == "η" ||
		string(runes[length-2:]) == "ο"):
		return string(runes[:length-1])
	// -ων (genitive plural)
	case length > 3 && string(runes[length-3:]) == "ων":
		return string(runes[:length-2])
	// -ου (genitive singular)
	case length > 3 && string(runes[length-2:]) == "ου":
		return string(runes[:length-2])
	// -ους (genitive plural)
	case length > 4 && string(runes[length-4:]) == "ους":
		return string(runes[:length-3])
	// -εις, -οις (dative plural)
	case length > 4 && (string(runes[length-4:]) == "εις" ||
		string(runes[length-4:]) == "οις"):
		return string(runes[:length-3])
	// -ει (dative singular)
	case length > 3 && string(runes[length-3:]) == "ει":
		return string(runes[:length-2])
	// -οντας, -ωντας (participles)
	case length > 5 && (string(runes[length-5:]) == "οντας" ||
		string(runes[length-5:]) == "ωντας"):
		return string(runes[:length-4])
	// -σα, -σε (past tense)
	case length > 3 && (string(runes[length-2:]) == "σα" ||
		string(runes[length-2:]) == "σε"):
		return string(runes[:length-2])
	// -μενος, -μενη, -μενο (passive participles)
	case length > 6 && (string(runes[length-5:]) == "μενος" ||
		string(runes[length-5:]) == "μενη" ||
		string(runes[length-5:]) == "μενο"):
		return string(runes[:length-4])
	}

	return word
}

// GreekStemFilter implements light stemming for Greek.
type GreekStemFilter struct {
	*BaseTokenFilter
	stemmer *GreekStemmer
}

// NewGreekStemFilter creates a new GreekStemFilter.
func NewGreekStemFilter(input TokenStream) *GreekStemFilter {
	return &GreekStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewGreekStemmer(),
	}
}

// IncrementToken processes the next token and applies Greek stemming.
func (f *GreekStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := f.stemmer.Stem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// GreekStemFilterFactory creates GreekStemFilter instances.
type GreekStemFilterFactory struct{}

// NewGreekStemFilterFactory creates a new GreekStemFilterFactory.
func NewGreekStemFilterFactory() *GreekStemFilterFactory {
	return &GreekStemFilterFactory{}
}

// Create creates a new GreekStemFilter.
func (f *GreekStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewGreekStemFilter(input)
}

var _ TokenFilterFactory = (*GreekStemFilterFactory)(nil)
