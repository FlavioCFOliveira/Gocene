// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "github.com/FlavioCFOliveira/Gocene/analysis/morph"

// Character class constants for kuromoji.
//
// These are the Go ports of the CharacterClass enum ordinals from
// org.apache.lucene.analysis.ja.dict.CharacterDefinition in Apache Lucene
// 10.4.0.
const (
	CharClassNGRAM       byte = iota // 0
	CharClassDEFAULT                 // 1
	CharClassSPACE                   // 2
	CharClassSYMBOL                  // 3
	CharClassNUMERIC                 // 4
	CharClassALPHA                   // 5
	CharClassCYRILLIC                // 6
	CharClassGREEK                   // 7
	CharClassHIRAGANA                // 8
	CharClassKATAKANA                // 9
	CharClassKANJI                   // 10
	CharClassKANJINUMERIC            // 11
	CharClassCount       = 12
)

// CharacterDefinition is the kuromoji-specific character category definition
// that extends the morph base and adds kanji detection.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.CharacterDefinition from Apache Lucene
// 10.4.0.
type CharacterDefinition struct {
	morph.CharacterDefinition
}

// NewCharacterDefinition creates a CharacterDefinition wrapping the given
// morph base.
func NewCharacterDefinition(base morph.CharacterDefinition) *CharacterDefinition {
	return &CharacterDefinition{CharacterDefinition: base}
}

// NewCharacterDefinitionEmpty creates a zero-value CharacterDefinition.
func NewCharacterDefinitionEmpty() *CharacterDefinition {
	return &CharacterDefinition{}
}

// IsKanji reports whether c belongs to the KANJI or KANJINUMERIC character
// class.
func (cd *CharacterDefinition) IsKanji(c rune) bool {
	cls := cd.CharacterClass(c)
	return cls == CharClassKANJI || cls == CharClassKANJINUMERIC
}

// LookupCharacterClass returns the byte class ID for a character class name.
func LookupCharacterClass(name string) byte {
	switch name {
	case "NGRAM":
		return CharClassNGRAM
	case "DEFAULT":
		return CharClassDEFAULT
	case "SPACE":
		return CharClassSPACE
	case "SYMBOL":
		return CharClassSYMBOL
	case "NUMERIC":
		return CharClassNUMERIC
	case "ALPHA":
		return CharClassALPHA
	case "CYRILLIC":
		return CharClassCYRILLIC
	case "GREEK":
		return CharClassGREEK
	case "HIRAGANA":
		return CharClassHIRAGANA
	case "KATAKANA":
		return CharClassKATAKANA
	case "KANJI":
		return CharClassKANJI
	case "KANJINUMERIC":
		return CharClassKANJINUMERIC
	default:
		return CharClassDEFAULT
	}
}
