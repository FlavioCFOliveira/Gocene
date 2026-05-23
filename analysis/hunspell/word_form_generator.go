// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"container/heap"
	"sort"
	"strings"

	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// WordFormGenerator generates possible word forms by adding affixes to stems, and suggests
// dictionary entries (stems + flags) that can generate a requested list of words.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.WordFormGenerator from Apache Lucene 10.4.0.
//
// Deviation: Java uses HPPC's CharHashSet for flag sets; Go uses map[rune]struct{}.
// Deviation: Java uses an IntsRefFSTEnum to traverse the affix FSTs; we replicate
// the same traversal via IntsRefFSTEnum from the fst package.
type WordFormGenerator struct {
	dictionary *Dictionary
	affixes    map[rune][]*affixEntry // flag → list of affix entries
	stemmer    *Stemmer
}

// affixEntry holds the decoded data for one affix (prefix or suffix).
type affixEntry struct {
	id        int
	flag      rune
	kind      AffixKind
	affix     string // the replacement string written to the word
	strip     string // the strip string removed from the stem
	condition *AffixCondition
}

// NewWordFormGenerator builds a WordFormGenerator for the given dictionary.
func NewWordFormGenerator(dictionary *Dictionary) *WordFormGenerator {
	wfg := &WordFormGenerator{
		dictionary: dictionary,
		affixes:    make(map[rune][]*affixEntry),
		stemmer:    NewStemmer(dictionary),
	}
	wfg.fillAffixMap(dictionary.prefixes, AffixKindPrefix)
	wfg.fillAffixMap(dictionary.suffixes, AffixKindSuffix)
	return wfg
}

func (wfg *WordFormGenerator) fillAffixMap(fst *gfst.FST[*util.IntsRef], kind AffixKind) {
	if fst == nil {
		return
	}
	enum, err := gfst.NewIntsRefFSTEnum[*util.IntsRef](fst)
	if err != nil {
		return
	}
	for {
		io, err := enum.Next()
		if err != nil || io == nil {
			break
		}
		affixIDs := io.Output
		for j := 0; j < affixIDs.Length; j++ {
			id := affixIDs.Ints[j]
			flag := wfg.dictionary.AffixData(id, AffixFlag)
			entry := &affixEntry{
				id:        id,
				flag:      flag,
				kind:      kind,
				affix:     wfg.affixString(kind, io.Input),
				strip:     wfg.stripString(id),
				condition: wfg.affixCondition(id),
			}
			wfg.affixes[flag] = append(wfg.affixes[flag], entry)
		}
	}
}

func (wfg *WordFormGenerator) affixString(kind AffixKind, input *util.IntsRef) string {
	runes := make([]rune, input.Length)
	for i := 0; i < input.Length; i++ {
		if kind == AffixKindPrefix {
			runes[i] = rune(input.Ints[i])
		} else {
			runes[input.Length-1-i] = rune(input.Ints[i])
		}
	}
	return string(runes)
}

func (wfg *WordFormGenerator) stripString(affixID int) string {
	d := wfg.dictionary
	stripOrd := int(d.AffixData(affixID, AffixStripOrd))
	start := d.stripOffsets[stripOrd]
	end := d.stripOffsets[stripOrd+1]
	return string(d.stripData[start:end])
}

func (wfg *WordFormGenerator) affixCondition(affixID int) *AffixCondition {
	cond := wfg.dictionary.GetAffixCondition(affixID)
	if cond == 0 {
		return alwaysTrueCond
	}
	return wfg.dictionary.patterns[cond]
}

// ── Public API ────────────────────────────────────────────────────────────────

// GetAllWordFormsFromDictionary generates all word forms for all dictionary entries with
// the given root word. Equivalent to Lucene's getAllWordForms(String, Runnable).
func (wfg *WordFormGenerator) GetAllWordFormsFromDictionary(root string, checkCanceled func()) []*AffixedWord {
	var result []*AffixedWord
	entries := wfg.dictionary.LookupEntries(root)
	for _, entry := range entries {
		result = append(result, wfg.GetAllWordForms(root, entry.GetFlags(), checkCanceled)...)
	}
	return result
}

// GetAllWordForms generates all word forms for the given stem pretending it has the given
// flags (in the same format as the dictionary uses).
func (wfg *WordFormGenerator) GetAllWordForms(stem, flags string, checkCanceled func()) []*AffixedWord {
	d := wfg.dictionary
	encodedFlags := d.flagParsingStrategy.ParseUtfFlags(flags)
	if !wfg.shouldConsiderAtAll(encodedFlags) {
		return nil
	}
	entry := NewDictEntryFromData(stem, flags, "")
	return wfg.getAllWordForms(entry, encodedFlags, checkCanceled)
}

func (wfg *WordFormGenerator) getAllWordForms(entry DictEntry, encodedFlags []rune, checkCanceled func()) []*AffixedWord {
	encodedFlags = sortAndDeduplicateFlags(encodedFlags)
	var result []*AffixedWord
	bare := NewAffixedWord(entry.GetStem(), entry, nil, nil)
	checkCanceled()
	if !HasFlagInSortedArray(wfg.dictionary.needaffix, encodedFlags, 0, len(encodedFlags)) {
		result = append(result, bare)
	}
	result = append(result, wfg.expand(bare, encodedFlags, checkCanceled)...)
	return result
}

// CanStemToOriginal checks whether the affixed word can be stemmed back to its
// dictionary entry. Override to skip this verification.
func (wfg *WordFormGenerator) CanStemToOriginal(derived *AffixedWord) bool {
	word := derived.GetWord()
	wordRunes := []rune(word)
	if wfg.isForbiddenWord(wordRunes, 0, len(wordRunes)) {
		return false
	}
	stem := derived.GetDictEntry().GetStem()
	foundStem := false
	foundForbidden := false
	wfg.stemmer.removeAffixes(wordRunes, 0, len(wordRunes), true, -1, -1, -1,
		&stemCandidateProc{
			context: WordContextSimpleWord,
			process: func(chars []rune, offset, length, lastAffix, outerPrefix, innerPrefix, outerSuffix, innerSuffix int) bool {
				if wfg.isForbiddenWord(chars, offset, length) {
					foundForbidden = true
					return false
				}
				if length == len([]rune(stem)) && string(chars[offset:offset+length]) == stem {
					foundStem = true
					return false
				}
				return !foundStem
			},
		})
	return foundStem && !foundForbidden
}

func (wfg *WordFormGenerator) isForbiddenWord(chars []rune, offset, length int) bool {
	d := wfg.dictionary
	if d.forbiddenword == flagUnset {
		return false
	}
	forms := d.LookupWord(chars, offset, length)
	if forms == nil {
		return false
	}
	step := d.FormStep()
	for i := 0; i < forms.Length; i += step {
		if d.HasFlag(forms.Ints[i], d.forbiddenword) {
			return true
		}
	}
	return false
}

func (wfg *WordFormGenerator) expand(stem *AffixedWord, flags []rune, checkCanceled func()) []*AffixedWord {
	var result []*AffixedWord
	for _, flag := range flags {
		entries := wfg.affixes[flag]
		if len(entries) == 0 {
			continue
		}
		kind := entries[0].kind
		if !wfg.isCompatibleWithPreviousAffixes(stem, kind, flag) {
			continue
		}
		for _, affix := range entries {
			checkCanceled()
			derived := affix.apply(stem, wfg.dictionary)
			if derived == nil {
				continue
			}
			appendFlags := wfg.appendFlags(affix)
			if !wfg.shouldConsiderAtAll(appendFlags) {
				continue
			}
			if wfg.CanStemToOriginal(derived) {
				result = append(result, derived)
			}
			if wfg.dictionary.IsCrossProduct(affix.id) {
				updated := updateFlags(flags, flag, appendFlags)
				result = append(result, wfg.expand(derived, updated, checkCanceled)...)
			}
		}
	}
	return result
}

func (wfg *WordFormGenerator) shouldConsiderAtAll(flags []rune) bool {
	d := wfg.dictionary
	for _, f := range flags {
		if f == d.compoundBegin || f == d.compoundMiddle || f == d.compoundEnd ||
			f == d.forbiddenword || f == d.onlyincompound {
			return false
		}
	}
	return true
}

func (wfg *WordFormGenerator) isCompatibleWithPreviousAffixes(stem *AffixedWord, kind AffixKind, flag rune) bool {
	isPrefix := kind == AffixKindPrefix
	var sameAffixes []Affix
	if isPrefix {
		sameAffixes = stem.GetPrefixes()
	} else {
		sameAffixes = stem.GetSuffixes()
	}
	size := len(sameAffixes)
	if size == 2 {
		return false
	}
	if isPrefix && size == 1 && !wfg.dictionary.complexPrefixes {
		return false
	}
	if !isPrefix && len(stem.GetPrefixes()) > 0 {
		return false
	}
	if size == 1 && !wfg.dictionary.IsFlagAppendedByAffix(sameAffixes[0].AffixID, flag) {
		return false
	}
	return true
}

func (wfg *WordFormGenerator) appendFlags(affix *affixEntry) []rune {
	appendID := wfg.dictionary.AffixData(affix.id, AffixAppend)
	if appendID == 0 {
		return nil
	}
	return wfg.dictionary.flagLookup.GetFlags(int(appendID))
}

// GenerateAllSimpleWords calls consumer for every derived word form in the dictionary.
func (wfg *WordFormGenerator) GenerateAllSimpleWords(consumer func(*AffixedWord), checkCanceled func()) {
	d := wfg.dictionary
	d.words.processAllWords(1, int(^uint(0)>>1), false, func(entry *FlyweightEntry) {
		rootStr := string(entry.Root())
		forms := entry.Forms()
		step := d.FormStep()
		for i := 0; i < forms.Length; i += step {
			form := forms.Ints[i]
			encodedFlags := d.flagLookup.GetFlags(form)
			if wfg.shouldConsiderAtAll(encodedFlags) {
				flagStr := d.flagParsingStrategy.PrintFlags(encodedFlags)
				de := NewDictEntryFromData(rootStr, flagStr, "")
				for _, aw := range wfg.getAllWordForms(de, encodedFlags, checkCanceled) {
					consumer(aw)
				}
			}
		}
	})
}

// Compress suggests dictionary entries (stems + flags) to generate the given list of words.
// Returns nil if nothing can be generated.
func (wfg *WordFormGenerator) Compress(words []string, forbidden map[string]struct{}, checkCanceled func()) *EntrySuggestion {
	if len(words) == 0 {
		return nil
	}
	for _, w := range words {
		if _, ok := forbidden[w]; ok {
			panic("words and forbidden must not intersect")
		}
	}
	return newWordCompressor(wfg, words, forbidden, checkCanceled).compress()
}

// ── affixEntry.apply ─────────────────────────────────────────────────────────

func (a *affixEntry) apply(stem *AffixedWord, d *Dictionary) *AffixedWord {
	word := stem.GetWord()
	isPrefix := a.kind == AffixKindPrefix
	if isPrefix {
		if !strings.HasPrefix(word, a.strip) {
			return nil
		}
	} else {
		if !strings.HasSuffix(word, a.strip) {
			return nil
		}
	}
	stripped := ""
	if isPrefix {
		stripped = word[len(a.strip):]
	} else {
		stripped = word[:len(word)-len(a.strip)]
	}
	if !a.condition.AcceptsStemString(stripped) {
		return nil
	}
	var applied string
	if isPrefix {
		applied = a.affix + stripped
	} else {
		applied = stripped + a.affix
	}
	prefixes := append([]Affix(nil), stem.GetPrefixes()...)
	suffixes := append([]Affix(nil), stem.GetSuffixes()...)
	newAffix := NewAffix(d, a.id)
	if isPrefix {
		prefixes = append([]Affix{newAffix}, prefixes...)
	} else {
		suffixes = append([]Affix{newAffix}, suffixes...)
	}
	return NewAffixedWord(applied, stem.GetDictEntry(), prefixes, suffixes)
}

// ── flag helpers ──────────────────────────────────────────────────────────────

func sortAndDeduplicateFlags(flags []rune) []rune {
	if len(flags) == 0 {
		return flags
	}
	sort.Slice(flags, func(i, j int) bool { return flags[i] < flags[j] })
	out := flags[:1]
	for i := 1; i < len(flags); i++ {
		if flags[i] != flags[i-1] {
			out = append(out, flags[i])
		}
	}
	return out
}

func updateFlags(flags []rune, toRemove rune, toAppend []rune) []rune {
	d := make(map[rune]struct{}, len(flags)+len(toAppend))
	for _, f := range flags {
		if f != toRemove {
			d[f] = struct{}{}
		}
	}
	for _, f := range toAppend {
		d[f] = struct{}{}
	}
	out := make([]rune, 0, len(d))
	for f := range d {
		out = append(out, f)
	}
	return sortAndDeduplicateFlags(out)
}

// ── WordCompressor ────────────────────────────────────────────────────────────

// wordCompressor holds state for the Compress algorithm.
type wordCompressor struct {
	wfg            *WordFormGenerator
	forbidden      map[string]struct{}
	checkCanceled  func()
	wordSet        map[string]struct{}
	existingStems  map[string]struct{}
	stemToPossible map[string][]flagSet // stem → possible flag combinations
	stemsToForms   map[string][]string  // stem → words it can generate
	cache          map[stemWithFlags][]string
}

type flagSet struct {
	flags map[rune]struct{}
}

func newFlagSet(flags ...rune) flagSet {
	m := make(map[rune]struct{}, len(flags))
	for _, f := range flags {
		m[f] = struct{}{}
	}
	return flagSet{flags: m}
}

func (fs flagSet) toRunes() []rune {
	out := make([]rune, 0, len(fs.flags))
	for f := range fs.flags {
		out = append(out, f)
	}
	return sortAndDeduplicateFlags(out)
}

func mergeFlagSets(sets []flagSet) map[rune]struct{} {
	merged := make(map[rune]struct{})
	for _, fs := range sets {
		for f := range fs.flags {
			merged[f] = struct{}{}
		}
	}
	return merged
}

type stemWithFlags struct {
	stem  string
	flags string // sorted string representation of flags (for map key)
}

type compressorState struct {
	stemToFlags       map[string][]flagSet
	underGenerated    int
	overGenerated     int
	potentialCoverage int
}

// compressorStateHeap is a max-heap by fitness (most potential coverage first,
// then fewest stems, then fewest under/over).
type compressorStateHeap []compressorState

func (h compressorStateHeap) Len() int { return len(h) }
func (h compressorStateHeap) Less(i, j int) bool {
	a, b := h[i], h[j]
	if a.potentialCoverage != b.potentialCoverage {
		return a.potentialCoverage > b.potentialCoverage
	}
	if len(a.stemToFlags) != len(b.stemToFlags) {
		return len(a.stemToFlags) < len(b.stemToFlags)
	}
	if a.underGenerated != b.underGenerated {
		return a.underGenerated < b.underGenerated
	}
	return a.overGenerated < b.overGenerated
}
func (h compressorStateHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *compressorStateHeap) Push(x any)   { *h = append(*h, x.(compressorState)) }
func (h *compressorStateHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func newWordCompressor(wfg *WordFormGenerator, words []string, forbidden map[string]struct{}, checkCanceled func()) *wordCompressor {
	wc := &wordCompressor{
		wfg:            wfg,
		forbidden:      forbidden,
		checkCanceled:  checkCanceled,
		wordSet:        make(map[string]struct{}, len(words)),
		existingStems:  make(map[string]struct{}),
		stemToPossible: make(map[string][]flagSet),
		stemsToForms:   make(map[string][]string),
		cache:          make(map[stemWithFlags][]string),
	}
	for _, w := range words {
		wc.wordSet[w] = struct{}{}
	}
	for _, word := range words {
		checkCanceled()
		// The word itself is always a potential stem
		wc.registerStemForm(word, word)
		if _, already := wc.stemToPossible[word]; !already {
			wc.stemToPossible[word] = nil
		}
		// Collect affix combinations that yield this word from some stem
		wc.wfg.stemmer.removeAffixes([]rune(word), 0, len([]rune(word)), true, -1, -1, -1,
			&stemCandidateProc{
				context: WordContextSimpleWord,
				process: func(chars []rune, offset, length, lastAffix, outerPrefix, innerPrefix, outerSuffix, innerSuffix int) bool {
					candidate := string(chars[offset : offset+length])
					var fs flagSet
					flags := make(map[rune]struct{})
					for _, idx := range []int{outerPrefix, innerPrefix, outerSuffix, innerSuffix} {
						if idx >= 0 {
							flags[wfg.dictionary.AffixData(idx, AffixFlag)] = struct{}{}
						}
					}
					fs = flagSet{flags: flags}
					swf := stemWithFlags{stem: candidate, flags: flagSetKey(fs)}
					checkForbidden := func() bool {
						generated := wc.allGeneratedFromSWF(stemWithFlags{stem: candidate, flags: flagSetKey(flagSet{flags: flags})})
						for _, g := range generated {
							if _, ok := forbidden[g]; ok {
								return true
							}
						}
						return false
					}
					if len(forbidden) == 0 || !checkForbidden() {
						wc.registerStemForm(candidate, word)
						wc.stemToPossible[candidate] = append(wc.stemToPossible[candidate], fs)
						_ = swf
					}
					return true
				},
			})
	}
	for stem := range wc.stemsToForms {
		if wfg.dictionary.LookupEntries(stem) != nil {
			wc.existingStems[stem] = struct{}{}
		}
	}
	return wc
}

func (wc *wordCompressor) registerStemForm(stem, word string) {
	if _, ok := wc.stemsToForms[stem]; !ok {
		wc.stemsToForms[stem] = nil
	}
	// avoid duplicates
	for _, f := range wc.stemsToForms[stem] {
		if f == word {
			return
		}
	}
	wc.stemsToForms[stem] = append(wc.stemsToForms[stem], word)
}

func flagSetKey(fs flagSet) string {
	runes := fs.toRunes()
	return string(runes)
}

func (wc *wordCompressor) allGeneratedFromSWF(swf stemWithFlags) []string {
	if cached, ok := wc.cache[swf]; ok {
		return cached
	}
	fs := flagSet{flags: make(map[rune]struct{})}
	for _, r := range []rune(swf.flags) {
		fs.flags[r] = struct{}{}
	}
	flagStr := wc.wfg.dictionary.flagParsingStrategy.PrintFlags(fs.toRunes())
	words := wc.wfg.GetAllWordForms(swf.stem, flagStr, wc.checkCanceled)
	result := make([]string, 0, len(words))
	for _, aw := range words {
		result = append(result, aw.GetWord())
	}
	wc.cache[swf] = result
	return result
}

func (wc *wordCompressor) compress() *EntrySuggestion {
	// Sort stems: existing first, then by number of forms (most first)
	stems := make([]string, 0, len(wc.stemsToForms))
	for s := range wc.stemsToForms {
		stems = append(stems, s)
	}
	sort.Slice(stems, func(i, j int) bool {
		_, ei := wc.existingStems[stems[i]]
		_, ej := wc.existingStems[stems[j]]
		if ei != ej {
			return ei // existing before non-existing
		}
		li := len(wc.stemsToForms[stems[i]])
		lj := len(wc.stemsToForms[stems[j]])
		return li > lj
	})

	initial := compressorState{
		stemToFlags:       make(map[string][]flagSet),
		underGenerated:    len(wc.wordSet),
		overGenerated:     0,
		potentialCoverage: 0,
	}

	pq := &compressorStateHeap{initial}
	heap.Init(pq)
	visited := make(map[string]struct{})

	var best *compressorState

	for pq.Len() > 0 {
		state := heap.Pop(pq).(compressorState)
		if state.underGenerated == 0 {
			best = &state
			break
		}

		for _, stem := range stems {
			if _, already := state.stemToFlags[stem]; !already {
				next := wc.addStem(state, stem)
				key := stateKey(next.stemToFlags)
				if _, seen := visited[key]; !seen {
					visited[key] = struct{}{}
					ns := wc.buildState(next.stemToFlags)
					if ns != nil && (state.underGenerated > ns.underGenerated || ns.potentialCoverage > state.potentialCoverage) {
						heap.Push(pq, *ns)
					}
				}
			}
		}

		if state.potentialCoverage < len(wc.wordSet) {
			continue
		}

		for stem, flagSets := range state.stemToFlags {
			for _, fs := range wc.stemToPossible[stem] {
				found := false
				for _, existing := range flagSets {
					if flagSetKey(existing) == flagSetKey(fs) {
						found = true
						break
					}
				}
				if !found {
					next := wc.addFlags(state, stem, fs)
					key := stateKey(next.stemToFlags)
					if _, seen := visited[key]; !seen {
						visited[key] = struct{}{}
						ns := wc.buildState(next.stemToFlags)
						if ns != nil && state.underGenerated > ns.underGenerated {
							heap.Push(pq, *ns)
						}
					}
				}
			}
		}
	}

	if best == nil {
		return nil
	}
	return wc.toSuggestion(*best)
}

func (wc *wordCompressor) addStem(state compressorState, stem string) compressorState {
	stf := make(map[string][]flagSet, len(state.stemToFlags)+1)
	for k, v := range state.stemToFlags {
		stf[k] = v
	}
	stf[stem] = nil
	return compressorState{stemToFlags: stf}
}

func (wc *wordCompressor) addFlags(state compressorState, stem string, fs flagSet) compressorState {
	stf := make(map[string][]flagSet, len(state.stemToFlags))
	for k, v := range state.stemToFlags {
		stf[k] = append([]flagSet(nil), v...)
	}
	stf[stem] = append(stf[stem], fs)
	return compressorState{stemToFlags: stf}
}

func (wc *wordCompressor) buildState(stemToFlags map[string][]flagSet) *compressorState {
	allGenerated := make(map[string]struct{})
	for stem, flagSets := range stemToFlags {
		merged := mergeFlagSets(flagSets)
		swf := stemWithFlags{stem: stem, flags: string(sortAndDeduplicateFlags(func() []rune {
			r := make([]rune, 0, len(merged))
			for f := range merged {
				r = append(r, f)
			}
			return r
		}()))}
		for _, w := range wc.allGeneratedFromSWF(swf) {
			if _, ok := wc.forbidden[w]; ok {
				return nil // would generate a forbidden word
			}
			allGenerated[w] = struct{}{}
		}
	}
	over := 0
	for w := range allGenerated {
		if _, ok := wc.wordSet[w]; !ok {
			over++
		}
	}
	under := 0
	for w := range wc.wordSet {
		if _, ok := allGenerated[w]; !ok {
			under++
		}
	}
	// potential coverage: how many requested forms could be covered by these stems
	potCovSet := make(map[string]struct{})
	for stem := range stemToFlags {
		for _, form := range wc.stemsToForms[stem] {
			potCovSet[form] = struct{}{}
		}
	}
	potCov := 0
	for f := range potCovSet {
		if _, ok := wc.wordSet[f]; ok {
			potCov++
		}
	}
	return &compressorState{
		stemToFlags:       stemToFlags,
		underGenerated:    under,
		overGenerated:     over,
		potentialCoverage: potCov,
	}
}

func stateKey(stemToFlags map[string][]flagSet) string {
	stems := make([]string, 0, len(stemToFlags))
	for s := range stemToFlags {
		stems = append(stems, s)
	}
	sort.Strings(stems)
	var sb strings.Builder
	for _, s := range stems {
		sb.WriteString(s)
		sb.WriteByte(':')
		flagSets := stemToFlags[s]
		keys := make([]string, 0, len(flagSets))
		for _, fs := range flagSets {
			keys = append(keys, flagSetKey(fs))
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(k)
			sb.WriteByte(';')
		}
		sb.WriteByte('|')
	}
	return sb.String()
}

func (wc *wordCompressor) toSuggestion(state compressorState) *EntrySuggestion {
	var toEdit, toAdd []DictEntry
	for stem, flagSets := range state.stemToFlags {
		merged := mergeFlagSets(flagSets)
		flagStr := wc.wfg.dictionary.flagParsingStrategy.PrintFlags(sortAndDeduplicateFlags(func() []rune {
			r := make([]rune, 0, len(merged))
			for f := range merged {
				r = append(r, f)
			}
			return r
		}()))
		entry := NewDictEntryFromData(stem, flagStr, "")
		if _, ok := wc.existingStems[stem]; ok {
			toEdit = append(toEdit, entry)
		} else {
			toAdd = append(toAdd, entry)
		}
	}

	// compute over-generated words
	allGen := wc.allGeneratedFromState(state)
	var extra []string
	for _, w := range allGen {
		if _, inWordSet := wc.wordSet[w]; inWordSet {
			continue
		}
		if _, isExisting := wc.existingStems[w]; isExisting {
			continue
		}
		if _, isForbidden := wc.forbidden[w]; isForbidden {
			d := wc.wfg.dictionary
			if d.forbiddenword != flagUnset {
				forbiddenFlagStr := d.flagParsingStrategy.PrintFlags([]rune{d.forbiddenword})
				toEdit = append(toEdit, NewDictEntryFromData(w, forbiddenFlagStr, ""))
			}
		} else {
			extra = append(extra, w)
		}
	}
	sort.Strings(extra)
	return NewEntrySuggestion(toEdit, toAdd, extra)
}

func (wc *wordCompressor) allGeneratedFromState(state compressorState) []string {
	seen := make(map[string]struct{})
	for stem, flagSets := range state.stemToFlags {
		merged := mergeFlagSets(flagSets)
		swf := stemWithFlags{stem: stem, flags: string(sortAndDeduplicateFlags(func() []rune {
			r := make([]rune, 0, len(merged))
			for f := range merged {
				r = append(r, f)
			}
			return r
		}()))}
		for _, w := range wc.allGeneratedFromSWF(swf) {
			seen[w] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for w := range seen {
		result = append(result, w)
	}
	return result
}
