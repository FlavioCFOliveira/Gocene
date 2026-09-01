package blockterms

import (
	"bytes"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/internal/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

const (
	TermsIndexExtension       = "tii"
	CodecName                 = "FixedGapTermsIndex"
	VersionStart              = 4
	VersionCurrent            = VersionStart
	BlockSize                 = 4096
	DefaultTermIndexInterval  = 32
)

type memoryDataOutput struct {
	buf *bytes.Buffer
}

func newMemoryDataOutput() *memoryDataOutput {
	return &memoryDataOutput{buf: new(bytes.Buffer)}
}

func (m *memoryDataOutput) WriteByte(b byte) error {
	return m.buf.WriteByte(b)
}

func (m *memoryDataOutput) WriteBytes(b []byte) error {
	_, err := m.buf.Write(b)
	return err
}

func (m *memoryDataOutput) WriteBytesN(b []byte, n int) error {
	_, err := m.buf.Write(b[:n])
	return err
}

func (m *memoryDataOutput) WriteInt(i int32) error {
	return store.WriteInt(m.buf, i)
}

func (m *memoryDataOutput) WriteVInt(i int32) error {
	return store.WriteVInt(m.buf, i)
}

func (m *memoryDataOutput) WriteLong(i int64) error {
	return store.WriteLong(m.buf, i)
}

func (m *memoryDataOutput) WriteVLong(i int64) error {
	return store.WriteVLong(m.buf, i)
}

func (m *memoryDataOutput) FilePointer() int64 {
	return int64(m.buf.Len())
}

func (m *memoryDataOutput) Bytes() []byte {
	return m.buf.Bytes()
}

func (m *memoryDataOutput) Reset() {
	m.buf.Reset()
}

// FixedGapTermsIndexWriter selects every Nth term as an index term, and holds
// term bytes (mostly) fully expanded in memory.
type FixedGapTermsIndexWriter struct {
	out               store.IndexOutput
	termIndexInterval int
	fields            []*simpleFieldWriter
}

func NewFixedGapTermsIndexWriter(state *store.SegmentWriteState, termIndexInterval int) (*FixedGapTermsIndexWriter, error) {
	if termIndexInterval <= 0 {
		return nil, fmt.Errorf("invalid termIndexInterval: %d", termIndexInterval)
	}

	indexFileName := store.IndexFileNamesSegmentFileName(
		state.SegmentInfo.Name, state.SegmentSuffix, TermsIndexExtension)
	out, err := state.Directory.CreateOutput(indexFileName, state.Context)
	if err != nil {
		return nil, err
	}

	success := false
	defer func() {
		if !success {
			out.Close()
		}
	}()

	if err := store.CodecUtilWriteIndexHeader(
		out, CodecName, VersionCurrent, state.SegmentInfo.ID, state.SegmentSuffix); err != nil {
		return nil, err
	}
	if err := out.WriteVInt(int32(termIndexInterval)); err != nil {
		return nil, err
	}
	if err := out.WriteVInt(int32(packed.VersionCurrent)); err != nil {
		return nil, err
	}
	if err := out.WriteVInt(int32(BlockSize)); err != nil {
		return nil, err
	}

	success = true
	return &FixedGapTermsIndexWriter{
		out:               out,
		termIndexInterval: termIndexInterval,
		fields:            make([]*simpleFieldWriter, 0),
	}, nil
}

func NewFixedGapTermsIndexWriterDefault(state *store.SegmentWriteState) (*FixedGapTermsIndexWriter, error) {
	return NewFixedGapTermsIndexWriter(state, DefaultTermIndexInterval)
}

func (w *FixedGapTermsIndexWriter) AddField(field *index.FieldInfo, termsFilePointer int64) (FieldWriter, error) {
	writer := &simpleFieldWriter{
		parent:            w,
		fieldInfo:         field,
		indexStart:        w.out.FilePointer(),
		termsStart:        termsFilePointer,
		termIndexInterval: w.termIndexInterval,
	}
	writer.init()
	w.fields = append(w.fields, writer)
	return writer, nil
}

func (w *FixedGapTermsIndexWriter) Close() error {
	if w.out == nil {
		return nil
	}

	dirStart := w.out.FilePointer()
	fieldCount := len(w.fields)

	nonNullFieldCount := 0
	for _, field := range w.fields {
		if field.numIndexTerms > 0 {
			nonNullFieldCount++
		}
	}

	if err := w.out.WriteVInt(int32(nonNullFieldCount)); err != nil {
		return err
	}

	for _, field := range w.fields {
		if field.numIndexTerms > 0 {
			if err := w.out.WriteVInt(int32(field.fieldInfo.Number)); err != nil {
				return err
			}
			if err := w.out.WriteVInt(int32(field.numIndexTerms)); err != nil {
				return err
			}
			if err := w.out.WriteVLong(field.termsStart); err != nil {
				return err
			}
			if err := w.out.WriteVLong(field.indexStart); err != nil {
				return err
			}
			if err := w.out.WriteVLong(field.packedIndexStart); err != nil {
				return err
			}
			if err := w.out.WriteVLong(field.packedOffsetsStart); err != nil {
				return err
			}
		}
	}

	if err := w.out.WriteLong(dirStart); err != nil {
		return err
	}

	if err := store.CodecUtilWriteFooter(w.out); err != nil {
		return err
	}

	return w.out.Close()
}

type simpleFieldWriter struct {
	parent            *FixedGapTermsIndexWriter
	fieldInfo         *index.FieldInfo
	numIndexTerms     int
	numTerms          int64
	indexStart        int64
	termsStart        int64
	packedIndexStart   int64
	packedOffsetsStart int64
	termIndexInterval  int

	offsetsBuffer   *memoryDataOutput
	termOffsets     *packed.MonotonicBlockPackedWriter
	currentOffset   int64

	addressBuffer   *memoryDataOutput
	termAddresses   *packed.MonotonicBlockPackedWriter

	lastTerm []byte
}

func (fw *simpleFieldWriter) init() {
	fw.offsetsBuffer = newMemoryDataOutput()
	fw.termOffsets, _ = packed.NewMonotonicBlockPackedWriter(fw.offsetsBuffer, BlockSize)

	fw.addressBuffer = newMemoryDataOutput()
	fw.termAddresses, _ = packed.NewMonotonicBlockPackedWriter(fw.addressBuffer, BlockSize)

	_ = fw.termOffsets.Add(0)
}

func (fw *simpleFieldWriter) CheckIndexTerm(text []byte, stats codecs.TermStats) (bool, error) {
	if fw.numTerms%int64(fw.termIndexInterval) == 0 {
		return true, nil
	} else {
		if (fw.numTerms+1)%int64(fw.termIndexInterval) == 0 {
			fw.lastTerm = append([]byte(nil), text...)
		}
		return false, nil
	}
}

func (fw *simpleFieldWriter) Add(text []byte, stats codecs.TermStats, termsFilePointer int64) error {
	var indexedTermLength int
	if fw.numIndexTerms == 0 {
		indexedTermLength = 0
	} else {
		p := util.NewBytesRef(fw.lastTerm)
		i := util.NewBytesRef(text)
		len, err := util.SortKeyLength(p, i)
		if err != nil {
			indexedTermLength = len(text)
		} else {
			indexedTermLength = len
		}
	}

	if err := fw.parent.out.WriteBytesN(text, indexedTermLength); err != nil {
		return err
	}

	if err := fw.termAddresses.Add(termsFilePointer - fw.termsStart); err != nil {
		return err
	}

	fw.currentOffset += int64(indexedTermLength)
	if err := fw.termOffsets.Add(fw.currentOffset); err != nil {
		return err
	}

	fw.lastTerm = append([]byte(nil), text...)
	fw.numIndexTerms++
	return nil
}

func (fw *simpleFieldWriter) Finish(termsFilePointer int64) error {
	fw.packedIndexStart = fw.parent.out.FilePointer()

	if err := fw.termAddresses.Finish(); err != nil {
		return err
	}
	if _, err := fw.parent.out.WriteBytes(fw.addressBuffer.Bytes()); err != nil {
		return err
	}

	fw.packedOffsetsStart = fw.parent.out.FilePointer()

	if err := fw.termOffsets.Finish(); err != nil {
		return err
	}
	if _, err := fw.parent.out.WriteBytes(fw.offsetsBuffer.Bytes()); err != nil {
		return err
	}

	return nil
}
