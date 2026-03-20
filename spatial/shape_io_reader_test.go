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

func TestNewShapeIOReader(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)

	if reader == nil {
		t.Fatal("expected non-nil reader")
	}
	if reader.GetFormat() != ShapeIOFormatWKB {
		t.Error("expected WKB format by default")
	}
}

func TestNewShapeIOReaderWithBuffer(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReaderWithBuffer(buf, 4096)

	if reader == nil {
		t.Fatal("expected non-nil reader")
	}
}

func TestNewShapeIOReaderWithFormat(t *testing.T) {
	buf := new(bytes.Buffer)

	// Test WKB format
	wkbReader := NewShapeIOReaderWithFormat(buf, ShapeIOFormatWKB)
	if wkbReader.GetFormat() != ShapeIOFormatWKB {
		t.Error("expected WKB format")
	}
	if _, ok := wkbReader.GetDecoder().(*JTSGeometryDecoder); !ok {
		t.Error("expected JTSGeometryDecoder for WKB format")
	}

	// Test Spatial4j format
	s4jReader := NewShapeIOReaderWithFormat(buf, ShapeIOFormatSpatial4j)
	if s4jReader.GetFormat() != ShapeIOFormatSpatial4j {
		t.Error("expected Spatial4j format")
	}
	if _, ok := s4jReader.GetDecoder().(*Spatial4jShapeDecoder); !ok {
		t.Error("expected Spatial4jShapeDecoder for Spatial4j format")
	}
}

func TestShapeIOReader_ReadShape(t *testing.T) {
	// Create encoded shape data
	// Format: length (4 bytes) + WKB data
	wkbData := createWKBPoint(10.5, 20.5)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(len(wkbData)))
	buf.Write(wkbData)

	reader := NewShapeIOReader(buf)
	shape, err := reader.ReadShape()
	if err != nil {
		t.Fatalf("read shape failed: %v", err)
	}

	point, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if point.X != 10.5 || point.Y != 20.5 {
		t.Errorf("expected Point(10.5, 20.5), got Point(%v, %v)", point.X, point.Y)
	}
}

func TestShapeIOReader_ReadShape_EOF(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)

	_, err := reader.ReadShape()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestShapeIOReader_ReadShape_InvalidLength(t *testing.T) {
	// Write length of 0 (invalid)
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(0))

	reader := NewShapeIOReader(buf)
	_, err := reader.ReadShape()
	if err == nil {
		t.Error("expected error for invalid length")
	}
}

func TestShapeIOReader_ReadShapeWithLength(t *testing.T) {
	wkbData := createWKBPoint(10.5, 20.5)

	buf := new(bytes.Buffer)
	buf.Write(wkbData)

	reader := NewShapeIOReader(buf)
	shape, err := reader.ReadShapeWithLength(len(wkbData))
	if err != nil {
		t.Fatalf("read shape failed: %v", err)
	}

	point, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if point.X != 10.5 || point.Y != 20.5 {
		t.Errorf("coordinates mismatch")
	}
}

func TestShapeIOReader_ReadShapeWithLength_Invalid(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)

	// Test negative length
	_, err := reader.ReadShapeWithLength(-1)
	if err == nil {
		t.Error("expected error for negative length")
	}

	// Test zero length
	_, err = reader.ReadShapeWithLength(0)
	if err == nil {
		t.Error("expected error for zero length")
	}
}

func TestShapeIOReader_ReadAllShapes(t *testing.T) {
	// Create multiple shapes
	buf := new(bytes.Buffer)

	// Write 3 shapes
	for i := 0; i < 3; i++ {
		wkbData := createWKBPoint(float64(i)*10, float64(i)*20)
		binary.Write(buf, binary.LittleEndian, uint32(len(wkbData)))
		buf.Write(wkbData)
	}

	reader := NewShapeIOReader(buf)
	shapes, err := reader.ReadAllShapes()
	if err != nil {
		t.Fatalf("read all shapes failed: %v", err)
	}

	if len(shapes) != 3 {
		t.Errorf("expected 3 shapes, got %d", len(shapes))
	}

	// Verify shapes
	for i, shape := range shapes {
		point, ok := shape.(Point)
		if !ok {
			t.Fatalf("expected Point at index %d, got %T", i, shape)
		}
		if point.X != float64(i)*10 || point.Y != float64(i)*20 {
			t.Errorf("shape %d has wrong coordinates", i)
		}
	}
}

func TestShapeIOReader_ReadAllShapes_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)

	shapes, err := reader.ReadAllShapes()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(shapes) != 0 {
		t.Errorf("expected 0 shapes, got %d", len(shapes))
	}
}

func TestShapeIOReader_SkipShape(t *testing.T) {
	// Create two shapes
	buf := new(bytes.Buffer)

	wkbData1 := createWKBPoint(10, 20)
	binary.Write(buf, binary.LittleEndian, uint32(len(wkbData1)))
	buf.Write(wkbData1)

	wkbData2 := createWKBPoint(30, 40)
	binary.Write(buf, binary.LittleEndian, uint32(len(wkbData2)))
	buf.Write(wkbData2)

	reader := NewShapeIOReader(buf)

	// Skip first shape
	skipped, err := reader.SkipShape()
	if err != nil {
		t.Fatalf("skip shape failed: %v", err)
	}

	expectedSkipped := int64(4 + len(wkbData1))
	if skipped != expectedSkipped {
		t.Errorf("expected to skip %d bytes, skipped %d", expectedSkipped, skipped)
	}

	// Read second shape
	shape, err := reader.ReadShape()
	if err != nil {
		t.Fatalf("read shape failed: %v", err)
	}

	point, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if point.X != 30 || point.Y != 40 {
		t.Errorf("expected Point(30, 40), got Point(%v, %v)", point.X, point.Y)
	}
}

func TestShapeIOReader_Reset(t *testing.T) {
	// Create first data
	buf1 := new(bytes.Buffer)
	wkbData := createWKBPoint(10, 20)
	binary.Write(buf1, binary.LittleEndian, uint32(len(wkbData)))
	buf1.Write(wkbData)

	reader := NewShapeIOReader(buf1)

	// Read first shape
	_, err := reader.ReadShape()
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}

	// Create second data
	buf2 := new(bytes.Buffer)
	wkbData2 := createWKBPoint(30, 40)
	binary.Write(buf2, binary.LittleEndian, uint32(len(wkbData2)))
	buf2.Write(wkbData2)

	// Reset reader
	reader.Reset(buf2)

	// Read from new source
	shape, err := reader.ReadShape()
	if err != nil {
		t.Fatalf("read after reset failed: %v", err)
	}

	point, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if point.X != 30 || point.Y != 40 {
		t.Errorf("coordinates mismatch after reset")
	}
}

func TestShapeIOReader_ReadRaw(t *testing.T) {
	buf := new(bytes.Buffer)
	buf.Write([]byte{1, 2, 3, 4, 5})

	reader := NewShapeIOReader(buf)

	data := make([]byte, 3)
	n, err := reader.ReadRaw(data)
	if err != nil {
		t.Fatalf("read raw failed: %v", err)
	}

	if n != 3 {
		t.Errorf("expected to read 3 bytes, read %d", n)
	}

	if data[0] != 1 || data[1] != 2 || data[2] != 3 {
		t.Error("data mismatch")
	}
}

func TestShapeIOReader_Close(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)

	err := reader.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Reader should be nil after close
	if reader.reader != nil {
		t.Error("reader should be nil after close")
	}
}

func TestShapeIOReader_SetDecoder(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)

	decoder := NewJTSGeometryDecoder()
	reader.SetDecoder(decoder)

	if reader.GetDecoder() != decoder {
		t.Error("decoder not set correctly")
	}
}

func TestShapeIOReader_SetFormat(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)

	// Set to Spatial4j format
	reader.SetFormat(ShapeIOFormatSpatial4j)
	if reader.GetFormat() != ShapeIOFormatSpatial4j {
		t.Error("format not set correctly")
	}
	if _, ok := reader.GetDecoder().(*Spatial4jShapeDecoder); !ok {
		t.Error("decoder not updated for Spatial4j format")
	}

	// Set back to WKB
	reader.SetFormat(ShapeIOFormatWKB)
	if reader.GetFormat() != ShapeIOFormatWKB {
		t.Error("format not set correctly")
	}
	if _, ok := reader.GetDecoder().(*JTSGeometryDecoder); !ok {
		t.Error("decoder not updated for WKB format")
	}
}

func TestShapeIOReader_SetByteOrder(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)

	// Default should be LittleEndian
	if reader.GetByteOrder() != binary.LittleEndian {
		t.Error("expected LittleEndian by default")
	}
}

// Test ShapeBatchReader
func TestNewShapeBatchReader(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)
	batchReader := NewShapeBatchReader(reader, 10)

	if batchReader == nil {
		t.Fatal("expected non-nil batch reader")
	}
	if batchReader.GetBatchSize() != 10 {
		t.Errorf("expected batch size 10, got %d", batchReader.GetBatchSize())
	}
}

func TestShapeBatchReader_ReadBatch(t *testing.T) {
	// Create 5 shapes
	buf := new(bytes.Buffer)
	for i := 0; i < 5; i++ {
		wkbData := createWKBPoint(float64(i), float64(i))
		binary.Write(buf, binary.LittleEndian, uint32(len(wkbData)))
		buf.Write(wkbData)
	}

	reader := NewShapeIOReader(buf)
	batchReader := NewShapeBatchReader(reader, 3)

	// Read first batch (should get 3 shapes)
	batch, err := batchReader.ReadBatch()
	if err != nil {
		t.Fatalf("first batch failed: %v", err)
	}
	if len(batch) != 3 {
		t.Errorf("expected 3 shapes in first batch, got %d", len(batch))
	}

	// Read second batch (should get 2 shapes and EOF)
	batch, err = batchReader.ReadBatch()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
	if len(batch) != 2 {
		t.Errorf("expected 2 shapes in second batch, got %d", len(batch))
	}
}

func TestShapeBatchReader_SetBatchSize(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewShapeIOReader(buf)
	batchReader := NewShapeBatchReader(reader, 10)

	batchReader.SetBatchSize(5)
	if batchReader.GetBatchSize() != 5 {
		t.Errorf("expected batch size 5, got %d", batchReader.GetBatchSize())
	}
}

// Test SeekableShapeIOReader
func TestNewSeekableShapeIOReader(t *testing.T) {
	buf := new(bytes.Buffer)
	reader := NewSeekableShapeIOReader(bytes.NewReader(buf.Bytes()))

	if reader == nil {
		t.Fatal("expected non-nil seekable reader")
	}
}

func TestSeekableShapeIOReader_Seek(t *testing.T) {
	// Create data with 2 shapes
	buf := new(bytes.Buffer)
	wkbData1 := createWKBPoint(10, 20)
	binary.Write(buf, binary.LittleEndian, uint32(len(wkbData1)))
	buf.Write(wkbData1)
	wkbData2 := createWKBPoint(30, 40)
	binary.Write(buf, binary.LittleEndian, uint32(len(wkbData2)))
	buf.Write(wkbData2)

	seeker := bytes.NewReader(buf.Bytes())
	reader := NewSeekableShapeIOReader(seeker)

	// Read first shape
	_, err := reader.ReadShape()
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}

	// Get current position
	pos, err := reader.GetPosition()
	if err != nil {
		t.Fatalf("get position failed: %v", err)
	}

	// Seek back to start
	_, err = reader.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatalf("seek failed: %v", err)
	}
	reader.bufReader.Reset(seeker)

	// Read first shape again
	shape, err := reader.ReadShape()
	if err != nil {
		t.Fatalf("second read failed: %v", err)
	}

	point, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if point.X != 10 || point.Y != 20 {
		t.Error("coordinates mismatch after seek")
	}

	_ = pos // Avoid unused variable warning
}

// Test ShapeIOReaderFactory
func TestNewShapeIOReaderFactory(t *testing.T) {
	factory := NewShapeIOReaderFactory()

	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
}

func TestShapeIOReaderFactory_CreateReader(t *testing.T) {
	factory := NewShapeIOReaderFactory()

	buf := new(bytes.Buffer)
	reader := factory.CreateReader(buf)

	if reader == nil {
		t.Fatal("expected non-nil reader")
	}
	if reader.GetFormat() != ShapeIOFormatWKB {
		t.Error("expected WKB format")
	}
}

func TestShapeIOReaderFactory_SetDefaults(t *testing.T) {
	factory := NewShapeIOReaderFactory()

	// Set custom defaults
	factory.SetDefaultFormat(ShapeIOFormatSpatial4j)
	factory.SetDefaultBufferSize(4096)
	factory.SetDefaultDecoder(NewJTSGeometryDecoder())

	buf := new(bytes.Buffer)
	reader := factory.CreateReader(buf)

	if reader.GetFormat() != ShapeIOFormatSpatial4j {
		t.Error("expected Spatial4j format")
	}
}

// Helper function to create WKB Point data
func createWKBPoint(x, y float64) []byte {
	buf := new(bytes.Buffer)
	buf.WriteByte(WKBNDR) // Little endian
	binary.Write(buf, binary.LittleEndian, WKBPoint)
	binary.Write(buf, binary.LittleEndian, x)
	binary.Write(buf, binary.LittleEndian, y)
	return buf.Bytes()
}

// Benchmarks
func BenchmarkShapeIOReader_ReadShape(b *testing.B) {
	// Create shape data
	buf := new(bytes.Buffer)
	wkbData := createWKBPoint(10.5, 20.5)

	for i := 0; i < 1000; i++ {
		binary.Write(buf, binary.LittleEndian, uint32(len(wkbData)))
		buf.Write(wkbData)
	}
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := NewShapeIOReader(bytes.NewReader(data))
		for {
			_, err := reader.ReadShape()
			if err != nil {
				break
			}
		}
	}
}

func BenchmarkShapeIOReader_SkipShape(b *testing.B) {
	// Create shape data
	buf := new(bytes.Buffer)
	wkbData := createWKBPoint(10.5, 20.5)

	for i := 0; i < 1000; i++ {
		binary.Write(buf, binary.LittleEndian, uint32(len(wkbData)))
		buf.Write(wkbData)
	}
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := NewShapeIOReader(bytes.NewReader(data))
		for {
			_, err := reader.SkipShape()
			if err != nil {
				break
			}
		}
	}
}
