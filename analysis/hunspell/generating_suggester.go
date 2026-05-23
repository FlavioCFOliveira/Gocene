// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"container/heap"
	"math"
	"sort"
	"strings"

	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Limits for generatingSuggester (mirrors Lucene's GeneratingSuggester constants).
const (
	gsMaxRoots         = 100
	gsMaxWords         = 100
	gsMaxGuesses       = 200
	gsMaxRootLengthDif = 4
)

// generatingSuggester traverses the dictionary and applies affix rules to produce
// n-gram–scored suggestions for the given misspelled word.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.GeneratingSuggester from Apache Lucene 10.4.0.
//
// Deviation: Java uses a generic record Weighted<T> with a priority-queue.
// Go uses concrete weightedRoot / weightedString structs and a min-heap.
type generatingSuggester struct {
	dictionary *Dictionary
	speller    *Hunspell
	entryCache *SuggestibleEntryCache
}

// suggest returns up to maxNGramSuggestions suggestions for the lower-cased word.
// prevSuggestions is the already-accumulated list so duplicates can be filtered.
func (g *generatingSuggester) suggest(word string, originalCase WordCase, prevSuggestions []rawSuggestion) []string {
	roots := g.findSimilarDictionaryEntries(word, originalCase)
	expanded := g.expandRoots(word, roots)
	bySimilarity := g.rankBySimilarity(word, expanded)
	return g.getMostRelevantSuggestions(bySimilarity, prevSuggestions)
}

// ── root scoring ──────────────────────────────────────────────────────────────

type weightedRoot struct {
	word    string
	entryID int
	score   int
}

// rootMinHeap is a min-heap of weightedRoot (worst score at top = easiest to evict).
type rootMinHeap []weightedRoot

func (h rootMinHeap) Len() int { return len(h) }
func (h rootMinHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score
	}
	return h[i].word > h[j].word // reverse-lexicographic for tie-break
}
func (h rootMinHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *rootMinHeap) Push(x any)   { *h = append(*h, x.(weightedRoot)) }
func (h *rootMinHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func (g *generatingSuggester) findSimilarDictionaryEntries(word string, originalCase WordCase) []weightedRoot {
	d := g.dictionary
	ignoreTitleCase := originalCase == WordCaseLower && !d.hasLanguage("de")
	automaton := NewTrigramAutomaton(word)
	wordLen := len([]rune(word))

	h := &rootMinHeap{}
	heap.Init(h)

	caseFold := d.CaseFoldRune

	processFn := func(entry *FlyweightEntry) {
		if ignoreTitleCase && entry.HasTitleCase() {
			return
		}
		lowerRoot := entry.LowerCaseRoot(caseFold)
		sc := automaton.NgramScore(lowerRoot)
		if sc == 0 {
			return
		}
		rootRunes := entry.Root()
		sc += gsCommonPrefix(word, string(rootRunes)) - gsLongerWorsePenalty(wordLen, len(rootRunes))

		g.speller.checkCanceled()

		root := string(rootRunes)
		forms := entry.Forms()
		step := d.FormStep()
		for i := 0; i < forms.Length; i += step {
			form := forms.Ints[i]
			wr := weightedRoot{word: root, entryID: form, score: sc}
			if h.Len() < gsMaxRoots {
				heap.Push(h, wr)
			} else if top := (*h)[0]; sc > top.score || (sc == top.score && root < top.word) {
				heap.Pop(h)
				heap.Push(h, wr)
			}
		}
	}

	minLen := max(1, wordLen-gsMaxRootLengthDif)
	maxLen := wordLen + gsMaxRootLengthDif
	if g.entryCache != nil {
		g.entryCache.ProcessSuggestibleWords(minLen, maxLen, processFn)
	} else {
		d.words.ProcessSuggestibleWords(minLen, maxLen, processFn)
	}

	// convert heap to sorted slice (best first)
	result := make([]weightedRoot, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(weightedRoot)
	}
	return result
}

// ── expansion ─────────────────────────────────────────────────────────────────

type weightedString struct {
	word  string
	score int
}

func (g *generatingSuggester) expandRoots(misspelled string, roots []weightedRoot) []weightedString {
	thresh := gsCalcThreshold(misspelled)
	d := g.dictionary

	seen := make(map[string]struct{})
	var expanded []weightedString

	for _, weighted := range roots {
		for _, guess := range g.expandRoot(weighted, misspelled) {
			lower := d.toLowerCase(guess)
			sc := gsAnyMismatchNgram(len([]rune(misspelled)), misspelled, lower, false) +
				gsCommonPrefix(misspelled, guess)
			if sc > thresh {
				if _, ok := seen[guess]; !ok {
					seen[guess] = struct{}{}
					expanded = append(expanded, weightedString{word: guess, score: sc})
				}
			}
		}
	}

	// sort descending by score then lexicographic, keep MAX_GUESSES
	sort.Slice(expanded, func(i, j int) bool {
		if expanded[i].score != expanded[j].score {
			return expanded[i].score > expanded[j].score
		}
		return expanded[i].word < expanded[j].word
	})
	if len(expanded) > gsMaxGuesses {
		expanded = expanded[:gsMaxGuesses]
	}
	return expanded
}

func gsCalcThreshold(word string) int {
	runes := []rune(word)
	l := len(runes)
	thresh := 0
	for sp := 1; sp < 4; sp++ {
		mw := make([]rune, l)
		copy(mw, runes)
		for k := sp; k < l; k += 4 {
			mw[k] = '*'
		}
		thresh += gsAnyMismatchNgram(l, word, string(mw), false)
	}
	return thresh/3 - 1
}

func (g *generatingSuggester) expandRoot(root weightedRoot, misspelled string) []string {
	d := g.dictionary
	var crossProducts [][]rune
	seen := make(map[string]struct{})
	var result []string

	addResult := func(s string) {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}

	if !d.HasFlag(root.entryID, d.needaffix) {
		addResult(root.word)
	}

	wordRunes := []rune(root.word)

	// suffixes
	g.processAffixes(false, misspelled, func(suffixLength, suffixID int) {
		stripLen := g.affixStripLength(suffixID)
		if !g.hasCompatibleFlags(root, suffixID) {
			return
		}
		stemLen := len(wordRunes) - stripLen
		if stemLen < 0 {
			return
		}
		if !g.checkAffixCondition(suffixID, wordRunes, 0, stemLen) {
			return
		}
		misspelledRunes := []rune(misspelled)
		suffix := string(misspelledRunes[len(misspelledRunes)-suffixLength:])
		withSuffix := string(wordRunes[:stemLen]) + suffix
		addResult(withSuffix)
		if d.IsCrossProduct(suffixID) {
			crossProducts = append(crossProducts, []rune(withSuffix))
		}
	})

	// cross-product prefixes
	g.processAffixes(true, misspelled, func(prefixLength, prefixID int) {
		if !d.HasFlag(root.entryID, d.AffixData(prefixID, AffixFlag)) {
			return
		}
		if !d.IsCrossProduct(prefixID) {
			return
		}
		stripLen := g.affixStripLength(prefixID)
		misspelledRunes := []rune(misspelled)
		prefix := string(misspelledRunes[:prefixLength])
		for _, suffixed := range crossProducts {
			stemLen := len(suffixed) - stripLen
			if stemLen < 0 {
				continue
			}
			if g.checkAffixCondition(prefixID, suffixed, stripLen, stemLen) {
				addResult(prefix + string(suffixed[stripLen:]))
			}
		}
	})

	// pure prefixes
	g.processAffixes(true, misspelled, func(prefixLength, prefixID int) {
		stripLen := g.affixStripLength(prefixID)
		stemLen := len(wordRunes) - stripLen
		if stemLen < 0 {
			return
		}
		if g.hasCompatibleFlags(root, prefixID) && g.checkAffixCondition(prefixID, wordRunes, stripLen, stemLen) {
			misspelledRunes := []rune(misspelled)
			prefix := string(misspelledRunes[:prefixLength])
			addResult(prefix + string(wordRunes[stripLen:]))
		}
	})

	if len(result) > gsMaxWords {
		result = result[:gsMaxWords]
	}
	return result
}

type affixProcessor func(affixLength, affixID int)

func (g *generatingSuggester) processAffixes(prefixes bool, word string, processor affixProcessor) {
	d := g.dictionary
	var fst *gfst.FST[*util.IntsRef]
	if prefixes {
		fst = d.Prefixes()
	} else {
		fst = d.Suffixes()
	}
	if fst == nil {
		return
	}

	outputs := gfst.IntSequenceOutputsSingleton()
	arc := fst.GetFirstArc(&gfst.Arc[*util.IntsRef]{})
	if arc.IsFinal() {
		g.processAffixIDs(0, outputs.Add(outputs.GetNoOutput(), arc.NextFinalOutput()), processor)
	}

	reader := fst.GetBytesReader()
	output := outputs.GetNoOutput()
	wordRunes := []rune(word)
	length := len(wordRunes)

	if prefixes {
		for i := 0; i < length; i++ {
			output = NextArc(fst, arc, reader, output, int(wordRunes[i]))
			if output == nil {
				break
			}
			if arc.IsFinal() {
				affixIDs := outputs.Add(output, arc.NextFinalOutput())
				g.processAffixIDs(i+1, affixIDs, processor)
			}
		}
	} else {
		for i := length - 1; i >= 0; i-- {
			output = NextArc(fst, arc, reader, output, int(wordRunes[i]))
			if output == nil {
				break
			}
			if arc.IsFinal() {
				affixIDs := outputs.Add(output, arc.NextFinalOutput())
				g.processAffixIDs(length-i, affixIDs, processor)
			}
		}
	}
}

func (g *generatingSuggester) processAffixIDs(affixLength int, affixIDs *util.IntsRef, processor affixProcessor) {
	if affixIDs == nil {
		return
	}
	for j := 0; j < affixIDs.Length; j++ {
		processor(affixLength, affixIDs.Ints[j])
	}
}

func (g *generatingSuggester) hasCompatibleFlags(root weightedRoot, affixID int) bool {
	d := g.dictionary
	if !d.HasFlag(root.entryID, d.AffixData(affixID, AffixFlag)) {
		return false
	}
	append := d.AffixData(affixID, AffixAppend)
	appendID := int(append)
	return !d.flagLookup.HasFlag(appendID, d.needaffix) &&
		!d.flagLookup.HasFlag(appendID, d.circumfix) &&
		!d.flagLookup.HasFlag(appendID, d.onlyincompound)
}

func (g *generatingSuggester) checkAffixCondition(affixID int, word []rune, offset, length int) bool {
	if length < 0 {
		return false
	}
	condition := g.dictionary.GetAffixCondition(affixID)
	if condition == 0 {
		return true
	}
	return g.dictionary.patterns[condition].AcceptsStem(word, offset, length)
}

func (g *generatingSuggester) affixStripLength(affixID int) int {
	d := g.dictionary
	stripOrd := int(d.AffixData(affixID, AffixStripOrd))
	return d.stripOffsets[stripOrd+1] - d.stripOffsets[stripOrd]
}

// ── ranking ───────────────────────────────────────────────────────────────────

func (g *generatingSuggester) rankBySimilarity(word string, expanded []weightedString) []weightedString {
	d := g.dictionary
	fact := (10.0 - float64(d.maxDiff)) / 5.0
	var result []weightedString

	for _, ws := range expanded {
		guess := ws.word
		lower := d.toLowerCase(guess)
		if lower == word {
			result = append(result, weightedString{word: guess, score: ws.score + 2000})
			continue
		}

		re := gsAnyMismatchNgram(2, word, lower, true) + gsAnyMismatchNgram(2, lower, word, true)

		score := 2*gsLCS(word, lower) -
			intAbs(len([]rune(word))-len([]rune(lower))) +
			gsCommonCharPositionScore(word, lower) +
			gsCommonPrefix(word, lower) +
			gsAnyMismatchNgram(4, word, lower, false) +
			re
		if float64(re) < float64(len([]rune(word))+len([]rune(lower)))*fact {
			score -= 1000
		}
		result = append(result, weightedString{word: guess, score: score})
	}

	// sort descending by score, then lexicographic
	sort.Slice(result, func(i, j int) bool {
		if result[i].score != result[j].score {
			return result[i].score > result[j].score
		}
		return result[i].word < result[j].word
	})
	return result
}

func (g *generatingSuggester) getMostRelevantSuggestions(bySimilarity []weightedString, prevSuggestions []rawSuggestion) []string {
	d := g.dictionary
	var result []string
	hasExcellent := false

	for _, ws := range bySimilarity {
		if ws.score > 1000 {
			hasExcellent = true
		} else if hasExcellent {
			break
		}

		bad := ws.score < -100
		if bad && (!isEmpty(result) || d.onlyMaxDiff) {
			break
		}

		// skip if already in previous suggestions (substring) or in current result
		skipWord := false
		for _, prev := range prevSuggestions {
			if strings.Contains(ws.word, prev.raw) {
				skipWord = true
				break
			}
		}
		if !skipWord {
			for _, r := range result {
				if strings.Contains(ws.word, r) {
					skipWord = true
					break
				}
			}
		}
		if !skipWord && g.speller.checkWordStr(ws.word) {
			result = append(result, ws.word)
			if len(result) >= d.maxNGramSuggestions {
				break
			}
		}
		if bad {
			break
		}
	}
	return result
}

func isEmpty(s []string) bool { return len(s) == 0 }

// ── scoring helpers (mirrors static methods in Lucene's GeneratingSuggester) ──

// gsCommonPrefix returns the length of the common prefix of s1 and s2 (rune-based).
func gsCommonPrefix(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	i := 0
	limit := len(r1)
	if len(r2) < limit {
		limit = len(r2)
	}
	for i < limit && r1[i] == r2[i] {
		i++
	}
	return i
}

// gsLongerWorsePenalty implements NGRAM_LONGER_WORSE: penalty for l2 > l1.
func gsLongerWorsePenalty(l1, l2 int) int {
	return max(l2-l1-2, 0)
}

// gsAnyMismatchNgram implements NGRAM_ANY_MISMATCH ngram scoring.
func gsAnyMismatchNgram(n int, s1, s2 string, weighted bool) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	return gsNgramScore(n, r1, r2, weighted) - max(intAbs(len(r2)-len(r1))-2, 0)
}

// gsNgramScore computes an n-gram overlap score between s1 and s2.
func gsNgramScore(n int, s1, s2 []rune, weighted bool) int {
	l1 := len(s1)
	score := 0
	lastStarts := make([]int, l1)
	for j := 1; j <= n; j++ {
		ns := 0
		for i := 0; i <= l1-j; i++ {
			if lastStarts[i] >= 0 {
				pos := gsIndexOfSubrune(s2, lastStarts[i], s1, i, j)
				lastStarts[i] = pos
				if pos >= 0 {
					ns++
					continue
				}
			}
			if weighted {
				ns--
				if i == 0 || i == l1-j {
					ns-- // side weight
				}
			}
		}
		score += ns
		if ns < 2 && !weighted {
			break
		}
	}
	return score
}

// gsIndexOfSubrune finds the first occurrence of needle[needlePos:needlePos+len] in
// haystack starting at haystackPos.
func gsIndexOfSubrune(haystack []rune, haystackPos int, needle []rune, needlePos, length int) int {
	if length == 0 {
		return haystackPos
	}
	c := needle[needlePos]
	limit := len(haystack) - length
	for i := haystackPos; i <= limit; i++ {
		if haystack[i] == c {
			match := true
			for k := 1; k < length; k++ {
				if haystack[i+k] != needle[needlePos+k] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}

// gsLCS computes the length of the longest common subsequence of s1 and s2.
func gsLCS(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	lengths := make([]int, len(r2)+1)
	for i := 1; i <= len(r1); i++ {
		prev := 0
		for j := 1; j <= len(r2); j++ {
			cur := lengths[j]
			if r1[i-1] == r2[j-1] {
				lengths[j] = prev + 1
			} else if lengths[j] < lengths[j-1] {
				lengths[j] = lengths[j-1]
			}
			prev = cur
		}
	}
	return lengths[len(r2)]
}

// gsCommonCharPositionScore scores common characters at the same position.
func gsCommonCharPositionScore(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	num := 0
	diffPos1 := -1
	diffPos2 := -1
	diff := 0
	limit := len(r1)
	if len(r2) < limit {
		limit = len(r2)
	}
	for i := 0; i < limit; i++ {
		if r1[i] == r2[i] {
			num++
		} else {
			if diff == 0 {
				diffPos1 = i
			} else if diff == 1 {
				diffPos2 = i
			}
			diff++
		}
	}
	commonScore := 0
	if num > 0 {
		commonScore = 1
	}
	if diff == 2 && limit == len(r1) && limit == len(r2) &&
		r1[diffPos1] == r2[diffPos2] && r1[diffPos2] == r2[diffPos1] {
		return commonScore + 10
	}
	return commonScore
}

func intAbs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Ensure math is used (imported for potential future use in float comparison).
var _ = math.MaxFloat64
