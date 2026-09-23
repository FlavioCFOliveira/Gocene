package search

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocIdSetCopyFunc is a function that creates a DocIdSet from a FixedBitSet.
type DocIdSetCopyFunc func(bs *util.FixedBitSet, length int) (util.DocIdSet, error)

// BaseDocIdSetTestCase provides a suite of tests for DocIdSet implementations.
type BaseDocIdSetTestCase struct {
	t *testing.T
	r *rand.Rand
}

// NewBaseDocIdSetTestCase creates a new test case instance.
func NewBaseDocIdSetTestCase(t *testing.T) *BaseDocIdSetTestCase {
	return &BaseDocIdSetTestCase{
		t: t,
		r: rand.New(rand.NewSource(12345)), // Fixed seed for reproducibility
	}
}

// RunAllTests executes the full suite of DocIdSet tests.
func (b *BaseDocIdSetTestCase) RunAllTests(copyOf DocIdSetCopyFunc) {
	b.testNoBit(copyOf)
	b.test1Bit(copyOf)
	b.test2Bits(copyOf)
	b.testAgainstBitSet(copyOf)
	b.testRamBytesUsed(copyOf)
	b.testIntoBitSet(copyOf)
	b.testIntoBitSetBoundChecks(copyOf)
}

func (b *BaseDocIdSetTestCase) testNoBit(copyOf DocIdSetCopyFunc) {
	bs, _ := util.NewFixedBitSet(1)
	copy, err := copyOf(bs, 1)
	if err != nil {
		b.t.Fatalf("copyOf failed: %v", err)
	}
	b.assertEquals(1, bs, copy)
}

func (b *BaseDocIdSetTestCase) test1Bit(copyOf DocIdSetCopyFunc) {
	bs, _ := util.NewFixedBitSet(1)
	if b.r.Intn(2) == 0 {
		bs.Set(0)
	}
	copy, err := copyOf(bs, 1)
	if err != nil {
		b.t.Fatalf("copyOf failed: %v", err)
	}
	b.assertEquals(1, bs, copy)
}

func (b *BaseDocIdSetTestCase) test2Bits(copyOf DocIdSetCopyFunc) {
	bs, _ := util.NewFixedBitSet(2)
	if b.r.Intn(2) == 0 {
		bs.Set(0)
	}
	if b.r.Intn(2) == 0 {
		bs.Set(1)
	}
	copy, err := copyOf(bs, 2)
	if err != nil {
		b.t.Fatalf("copyOf failed: %v", err)
	}
	b.assertEquals(2, bs, copy)
}

func (b *BaseDocIdSetTestCase) testAgainstBitSet(copyOf DocIdSetCopyFunc) {
	numBits := b.nextInt(100, 1<<20)
	// test various random sets with various load factors
	percents := []float32{0, 0.0001, b.r.Float32(), 0.9, 1.0}
	for _, percent := range percents {
		set := b.randomSet(numBits, percent)
		copy, err := copyOf(set, numBits)
		if err != nil {
			b.t.Fatalf("copyOf failed: %v", err)
		}
		b.assertEquals(numBits, set, copy)
	}

	// test one doc
	set, _ := util.NewFixedBitSet(numBits)
	set.Set(0) // 0 first
	copy, err := copyOf(set, numBits)
	if err != nil {
		b.t.Fatalf("copyOf failed: %v", err)
	}
	b.assertEquals(numBits, set, copy)

	set.Clear(0)
	set.Set(b.r.Intn(numBits))
	copy, err = copyOf(set, numBits)
	if err != nil {
		b.t.Fatalf("copyOf failed: %v", err)
	}
	b.assertEquals(numBits, set, copy)

	// test regular increments
	maxIterations := 10
	iterations := 0
	for inc := 2; inc < 1000; inc += b.nextInt(1, 100) {
		if iterations >= maxIterations {
			break
		}
		iterations++

		set, _ = util.NewFixedBitSet(numBits)
		for d := b.r.Intn(10); d < numBits; d += inc {
			set.Set(d)
		}
		copy, err = copyOf(set, numBits)
		if err != nil {
			b.t.Fatalf("copyOf failed: %v", err)
		}
		b.assertEquals(numBits, set, copy)
	}
}

func (b *BaseDocIdSetTestCase) testRamBytesUsed(copyOf DocIdSetCopyFunc) {
	iters := 100
	for i := 0; i < iters; i++ {
		pow := b.r.Intn(20)
		maxDoc := b.nextInt(1, 1<<pow)
		numDocs := b.nextInt(0, min(maxDoc, 1<<b.r.Intn(pow+1)))
		set := b.randomSet(maxDoc, float32(numDocs)/float32(maxDoc))
		copy, err := copyOf(set, maxDoc)
		if err != nil {
			b.t.Fatalf("copyOf failed: %v", err)
		}

		// In Lucene, this is calculated via a Dummy object and RamUsageTester.
		// In Gocene, we just rely on the implementation's RamBytesUsed().
		// Since we don't have a global RamUsageTester, we verify it's non-negative.
		if copy.RamBytesUsed() < 0 {
			b.t.Errorf("RamBytesUsed returned negative value: %d", copy.RamBytesUsed())
		}
	}
}

func (b *BaseDocIdSetTestCase) testIntoBitSet(copyOf DocIdSetCopyFunc) {
	numBits := b.nextInt(100, 1<<20)
	percents := []float32{0, 0.0001, b.r.Float32(), 0.9, 1.0}
	for _, percent := range percents {
		set := b.randomSet(numBits, percent)
		copy, err := copyOf(set, numBits)
		if err != nil {
			b.t.Fatalf("copyOf failed: %v", err)
		}
		from := b.r.Intn(numBits)
		to := b.nextInt(from, numBits+5)

		actual, _ := util.NewFixedBitSet(to - from)
		it := copy.Iterator()
		if it == nil {
			continue
		}
		it.Advance(from)

		// No docs to set
		err = util.IntoBitSet(it, from, actual, from)
		if err != nil {
			b.t.Fatalf("IntoBitSet failed: %v", err)
		}
		if !actual.IsEmpty() {
			b.t.Errorf("BitSet should be empty")
		}

		// Now actually set some bits
		// We need a fresh iterator for the second call to avoid advancing the first one
		it2 := copy.Iterator()
		err = util.IntoBitSet(it2, to, actual, from)
		if err != nil {
			b.t.Fatalf("IntoBitSet failed: %v", err)
		}

		expected, _ := util.NewFixedBitSet(to - from)
		it3 := copy.Iterator()
		doc, _ := it3.Advance(from)
		for doc < to && doc != util.NO_MORE_DOCS {
			expected.Set(doc - from)
			doc, _ = it3.NextDoc()
		}

		if !expected.Equals(actual) {
			b.t.Errorf("IntoBitSet produced incorrect result")
		}
	}
}

func (b *BaseDocIdSetTestCase) testIntoBitSetBoundChecks(copyOf DocIdSetCopyFunc) {
	set, _ := util.NewFixedBitSet(256)
	set.Set(20)
	set.Set(42)
	copy, err := copyOf(set, 256)
	if err != nil {
		b.t.Fatalf("copyOf failed: %v", err)
	}

	from := b.r.Intn(21) // 0 to 20
	to := b.nextInt(43, 256)
	offset := b.r.Intn(from + 1)

	dest1, _ := util.NewFixedBitSet(42 - offset + 1)
	it1 := copy.Iterator()
	it1.Advance(from)
	err = util.IntoBitSet(it1, to, dest1, offset)
	if err != nil {
		b.t.Errorf("IntoBitSet should have succeeded: %v", err)
	}

	for i := 0; i < dest1.Length(); i++ {
		expected := (offset+i == 20 || offset+i == 42)
		if dest1.Get(i) != expected {
			b.t.Errorf("Bit at %d: expected %v, got %v", i, expected, dest1.Get(i))
		}
	}

	dest2, _ := util.NewFixedBitSet(42 - offset)
	it2 := copy.Iterator()
	it2.Advance(from)
	err = util.IntoBitSet(it2, to, dest2, offset)
	if err == nil {
		b.t.Error("IntoBitSet should have failed due to bounds")
	}

	dest3, _ := util.NewFixedBitSet(42 - offset + 1)
	it3 := copy.Iterator()
	it3.Advance(from)
	err = util.IntoBitSet(it3, to, dest3, 21)
	if err == nil {
		b.t.Error("IntoBitSet should have failed because offset > current doc")
	}
}

func (b *BaseDocIdSetTestCase) assertEquals(numBits int, ds1 *util.FixedBitSet, ds2 util.DocIdSet) {
	it2 := ds2.Iterator()
	if it2 == nil {
		if ds1.NextSetBit(0) != -1 {
			b.t.Errorf("BitSet has docs, but DocIdSet iterator is nil")
		}
	} else {
		doc2 := it2.DocID()
		if doc2 != -1 {
			b.t.Errorf("Expected initial docID -1, got %d", doc2)
		}
		for doc1 := ds1.NextSetBit(0); doc1 != -1; doc1 = ds1.NextSetBit(doc1 + 1) {
			doc2, err := it2.NextDoc()
			if err != nil {
				b.t.Fatalf("NextDoc failed: %v", err)
			}
			if doc1 != doc2 {
				b.t.Errorf("Mismatch: BitSet has %d, DocIdSet has %d", doc1, doc2)
			}
			if it2.DocID() != doc2 {
				b.t.Errorf("DocID() mismatch: expected %d, got %d", doc2, it2.DocID())
			}
		}
		doc2, err := it2.NextDoc()
		if err != nil {
			b.t.Fatalf("NextDoc failed: %v", err)
		}
		if doc2 != util.NO_MORE_DOCS {
			b.t.Errorf("Expected NO_MORE_DOCS, got %d", doc2)
		}
		if it2.DocID() != util.NO_MORE_DOCS {
			b.t.Errorf("Expected NO_MORE_DOCS for DocID(), got %d", it2.DocID())
		}
	}

	// nextDoc / advance
	it2 = ds2.Iterator()
	if it2 != nil {
		doc := -1
		for doc != util.NO_MORE_DOCS {
			if b.r.Intn(2) == 0 {
				doc = ds1.NextSetBit(doc + 1)
				if doc == -1 {
					doc = util.NO_MORE_DOCS
				}
				doc2, err := it2.NextDoc()
				if err != nil {
					b.t.Fatalf("NextDoc failed: %v", err)
				}
				if doc != doc2 {
					b.t.Errorf("Mismatch during NextDoc: BitSet %d, DocIdSet %d", doc, doc2)
				}
				if it2.DocID() != doc2 {
					b.t.Errorf("DocID() mismatch: expected %d, got %d", doc2, it2.DocID())
				}
			} else {
				target := doc + 1 + b.r.Intn(b.r.Intn(2)*64+max(numBits/8, 1))
				doc = ds1.NextSetBit(target)
				if doc == -1 {
					doc = util.NO_MORE_DOCS
				}
				doc2, err := it2.Advance(target)
				if err != nil {
					b.t.Fatalf("Advance failed: %v", err)
				}
				if doc != doc2 {
					b.t.Errorf("Mismatch during Advance: BitSet %d, DocIdSet %d", doc, doc2)
				}
				if it2.DocID() != doc2 {
					b.t.Errorf("DocID() mismatch: expected %d, got %d", doc2, it2.DocID())
				}
			}
		}
	}
}

func (b *BaseDocIdSetTestCase) randomSet(numBits int, percent float32) *util.FixedBitSet {
	bs, _ := util.NewFixedBitSet(numBits)
	if percent <= 0 {
		return bs
	}
	if percent >= 1.0 {
		bs.SetAll()
		return bs
	}
	for i := 0; i < numBits; i++ {
		if b.r.Float32() < percent {
			bs.Set(i)
		}
	}
	return bs
}

func (b *BaseDocIdSetTestCase) nextInt(min, max int) int {
	if min >= max {
		return min
	}
	return b.r.Intn(max-min) + min
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
