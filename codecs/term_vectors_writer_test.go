// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type mockTermVectorsWriter struct {
	docsWritten    int
	fieldsWritten  int
	termsWritten   int
	positionsAdded int
	finished       bool
	closed         bool
}

func (m *mockTermVectorsWriter) StartDocument(numFields int) error {
	m.docsWritten++
	return nil
}

func (m *mockTermVectorsWriter) FinishDocument() error {
	return nil
}

func (m *mockTermVectorsWriter) StartField(fieldInfo *spi.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	m.fieldsWritten++
	return nil
}

func (m *mockTermVectorsWriter) FinishField() error {
	return nil
}

func (m *mockTermVectorsWriter) StartTerm(term []byte, freq int) error {
	m.termsWritten++
	return nil
}

func (m *mockTermVectorsWriter) FinishTerm() error {
	return nil
}

func (m *mockTermVectorsWriter) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	m.positionsAdded++
	return nil
}

func (m *mockTermVectorsWriter) Finish(numDocs int) error {
	m.finished = true
	return nil
}

func (m *mockTermVectorsWriter) Close() error {
	m.closed = true
	return nil
}

func TestAddProx(t *testing.T) {
	writer := &mockTermVectorsWriter{}
	helper := &TermVectorsWriterHelper{}

	vint := func(v int32) []byte {
		var res []byte
		for v >= 0x80 {
			res = append(res, byte(v)|0x80)
			v >>= 7
		}
		res = append(res, byte(v))
		return res
	}

	posData := append(vint(0), vint(1)...)
	posData = append(posData, vint(3)...)
	posData = append(posData, 'a', 'b', 'c')

	offData := append(vint(10), vint(5)...)
	offData = append(offData, vint(2), vint(3)...)

	posIn := store.NewByteArrayDataInput(posData)
	offIn := store.NewByteArrayDataInput(offData)

	err := helper.AddProx(writer, 2, posIn, offIn)
	if err != nil {
		t.Fatalf("AddProx failed: %v", err)
	}

	if writer.positionsAdded != 2 {
		t.Errorf("expected 2 positions, got %d", writer.positionsAdded)
	}
}

func TestMerge(t *testing.T) {
	writer := &mockTermVectorsWriter{}
	helper := &TermVectorsWriterHelper{}

	mergeState := &index.MergeState{
		TermVectorsReaders: []*spi.TermVectorsReader{nil},
		DocMaps:            []index.DocMap{nil},
		MaxDocs:            []int{0},
		NeedsIndexSort:     false,
	}

	count, err := helper.Merge(writer, mergeState)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 docs, got %d", count)
	}
	if !writer.finished {
		t.Error("expected writer.Finish to be called")
	}
}

func TestAddAllDocVectors(t *testing.T) {
	writer := &mockTermVectorsWriter{}
	helper := &TermVectorsWriterHelper{}
	mergeState := &index.MergeState{
		MergeFieldInfos: &index.FieldInfos{},
	}

	err := helper.addAllDocVectors(writer, nil, mergeState)
	if err != nil {
		t.Fatalf("addAllDocVectors failed: %v", err)
	}
	if writer.docsWritten != 1 {
		t.Errorf("expected 1 doc written, got %d", writer.docsWritten)
	}
}
