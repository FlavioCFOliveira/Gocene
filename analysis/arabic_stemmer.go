// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
)

// ArabicStemmer implements light stemming for Arabic words.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ar.ArabicStemmer.
//
// The stemmer removes common prefixes and suffixes from Arabic words.
// It is a light stemmer, meaning it only removes affixes without
// trying to find the root of the word.
type ArabicStemmer struct{}

// NewArabicStemmer creates a new ArabicStemmer.
func NewArabicStemmer() *ArabicStemmer {
	return &ArabicStemmer{}
}

// Stem performs stemming on an Arabic word.
// Returns the stemmed word.
func (s *ArabicStemmer) Stem(word string) string {
	if word == "" {
		return word
	}

	runes := []rune(word)

	// First normalize the word
	runes = s.normalize(runes)

	// Remove suffixes first (longest to shortest)
	runes = s.removeSuffixes(runes)

	// Then remove prefixes (longest to shortest)
	runes = s.removePrefixes(runes)

	return string(runes)
}

// normalize performs basic normalization on Arabic text.
func (s *ArabicStemmer) normalize(runes []rune) []rune {
	result := make([]rune, 0, len(runes))
	for _, r := range runes {
		switch r {
		// Remove Kashida/Tatweel
		case 0x0640:
			continue
		// Normalize Alef variants
		case 0x0622, 0x0623, 0x0625:
			result = append(result, 0x0627)
		// Normalize Alef Maksura to Yeh
		case 0x0649:
			result = append(result, 0x064A)
		default:
			result = append(result, r)
		}
	}
	return result
}

// removeSuffixes removes common Arabic suffixes.
func (s *ArabicStemmer) removeSuffixes(runes []rune) []rune {
	length := len(runes)
	if length < 4 {
		return runes
	}

	// Check for definite article + preposition combinations at end (possessive)
	// These are pronoun suffixes attached to nouns/verbs
	// Check longer suffixes first (3 chars)
	if length > 4 {
		suffix := string(runes[length-3:])
		switch suffix {
		case "هما", // dual possessive (his/her/their dual)
			"كما": // dual possessive (your dual)
			return runes[:length-3]
		}
	}

	// Check 2-char suffixes
	if length > 3 {
		suffix := string(runes[length-2:])
		switch suffix {
		case "هم", // masculine plural possessive
			"هن", // feminine plural possessive
			"ها", // singular feminine possessive
			"نا", // first person plural possessive
			"كم", // second person masculine plural possessive
			"كن": // second person feminine plural possessive
			return runes[:length-2]
		}
	}

	// Check 1-char suffixes (only for longer words to preserve root)
	if length > 4 {
		suffix := string(runes[length-1:])
		switch suffix {
		case "ه", // singular masculine possessive
			"ك", // second person singular possessive
			"ي", // first person singular possessive
			"ة": // ta marbuta (feminine marker)
			return runes[:length-1]
		}
	}

	return runes
}

// removePrefixes removes common Arabic prefixes.
func (s *ArabicStemmer) removePrefixes(runes []rune) []rune {
	length := len(runes)
	if length < 4 {
		return runes
	}

	// Check for 3-letter prefixes with definite article
	if length > 4 {
		prefix := string(runes[0:3])
		switch prefix {
		case "بال", // bi + al (with the)
			"فال", // fa + al (so the)
			"كال", // ka + al (like the)
			"وال": // wa + al (and the)
			return runes[3:] // Remove first 3 characters (ba/ka/fa/wa + al)
		}
	}

	// Check for definite article (ال)
	if length > 2 && runes[0] == 0x0627 && runes[1] == 0x0644 {
		// Check if it's followed by a solar or lunar letter
		// For light stemming, we remove ال prefix
		return runes[2:]
	}

	// Single letter prefixes (only if word is long enough)
	if length > 3 {
		switch runes[0] {
		case 0x0628: // ب (bi - with/in)
			return runes[1:]
		case 0x0641: // ف (fa - so/then)
			return runes[1:]
		case 0x0643: // ك (ka - like/as)
			return runes[1:]
		case 0x0644: // ل (li - for/to)
			return runes[1:]
		case 0x0648: // و (wa - and)
			return runes[1:]
		}
	}

	return runes
}

// StemSentence stems all words in a sentence.
// Words are separated by whitespace.
func (s *ArabicStemmer) StemSentence(sentence string) string {
	words := strings.Fields(sentence)
	stemmed := make([]string, len(words))
	for i, word := range words {
		stemmed[i] = s.Stem(word)
	}
	return strings.Join(stemmed, " ")
}

// ArabicStemFilter is a TokenFilter that applies Arabic stemming.
type ArabicStemFilter struct {
	*BaseTokenFilter
	stemmer *ArabicStemmer
}

// NewArabicStemFilter creates a new ArabicStemFilter.
func NewArabicStemFilter(input TokenStream) *ArabicStemFilter {
	return &ArabicStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		stemmer:         NewArabicStemmer(),
	}
}

// IncrementToken processes the next token and applies Arabic stemming.
func (f *ArabicStemFilter) IncrementToken() (bool, error) {
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

// ArabicStemFilterFactory creates ArabicStemFilter instances.
type ArabicStemFilterFactory struct{}

// NewArabicStemFilterFactory creates a new ArabicStemFilterFactory.
func NewArabicStemFilterFactory() *ArabicStemFilterFactory {
	return &ArabicStemFilterFactory{}
}

// Create creates a new ArabicStemFilter.
func (f *ArabicStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewArabicStemFilter(input)
}

// Ensure ArabicStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*ArabicStemFilterFactory)(nil)
