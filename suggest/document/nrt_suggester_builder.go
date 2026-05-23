package document

import (
	"container/heap"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	util "github.com/FlavioCFOliveira/Gocene/util"
	utilfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// payloadSep is the label used to separate the surface form and docID inside
// the FST output. Mirrors NRTSuggesterBuilder.PAYLOAD_SEP
// (ConcatenateGraphFilter.SEP_LABEL = 0x001F).
const payloadSep = 0x001F

// endByte marks the end of the analysed input and the start of the dedup byte.
// Mirrors NRTSuggesterBuilder.END_BYTE.
const endByte = 0x00

// maxDocIDLenWithSep is the maximum number of extra bytes for the separator
// byte plus a vint-encoded document ID (1 + 5 = 6).
// Mirrors NRTSuggester.PayLoadProcessor.MAX_DOC_ID_LEN_WITH_SEP.
const maxDocIDLenWithSep = 6

// integerMaxValue mirrors Java's Integer.MAX_VALUE.
const integerMaxValue = int64(^uint32(0) >> 1) // 2147483647

// NRTSuggesterEncode encodes a weight for storage in the FST.
// Mirrors NRTSuggester.encode(long).
func NRTSuggesterEncode(input int64) (int64, error) {
	if input < 0 || input > integerMaxValue {
		return 0, fmt.Errorf("nrtsuggester: cannot encode value: %d", input)
	}
	return integerMaxValue - input, nil
}

// NRTSuggesterDecode decodes a weight from the FST.
// Mirrors NRTSuggester.decode(long).
func NRTSuggesterDecode(output int64) int64 {
	return integerMaxValue - output
}

// makePayload constructs the FST output bytes: surface form, PAYLOAD_SEP byte,
// and a vint-encoded docID. Mirrors
// NRTSuggester.PayLoadProcessor.make(BytesRef, int, int).
func makePayload(surface []byte, docID int, sep int) *util.BytesRef {
	out := store.NewByteArrayDataOutput(len(surface) + maxDocIDLenWithSep)
	_ = out.WriteBytes(surface)
	_ = out.WriteByte(byte(sep))
	_ = store.WriteVInt(out, int32(docID))
	pos := out.GetPosition()
	src := out.GetBytes()
	b := make([]byte, pos)
	copy(b, src[:pos])
	return &util.BytesRef{Bytes: b, Offset: 0, Length: pos}
}

// nrtEntry is a single (payload, weight) pair queued during term processing.
// Mirrors NRTSuggesterBuilder.Entry (implements Comparable<Entry>).
type nrtEntry struct {
	payload *util.BytesRef
	weight  int64
}

// nrtEntryHeap is a min-heap of nrtEntry ordered by weight ascending.
type nrtEntryHeap []nrtEntry

func (h nrtEntryHeap) Len() int            { return len(h) }
func (h nrtEntryHeap) Less(i, j int) bool  { return h[i].weight < h[j].weight }
func (h nrtEntryHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *nrtEntryHeap) Push(x interface{}) { *h = append(*h, x.(nrtEntry)) }
func (h *nrtEntryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// NRTSuggesterBuilder builds a weighted FST that NRTSuggester can search.
// Mirrors org.apache.lucene.search.suggest.document.NRTSuggesterBuilder.
type NRTSuggesterBuilder struct {
	outputs              *utilfst.PairOutputsImpl[int64, *util.BytesRef]
	compiler             *utilfst.FSTCompiler[*utilfst.Pair[int64, *util.BytesRef]]
	scratchInts          *util.IntsRefBuilder
	analyzed             []byte
	entries              nrtEntryHeap
	maxAnalyzedPerOutput int
}

// NewNRTSuggesterBuilder creates a new builder for NRTSuggester. Mirrors
// NRTSuggesterBuilder().
func NewNRTSuggesterBuilder() (*NRTSuggesterBuilder, error) {
	posOut := utilfst.PositiveIntOutputsSingleton()
	bsOut := utilfst.ByteSequenceOutputsSingleton()
	pairOut := utilfst.NewPairOutputs[int64, *util.BytesRef](posOut, bsOut)

	compiler := utilfst.NewFSTCompilerBuilder[*utilfst.Pair[int64, *util.BytesRef]](
		utilfst.InputTypeByte1, pairOut).Build()

	h := nrtEntryHeap{}
	heap.Init(&h)
	return &NRTSuggesterBuilder{
		outputs:     pairOut,
		compiler:    compiler,
		scratchInts: util.NewIntsRefBuilder(),
		entries:     h,
	}, nil
}

// StartTerm initialises processing for the given analysed form. Mirrors
// NRTSuggesterBuilder.startTerm(BytesRef).
func (b *NRTSuggesterBuilder) StartTerm(analyzed []byte) {
	b.analyzed = append(append([]byte(nil), analyzed...), byte(endByte))
}

// AddEntry records a (docID, surfaceForm, weight) tuple for the current term.
// Mirrors NRTSuggesterBuilder.addEntry(int, BytesRef, long).
func (b *NRTSuggesterBuilder) AddEntry(docID int, surfaceForm []byte, weight int64) error {
	encoded, err := NRTSuggesterEncode(weight)
	if err != nil {
		return err
	}
	payload := makePayload(surfaceForm, docID, payloadSep)
	heap.Push(&b.entries, nrtEntry{payload: payload, weight: encoded})
	return nil
}

// maxNumArcsForDedupByte mirrors NRTSuggesterBuilder.maxNumArcsForDedupByte.
func maxNumArcsForDedupByte(n int) int {
	maxArcs := int64(2)*int64(n) + 1
	if maxArcs >= 255 {
		return 255
	}
	if n > 5 {
		maxArcs *= int64(n)
	}
	if maxArcs >= 255 {
		return 255
	}
	return int(maxArcs)
}

// FinishTerm writes all queued entries for the current term into the FST
// compiler. Mirrors NRTSuggesterBuilder.finishTerm().
func (b *NRTSuggesterBuilder) FinishTerm() error {
	numArcs := 0
	numDedupBytes := 1
	// append one extra byte for the dedup suffix
	b.analyzed = append(b.analyzed, 0)
	entriesCount := b.entries.Len()

	for b.entries.Len() > 0 {
		entry := heap.Pop(&b.entries).(nrtEntry)
		if numArcs == maxNumArcsForDedupByte(numDedupBytes) {
			b.analyzed[len(b.analyzed)-1] = byte(numArcs)
			b.analyzed = append(b.analyzed, 0)
			numArcs = 0
			numDedupBytes++
		}
		b.analyzed[len(b.analyzed)-1] = byte(numArcs)
		numArcs++

		intsRef := utilfst.ToIntsRef(
			&util.BytesRef{Bytes: b.analyzed, Offset: 0, Length: len(b.analyzed)},
			b.scratchInts)
		pair := b.outputs.NewPair(entry.weight, entry.payload)
		if err := b.compiler.Add(intsRef, pair); err != nil {
			return err
		}
	}
	if entriesCount > b.maxAnalyzedPerOutput {
		b.maxAnalyzedPerOutput = entriesCount
	}
	// strip the dedup suffix bytes added above; restore to base analyzed length
	b.analyzed = b.analyzed[:len(b.analyzed)-numDedupBytes]
	return nil
}

// Store builds the FST and writes it to output. Returns false if the FST is
// empty (no terms were added). Mirrors NRTSuggesterBuilder.store(DataOutput).
func (b *NRTSuggesterBuilder) Store(output store.DataOutput) (bool, error) {
	metadata, err := b.compiler.Compile()
	if err != nil {
		return false, err
	}
	fst, err := utilfst.FromFSTReader[*utilfst.Pair[int64, *util.BytesRef]](metadata, b.compiler.GetFSTReader())
	if err != nil {
		return false, err
	}
	if fst == nil {
		return false, nil
	}
	if err := fst.Save(output, output); err != nil {
		return false, err
	}
	if b.maxAnalyzedPerOutput == 0 {
		// This should not happen since we only reach here with a non-nil FST,
		// but guard to avoid writing invalid metadata.
		return false, fmt.Errorf("nrtsuggester: maxAnalyzedPathsPerOutput must be > 0")
	}
	if err := store.WriteVInt(output, int32(b.maxAnalyzedPerOutput)); err != nil {
		return false, err
	}
	if err := store.WriteVInt(output, int32(endByte)); err != nil {
		return false, err
	}
	if err := store.WriteVInt(output, int32(payloadSep)); err != nil {
		return false, err
	}
	return true, nil
}
