// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// UnknownDictionaryWriter builds binary data for an UnknownDictionary.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.UnknownDictionaryWriter from Apache
// Lucene 10.4.0.
//
// Deviation: the Java original extends BinaryDictionaryWriter and writes
// codec-formatted binary files. This Go port provides the public struct;
// binary serialisation is deferred to the codec sprint.
type UnknownDictionaryWriter struct {
	entries [][]byte
	posDict []string
}

// NewUnknownDictionaryWriter creates an UnknownDictionaryWriter.
func NewUnknownDictionaryWriter() *UnknownDictionaryWriter {
	return &UnknownDictionaryWriter{}
}

// AddEntry appends a binary dictionary entry and returns its word ID.
func (w *UnknownDictionaryWriter) AddEntry(data []byte) int {
	id := len(w.entries)
	w.entries = append(w.entries, data)
	return id
}

// AddPOS appends a POS string and returns its index.
func (w *UnknownDictionaryWriter) AddPOS(pos string) int {
	idx := len(w.posDict)
	w.posDict = append(w.posDict, pos)
	return idx
}
