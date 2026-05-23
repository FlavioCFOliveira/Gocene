// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "github.com/FlavioCFOliveira/Gocene/analysis/morph"

// CharacterClass byte constants for Korean (Nori).
//
// These are the Go ports of the CharacterClass enum ordinals from
// org.apache.lucene.analysis.ko.dict.CharacterDefinition in Apache Lucene
// 10.4.0.
const (
	CharClassNGRAM        byte = iota // 0
	CharClassDEFAULT                  // 1
	CharClassSPACE                    // 2
	CharClassSYMBOL                   // 3
	CharClassNUMERIC                  // 4
	CharClassALPHA                    // 5
	CharClassCYRILLIC                 // 6
	CharClassGREEK                    // 7
	CharClassHIRAGANA                 // 8
	CharClassKATAKANA                 // 9
	CharClassKANJI                    // 10
	CharClassHANGUL                   // 11
	CharClassHANJA                    // 12
	CharClassHANJANUMERIC             // 13
	CharClassCount        = 14
)

// CharacterDefinition is the Korean-specific character category definition
// that extends the morph base.
//
// This is the Go port of org.apache.lucene.analysis.ko.dict.CharacterDefinition
// from Apache Lucene 10.4.0.
//
// Deviation: the Java original loads binary resources from the JAR classpath.
// The Go port provides the type with constructor; resource loading is deferred
// to the nori codec sprint. A singleton holding a zero-value instance is
// provided so that callers that require a non-nil *CharacterDefinition compile.
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

// IsHanja reports whether c belongs to the HANJA or HANJANUMERIC character
// class.
func (cd *CharacterDefinition) IsHanja(c rune) bool {
	cls := cd.CharacterClass(c)
	return cls == CharClassHANJA || cls == CharClassHANJANUMERIC
}

// IsHangul reports whether c belongs to the HANGUL character class.
func (cd *CharacterDefinition) IsHangul(c rune) bool {
	return cd.CharacterClass(c) == CharClassHANGUL
}

// HasCoda reports whether the Hangul syllable ch has a coda (final consonant).
func HasCoda(ch rune) bool {
	return ((ch - 0xAC00) % 0x001C) != 0
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
	case "HANGUL":
		return CharClassHANGUL
	case "HANJA":
		return CharClassHANJA
	case "HANJANUMERIC":
		return CharClassHANJANUMERIC
	default:
		return CharClassDEFAULT
	}
}

// defaultCharacterDefinition is the zero-value singleton.
var defaultCharacterDefinition = NewCharacterDefinitionEmpty()

// GetInstance returns the default CharacterDefinition singleton.
//
// Deviation: the Java original loads binary data from the JAR classpath. The
// Go port returns a zero-value instance; full binary loading is deferred to
// the nori codec sprint.
func GetCharacterDefinitionInstance() *CharacterDefinition {
	return defaultCharacterDefinition
}
