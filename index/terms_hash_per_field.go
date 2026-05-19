// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// termsHashInitSize mirrors Lucene's HASH_INIT_SIZE constant.
const termsHashInitSize = 4

// ParallelPostingsArray maps a term ID to its text-start offset inside a
// BytesRefHash, the per-stream address inside an IntBlockPool and the
// per-stream byte-start offset inside a ByteBlockPool.
//
// Port of Lucene's org.apache.lucene.index.ParallelPostingsArray. The struct
// is kept package-private (named-exported in Go) because it is only consumed
// by TermsHashPerField and its concrete subtypes (FreqProx / TermVectors).
//
// Divergence: Lucene relies on subclass overrides of newInstance/grow/copyTo
// to grow side arrays. Go does not support that pattern, so concrete
// PostingsArrays embed *ParallelPostingsArray and re-implement Grow via the
// CreatePostingsArray hook on TermsHashPerField (Lucene's createPostingsArray
// abstract method). This keeps the parent struct usable on its own when the
// subclass adds no extra streams.
type ParallelPostingsArray struct {
	// Size is the number of term slots; it does not necessarily equal the
	// current term count.
	Size int
	// TextStarts maps a term ID to the start offset of the term's bytes in
	// the parent BytesRefHash.
	TextStarts []int
	// AddressOffset maps a term ID to the global int-pool offset where the
	// per-stream byte-pool addresses for that term are stored.
	AddressOffset []int
	// ByteStarts maps a term ID to the start offset of the first stream
	// slice in the byte pool.
	ByteStarts []int
}

// NewParallelPostingsArray allocates a base ParallelPostingsArray with the
// given number of term slots. All three side arrays are zero-initialised.
func NewParallelPostingsArray(size int) *ParallelPostingsArray {
	return &ParallelPostingsArray{
		Size:          size,
		TextStarts:    make([]int, size),
		AddressOffset: make([]int, size),
		ByteStarts:    make([]int, size),
	}
}

// BytesPerPosting returns the number of bytes that a single term posting
// occupies in the side arrays. Mirrors Lucene's BYTES_PER_POSTING (3 ints).
func (p *ParallelPostingsArray) BytesPerPosting() int {
	return 3 * 4
}

// CopyTo copies the first numToCopy slots of this array into dst.
func (p *ParallelPostingsArray) CopyTo(dst *ParallelPostingsArray, numToCopy int) {
	copy(dst.TextStarts, p.TextStarts[:numToCopy])
	copy(dst.AddressOffset, p.AddressOffset[:numToCopy])
	copy(dst.ByteStarts, p.ByteStarts[:numToCopy])
}

// TermsHashPerField stores one stream per term for a single field. Each
// stream typically carries one level of information (e.g. doc+freq, or
// prox+offset). Internally the type appends to a linked list of byte slices
// managed by ByteSlicePool, indexed through a BytesRefHash that deduplicates
// terms.
//
// Port of Lucene's org.apache.lucene.index.TermsHashPerField. Lucene declares
// the class abstract and forces subclasses to implement newTerm, addTerm,
// newPostingsArray and createPostingsArray. Gocene translates that contract
// into four function-valued hooks on the struct itself; concrete fields wire
// them in their constructors. Callers must set all four hooks before calling
// any Add / Finish method.
//
// Divergences from Lucene 10.4.0:
//   - The abstract subclass pattern becomes struct-with-hooks (NewTerm,
//     AddTerm, NewPostingsArray, CreatePostingsArray). The hooks are
//     mandatory; calling Add without them panics with a clear message.
//   - PostingsBytesStartArray is implemented as an unexported struct that
//     satisfies util.BytesStartArray, mirroring the Java inner class.
//   - assertDocId(int) and the lastDocID guard are kept but compiled out
//     of hot paths (no -ea analogue in Go). Out-of-order docIDs return an
//     error from Add to keep the contract observable.
//   - The comparator interface (Comparable<TermsHashPerField>) is exposed
//     as a CompareTo(*TermsHashPerField) int method instead of implementing
//     sort.Interface; callers wrap a slice with sort.Slice if needed.
type TermsHashPerField struct {
	// NextPerField is the next per-field handler in the chain. May be nil.
	// The first field in the chain receives raw term bytes; subsequent
	// fields are addressed by pool offset, mirroring Lucene's "term vector"
	// secondary entry point.
	NextPerField *TermsHashPerField

	intPool   *util.IntBlockPool
	bytePool  *util.ByteBlockPool
	slicePool *ByteSlicePool

	// termStreamAddressBuffer caches IntBlockPool.Buffer for the current
	// term so writeByte/writeBytes can index without re-resolving on every
	// call. streamAddressOffset is the term's offset within that buffer.
	termStreamAddressBuffer []int32
	streamAddressOffset     int

	streamCount int
	fieldName   string

	// IndexOptions is the field's index options. It must not be IndexOptionsNone.
	IndexOptions IndexOptions

	// bytesHash deduplicates incoming terms; it owns the term bytes and
	// returns a stable term ID.
	bytesHash *util.BytesRefHash

	// PostingsArray holds the per-term side arrays; concrete subtypes may
	// override it with an extended struct via NewPostingsArray.
	PostingsArray *ParallelPostingsArray

	// doNextCall mirrors Lucene's doNextCall flag: it is set by Start()
	// based on whether the next field in the chain wants to receive a
	// term-vector style notification.
	doNextCall bool

	// lastDocID tracks the most recently indexed docID to detect
	// out-of-order Add calls. Lucene uses a Java assertion; Gocene
	// returns an error so the check is observable in release builds too.
	lastDocID int

	// sortedTermIDs caches the result of SortTerms() between SortTerms and
	// the consumer's flush. It is reset to nil by Reset / ReinitHash.
	sortedTermIDs []int

	// NewTerm is invoked when a term is observed for the first time
	// (called from initStreamSlices after slice allocation).
	NewTerm func(termID, docID int) error
	// AddTerm is invoked when a previously seen term is observed again.
	AddTerm func(termID, docID int) error
	// NewPostingsArray is invoked when PostingsArray is freshly allocated
	// or resized so the subclass can refresh any cached references to the
	// side arrays it owns.
	NewPostingsArray func()
	// CreatePostingsArray returns a freshly allocated postings array of
	// the given size. Concrete subtypes return their own embedded variant.
	CreatePostingsArray func(size int) *ParallelPostingsArray
}

// TermsHashPerFieldHooks bundles the four subclass-equivalent callbacks
// that concrete TermsHashPerField variants must supply. Lucene expresses
// them as abstract methods; Gocene takes them as a value type so the
// constructor can validate them up front. All four fields are required.
type TermsHashPerFieldHooks struct {
	// NewTerm is invoked when a term is observed for the first time
	// (called from initStreamSlices after slice allocation).
	NewTerm func(termID, docID int) error
	// AddTerm is invoked when a previously seen term is observed again.
	AddTerm func(termID, docID int) error
	// NewPostingsArray is invoked when PostingsArray is freshly allocated
	// or resized so the subclass can refresh any cached references to the
	// side arrays it owns. May be nil if no caching is needed.
	NewPostingsArray func()
	// CreatePostingsArray returns a freshly allocated postings array of
	// the given size. Concrete subtypes return their own embedded variant.
	CreatePostingsArray func(size int) *ParallelPostingsArray
}

// NewTermsHashPerField wires a new per-field handler.
//
// streamCount describes how many parallel streams this field encodes per
// term (e.g. 1 for doc+freq, 2 for doc+freq plus prox+offset).
//
// intPool, bytePool and termBytePool are shared across all per-field
// handlers belonging to the same DocumentsWriterPerThread.
//
// bytesUsed accumulates the memory consumed by the postings side arrays.
//
// nextPerField, fieldName and indexOptions correspond to the same Lucene
// parameters. indexOptions must not be IndexOptionsNone.
//
// hooks supplies the subclass-equivalent callbacks. NewTerm, AddTerm and
// CreatePostingsArray are mandatory; NewPostingsArray may be nil.
func NewTermsHashPerField(
	streamCount int,
	intPool *util.IntBlockPool,
	bytePool *util.ByteBlockPool,
	termBytePool *util.ByteBlockPool,
	bytesUsed *util.Counter,
	nextPerField *TermsHashPerField,
	fieldName string,
	indexOptions IndexOptions,
	hooks TermsHashPerFieldHooks,
) (*TermsHashPerField, error) {
	if indexOptions == IndexOptionsNone {
		return nil, fmt.Errorf("TermsHashPerField: indexOptions must not be NONE for field %q", fieldName)
	}
	if intPool == nil {
		return nil, fmt.Errorf("TermsHashPerField: intPool must not be nil")
	}
	if bytePool == nil {
		return nil, fmt.Errorf("TermsHashPerField: bytePool must not be nil")
	}
	if termBytePool == nil {
		return nil, fmt.Errorf("TermsHashPerField: termBytePool must not be nil")
	}
	if hooks.NewTerm == nil || hooks.AddTerm == nil || hooks.CreatePostingsArray == nil {
		return nil, fmt.Errorf("TermsHashPerField: NewTerm, AddTerm and CreatePostingsArray hooks are mandatory")
	}
	if bytesUsed == nil {
		bytesUsed = util.NewCounter()
	}

	t := &TermsHashPerField{
		NextPerField:        nextPerField,
		intPool:             intPool,
		bytePool:            bytePool,
		slicePool:           NewByteSlicePool(bytePool),
		streamCount:         streamCount,
		fieldName:           fieldName,
		IndexOptions:        indexOptions,
		NewTerm:             hooks.NewTerm,
		AddTerm:             hooks.AddTerm,
		NewPostingsArray:    hooks.NewPostingsArray,
		CreatePostingsArray: hooks.CreatePostingsArray,
	}
	startArray := newPostingsBytesStartArray(t, bytesUsed)
	t.bytesHash = util.NewBytesRefHashWithCapacity(termBytePool, termsHashInitSize, startArray)
	return t, nil
}

// Reset clears the per-field state and propagates the reset down the chain.
// The underlying byte pool is NOT released.
func (t *TermsHashPerField) Reset() {
	t.bytesHash.Clear(false)
	t.sortedTermIDs = nil
	t.lastDocID = 0
	if t.NextPerField != nil {
		t.NextPerField.Reset()
	}
}

// InitReader configures reader to read stream `stream` for term `termID`.
// stream must be smaller than the streamCount supplied at construction time.
func (t *TermsHashPerField) InitReader(reader *ByteSliceReader, termID, stream int) error {
	if stream < 0 || stream >= t.streamCount {
		return fmt.Errorf("TermsHashPerField: stream %d out of range [0,%d)", stream, t.streamCount)
	}
	streamStartOffset := t.PostingsArray.AddressOffset[termID]
	addrBuffer := t.intPool.Buffers[streamStartOffset>>util.IntBlockShift]
	offsetInAddrBuffer := streamStartOffset & util.IntBlockMask
	start := t.PostingsArray.ByteStarts[termID] + stream*ByteSliceFirstLevelSize
	end := int(addrBuffer[offsetInAddrBuffer+stream])
	return reader.Init(t.bytePool, start, end)
}

// SortTerms collapses the BytesRefHash and sorts the resulting term IDs
// in-place. The result is cached on the receiver and returned by
// GetSortedTermIDs. Must not be called twice without an intervening Reset
// or ReinitHash.
func (t *TermsHashPerField) SortTerms() {
	if t.sortedTermIDs != nil {
		panic("TermsHashPerField: SortTerms called twice without Reset/ReinitHash")
	}
	t.sortedTermIDs = t.bytesHash.Sort()
}

// GetSortedTermIDs returns the sorted term IDs computed by SortTerms.
// Panics if SortTerms has not been called.
func (t *TermsHashPerField) GetSortedTermIDs() []int {
	if t.sortedTermIDs == nil {
		panic("TermsHashPerField: GetSortedTermIDs called before SortTerms")
	}
	return t.sortedTermIDs
}

// ReinitHash resets the BytesRefHash to its post-Clear state and clears any
// cached sortedTermIDs, allowing the per-field handler to be reused.
func (t *TermsHashPerField) ReinitHash() {
	t.sortedTermIDs = nil
	t.bytesHash.Reinit()
}

// AddByPoolOffset is the secondary entry point used by chained handlers
// (e.g. the term-vectors layer). The term bytes are already interned, so
// we hash by the pool offset rather than the bytes themselves.
func (t *TermsHashPerField) AddByPoolOffset(textStart, docID int) error {
	termID := t.bytesHash.AddByPoolOffset(textStart)
	if termID >= 0 {
		return t.initStreamSlices(termID, docID)
	}
	_, err := t.positionStreamSlice(termID, docID)
	return err
}

// Add is the primary entry point. It interns termBytes into the per-field
// BytesRefHash, allocates per-term stream slices on the first observation,
// dispatches the NewTerm / AddTerm hook, and finally forwards the call to
// the next handler in the chain when Start() requested it.
func (t *TermsHashPerField) Add(termBytes *util.BytesRef, docID int) error {
	if err := t.assertDocID(docID); err != nil {
		return err
	}
	termID, err := t.bytesHash.Add(termBytes)
	if err != nil {
		return err
	}
	if termID >= 0 {
		if err := t.initStreamSlices(termID, docID); err != nil {
			return err
		}
	} else {
		termID, err = t.positionStreamSlice(termID, docID)
		if err != nil {
			return err
		}
	}
	if t.doNextCall && t.NextPerField != nil {
		return t.NextPerField.AddByPoolOffset(t.PostingsArray.TextStarts[termID], docID)
	}
	return nil
}

// initStreamSlices reserves one slice per stream for a freshly-seen term,
// records the resulting addresses in the int pool, and dispatches NewTerm.
func (t *TermsHashPerField) initStreamSlices(termID, docID int) error {
	if t.NewTerm == nil || t.CreatePostingsArray == nil {
		panic("TermsHashPerField: NewTerm/CreatePostingsArray hooks not set")
	}
	if t.streamCount+t.intPool.IntUpto > util.IntBlockSize {
		t.intPool.NextBuffer()
	}
	if util.ByteBlockSize-t.bytePool.ByteUpto < (2*t.streamCount)*ByteSliceFirstLevelSize {
		t.bytePool.NextBuffer()
	}

	t.termStreamAddressBuffer = t.intPool.Buffer
	t.streamAddressOffset = t.intPool.IntUpto
	t.intPool.IntUpto += t.streamCount

	t.PostingsArray.AddressOffset[termID] = t.streamAddressOffset + t.intPool.IntOffset

	for i := 0; i < t.streamCount; i++ {
		upto, err := t.slicePool.NewSlice(ByteSliceFirstLevelSize)
		if err != nil {
			return err
		}
		t.termStreamAddressBuffer[t.streamAddressOffset+i] = int32(upto + t.bytePool.ByteOffset)
	}
	t.PostingsArray.ByteStarts[termID] = int(t.termStreamAddressBuffer[t.streamAddressOffset])
	return t.NewTerm(termID, docID)
}

// assertDocID enforces the monotonic-docID contract. Lucene relies on a
// Java assertion; Gocene returns an error so the violation is observable
// in release builds.
func (t *TermsHashPerField) assertDocID(docID int) error {
	if docID < t.lastDocID {
		return fmt.Errorf("TermsHashPerField: docID must be >= %d but was %d", t.lastDocID, docID)
	}
	t.lastDocID = docID
	return nil
}

// positionStreamSlice loads the cached per-stream addresses for an existing
// term and dispatches AddTerm.
func (t *TermsHashPerField) positionStreamSlice(termID, docID int) (int, error) {
	if t.AddTerm == nil {
		panic("TermsHashPerField: AddTerm hook not set")
	}
	termID = (-termID) - 1
	intStart := t.PostingsArray.AddressOffset[termID]
	t.termStreamAddressBuffer = t.intPool.Buffers[intStart>>util.IntBlockShift]
	t.streamAddressOffset = intStart & util.IntBlockMask
	if err := t.AddTerm(termID, docID); err != nil {
		return termID, err
	}
	return termID, nil
}

// WriteStreamByte appends b to the requested stream, hopping to a new slice when
// the current one is full.
func (t *TermsHashPerField) WriteStreamByte(stream int, b byte) {
	streamAddress := t.streamAddressOffset + stream
	upto := int(t.termStreamAddressBuffer[streamAddress])
	bs := t.bytePool.GetBuffer(upto >> util.ByteBlockShift)
	offset := upto & util.ByteBlockMask
	if bs[offset] != 0 {
		// End of slice; allocate a new one.
		offset = t.slicePool.AllocSlice(bs, offset)
		bs = t.bytePool.Buffer
		t.termStreamAddressBuffer[streamAddress] = int32(offset + t.bytePool.ByteOffset)
	}
	bs[offset] = b
	t.termStreamAddressBuffer[streamAddress]++
}

// WriteStreamBytes appends len bytes of b starting at offset to the requested
// stream, hopping across slices as needed.
func (t *TermsHashPerField) WriteStreamBytes(stream int, b []byte, offset, length int) {
	end := offset + length
	streamAddress := t.streamAddressOffset + stream
	upto := int(t.termStreamAddressBuffer[streamAddress])
	slice := t.bytePool.GetBuffer(upto >> util.ByteBlockShift)
	sliceOffset := upto & util.ByteBlockMask

	for sliceOffset < len(slice) && slice[sliceOffset] == 0 && offset < end {
		slice[sliceOffset] = b[offset]
		sliceOffset++
		offset++
		t.termStreamAddressBuffer[streamAddress]++
	}

	for offset < end {
		packed := t.slicePool.AllocKnownSizeSlice(slice, sliceOffset)
		sliceOffset = packed >> 8
		sliceLen := packed & 0xff
		slice = t.bytePool.Buffer
		writeLen := sliceLen - 1
		if remaining := end - offset; remaining < writeLen {
			writeLen = remaining
		}
		copy(slice[sliceOffset:sliceOffset+writeLen], b[offset:offset+writeLen])
		sliceOffset += writeLen
		offset += writeLen
		t.termStreamAddressBuffer[streamAddress] = int32(sliceOffset + t.bytePool.ByteOffset)
	}
}

// WriteStreamVInt encodes i as a variable-length integer in the requested stream.
func (t *TermsHashPerField) WriteStreamVInt(stream int, i int32) {
	if stream < 0 || stream >= t.streamCount {
		panic(fmt.Sprintf("TermsHashPerField: stream %d out of range [0,%d)", stream, t.streamCount))
	}
	u := uint32(i)
	for u&^0x7F != 0 {
		t.WriteStreamByte(stream, byte(u&0x7f)|0x80)
		u >>= 7
	}
	t.WriteStreamByte(stream, byte(u))
}

// GetNextPerField returns the next per-field handler in the chain (may be nil).
func (t *TermsHashPerField) GetNextPerField() *TermsHashPerField {
	return t.NextPerField
}

// GetFieldName returns the field name supplied at construction time.
func (t *TermsHashPerField) GetFieldName() string {
	return t.fieldName
}

// CompareTo implements Lucene's Comparable contract by field name.
// Wrap a []*TermsHashPerField with sort.Slice when ordering is required.
func (t *TermsHashPerField) CompareTo(other *TermsHashPerField) int {
	switch {
	case t.fieldName < other.fieldName:
		return -1
	case t.fieldName > other.fieldName:
		return 1
	default:
		return 0
	}
}

// Finish flushes the current document and propagates the call down the chain.
// Concrete subtypes typically override behaviour through additional hooks; the
// base implementation is a pass-through, matching Lucene's non-abstract
// finish() body.
func (t *TermsHashPerField) Finish() error {
	if t.NextPerField != nil {
		return t.NextPerField.Finish()
	}
	return nil
}

// GetNumTerms returns the number of distinct terms recorded for this field.
func (t *TermsHashPerField) GetNumTerms() int {
	return t.bytesHash.Size()
}

// Start begins a new occurrence of `field` in the current document. first
// must be true the first time this field name is seen within the document.
// The base implementation always returns true; concrete subtypes may decide
// to gate the field. The next handler in the chain is consulted to update
// doNextCall, mirroring Lucene's behaviour.
func (t *TermsHashPerField) Start(field IndexableField, first bool) bool {
	if t.NextPerField != nil {
		t.doNextCall = t.NextPerField.Start(field, first)
	}
	return true
}

// postingsBytesStartArray adapts util.BytesStartArray for the per-field
// postings arrays. It mirrors Lucene's PostingsBytesStartArray inner class.
type postingsBytesStartArray struct {
	perField  *TermsHashPerField
	bytesUsed *util.Counter
}

func newPostingsBytesStartArray(perField *TermsHashPerField, bytesUsed *util.Counter) *postingsBytesStartArray {
	return &postingsBytesStartArray{perField: perField, bytesUsed: bytesUsed}
}

// Init allocates the initial postings array if one is not present yet and
// returns the TextStarts side array used by the parent BytesRefHash.
func (a *postingsBytesStartArray) Init() []int {
	if a.perField.PostingsArray == nil {
		if a.perField.CreatePostingsArray == nil {
			panic("TermsHashPerField: CreatePostingsArray hook not set before BytesRefHash init")
		}
		a.perField.PostingsArray = a.perField.CreatePostingsArray(2)
		if a.perField.NewPostingsArray != nil {
			a.perField.NewPostingsArray()
		}
		size := a.perField.PostingsArray.Size
		a.bytesUsed.AddAndGet(int64(size) * int64(a.perField.PostingsArray.BytesPerPosting()))
	}
	return a.perField.PostingsArray.TextStarts
}

// Grow expands the postings array via the CreatePostingsArray hook.
func (a *postingsBytesStartArray) Grow() []int {
	if a.perField.CreatePostingsArray == nil {
		panic("TermsHashPerField: CreatePostingsArray hook not set before BytesRefHash grow")
	}
	old := a.perField.PostingsArray
	newSize := util.Oversize(old.Size+1, old.BytesPerPosting())
	grown := a.perField.CreatePostingsArray(newSize)
	old.CopyTo(grown, old.Size)
	a.perField.PostingsArray = grown
	if a.perField.NewPostingsArray != nil {
		a.perField.NewPostingsArray()
	}
	a.bytesUsed.AddAndGet(int64(grown.BytesPerPosting()) * int64(grown.Size-old.Size))
	return grown.TextStarts
}

// Clear discards the postings array and refunds its memory to the counter.
func (a *postingsBytesStartArray) Clear() []int {
	if a.perField.PostingsArray != nil {
		size := a.perField.PostingsArray.Size
		a.bytesUsed.AddAndGet(-int64(size) * int64(a.perField.PostingsArray.BytesPerPosting()))
		a.perField.PostingsArray = nil
		if a.perField.NewPostingsArray != nil {
			a.perField.NewPostingsArray()
		}
	}
	return nil
}

// BytesUsed returns the shared memory counter.
func (a *postingsBytesStartArray) BytesUsed() *util.Counter {
	return a.bytesUsed
}
