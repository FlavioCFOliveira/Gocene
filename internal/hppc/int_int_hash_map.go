package hppc

import (
	"fmt"
)

// IntIntHashMap is a hash map of int to int, implemented using open addressing with linear
// probing for collision resolution.
type IntIntHashMap struct {
	keys       []int
	values     []int
	assigned   int
	mask       int
	resizeAt   int
	hasEmptyKey bool
	loadFactor float64
	iterationSeed int32
}

func NewIntIntHashMap() *IntIntHashMap {
	return NewIntIntHashMapWithExpectedElements(DefaultExpectedElements)
}

func NewIntIntHashMapWithExpectedElements(expectedElements int) *IntIntHashMap {
	return NewIntIntHashMapWithDefaults(expectedElements, DefaultLoadFactor)
}

func NewIntIntHashMapWithDefaults(expectedElements int, loadFactor float64) *IntIntHashMap {
	m := &IntIntHashMap{
		loadFactor: loadFactor,
	}
	m.ensureCapacity(expectedElements)
	return m
}

func (m *IntIntHashMap) Put(key, value int) int {
	if key == 0 {
		previousValue := 0
		if m.hasEmptyKey {
			previousValue = m.values[m.mask+1]
		}
		m.hasEmptyKey = true
		m.values[m.mask+1] = value
		return previousValue
	}

	slot := int(MixPhi(int32(key))) & m.mask

	for {
		existing := m.keys[slot]
		if existing == 0 {
			break
		}
		if existing == key {
			previousValue := m.values[slot]
			m.values[slot] = value
			return previousValue
		}
		slot = (slot + 1) & m.mask
	}

	if m.assigned == m.resizeAt {
		m.allocateThenInsertThenRehash(slot, key, value)
	} else {
		m.keys[slot] = key
		m.values[slot] = value
	}

	m.assigned++
	return 0
}

func (m *IntIntHashMap) PutAll(iterable iterableIntIntCursor) int {
	count := m.Size()
	for iterable.Next() {
		m.Put(iterable.Cursor().Key, iterable.Cursor().Value)
	}
	return m.Size() - count
}

type iterableIntIntCursor interface {
	Next() bool
	Cursor() *IntIntCursor
}

func (m *IntIntHashMap) PutIfAbsent(key, value int) bool {
	keyIndex := m.IndexOf(key)
	if !m.indexExists(keyIndex) {
		m.indexInsert(keyIndex, key, value)
		return true
	}
	return false
}

func (m *IntIntHashMap) PutOrAdd(key, putValue, incrementValue int) int {
	keyIndex := m.IndexOf(key)
	if m.indexExists(keyIndex) {
		putValue = m.values[keyIndex] + incrementValue
		m.IndexReplace(keyIndex, putValue)
	} else {
		m.indexInsert(keyIndex, key, putValue)
	}
	return putValue
}

func (m *IntIntHashMap) AddTo(key, incrementValue int) int {
	return m.PutOrAdd(key, incrementValue, incrementValue)
}

func (m *IntIntHashMap) Remove(key int) int {
	if key == 0 {
		if !m.hasEmptyKey {
			return 0
		}
		m.hasEmptyKey = false
		previousValue := m.values[m.mask+1]
		m.values[m.mask+1] = 0
		return previousValue
	}

	slot := int(MixPhi(int32(key))) & m.mask

	for {
		existing := m.keys[slot]
		if existing == 0 {
			break
		}
		if existing == key {
			previousValue := m.values[slot]
			m.shiftConflictingKeys(slot)
			return previousValue
		}
		slot = (slot + 1) & m.mask
	}

	return 0
}

func (m *IntIntHashMap) Get(key int) int {
	if key == 0 {
		if m.hasEmptyKey {
			return m.values[m.mask+1]
		}
		return 0
	}

	slot := int(MixPhi(int32(key))) & m.mask

	for {
		existing := m.keys[slot]
		if existing == 0 {
			break
		}
		if existing == key {
			return m.values[slot]
		}
		slot = (slot + 1) & m.mask
	}

	return 0
}

func (m *IntIntHashMap) GetOrDefault(key, defaultValue int) int {
	if key == 0 {
		if m.hasEmptyKey {
			return m.values[m.mask+1]
		}
		return defaultValue
	}

	slot := int(MixPhi(int32(key))) & m.mask

	for {
		existing := m.keys[slot]
		if existing == 0 {
			break
		}
		if existing == key {
			return m.values[slot]
		}
		slot = (slot + 1) & m.mask
	}

	return defaultValue
}

func (m *IntIntHashMap) ContainsKey(key int) bool {
	if key == 0 {
		return m.hasEmptyKey
	}

	slot := int(MixPhi(int32(key))) & m.mask

	for {
		existing := m.keys[slot]
		if existing == 0 {
			break
		}
		if existing == key {
			return true
		}
		slot = (slot + 1) & m.mask
	}

	return false
}

func (m *IntIntHashMap) IndexOf(key int) int {
	if key == 0 {
		if m.hasEmptyKey {
			return m.mask + 1
		}
		return ^(m.mask + 1)
	}

	slot := int(MixPhi(int32(key))) & m.mask

	for {
		existing := m.keys[slot]
		if existing == 0 {
			break
		}
		if existing == key {
			return slot
		}
		slot = (slot + 1) & m.mask
	}

	return ^slot
}

func (m *IntIntHashMap) indexExists(index int) bool {
	return index >= 0
}

func (m *IntIntHashMap) IndexGet(index int) int {
	return m.values[index]
}

func (m *IntIntHashMap) IndexReplace(index, newValue int) int {
	previousValue := m.values[index]
	m.values[index] = newValue
	return previousValue
}

func (m *IntIntHashMap) indexInsert(index, key, value int) {
	if index < 0 {
		index = ^index
	}
	if key == 0 {
		m.values[index] = value
		m.hasEmptyKey = true
	} else {
		if m.assigned == m.resizeAt {
			m.allocateThenInsertThenRehash(index, key, value)
		} else {
			m.keys[index] = key
			m.values[index] = value
		}
		m.assigned++
	}
}

func (m *IntIntHashMap) IndexRemove(index int) int {
	previousValue := m.values[index]
	if index > m.mask {
		m.hasEmptyKey = false
		m.values[index] = 0
	} else {
		m.shiftConflictingKeys(index)
	}
	return previousValue
}

func (m *IntIntHashMap) Clear() {
	m.assigned = 0
	m.hasEmptyKey = false
	for i := range m.keys {
		m.keys[i] = 0
	}
}

func (m *IntIntHashMap) Release() {
	m.assigned = 0
	m.hasEmptyKey = false
	m.keys = nil
	m.values = nil
	m.ensureCapacity(DefaultExpectedElements)
}

func (m *IntIntHashMap) Size() int {
	size := m.assigned
	if m.hasEmptyKey {
		size++
	}
	return size
}

func (m *IntIntHashMap) IsEmpty() bool {
	return m.Size() == 0
}

func (m *IntIntHashMap) String() string {
	res := "["
	first := true
	for _, cursor := range m.Iter() {
		if !first {
			res += ", "
		}
		res += fmt.Sprintf("%d=>%d", cursor.Key, cursor.Value)
		first = false
	}
	res += "]"
	return res
}

func (m *IntIntHashMap) ensureCapacity(expectedElements int) {
	if m.keys == nil || expectedElements > m.resizeAt {
		prevKeys := m.keys
		prevValues := m.values
		m.allocateBuffers(MinBufferSize(expectedElements, m.loadFactor))
		if prevKeys != nil && !m.IsEmpty() {
			m.rehash(prevKeys, prevValues)
		}
	}
}

func (m *IntIntHashMap) allocateBuffers(arraySize int) {
	emptyElementSlot := 1
	m.keys = make([]int, arraySize+emptyElementSlot)
	m.values = make([]int, arraySize+emptyElementSlot)
	m.resizeAt = ExpandAtCount(arraySize, m.loadFactor)
	m.mask = arraySize - 1
}

func (m *IntIntHashMap) rehash(fromKeys, fromValues []int) {
	keys := m.keys
	values := m.values
	mask := m.mask

	from := len(fromKeys) - 1
	keys[len(keys)-1] = fromKeys[from]
	values[len(values)-1] = fromValues[from]
	for from > 0 {
		from--
		existing := fromKeys[from]
		if existing != 0 {
			slot := int(MixPhi(int32(existing))) & mask
			for keys[slot] != 0 {
				slot = (slot + 1) & mask
			}
			keys[slot] = existing
			values[slot] = fromValues[from]
		}
	}
}

func (m *IntIntHashMap) allocateThenInsertThenRehash(slot, pendingKey, pendingValue int) {
	prevKeys := m.keys
	prevValues := m.values
	m.allocateBuffers(NextBufferSize(m.mask+1, m.Size(), m.loadFactor))

	prevKeys[slot] = pendingKey
	prevValues[slot] = pendingValue

	m.rehash(prevKeys, prevValues)
}

func (m *IntIntHashMap) shiftConflictingKeys(gapSlot int) {
	keys := m.keys
	values := m.values
	mask := m.mask

	distance := 0
	for {
		slot := (gapSlot + (distance + 1)) & mask
		existing := keys[slot]
		if existing == 0 {
			break
		}

		idealSlot := int(MixPhi(int32(existing))) & mask
		shift := (slot - idealSlot) & mask
		if shift >= distance+1 {
			keys[gapSlot] = existing
			values[gapSlot] = values[slot]
			gapSlot = slot
			distance = 0
		} else {
			distance++
		}
	}

	keys[gapSlot] = 0
	values[gapSlot] = 0
	m.assigned--
}

func (m *IntIntHashMap) Iter() []IntIntCursor {
	// This is a simplified iterator for Go.
	// Lucene's iterator is more complex to support randomized iteration seeds.
	// For correctness, we can just return all entries.
	res := make([]IntIntCursor, 0, m.Size())
	for i := 0; i < len(m.keys); i++ {
		if m.keys[i] != 0 {
			res = append(res, IntIntCursor{Index: i, Key: m.keys[i], Value: m.values[i]})
		}
	}
	if m.hasEmptyKey {
		res = append(res, IntIntCursor{Index: m.mask + 1, Key: 0, Value: m.values[m.mask+1]})
	}
	return res
}

// IntIntCursor compatible with internal/hppc/int_cursor.go
type IntIntCursor struct {
	Index int
	Key   int
	Value int
}
