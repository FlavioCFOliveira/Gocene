// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import "strings"

// POSType is the type of a Korean morphological token.
//
// This is the Go port of org.apache.lucene.analysis.ko.POS.Type from Apache
// Lucene 10.4.0.
type POSType int

const (
	// POSTypeMorpheme is a simple morpheme.
	POSTypeMorpheme POSType = iota
	// POSTypeCompound is a compound noun.
	POSTypeCompound
	// POSTypeInflect is an inflected token.
	POSTypeInflect
	// POSTypePreanalysis is a pre-analysis token.
	POSTypePreanalysis
)

// String returns the name of the POS type.
func (t POSType) String() string {
	switch t {
	case POSTypeMorpheme:
		return "MORPHEME"
	case POSTypeCompound:
		return "COMPOUND"
	case POSTypeInflect:
		return "INFLECT"
	case POSTypePreanalysis:
		return "PREANALYSIS"
	default:
		return "MORPHEME"
	}
}

// POSTag is a part-of-speech tag for Korean based on Sejong corpus
// classification.
//
// This is the Go port of org.apache.lucene.analysis.ko.POS.Tag from Apache
// Lucene 10.4.0.
type POSTag int

// Korean POS tags ordered by their Lucene ordinal so that ResolveTagByByte
// works correctly.
const (
	POSTagEP      POSTag = iota // 0  Pre-final ending
	POSTagEF                    // 1  Sentence-closing ending
	POSTagEC                    // 2  Connective ending
	POSTagETN                   // 3  Nominal transformative ending
	POSTagETM                   // 4  Adnominal form transformative ending
	POSTagIC                    // 5  Interjection
	POSTagJKS                   // 6  Subject case marker
	POSTagJKC                   // 7  Complement case marker
	POSTagJKG                   // 8  Adnominal case marker
	POSTagJKO                   // 9  Object case marker
	POSTagJKB                   // 10 Adverbial case marker
	POSTagJKV                   // 11 Vocative case marker
	POSTagJKQ                   // 12 Quotative case marker
	POSTagJX                    // 13 Auxiliary postpositional particle
	POSTagJC                    // 14 Conjunctive postpositional particle
	POSTagMAG                   // 15 General Adverb
	POSTagMAJ                   // 16 Conjunctive adverb
	POSTagMM                    // 17 Modifier
	POSTagNNG                   // 18 General Noun
	POSTagNNP                   // 19 Proper Noun
	POSTagNNB                   // 20 Dependent noun (following nouns)
	POSTagNNBC                  // 21 Dependent noun
	POSTagNP                    // 22 Pronoun
	POSTagNR                    // 23 Numeral
	POSTagSF                    // 24 Terminal punctuation
	POSTagSH                    // 25 Chinese character
	POSTagSL                    // 26 Foreign language
	POSTagSN                    // 27 Number
	POSTagSP                    // 28 Space
	POSTagSSC                   // 29 Closing brackets
	POSTagSSO                   // 30 Opening brackets
	POSTagSC                    // 31 Separator
	POSTagSY                    // 32 Other symbol
	POSTagSE                    // 33 Ellipsis
	POSTagVA                    // 34 Adjective
	POSTagVCN                   // 35 Negative designator
	POSTagVCP                   // 36 Positive designator
	POSTagVV                    // 37 Verb
	POSTagVX                    // 38 Auxiliary Verb or Adjective
	POSTagXPN                   // 39 Prefix
	POSTagXR                    // 40 Root
	POSTagXSA                   // 41 Adjective Suffix
	POSTagXSN                   // 42 Noun Suffix
	POSTagXSV                   // 43 Verb Suffix
	POSTagUNKNOWN               // 44 Unknown
	POSTagUNA                   // 45 Unknown
	POSTagNA                    // 46 Unknown
	POSTagVSV                   // 47 Unknown
)

// posTagNames maps ordinal to name, matching Java enum declaration order.
var posTagNames = []string{
	"EP", "EF", "EC", "ETN", "ETM",
	"IC",
	"JKS", "JKC", "JKG", "JKO", "JKB", "JKV", "JKQ", "JX", "JC",
	"MAG", "MAJ", "MM",
	"NNG", "NNP", "NNB", "NNBC", "NP", "NR",
	"SF", "SH", "SL", "SN", "SP", "SSC", "SSO", "SC", "SY", "SE",
	"VA", "VCN", "VCP", "VV", "VX",
	"XPN", "XR", "XSA", "XSN", "XSV",
	"UNKNOWN", "UNA", "NA", "VSV",
}

// posTagDescriptions maps ordinal to description.
var posTagDescriptions = []string{
	"Pre-final ending",
	"Sentence-closing ending",
	"Connective ending",
	"Nominal transformative ending",
	"Adnominal form transformative ending",
	"Interjection",
	"Subject case marker",
	"Complement case marker",
	"Adnominal case marker",
	"Object case marker",
	"Adverbial case marker",
	"Vocative case marker",
	"Quotative case marker",
	"Auxiliary postpositional particle",
	"Conjunctive postpositional particle",
	"General Adverb",
	"Conjunctive adverb",
	"Modifier",
	"General Noun",
	"Proper Noun",
	"Dependent noun",
	"Dependent noun",
	"Pronoun",
	"Numeral",
	"Terminal punctuation",
	"Chinese Characeter",
	"Foreign language",
	"Number",
	"Space",
	"Closing brackets",
	"Opening brackets",
	"Separator",
	"Other symbol",
	"Ellipsis",
	"Adjective",
	"Negative designator",
	"Positive designator",
	"Verb",
	"Auxiliary Verb or Adjective",
	"Prefix",
	"Root",
	"Adjective Suffix",
	"Noun Suffix",
	"Verb Suffix",
	"Unknown",
	"Unknown",
	"Unknown",
	"Unknown",
}

// posTagByName maps tag name to POSTag.
var posTagByName map[string]POSTag

func init() {
	posTagByName = make(map[string]POSTag, len(posTagNames))
	for i, name := range posTagNames {
		posTagByName[name] = POSTag(i)
	}
}

// String returns the name of the POS tag.
func (t POSTag) String() string {
	if int(t) < len(posTagNames) {
		return posTagNames[t]
	}
	return "UNKNOWN"
}

// Description returns the human-readable description of the POS tag.
func (t POSTag) Description() string {
	if int(t) < len(posTagDescriptions) {
		return posTagDescriptions[t]
	}
	return "Unknown"
}

// ResolveTagByName returns the POSTag for the given tag name.
func ResolveTagByName(name string) POSTag {
	if tag, ok := posTagByName[strings.ToUpper(name)]; ok {
		return tag
	}
	return POSTagUNKNOWN
}

// ResolveTagByByte returns the POSTag for the given byte ordinal.
func ResolveTagByByte(tag byte) POSTag {
	if int(tag) < len(posTagNames) {
		return POSTag(tag)
	}
	return POSTagUNKNOWN
}

// ResolveTypeByName returns the POSType for the given type name.
func ResolveTypeByName(name string) POSType {
	switch strings.ToUpper(name) {
	case "*":
		return POSTypeMorpheme
	case "MORPHEME":
		return POSTypeMorpheme
	case "COMPOUND":
		return POSTypeCompound
	case "INFLECT":
		return POSTypeInflect
	case "PREANALYSIS":
		return POSTypePreanalysis
	default:
		return POSTypeMorpheme
	}
}

// ResolveTypeByByte returns the POSType for the given byte ordinal.
func ResolveTypeByByte(t byte) POSType {
	switch t {
	case 0:
		return POSTypeMorpheme
	case 1:
		return POSTypeCompound
	case 2:
		return POSTypeInflect
	case 3:
		return POSTypePreanalysis
	default:
		return POSTypeMorpheme
	}
}

// Morpheme is a morpheme extracted from a compound token.
//
// This is the Go port of the KoMorphData.Morpheme record from Apache Lucene
// 10.4.0.
type Morpheme struct {
	// PosTag is the part-of-speech tag of this morpheme.
	PosTag POSTag
	// SurfaceForm is the surface form of this morpheme.
	SurfaceForm string
}
