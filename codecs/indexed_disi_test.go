// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: indexed_disi_test.go
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestIndexedDISI.java
// Purpose: Tests for IndexedDISI - Disk-based DocIdSetIterator with skip lists
//
// IndexedDISI is a disk-based implementation of DocIdSetIterator that can return
// the index of the current document. It uses three encoding methods depending
// on block density:
//   - ALL: Block contains exactly 65536 documents
//   - DENSE: Block contains 4096 or more documents (stored as bitset)
//   - SPARSE: Block contains fewer than 4096 documents (stored as shorts)

package codecs_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockSize is the number of docIDs that a single block represents (65536)
const BlockSize = 65536

// MaxArrayLength is the maximum number of entries for SPARSE encoding (4095)
const MaxArrayLength = (1 << 12) - 1

// DefaultDenseRankPower is the default rank power (9 = every 512 docIDs)
const DefaultDenseRankPower = 9

// Method represents the encoding method for a block
type Method int

const (
	MethodSparse Method = iota
	MethodDense
	MethodAll
)

// TestIndexedDISI_Empty tests empty bitsets
func TestIndexedDISI_Empty(t *testing.T) {
	maxDoc := disiNextInt(1, 100000)
	set := util.NewSparseFixedBitSet(maxDoc)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTest(t, set, dir)
}

// TestIndexedDISI_EmptyBlocks tests EMPTY blocks with regard to jumps
// EMPTY blocks are special as they have size 0
func TestIndexedDISI_EmptyBlocks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	const B = 65536
	maxDoc := B * 11
	set := util.NewSparseFixedBitSet(maxDoc)

	// block 0: EMPTY
	set.Set(B + 5) // block 1: SPARSE
	// block 2: EMPTY
	// block 3: EMPTY
	set.Set(B*4 + 5) // block 4: SPARSE

	for i := 0; i < B; i++ {
		set.Set(B*6 + i) // block 6: ALL
	}
	for i := 0; i < B; i += 3 {
		set.Set(B*7 + i) // block 7: DENSE
	}
	for i := 0; i < B; i++ {
		if i != 32768 {
			set.Set(B*8 + i) // block 8: DENSE (all-1)
		}
	}
	// block 9-11: EMPTY

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTestAllSingleJump(t, set, dir)

	// Change the first block to DENSE to see if jump-tables sets to position 0
	set.Set(0)
	doTestAllSingleJump(t, set, dir)
}

// TestIndexedDISI_LastEmptyBlocks tests last empty blocks
func TestIndexedDISI_LastEmptyBlocks(t *testing.T) {
	const B = 65536
	maxDoc := B * 3
	set := util.NewSparseFixedBitSet(maxDoc)
	for docID := 0; docID < B*2; docID++ { // first 2 blocks are ALL
		set.Set(docID)
	}
	// Last block is EMPTY

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTestAllSingleJump(t, set, dir)
	assertAdvanceBeyondEnd(t, set, dir)
}

// assertAdvanceBeyondEnd checks that advance after the end of blocks has correct behavior
func assertAdvanceBeyondEnd(t *testing.T, set util.BitSet, dir store.Directory) {
	cardinality := set.Cardinality()
	denseRankPower := int8(DefaultDenseRankPower)

	var jumpTableEntryCount int16
	out, err := dir.CreateOutput("bar", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	jumpTableEntryCount, err = codecs.WriteBitSet(util.NewBitSetIterator(set, cardinality), out, denseRankPower)
	if err != nil {
		t.Fatal(err)
	}
	out.Close()

	in, err := dir.OpenInput("bar", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	disi2 := util.NewBitSetIterator(set, cardinality)
	doc := disi2.DocID()
	index := 0
	for doc < cardinality {
		doc, _ = disi2.NextDoc()
		index++
	}

	disi, err := codecs.NewIndexedDISI(in, 0, in.Length(), int(jumpTableEntryCount), denseRankPower, int64(cardinality))
	if err != nil {
		t.Fatal(err)
	}

	// Advance 1 docID beyond end
	exists, err := disi.AdvanceExact(set.Length())
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("There should be no set bit beyond the valid docID range")
	}

	disi.Advance(doc) // Should be the special docID signifying NO_MORE_DOCS
	// disi.index()+1 as the while-loop also counts the NO_MORE_DOCS
	if disi.Index() != index-1 {
		t.Errorf("The index when advancing beyond the last defined docID should be correct: expected %d, got %d", index, disi.Index()+1)
	}
}

// TestIndexedDISI_RandomBlocks tests random block configurations
func TestIndexedDISI_RandomBlocks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	const blocks = 5
	set := createSetWithRandomBlocks(blocks)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTestAllSingleJump(t, set, dir)
}

// TestIndexedDISI_PositionNotZero tests IndexedDISI created from slices where offset is not 0
// This is used in merges in Lucene80NormsProducer
func TestIndexedDISI_PositionNotZero(t *testing.T) {
	const blocks = 10
	denseRankPower := int8(-1)
	if rand.Intn(10) < 9 { // rarely() equivalent - 90% chance
		denseRankPower = int8(rand.Intn(7) + 7) // sane + chance of disable
	}

	set := createSetWithRandomBlocks(blocks)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cardinality := set.Cardinality()
	var jumpTableEntryCount int16
	out, err := dir.CreateOutput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	jumpTableEntryCount, err = codecs.WriteBitSet(util.NewBitSetIterator(set, cardinality), out, denseRankPower)
	if err != nil {
		t.Fatal(err)
	}
	out.Close()

	fullInput, err := dir.OpenInput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer fullInput.Close()

	blockData, err := codecs.CreateBlockSlice(fullInput, "blocks", 0, fullInput.Length(), int(jumpTableEntryCount))
	if err != nil {
		t.Fatal(err)
	}
	// Seek to random position
	blockData.SetPosition(int64(rand.Intn(int(blockData.Length()))))

	jumpTable, err := codecs.CreateJumpTable(fullInput, 0, fullInput.Length(), int(jumpTableEntryCount))
	if err != nil {
		t.Fatal(err)
	}

	disi, err := codecs.NewIndexedDISIWithSlices(blockData, jumpTable, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
	if err != nil {
		t.Fatal(err)
	}

	// This failed at some point during LUCENE-8585 development as it did not reset the slice position
	disi.AdvanceExact(blocks*65536 - 1)
}

// createSetWithRandomBlocks creates a bitset with random block types
func createSetWithRandomBlocks(blockCount int) util.BitSet {
	const B = 65536
	set := util.NewSparseFixedBitSet(blockCount * B)
	for block := 0; block < blockCount; block++ {
		switch rand.Intn(4) {
		case 0: // EMPTY
			// Do nothing
		case 1: // ALL
			for docID := block * B; docID < (block+1)*B; docID++ {
				set.Set(docID)
			}
		case 2: // SPARSE (< 4096)
			for docID := block * B; docID < (block+1)*B; docID += 101 {
				set.Set(docID)
			}
		case 3: // DENSE (>= 4096)
			for docID := block * B; docID < (block+1)*B; docID += 3 {
				set.Set(docID)
			}
		}
	}
	return set
}

// doTestAllSingleJump tests all single jumps
func doTestAllSingleJump(t *testing.T, set util.BitSet, dir store.Directory) {
	cardinality := set.Cardinality()
	denseRankPower := int8(-1)
	if rand.Intn(10) < 9 { // rarely() equivalent
		denseRankPower = int8(rand.Intn(7) + 7)
	}

	var length int64
	var jumpTableEntryCount int16
	out, err := dir.CreateOutput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	jumpTableEntryCount, err = codecs.WriteBitSet(util.NewBitSetIterator(set, cardinality), out, denseRankPower)
	if err != nil {
		t.Fatal(err)
	}
	length = out.GetFilePointer()
	out.Close()

	for i := 0; i < set.Length(); i++ {
		in, err := dir.OpenInput("foo", store.IOContextDefault)
		if err != nil {
			t.Fatal(err)
		}

		disi, err := codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
		if err != nil {
			in.Close()
			t.Fatal(err)
		}

		exists, err := disi.AdvanceExact(i)
		if err != nil {
			in.Close()
			t.Fatal(err)
		}
		if exists != set.Get(i) {
			in.Close()
			t.Errorf("The bit at %d should be correct with advanceExact: expected %v, got %v", i, set.Get(i), exists)
		}
		in.Close()

		in, err = dir.OpenInput("foo", store.IOContextDefault)
		if err != nil {
			t.Fatal(err)
		}

		disi2, err := codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
		if err != nil {
			in.Close()
			t.Fatal(err)
		}

		disi2.Advance(i)
		// Proper sanity check with jump tables as an error could make them seek backwards
		if disi2.DocID() < i {
			in.Close()
			t.Errorf("The docID should at least be %d after advance(%d) but was %d", i, i, disi2.DocID())
		}
		if set.Get(i) {
			if disi2.DocID() != i {
				in.Close()
				t.Errorf("The docID should be present with advance: expected %d, got %d", i, disi2.DocID())
			}
		} else {
			if disi2.DocID() == i {
				in.Close()
				t.Error("The docID should not be present with advance")
			}
		}
		in.Close()
	}
}

// TestIndexedDISI_OneDoc tests single document
func TestIndexedDISI_OneDoc(t *testing.T) {
	maxDoc := disiNextInt(1, 100000)
	set := util.NewSparseFixedBitSet(maxDoc)
	set.Set(rand.Intn(maxDoc))

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTest(t, set, dir)
}

// TestIndexedDISI_TwoDocs tests two documents
func TestIndexedDISI_TwoDocs(t *testing.T) {
	maxDoc := disiNextInt(1, 100000)
	set := util.NewSparseFixedBitSet(maxDoc)
	set.Set(rand.Intn(maxDoc))
	set.Set(rand.Intn(maxDoc))

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTest(t, set, dir)
}

// TestIndexedDISI_AllDocs tests all documents set
func TestIndexedDISI_AllDocs(t *testing.T) {
	maxDoc := disiNextInt(1, 100000)
	set := util.NewFixedBitSet(maxDoc)
	for i := 1; i < maxDoc; i++ {
		set.Set(i)
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTest(t, set, dir)
}

// TestIndexedDISI_HalfFull tests half-full bitset
func TestIndexedDISI_HalfFull(t *testing.T) {
	maxDoc := disiNextInt(1, 100000)
	set := util.NewSparseFixedBitSet(maxDoc)
	start := rand.Intn(2)
	for i := start; i < maxDoc; i += disiNextInt(1, 3) {
		set.Set(i)
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTest(t, set, dir)
}

// TestIndexedDISI_DocRange tests document ranges
func TestIndexedDISI_DocRange(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	for iter := 0; iter < 10; iter++ {
		maxDoc := disiNextInt(1, 1000000)
		set := util.NewFixedBitSet(maxDoc)
		start := rand.Intn(maxDoc)
		end := disiNextInt(start+1, maxDoc)
		for i := start; i < end; i++ {
			set.Set(i)
		}
		doTest(t, set, dir)
	}
}

// TestIndexedDISI_SparseDenseBoundary tests the boundary between SPARSE and DENSE encoding
func TestIndexedDISI_SparseDenseBoundary(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	set := util.NewFixedBitSet(200000)
	start := 65536 + rand.Intn(100)
	denseRankPower := int8(-1)
	if rand.Intn(10) < 9 { // rarely() equivalent
		denseRankPower = int8(rand.Intn(7) + 7)
	}

	// We set MAX_ARRAY_LENGTH bits so the encoding will be sparse
	for i := start; i < start+MaxArrayLength; i++ {
		set.Set(i)
	}

	var length int64
	var jumpTableEntryCount int16
	out, err := dir.CreateOutput("sparse", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	jumpTableEntryCount, err = codecs.WriteBitSet(util.NewBitSetIterator(set, MaxArrayLength), out, denseRankPower)
	if err != nil {
		t.Fatal(err)
	}
	length = out.GetFilePointer()
	out.Close()

	in, err := dir.OpenInput("sparse", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}

	disi, err := codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(MaxArrayLength))
	if err != nil {
		in.Close()
		t.Fatal(err)
	}

	doc, err := disi.NextDoc()
	if err != nil {
		in.Close()
		t.Fatal(err)
	}
	if doc != start {
		in.Close()
		t.Errorf("Expected start doc %d, got %d", start, doc)
	}
	if disi.Method() != codecs.MethodSparse {
		in.Close()
		t.Errorf("Expected SPARSE method, got %v", disi.Method())
	}
	in.Close()

	doTest(t, set, dir)

	// Now we set one more bit so the encoding will be dense
	set.Set(start + MaxArrayLength + rand.Intn(100))

	out, err = dir.CreateOutput("bar", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	_, err = codecs.WriteBitSet(util.NewBitSetIterator(set, MaxArrayLength+1), out, denseRankPower)
	if err != nil {
		t.Fatal(err)
	}
	length = out.GetFilePointer()
	out.Close()

	in, err = dir.OpenInput("bar", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}

	disi, err = codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(MaxArrayLength+1))
	if err != nil {
		in.Close()
		t.Fatal(err)
	}

	doc, err = disi.NextDoc()
	if err != nil {
		in.Close()
		t.Fatal(err)
	}
	if doc != start {
		in.Close()
		t.Errorf("Expected start doc %d, got %d", start, doc)
	}
	if disi.Method() != codecs.MethodDense {
		in.Close()
		t.Errorf("Expected DENSE method, got %v", disi.Method())
	}
	in.Close()

	doTest(t, set, dir)
}

// TestIndexedDISI_OneDocMissing tests one missing document
func TestIndexedDISI_OneDocMissing(t *testing.T) {
	maxDoc := disiNextInt(1, 1000000)
	set := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		set.Set(i)
	}
	set.Clear(rand.Intn(maxDoc))

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doTest(t, set, dir)
}

// TestIndexedDISI_FewMissingDocs tests a few missing documents
func TestIndexedDISI_FewMissingDocs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	numIters := disiAtLeast(10)
	for iter := 0; iter < numIters; iter++ {
		maxDoc := disiNextInt(1, 100000)
		set := util.NewFixedBitSet(maxDoc)
		for i := 0; i < maxDoc; i++ {
			set.Set(i)
		}
		numMissingDocs := disiNextInt(2, 1000)
		for i := 0; i < numMissingDocs; i++ {
			set.Clear(rand.Intn(maxDoc))
		}
		doTest(t, set, dir)
	}
}

// TestIndexedDISI_DenseMultiBlock tests dense encoding across multiple blocks
func TestIndexedDISI_DenseMultiBlock(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	maxDoc := 10 * 65536 // 10 blocks
	set := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i += 2 { // Set every other to ensure dense
		set.Set(i)
	}

	doTest(t, set, dir)
}

// TestIndexedDISI_DenseBitSizeLessThanBlockSize tests dense bitsets smaller than block size
func TestIndexedDISI_DenseBitSizeLessThanBlockSize(t *testing.T) {
	denseRankPower := int8(rand.Intn(7) + 7)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Initialize a maxDoc that is less than BlockSize
	maxDoc := rand.Intn(4096*2) + 4096*2 // Random between 8192 and 12288
	if maxDoc > 65536 {
		maxDoc = 65536
	}

	set := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i += 2 { // Set every other to ensure dense
		set.Set(i)
	}

	var jumpTableEntryCount int16
	var length int64
	out, err := dir.CreateOutput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	jumpTableEntryCount, err = codecs.WriteBitSet(util.NewBitSetIterator(set, set.Cardinality()), out, denseRankPower)
	if err != nil {
		t.Fatal(err)
	}
	length = out.GetFilePointer()
	out.Close()

	// jumpTableEntryCount should be 0 for dense bitsets with size < BLOCK_SIZE
	if jumpTableEntryCount != 0 {
		t.Error("jumpTableEntryCount should be 0 for dense bitsets with size < BLOCK_SIZE")
	}

	in, err := dir.OpenInput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	disi, err := codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(set.Cardinality()))
	if err != nil {
		t.Fatal(err)
	}

	disiSet := util.NewFixedBitSet(maxDoc)
	// This would throw IOOB if bitset size is not handled correctly as per #14882
	err = disiSet.Or(disi)
	if err != nil {
		t.Fatal(err)
	}
}

// TestIndexedDISI_IllegalDenseRankPower tests illegal denseRankPower values
func TestIndexedDISI_IllegalDenseRankPower(t *testing.T) {
	// Legal values
	legalValues := []int8{-1, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	for _, denseRankPower := range legalValues {
		err := createAndOpenDISI(denseRankPower, denseRankPower)
		if err != nil {
			t.Errorf("Expected no error for legal denseRankPower %d, got %v", denseRankPower, err)
		}
	}

	// Illegal values
	illegalValues := []int8{-2, 0, 1, 6, 16}
	for _, denseRankPower := range illegalValues {
		// Illegal write, legal read (should not reach read)
		err := createAndOpenDISI(denseRankPower, 8)
		if err == nil {
			t.Errorf("Expected error for illegal denseRankPower %d on write", denseRankPower)
		}

		// Legal write, illegal read (should reach read)
		err = createAndOpenDISI(8, denseRankPower)
		if err == nil {
			t.Errorf("Expected error for illegal denseRankPower %d on read", denseRankPower)
		}
	}
}

// createAndOpenDISI creates and opens an IndexedDISI with the given denseRankPower values
func createAndOpenDISI(denseRankPowerWrite, denseRankPowerRead int8) error {
	set := util.NewFixedBitSet(10)
	set.Set(set.Length() - 1)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	var length int64
	var jumpTableEntryCount int16 = -1
	out, err := dir.CreateOutput("foo", store.IOContextDefault)
	if err != nil {
		return err
	}
	jumpTableEntryCount, err = codecs.WriteBitSet(util.NewBitSetIterator(set, set.Cardinality()), out, denseRankPowerWrite)
	if err != nil {
		out.Close()
		return err
	}
	length = out.GetFilePointer()
	out.Close()

	in, err := dir.OpenInput("foo", store.IOContextDefault)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPowerRead, int64(set.Cardinality()))
	return err
}

// TestIndexedDISI_OneDocMissingFixed tests one missing document with fixed values
func TestIndexedDISI_OneDocMissingFixed(t *testing.T) {
	maxDoc := 9699
	denseRankPower := int8(-1)
	if rand.Intn(10) < 9 { // rarely() equivalent
		denseRankPower = int8(rand.Intn(7) + 7)
	}

	set := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		set.Set(i)
	}
	set.Clear(1345)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cardinality := set.Cardinality()
	var length int64
	var jumpTableEntryCount int16
	out, err := dir.CreateOutput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	jumpTableEntryCount, err = codecs.WriteBitSet(util.NewBitSetIterator(set, cardinality), out, denseRankPower)
	if err != nil {
		t.Fatal(err)
	}
	length = out.GetFilePointer()
	out.Close()

	step := 16000
	in, err := dir.OpenInput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	disi, err := codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
	if err != nil {
		t.Fatal(err)
	}

	disi2 := util.NewBitSetIterator(set, cardinality)
	assertAdvanceEquality(t, disi, disi2, step)
}

// TestIndexedDISI_Random tests random document patterns
func TestIndexedDISI_Random(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping nightly test in short mode")
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	numIters := disiAtLeast(3)
	for i := 0; i < numIters; i++ {
		doTestRandom(t, dir)
	}
}

// doTestRandom performs a random test
func doTestRandom(t *testing.T, dir store.Directory) {
	maxStep := disiNextInt(1, 1<<disiNextInt(2, 20))
	numDocs := disiNextInt(1, disiMin(100000, (2147483647-1)/maxStep))

	docs := util.NewSparseFixedBitSet(numDocs*maxStep + 1)
	lastDoc := -1
	doc := -1
	for i := 0; i < numDocs; i++ {
		doc += disiNextInt(1, maxStep)
		docs.Set(doc)
		lastDoc = doc
	}
	maxDoc := lastDoc + disiNextInt(1, 100)

	set := util.BitSetOf(util.NewBitSetIterator(docs, docs.ApproximateCardinality()), maxDoc)
	doTest(t, set, dir)
}

// doTest is the main test function that tests IndexedDISI against a BitSet
func doTest(t *testing.T, set util.BitSet, dir store.Directory) {
	cardinality := set.Cardinality()
	denseRankPower := int8(-1)
	if rand.Intn(10) < 9 { // rarely() equivalent - 90% chance
		denseRankPower = int8(rand.Intn(7) + 7)
	}

	var length int64
	var jumpTableEntryCount int16
	out, err := dir.CreateOutput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	jumpTableEntryCount, err = codecs.WriteBitSet(util.NewBitSetIterator(set, cardinality), out, denseRankPower)
	if err != nil {
		t.Fatal(err)
	}
	length = out.GetFilePointer()
	out.Close()

	// Test single step equality
	in, err := dir.OpenInput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}

	disi, err := codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
	if err != nil {
		in.Close()
		t.Fatal(err)
	}

	disi2 := util.NewBitSetIterator(set, cardinality)
	assertSingleStepEquality(t, disi, disi2)
	in.Close()

	// Test advance equality with various steps
	steps := []int{1, 10, 100, 1000, 10000, 100000}
	for _, step := range steps {
		in, err := dir.OpenInput("foo", store.IOContextDefault)
		if err != nil {
			t.Fatal(err)
		}

		disi, err = codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
		if err != nil {
			in.Close()
			t.Fatal(err)
		}

		disi2 = util.NewBitSetIterator(set, cardinality)
		assertAdvanceEquality(t, disi, disi2, step)
		in.Close()
	}

	// Test advance exact randomized
	steps = []int{10, 100, 1000, 10000, 100000}
	for _, step := range steps {
		in, err := dir.OpenInput("foo", store.IOContextDefault)
		if err != nil {
			t.Fatal(err)
		}

		disi, err = codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
		if err != nil {
			in.Close()
			t.Fatal(err)
		}

		disi2 = util.NewBitSetIterator(set, cardinality)
		disi2length := set.Length()
		assertAdvanceExactRandomized(t, disi, disi2, disi2length, step)
		in.Close()
	}

	// Test intoBitSet randomized
	steps = []int{100, 1000, 10000, 100000}
	for _, step := range steps {
		in, err := dir.OpenInput("foo", store.IOContextDefault)
		if err != nil {
			t.Fatal(err)
		}

		disi, err = codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
		if err != nil {
			in.Close()
			t.Fatal(err)
		}

		disi2 = util.NewBitSetIterator(set, cardinality)
		disi2length := set.Length()
		assertIntoBitsetRandomized(t, disi, disi2, disi2length, step)
		in.Close()
	}

	// Test docIDRunEnd randomized
	steps = []int{100, 1000, 10000, 100000}
	for _, step := range steps {
		in, err := dir.OpenInput("foo", store.IOContextDefault)
		if err != nil {
			t.Fatal(err)
		}

		disi, err = codecs.NewIndexedDISI(in, 0, length, int(jumpTableEntryCount), denseRankPower, int64(cardinality))
		if err != nil {
			in.Close()
			t.Fatal(err)
		}

		disi2 = util.NewBitSetIterator(set, cardinality)
		disi2length := set.Length()
		assertDocIDRunEndRandomized(t, disi, disi2, disi2length, step)
		in.Close()
	}

	dir.DeleteFile("foo")
}

// assertAdvanceExactRandomized tests advanceExact with randomized targets
func assertAdvanceExactRandomized(t *testing.T, disi codecs.IndexedDISI, disi2 *util.BitSetIterator, disi2length, step int) {
	index := -1
	for target := 0; target < disi2length; {
		target += rand.Intn(step + 1)
		doc := disi2.DocID()
		for doc < target {
			d, _ := disi2.NextDoc()
			doc = d
			index++
		}

		exists, err := disi.AdvanceExact(target)
		if err != nil {
			t.Fatal(err)
		}
		if exists != (doc == target) {
			t.Errorf("advanceExact mismatch at target %d: expected %v, got %v", target, doc == target, exists)
		}
		if exists {
			if disi.Index() != index {
				t.Errorf("Index mismatch: expected %d, got %d", index, disi.Index())
			}
		} else if rand.Intn(2) == 0 {
			d, _ := disi.NextDoc()
			if d != doc {
				t.Errorf("NextDoc mismatch after advanceExact: expected %d, got %d", doc, d)
			}
			// This is a bit strange when doc == NO_MORE_DOCS as the index overcounts in the disi2 while-loop
			if disi.Index() != index {
				t.Errorf("Index mismatch after nextDoc: expected %d, got %d", index, disi.Index())
			}
			target = doc
		}
	}
}

// assertIntoBitsetRandomized tests intoBitSet with randomized ranges
func assertIntoBitsetRandomized(t *testing.T, disi codecs.IndexedDISI, disi2 *util.BitSetIterator, disi2length, step int) {
	index := -1
	set1 := util.NewFixedBitSet(step)
	set2 := util.NewFixedBitSet(step)

	for upTo := 0; upTo < disi2length; {
		lastUpTo := upTo
		upTo += rand.Intn(step + 1)
		offset := lastUpTo + rand.Intn(upTo-lastUpTo+1)

		if disi.DocID() < offset {
			disi.Advance(offset)
		}
		doc := disi2.DocID()
		for doc < offset {
			index++
			d, _ := disi2.NextDoc()
			doc = d
		}
		for doc < upTo {
			set2.Set(doc - offset)
			index++
			d, _ := disi2.NextDoc()
			doc = d
		}

		disi.IntoBitSet(upTo, set1, offset)
		if disi.Index() != index {
			t.Errorf("Index mismatch after intoBitSet: expected %d, got %d", index, disi.Index())
		}
		if disi2.DocID() != disi.DocID() {
			t.Errorf("DocID mismatch after intoBitSet: expected %d, got %d", disi2.DocID(), disi.DocID())
		}

		expected := util.NewBitSetIterator(set2, set2.Cardinality())
		actual := util.NewBitSetIterator(set1, set1.Cardinality())
		for expectedDoc, _ := expected.NextDoc(); expectedDoc != search.NO_MORE_DOCS; expectedDoc, _ = expected.NextDoc() {
			actualDoc, _ := actual.NextDoc()
			if expectedDoc+offset != actualDoc+offset {
				t.Errorf("BitSet content mismatch: expected %d, got %d", expectedDoc+offset, actualDoc+offset)
			}
		}
		actualDoc, _ := actual.NextDoc()
		if actualDoc != search.NO_MORE_DOCS {
			t.Error("Expected NO_MORE_DOCS from actual iterator")
		}

		if disi2.DocID() != search.NO_MORE_DOCS {
			d1, _ := disi2.NextDoc()
			d2, _ := disi.NextDoc()
			if d1 != d2 {
				t.Errorf("NextDoc mismatch after intoBitSet: expected %d, got %d", d1, d2)
			}
			if disi.Index() != index+1 {
				t.Errorf("Index mismatch after nextDoc: expected %d, got %d", index+1, disi.Index())
			}
			index++
		}

		set1.Clear()
		set2.Clear()
	}
}

// assertDocIDRunEndRandomized tests docIDRunEnd with randomized targets
func assertDocIDRunEndRandomized(t *testing.T, disi codecs.IndexedDISI, disi2 *util.BitSetIterator, disi2length, step int) {
	for target := 0; target < disi2length; {
		target += rand.Intn(step + 1)
		if disi.DocID() < target {
			disi.Advance(target)
			disi2.Advance(target)
			if disi2.DocID() != disi.DocID() {
				t.Errorf("DocID mismatch after advance: expected %d, got %d", disi2.DocID(), disi.DocID())
			}
			end := disi.DocIDRunEnd()
			if end == 0 {
				t.Error("docIDRunEnd should not return 0")
			}
			for it := disi.DocID(); it != search.NO_MORE_DOCS && it+1 < end; it++ {
				d1, _ := disi.NextDoc()
				d2, _ := disi2.NextDoc()
				if d1 != it+1 {
					t.Errorf("NextDoc from disi: expected %d, got %d", it+1, d1)
				}
				if d2 != it+1 {
					t.Errorf("NextDoc from disi2: expected %d, got %d", it+1, d2)
				}
			}
		}
	}
}

// assertSingleStepEquality tests that IndexedDISI matches BitSetIterator step by step
func assertSingleStepEquality(t *testing.T, disi codecs.IndexedDISI, disi2 *util.BitSetIterator) {
	i := 0
	for doc, _ := disi2.NextDoc(); doc != search.NO_MORE_DOCS; doc, _ = disi2.NextDoc() {
		d, err := disi.NextDoc()
		if err != nil {
			t.Fatal(err)
		}
		if doc != d {
			t.Errorf("Step %d: expected doc %d, got %d", i, doc, d)
		}
		if disi.Index() != i {
			t.Errorf("Step %d: expected index %d, got %d", i, i, disi.Index())
		}
		i++
	}
	d, err := disi.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if d != search.NO_MORE_DOCS {
		t.Errorf("Expected NO_MORE_DOCS, got %d", d)
	}
}

// assertAdvanceEquality tests advance with a specific step size
func assertAdvanceEquality(t *testing.T, disi codecs.IndexedDISI, disi2 *util.BitSetIterator, step int) {
	index := -1
	for {
		target := disi2.DocID() + step
		doc := -1
		for doc < target {
			d, _ := disi2.NextDoc()
			doc = d
			index++
		}
		d, err := disi.Advance(target)
		if err != nil {
			t.Fatal(err)
		}
		if doc != d {
			t.Errorf("Expected equality using step %d at docID %d: expected %d, got %d", step, doc, doc, d)
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		if disi.Index() != index {
			t.Errorf("Index mismatch using step %d at docID %d: expected %d, got %d", step, doc, index, disi.Index())
		}
	}
}

// Helper functions for IndexedDISI tests

// disiNextInt returns a random integer between min and max (inclusive)
func disiNextInt(min, max int) int {
	if min >= max {
		return min
	}
	return min + rand.Intn(max-min)
}

// disiAtLeast returns at least the specified number, scaled for testing
func disiAtLeast(n int) int {
	// In Lucene tests, this scales with the test multiplier
	// For now, just return n
	return n
}

// disiMin returns the minimum of two integers
func disiMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
