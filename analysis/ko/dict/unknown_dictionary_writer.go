// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// UnknownDictionaryWriter builds binary data for an UnknownDictionary from
// mecab-ko-dic unk.def and char.def source files.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.dict.UnknownDictionaryWriter from Apache
// Lucene 10.4.0.
//
// Deviation: the Java original extends BinaryDictionaryWriter and writes
// codec-formatted binary files plus a CharacterDefinition binary file.
// This Go port provides the public struct; binary serialisation is deferred
// to the nori codec sprint.
type UnknownDictionaryWriter struct {
	entries [][]byte
	posDict []string
}

// NewUnknownDictionaryWriter creates an UnknownDictionaryWriter with the given
// initial capacity hint.
func NewUnknownDictionaryWriter(size int) *UnknownDictionaryWriter {
	return &UnknownDictionaryWriter{
		entries: make([][]byte, 0, size),
	}
}

// AddEntry appends a binary dictionary entry and returns its word ID.
func (w *UnknownDictionaryWriter) AddEntry(data []byte) int {
	id := len(w.entries)
	w.entries = append(w.entries, data)
	return id
}

// AddPOS appends a POS tag string and returns its index.
func (w *UnknownDictionaryWriter) AddPOS(pos string) int {
	idx := len(w.posDict)
	w.posDict = append(w.posDict, pos)
	return idx
}

// Entries returns all accumulated binary entries.
func (w *UnknownDictionaryWriter) Entries() [][]byte { return w.entries }

// POSDict returns all accumulated POS tag strings.
func (w *UnknownDictionaryWriter) POSDict() []string { return w.posDict }
