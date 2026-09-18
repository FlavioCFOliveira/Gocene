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

// TestCompressingNormsFormat_Basic tests basic format creation
func TestCompressingNormsFormat_Basic(t *testing.T) {
	// Test default format
	format := DefaultCompressingNormsFormat()
	if format == nil {
		t.Fatal("DefaultCompressingNormsFormat returned nil")
	}

	if format.Name() != "CompressingNormsFormat" {
		t.Errorf("expected name 'CompressingNormsFormat', got '%s'", format.Name())
	}

	if format.CompressionMode() != CompressionModeLZ4Fast {
		t.Errorf("expected LZ4Fast mode, got %v", format.CompressionMode())
	}

	if format.ChunkSize() != 16*1024 {
		t.Errorf("expected chunk size 16KB, got %d", format.ChunkSize())
	}
}

// TestCompressingNormsFormat_CustomOptions tests format with custom options
func TestCompressingNormsFormat_CustomOptions(t *testing.T) {
	format := NewCompressingNormsFormat(CompressionModeDeflate, 8192)

	if format.CompressionMode() != CompressionModeDeflate {
		t.Errorf("expected DEFLATE mode, got %v", format.CompressionMode())
	}

	if format.ChunkSize() != 8192 {
		t.Errorf("expected chunk size 8192, got %d", format.ChunkSize())
	}
}

// TestCompressingNormsFormat_MinimumChunkSize tests minimum chunk size enforcement
func TestCompressingNormsFormat_MinimumChunkSize(t *testing.T) {
	// Pass chunk size below minimum
	format := NewCompressingNormsFormat(CompressionModeLZ4Fast, 512)

	// Should be clamped to 1024
	if format.ChunkSize() != 1024 {
		t.Errorf("expected chunk size clamped to 1024, got %d", format.ChunkSize())
	}
}

// TestCompressingNormsFormat_AllCompressionModes tests all compression modes
func TestCompressingNormsFormat_AllCompressionModes(t *testing.T) {
	modes := []CompressionMode{
		CompressionModeLZ4Fast,
		CompressionModeLZ4High,
		CompressionModeDeflate,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			format := NewCompressingNormsFormat(mode, 4096)

			if format.CompressionMode() != mode {
				t.Errorf("expected %v mode, got %v", mode, format.CompressionMode())
			}
		})
	}
}

// TestCompressingNormsConsumer_Basic tests basic consumer creation
func TestCompressingNormsConsumer_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentWriteState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	consumer, err := NewCompressingNormsConsumer(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingNormsConsumer failed: %v", err)
	}
	if consumer == nil {
		t.Fatal("NewCompressingNormsConsumer returned nil")
	}

	// Close should succeed
	err = consumer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Closing again should be safe
	err = consumer.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

// TestCompressingNormsProducer_Basic tests basic producer creation
func TestCompressingNormsProducer_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentReadState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	producer, err := NewCompressingNormsProducer(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingNormsProducer failed: %v", err)
	}
	if producer == nil {
		t.Fatal("NewCompressingNormsProducer returned nil")
	}

	// Close should succeed
	err = producer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Closing again should be safe
	err = producer.Close()
	if err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

// TestCompressingNormsProducer_CheckIntegrity tests integrity checking
func TestCompressingNormsProducer_CheckIntegrity(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentReadState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	producer, err := NewCompressingNormsProducer(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingNormsProducer failed: %v", err)
	}
	defer producer.Close()

	// CheckIntegrity should succeed
	err = producer.CheckIntegrity()
	if err != nil {
		t.Errorf("CheckIntegrity failed: %v", err)
	}
}

// TestCompressingNormsProducer_GetNorms tests getting norms for a field
func TestCompressingNormsProducer_GetNorms(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentReadState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	producer, err := NewCompressingNormsProducer(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingNormsProducer failed: %v", err)
	}
	defer producer.Close()

	fieldInfo := index.NewFieldInfo("test_field", 1, index.FieldInfoOptions{
		IndexOptions:  index.IndexOptionsDocs,
		DocValuesType: index.DocValuesTypeNumeric,
		Stored:        true,
	})

	norms, err := producer.GetNorms(fieldInfo)
	if err != nil {
		t.Fatalf("GetNorms failed: %v", err)
	}
	if norms == nil {
		t.Fatal("GetNorms returned nil")
	}
}

// TestEmptyNormsDocValues tests the empty norms doc values implementation
func TestEmptyNormsDocValues(t *testing.T) {
	empty := &emptyNormsDocValues{}

	if empty.DocID() != -1 {
		t.Errorf("expected DocID() to return -1, got %d", empty.DocID())
	}

	docID, err := empty.NextDoc()
	if err != nil {
		t.Errorf("NextDoc() returned error: %v", err)
	}
	if docID != -1 {
		t.Errorf("expected NextDoc() to return -1, got %d", docID)
	}

	advDocID, err := empty.Advance(10)
	if err != nil {
		t.Errorf("Advance() returned error: %v", err)
	}
	if advDocID != -1 {
		t.Errorf("expected Advance() to return -1, got %d", advDocID)
	}

	value, err := empty.LongValue()
	if err != nil {
		t.Errorf("LongValue() returned error: %v", err)
	}
	if value != 0 {
		t.Errorf("expected LongValue() to return 0, got %d", value)
	}

	if empty.Cost() != 0 {
		t.Errorf("expected Cost() to return 0, got %d", empty.Cost())
	}
}

// TestCompressingNormsConsumer_AddNormsField tests adding norms fields
func TestCompressingNormsConsumer_AddNormsField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentWriteState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	consumer, err := NewCompressingNormsConsumer(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingNormsConsumer failed: %v", err)
	}
	defer consumer.Close()

	fieldInfo := index.NewFieldInfo("test_field", 1, index.FieldInfoOptions{
		IndexOptions:  index.IndexOptionsDocs,
		DocValuesType: index.DocValuesTypeNumeric,
		Stored:        true,
	})

	// Create a simple norms producer
	normsData := []int64{10, 20, 30, 40, 50}
	producer := &testNormsProducer{data: normsData}

	err = consumer.AddNormsField(fieldInfo, producer)
	if err != nil {
		t.Errorf("AddNormsField failed: %v", err)
	}
}

// TestCompressingNormsConsumer_AddNormsFieldAfterClose tests adding norms after close
func TestCompressingNormsConsumer_AddNormsFieldAfterClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	segInfo := index.NewSegmentInfo("test_segment", 100, dir)

	state := &SegmentWriteState{
		SegmentInfo: segInfo,
		FieldInfos:  &index.FieldInfos{},
		Directory:   dir,
	}

	consumer, err := NewCompressingNormsConsumer(state, CompressionModeLZ4Fast, 16*1024)
	if err != nil {
		t.Fatalf("NewCompressingNormsConsumer failed: %v", err)
	}

	// Close the consumer
	err = consumer.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	fieldInfo := index.NewFieldInfo("test_field", 1, index.FieldInfoOptions{
		IndexOptions:  index.IndexOptionsDocs,
		DocValuesType: index.DocValuesTypeNumeric,
		Stored:        true,
	})

	producer := &testNormsProducer{data: []int64{10, 20, 30}}

	// Adding norms after close should fail
	err = consumer.AddNormsField(fieldInfo, producer)
	if err == nil {
		t.Error("expected AddNormsField to fail after Close, but it succeeded")
	}
}

// testNormsProducer is a test implementation of spi.NormsProducer over a
// dense []int64 (docID i carries data[i]).
type testNormsProducer struct {
	data []int64
}

func (p *testNormsProducer) GetNorms(*index.FieldInfo) (index.NumericDocValues, error) {
	return &testNormsValues{data: p.data, docID: -1, pos: -1}, nil
}

func (p *testNormsProducer) CheckIntegrity() error               { return nil }
func (p *testNormsProducer) GetMergeInstance() spi.NormsProducer { return p }
func (p *testNormsProducer) Close() error                        { return nil }

// testNormsValues is the NumericDocValues cursor testNormsProducer hands out.
type testNormsValues struct {
	data  []int64
	pos   int
	docID int
}

func (t *testNormsValues) DocID() int { return t.docID }

func (t *testNormsValues) NextDoc() (int, error) {
	t.pos++
	if t.pos >= len(t.data) {
		t.docID = index.NO_MORE_DOCS
		return t.docID, nil
	}
	t.docID = t.pos
	return t.docID, nil
}

func (t *testNormsValues) Advance(target int) (int, error) {
	if target >= len(t.data) {
		t.pos = len(t.data)
		t.docID = index.NO_MORE_DOCS
		return t.docID, nil
	}
	t.pos = target
	t.docID = target
	return t.docID, nil
}

func (t *testNormsValues) AdvanceExact(target int) (bool, error) {
	doc, err := t.Advance(target)
	if err != nil {
		return false, err
	}
	return doc == target, nil
}

func (t *testNormsValues) LongValue() (int64, error) { return t.data[t.pos], nil }

func (t *testNormsValues) Cost() int64 { return int64(len(t.data)) }

func (t *testNormsValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(t, upTo, bitSet, offset)
}

func (t *testNormsValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(t)
}
