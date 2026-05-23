// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import (
	"encoding/binary"
	"fmt"
	"io"
)

const eom = '*'
const eomStr = "*"

// MultiTrie is a Trie of Tries. It stores words and their associated patch
// commands, handling each command individually.
//
// This is the Go port of org.egothor.stemmer.MultiTrie (Lucene 10.4.0).
type MultiTrie struct {
	Trie
	tries []*Trie
	by    int
}

// NewMultiTrieFromReader deserialises a MultiTrie from a Java DataInput stream.
func NewMultiTrieFromReader(r io.Reader) (*MultiTrie, error) {
	m := &MultiTrie{Trie: Trie{rows: []*Row{newRow()}, root: 0}}
	var fwd uint8
	if err := binary.Read(r, binary.BigEndian, &fwd); err != nil {
		return nil, err
	}
	m.forward = fwd != 0

	var by int32
	if err := binary.Read(r, binary.BigEndian, &by); err != nil {
		return nil, err
	}
	m.by = int(by)

	var n int32
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return nil, err
	}
	m.tries = make([]*Trie, n)
	for i := int32(0); i < n; i++ {
		t, err := NewTrieFromReader(r)
		if err != nil {
			return nil, err
		}
		m.tries[i] = t
	}
	return m, nil
}

// NewMultiTrie creates an empty MultiTrie.
func NewMultiTrie(forward bool) *MultiTrie {
	return &MultiTrie{
		Trie: Trie{rows: []*Row{newRow()}, root: 0, forward: forward},
		by:   1,
	}
}

// GetFully returns the patch command stored in the cell exactly associated
// with key.
func (m *MultiTrie) GetFully(key []rune) string {
	var result []rune
	for _, t := range m.tries {
		r := t.GetFully(key)
		if r == "" || (len(r) == 1 && r[0] == eom) {
			return string(result)
		}
		result = append(result, []rune(r)...)
	}
	return string(result)
}

// GetLastOnPath returns the patch command stored last on the path for key.
func (m *MultiTrie) GetLastOnPath(key []rune) string {
	var result []rune
	for _, t := range m.tries {
		r := t.GetLastOnPath(key)
		if r == "" || (len(r) == 1 && r[0] == eom) {
			return string(result)
		}
		result = append(result, []rune(r)...)
	}
	return string(result)
}

// Store serialises this MultiTrie using Java DataOutput wire format.
func (m *MultiTrie) Store(w io.Writer) error {
	var fwd uint8
	if m.forward {
		fwd = 1
	}
	if err := binary.Write(w, binary.BigEndian, fwd); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, int32(m.by)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, int32(len(m.tries))); err != nil {
		return err
	}
	for _, t := range m.tries {
		if err := t.Store(w); err != nil {
			return err
		}
	}
	return nil
}

// Add associates key with the given patch command cmd.
func (m *MultiTrie) Add(key []rune, cmd string) {
	if len(cmd) == 0 {
		return
	}
	levels := len([]rune(cmd)) / m.by
	for levels >= len(m.tries) {
		m.tries = append(m.tries, NewTrie(m.forward))
	}
	cmdRunes := []rune(cmd)
	for i := 0; i < levels; i++ {
		part := string(cmdRunes[m.by*i : m.by*i+m.by])
		m.tries[i].Add(key, part)
	}
	m.tries[levels].Add(key, eomStr)
}

// Reduce removes empty rows using the given Reducer and returns a new MultiTrie.
func (m *MultiTrie) Reduce(by Reducer) *MultiTrie {
	result := make([]*Trie, len(m.tries))
	for i, t := range m.tries {
		result[i] = t.Reduce(by)
	}
	nm := NewMultiTrie(m.forward)
	nm.tries = result
	return nm
}

// PrintInfo writes diagnostic info to w.
func (m *MultiTrie) PrintInfo(w io.Writer, prefix string) {
	c := 0
	for _, t := range m.tries {
		c++
		t.PrintInfo(w, fmt.Sprintf("%s[%d] ", prefix, c))
	}
}
