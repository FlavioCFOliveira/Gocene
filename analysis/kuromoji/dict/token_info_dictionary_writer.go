// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// TokenInfoDictionaryWriter writes a TokenInfoDictionary to binary files.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.TokenInfoDictionaryWriter from Apache
// Lucene 10.4.0.
//
// Deviation: the Java original extends BinaryDictionaryWriter and writes
// codec-formatted binary files plus an FST file. This Go port provides the
// public struct; binary serialisation is deferred to the codec sprint.
type TokenInfoDictionaryWriter struct {
	entries [][]byte
	posDict []string
}

// NewTokenInfoDictionaryWriter creates a TokenInfoDictionaryWriter with an
// initial capacity hint.
func NewTokenInfoDictionaryWriter(size int) *TokenInfoDictionaryWriter {
	return &TokenInfoDictionaryWriter{
		entries: make([][]byte, 0, size),
	}
}

// AddEntry appends a binary dictionary entry and returns its word ID.
func (w *TokenInfoDictionaryWriter) AddEntry(data []byte) int {
	id := len(w.entries)
	w.entries = append(w.entries, data)
	return id
}

// AddPOS appends a POS string and returns its index.
func (w *TokenInfoDictionaryWriter) AddPOS(pos string) int {
	idx := len(w.posDict)
	w.posDict = append(w.posDict, pos)
	return idx
}
