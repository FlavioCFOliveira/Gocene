// Package hppc contains the stand-in declarations for
// org.apache.lucene.internal.hppc — Lucene's specialised primitive
// collections.
//
// The Gocene port relies on Go's native data structures ([]int, map[int]int,
// etc.) rather than HPPC's hand-tuned arrays, so each type here is reduced
// to a thin alias that documents which native form callers should reach
// for. The declarations exist so the codec/index packages that reference
// them can compile against a stable name; reach-into-internal callers
// should migrate to the native equivalents as they update.
package hppc

// AbstractIterator is a marker for the generic-iterator base class.
type AbstractIterator[T any] struct{}

// BitMixer is the static hash-mixing helper.
type BitMixer struct{}

// Mix64 mirrors Lucene's HPPC bit-mixing for 64-bit hashes.
func (BitMixer) Mix64(v uint64) uint64 {
	v ^= v >> 33
	v *= 0xff51afd7ed558ccd
	v ^= v >> 33
	v *= 0xc4ceb9fe1a85ec53
	v ^= v >> 33
	return v
}

// BufferAllocationException flags a buffer-allocation failure.
type BufferAllocationException struct{ Message string }

func (e *BufferAllocationException) Error() string { return e.Message }

// Cursor types — used in HPPC iteration. Each is a tiny tuple.
type CharCursor struct {
	Index int
	Value rune
}

type DoubleCursor struct {
	Index int
	Value float64
}

type FloatCursor struct {
	Index int
	Value float32
}

type IntCursor struct {
	Index int
	Value int32
}

type LongCursor struct {
	Index int
	Value int64
}

type ObjectCursor[T any] struct {
	Index int
	Value T
}

// Collection aliases — every HPPC collection maps to a Go native equivalent.
// The aliases exist so the codec/index packages can compile against the
// canonical name while callers migrate.

type CharHashSet = map[rune]struct{}
type CharObjectHashMap[V any] = map[rune]V
type FloatArrayList = []float32
type IntArrayList = []int32
type LongArrayList = []int64
type IntHashSet = map[int32]struct{}
type IntIntHashMap = map[int32]int32
type IntLongHashMap = map[int32]int64
type IntFloatHashMap = map[int32]float32
type IntDoubleHashMap = map[int32]float64
type IntObjectHashMap[V any] = map[int32]V
type LongHashSet = map[int64]struct{}
type LongIntHashMap = map[int64]int32
type LongFloatHashMap = map[int64]float32
type LongObjectHashMap[V any] = map[int64]V

// MaxSized variants enforce an upper bound at the call site rather than via
// a wrapper type.
type MaxSizedFloatArrayList = []float32
type MaxSizedIntArrayList = []int32
