// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

// WordContext identifies the position of a root word in a compound.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.WordContext from Apache Lucene 10.4.0.
type WordContext int

const (
	// WordContextSimpleWord is a non-compound word.
	WordContextSimpleWord WordContext = iota
	// WordContextCompoundBegin is the first root in a COMPOUNDFLAG/BEGIN/MIDDLE/END compound.
	WordContextCompoundBegin
	// WordContextCompoundMiddle is a middle root in a compound.
	WordContextCompoundMiddle
	// WordContextCompoundEnd is the final root in a COMPOUNDFLAG/BEGIN/MIDDLE/END compound.
	WordContextCompoundEnd
	// WordContextCompoundRuleEnd is the final root in a COMPOUNDRULE compound.
	// Unlike CompoundEnd it does not require COMPOUNDFLAG/COMPOUNDEND flags but
	// allows ONLYINCOMPOUND.
	WordContextCompoundRuleEnd
)

// IsCompound reports whether this context is inside a compound word.
func (wc WordContext) IsCompound() bool {
	return wc != WordContextSimpleWord
}

// IsAffixAllowedWithoutSpecialPermit reports whether an affix of the given
// kind is allowed without special permission in this compound position.
func (wc WordContext) IsAffixAllowedWithoutSpecialPermit(isPrefix bool) bool {
	if isPrefix {
		return wc == WordContextCompoundBegin
	}
	return wc == WordContextCompoundEnd || wc == WordContextCompoundRuleEnd
}

// RequiredFlag returns the dictionary flag required for a root to appear in
// this compound position, or flagUnset if no such flag is required.
// d may be nil; in that case flagUnset is always returned.
func (wc WordContext) RequiredFlag(d *Dictionary) rune {
	if d == nil {
		return flagUnset
	}
	switch wc {
	case WordContextCompoundBegin:
		return d.compoundBegin
	case WordContextCompoundMiddle:
		return d.compoundMiddle
	case WordContextCompoundEnd:
		return d.compoundEnd
	default:
		return flagUnset
	}
}

func (wc WordContext) String() string {
	switch wc {
	case WordContextSimpleWord:
		return "SIMPLE_WORD"
	case WordContextCompoundBegin:
		return "COMPOUND_BEGIN"
	case WordContextCompoundMiddle:
		return "COMPOUND_MIDDLE"
	case WordContextCompoundEnd:
		return "COMPOUND_END"
	case WordContextCompoundRuleEnd:
		return "COMPOUND_RULE_END"
	default:
		return "UNKNOWN"
	}
}
