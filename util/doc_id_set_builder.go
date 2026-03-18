// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math"
	"sort"
)

// BulkAdder is a utility class to efficiently add many docs in one go.
// This is the Go port of Lucene's DocIdSetBuilder.BulkAdder.
type BulkAdder interface {
	// Add adds a single document ID.
	Add(doc int)
	// AddBatch adds multiple document IDs from a slice.
	AddBatch(docs []int)
	// AddIterator adds all document IDs from a DocIdSetIterator.
	AddIterator(iter DocIdSetIterator) error
}

// FixedBitSetAdder adds documents to a FixedBitSet.
type FixedBitSetAdder struct {
	bitSet *FixedBitSet
}

// Add adds a document to the bitset.
func (f *FixedBitSetAdder) Add(doc int) {
	f.bitSet.Set(doc)
}

// AddBatch adds multiple documents to the bitset.
func (f *FixedBitSetAdder) AddBatch(docs []int) {
	for _, doc := range docs {
		f.bitSet.Set(doc)
	}
}

// AddIterator adds all documents from an iterator to the bitset.
func (f *FixedBitSetAdder) AddIterator(iter DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		f.bitSet.Set(doc)
	}
	return nil
}

// Ensure FixedBitSetAdder implements BulkAdder
var _ BulkAdder = (*FixedBitSetAdder)(nil)

// Buffer holds a chunk of document IDs.
type Buffer struct {
	array  []int
	length int
}

// BufferAdder adds documents to a Buffer.
type BufferAdder struct {
	buffer *Buffer
}

// Add adds a document to the buffer.
func (b *BufferAdder) Add(doc int) {
	b.buffer.array[b.buffer.length] = doc
	b.buffer.length++
}

// AddBatch adds multiple documents to the buffer.
func (b *BufferAdder) AddBatch(docs []int) {
	copy(b.buffer.array[b.buffer.length:], docs)
	b.buffer.length += len(docs)
}

// AddIterator adds all documents from an iterator to the buffer.
func (b *BufferAdder) AddIterator(iter DocIdSetIterator) error {
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		b.Add(doc)
	}
	return nil
}

// Ensure BufferAdder implements BulkAdder
var _ BulkAdder = (*BufferAdder)(nil)

// DocIdSetBuilder builds DocIdSet instances.
// At first it uses a sparse structure to gather documents, and then
// upgrades to a non-sparse bit set once enough hits match.
// This is the Go port of Lucene's org.apache.lucene.util.DocIdSetBuilder.
type DocIdSetBuilder struct {
	maxDoc          int
	threshold       int
	Multivalued     bool    // Exported for testing
	NumValuesPerDoc float64 // Exported for testing

	buffers        []*Buffer
	totalAllocated int
	bitSet         *FixedBitSet
	counter        int64
	adder          BulkAdder
}

// NewDocIdSetBuilder creates a builder that can contain doc IDs between 0 and maxDoc.
func NewDocIdSetBuilder(maxDoc int) *DocIdSetBuilder {
	return NewDocIdSetBuilderWithStats(maxDoc, -1, -1)
}

// NewDocIdSetBuilderWithStats creates a DocIdSetBuilder with document statistics.
// docCount is the number of documents with values.
// valueCount is the total number of values.
func NewDocIdSetBuilderWithStats(maxDoc, docCount int, valueCount int64) *DocIdSetBuilder {
	builder := &DocIdSetBuilder{
		maxDoc:  maxDoc,
		bitSet:  nil,
		counter: -1,
		buffers: make([]*Buffer, 0),
	}

	builder.Multivalued = docCount < 0 || int64(docCount) != valueCount
	if docCount <= 0 || valueCount < 0 {
		// Assume one value per doc
		builder.NumValuesPerDoc = 1.0
	} else {
		builder.NumValuesPerDoc = float64(valueCount) / float64(docCount)
	}

	// For ridiculously small sets, we'll just use a sorted int[]
	// maxDoc >> 7 is a good value if you want to save memory
	builder.threshold = maxDoc >> 7

	return builder
}

// Add adds the content of the provided DocIdSetIterator to this builder.
func (b *DocIdSetBuilder) Add(iter DocIdSetIterator) error {
	cost := int(math.Min(float64(int(^uint(0)>>1)), float64(iter.Cost())))
	adder := b.Grow(cost)

	if b.bitSet != nil {
		// Add directly to bitset
		for {
			doc, err := iter.NextDoc()
			if err != nil {
				return err
			}
			if doc == NO_MORE_DOCS {
				break
			}
			b.bitSet.Set(doc)
		}
		return nil
	}

	// Add via adder
	for i := 0; i < cost; i++ {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == NO_MORE_DOCS {
			return nil
		}
		adder.Add(doc)
	}

	// Continue adding remaining docs
	for {
		doc, err := iter.NextDoc()
		if err != nil {
			return err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		b.Grow(1).Add(doc)
	}
	return nil
}

// Grow reserves space and returns a BulkAdder that can be used to add up to numDocs documents.
func (b *DocIdSetBuilder) Grow(numDocs int) BulkAdder {
	if b.bitSet == nil {
		if b.totalAllocated+numDocs <= b.threshold {
			b.ensureBufferCapacity(numDocs)
		} else {
			b.upgradeToBitSet()
			b.counter += int64(numDocs)
		}
	} else {
		b.counter += int64(numDocs)
	}
	return b.adder
}

func (b *DocIdSetBuilder) ensureBufferCapacity(numDocs int) {
	if len(b.buffers) == 0 {
		b.addBuffer(b.additionalCapacity(numDocs))
		return
	}

	current := b.buffers[len(b.buffers)-1]
	if len(current.array)-current.length >= numDocs {
		// Current buffer is large enough
		return
	}
	if current.length < len(current.array)-(len(current.array)>>3) {
		// Current buffer is less than 7/8 full, resize rather than waste space
		b.growBuffer(current, b.additionalCapacity(numDocs))
	} else {
		b.addBuffer(b.additionalCapacity(numDocs))
	}
}

func (b *DocIdSetBuilder) additionalCapacity(numDocs int) int {
	// Exponential growth: the new array has a size equal to the sum of what
	// has been allocated so far
	c := b.totalAllocated
	// But is also >= numDocs + 1 so that we can store the next batch of docs
	// (plus an empty slot so that we are more likely to reuse the array in build())
	if c < numDocs+1 {
		c = numDocs + 1
	}
	// Avoid cold starts
	if c < 32 {
		c = 32
	}
	// Do not go beyond the threshold
	if c > b.threshold-b.totalAllocated {
		c = b.threshold - b.totalAllocated
	}
	return c
}

func (b *DocIdSetBuilder) addBuffer(length int) *Buffer {
	buffer := &Buffer{
		array:  make([]int, length),
		length: 0,
	}
	b.buffers = append(b.buffers, buffer)
	b.adder = &BufferAdder{buffer: buffer}
	b.totalAllocated += length
	return buffer
}

func (b *DocIdSetBuilder) growBuffer(buffer *Buffer, additionalCapacity int) {
	newArray := make([]int, len(buffer.array)+additionalCapacity)
	copy(newArray, buffer.array)
	buffer.array = newArray
	b.totalAllocated += additionalCapacity
}

func (b *DocIdSetBuilder) upgradeToBitSet() {
	if b.bitSet != nil {
		return
	}

	bitSet, _ := NewFixedBitSet(b.maxDoc)
	var counter int64 = 0

	for _, buffer := range b.buffers {
		for i := 0; i < buffer.length; i++ {
			bitSet.Set(buffer.array[i])
		}
		counter += int64(buffer.length)
	}

	b.bitSet = bitSet
	b.counter = counter
	b.buffers = nil
	b.adder = &FixedBitSetAdder{bitSet: bitSet}
}

// Build builds a DocIdSet from the accumulated doc IDs.
func (b *DocIdSetBuilder) Build() (DocIdSet, error) {
	defer func() {
		b.buffers = nil
		b.bitSet = nil
	}()

	if b.bitSet != nil {
		cost := int64(math.Round(float64(b.counter) / b.NumValuesPerDoc))
		return NewBitDocIdSet(b.bitSet, cost)
	}

	// Concatenate buffers and sort
	concatenated := b.concat()

	// Sort the array
	if concatenated.length > 0 {
		sort.Ints(concatenated.array[:concatenated.length])
	}

	// Deduplicate if multivalued
	l := concatenated.length
	if b.Multivalued {
		l = b.dedup(concatenated.array, concatenated.length)
	}

	// Ensure there's room for NO_MORE_DOCS sentinel
	if l >= len(concatenated.array) {
		newArray := make([]int, l+1)
		copy(newArray, concatenated.array[:l])
		concatenated.array = newArray
	}
	concatenated.array[l] = NO_MORE_DOCS

	return NewIntArrayDocIdSet(concatenated.array, l)
}

func (b *DocIdSetBuilder) concat() *Buffer {
	totalLength := 0
	var largestBuffer *Buffer
	for _, buffer := range b.buffers {
		totalLength += buffer.length
		if largestBuffer == nil || len(buffer.array) > len(largestBuffer.array) {
			largestBuffer = buffer
		}
	}

	if largestBuffer == nil {
		return &Buffer{array: make([]int, 1), length: 0}
	}

	docs := largestBuffer.array
	if len(docs) < totalLength+1 {
		docs = make([]int, totalLength+1)
		copy(docs, largestBuffer.array[:largestBuffer.length])
	}

	totalLength = largestBuffer.length
	for _, buffer := range b.buffers {
		if buffer != largestBuffer {
			copy(docs[totalLength:], buffer.array[:buffer.length])
			totalLength += buffer.length
		}
	}

	return &Buffer{array: docs, length: totalLength}
}

func (b *DocIdSetBuilder) dedup(arr []int, length int) int {
	if length == 0 {
		return 0
	}
	l := 1
	previous := arr[0]
	for i := 1; i < length; i++ {
		value := arr[i]
		if value != previous {
			arr[l] = value
			l++
			previous = value
		}
	}
	return l
}
