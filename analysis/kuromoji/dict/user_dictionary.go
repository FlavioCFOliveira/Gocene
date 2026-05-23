// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"bufio"
	"io"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

// UserDictionary provides custom phrase segmentation and morphological
// information from a user-supplied dictionary.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.UserDictionary from Apache Lucene
// 10.4.0.
type UserDictionary struct {
	// fst maps phrase text to phrase IDs.
	fst *morph.TokenInfoFST
	// segmentations holds [wordID, length, length, ...] arrays indexed by
	// phrase ID.
	segmentations [][]int
	// morphAttrs provides morphological data for user words.
	morphAttrs *UserMorphData
}

// Open parses a CSV user dictionary from r and returns a UserDictionary.
// Returns nil if r contains no valid entries.
func Open(r io.Reader) (*UserDictionary, error) {
	type entry struct {
		surface string
		values  []string
	}
	var entries []entry

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := csvParse(line)
		if len(parts) < 4 {
			continue
		}
		surface := strings.Join(strings.Fields(parts[0]), "")
		entries = append(entries, entry{surface: surface, values: parts})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil //nolint:nilnil // Lucene returns null for empty user dict
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].surface < entries[j].surface
	})

	fst := morph.NewTokenInfoFST()
	var data []string
	var segmentations [][]int
	wordID := CustomDictionaryWordIDOffset

	for i, e := range entries {
		reading := strings.Join(strings.Fields(e.values[2]), " ")
		pos := e.values[3]
		data = append(data, reading+"\x00"+pos)

		segWords := strings.Fields(e.values[1])
		seg := make([]int, 1+len(segWords))
		seg[0] = wordID
		for j, w := range segWords {
			seg[j+1] = len([]rune(w))
		}
		segmentations = append(segmentations, seg)
		fst.Put([]byte(e.surface), int64(i))
		wordID++
	}

	return &UserDictionary{
		fst:           fst,
		segmentations: segmentations,
		morphAttrs:    NewUserMorphData(data),
	}, nil
}

// GetFST returns the TokenInfoFST used for phrase lookup.
func (d *UserDictionary) GetFST() *morph.TokenInfoFST { return d.fst }

// GetMorphAttributes returns the UserMorphData for this dictionary.
func (d *UserDictionary) GetMorphAttributes() *UserMorphData { return d.morphAttrs }

// LookupSegmentation returns the [wordID, len1, len2, ...] segmentation for
// phraseID.
func (d *UserDictionary) LookupSegmentation(phraseID int) []int {
	if phraseID < 0 || phraseID >= len(d.segmentations) {
		return nil
	}
	return d.segmentations[phraseID]
}

// Lookup returns a slice of [wordID, position, length] triples for all user
// phrases found in chars[off:off+length].
func (d *UserDictionary) Lookup(chars []rune, off, length int) [][]int {
	end := off + length
	var result [][]int
	for start := off; start < end; start++ {
		key := string(chars[start:end])
		for l := len([]rune(key)); l >= 1; l-- {
			sub := string([]rune(key)[:l])
			phraseID64 := d.fst.Lookup([]byte(sub))
			if phraseID64 < 0 {
				continue
			}
			phraseID := int(phraseID64)
			seg := d.LookupSegmentation(phraseID)
			if seg == nil || len(seg) < 2 {
				continue
			}
			wid := seg[0]
			pos := start - off
			for j := 1; j < len(seg); j++ {
				result = append(result, []int{wid + j - 1, pos, seg[j]})
				pos += seg[j]
			}
		}
	}
	return result
}

// LeftID delegates to morphAttrs.
func (d *UserDictionary) LeftID(wordID int) int {
	if d.morphAttrs == nil {
		return UserMorphLeftID
	}
	return d.morphAttrs.LeftID(wordID)
}

// RightID delegates to morphAttrs.
func (d *UserDictionary) RightID(wordID int) int {
	if d.morphAttrs == nil {
		return UserMorphRightID
	}
	return d.morphAttrs.RightID(wordID)
}

// WordCost delegates to morphAttrs.
func (d *UserDictionary) WordCost(wordID int) int {
	if d.morphAttrs == nil {
		return UserMorphWordCost
	}
	return d.morphAttrs.WordCost(wordID)
}

// PartOfSpeech delegates to morphAttrs.
func (d *UserDictionary) PartOfSpeech(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.PartOfSpeech(wordID)
}

// Reading delegates to morphAttrs.
func (d *UserDictionary) Reading(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.Reading(wordID, surface, off, length)
}

// Pronunciation delegates to morphAttrs.
func (d *UserDictionary) Pronunciation(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.Pronunciation(wordID, surface, off, length)
}

// BaseForm delegates to morphAttrs.
func (d *UserDictionary) BaseForm(wordID int, surface []rune, off, length int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.BaseForm(wordID, surface, off, length)
}

// InflectionType delegates to morphAttrs.
func (d *UserDictionary) InflectionType(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.InflectionType(wordID)
}

// InflectionForm delegates to morphAttrs.
func (d *UserDictionary) InflectionForm(wordID int) string {
	if d.morphAttrs == nil {
		return ""
	}
	return d.morphAttrs.InflectionForm(wordID)
}

// Ensure UserDictionary implements JaMorphData.
var _ JaMorphData = (*UserDictionary)(nil)

// csvParse is a minimal CSV line parser for user dictionary entries.
func csvParse(line string) []string {
	var fields []string
	var buf strings.Builder
	inQuote := false
	for i, c := range line {
		switch {
		case c == '"' && !inQuote:
			inQuote = true
		case c == '"' && inQuote:
			if i+1 < len(line) && line[i+1] == '"' {
				buf.WriteByte('"')
			} else {
				inQuote = false
			}
		case c == ',' && !inQuote:
			fields = append(fields, buf.String())
			buf.Reset()
		default:
			buf.WriteRune(c)
		}
	}
	fields = append(fields, buf.String())
	return fields
}
