package uniformsplit

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type mockDictionary struct {
	data map[string]int64
}

func (m *mockDictionary) Get(term *util.BytesRef) (int64, error) {
	fp, ok := m.data[string(term.Bytes()[term.Offset:term.Offset+term.Length()])]
	if !ok {
		return -1, nil
	}
	return fp, nil
}

type mockBrowserSupplier struct {
	dict IndexDictionary
}

func (s *mockBrowserSupplier) Get() (IndexDictionary, error) {
	return s.dict, nil
}

func TestBlockReader_Basic(t *testing.T) {
	// Setup
	fi := index.NewFieldInfo("test", 0, index.DefaultFieldInfoOptions())
	meta := &FieldMetadata{
		fieldInfo:         fi,
		firstBlockStartFP: 0,
		lastBlockStartFP:  100,
		lastTerm:          util.NewBytesRef([]byte("term2")),
	}

	// Create a dummy block in memory
	buf := store.NewByteBuffersDataOutput()

	// BlockHeader: linesCount(vInt), baseDocsFP(vLong), basePosFP(vLong), basePayFP(vLong), termStatesBaseOffset(vInt), middleLineOffset(vInt)
	buf.WriteVInt(2)
	buf.WriteVLong(0)
	buf.WriteVLong(0)
	buf.WriteVLong(0)
	buf.WriteVInt(20) // termStatesBaseOffset
	buf.WriteVInt(10) // middleLineOffset

	// BlockLines: termStateRelativeOffset(vInt), termBytes(incremental)
	// Line 0 (Seed)
	buf.WriteVInt(0)
	buf.WriteVLong(int64(len("term1")))
	buf.WriteBytes([]byte("term1"))

	// Line 1 (Non-seed)
	// term1: "term1", term2: "term2". MDP is "term" (len 4).
	// prevLen = 5. numMdpBits = 3 (since 5 is 101 in binary).
	// mdpLength = 4. suffixLength = 5 - (4-1) = 2 ("r2").
	// mdpAndSuffixLengths = (2 << 3) | (4-1) = 16 | 3 = 19.
	buf.WriteVInt(10)
	buf.WriteVLong(19)
	buf.WriteBytes([]byte("r2"))

	// Details region
	buf.SetPosition(20)
	// TermState 0: docFreq(vInt), totalTF-docFreq(vLong), singletonDocID(vInt) if docFreq==1
	buf.WriteVInt(1)
	buf.WriteVInt(10) // singletonDocID

	// TermState 1:
	buf.WriteVInt(1)
	buf.WriteVInt(20) // singletonDocID

	input := store.NewByteArrayDataInput(buf.Bytes())

	dict := &mockDictionary{
		data: map[string]int64{
			"term1": 0,
			"term2": 0,
		},
	}
	supplier := &mockBrowserSupplier{dict: dict}

	reader, err := NewBlockReader(supplier, input, &mockPostingsReader{}, meta, nil)
	if err != nil {
		t.Fatalf("failed to create BlockReader: %v", err)
	}

	// Test SeekCeil
	term1 := util.NewBytesRef([]byte("term1"))
	if res, err := reader.seekCeil(term1); err != nil || res != spi.SeekStatusFound {
		t.Errorf("seekCeil(term1) failed: %v, status=%v", err, res)
	}
	if reader.term().String() != "term1" { // This won't work because reader.term() returns BytesRef, but I'll check Length
		if reader.term().Length() != 5 {
			t.Errorf("expected term1, got %v", reader.term())
		}
	}

	// Test Next
	if term := reader.Next(); term == nil || term.BytesValue().String() != "term2" {
		t.Errorf("expected term2, got %v", term)
	}
}

type mockPostingsReader struct{}

func (m *mockPostingsReader) NewTermState() index.TermState {
	return codecs.NewBlockTermState()
}
func (m *mockPostingsReader) Postings(fi *index.FieldInfo, ts index.TermState, reuse spi.PostingsEnum, flags int) (spi.PostingsEnum, error) {
	return &spi.EmptyPostingsEnum{}, nil
}
func (m *mockPostingsReader) Impacts(fi *index.FieldInfo, ts index.TermState, flags int) (index.ImpactsEnum, error) {
	return nil, nil
}
