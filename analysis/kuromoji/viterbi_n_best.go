// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

// Search-mode penalty constants.
//
// These are the Go ports of the corresponding private constants in
// org.apache.lucene.analysis.ja.ViterbiNBest from Apache Lucene 10.4.0.
const (
	searchModeKanjiLength  = 2
	searchModeOtherLength  = 7 // must be >= searchModeKanjiLength
	searchModeKanjiPenalty = 3000
	searchModeOtherPenalty = 1700
)

// ViterbiNBest is the kuromoji-specific Viterbi decoder that supports n-best
// path calculation, search mode, and extended mode decomposition.
//
// This is the Go port of org.apache.lucene.analysis.ja.ViterbiNBest from
// Apache Lucene 10.4.0.
//
// Deviation: the Java original extends
// org.apache.lucene.analysis.morph.ViterbiNBest<Token, JaMorphData> via a
// deep generics hierarchy that depends on lattice types not yet fully ported.
// This Go implementation provides the struct with all configuration fields
// and exposes the search-mode penalty computation. Full lattice decoding is
// wired in by JapaneseTokenizer once the dictionary types are complete.
type ViterbiNBest struct {
	fst           *morph.TokenInfoFST
	userFST       *morph.TokenInfoFST
	costs         *morph.ConnectionCosts
	unkDictionary *dict.UnknownDictionary
	charDef       *morph.CharacterDefinition

	discardPunctuation bool
	searchMode         bool
	extendedMode       bool
	outputCompounds    bool

	// dictionaryMap holds the three dictionary slots indexed by TokenType.
	dictionaryMap map[morph.TokenType]dict.JaMorphData
}

// NewViterbiNBest creates a kuromoji ViterbiNBest decoder.
func NewViterbiNBest(
	fst *morph.TokenInfoFST,
	userFST *morph.TokenInfoFST,
	costs *morph.ConnectionCosts,
	unkDictionary *dict.UnknownDictionary,
	charDef *morph.CharacterDefinition,
	discardPunctuation bool,
	searchMode bool,
	extendedMode bool,
	outputCompounds bool,
) *ViterbiNBest {
	return &ViterbiNBest{
		fst:                fst,
		userFST:            userFST,
		costs:              costs,
		unkDictionary:      unkDictionary,
		charDef:            charDef,
		discardPunctuation: discardPunctuation,
		searchMode:         searchMode,
		extendedMode:       extendedMode,
		outputCompounds:    outputCompounds,
		dictionaryMap:      make(map[morph.TokenType]dict.JaMorphData),
	}
}

// RegisterDictionary associates a JaMorphData dictionary with a token type.
func (v *ViterbiNBest) RegisterDictionary(tt morph.TokenType, d dict.JaMorphData) {
	v.dictionaryMap[tt] = d
}

// ComputePenalty returns the search-mode penalty for a token of the given
// length starting at pos in buf.
//
// Kanji-only tokens longer than searchModeKanjiLength incur a per-character
// penalty; other tokens longer than searchModeOtherLength incur a smaller
// per-character penalty.
func (v *ViterbiNBest) ComputePenalty(buf []rune, pos, length int) int {
	if length > searchModeKanjiLength {
		allKanji := true
		end := pos + length
		for i := pos; i < end && i < len(buf); i++ {
			if !unicode.Is(unicode.Han, buf[i]) {
				allKanji = false
				break
			}
		}
		if allKanji {
			return (length - searchModeKanjiLength) * searchModeKanjiPenalty
		} else if length > searchModeOtherLength {
			return (length - searchModeOtherLength) * searchModeOtherPenalty
		}
	}
	return 0
}

// IsPunctuation reports whether c is a punctuation or symbol character,
// following Java's Character.getType classification used to decide whether
// tokens should be discarded when discardPunctuation is true.
func IsPunctuation(c rune) bool {
	switch {
	case unicode.Is(unicode.Z, c): // separators
		return true
	case unicode.Is(unicode.Cc, c): // control
		return true
	case unicode.Is(unicode.Cf, c): // format
		return true
	case unicode.Is(unicode.Pd, c): // dash punctuation
		return true
	case unicode.Is(unicode.Ps, c): // start punctuation
		return true
	case unicode.Is(unicode.Pe, c): // end punctuation
		return true
	case unicode.Is(unicode.Pc, c): // connector punctuation
		return true
	case unicode.Is(unicode.Po, c): // other punctuation
		return true
	case unicode.Is(unicode.Sm, c): // math symbol
		return true
	case unicode.Is(unicode.Sc, c): // currency symbol
		return true
	case unicode.Is(unicode.Sk, c): // modifier symbol
		return true
	case unicode.Is(unicode.So, c): // other symbol
		return true
	case unicode.Is(unicode.Pi, c): // initial quote punctuation
		return true
	case unicode.Is(unicode.Pf, c): // final quote punctuation
		return true
	default:
		return false
	}
}

// DiscardPunctuation reports whether this decoder discards punctuation tokens.
func (v *ViterbiNBest) DiscardPunctuation() bool { return v.discardPunctuation }

// SearchMode reports whether search-mode decomposition is active.
func (v *ViterbiNBest) SearchMode() bool { return v.searchMode }

// ExtendedMode reports whether extended-mode output is active.
func (v *ViterbiNBest) ExtendedMode() bool { return v.extendedMode }

// OutputCompounds reports whether compound tokens are emitted.
func (v *ViterbiNBest) OutputCompounds() bool { return v.outputCompounds }
