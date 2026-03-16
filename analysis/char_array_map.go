/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package analysis

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// CharArrayMap is a simple map that stores keys as char slices (rune slices in Go).
// It is designed for quick lookup of char[] keys without converting to string.
// Note: This is not a general-purpose map - it does not support removal and does not resize smaller.
//
// This is a port of Lucene's CharArrayMap.
type CharArrayMap[V any] struct {
	ignoreCase bool
	count      int
	keys       [][]rune
	values     []V
}

const initSize = 8

// NewCharArrayMap creates a new CharArrayMap with the given initial capacity and case sensitivity.
func NewCharArrayMap[V any](startSize int, ignoreCase bool) *CharArrayMap[V] {
	size := initSize
	for startSize+(startSize>>2) > size {
		size <<= 1
	}
	return &CharArrayMap[V]{
		ignoreCase: ignoreCase,
		keys:       make([][]rune, size),
		values:     make([]V, size),
	}
}

// NewCharArrayMapFromMap creates a new CharArrayMap from an existing map.
func NewCharArrayMapFromMap[V any](src map[string]V, ignoreCase bool) *CharArrayMap[V] {
	m := NewCharArrayMap[V](len(src), ignoreCase)
	for k, v := range src {
		m.Put(k, v)
	}
	return m
}

// Clear removes all entries from this map.
func (m *CharArrayMap[V]) Clear() {
	m.count = 0
	for i := range m.keys {
		m.keys[i] = nil
	}
	for i := range m.values {
		var zero V
		m.values[i] = zero
	}
}

// ContainsKey returns true if the given char slice is in the map.
func (m *CharArrayMap[V]) ContainsKey(text []rune, off, length int) bool {
	return m.keys[m.getSlot(text, off, length)] != nil
}

// ContainsKeyString returns true if the given string is in the map.
func (m *CharArrayMap[V]) ContainsKeyString(text string) bool {
	return m.keys[m.getSlotString(text)] != nil
}

// ContainsKeyInterface returns true if the given object is in the map.
// The object can be a string, []rune, or any type that has a String() method.
func (m *CharArrayMap[V]) ContainsKeyInterface(obj interface{}) bool {
	switch v := obj.(type) {
	case string:
		return m.ContainsKeyString(v)
	case []rune:
		return m.ContainsKey(v, 0, len(v))
	case fmt.Stringer:
		return m.ContainsKeyString(v.String())
	default:
		return m.ContainsKeyString(fmt.Sprint(v))
	}
}

// Get returns the value associated with the given char slice.
func (m *CharArrayMap[V]) Get(text []rune, off, length int) V {
	var zero V
	slot := m.getSlot(text, off, length)
	if m.keys[slot] == nil {
		return zero
	}
	return m.values[slot]
}

// GetString returns the value associated with the given string.
func (m *CharArrayMap[V]) GetString(text string) V {
	var zero V
	slot := m.getSlotString(text)
	if m.keys[slot] == nil {
		return zero
	}
	return m.values[slot]
}

// GetInterface returns the value associated with the given object.
func (m *CharArrayMap[V]) GetInterface(obj interface{}) V {
	switch v := obj.(type) {
	case string:
		return m.GetString(v)
	case []rune:
		return m.Get(v, 0, len(v))
	case fmt.Stringer:
		return m.GetString(v.String())
	default:
		return m.GetString(fmt.Sprint(v))
	}
}

// Put adds the given key-value pair to the map.
// If ignoreCase is true, the key will be converted to lower case.
// The key is stored as-is if it's already a []rune.
func (m *CharArrayMap[V]) Put(key string, value V) V {
	runes := []rune(key)
	return m.PutRunes(runes, value)
}

// PutRunes adds the given key-value pair to the map with a []rune key.
func (m *CharArrayMap[V]) PutRunes(text []rune, value V) V {
	if m.ignoreCase {
		ToLowerRunes(text, 0, len(text))
	}
	slot := m.getSlot(text, 0, len(text))
	if m.keys[slot] != nil {
		oldValue := m.values[slot]
		m.values[slot] = value
		return oldValue
	}
	m.keys[slot] = text
	m.values[slot] = value
	m.count++

	if m.count+(m.count>>2) > len(m.keys) {
		m.rehash()
	}

	var zero V
	return zero
}

// PutInterface adds the given key-value pair to the map.
// The key can be a string, []rune, or any type that can be converted to string.
func (m *CharArrayMap[V]) PutInterface(key interface{}, value V) V {
	switch v := key.(type) {
	case string:
		return m.Put(v, value)
	case []rune:
		return m.PutRunes(v, value)
	case fmt.Stringer:
		return m.Put(v.String(), value)
	default:
		return m.Put(fmt.Sprint(v), value)
	}
}

// Size returns the number of entries in this map.
func (m *CharArrayMap[V]) Size() int {
	return m.count
}

// IsEmpty returns true if this map is empty.
func (m *CharArrayMap[V]) IsEmpty() bool {
	return m.count == 0
}

// getSlot finds the slot for the given key.
func (m *CharArrayMap[V]) getSlot(text []rune, off, length int) int {
	code := m.getHashCode(text, off, length)
	pos := code & (len(m.keys) - 1)
	text2 := m.keys[pos]
	if text2 != nil && !m.equals(text, off, length, text2) {
		inc := ((code >> 8) + code) | 1
		for {
			code += inc
			pos = code & (len(m.keys) - 1)
			text2 = m.keys[pos]
			if text2 == nil || m.equals(text, off, length, text2) {
				break
			}
		}
	}
	return pos
}

// getSlotString finds the slot for the given string key.
func (m *CharArrayMap[V]) getSlotString(text string) int {
	code := m.getHashCodeString(text)
	pos := code & (len(m.keys) - 1)
	text2 := m.keys[pos]
	if text2 != nil && !m.equalsString(text, text2) {
		inc := ((code >> 8) + code) | 1
		for {
			code += inc
			pos = code & (len(m.keys) - 1)
			text2 = m.keys[pos]
			if text2 == nil || m.equalsString(text, text2) {
				break
			}
		}
	}
	return pos
}

// rehash doubles the size of the hash table and rehashes all entries.
func (m *CharArrayMap[V]) rehash() {
	newSize := 2 * len(m.keys)
	oldKeys := m.keys
	oldValues := m.values
	m.keys = make([][]rune, newSize)
	m.values = make([]V, newSize)

	for i := 0; i < len(oldKeys); i++ {
		text := oldKeys[i]
		if text != nil {
			slot := m.getSlot(text, 0, len(text))
			m.keys[slot] = text
			m.values[slot] = oldValues[i]
		}
	}
}

// equals compares two char arrays for equality.
func (m *CharArrayMap[V]) equals(text1 []rune, off, length int, text2 []rune) bool {
	if length != len(text2) {
		return false
	}
	if m.ignoreCase {
		for i := 0; i < length; {
			codePointAt := text1[off+i]
			if unicode.ToLower(codePointAt) != text2[i] {
				return false
			}
			i++
		}
	} else {
		for i := 0; i < length; i++ {
			if text1[off+i] != text2[i] {
				return false
			}
		}
	}
	return true
}

// equalsString compares a string with a char array for equality.
func (m *CharArrayMap[V]) equalsString(text1 string, text2 []rune) bool {
	runes := []rune(text1)
	if len(runes) != len(text2) {
		return false
	}
	if m.ignoreCase {
		for i := 0; i < len(runes); i++ {
			if unicode.ToLower(runes[i]) != unicode.ToLower(text2[i]) {
				return false
			}
		}
	} else {
		for i := 0; i < len(runes); i++ {
			if runes[i] != text2[i] {
				return false
			}
		}
	}
	return true
}

// getHashCode computes the hash code for a char array.
func (m *CharArrayMap[V]) getHashCode(text []rune, off, length int) int {
	if text == nil {
		panic("null text")
	}
	var code int
	if m.ignoreCase {
		stop := off + length
		for i := off; i < stop; i++ {
			codePointAt := text[i]
			code = code*31 + int(unicode.ToLower(codePointAt))
		}
	} else {
		for i := off; i < off+length; i++ {
			code = code*31 + int(text[i])
		}
	}
	return code
}

// getHashCodeString computes the hash code for a string.
func (m *CharArrayMap[V]) getHashCodeString(text string) int {
	if text == "" {
		return 0
	}
	var code int
	if m.ignoreCase {
		for _, r := range text {
			code = code*31 + int(unicode.ToLower(r))
		}
	} else {
		for _, r := range text {
			code = code*31 + int(r)
		}
	}
	return code
}

// String returns a string representation of this map.
func (m *CharArrayMap[V]) String() string {
	result := "{"
	first := true
	for i := 0; i < len(m.keys); i++ {
		if m.keys[i] != nil {
			if !first {
				result += ", "
			}
			first = false
			result += fmt.Sprintf("%s=%v", string(m.keys[i]), m.values[i])
		}
	}
	result += "}"
	return result
}

// Keys returns all keys in this map.
func (m *CharArrayMap[V]) Keys() [][]rune {
	keys := make([][]rune, 0, m.count)
	for i := 0; i < len(m.keys); i++ {
		if m.keys[i] != nil {
			keys = append(keys, m.keys[i])
		}
	}
	return keys
}

// Values returns all values in this map.
func (m *CharArrayMap[V]) Values() []V {
	values := make([]V, 0, m.count)
	for i := 0; i < len(m.values); i++ {
		if m.keys[i] != nil {
			values = append(values, m.values[i])
		}
	}
	return values
}

// Entry represents a key-value pair in the CharArrayMap.
type Entry[V any] struct {
	Key   []rune
	Value V
}

// Entries returns all key-value pairs in this map.
func (m *CharArrayMap[V]) Entries() []Entry[V] {
	entries := make([]Entry[V], 0, m.count)
	for i := 0; i < len(m.keys); i++ {
		if m.keys[i] != nil {
			entries = append(entries, Entry[V]{
				Key:   m.keys[i],
				Value: m.values[i],
			})
		}
	}
	return entries
}

// ForEach iterates over all entries in the map.
func (m *CharArrayMap[V]) ForEach(fn func(key []rune, value V) bool) {
	for i := 0; i < len(m.keys); i++ {
		if m.keys[i] != nil {
			if !fn(m.keys[i], m.values[i]) {
				break
			}
		}
	}
}

// ToMap converts this CharArrayMap to a standard Go map.
func (m *CharArrayMap[V]) ToMap() map[string]V {
	result := make(map[string]V, m.count)
	for i := 0; i < len(m.keys); i++ {
		if m.keys[i] != nil {
			result[string(m.keys[i])] = m.values[i]
		}
	}
	return result
}

// Copy returns a copy of the given CharArrayMap.
// If the input is nil, returns nil.
// The ignoreCase property is preserved.
func CopyCharArrayMap[V any](src *CharArrayMap[V]) *CharArrayMap[V] {
	if src == nil {
		return nil
	}
	if src.count == 0 {
		return NewCharArrayMap[V](0, src.ignoreCase)
	}

	newMap := &CharArrayMap[V]{
		ignoreCase: src.ignoreCase,
		count:      src.count,
		keys:       make([][]rune, len(src.keys)),
		values:     make([]V, len(src.values)),
	}

	copy(newMap.keys, src.keys)
	copy(newMap.values, src.values)

	return newMap
}

// UnmodifiableCharArrayMap wraps a CharArrayMap to make it unmodifiable.
type UnmodifiableCharArrayMap[V any] struct {
	*CharArrayMap[V]
}

// NewUnmodifiableCharArrayMap creates an unmodifiable view of the given CharArrayMap.
func NewUnmodifiableCharArrayMap[V any](m *CharArrayMap[V]) *UnmodifiableCharArrayMap[V] {
	return &UnmodifiableCharArrayMap[V]{CharArrayMap: m}
}

// Clear panics for unmodifiable maps.
func (m *UnmodifiableCharArrayMap[V]) Clear() {
	panic("unmodifiable map")
}

// Put panics for unmodifiable maps.
func (m *UnmodifiableCharArrayMap[V]) Put(key string, value V) V {
	panic("unmodifiable map")
}

// PutRunes panics for unmodifiable maps.
func (m *UnmodifiableCharArrayMap[V]) PutRunes(text []rune, value V) V {
	panic("unmodifiable map")
}

// PutInterface panics for unmodifiable maps.
func (m *UnmodifiableCharArrayMap[V]) PutInterface(key interface{}, value V) V {
	panic("unmodifiable map")
}

// EmptyCharArrayMap is an empty, unmodifiable CharArrayMap optimized for speed.
// Contains checks always return false or panic on null.
type EmptyCharArrayMap[V any] struct {
	*UnmodifiableCharArrayMap[V]
}

// NewEmptyCharArrayMap creates an empty CharArrayMap.
func NewEmptyCharArrayMap[V any]() *EmptyCharArrayMap[V] {
	return &EmptyCharArrayMap[V]{
		UnmodifiableCharArrayMap: &UnmodifiableCharArrayMap[V]{
			CharArrayMap: NewCharArrayMap[V](0, false),
		},
	}
}

// ContainsKey returns false (empty map).
func (m *EmptyCharArrayMap[V]) ContainsKey(text []rune, off, length int) bool {
	if text == nil {
		panic("null text")
	}
	return false
}

// ContainsKeyString returns false (empty map).
func (m *EmptyCharArrayMap[V]) ContainsKeyString(text string) bool {
	if text == "" {
		return false
	}
	return false
}

// ContainsKeyInterface returns false (empty map).
func (m *EmptyCharArrayMap[V]) ContainsKeyInterface(obj interface{}) bool {
	if obj == nil {
		panic("null object")
	}
	return false
}

// Get returns the zero value (empty map).
func (m *EmptyCharArrayMap[V]) Get(text []rune, off, length int) V {
	var zero V
	if text == nil {
		panic("null text")
	}
	return zero
}

// GetString returns the zero value (empty map).
func (m *EmptyCharArrayMap[V]) GetString(text string) V {
	var zero V
	return zero
}

// GetInterface returns the zero value (empty map).
func (m *EmptyCharArrayMap[V]) GetInterface(obj interface{}) V {
	var zero V
	if obj == nil {
		panic("null object")
	}
	return zero
}

// ToLowerRunes converts all runes in the slice to lower case in place.
func ToLowerRunes(runes []rune, off, length int) {
	for i := off; i < off+length && i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
}

// ToUpperRunes converts all runes in the slice to upper case in place.
func ToUpperRunes(runes []rune, off, length int) {
	for i := off; i < off+length && i < len(runes); i++ {
		runes[i] = unicode.ToUpper(runes[i])
	}
}

// HashCode computes a hash code for a string (similar to Java's String.hashCode).
func HashCode(s string) int {
	h := 0
	for _, r := range s {
		h = 31*h + int(r)
	}
	return h
}

// HashCodeRunes computes a hash code for a rune slice.
func HashCodeRunes(runes []rune, off, length int) int {
	h := 0
	for i := off; i < off+length && i < len(runes); i++ {
		h = 31*h + int(runes[i])
	}
	return h
}

// RunesToString converts a rune slice with offset and length to a string.
func RunesToString(runes []rune, off, length int) string {
	if off < 0 || length < 0 || off+length > len(runes) {
		return ""
	}
	return string(runes[off : off+length])
}

// StringToRunes converts a string to a rune slice.
func StringToRunes(s string) []rune {
	return []rune(s)
}

// EqualsRunes compares two rune slices for equality.
func EqualsRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// EqualsRunesIgnoreCase compares two rune slices for equality, ignoring case.
func EqualsRunesIgnoreCase(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if unicode.ToLower(a[i]) != unicode.ToLower(b[i]) {
			return false
		}
	}
	return true
}

// RuneLen returns the number of bytes needed to encode the rune.
func RuneLen(r rune) int {
	return utf8.RuneLen(r)
}
