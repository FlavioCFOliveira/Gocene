// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

// PackedLongValuesDefaultPageSize is the page size used by the
// zero-argument builder helpers; matches Lucene's DEFAULT_PAGE_SIZE.
const PackedLongValuesDefaultPageSize = 256

const (
	packedLongValuesMinPageSize = 64
	packedLongValuesMaxPageSize = 1 << 20
)

// packStrategy abstracts how a block's pending int64 buffer is
// packed into a PackedInts.Reader; the variants Lucene exposes
// (packed / delta-packed) plug a different implementation in.
type packStrategy interface {
	// pack writes the contents of pending[0:numValues] into the block.
	// May mutate pending (e.g. delta strategies subtract a min).
	pack(pending []int64, numValues, block int, ratio float32, plv *PackedLongValues)
	// get returns the value at (block, element), undoing any per-block
	// adjustment applied by pack.
	get(plv *PackedLongValues, block, element int) int64
	// describe returns a human-readable strategy name (debug only).
	describe() string
}

// PackedLongValues is a compressed long sequence built block-by-block.
// Use one of the *Builder* constructors to produce a value, then call
// Get(index) or Iterator() to read it back.
//
// This is the Go port of org.apache.lucene.util.packed.PackedLongValues
// in Apache Lucene 10.4.0 (including DeltaPackedLongValues).
type PackedLongValues struct {
	values       []Reader
	mins         []int64   // populated by delta/monotonic strategies; nil for packed
	averages     []float32 // populated by monotonic strategy only; nil otherwise
	pageShift    int
	pageMask     int
	size         int64
	ramBytesUsed int64
	strategy     packStrategy
}

// Size returns the number of values stored.
func (p *PackedLongValues) Size() int64 { return p.size }

// RamBytesUsed reports the heap size of the structure.
func (p *PackedLongValues) RamBytesUsed() int64 { return p.ramBytesUsed }

// Get returns the value at the given flat index.
func (p *PackedLongValues) Get(index int64) int64 {
	block := int(index >> uint(p.pageShift))
	element := int(index) & p.pageMask
	return p.strategy.get(p, block, element)
}

// decodeBlock decodes block i into dest and returns the value count.
// Iterator uses this to amortise the per-value Get cost.
func (p *PackedLongValues) decodeBlock(block int, dest []int64) int {
	r := p.values[block]
	size := r.Size()
	for k := 0; k < size; {
		k += r.GetBulk(k, dest, k, size-k)
	}
	if p.mins != nil {
		min := p.mins[block]
		for i := 0; i < size; i++ {
			dest[i] += min
		}
	}
	if p.averages != nil {
		avg := p.averages[block]
		for i := 0; i < size; i++ {
			dest[i] += monotonicExpected(0, avg, i)
		}
	}
	return size
}

// PackedLongValuesIterator iterates a PackedLongValues sequentially.
// Materialise it via plv.Iterator().
type PackedLongValuesIterator struct {
	plv           *PackedLongValues
	currentValues []int64
	vOff, pOff    int
	currentCount  int
}

// Iterator returns a fresh sequential iterator.
func (p *PackedLongValues) Iterator() *PackedLongValuesIterator {
	pageSize := p.pageMask + 1
	if int64(pageSize) > p.size {
		pageSize = int(p.size)
	}
	it := &PackedLongValuesIterator{
		plv:           p,
		currentValues: make([]int64, pageSize),
	}
	it.fillBlock()
	return it
}

func (it *PackedLongValuesIterator) fillBlock() {
	if it.vOff == len(it.plv.values) {
		it.currentCount = 0
		return
	}
	it.currentCount = it.plv.decodeBlock(it.vOff, it.currentValues)
}

// HasNext reports whether more values remain.
func (it *PackedLongValuesIterator) HasNext() bool { return it.pOff < it.currentCount }

// Next returns the next value; HasNext must be true.
func (it *PackedLongValuesIterator) Next() int64 {
	v := it.currentValues[it.pOff]
	it.pOff++
	if it.pOff == it.currentCount {
		it.vOff++
		it.pOff = 0
		it.fillBlock()
	}
	return v
}

// PackedLongValuesBuilder accumulates values into a PackedLongValues.
type PackedLongValuesBuilder struct {
	pageShift               int
	pageMask                int
	acceptableOverheadRatio float32
	pending                 []int64
	pendingOff              int
	size                    int64
	valuesOff               int
	values                  []Reader
	mins                    []int64
	averages                []float32
	strategy                packStrategy
	ramBytesUsed            int64
}

// PackedBuilder returns a builder that packs values directly (no
// delta encoding).
func PackedBuilder(pageSize int, acceptableOverheadRatio float32) (*PackedLongValuesBuilder, error) {
	return newPackedBuilder(pageSize, acceptableOverheadRatio, false)
}

// DeltaPackedBuilder returns a builder that subtracts a per-block min
// before packing — efficient for sequences whose values are close to
// each other.
func DeltaPackedBuilder(pageSize int, acceptableOverheadRatio float32) (*PackedLongValuesBuilder, error) {
	return newPackedBuilder(pageSize, acceptableOverheadRatio, true)
}

func newPackedBuilder(pageSize int, ratio float32, delta bool) (*PackedLongValuesBuilder, error) {
	pageShift, err := CheckBlockSize(pageSize, packedLongValuesMinPageSize, packedLongValuesMaxPageSize)
	if err != nil {
		return nil, err
	}
	b := &PackedLongValuesBuilder{
		pageShift:               pageShift,
		pageMask:                pageSize - 1,
		acceptableOverheadRatio: ratio,
		pending:                 make([]int64, pageSize),
		values:                  make([]Reader, 16),
	}
	if delta {
		b.strategy = deltaPackStrategy{}
		b.mins = make([]int64, 16)
	} else {
		b.strategy = plainPackStrategy{}
	}
	return b, nil
}

// Add appends a value to the builder.
func (b *PackedLongValuesBuilder) Add(v int64) error {
	if b.pending == nil {
		return errAlreadyBuilt
	}
	if b.pendingOff == len(b.pending) {
		if b.valuesOff == len(b.values) {
			b.grow(b.valuesOff + 1)
		}
		b.pack()
	}
	b.pending[b.pendingOff] = v
	b.pendingOff++
	b.size++
	return nil
}

// Build finalises and returns the immutable PackedLongValues. The
// builder is no longer usable.
func (b *PackedLongValuesBuilder) Build() *PackedLongValues {
	b.finish()
	b.pending = nil
	values := make([]Reader, b.valuesOff)
	copy(values, b.values[:b.valuesOff])
	var mins []int64
	if b.mins != nil {
		mins = make([]int64, b.valuesOff)
		copy(mins, b.mins[:b.valuesOff])
	}
	var averages []float32
	if b.averages != nil {
		averages = make([]float32, b.valuesOff)
		copy(averages, b.averages[:b.valuesOff])
	}
	return &PackedLongValues{
		pageShift:    b.pageShift,
		pageMask:     b.pageMask,
		values:       values,
		mins:         mins,
		averages:     averages,
		size:         b.size,
		ramBytesUsed: packedLongValuesBytesUsed(values, mins, averages),
		strategy:     b.strategy,
	}
}

// Size returns the number of values added so far.
func (b *PackedLongValuesBuilder) Size() int64 { return b.size }

func (b *PackedLongValuesBuilder) finish() {
	if b.pendingOff > 0 {
		if b.valuesOff == len(b.values) {
			b.grow(b.valuesOff + 1)
		}
		b.pack()
	}
}

func (b *PackedLongValuesBuilder) grow(newBlockCount int) {
	cap := len(b.values)
	for cap < newBlockCount {
		if cap == 0 {
			cap = 1
		} else {
			cap *= 2
		}
	}
	next := make([]Reader, cap)
	copy(next, b.values[:b.valuesOff])
	b.values = next
	if b.mins != nil {
		nextMins := make([]int64, cap)
		copy(nextMins, b.mins[:b.valuesOff])
		b.mins = nextMins
	}
	if b.averages != nil {
		nextAvg := make([]float32, cap)
		copy(nextAvg, b.averages[:b.valuesOff])
		b.averages = nextAvg
	}
}

func (b *PackedLongValuesBuilder) pack() {
	plv := &PackedLongValues{values: b.values, mins: b.mins, averages: b.averages, pageMask: b.pageMask}
	b.strategy.pack(b.pending, b.pendingOff, b.valuesOff, b.acceptableOverheadRatio, plv)
	b.values = plv.values
	b.mins = plv.mins
	b.averages = plv.averages
	b.valuesOff++
	b.pendingOff = 0
}

// errAlreadyBuilt is returned from Add after Build has been called.
var errAlreadyBuilt = packedLongValuesError("PackedLongValuesBuilder cannot be reused after Build()")

type packedLongValuesError string

func (e packedLongValuesError) Error() string { return string(e) }

func packedLongValuesBytesUsed(values []Reader, mins []int64, averages []float32) int64 {
	var bytes int64
	for _, r := range values {
		bytes += r.RamBytesUsed()
	}
	if mins != nil {
		bytes += int64(8 * len(mins))
	}
	if averages != nil {
		bytes += int64(4 * len(averages))
	}
	return bytes + 64
}

// plainPackStrategy packs values without any per-block adjustment.
type plainPackStrategy struct{}

func (plainPackStrategy) describe() string { return "packed" }

func (plainPackStrategy) get(plv *PackedLongValues, block, element int) int64 {
	return plv.values[block].Get(element)
}

func (plainPackStrategy) pack(pending []int64, numValues, block int, ratio float32, plv *PackedLongValues) {
	if numValues <= 0 {
		return
	}
	minValue := pending[0]
	maxValue := pending[0]
	for i := 1; i < numValues; i++ {
		if pending[i] < minValue {
			minValue = pending[i]
		}
		if pending[i] > maxValue {
			maxValue = pending[i]
		}
	}
	if minValue == 0 && maxValue == 0 {
		plv.values[block] = NewNullReader(numValues)
		return
	}
	bitsRequired := 64
	if minValue >= 0 {
		bitsRequired = BitsRequired(maxValue)
	}
	mut := GetMutable(numValues, bitsRequired, ratio)
	for i := 0; i < numValues; {
		i += mut.SetBulk(i, pending, i, numValues-i)
	}
	plv.values[block] = mut
}

// deltaPackStrategy subtracts a per-block minimum and packs the
// resulting non-negative deltas.
type deltaPackStrategy struct{}

func (deltaPackStrategy) describe() string { return "delta-packed" }

func (deltaPackStrategy) get(plv *PackedLongValues, block, element int) int64 {
	return plv.mins[block] + plv.values[block].Get(element)
}

func (deltaPackStrategy) pack(pending []int64, numValues, block int, ratio float32, plv *PackedLongValues) {
	if numValues <= 0 {
		return
	}
	min := pending[0]
	for i := 1; i < numValues; i++ {
		if pending[i] < min {
			min = pending[i]
		}
	}
	for i := 0; i < numValues; i++ {
		pending[i] -= min
	}
	plainPackStrategy{}.pack(pending, numValues, block, ratio, plv)
	if plv.mins == nil {
		// Defensive: should never happen when constructed via DeltaPackedBuilder.
		plv.mins = make([]int64, len(plv.values))
	}
	plv.mins[block] = min
}
