package blockterms

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/internal/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

const (
	FixedGapTermsIndexExtension       = "tii"
	FixedGapTermsIndexCodecName       = "FixedGapTermsIndex"
	FixedGapTermsIndexVersionStart    = 4
	FixedGapTermsIndexVersionCurrent  = FixedGapTermsIndexVersionStart
	BlockSize                         = 4096
	DefaultTermIndexInterval         = 32
)

type bufferPrimitiveWriter struct {
	buf *bytes.Buffer
}

func (b *bufferPrimitiveWriter) WriteByte(c byte) error {
	return b.buf.WriteByte(c)
}

func (b *bufferPrimitiveWriter) WriteBytes(p []byte, offset, length int) error {
	_, err := b.buf.Write(p[offset : offset+length])
	return err
}

type memoryDataOutput struct {
	*store.BaseDataOutput
	buf *bytes.Buffer
}

func newMemoryDataOutput() *memoryDataOutput {
	buf := new(bytes.Buffer)
	return &memoryDataOutput{
		BaseDataOutput: store.NewBaseDataOutput(&bufferPrimitiveWriter{buf: buf}),
		buf:            buf,
	}
}

func (m *memoryDataOutput) WriteBytes(b []byte, offset, length int) error {
	_, err := m.buf.Write(b[offset : offset+length])
	return err
}

func (m *memoryDataOutput) WriteBytesN(b []byte, n int) error {
	_, err := m.buf.Write(b[:n])
	return err
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

func NewFixedGapTermsIndexWriter(state *spi.SegmentWriteState, termIndexInterval int) (*FixedGapTermsIndexWriter, error) {
	if termIndexInterval <= 0 {
		return nil, fmt.Errorf("invalid termIndexInterval: %d", termIndexInterval)
	}

	indexFileName := store.IndexFileNamesSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, FixedGapTermsIndexExtension)
	out, err := state.Directory.CreateOutput(indexFileName, state.Context)
	if err != nil {
		return nil, err
	}
	storeOut, ok := out.(store.IndexOutput)
	if !ok {
		return nil, fmt.Errorf("output does not implement store.IndexOutput")
	}

	success := false
	defer func() {
		if !success {
			out.Close()
		}
	}()

	if err := store.CodecUtilWriteIndexHeader(
		storeOut, FixedGapTermsIndexCodecName, FixedGapTermsIndexVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, err
	}
	if err := storeOut.WriteVInt(int32(termIndexInterval)); err != nil {
		return nil, err
	}
	if err := storeOut.WriteVInt(int32(packed.VersionCurrent)); err != nil {
		return nil, err
	}
	if err := storeOut.WriteVInt(int32(BlockSize)); err != nil {
		return nil, err
	}

	success = true
	return &FixedGapTermsIndexWriter{
		out:               storeOut,
		termIndexInterval: termIndexInterval,
		fields:            make([]*simpleFieldWriter, 0),
	}, nil
}

func NewFixedGapTermsIndexWriterDefault(state *spi.SegmentWriteState) (*FixedGapTermsIndexWriter, error) {
	return NewFixedGapTermsIndexWriter(state, DefaultTermIndexInterval)
}

func (w *FixedGapTermsIndexWriter) AddField(field *index.FieldInfo, termsFilePointer int64) (FieldWriter, error) {
	writer := &simpleFieldWriter{
		parent:            w,
		fieldInfo:         field,
		indexStart:        w.out.GetFilePointer(),
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

	dirStart := w.out.GetFilePointer()
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
			if err := w.out.WriteVInt(int32(field.fieldInfo.Number())); err != nil {
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

	if err := store.WriteFooter(w.out); err != nil {
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
		length, err := util.SortKeyLength(p, i)
		if err != nil {
			indexedTermLength = len(text)
		} else {
			indexedTermLength = length
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
	fw.packedIndexStart = fw.parent.out.GetFilePointer()

	if err := fw.termAddresses.Finish(); err != nil {
		return err
	}
	if err := fw.parent.out.WriteBytes(fw.addressBuffer.Bytes(), 0, len(fw.addressBuffer.Bytes())); err != nil {
		return err
	}

	fw.packedOffsetsStart = fw.parent.out.GetFilePointer()

	if err := fw.termOffsets.Finish(); err != nil {
		return err
	}
	if err := fw.parent.out.WriteBytes(fw.offsetsBuffer.Bytes(), 0, len(fw.offsetsBuffer.Bytes())); err != nil {
		return err
	}

	return nil
}
