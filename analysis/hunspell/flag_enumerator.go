// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"sort"
)

// flagUnset is the zero value used when no flag is set.
// Mirrors Dictionary.FLAG_UNSET in Apache Lucene 10.4.0.
const flagUnset = rune(0)

// FlagEnumerator deduplicates sorted flag sequences and assigns each unique
// sequence a stable integer id.  The id can later be used with
// FlagLookup.HasFlag to test membership in O(1) time.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.FlagEnumerator from Apache Lucene 10.4.0.
//
// Implementation note: Java uses char[] backed by a StringBuilder; ids are
// char (16-bit) positions.  Go uses []rune to preserve the same semantics —
// ids are rune positions, not byte positions.  Using a strings.Builder and
// converting back via []rune(s) would give wrong ids whenever a flag rune
// encodes to more than one UTF-8 byte (e.g. DictionaryHiddenFlag = 65511).
type FlagEnumerator struct {
	data    []rune
	indices map[string]int
}

// NewFlagEnumerator creates an empty FlagEnumerator with the zero-flags entry
// pre-inserted at id 0.
func NewFlagEnumerator() *FlagEnumerator {
	fe := &FlagEnumerator{indices: make(map[string]int)}
	fe.Add(nil) // no-flags entry → ord 0
	return fe
}

// Add inserts a flag set (a []rune that will be sorted in-place) and returns
// its stable id.  Duplicate flag sets share the same id.
func (fe *FlagEnumerator) Add(flags []rune) int {
	if len(flags) > 0 {
		sort.Slice(flags, func(i, j int) bool { return flags[i] < flags[j] })
	}
	// Use the rune-encoded form as map key for deduplication.
	key := string(flags)
	if len(flags) > int(^uint16(0)) {
		panic("hunspell: too many flags")
	}

	if id, ok := fe.indices[key]; ok {
		return id
	}

	// id is the rune position, not the byte position.
	id := len(fe.data)
	fe.indices[key] = id
	// Write the count of flags as a length-prefix rune, then the flags themselves.
	fe.data = append(fe.data, rune(len(flags)))
	fe.data = append(fe.data, flags...)
	return id
}

// Finish freezes the enumerator and returns a FlagLookup backed by the
// accumulated data.
func (fe *FlagEnumerator) Finish() *FlagLookup {
	cp := make([]rune, len(fe.data))
	copy(cp, fe.data)
	return &FlagLookup{data: cp}
}

// HasFlagInSortedArray reports whether flag is present in the sorted sub-array
// array[start : start+length].
func HasFlagInSortedArray(flag rune, array []rune, start, length int) bool {
	if flag == flagUnset {
		return false
	}
	for i := start; i < start+length; i++ {
		c := array[i]
		if c == flag {
			return true
		}
		if c > flag {
			return false
		}
	}
	return false
}

// FlagLookup provides flag membership tests against the frozen flag
// data produced by FlagEnumerator.Finish.
//
// This is the Go port of FlagEnumerator.Lookup in Apache Lucene 10.4.0.
type FlagLookup struct {
	data []rune
}

// HasFlag reports whether the entry identified by entryID has the given flag.
func (l *FlagLookup) HasFlag(entryID int, flag rune) bool {
	if entryID < 0 {
		return false
	}
	return HasFlagInSortedArray(flag, l.data, entryID+1, int(l.data[entryID]))
}

// HasAnyFlag reports whether the entry identified by entryID has any flag in
// sortedFlags (which must be sorted in ascending order).
func (l *FlagLookup) HasAnyFlag(entryID int, sortedFlags []rune) bool {
	length := int(l.data[entryID])
	if length == 0 {
		return false
	}

	pos1 := entryID + 1
	limit1 := entryID + 1 + length

	pos2 := 0
	limit2 := len(sortedFlags)
	if limit2 == 0 {
		return false
	}

	c1 := l.data[pos1]
	c2 := sortedFlags[pos2]
	for {
		if c1 == c2 {
			return true
		}
		if c1 < c2 {
			pos1++
			if pos1 >= limit1 {
				return false
			}
			c1 = l.data[pos1]
		} else {
			pos2++
			if pos2 >= limit2 {
				return false
			}
			c2 = sortedFlags[pos2]
		}
	}
}

// GetFlags returns a copy of the flags for the entry identified by entryID.
func (l *FlagLookup) GetFlags(entryID int) []rune {
	length := int(l.data[entryID])
	result := make([]rune, length)
	copy(result, l.data[entryID+1:entryID+1+length])
	return result
}
