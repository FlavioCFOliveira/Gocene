// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"strings"
)

// DictEntry represents a single *.dic file entry with its word, flags and
// morphological data.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.DictEntry from Apache Lucene 10.4.0.
type DictEntry interface {
	// GetStem returns the stem word in the dictionary.
	GetStem() string
	// GetFlags returns the flags associated with the entry, encoded in the
	// same format as in the *.dic file.
	GetFlags() string
	// GetMorphologicalData returns morphological fields (kk:vvvvvv form,
	// sorted, space-separated, excluding ph:), or an empty string.
	GetMorphologicalData() string
	// GetMorphologicalValues returns values for the given morphological key
	// (e.g. "po:").
	GetMorphologicalValues(key string) []string
	// String returns the canonical representation of the entry.
	String() string
}

// DictEntries is an ordered list of homonym dictionary entries.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.DictEntries from Apache Lucene 10.4.0.
type DictEntries []DictEntry

// dictEntryImpl is the concrete, immutable implementation.
type dictEntryImpl struct {
	stem  string
	flags string
	morph string
}

// NewDictEntryFromData constructs a DictEntry from its component parts.
func NewDictEntryFromData(stem, flags, morphData string) DictEntry {
	return &dictEntryImpl{stem: stem, flags: flags, morph: morphData}
}

func (e *dictEntryImpl) GetStem() string              { return e.stem }
func (e *dictEntryImpl) GetFlags() string             { return e.flags }
func (e *dictEntryImpl) GetMorphologicalData() string { return e.morph }

func (e *dictEntryImpl) GetMorphologicalValues(key string) []string {
	data := e.morph
	if data == "" || !strings.Contains(data, key) {
		return nil
	}
	var result []string
	for _, field := range strings.Fields(data) {
		if strings.HasPrefix(field, key) {
			result = append(result, field[len(key):])
		}
	}
	return result
}

func (e *dictEntryImpl) String() string {
	var sb strings.Builder
	sb.WriteString(e.stem)
	if e.flags != "" {
		sb.WriteByte('/')
		sb.WriteString(e.flags)
	}
	if e.morph != "" {
		sb.WriteByte(' ')
		sb.WriteString(e.morph)
	}
	return sb.String()
}
