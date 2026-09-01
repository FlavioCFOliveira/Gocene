// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"unicode"
)

// CharArrayMap is a simple class that stores key strings as slices in a hash table.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.CharArrayMap.
type CharArrayMap[V any] struct {
	ignoreCase bool
	count      int
	keys       [][]rune
	values     []V
}

func NewCharArrayMap[V any](startSize int, ignoreCase bool) *CharArrayMap[V] {
	size := 8
	for startSize+(startSize>>2) > size {
		size <<= 1
	}
	return &CharArrayMap[V]{
		ignoreCase: ignoreCase,
		keys:       make([][]rune, size),
		values:     make([]V, size),
	}
}

func (m *CharArrayMap[V]) Clear() {
	m.count = 0
	for i := range m.keys {
		m.keys[i] = nil
		var zero V
		m.values[i] = zero
	}
}

func (m *CharArrayMap[V]) ContainsKey(text []rune, off, len int) bool {
	return m.keys[m.getSlot(text, off, len)] != nil
}

func (m *CharArrayMap[V]) Get(text []rune, off, len int) (V, bool) {
	slot := m.getSlot(text, off, len)
	if m.keys[slot] == nil {
		var zero V
		return zero, false
	}
	return m.values[slot], true
}

func (m *CharArrayMap[V]) Put(text []rune, value V) (V, bool) {
	if m.ignoreCase {
		for i := range text {
			text[i] = unicode.ToLower(text[i])
		}
	}
	slot := m.getSlot(text, 0, len(text))
	if m.keys[slot] != nil {
		oldValue := m.values[slot]
		m.values[slot] = value
		return oldValue, true
	}
	m.keys[slot] = text
	m.values[slot] = value
	m.count++

	if m.count+(m.count>>2) > len(m.keys) {
		m.rehash()
	}
	var zero V
	return zero, false
}

func (m *CharArrayMap[V]) getSlot(text []rune, off, len int) int {
	code := m.getHashCode(text, off, len)
	pos := code & (len(m.keys) - 1)
	text2 := m.keys[pos]
	if text2 != nil && !m.equals(text, off, len, text2) {
		inc := ((code >> 8) + code) | 1
		for {
			code += inc
			pos = code & (len(m.keys) - 1)
			text2 = m.keys[pos]
			if text2 == nil || m.equals(text, off, len, text2) {
				break
			}
		}
	}
	return pos
}

func (m *CharArrayMap[V]) getHashCode(text []rune, off, len int) int {
	if text == nil {
		panic("null text")
	}
	code := 0
	if m.ignoreCase {
		for i := off; i < off+len; i++ {
			code = code*31 + int(unicode.ToLower(text[i]))
		}
	} else {
		for i := off; i < off+len; i++ {
			code = code*31 + int(text[i])
		}
	}
	return code
}

func (m *CharArrayMap[V]) equals(text1 []rune, off, len int, text2 []rune) bool {
	if len != len(text2) {
		return false
	}
	if m.ignoreCase {
		for i := 0; i < len; i++ {
			if unicode.ToLower(text1[off+i]) != unicode.ToLower(text2[i]) {
				return false
			}
		}
	} else {
		for i := 0; i < len; i++ {
			if text1[off+i] != text2[i] {
				return false
			}
		}
	}
	return true
}

func (m *CharArrayMap[V]) rehash() {
	oldKeys := m.keys
	oldValues := m.values
	newSize := 2 * len(m.keys)
	m.keys = make([][]rune, newSize)
	m.values = make([]V, newSize)
	m.count = 0

	for i := range oldKeys {
		if oldKeys[i] != nil {
			m.Put(oldKeys[i], oldValues[i])
		}
	}
}

func (m *CharArrayMap[V]) Size() int {
	return m.count
}

func (m *CharArrayMap[V]) Keys() [][]rune {
	var keys [][]rune
	for _, k := range m.keys {
		if k != nil {
			keys = append(keys, k)
		}
	}
	return keys
}

// CharArraySet is a simple class that stores strings as slices in a hash table.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.CharArraySet.
type CharArraySet struct {
	map *CharArrayMap[struct{}]
}

func NewCharArraySet(startSize int, ignoreCase bool) *CharArraySet {
	return &CharArraySet{
		map: NewCharArrayMap[struct{}](startSize, ignoreCase),
	}
}

func (s *CharArraySet) Clear() {
	s.map.Clear()
}

func (s *CharArraySet) Contains(text []rune, off, len int) bool {
	return s.map.ContainsKey(text, off, len)
}

func (s *CharArraySet) Add(text []rune) bool {
	_, exists := s.map.Put(text, struct{}{})
	return !exists
}

func (s *CharArraySet) Size() int {
	return s.map.Size()
}

func (s *CharArraySet) Iter() [][]rune {
	return s.map.Keys()
}

func (s *CharArraySet) String() string {
	keys := s.Iter()
	res := "["
	for i, k := range keys {
		if i > 0 {
			res += ", "
		}
		res += string(k)
	}
	res += "]"
	return res
}
