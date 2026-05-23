// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"strings"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// Stemmer uses the affix rules declared in the Dictionary to generate one or
// more stems for a word.  It conforms to the algorithm in the original Hunspell
// implementation, including recursive suffix stripping.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.Stemmer from Apache Lucene 10.4.0.
type Stemmer struct {
	dictionary *Dictionary
	formStep   int
}

// NewStemmer constructs a new Stemmer for the given Dictionary.
func NewStemmer(dictionary *Dictionary) *Stemmer {
	return &Stemmer{
		dictionary: dictionary,
		formStep:   dictionary.FormStep(),
	}
}

// Stem finds the stem(s) of the provided word (string form).
func (s *Stemmer) Stem(word string) []string {
	return s.StemRunes([]rune(word), len([]rune(word)))
}

// StemRunes finds the stem(s) of the provided rune slice.
func (s *Stemmer) StemRunes(word []rune, length int) []string {
	var list []string
	s.Analyze(word, length, func(stem []rune, _, morphDataID, _, _, _, _ int) bool {
		list = append(list, s.newStem(stem, morphDataID))
		return true
	})
	return list
}

// UniqueStems returns deduplicated stems.
func (s *Stemmer) UniqueStems(word []rune, length int) []string {
	stems := s.StemRunes(word, length)
	if len(stems) < 2 {
		return stems
	}
	seen := make(map[string]struct{}, len(stems))
	var deduped []string
	for _, st := range stems {
		key := st
		if s.dictionary.IgnoreCase {
			key = strings.ToLower(st)
		}
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			deduped = append(deduped, st)
		}
	}
	return deduped
}

// RootProcessor is the callback for each root found during analysis.
// Parameters: stem runes, formID, morphDataID, outerPrefix, innerPrefix,
// outerSuffix, innerSuffix (all affix ids, -1 if absent).
// Returns false to stop processing.
type RootProcessor func(stem []rune, formID, morphDataID, outerPrefix, innerPrefix, outerSuffix, innerSuffix int) bool

// Analyze calls processor for each analysis of the word.
func (s *Stemmer) Analyze(word []rune, length int, processor RootProcessor) {
	d := s.dictionary
	if d.MayNeedInputCleaning() {
		input := string(word[:length])
		if d.NeedsInputCleaning(input) {
			var sb strings.Builder
			cleaned := d.CleanInputString(input, &sb)
			word = []rune(cleaned)
			length = len(word)
		}
	}
	if length == 0 {
		return
	}

	if !s.doStem(word, 0, length, WordContextSimpleWord, processor) {
		return
	}

	wordCase := s.caseOf(word, length)
	if wordCase == WordCaseUpper || wordCase == WordCaseTitle {
		s.varyCase(word, length, wordCase, func(variant []rune, varLen int, _ WordCase) bool {
			return s.doStem(variant, 0, varLen, WordContextSimpleWord, processor)
		})
	}
}

func (s *Stemmer) caseOf(word []rune, length int) WordCase {
	d := s.dictionary
	if d.IgnoreCase || length == 0 || unicode.IsLower(word[0]) {
		return WordCaseMixed
	}
	return CaseOfRunes(word, length)
}

type caseVariationProcessor func(word []rune, length int, originalCase WordCase) bool

func (s *Stemmer) varyCase(word []rune, length int, wordCase WordCase, processor caseVariationProcessor) bool {
	d := s.dictionary
	var titleBuffer []rune
	if wordCase == WordCaseUpper {
		titleBuffer = s.caseFoldTitle(word, length)

		aposCase := capitalizeAfterApostrophe(titleBuffer, length)
		if aposCase != nil {
			if !processor(aposCase, length, wordCase) {
				return false
			}
		}
		if !processor(titleBuffer, length, wordCase) {
			return false
		}
		if d.CheckSharpS {
			if !s.varySharpS(titleBuffer, length, processor) {
				return false
			}
		}
	}

	if d.IsDotICaseChangeDisallowed(word) {
		return true
	}

	src := titleBuffer
	if src == nil {
		src = word
	}
	lowerBuffer := s.caseFoldLower(src, length)
	if !processor(lowerBuffer, length, wordCase) {
		return false
	}
	if wordCase == WordCaseUpper && d.CheckSharpS {
		if !s.varySharpS(lowerBuffer, length, processor) {
			return false
		}
	}
	return true
}

func (s *Stemmer) caseFoldTitle(word []rune, length int) []rune {
	buf := make([]rune, length)
	copy(buf, word[:length])
	for i := 1; i < length; i++ {
		buf[i] = s.dictionary.CaseFoldRune(buf[i])
	}
	return buf
}

func (s *Stemmer) caseFoldLower(word []rune, length int) []rune {
	buf := make([]rune, length)
	copy(buf, word[:length])
	buf[0] = s.dictionary.CaseFoldRune(buf[0])
	return buf
}

func capitalizeAfterApostrophe(word []rune, length int) []rune {
	for i := 1; i < length-1; i++ {
		if word[i] == '\'' {
			next := word[i+1]
			upper := unicode.ToUpper(next)
			if upper != next {
				cp := make([]rune, length)
				copy(cp, word[:length])
				cp[i+1] = upper
				return cp
			}
		}
	}
	return nil
}

func (s *Stemmer) varySharpS(word []rune, length int, processor caseVariationProcessor) bool {
	src := string(word[:length])
	variants := replaceSharpS(src, 0)
	if variants == nil {
		return true
	}
	for _, v := range variants {
		if v != src {
			vr := []rune(v)
			if !processor(vr, len(vr), WordCaseMixed) {
				return false
			}
		}
	}
	return true
}

func replaceSharpS(word string, depth int) []string {
	if depth > 5 {
		return []string{word}
	}
	// find "ss"
	for i := 0; i < len([]rune(word))-1; i++ {
		r := []rune(word)
		if r[i] == 's' && r[i+1] == 's' {
			prefix := string(r[:i])
			tails := replaceSharpS(string(r[i+2:]), depth+1)
			if tails == nil {
				tails = []string{string(r[i+2:])}
			}
			var result []string
			for _, tail := range tails {
				result = append(result, prefix+"ss"+tail)
				result = append(result, prefix+"ß"+tail)
			}
			return result
		}
	}
	return nil
}

// doStem performs the core stemming search.
func (s *Stemmer) doStem(word []rune, offset, length int, context WordContext, processor RootProcessor) bool {
	d := s.dictionary
	forms := d.LookupWord(word, offset, length)
	if forms != nil {
		for i := 0; i < forms.Length; i += s.formStep {
			entryID := forms.Ints[forms.Offset+i]
			if d.HasFlag(entryID, d.needaffix) {
				continue
			}
			if (context == WordContextCompoundBegin || context == WordContextCompoundMiddle) &&
				d.HasFlag(entryID, d.compoundForbid) {
				return false
			}
			if !s.isRootCompatibleWithContext(context, -1, entryID) {
				continue
			}
			morphID := s.morphDataID(forms, i)
			if !processor(word[offset:offset+length], entryID, morphID, -1, -1, -1, -1) {
				return false
			}
		}
	}

	stemProc := &stemCandidateProc{
		context: context,
		process: func(stemWord []rune, stemOffset, stemLen, lastAffix, outerPrefix, innerPrefix, outerSuffix, innerSuffix int) bool {
			sf := d.LookupWord(stemWord, stemOffset, stemLen)
			if sf == nil {
				return true
			}
			flag := d.AffixData(lastAffix, AffixFlag)
			prefixID := innerPrefix
			if prefixID < 0 {
				prefixID = outerPrefix
			}
			for i := 0; i < sf.Length; i += s.formStep {
				entryID := sf.Ints[sf.Offset+i]
				if d.HasFlag(entryID, flag) || d.IsFlagAppendedByAffix(prefixID, flag) {
					if innerPrefix < 0 && outerPrefix >= 0 {
						prefixFlag := d.AffixData(outerPrefix, AffixFlag)
						if !d.HasFlag(entryID, prefixFlag) && !d.IsFlagAppendedByAffix(lastAffix, prefixFlag) {
							continue
						}
					}
					if !s.isRootCompatibleWithContext(context, lastAffix, entryID) {
						continue
					}
					morphID := s.morphDataID(sf, i)
					if !processor(stemWord[stemOffset:stemOffset+stemLen], entryID, morphID, outerPrefix, innerPrefix, outerSuffix, innerSuffix) {
						return false
					}
				}
			}
			return true
		},
	}
	return s.removeAffixes(word, offset, length, true, -1, -1, -1, stemProc)
}

// removeAffixes tries to strip prefixes and suffixes from word, calling the
// processor for each candidate stem found.
func (s *Stemmer) removeAffixes(
	word []rune, offset, length int, doPrefix bool,
	outerPrefix, innerPrefix, outerSuffix int,
	processor *stemCandidateProc,
) bool {
	d := s.dictionary

	if doPrefix && d.prefixes != nil {
		fst := d.prefixes
		br := fst.GetBytesReader()
		arc := fst.GetFirstArc(new(gfst.Arc[*util.IntsRef]))
		output := fst.Outputs().GetNoOutput()
		limit := length
		if d.fullStrip {
			limit = length + 1
		}
		for i := 0; i < limit; i++ {
			if i > 0 {
				output = NextArc(fst, arc, br, output, int(word[offset+i-1]))
				if output == nil {
					break
				}
			}
			if !arc.IsFinal() {
				continue
			}
			prefixIDs := fst.Outputs().Add(output, arc.NextFinalOutput())
			for j := 0; j < prefixIDs.Length; j++ {
				prefix := prefixIDs.Ints[prefixIDs.Offset+j]
				if prefix == outerPrefix {
					continue
				}
				if s.isAffixCompatible(prefix, true, outerPrefix, outerSuffix, processor.context) {
					stripped := s.stripAffix(word, offset, length, i, prefix, true)
					if stripped == nil {
						continue
					}
					pure := len(stripped) == len(word) // reference equality approximation
					var sOff, sLen int
					if pure {
						sOff = offset + i
						sLen = length - i
					} else {
						sOff = 0
						sLen = len(stripped)
					}
					if !s.applyAffix(stripped, sOff, sLen, prefix, true, outerPrefix, innerPrefix, outerSuffix, processor) {
						return false
					}
				}
			}
		}
	}

	if d.suffixes != nil {
		fst := d.suffixes
		br := fst.GetBytesReader()
		arc := fst.GetFirstArc(new(gfst.Arc[*util.IntsRef]))
		output := fst.Outputs().GetNoOutput()
		limit := 1
		if d.fullStrip {
			limit = 0
		}
		for i := length; i >= limit; i-- {
			if i < length {
				output = NextArc(fst, arc, br, output, int(word[offset+i]))
				if output == nil {
					break
				}
			}
			if !arc.IsFinal() {
				continue
			}
			suffixIDs := fst.Outputs().Add(output, arc.NextFinalOutput())
			for j := 0; j < suffixIDs.Length; j++ {
				suffix := suffixIDs.Ints[suffixIDs.Offset+j]
				if suffix == outerSuffix {
					continue
				}
				if s.isAffixCompatible(suffix, false, outerPrefix, outerSuffix, processor.context) {
					stripped := s.stripAffix(word, offset, length, length-i, suffix, false)
					if stripped == nil {
						continue
					}
					pure := len(stripped) == len(word)
					var sOff, sLen int
					if pure {
						sOff = offset
						sLen = i
					} else {
						sOff = 0
						sLen = len(stripped)
					}
					if !s.applyAffix(stripped, sOff, sLen, suffix, false, outerPrefix, innerPrefix, outerSuffix, processor) {
						return false
					}
				}
			}
		}
	}
	return true
}

// stripAffix strips the affix from word, returning nil if conditions aren't met.
func (s *Stemmer) stripAffix(word []rune, offset, length, affixLen, affix int, isPrefix bool) []rune {
	d := s.dictionary
	deAffixedLen := length - affixLen

	stripOrd := int(d.AffixData(affix, AffixStripOrd))
	stripStart := d.stripOffsets[stripOrd]
	stripEnd := d.stripOffsets[stripOrd+1]
	stripLen := stripEnd - stripStart

	if stripLen+deAffixedLen == 0 {
		return nil
	}

	condition := d.GetAffixCondition(affix)
	if condition != 0 {
		deAffixedOffset := offset + affixLen
		if !isPrefix {
			deAffixedOffset = offset
		}
		if !d.patterns[condition].AcceptsStem(word, deAffixedOffset, deAffixedLen) {
			return nil
		}
	}

	if stripLen == 0 {
		return word
	}

	stripped := make([]rune, stripLen+deAffixedLen)
	if isPrefix {
		copy(stripped[stripLen:], word[offset+affixLen:offset+length])
		copy(stripped[:stripLen], d.stripData[stripStart:stripEnd])
	} else {
		copy(stripped[:deAffixedLen], word[offset:offset+deAffixedLen])
		copy(stripped[deAffixedLen:], d.stripData[stripStart:stripEnd])
	}
	return stripped
}

func (s *Stemmer) isAffixCompatible(affix int, isPrefix bool, outerPrefix, outerSuffix int, context WordContext) bool {
	d := s.dictionary
	append_ := int(d.AffixData(affix, AffixAppend))

	previousWasPrefix := outerSuffix < 0 && outerPrefix >= 0
	if context.IsCompound() {
		if !isPrefix && d.HasFlag(append_, d.compoundForbid) {
			return false
		}
		if !context.IsAffixAllowedWithoutSpecialPermit(isPrefix) && !d.HasFlag(append_, d.compoundPermit) {
			return false
		}
		if context == WordContextCompoundEnd && !isPrefix && !previousWasPrefix && d.HasFlag(append_, d.onlyincompound) {
			return false
		}
	} else if d.HasFlag(append_, d.onlyincompound) {
		return false
	}

	if outerPrefix == -1 && outerSuffix == -1 {
		return true
	}

	if d.IsCrossProduct(affix) {
		if previousWasPrefix {
			return true
		}
		if outerSuffix >= 0 {
			prevFlag := d.AffixData(outerSuffix, AffixFlag)
			return d.HasFlag(append_, prevFlag)
		}
	}
	return false
}

func (s *Stemmer) applyAffix(
	word []rune, offset, length, affix int, prefix bool,
	outerPrefix, innerPrefix, outerSuffix int,
	processor *stemCandidateProc,
) bool {
	d := s.dictionary
	prefixID := innerPrefix
	if prefixID < 0 {
		prefixID = outerPrefix
	}
	previousAffix := outerSuffix
	if previousAffix < 0 {
		previousAffix = prefixID
	}

	innerSuffix := -1
	if prefix {
		if outerPrefix < 0 {
			outerPrefix = affix
		} else {
			innerPrefix = affix
		}
	} else {
		if outerSuffix < 0 {
			outerSuffix = affix
		} else {
			innerSuffix = affix
		}
	}

	skipLookup := s.needsAnotherAffix(affix, previousAffix, !prefix, prefixID)
	if !skipLookup {
		if !processor.process(word, offset, length, affix, outerPrefix, innerPrefix, outerSuffix, innerSuffix) {
			return false
		}
	}

	if innerSuffix >= 0 {
		return true
	}

	recursionDepth := 0
	if outerSuffix >= 0 {
		recursionDepth++
	}
	if innerPrefix >= 0 {
		recursionDepth += 2
	} else if outerPrefix >= 0 {
		recursionDepth++
	}
	recursionDepth--

	if d.IsCrossProduct(affix) && recursionDepth <= 1 {
		flag := d.AffixData(affix, AffixFlag)
		var doPrefix bool
		if recursionDepth == 0 {
			if prefix {
				doPrefix = d.complexPrefixes && d.IsSecondStagePrefix(flag)
			} else if !d.complexPrefixes && d.IsSecondStageSuffix(flag) {
				doPrefix = false
			} else {
				return true
			}
		} else {
			if prefix && d.complexPrefixes {
				doPrefix = true
			} else if prefix || d.complexPrefixes || !d.IsSecondStageSuffix(flag) {
				return true
			} else {
				doPrefix = false
			}
		}
		return s.removeAffixes(word, offset, length, doPrefix, outerPrefix, innerPrefix, outerSuffix, processor)
	}
	return true
}

func (s *Stemmer) isRootCompatibleWithContext(context WordContext, lastAffix, entryID int) bool {
	d := s.dictionary
	if !context.IsCompound() && d.HasFlag(entryID, d.onlyincompound) {
		return false
	}
	if context.IsCompound() && context != WordContextCompoundRuleEnd {
		cFlag := context.RequiredFlag(d)
		return d.HasFlag(entryID, cFlag) ||
			d.IsFlagAppendedByAffix(lastAffix, cFlag) ||
			d.HasFlag(entryID, d.compoundFlag) ||
			d.IsFlagAppendedByAffix(lastAffix, d.compoundFlag)
	}
	return true
}

func (s *Stemmer) morphDataID(forms *util.IntsRef, i int) int {
	if s.dictionary.hasCustomMorphData {
		return forms.Ints[forms.Offset+i+1]
	}
	return 0
}

func (s *Stemmer) needsAnotherAffix(affix, previousAffix int, isSuffix bool, prefixID int) bool {
	d := s.dictionary
	circumfix := d.circumfix
	if isSuffix {
		if d.IsFlagAppendedByAffix(prefixID, circumfix) != d.IsFlagAppendedByAffix(affix, circumfix) {
			return true
		}
	}
	if d.IsFlagAppendedByAffix(affix, d.needaffix) {
		return !isSuffix || previousAffix < 0 || d.IsFlagAppendedByAffix(previousAffix, d.needaffix)
	}
	return false
}

func (s *Stemmer) stemException(morphDataID int) string {
	morphData := s.dictionary.morphData
	if morphDataID > 0 && morphDataID < len(morphData) {
		data := morphData[morphDataID]
		start := 0
		if strings.HasPrefix(data, "st:") {
			start = 0
		} else {
			idx := strings.Index(data, " st:")
			if idx < 0 {
				return ""
			}
			start = idx + 1
		}
		// data[start:] starts with "st:"
		sub := data[start+3:]
		end := strings.IndexByte(sub, ' ')
		if end < 0 {
			return sub
		}
		return sub[:end]
	}
	return ""
}

func (s *Stemmer) newStem(stem []rune, morphDataID int) string {
	exception := s.stemException(morphDataID)

	d := s.dictionary
	if d.oconv != nil {
		var sb strings.Builder
		if exception != "" {
			sb.WriteString(exception)
		} else {
			sb.WriteString(string(stem))
		}
		d.oconv.ApplyMappings(&sb)
		if d.IgnoreCase {
			result := []rune(sb.String())
			for i, r := range result {
				result[i] = d.CaseFoldRune(r)
			}
			return string(result)
		}
		return sb.String()
	}
	if exception != "" {
		return exception
	}
	return string(stem)
}

// ─── stemCandidateProc ────────────────────────────────────────────────────────

type stemCandidateProc struct {
	context WordContext
	process func(word []rune, offset, length, lastAffix, outerPrefix, innerPrefix, outerSuffix, innerSuffix int) bool
}
