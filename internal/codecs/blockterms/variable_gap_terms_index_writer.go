package blockterms

import (
	"github.com/FlavioCFOliveira/Gocene/internal/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

const (
	VariableGapTermsIndexExtension = "tiv"
	TermsMetaExtension             = "tmv"
	MetaCodecName                   = "VariableGapTermsMeta"
	VariableGapTermsIndexCodecName  = "VariableGapTermsIndex"
	VariableGapTermsIndexVersionStart = 4
	VariableGapTermsIndexVersionCurrent = VariableGapTermsIndexVersionStart
)

// IndexTermSelector defines a policy for selecting which terms should be indexed.
type IndexTermSelector interface {
	IsIndexTerm(term []byte, stats codecs.TermStats) bool
	NewField(fieldInfo *index.FieldInfo)
}

// EveryNTermSelector indexes every Nth term.
type EveryNTermSelector struct {
	count    int
	interval int
}

func NewEveryNTermSelector(interval int) *EveryNTermSelector {
	return &EveryNTermSelector{
		count:    interval,
		interval: interval,
	}
}

func (s *EveryNTermSelector) IsIndexTerm(term []byte, stats codecs.TermStats) bool {
	if s.count >= s.interval {
		s.count = 1
		return true
	}
	s.count++
	return false
}

func (s *EveryNTermSelector) NewField(fieldInfo *index.FieldInfo) {
	s.count = s.interval
}

// EveryNOrDocFreqTermSelector indexes terms based on doc frequency or interval.
type EveryNOrDocFreqTermSelector struct {
	count        int
	docFreqThresh int32
	interval     int
}

func NewEveryNOrDocFreqTermSelector(docFreqThresh int32, interval int) *EveryNOrDocFreqTermSelector {
	return &EveryNOrDocFreqTermSelector{
		count:        interval,
		docFreqThresh: docFreqThresh,
		interval:     interval,
	}
}

func (s *EveryNOrDocFreqTermSelector) IsIndexTerm(term []byte, stats codecs.TermStats) bool {
	if stats.DocFreq >= s.docFreqThresh || s.count >= s.interval {
		s.count = 1
		return true
	}
	s.count++
	return false
}

func (s *EveryNOrDocFreqTermSelector) NewField(fieldInfo *index.FieldInfo) {
	s.count = s.interval
}

type VariableGapTermsIndexWriter struct {
	metaOut store.IndexOutput
	out     store.IndexOutput
	policy  IndexTermSelector
}

func NewVariableGapTermsIndexWriter(state *spi.SegmentWriteState, policy IndexTermSelector) (*VariableGapTermsIndexWriter, error) {
	indexFileName := store.IndexFileNamesSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, VariableGapTermsIndexExtension)
	metaFileName := store.IndexFileNamesSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, TermsMetaExtension)

	metaOut, err := state.Directory.CreateOutput(metaFileName, state.Context)
	if err != nil {
		return nil, err
	}

	out, err := state.Directory.CreateOutput(indexFileName, state.Context)
	if err != nil {
		metaOut.Close()
		return nil, err
	}

	if err := store.CodecUtilWriteIndexHeader(metaOut, MetaCodecName, VariableGapTermsIndexVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		metaOut.Close()
		out.Close()
		return nil, err
	}
	if err := store.CodecUtilWriteIndexHeader(out, VariableGapTermsIndexCodecName, VariableGapTermsIndexVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		metaOut.Close()
		out.Close()
		return nil, err
	}

	return &VariableGapTermsIndexWriter{
		metaOut: metaOut,
		out:     out,
		policy:  policy,
	}, nil
}

func (w *VariableGapTermsIndexWriter) AddField(field *index.FieldInfo, termsFilePointer int64) (FieldWriter, error) {
	w.policy.NewField(field)
	return &fstFieldWriter{
		fieldInfo:         field,
		startTermsFilePointer: termsFilePointer,
		writer:            w,
	}, nil
}

func (w *VariableGapTermsIndexWriter) indexedTermPrefixLength(priorTerm, indexedTerm []byte) int {
	limit := len(priorTerm)
	if len(indexedTerm) < limit {
		limit = len(indexedTerm)
	}
	for i := 0; i < limit; i++ {
		if priorTerm[i] != indexedTerm[i] {
			return i + 1
		}
	}
	if len(priorTerm)+1 < len(indexedTerm) {
		return len(priorTerm) + 1
	}
	return len(indexedTerm)
}

type fstFieldWriter struct {
	fieldInfo             *index.FieldInfo
	startTermsFilePointer int64
	writer                *VariableGapTermsIndexWriter
	compiler              *fst.FSTCompiler[int64]
	lastTerm              []byte
	first                 bool
}

func newFstFieldWriter(field *index.FieldInfo, termsFilePointer int64, writer *VariableGapTermsIndexWriter) *fstFieldWriter {
	compiler := fst.NewFSTCompilerBuilder[int64](fst.InputTypeByte1, fst.PositiveIntOutputs()).Build()
	compiler.Add(util.NewIntsRef([]int{}), termsFilePointer)

	return &fstFieldWriter{
		fieldInfo:             field,
		startTermsFilePointer: termsFilePointer,
		writer:                writer,
		compiler:              compiler,
		first:                 true,
	}
}

func (fw *fstFieldWriter) CheckIndexTerm(text []byte, stats codecs.TermStats) (bool, error) {
	if fw.writer.policy.IsIndexTerm(text, stats) || fw.first {
		fw.first = false
		return true, nil
	}
	fw.lastTerm = text
	return false, nil
}

func (fw *fstFieldWriter) Add(text []byte, stats codecs.TermStats, termsFilePointer int64) error {
	if len(text) == 0 {
		return nil
	}

	originalLength := len(text)
	prefixLen := fw.writer.indexedTermPrefixLength(fw.lastTerm, text)
	trimmedText := text[:prefixLen]

	// Convert bytes to ints for FST input
	ints := make([]int, len(trimmedText))
	for i, b := range trimmedText {
		ints[i] = int(b)
	}

	if err := fw.compiler.Add(util.NewIntsRef(ints), termsFilePointer); err != nil {
		return err
	}
	fw.lastTerm = text
	_ = originalLength
	return nil
}

func (fw *fstFieldWriter) Finish(termsFilePointer int64) error {
	f, err := fw.compiler.Compile()
	if err != nil {
		return err
	}
	if f != nil {
		if err := fw.writer.metaOut.WriteInt(int32(fw.fieldInfo.Number())); err != nil {
			return err
		}
		if err := fw.writer.metaOut.WriteVLong(fw.writer.out.GetFilePointer()); err != nil {
			return err
		}
		if err := f.Save(fw.writer.out); err != nil {
			return err
		}
	}
	return nil
}

func (w *VariableGapTermsIndexWriter) Close() error {
	var err error
	if w.metaOut != nil {
		w.metaOut.WriteInt(-1)
		store.CodecUtilWriteFooter(w.metaOut)
	}
	if w.out != nil {
		store.CodecUtilWriteFooter(w.out)
	}
	if w.metaOut != nil {
		err = w.metaOut.Close()
	}
	if w.out != nil {
		if cerr := w.out.Close(); cerr != nil {
			err = cerr
		}
	}
	return err
}
