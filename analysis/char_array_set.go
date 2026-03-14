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
)

// placeholder is used as the value in the underlying map for CharArraySet.
var placeholder = struct{}{}

// CharArraySet is a simple set that stores Strings as char[]'s in a hash table.
// Note that this is not a general-purpose set - it cannot remove items and does not resize smaller.
// It is designed to quickly test if a char[] is in the set without converting it to a string first.
//
// This is a port of Lucene's CharArraySet.
type CharArraySet struct {
	m *CharArrayMap[struct{}]
}

// NewCharArraySet creates a new CharArraySet with the given initial capacity and case sensitivity.
func NewCharArraySet(startSize int, ignoreCase bool) *CharArraySet {
	return &CharArraySet{
		m: NewCharArrayMap[struct{}](startSize, ignoreCase),
	}
}

// NewCharArraySetFromCollection creates a new CharArraySet from a collection of strings.
func NewCharArraySetFromCollection(items []string, ignoreCase bool) *CharArraySet {
	s := NewCharArraySet(len(items), ignoreCase)
	for _, item := range items {
		s.Add(item)
	}
	return s
}

// NewCharArraySetFromStrings creates a new CharArraySet from variadic string arguments.
func NewCharArraySetFromStrings(ignoreCase bool, items ...string) *CharArraySet {
	s := NewCharArraySet(len(items), ignoreCase)
	for _, item := range items {
		s.Add(item)
	}
	return s
}

// Clear removes all entries from this set.
// This method is supported for reusing, but not remove.
func (s *CharArraySet) Clear() {
	s.m.Clear()
}

// Contains returns true if the given char slice is in the set.
func (s *CharArraySet) Contains(text []rune, off, length int) bool {
	return s.m.ContainsKey(text, off, length)
}

// ContainsString returns true if the given string is in the set.
func (s *CharArraySet) ContainsString(text string) bool {
	return s.m.ContainsKeyString(text)
}

// ContainsInterface returns true if the given object is in the set.
// The object can be a string, []rune, or any type that has a String() method.
func (s *CharArraySet) ContainsInterface(obj interface{}) bool {
	return s.m.ContainsKeyInterface(obj)
}

// Add adds the given string to the set.
// Returns true if the element was added (was not already present).
func (s *CharArraySet) Add(text string) bool {
	// Check if already present before adding
	if s.m.ContainsKeyString(text) {
		return false
	}
	s.m.Put(text, placeholder)
	return true
}

// AddRunes adds the given char slice to the set.
// If ignoreCase is true for this Set, the text array will be directly modified.
// The user should never modify this text array after calling this method.
// Returns true if the element was added (was not already present).
func (s *CharArraySet) AddRunes(text []rune) bool {
	// Check if already present before adding
	if s.m.ContainsKey(text, 0, len(text)) {
		return false
	}
	s.m.PutRunes(text, placeholder)
	return true
}

// AddInterface adds the given object to the set.
// Returns true if the element was added (was not already present).
func (s *CharArraySet) AddInterface(obj interface{}) bool {
	// Check if already present before adding
	if s.m.ContainsKeyInterface(obj) {
		return false
	}
	s.m.PutInterface(obj, placeholder)
	return true
}

// AddAll adds all elements from the given slice to the set.
func (s *CharArraySet) AddAll(items []string) {
	for _, item := range items {
		s.Add(item)
	}
}

// Size returns the number of elements in this set.
func (s *CharArraySet) Size() int {
	return s.m.Size()
}

// IsEmpty returns true if this set is empty.
func (s *CharArraySet) IsEmpty() bool {
	return s.m.IsEmpty()
}

// String returns a string representation of this set.
func (s *CharArraySet) String() string {
	result := "["
	first := true
	for i := 0; i < len(s.m.keys); i++ {
		if s.m.keys[i] != nil {
			if !first {
				result += ", "
			}
			first = false
			result += string(s.m.keys[i])
		}
	}
	result += "]"
	return result
}

// Items returns all elements in this set as strings.
func (s *CharArraySet) Items() []string {
	items := make([]string, 0, s.m.count)
	for i := 0; i < len(s.m.keys); i++ {
		if s.m.keys[i] != nil {
			items = append(items, string(s.m.keys[i]))
		}
	}
	return items
}

// Runes returns all elements in this set as rune slices.
func (s *CharArraySet) Runes() [][]rune {
	runes := make([][]rune, 0, s.m.count)
	for i := 0; i < len(s.m.keys); i++ {
		if s.m.keys[i] != nil {
			runes = append(runes, s.m.keys[i])
		}
	}
	return runes
}

// ForEach iterates over all elements in the set.
func (s *CharArraySet) ForEach(fn func(item string) bool) {
	s.m.ForEach(func(key []rune, _ struct{}) bool {
		return fn(string(key))
	})
}

// ForEachRunes iterates over all elements in the set as rune slices.
func (s *CharArraySet) ForEachRunes(fn func(item []rune) bool) {
	s.m.ForEach(func(key []rune, _ struct{}) bool {
		return fn(key)
	})
}

// ContainsAll returns true if this set contains all elements of the given collection.
func (s *CharArraySet) ContainsAll(items []string) bool {
	for _, item := range items {
		if !s.ContainsString(item) {
			return false
		}
	}
	return true
}

// ContainsAllRunes returns true if this set contains all elements of the given rune slices.
func (s *CharArraySet) ContainsAllRunes(items [][]rune) bool {
	for _, item := range items {
		if !s.Contains(item, 0, len(item)) {
			return false
		}
	}
	return true
}

// UnmodifiableCharArraySet wraps a CharArraySet to make it unmodifiable.
type UnmodifiableCharArraySet struct {
	*CharArraySet
}

// NewUnmodifiableCharArraySet creates an unmodifiable view of the given CharArraySet.
func NewUnmodifiableCharArraySet(set *CharArraySet) *UnmodifiableCharArraySet {
	if set == nil {
		panic("null set")
	}
	return &UnmodifiableCharArraySet{CharArraySet: set}
}

// Add panics for unmodifiable sets.
func (s *UnmodifiableCharArraySet) Add(text string) bool {
	panic("unmodifiable set")
}

// AddRunes panics for unmodifiable sets.
func (s *UnmodifiableCharArraySet) AddRunes(text []rune) bool {
	panic("unmodifiable set")
}

// AddInterface panics for unmodifiable sets.
func (s *UnmodifiableCharArraySet) AddInterface(obj interface{}) bool {
	panic("unmodifiable set")
}

// AddAll panics for unmodifiable sets.
func (s *UnmodifiableCharArraySet) AddAll(items []string) {
	panic("unmodifiable set")
}

// Clear panics for unmodifiable sets.
func (s *UnmodifiableCharArraySet) Clear() {
	panic("unmodifiable set")
}

// EmptyCharArraySet is an empty, unmodifiable CharArraySet.
// This is optimized for speed - contains checks always return false.
type EmptyCharArraySet struct {
	*UnmodifiableCharArraySet
}

// NewEmptyCharArraySet creates an empty CharArraySet.
func NewEmptyCharArraySet() *EmptyCharArraySet {
	return &EmptyCharArraySet{
		UnmodifiableCharArraySet: &UnmodifiableCharArraySet{
			CharArraySet: NewCharArraySet(0, false),
		},
	}
}

// Contains returns false (empty set).
func (s *EmptyCharArraySet) Contains(text []rune, off, length int) bool {
	if text == nil {
		panic("null text")
	}
	return false
}

// ContainsString returns false (empty set).
func (s *EmptyCharArraySet) ContainsString(text string) bool {
	return false
}

// ContainsInterface returns false (empty set).
func (s *EmptyCharArraySet) ContainsInterface(obj interface{}) bool {
	if obj == nil {
		panic("null object")
	}
	return false
}

// Size returns 0 (empty set).
func (s *EmptyCharArraySet) Size() int {
	return 0
}

// IsEmpty returns true (empty set).
func (s *EmptyCharArraySet) IsEmpty() bool {
	return true
}

// String returns "[]" (empty set).
func (s *EmptyCharArraySet) String() string {
	return "[]"
}

// Items returns nil (empty set).
func (s *EmptyCharArraySet) Items() []string {
	return nil
}

// Runes returns nil (empty set).
func (s *EmptyCharArraySet) Runes() [][]rune {
	return nil
}

// UnmodifiableSet returns an unmodifiable view of the given set.
// If the set is already unmodifiable, returns the same set.
// If the set is nil, panics.
func UnmodifiableSet(set *CharArraySet) *UnmodifiableCharArraySet {
	if set == nil {
		panic("null set")
	}
	return NewUnmodifiableCharArraySet(set)
}

// Copy returns a copy of the given set as a CharArraySet.
// If the given set is a CharArraySet, the ignoreCase property is preserved.
func CopySet(set *CharArraySet) *CharArraySet {
	if set == nil || set.Size() == 0 {
		return NewCharArraySet(0, false)
	}

	// Create a new set with the same capacity and ignoreCase setting
	result := NewCharArraySet(set.Size(), set.m.ignoreCase)

	// Copy all elements
	for i := 0; i < len(set.m.keys); i++ {
		if set.m.keys[i] != nil {
			// Make a copy of the key slice to avoid mutations
			keyCopy := make([]rune, len(set.m.keys[i]))
			copy(keyCopy, set.m.keys[i])
			result.AddRunes(keyCopy)
		}
	}

	return result
}

// CopyStrings returns a copy of the given strings as a CharArraySet.
func CopyStrings(items []string, ignoreCase bool) *CharArraySet {
	return NewCharArraySetFromCollection(items, ignoreCase)
}

// CopyInterfaceSet returns a copy of the given set (interface version).
// This is similar to Lucene's copy method that accepts Set<?>.
func CopyInterfaceSet(items []interface{}, ignoreCase bool) *CharArraySet {
	s := NewCharArraySet(len(items), ignoreCase)
	for _, item := range items {
		s.AddInterface(item)
	}
	return s
}

// SetEquals returns true if two CharArraySets contain the same elements.
func SetEquals(a, b *CharArraySet) bool {
	if a.Size() != b.Size() {
		return false
	}
	for i := 0; i < len(a.m.keys); i++ {
		if a.m.keys[i] != nil {
			if !b.Contains(a.m.keys[i], 0, len(a.m.keys[i])) {
				return false
			}
		}
	}
	return true
}

// SetUnion returns a new set containing all elements from both sets.
func SetUnion(a, b *CharArraySet) *CharArraySet {
	ignoreCase := a.m.ignoreCase
	result := NewCharArraySet(a.Size()+b.Size(), ignoreCase)

	// Add all from a
	for i := 0; i < len(a.m.keys); i++ {
		if a.m.keys[i] != nil {
			result.AddRunes(a.m.keys[i])
		}
	}

	// Add all from b
	for i := 0; i < len(b.m.keys); i++ {
		if b.m.keys[i] != nil {
			result.AddRunes(b.m.keys[i])
		}
	}

	return result
}

// SetIntersection returns a new set containing elements present in both sets.
func SetIntersection(a, b *CharArraySet) *CharArraySet {
	ignoreCase := a.m.ignoreCase
	result := NewCharArraySet(min(a.Size(), b.Size()), ignoreCase)

	// Iterate over smaller set
	smaller := a
	larger := b
	if a.Size() > b.Size() {
		smaller = b
		larger = a
	}

	for i := 0; i < len(smaller.m.keys); i++ {
		if smaller.m.keys[i] != nil {
			if larger.Contains(smaller.m.keys[i], 0, len(smaller.m.keys[i])) {
				result.AddRunes(smaller.m.keys[i])
			}
		}
	}

	return result
}

// SetDifference returns a new set containing elements in a but not in b.
func SetDifference(a, b *CharArraySet) *CharArraySet {
	ignoreCase := a.m.ignoreCase
	result := NewCharArraySet(a.Size(), ignoreCase)

	for i := 0; i < len(a.m.keys); i++ {
		if a.m.keys[i] != nil {
			if !b.Contains(a.m.keys[i], 0, len(a.m.keys[i])) {
				result.AddRunes(a.m.keys[i])
			}
		}
	}

	return result
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Format returns a formatted string representation using the given format.
func (s *CharArraySet) Format(format string) string {
	if format == "" {
		return s.String()
	}
	return fmt.Sprintf(format, s.Items())
}