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
	"math/rand"
	"strings"
	"testing"
)

func TestCharArrayMap_NewCharArrayMap(t *testing.T) {
	tests := []struct {
		name       string
		startSize  int
		ignoreCase bool
		wantEmpty  bool
	}{
		{"empty map", 0, true, true},
		{"small map", 10, true, true},
		{"large map", 100, false, true},
		{"case sensitive", 10, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewCharArrayMap[int](tt.startSize, tt.ignoreCase)
			if m.IsEmpty() != tt.wantEmpty {
				t.Errorf("NewCharArrayMap(%d, %v).IsEmpty() = %v, want %v",
					tt.startSize, tt.ignoreCase, m.IsEmpty(), tt.wantEmpty)
			}
			if m.Size() != 0 {
				t.Errorf("NewCharArrayMap(%d, %v).Size() = %d, want 0",
					tt.startSize, tt.ignoreCase, m.Size())
			}
		})
	}
}

func TestCharArrayMap_BasicOperations(t *testing.T) {
	m := NewCharArrayMap[string](10, false)

	// Test Put and Get
	m.Put("foo", "bar")
	if m.Size() != 1 {
		t.Errorf("Size() = %d, want 1", m.Size())
	}
	if v := m.GetString("foo"); v != "bar" {
		t.Errorf("GetString('foo') = %v, want 'bar'", v)
	}
	if !m.ContainsKeyString("foo") {
		t.Error("ContainsKeyString('foo') = false, want true")
	}

	// Test Put with runes
	runes := []rune("hello")
	m.PutRunes(runes, "world")
	if !m.ContainsKey(runes, 0, len(runes)) {
		t.Error("ContainsKey('hello') = false, want true")
	}
	if v := m.Get(runes, 0, len(runes)); v != "world" {
		t.Errorf("Get('hello') = %v, want 'world'", v)
	}

	// Test PutInterface
	m.PutInterface(42, "answer")
	if !m.ContainsKeyInterface(42) {
		t.Error("ContainsKeyInterface(42) = false, want true")
	}
	if v := m.GetInterface(42); v != "answer" {
		t.Errorf("GetInterface(42) = %v, want 'answer'", v)
	}
}

func TestCharArrayMap_Clear(t *testing.T) {
	m := NewCharArrayMap[int](10, false)
	m.Put("foo", 1)
	m.Put("bar", 2)
	m.Put("baz", 3)

	if m.Size() != 3 {
		t.Errorf("Size() = %d, want 3", m.Size())
	}

	m.Clear()
	if m.Size() != 0 {
		t.Errorf("After Clear(), Size() = %d, want 0", m.Size())
	}
	if m.ContainsKeyString("foo") {
		t.Error("After Clear(), should not contain 'foo'")
	}
}

func TestCharArrayMap_CaseInsensitive(t *testing.T) {
	m := NewCharArrayMap[string](10, true) // case insensitive

	m.Put("Hello", "World")

	// Should find it with different case
	if !m.ContainsKeyString("hello") {
		t.Error("Case insensitive map should contain 'hello'")
	}
	if !m.ContainsKeyString("HELLO") {
		t.Error("Case insensitive map should contain 'HELLO'")
	}
	if !m.ContainsKeyString("Hello") {
		t.Error("Case insensitive map should contain 'Hello'")
	}

	if v := m.GetString("hello"); v != "World" {
		t.Errorf("GetString('hello') = %v, want 'World'", v)
	}
	if v := m.GetString("HELLO"); v != "World" {
		t.Errorf("GetString('HELLO') = %v, want 'World'", v)
	}
}

func TestCharArrayMap_CaseSensitive(t *testing.T) {
	m := NewCharArrayMap[string](10, false) // case sensitive

	m.Put("Hello", "World")

	// Should NOT find it with different case
	if m.ContainsKeyString("hello") {
		t.Error("Case sensitive map should NOT contain 'hello'")
	}
	if m.ContainsKeyString("HELLO") {
		t.Error("Case sensitive map should NOT contain 'HELLO'")
	}
	if !m.ContainsKeyString("Hello") {
		t.Error("Case sensitive map should contain 'Hello'")
	}

	if v := m.GetString("hello"); v != "" {
		t.Errorf("GetString('hello') = %v, want ''", v)
	}
	if v := m.GetString("Hello"); v != "World" {
		t.Errorf("GetString('Hello') = %v, want 'World'", v)
	}
}

func TestCharArrayMap_RandomOperations(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	for i := 0; i < 100; i++ {
		doRandom(t, r, 100, true)  // case insensitive
		doRandom(t, r, 100, false) // case sensitive
	}
}

func doRandom(t *testing.T, r *rand.Rand, iterations int, ignoreCase bool) {
	cm := NewCharArrayMap[int](1, ignoreCase)
	hm := make(map[string]int)

	for i := 0; i < iterations; i++ {
		// Generate random key
		keyLen := r.Intn(5)
		key := make([]rune, keyLen)
		for j := 0; j < keyLen; j++ {
			key[j] = rune(r.Intn(127))
		}
		keyStr := string(key)

		// For case insensitive, use lowercase as hashmap key
		hmapKey := keyStr
		if ignoreCase {
			hmapKey = strings.ToLower(keyStr)
		}

		val := r.Intn(1000)

		// Put
		cm.Put(keyStr, val)
		hm[hmapKey] = val

		// Verify
		if cm.Size() != len(hm) {
			t.Errorf("Size() = %d, want %d", cm.Size(), len(hm))
		}

		if v := cm.GetString(keyStr); v != val {
			t.Errorf("GetString('%s') = %v, want %d", keyStr, v, val)
		}

		if v := cm.Get(key, 0, keyLen); v != val {
			t.Errorf("Get('%s') = %v, want %d", keyStr, v, val)
		}
	}
}

func TestCharArrayMap_KeysValues(t *testing.T) {
	m := NewCharArrayMap[int](10, false)
	m.Put("foo", 1)
	m.Put("bar", 2)
	m.Put("baz", 3)

	keys := m.Keys()
	if len(keys) != 3 {
		t.Errorf("len(Keys()) = %d, want 3", len(keys))
	}

	values := m.Values()
	if len(values) != 3 {
		t.Errorf("len(Values()) = %d, want 3", len(values))
	}

	// Check all values are present
	valueSet := make(map[int]bool)
	for _, v := range values {
		valueSet[v] = true
	}
	if !valueSet[1] || !valueSet[2] || !valueSet[3] {
		t.Error("Values() missing expected values")
	}
}

func TestCharArrayMap_Entries(t *testing.T) {
	m := NewCharArrayMap[int](10, false)
	m.Put("foo", 1)
	m.Put("bar", 2)

	entries := m.Entries()
	if len(entries) != 2 {
		t.Errorf("len(Entries()) = %d, want 2", len(entries))
	}

	// Check entries contain correct data
	entryMap := make(map[string]int)
	for _, e := range entries {
		entryMap[string(e.Key)] = e.Value
	}
	if entryMap["foo"] != 1 {
		t.Errorf("Entry for 'foo' = %d, want 1", entryMap["foo"])
	}
	if entryMap["bar"] != 2 {
		t.Errorf("Entry for 'bar' = %d, want 2", entryMap["bar"])
	}
}

func TestCharArrayMap_ForEach(t *testing.T) {
	m := NewCharArrayMap[string](10, false)
	m.Put("foo", "bar")
	m.Put("hello", "world")

	count := 0
	visited := make(map[string]string)
	m.ForEach(func(key []rune, value string) bool {
		count++
		visited[string(key)] = value
		return true
	})

	if count != 2 {
		t.Errorf("ForEach visited %d items, want 2", count)
	}
	if visited["foo"] != "bar" {
		t.Errorf("ForEach visited foo=%s, want bar", visited["foo"])
	}
	if visited["hello"] != "world" {
		t.Errorf("ForEach visited hello=%s, want world", visited["hello"])
	}
}

func TestCharArrayMap_EarlyTermination(t *testing.T) {
	m := NewCharArrayMap[int](10, false)
	for i := 0; i < 10; i++ {
		m.Put(fmt.Sprintf("key%d", i), i)
	}

	count := 0
	m.ForEach(func(key []rune, value int) bool {
		count++
		return count < 3 // Stop after 3
	})

	if count != 3 {
		t.Errorf("ForEach should have stopped early, visited %d items", count)
	}
}

func TestCharArrayMap_ToMap(t *testing.T) {
	m := NewCharArrayMap[string](10, false)
	m.Put("foo", "bar")
	m.Put("hello", "world")

	result := m.ToMap()
	if len(result) != 2 {
		t.Errorf("ToMap() returned %d items, want 2", len(result))
	}
	if result["foo"] != "bar" {
		t.Errorf("ToMap()['foo'] = %s, want bar", result["foo"])
	}
	if result["hello"] != "world" {
		t.Errorf("ToMap()['hello'] = %s, want world", result["hello"])
	}
}

func TestCharArrayMap_Unmodifiable(t *testing.T) {
	m := NewCharArrayMap[int](10, false)
	m.Put("foo", 1)
	m.Put("bar", 2)
	size := m.Size()

	um := NewUnmodifiableCharArrayMap[int](m)
	if um.Size() != size {
		t.Errorf("Unmodifiable map size = %d, want %d", um.Size(), size)
	}

	// Verify content
	if !um.ContainsKeyString("foo") {
		t.Error("Unmodifiable map should contain 'foo'")
	}
	if !um.ContainsKeyString("bar") {
		t.Error("Unmodifiable map should contain 'bar'")
	}

	// Test that modification methods panic
	t.Run("Put panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Put should panic on unmodifiable map")
			}
		}()
		um.Put("test", 3)
	})

	t.Run("PutRunes panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("PutRunes should panic on unmodifiable map")
			}
		}()
		um.PutRunes([]rune("test"), 3)
	})

	t.Run("Clear panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Clear should panic on unmodifiable map")
			}
		}()
		um.Clear()
	})

	// Verify size unchanged after attempted modifications
	if um.Size() != size {
		t.Errorf("Unmodifiable map size changed to %d, want %d", um.Size(), size)
	}
}

func TestCharArrayMap_Empty(t *testing.T) {
	em := NewEmptyCharArrayMap[int]()

	if em.Size() != 0 {
		t.Errorf("Empty map size = %d, want 0", em.Size())
	}
	if !em.IsEmpty() {
		t.Error("Empty map should be empty")
	}

	// Test contains returns false
	if em.ContainsKeyString("foo") {
		t.Error("Empty map should not contain 'foo'")
	}
	if em.ContainsKeyInterface("foo") {
		t.Error("Empty map should not contain 'foo' (interface)")
	}

	// Test panic on nil
	t.Run("ContainsKey panics on nil", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("ContainsKey(nil) should panic")
			}
		}()
		em.ContainsKey(nil, 0, 0)
	})
}

func TestCharArrayMap_Copy(t *testing.T) {
	t.Run("copy case insensitive", func(t *testing.T) {
		original := NewCharArrayMap[int](10, true)
		original.Put("foo", 1)
		original.Put("bar", 2)
		original.PutInterface(42, 3)

		copy := CopyCharArrayMap(original)
		if copy.Size() != original.Size() {
			t.Errorf("Copy size = %d, want %d", copy.Size(), original.Size())
		}

		// Verify all keys are present
		if !copy.ContainsKeyString("foo") {
			t.Error("Copy should contain 'foo'")
		}
		if !copy.ContainsKeyString("FOO") { // case insensitive
			t.Error("Copy should contain 'FOO' (case insensitive)")
		}

		// Modify copy should not affect original
		copy.Put("new", 4)
		if original.ContainsKeyString("new") {
			t.Error("Original should not contain 'new'")
		}
	})

	t.Run("copy case sensitive", func(t *testing.T) {
		original := NewCharArrayMap[int](10, false)
		original.Put("foo", 1)
		original.Put("bar", 2)

		copy := CopyCharArrayMap(original)
		if copy.Size() != original.Size() {
			t.Errorf("Copy size = %d, want %d", copy.Size(), original.Size())
		}

		// Case sensitive - FOO should NOT match
		if copy.ContainsKeyString("FOO") {
			t.Error("Copy should NOT contain 'FOO' (case sensitive)")
		}
	})
}

func TestCharArrayMap_String(t *testing.T) {
	m := NewCharArrayMap[int](10, false)
	m.Put("test", 1)

	// Check toString contains expected content
	s := m.String()
	if !strings.Contains(s, "test") {
		t.Errorf("String() = %s, should contain 'test'", s)
	}
	if !strings.Contains(s, "1") {
		t.Errorf("String() = %s, should contain '1'", s)
	}
}

func TestCharArrayMap_LargeCapacity(t *testing.T) {
	m := NewCharArrayMap[int](10000, true)

	for i := 0; i < 10000; i++ {
		m.Put(fmt.Sprintf("key%d", i), i)
	}

	if m.Size() != 10000 {
		t.Errorf("Large map size = %d, want 10000", m.Size())
	}

	// Verify random keys
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i*100)
		if !m.ContainsKeyString(key) {
			t.Errorf("Large map should contain '%s'", key)
		}
	}
}

func TestCharArrayMap_ContainsAll(t *testing.T) {
	m := NewCharArrayMap[int](10, false)
	m.Put("foo", 1)
	m.Put("bar", 2)
	m.Put("baz", 3)

	// Test contains with different input types
	if !m.ContainsKeyString("foo") {
		t.Error("Should contain 'foo'")
	}
	if !m.ContainsKey([]rune("bar"), 0, 3) {
		t.Error("Should contain 'bar' via rune slice")
	}
	if !m.ContainsKeyInterface("baz") {
		t.Error("Should contain 'baz' via interface")
	}
}

func TestCharArrayMap_Overwrite(t *testing.T) {
	m := NewCharArrayMap[int](10, false)

	// Put first value
	old := m.Put("key", 1)
	if old != 0 {
		t.Errorf("First put returned %d, want 0 (zero value)", old)
	}

	// Overwrite
	old = m.Put("key", 2)
	if old != 1 {
		t.Errorf("Overwrite returned %d, want 1", old)
	}

	if m.Size() != 1 {
		t.Errorf("Size() = %d, want 1 (key was overwritten)", m.Size())
	}

	if v := m.GetString("key"); v != 2 {
		t.Errorf("GetString('key') = %d, want 2", v)
	}
}

func TestCharArrayMap_NilKey(t *testing.T) {
	m := NewCharArrayMap[int](10, true)

	// Test panic on nil
	t.Run("ContainsKey panics on nil", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("ContainsKey(nil) should panic")
			}
		}()
		m.ContainsKey(nil, 0, 0)
	})
}

// Benchmark tests
func BenchmarkCharArrayMap_Put(b *testing.B) {
	m := NewCharArrayMap[int](b.N, true)
	for i := 0; i < b.N; i++ {
		m.Put(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkCharArrayMap_Get(b *testing.B) {
	m := NewCharArrayMap[int](1000, true)
	for i := 0; i < 1000; i++ {
		m.Put(fmt.Sprintf("key%d", i), i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.GetString(fmt.Sprintf("key%d", i%1000))
	}
}

func BenchmarkCharArrayMap_GetIgnoreCase(b *testing.B) {
	m := NewCharArrayMap[int](1000, true)
	for i := 0; i < 1000; i++ {
		m.Put(fmt.Sprintf("key%d", i), i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.GetString(strings.ToUpper(fmt.Sprintf("key%d", i%1000)))
	}
}

func BenchmarkCharArrayMap_GetRunes(b *testing.B) {
	m := NewCharArrayMap[int](1000, true)
	keys := make([][]rune, 1000)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		m.Put(key, i)
		keys[i] = []rune(key)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Get(keys[i%1000], 0, len(keys[i%1000]))
	}
}