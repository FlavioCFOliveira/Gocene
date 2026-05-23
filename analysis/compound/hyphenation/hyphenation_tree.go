// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hyphenation

import (
	"io"
)

// HyphenationTree extends TernaryTree with hyphenation-pattern lookup and
// implements PatternConsumer to receive parsed pattern data.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.hyphenation.HyphenationTree from
// Apache Lucene 10.4.0. Taken originally from the Apache FOP project.
type HyphenationTree struct {
	TernaryTree

	// vspace stores packed interletter values (4 bits each).
	vspace *ByteVector

	// stoplist stores hand-crafted hyphenation exceptions.
	stoplist map[string][]any

	// classmap maps each character to its equivalence class representative.
	classmap *TernaryTree

	// ivalues is a transient map used during pattern loading to deduplicate
	// packed value sequences.
	ivalues *TernaryTree
}

// NewHyphenationTree creates an empty HyphenationTree.
func NewHyphenationTree() *HyphenationTree {
	t := &HyphenationTree{
		stoplist: make(map[string][]any, 23),
		classmap: newTernaryTree(),
		vspace:   NewByteVector(),
	}
	t.TernaryTree.init()
	t.vspace.Alloc(1) // index 0 is reserved
	return t
}

// LoadPatterns reads and parses an FOP-format XML hyphenation pattern file
// from r, populating the tree.
func (t *HyphenationTree) LoadPatterns(r io.Reader) error {
	t.ivalues = newTernaryTree()
	pp := NewPatternParserWithConsumer(t)
	if err := pp.Parse(r); err != nil {
		return err
	}
	t.TrimToSize()
	t.vspace.TrimToSize()
	t.classmap.TrimToSize()
	t.ivalues = nil
	return nil
}

// packValues packs a string of digit characters into 4-bit pairs in vspace.
// Returns the vspace index of the first byte.
func (t *HyphenationTree) packValues(values string) int {
	n := len(values)
	m := (n >> 1) + 1
	if (n & 1) == 1 {
		m++
	}
	offset := t.vspace.Alloc(m)
	va := t.vspace.GetArray()
	for i := 0; i < n; i++ {
		j := i >> 1
		v := byte((values[i]-'0'+1) & 0x0f)
		if (i & 1) == 1 {
			va[j+offset] |= v
		} else {
			va[j+offset] = v << 4
		}
	}
	va[m-1+offset] = 0 // terminator
	return offset
}

// getValues unpacks the interletter byte values stored at vspace index k.
func (t *HyphenationTree) getValues(k int) []byte {
	var buf []byte
	v := t.vspace.Get(k)
	k++
	for v != 0 {
		c := byte((v&0xf0)>>4) - 1
		buf = append(buf, c)
		c = v & 0x0f
		if c == 0 {
			break
		}
		buf = append(buf, c-1)
		v = t.vspace.Get(k)
		k++
	}
	return buf
}

// hstrcmp compares two null-terminated uint16 slices; returns 0 if equal or
// t is a substring of s.
func hstrcmp(s []uint16, si int, tt []uint16, ti int) int {
	for s[si] == tt[ti] {
		if s[si] == 0 {
			return 0
		}
		si++
		ti++
	}
	if tt[ti] == 0 {
		return 0
	}
	return int(s[si]) - int(tt[ti])
}

// searchPatterns updates interletter value array il by scanning all patterns
// that match at word[index:].
func (t *HyphenationTree) searchPatterns(word []uint16, index int, il []byte) {
	p := t.root
	i := index
	sp := word[i]

	for p != 0 && int(p) < len(t.sc) {
		if t.sc[p] == 0xFFFF {
			if hstrcmp(word, i, t.kv.GetArray(), int(t.lo[p])) == 0 {
				values := t.getValues(int(t.eq[p]))
				j := index
				for _, val := range values {
					if j < len(il) && val > il[j] {
						il[j] = val
					}
					j++
				}
			}
			return
		}
		d := int(sp) - int(t.sc[p])
		if d == 0 {
			if sp == 0 {
				break
			}
			i++
			sp = word[i]
			p = t.eq[p]
			q := p
			for q != 0 && int(q) < len(t.sc) {
				if t.sc[q] == 0xFFFF {
					break
				}
				if t.sc[q] == 0 {
					values := t.getValues(int(t.eq[q]))
					j := index
					for _, val := range values {
						if j < len(il) && val > il[j] {
							il[j] = val
						}
						j++
					}
					break
				}
				q = t.lo[q]
			}
		} else if d < 0 {
			p = t.lo[p]
		} else {
			p = t.hi[p]
		}
	}
}

// Hyphenate hyphenates word (given as rune slice w[offset:offset+length]) and
// returns a Hyphenation or nil if no hyphenation points were found.
func (t *HyphenationTree) Hyphenate(w []rune, offset, length, remainCharCount, pushCharCount int) *Hyphenation {
	word := make([]uint16, length+3)

	// Normalize word via classmap.
	iIgnoreAtBeginning := 0
	iLength := length
	bEndOfLetters := false
	key := make([]uint16, 2)
	for i := 1; i <= length; i++ {
		key[0] = uint16(w[offset+i-1])
		key[1] = 0
		nc := t.classmap.findSlice(key, 0)
		if nc < 0 {
			if i == 1+iIgnoreAtBeginning {
				iIgnoreAtBeginning++
			} else {
				bEndOfLetters = true
			}
			iLength--
		} else {
			if !bEndOfLetters {
				word[i-iIgnoreAtBeginning] = uint16(nc)
			} else {
				return nil
			}
		}
	}
	length = iLength
	if length < remainCharCount+pushCharCount {
		return nil
	}

	result := make([]int, length+1)
	k := 0

	// Check exception list.
	sw := uint16SliceToString(word[1 : length+1])
	if hw, ok := t.stoplist[sw]; ok {
		j := 0
		for _, item := range hw {
			if s, ok2 := item.(string); ok2 {
				j += len([]rune(s))
				if j >= remainCharCount && j < length-pushCharCount {
					result[k] = j + iIgnoreAtBeginning
					k++
				}
			}
		}
	} else {
		word[0] = '.'
		word[length+1] = '.'
		word[length+2] = 0
		il := make([]byte, length+3)
		for i := 0; i < length+1; i++ {
			t.searchPatterns(word, i, il)
		}
		for i := 0; i < length; i++ {
			if (il[i+1]&1) == 1 && i >= remainCharCount && i <= length-pushCharCount {
				result[k] = i + iIgnoreAtBeginning
				k++
			}
		}
	}

	if k > 0 {
		res := make([]int, k+2)
		copy(res[1:], result[:k])
		res[0] = 0
		res[k+1] = length
		return NewHyphenation(res)
	}
	return nil
}

func uint16SliceToString(s []uint16) string {
	rs := make([]rune, len(s))
	for i, c := range s {
		rs[i] = rune(c)
	}
	return string(rs)
}

// HyphenateString is a convenience wrapper for string input.
func (t *HyphenationTree) HyphenateString(word string, remainCharCount, pushCharCount int) *Hyphenation {
	w := []rune(word)
	return t.Hyphenate(w, 0, len(w), remainCharCount, pushCharCount)
}

// AddClass implements PatternConsumer.
func (t *HyphenationTree) AddClass(chargroup string) {
	if len(chargroup) > 0 {
		equivChar := rune(chargroup[0])
		key := make([]uint16, 2)
		key[1] = 0
		for _, c := range chargroup {
			key[0] = uint16(c)
			t.classmap.insertSlice(t.classmap.root, key, 0, uint16(equivChar))
			t.classmap.root = t.classmap.insertSlice(t.classmap.root, key, 0, uint16(equivChar))
		}
	}
}

// AddException implements PatternConsumer.
func (t *HyphenationTree) AddException(word string, hyphenatedWord []any) {
	t.stoplist[word] = hyphenatedWord
}

// AddPattern implements PatternConsumer.
func (t *HyphenationTree) AddPattern(pattern, ivalue string) {
	k := t.ivalues.Find(ivalue)
	if k <= 0 {
		k = t.packValues(ivalue)
		t.ivalues.Insert(ivalue, uint16(k))
	}
	t.Insert(pattern, uint16(k))
}

// Ensure HyphenationTree implements PatternConsumer.
var _ PatternConsumer = (*HyphenationTree)(nil)
