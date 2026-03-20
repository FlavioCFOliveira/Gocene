// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestNewShapeIOWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	if writer == nil {
		t.Fatal("expected non-nil writer")
	}
	if writer.GetFormat() != ShapeIOFormatWKB {
		t.Error("expected WKB format by default")
	}
}

func TestNewShapeIOWriterWithBuffer(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriterWithBuffer(buf, 4096)

	if writer == nil {
		t.Fatal("expected non-nil writer")
	}
}

func TestNewShapeIOWriterWithFormat(t *testing.T) {
	buf := new(bytes.Buffer)

	// Test WKB format
	wkbWriter := NewShapeIOWriterWithFormat(buf, ShapeIOFormatWKB)
	if wkbWriter.GetFormat() != ShapeIOFormatWKB {
		t.Error("expected WKB format")
	}
	// Encoder should be wrapped in adapter
	if wkbWriter.GetEncoder() == nil {
		t.Error("expected non-nil encoder")
	}

	// Test Spatial4j format
	buf2 := new(bytes.Buffer)
	s4jWriter := NewShapeIOWriterWithFormat(buf2, ShapeIOFormatSpatial4j)
	if s4jWriter.GetFormat() != ShapeIOFormatSpatial4j {
		t.Error("expected Spatial4j format")
	}
	if s4jWriter.GetEncoder() == nil {
		t.Error("expected non-nil encoder")
	}
}

func TestShapeIOWriter_WriteShape(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	point := NewPoint(10.5, 20.5)
	n, err := writer.WriteShape(point)
	if err != nil {
		t.Fatalf("write shape failed: %v", err)
	}

	// Flush to ensure data is written
	writer.Flush()

	// Verify length was written
	var length uint32
	if err := binary.Read(bytes.NewReader(buf.Bytes()), binary.LittleEndian, &length); err != nil {
		t.Fatalf("failed to read length: %v", err)
	}

	if length == 0 {
		t.Error("expected non-zero length")
	}

	// Verify bytes written
	if n != int(4+length) {
		t.Errorf("expected %d bytes written, got %d", 4+length, n)
	}
}

func TestShapeIOWriter_WriteShape_Nil(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	_, err := writer.WriteShape(nil)
	if err == nil {
		t.Error("expected error for nil shape")
	}
}

func TestShapeIOWriter_WriteShapeData(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	wkbData := createWKBPoint(10.5, 20.5)
	n, err := writer.WriteShapeData(wkbData)
	if err != nil {
		t.Fatalf("write shape data failed: %v", err)
	}

	writer.Flush()

	// Verify
	if n != 4+len(wkbData) {
		t.Errorf("expected %d bytes, got %d", 4+len(wkbData), n)
	}
}

func TestShapeIOWriter_WriteShapeData_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	_, err := writer.WriteShapeData([]byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestShapeIOWriter_WriteRaw(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	data := []byte{1, 2, 3, 4, 5}
	n, err := writer.WriteRaw(data)
	if err != nil {
		t.Fatalf("write raw failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("expected to write %d bytes, wrote %d", len(data), n)
	}

	writer.Flush()

	// Verify data was written
	if !bytes.Equal(buf.Bytes(), data) {
		t.Error("data mismatch")
	}
}

func TestShapeIOWriter_WriteShapes(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	shapes := []Shape{
		NewPoint(0, 0),
		NewPoint(10, 20),
		NewPoint(30, 40),
	}

	n, err := writer.WriteShapes(shapes)
	if err != nil {
		t.Fatalf("write shapes failed: %v", err)
	}

	writer.Flush()

	if n == 0 {
		t.Error("expected non-zero bytes written")
	}

	// Should have written 3 shapes
	if buf.Len() == 0 {
		t.Error("expected data in buffer")
	}
}

func TestShapeIOWriter_Flush(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriterWithBuffer(buf, 1024) // Large buffer

	// Write without flushing
	writer.WriteShape(NewPoint(10, 20))

	// Buffer should be empty (data buffered)
	if buf.Len() != 0 {
		t.Error("expected empty buffer before flush")
	}

	// Flush
	err := writer.Flush()
	if err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	// Now buffer should have data
	if buf.Len() == 0 {
		t.Error("expected data in buffer after flush")
	}
}

func TestShapeIOWriter_Reset(t *testing.T) {
	buf1 := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf1)

	// Write first shape
	writer.WriteShape(NewPoint(10, 20))
	writer.Flush()

	// Verify first buffer has data
	if buf1.Len() == 0 {
		t.Error("expected data in first buffer")
	}

	// Reset with new buffer
	buf2 := new(bytes.Buffer)
	writer.Reset(buf2)

	// Verify bytes written reset
	if writer.GetBytesWritten() != 0 {
		t.Error("expected bytes written to be reset")
	}

	// Write to new buffer
	writer.WriteShape(NewPoint(30, 40))
	writer.Flush()

	// Verify second buffer has data
	if buf2.Len() == 0 {
		t.Error("expected data in second buffer")
	}
}

func TestShapeIOWriter_GetBytesWritten(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	// Initial should be 0
	if writer.GetBytesWritten() != 0 {
		t.Error("expected 0 bytes written initially")
	}

	// Write shape
	writer.WriteShape(NewPoint(10, 20))

	// Should track bytes
	if writer.GetBytesWritten() == 0 {
		t.Error("expected non-zero bytes written")
	}
}

func TestShapeIOWriter_Close(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	// Write some data
	writer.WriteShape(NewPoint(10, 20))

	err := writer.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Reader should be nil after close
	if writer.writer != nil {
		t.Error("writer should be nil after close")
	}

	// Data should be flushed
	if buf.Len() == 0 {
		t.Error("expected data to be flushed on close")
	}
}

func TestShapeIOWriter_SetEncoder(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	encoder := NewShapeEncoderFromSerializer(NewJTSGeometrySerializer())
	writer.SetEncoder(encoder)

	if writer.GetEncoder() != encoder {
		t.Error("encoder not set correctly")
	}
}

func TestShapeIOWriter_SetFormat(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	// Set to Spatial4j format
	writer.SetFormat(ShapeIOFormatSpatial4j)
	if writer.GetFormat() != ShapeIOFormatSpatial4j {
		t.Error("format not set correctly")
	}
	// Encoder should be spatial4jEncoder
	if _, ok := writer.GetEncoder().(*spatial4jEncoder); !ok {
		t.Error("encoder not updated for Spatial4j format")
	}

	// Set back to WKB
	writer.SetFormat(ShapeIOFormatWKB)
	if writer.GetFormat() != ShapeIOFormatWKB {
		t.Error("format not set correctly")
	}
	// Encoder should be shapeEncoderAdapter
	if _, ok := writer.GetEncoder().(*shapeEncoderAdapter); !ok {
		t.Error("encoder not updated for WKB format")
	}
}

func TestShapeIOWriter_SetByteOrder(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)

	// Default should be LittleEndian
	if writer.GetByteOrder() != binary.LittleEndian {
		t.Error("expected LittleEndian by default")
	}
}

// Test ShapeBatchWriter
func TestNewShapeBatchWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)
	batchWriter := NewShapeBatchWriter(writer, 10)

	if batchWriter == nil {
		t.Fatal("expected non-nil batch writer")
	}
	if batchWriter.GetBatchSize() != 10 {
		t.Errorf("expected batch size 10, got %d", batchWriter.GetBatchSize())
	}
}

func TestShapeBatchWriter_WriteShape(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)
	batchWriter := NewShapeBatchWriter(writer, 3)

	// Write 2 shapes (should not flush)
	_, err := batchWriter.WriteShape(NewPoint(0, 0))
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	_, err = batchWriter.WriteShape(NewPoint(10, 10))
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	// Should be buffered
	if batchWriter.GetBufferedCount() != 2 {
		t.Errorf("expected 2 buffered shapes, got %d", batchWriter.GetBufferedCount())
	}

	// Write third shape (should flush)
	_, err = batchWriter.WriteShape(NewPoint(20, 20))
	if err != nil {
		t.Fatalf("third write failed: %v", err)
	}

	// Should have flushed
	if batchWriter.GetBufferedCount() != 0 {
		t.Errorf("expected 0 buffered shapes after flush, got %d", batchWriter.GetBufferedCount())
	}
}

func TestShapeBatchWriter_Flush(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)
	batchWriter := NewShapeBatchWriter(writer, 10)

	// Write some shapes
	batchWriter.WriteShape(NewPoint(0, 0))
	batchWriter.WriteShape(NewPoint(10, 10))

	// Flush manually
	n, err := batchWriter.Flush()
	if err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	if n == 0 {
		t.Error("expected non-zero bytes written")
	}

	// Buffer should be empty
	if batchWriter.GetBufferedCount() != 0 {
		t.Error("expected empty buffer after flush")
	}
}

func TestShapeBatchWriter_Close(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)
	batchWriter := NewShapeBatchWriter(writer, 10)

	// Write some shapes
	batchWriter.WriteShape(NewPoint(0, 0))
	batchWriter.WriteShape(NewPoint(10, 10))

	// Close should flush the batch
	err := batchWriter.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Flush the underlying writer to ensure data is written
	writer.Flush()

	// Data should be in buffer
	if buf.Len() == 0 {
		t.Error("expected data in buffer after close")
	}
}

func TestShapeBatchWriter_SetBatchSize(t *testing.T) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)
	batchWriter := NewShapeBatchWriter(writer, 10)

	batchWriter.SetBatchSize(5)
	if batchWriter.GetBatchSize() != 5 {
		t.Errorf("expected batch size 5, got %d", batchWriter.GetBatchSize())
	}
}

// Test ShapeIOWriterFactory
func TestNewShapeIOWriterFactory(t *testing.T) {
	factory := NewShapeIOWriterFactory()

	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
}

func TestShapeIOWriterFactory_CreateWriter(t *testing.T) {
	factory := NewShapeIOWriterFactory()

	buf := new(bytes.Buffer)
	writer := factory.CreateWriter(buf)

	if writer == nil {
		t.Fatal("expected non-nil writer")
	}
	if writer.GetFormat() != ShapeIOFormatWKB {
		t.Error("expected WKB format")
	}
}

func TestShapeIOWriterFactory_SetDefaults(t *testing.T) {
	factory := NewShapeIOWriterFactory()

	// Set custom defaults
	factory.SetDefaultFormat(ShapeIOFormatSpatial4j)
	factory.SetDefaultBufferSize(4096)
	factory.SetDefaultEncoderFromSpatial4j(NewSpatial4jShapeDecoder(NewSpatialContext()))

	buf := new(bytes.Buffer)
	writer := factory.CreateWriter(buf)

	if writer.GetFormat() != ShapeIOFormatSpatial4j {
		t.Error("expected Spatial4j format")
	}
}

// Integration test - round trip with reader
func TestShapeIOWriter_RoundTrip(t *testing.T) {
	buf := new(bytes.Buffer)

	// Write shapes
	writer := NewShapeIOWriter(buf)
	shapes := []Shape{
		NewPoint(10.5, 20.5),
		NewPoint(30.5, 40.5),
	}
	writer.WriteShapes(shapes)
	writer.Close()

	// Read shapes back
	reader := NewShapeIOReader(bytes.NewReader(buf.Bytes()))
	readShapes, err := reader.ReadAllShapes()
	if err != nil && err != io.EOF {
		t.Fatalf("read failed: %v", err)
	}

	if len(readShapes) != 2 {
		t.Errorf("expected 2 shapes, got %d", len(readShapes))
	}

	// Verify first shape
	point1, ok := readShapes[0].(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", readShapes[0])
	}
	if point1.X != 10.5 || point1.Y != 20.5 {
		t.Errorf("first point mismatch")
	}

	// Verify second shape
	point2, ok := readShapes[1].(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", readShapes[1])
	}
	if point2.X != 30.5 || point2.Y != 40.5 {
		t.Errorf("second point mismatch")
	}
}

// Benchmarks
func BenchmarkShapeIOWriter_WriteShape(b *testing.B) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)
	point := NewPoint(10.5, 20.5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		writer.Reset(buf)
		for j := 0; j < 1000; j++ {
			writer.WriteShape(point)
		}
		writer.Flush()
	}
}

func BenchmarkShapeIOWriter_BatchWrite(b *testing.B) {
	buf := new(bytes.Buffer)
	writer := NewShapeIOWriter(buf)
	batchWriter := NewShapeBatchWriter(writer, 100)
	point := NewPoint(10.5, 20.5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		writer.Reset(buf)
		for j := 0; j < 1000; j++ {
			batchWriter.WriteShape(point)
		}
		batchWriter.Close()
	}
}
