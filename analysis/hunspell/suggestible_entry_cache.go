// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import "github.com/FlavioCFOliveira/Gocene/util"

// SuggestibleEntryCache provides CPU-cache-friendlier iteration over
// WordStorage entries that can be used for suggestions.  Words and form data
// are stored in plain contiguous arrays without compression.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.SuggestibleEntryCache from Apache Lucene 10.4.0.
type SuggestibleEntryCache struct {
	sections []*cacheSection
}

type cacheSection struct {
	rootLength int
	// meta[i*2] = formDataLength, meta[i*2+1] = wordCase ordinal
	meta     []int16
	roots    []rune // upper-case / title-case roots (only for non-lower entries)
	lowRoots []rune // always lower-case roots
	formData []int
}

// BuildSuggestibleEntryCache builds the cache from a WordStorage.
func BuildSuggestibleEntryCache(storage *WordStorage) *SuggestibleEntryCache {
	builders := make(map[int]*cacheSectionBuilder)
	maxLen := 0

	storage.ProcessSuggestibleWords(1, int(^uint(0)>>1), func(entry *FlyweightEntry) {
		word := entry.Root()
		n := len(word)
		if n > maxLen {
			maxLen = n
		}
		if n > int(^int16(0)) {
			return // skip unreasonably long entries
		}
		b, ok := builders[n]
		if !ok {
			b = &cacheSectionBuilder{}
			builders[n] = b
		}
		b.add(entry)
	})

	cache := &SuggestibleEntryCache{sections: make([]*cacheSection, maxLen+1)}
	for length, b := range builders {
		if length < len(cache.sections) {
			cache.sections[length] = b.build(length)
		}
	}
	return cache
}

type cacheSectionBuilder struct {
	roots    []rune
	lowRoots []rune
	meta     []int16
	formData []int
}

func (b *cacheSectionBuilder) add(entry *FlyweightEntry) {
	word := entry.Root()
	forms := entry.Forms()
	wc := CaseOfRunes(word, len(word))
	b.meta = append(b.meta, int16(forms.Length), int16(wc))
	b.lowRoots = append(b.lowRoots, word...) // stored as provided (caller supplies lower)
	if wc != WordCaseLower && wc != WordCaseNeutral {
		b.roots = append(b.roots, word...)
	}
	b.formData = append(b.formData, forms.Ints[:forms.Length]...)
}

func (b *cacheSectionBuilder) build(rootLength int) *cacheSection {
	return &cacheSection{
		rootLength: rootLength,
		meta:       append([]int16(nil), b.meta...),
		roots:      append([]rune(nil), b.roots...),
		lowRoots:   append([]rune(nil), b.lowRoots...),
		formData:   append([]int(nil), b.formData...),
	}
}

// ProcessSuggestibleWords calls processor for every entry with length in
// [minLength, maxLength].
func (c *SuggestibleEntryCache) ProcessSuggestibleWords(minLength, maxLength int, processor func(*FlyweightEntry)) {
	if maxLength >= len(c.sections) {
		maxLength = len(c.sections) - 1
	}
	for i := minLength; i <= maxLength; i++ {
		sec := c.sections[i]
		if sec != nil {
			sec.processWords(processor)
		}
	}
}

func (sec *cacheSection) processWords(processor func(*FlyweightEntry)) {
	rl := sec.rootLength
	wordBuf := make([]rune, rl)
	forms := &util.IntsRef{}

	entry := &FlyweightEntry{}

	lowOffset := 0
	hiOffset := 0
	fdOffset := 0

	for i := 0; i < len(sec.meta); i += 2 {
		formLen := int(sec.meta[i])
		wc := WordCase(sec.meta[i+1])

		forms.Ints = sec.formData[fdOffset : fdOffset+formLen]
		forms.Length = formLen

		if wc != WordCaseLower && wc != WordCaseNeutral {
			copy(wordBuf, sec.roots[hiOffset:hiOffset+rl])
			hiOffset += rl
		} else {
			copy(wordBuf, sec.lowRoots[lowOffset:lowOffset+rl])
		}

		entry.word = wordBuf
		entry.dataPos = fdOffset // abused here just to satisfy Forms()

		// For this cache, Forms() is not used — we call processor with explicit forms.
		// Use a closure-based approach: wrap in a local entry that overrides Forms.
		localEntry := &cacheEntry{FlyweightEntry: entry, cachedForms: forms}
		processor((*FlyweightEntry)(localEntry.FlyweightEntry))

		lowOffset += rl
		fdOffset += formLen
	}
}

type cacheEntry struct {
	*FlyweightEntry
	cachedForms *util.IntsRef
}
