// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import "fmt"

// AffixedWord represents the analysis result of a simple (non-compound) word.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.AffixedWord from Apache Lucene 10.4.0.
type AffixedWord struct {
	word     string
	entry    DictEntry
	prefixes []Affix
	suffixes []Affix
}

// NewAffixedWord constructs an AffixedWord.
func NewAffixedWord(word string, entry DictEntry, prefixes, suffixes []Affix) *AffixedWord {
	return &AffixedWord{
		word:     word,
		entry:    entry,
		prefixes: prefixes,
		suffixes: suffixes,
	}
}

// GetWord returns the word being analyzed.
func (aw *AffixedWord) GetWord() string { return aw.word }

// GetDictEntry returns the dictionary entry for the stem.
func (aw *AffixedWord) GetDictEntry() DictEntry { return aw.entry }

// GetPrefixes returns the prefixes applied to the stem (outermost first).
func (aw *AffixedWord) GetPrefixes() []Affix { return aw.prefixes }

// GetSuffixes returns the suffixes applied to the stem (outermost first).
func (aw *AffixedWord) GetSuffixes() []Affix { return aw.suffixes }

func (aw *AffixedWord) String() string {
	return fmt.Sprintf("AffixedWord[word=%s, entry=%s, prefixes=%v, suffixes=%v]",
		aw.word, aw.entry, aw.prefixes, aw.suffixes)
}

// Affix represents a prefix or suffix applied to a word stem.
//
// This is the Go port of AffixedWord.Affix from Apache Lucene 10.4.0.
type Affix struct {
	AffixID         int
	PresentableFlag string
}

// NewAffix builds an Affix from a dictionary and an affix id.
func NewAffix(dictionary *Dictionary, affixID int) Affix {
	encodedFlag := dictionary.AffixData(affixID, AffixFlag)
	return Affix{
		AffixID:         affixID,
		PresentableFlag: dictionary.flagParsingStrategy.PrintFlag(encodedFlag),
	}
}

// GetFlag returns the affix flag as it appears in the *.aff file.
func (a Affix) GetFlag() string { return a.PresentableFlag }

func (a Affix) String() string { return fmt.Sprintf("%s(id=%d)", a.PresentableFlag, a.AffixID) }
