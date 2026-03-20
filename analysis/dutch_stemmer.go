// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// DutchStemmer implements the Porter stemming algorithm for Dutch language.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.nl.DutchStemmer.
type DutchStemmer struct{}

// NewDutchStemmer creates a new DutchStemmer.
func NewDutchStemmer() *DutchStemmer {
	return &DutchStemmer{}
}

// Stem performs stemming on a Dutch word.
func (s *DutchStemmer) Stem(word string) string {
	if len(word) < 3 {
		return word
	}

	runes := []rune(word)
	length := len(runes)

	// Remove common Dutch suffixes
	switch {
	// -heden, -heid (abstract noun suffixes)
	case length > 5 && string(runes[length-5:]) == "heden":
		return string(runes[:length-3])
	case length > 4 && string(runes[length-4:]) == "heid":
		return string(runes[:length-2])
	// -ingen, -ing (verbal noun suffixes)
	case length > 5 && string(runes[length-5:]) == "ingen":
		return string(runes[:length-4])
	case length > 3 && string(runes[length-3:]) == "ing":
		return string(runes[:length-2])
	// -ende, -ende (present participle/participle)
	case length > 4 && string(runes[length-4:]) == "ende":
		return string(runes[:length-2])
	// -en (infinitive/plural suffix)
	case length > 4 && runes[length-1] == 'n' && runes[length-2] == 'e':
		return string(runes[:length-2])
	// -te, -ten, -t (past tense suffixes)
	case length > 3 && runes[length-1] == 'e' && runes[length-2] == 't':
		return string(runes[:length-2])
	case length > 4 && string(runes[length-3:]) == "ten":
		return string(runes[:length-3])
	// -s (plural) - only for longer words
	case length > 4 && runes[length-1] == 's':
		return string(runes[:length-1])
	// -isch (adjective suffix)
	case length > 4 && string(runes[length-4:]) == "isch":
		return string(runes[:length-3])
	// -lijk (adjective suffix)
	case length > 4 && string(runes[length-4:]) == "lijk":
		return string(runes[:length-3])
	// -baar (adjective suffix)
	case length > 4 && string(runes[length-4:]) == "baar":
		return string(runes[:length-2])
	// -achtig (adjective suffix)
	case length > 6 && string(runes[length-6:]) == "achtig":
		return string(runes[:length-4])
	}

	return word
}

// DutchStemFilter implements light stemming for Dutch.
type DutchStemFilter struct {
	*BaseTokenFilter
	stemmer *DutchStemmer
}

// NewDutchStemFilter creates a new DutchStemFilter.
func NewDutchStemFilter(input TokenStream) *DutchStemFilter {
	return &DutchStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewDutchStemmer(),
	}
}

// IncrementToken processes the next token and applies Dutch stemming.
func (f *DutchStemFilter) IncrementToken() (bool, error) {
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

// DutchStemFilterFactory creates DutchStemFilter instances.
type DutchStemFilterFactory struct{}

// NewDutchStemFilterFactory creates a new DutchStemFilterFactory.
func NewDutchStemFilterFactory() *DutchStemFilterFactory {
	return &DutchStemFilterFactory{}
}

// Create creates a new DutchStemFilter.
func (f *DutchStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewDutchStemFilter(input)
}

var _ TokenFilterFactory = (*DutchStemFilterFactory)(nil)
