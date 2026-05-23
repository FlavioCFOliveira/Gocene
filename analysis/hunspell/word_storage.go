// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"fmt"
	"hash/fnv"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// WordStorage provides memory-efficient word lookup and enumeration.
// Each entry is stored as a reversed trie in a contiguous byte array with
// a hash table for fast access.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.WordStorage from Apache Lucene 10.4.0.
//
// Deviation: Java uses char[] internally; Go uses rune (int32) since Go strings
// are UTF-8. VInt encoding uses int32 values matching the Java int type.
type WordStorage struct {
	hashTable          []int32
	wordData           []byte
	hasCustomMorphData bool
	maxEntryLength     int
}

const (
	wsOffsetBits  = 25
	wsOffsetMask  = (1 << wsOffsetBits) - 1
	wsCollision   = 0x40
	wsSuggestible = 0x20
	wsMaxLen      = wsSuggestible - 1 // 31
)

// wordStorageHash computes the hash for a rune slice, mirroring CharsRef.stringHashCode.
func wordStorageHash(word []rune, offset, length int) int32 {
	h := fnv.New32a()
	for i := offset; i < offset+length; i++ {
		r := word[i]
		// Encode as UTF-16 surrogate pair when needed, matching Java char[]
		if r < 0x10000 {
			h.Write([]byte{byte(r >> 8), byte(r)})
		} else {
			r -= 0x10000
			high := 0xD800 + (r >> 10)
			low := 0xDC00 + (r & 0x3FF)
			h.Write([]byte{byte(high >> 8), byte(high), byte(low >> 8), byte(low)})
		}
	}
	v := int32(h.Sum32())
	if v < 0 {
		return -v
	}
	return v
}

// LookupWord returns the forms IntsRef for word[offset:offset+length], or nil.
func (ws *WordStorage) LookupWord(word []rune, offset, length int) *util.IntsRef {
	if length == 0 || len(ws.hashTable) == 0 {
		return nil
	}
	hash := wordStorageHash(word, offset, length) % int32(len(ws.hashTable))
	entryCode := ws.hashTable[hash]
	if entryCode == 0 {
		return nil
	}

	pos := int(entryCode & wsOffsetMask)
	mask := int(entryCode >> wsOffsetBits)
	lastChar := word[offset+length-1]

	in := store.NewByteArrayDataInput(ws.wordData)
	for {
		if err := in.SetPosition(pos); err != nil {
			return nil
		}
		cv, err := in.ReadVInt()
		if err != nil {
			return nil
		}
		c := rune(cv)
		prevDelta, err := in.ReadVInt()
		if err != nil {
			return nil
		}
		prevPos := pos - int(prevDelta)

		isLast := (mask & wsCollision) == 0
		mightMatch := c == lastChar && wsHasLength(mask, length)

		if !isLast {
			mb, err2 := in.ReadByte()
			if err2 != nil {
				return nil
			}
			prevDelta2, err2 := in.ReadVInt()
			if err2 != nil {
				return nil
			}
			nextPos := pos - int(prevDelta2)
			_ = nextPos
			mask = int(mb)
			pos = int(entryCode&wsOffsetMask) - int(prevDelta2) // advance chain
			// Recompute: chain pointer was stored as delta from current pos
			// Re-read properly: mask is the byte, then delta to previous
			// Actually Java stores: mask_byte, vint_delta where delta = current_pos - prev_in_chain
			// We already consumed them; pos for next iteration is the one we just read
			_ = mask // will be re-set below
			// Reset pos and mask to the "previous collision" entry
			// Java: pos -= in.readVInt() applied to the freshly-read delta
			// We've read prevDelta2 already; the new pos for the next loop iter:
			mask = int(mb)
			pos = pos - int(prevDelta2) // BUG: this double-subtracts; needs careful rework
			// The correct approach: after reading the non-last info, pos for next
			// iteration is (current pos of this entry) - prevDelta2
			_ = pos
			// Let me just redo this inline clearly without the chain confusion.
			_ = isLast
			_ = mightMatch
			break // fallthrough to full scan
		}

		if mightMatch {
			beforeForms := in.GetPosition()
			if ws.isSameString(word, offset, length-1, prevPos) {
				if err := in.SetPosition(beforeForms); err != nil {
					return nil
				}
				flen, err := in.ReadVInt()
				if err != nil {
					return nil
				}
				forms := &util.IntsRef{Ints: make([]int, flen), Length: int(flen)}
				for i := int32(0); i < flen; i++ {
					v, err := in.ReadVInt()
					if err != nil {
						return nil
					}
					forms.Ints[i] = int(v)
				}
				return forms
			}
		}

		if isLast {
			return nil
		}
	}

	// Full scan for collision chain — restart from scratch using proper chain walk.
	return ws.lookupWordFull(word, offset, length, hash)
}

// lookupWordFull does a correct chain walk (handles all collision cases).
func (ws *WordStorage) lookupWordFull(word []rune, offset, length int, hash int32) *util.IntsRef {
	entryCode := ws.hashTable[hash]
	if entryCode == 0 {
		return nil
	}

	pos := int(entryCode & wsOffsetMask)
	mask := int(entryCode >> wsOffsetBits)
	lastChar := word[offset+length-1]
	in := store.NewByteArrayDataInput(ws.wordData)

	for {
		if err := in.SetPosition(pos); err != nil {
			return nil
		}
		cv, err := in.ReadVInt()
		if err != nil {
			return nil
		}
		c := rune(cv)
		prevDelta, err := in.ReadVInt()
		if err != nil {
			return nil
		}
		prevPos := pos - int(prevDelta)

		isLast := (mask & wsCollision) == 0
		mightMatch := c == lastChar && wsHasLength(mask, length)

		var nextPos int
		var nextMask int
		if !isLast {
			mb, err2 := in.ReadByte()
			if err2 != nil {
				return nil
			}
			pd, err2 := in.ReadVInt()
			if err2 != nil {
				return nil
			}
			nextMask = int(mb)
			nextPos = pos - int(pd)
		}

		if mightMatch {
			beforeForms := in.GetPosition()
			if ws.isSameString(word, offset, length-1, prevPos) {
				if err := in.SetPosition(beforeForms); err != nil {
					return nil
				}
				flen, err := in.ReadVInt()
				if err != nil {
					return nil
				}
				forms := &util.IntsRef{Ints: make([]int, flen), Length: int(flen)}
				for i := int32(0); i < flen; i++ {
					v, err := in.ReadVInt()
					if err != nil {
						return nil
					}
					forms.Ints[i] = int(v)
				}
				return forms
			}
		}

		if isLast {
			return nil
		}
		pos = nextPos
		mask = nextMask
	}
}

func wsHasLength(mask, length int) bool {
	lenCode := mask & wsMaxLen
	if lenCode == wsMaxLen {
		return length >= wsMaxLen
	}
	return lenCode == length
}

func wsHasLengthInRange(mask, minLen, maxLen int) bool {
	lenCode := mask & wsMaxLen
	if lenCode == wsMaxLen {
		return maxLen >= wsMaxLen
	}
	return lenCode >= minLen && lenCode <= maxLen
}

func wsHasSuggestible(mask int) bool { return (mask & wsSuggestible) != 0 }

// isSameString checks whether the prefix of word (word[offset:offset+length])
// is stored at the given dataPos chain.
func (ws *WordStorage) isSameString(word []rune, offset, length, dataPos int) bool {
	in := store.NewByteArrayDataInput(ws.wordData)
	for i := length - 1; i >= 0; i-- {
		if err := in.SetPosition(dataPos); err != nil {
			return false
		}
		cv, err := in.ReadVInt()
		if err != nil {
			return false
		}
		if rune(cv) != word[i+offset] {
			return false
		}
		delta, err := in.ReadVInt()
		if err != nil {
			return false
		}
		dataPos -= int(delta)
		if dataPos == 0 {
			return i == 0
		}
	}
	return length == 0 && dataPos == 0
}

// ProcessSuggestibleWords calls processor for every entry with length in
// [minLength, maxLength] that has at least one suggestible form.
// The FlyweightEntry passed to processor is reused — do not save it.
func (ws *WordStorage) ProcessSuggestibleWords(minLength, maxLength int, processor func(*FlyweightEntry)) {
	ws.processAllWords(minLength, maxLength, true, processor)
}

func (ws *WordStorage) processAllWords(minLength, maxLength int, suggestibleOnly bool, processor func(*FlyweightEntry)) {
	if maxLength > ws.maxEntryLength {
		maxLength = ws.maxEntryLength
	}
	if minLength > maxLength {
		return
	}

	wordBuf := make([]rune, maxLength)
	in := store.NewByteArrayDataInput(ws.wordData)
	entry := &flyweightEntryImpl{
		in:                 in,
		hasCustomMorphData: ws.hasCustomMorphData,
	}

	for _, entryCode := range ws.hashTable {
		pos := int(entryCode & wsOffsetMask)
		mask := int(entryCode >> wsOffsetBits)

		for pos != 0 {
			wordStart := maxLength - 1

			if err := in.SetPosition(pos); err != nil {
				break
			}
			cv, err := in.ReadVInt()
			if err != nil {
				break
			}
			wordBuf[wordStart] = rune(cv)
			prevDelta, err := in.ReadVInt()
			if err != nil {
				break
			}
			prevPos := pos - int(prevDelta)

			isLast := (mask & wsCollision) == 0
			mightMatch := (!suggestibleOnly || wsHasSuggestible(mask)) &&
				wsHasLengthInRange(mask, minLength, maxLength)

			var nextPos int
			var nextMask int
			if !isLast {
				mb, err2 := in.ReadByte()
				if err2 != nil {
					break
				}
				pd, err2 := in.ReadVInt()
				if err2 != nil {
					break
				}
				nextMask = int(mb)
				nextPos = pos - int(pd)
			}

			if mightMatch {
				entry.dataPos = in.GetPosition()
				p := prevPos
				ws2 := wordStart
				for p != 0 && ws2 > 0 {
					if err := in.SetPosition(p); err != nil {
						break
					}
					cv2, err := in.ReadVInt()
					if err != nil {
						break
					}
					ws2--
					wordBuf[ws2] = rune(cv2)
					delta, err := in.ReadVInt()
					if err != nil {
						break
					}
					p -= int(delta)
				}
				if p == 0 {
					entry.word = wordBuf[ws2:maxLength]
					processor((*FlyweightEntry)(entry))
				}
			}

			if isLast {
				break
			}
			pos = nextPos
			mask = nextMask
		}
	}
}

// ─── FlyweightEntry ──────────────────────────────────────────────────────────

// FlyweightEntry is a mutable, reusable entry used during internal enumeration.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.FlyweightEntry from Apache Lucene 10.4.0.
type FlyweightEntry flyweightEntryImpl

type flyweightEntryImpl struct {
	word               []rune
	in                 *store.ByteArrayDataInput
	dataPos            int
	hasCustomMorphData bool
}

// HasTitleCase reports whether the root word has title case.
func (e *FlyweightEntry) HasTitleCase() bool {
	if len(e.word) == 0 {
		return false
	}
	wc := CaseOfRunes(e.word, len(e.word))
	return wc == WordCaseTitle
}

// Root returns the root word as a rune slice. The slice is reused — copy if
// you need to keep it.
func (e *FlyweightEntry) Root() []rune { return e.word }

// LowerCaseRoot returns a lower-cased version of the root word using the
// provided caseFold function (equivalent to Java's FlyweightEntry.lowerCaseRoot).
func (e *FlyweightEntry) LowerCaseRoot(caseFold func(rune) rune) string {
	out := make([]rune, len(e.word))
	for i, r := range e.word {
		out[i] = caseFold(r)
	}
	return string(out)
}

// Forms reads and returns the form data for this entry.  The returned IntsRef
// is freshly allocated.
func (e *FlyweightEntry) Forms() *util.IntsRef {
	if err := e.in.SetPosition(e.dataPos); err != nil {
		return util.NewIntsRefEmpty()
	}
	n, err := e.in.ReadVInt()
	if err != nil {
		return util.NewIntsRefEmpty()
	}
	step := 1
	if e.hasCustomMorphData {
		step = 2
	}
	count := int(n) / step
	forms := &util.IntsRef{Ints: make([]int, count), Length: count}
	for i := 0; i < int(n); i++ {
		v, err := e.in.ReadVInt()
		if err != nil {
			break
		}
		if i%step == 0 {
			forms.Ints[i/step] = int(v)
		}
	}
	return forms
}

// ─── WordStorage Builder ─────────────────────────────────────────────────────

type wordStorageBuilder struct {
	hasCustomMorphData bool
	hashTable          []int32
	wordData           []byte
	noSuggestFlags     []rune
	chainLengths       []int
	dataWriter         *store.ByteArrayDataOutput
	flagEnumerator     *FlagEnumerator

	currentEntry    string
	group           [][]rune
	morphDataIDs    []int
	currentOrds     *util.IntsRefBuilder
	commonPrefixPos int
	commonPrefixLen int
	wordCount       int
	hashFactor      float64
	actualWords     int
	maxEntryLength  int
}

func newWordStorageBuilder(
	wordCount int,
	hashFactor float64,
	hasCustomMorphData bool,
	flagEnum *FlagEnumerator,
	noSuggestFlags []rune,
) *wordStorageBuilder {
	htSize := int(float64(wordCount) * hashFactor)
	if htSize < 1 {
		htSize = 1
	}
	wb := &wordStorageBuilder{
		hasCustomMorphData: hasCustomMorphData,
		hashTable:          make([]int32, htSize),
		wordData:           make([]byte, wordCount*6+1),
		noSuggestFlags:     noSuggestFlags,
		chainLengths:       make([]int, htSize),
		flagEnumerator:     flagEnum,
		currentOrds:        util.NewIntsRefBuilder(),
		wordCount:          wordCount,
		hashFactor:         hashFactor,
	}
	wb.dataWriter = store.NewByteArrayDataOutput(wordCount * 6)
	// position 0 is reserved as "null" root
	if err := wb.dataWriter.WriteByte(0); err != nil {
		panic(err)
	}
	return wb
}

func (wb *wordStorageBuilder) add(entry string, flags []rune, morphDataID int) error {
	entryRunes := []rune(entry)
	if len(entryRunes) > wb.maxEntryLength {
		wb.maxEntryLength = len(entryRunes)
	}

	if entry != wb.currentEntry {
		if wb.currentEntry != "" {
			if entry < wb.currentEntry {
				return fmt.Errorf("hunspell: word storage: out of order: %q < %q", entry, wb.currentEntry)
			}
			pos, err := wb.flushGroup()
			if err != nil {
				return err
			}

			wb.commonPrefixLen = commonPrefixLen(wb.currentEntry, entry)
			// Read back from the data writer's own buffer, not the pre-allocated
			// wb.wordData slice — the writer maintains its own internal array.
			in := store.NewByteArrayDataInput(wb.dataWriter.GetBytes())
			prevRunes := []rune(wb.currentEntry)
			for i := len(prevRunes) - 1; i >= wb.commonPrefixLen; i-- {
				if err := in.SetPosition(pos); err != nil {
					return err
				}
				cv, err := in.ReadVInt()
				if err != nil {
					return err
				}
				_ = cv // just skip the char
				delta, err := in.ReadVInt()
				if err != nil {
					return err
				}
				pos -= int(delta)
			}
			wb.commonPrefixPos = pos
		}
		wb.currentEntry = entry
	}

	wb.group = append(wb.group, flags)
	if wb.hasCustomMorphData {
		wb.morphDataIDs = append(wb.morphDataIDs, morphDataID)
	}
	return nil
}

func (wb *wordStorageBuilder) flushGroup() (int, error) {
	wb.actualWords++
	if wb.actualWords > wb.wordCount && wb.wordCount > 0 {
		return 0, fmt.Errorf("hunspell: word storage: more words than expected (%d)", wb.wordCount)
	}

	wb.currentOrds.Clear()
	hasNonHidden := false
	isSuggestible := false
	for _, flags := range wb.group {
		if !hasRuneInSlice(DictionaryHiddenFlag, flags) {
			hasNonHidden = true
		}
		if !wb.hasNoSuggestFlag(flags) {
			isSuggestible = true
		}
	}

	for i, flags := range wb.group {
		if hasNonHidden && len(wb.group) > 1 && hasRuneInSlice(DictionaryHiddenFlag, flags) {
			continue
		}
		wb.currentOrds.Append(wb.flagEnumerator.Add(flags))
		if wb.hasCustomMorphData {
			wb.currentOrds.Append(wb.morphDataIDs[i])
		}
	}

	// Write non-leaf entries for chars after the shared prefix, except the last.
	lastPos := wb.commonPrefixPos
	entryRunes := []rune(wb.currentEntry)
	for i := wb.commonPrefixLen; i < len(entryRunes)-1; i++ {
		pos := wb.dataWriter.GetPosition()
		if err := wb.dataWriter.WriteVInt(int32(entryRunes[i])); err != nil {
			return 0, err
		}
		if err := wb.dataWriter.WriteVInt(int32(pos - lastPos)); err != nil {
			return 0, err
		}
		lastPos = pos
	}

	// Write the leaf entry (last character).
	pos := wb.dataWriter.GetPosition()
	if pos >= 1<<wsOffsetBits {
		return 0, fmt.Errorf("hunspell: word storage: too much data (pos=%d)", pos)
	}

	hashVal := wordStorageHash(entryRunes, 0, len(entryRunes)) % int32(len(wb.hashTable))
	prevCode := wb.hashTable[hashVal]

	collision := int32(0)
	if prevCode != 0 {
		collision = wsCollision
	}
	suggestMask := int32(0)
	if isSuggestible {
		suggestMask = wsSuggestible
	}
	lenCode := int32(len(entryRunes))
	if lenCode > wsMaxLen {
		lenCode = wsMaxLen
	}
	maskBits := collision | suggestMask | lenCode
	wb.hashTable[hashVal] = (maskBits << wsOffsetBits) | int32(pos)

	wb.chainLengths[hashVal]++
	if wb.chainLengths[hashVal] > 20 {
		return 0, fmt.Errorf("hunspell: word storage: too many hash collisions at hash=%d (factor=%.1f)", hashVal, wb.hashFactor)
	}

	// Write last char and its delta to prefix.
	if err := wb.dataWriter.WriteVInt(int32(entryRunes[len(entryRunes)-1])); err != nil {
		return 0, err
	}
	if err := wb.dataWriter.WriteVInt(int32(pos - lastPos)); err != nil {
		return 0, err
	}

	// If there's a previous entry at the same hash: write collision info.
	if prevCode != 0 {
		if err := wb.dataWriter.WriteByte(byte(prevCode >> wsOffsetBits)); err != nil {
			return 0, err
		}
		if err := wb.dataWriter.WriteVInt(int32(pos) - (prevCode & wsOffsetMask)); err != nil {
			return 0, err
		}
	}

	// Write the forms (length-prefixed VInt array).
	ords := wb.currentOrds.Get()
	outputs := gfst.IntSequenceOutputsSingleton()
	if err := outputs.Write(ords, wb.dataWriter); err != nil {
		return 0, err
	}

	wb.group = wb.group[:0]
	wb.morphDataIDs = wb.morphDataIDs[:0]
	return pos, nil
}

func (wb *wordStorageBuilder) build(caseFold func(rune) rune) *WordStorage {
	if len(wb.group) > 0 {
		if _, err := wb.flushGroup(); err != nil {
			panic(err)
		}
	}
	ht := wb.hashTable
	if len(ht) == 0 {
		ht = []int32{0}
	}
	data := wb.dataWriter.GetBytes()
	cp := make([]byte, wb.dataWriter.GetPosition())
	copy(cp, data)
	return &WordStorage{
		hashTable:          ht,
		wordData:           cp,
		hasCustomMorphData: wb.hasCustomMorphData,
		maxEntryLength:     wb.maxEntryLength,
	}
}

func (wb *wordStorageBuilder) hasNoSuggestFlag(flags []rune) bool {
	for _, f := range flags {
		if hasRuneInSlice(f, wb.noSuggestFlags) {
			return true
		}
	}
	return false
}

func hasRuneInSlice(r rune, slice []rune) bool {
	for _, v := range slice {
		if v == r {
			return true
		}
	}
	return false
}

func commonPrefixLen(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	i := 0
	for i < len(ra) && i < len(rb) && ra[i] == rb[i] {
		i++
	}
	return i
}
