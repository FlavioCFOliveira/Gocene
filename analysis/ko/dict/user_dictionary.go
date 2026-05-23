// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"bufio"
	"io"
	"sort"
	"strings"
)

// userDictionaryRightID is NNG right connection ID.
const userDictionaryRightID int16 = 3533

// userDictionaryRightIDT is NNG right with hangul and coda on the last char.
const userDictionaryRightIDT int16 = 3535

// userDictionaryRightIDF is NNG right with hangul and no coda on the last char.
const userDictionaryRightIDF int16 = 3534

// UserDictionary provides custom noun (세종) or compound (세종시 세종 시) entries.
//
// This is the Go port of org.apache.lucene.analysis.ko.dict.UserDictionary
// from Apache Lucene 10.4.0.
//
// Deviation: the Lucene reference uses an FST. The Go port uses a sorted slice
// of entries with binary search; full FST-based lookup is deferred to the nori
// codec sprint.
type UserDictionary struct {
	// entries is sorted by surface form.
	entries []userEntry
	// morphAtts provides morphological data.
	morphAtts *UserMorphData
}

type userEntry struct {
	surface string
	ordinal int
}

// Open reads a user dictionary from r and returns a UserDictionary, or nil if
// r contains no entries.
func Open(r io.Reader) (*UserDictionary, error) {
	scanner := bufio.NewScanner(r)
	type rawEntry struct {
		token    string
		segments []int
	}
	var entries []rawEntry
	for scanner.Scan() {
		line := scanner.Text()
		// Remove comments
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		fields := strings.Fields(line)
		token := fields[0]
		var segs []int
		if len(fields) > 1 {
			segs = make([]int, len(fields)-1)
			for i, f := range fields[1:] {
				segs[i] = len([]rune(f))
			}
		}
		entries = append(entries, rawEntry{token: token, segments: segs})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil //nolint:nilnil // matches Java behaviour: returns null for empty input
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].token < entries[j].token
	})

	charDef := GetCharacterDefinitionInstance()
	segmentations := make([][]int, 0, len(entries))
	rightIDs := make([]int16, 0, len(entries))
	sortedEntries := make([]userEntry, 0, len(entries))
	seen := make(map[string]bool)
	ordinal := 0
	for _, e := range entries {
		if seen[e.token] {
			continue
		}
		seen[e.token] = true
		sortedEntries = append(sortedEntries, userEntry{surface: e.token, ordinal: ordinal})

		// right ID depends on the last character
		runes := []rune(e.token)
		lastChar := runes[len(runes)-1]
		var rid int16
		if charDef.IsHangul(lastChar) {
			if HasCoda(lastChar) {
				rid = userDictionaryRightIDT
			} else {
				rid = userDictionaryRightIDF
			}
		} else {
			rid = userDictionaryRightID
		}
		rightIDs = append(rightIDs, rid)
		segmentations = append(segmentations, e.segments)
		ordinal++
	}
	return &UserDictionary{
		entries:   sortedEntries,
		morphAtts: NewUserMorphData(segmentations, rightIDs),
	}, nil
}

// GetMorphAttributes returns the UserMorphData for this dictionary.
func (d *UserDictionary) GetMorphAttributes() *UserMorphData { return d.morphAtts }

// GetFST returns nil; full FST lookup is deferred to the nori codec sprint.
//
// Deviation: returns nil instead of a real TokenInfoFST. Callers must guard
// against nil.
func (d *UserDictionary) GetFST() *TokenInfoFST { return nil }

// Lookup returns word IDs for all user entries that match chars[off:off+len].
// Returns an empty slice if no match is found.
func (d *UserDictionary) Lookup(chars []rune, off, length int) []int {
	if d == nil || len(d.entries) == 0 {
		return nil
	}
	target := string(chars[off : off+length])
	// binary search for the first entry >= target
	idx := sort.Search(len(d.entries), func(i int) bool {
		return d.entries[i].surface >= target
	})
	var result []int
	for i := idx; i < len(d.entries) && d.entries[i].surface == target; i++ {
		result = append(result, d.entries[i].ordinal)
	}
	return result
}
