// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestJTSGeometryDecoder_New(t *testing.T) {
	decoder := NewJTSGeometryDecoder()
	if decoder == nil {
		t.Fatal("expected non-nil decoder")
	}
	if decoder.GetCalculator() == nil {
		t.Error("expected non-nil calculator")
	}
}

func TestJTSGeometryDecoder_NewWithCalculator(t *testing.T) {
	calc := &CartesianCalculator{}
	decoder := NewJTSGeometryDecoderWithCalculator(calc)
	if decoder.GetCalculator() != calc {
		t.Error("expected custom calculator")
	}
}

func TestJTSGeometryDecoder_DecodePoint(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	tests := []struct {
		name     string
		expected Point
	}{
		{"origin", NewPoint(0, 0)},
		{"simple", NewPoint(10.5, 20.5)},
		{"negative", NewPoint(-10.5, -20.5)},
		{"mixed", NewPoint(-10.5, 20.5)},
		{"large", NewPoint(123456789.123, -987654321.456)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create WKB data manually
			buf := new(bytes.Buffer)
			buf.WriteByte(WKBNDR) // Little endian
			binary.Write(buf, binary.LittleEndian, WKBPoint)
			binary.Write(buf, binary.LittleEndian, tt.expected.X)
			binary.Write(buf, binary.LittleEndian, tt.expected.Y)

			// Decode
			shape, err := decoder.Decode(buf.Bytes())
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			// Check type
			point, ok := shape.(Point)
			if !ok {
				t.Fatalf("expected Point, got %T", shape)
			}

			// Compare
			if point.X != tt.expected.X || point.Y != tt.expected.Y {
				t.Errorf("expected Point(%v, %v), got Point(%v, %v)",
					tt.expected.X, tt.expected.Y, point.X, point.Y)
			}
		})
	}
}

func TestJTSGeometryDecoder_DecodePoint_BigEndian(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	point := NewPoint(10.5, 20.5)

	// Create WKB data in BigEndian
	buf := new(bytes.Buffer)
	buf.WriteByte(WKBXDR) // Big endian
	binary.Write(buf, binary.BigEndian, WKBPoint)
	binary.Write(buf, binary.BigEndian, point.X)
	binary.Write(buf, binary.BigEndian, point.Y)

	// Decode
	shape, err := decoder.Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Check
	p, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if p.X != point.X || p.Y != point.Y {
		t.Errorf("coordinates mismatch: expected (%v, %v), got (%v, %v)",
			point.X, point.Y, p.X, p.Y)
	}
}

func TestJTSGeometryDecoder_DecodeRectangle(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	tests := []struct {
		name     string
		expected *Rectangle
	}{
		{"simple", NewRectangle(0, 0, 10, 20)},
		{"negative", NewRectangle(-100, -50, -10, -20)},
		{"coordinates", NewRectangle(-180, -90, 180, 90)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create WKB polygon data
			buf := new(bytes.Buffer)
			buf.WriteByte(WKBNDR) // Little endian
			binary.Write(buf, binary.LittleEndian, WKBPolygon)
			binary.Write(buf, binary.LittleEndian, uint32(1)) // 1 ring
			binary.Write(buf, binary.LittleEndian, uint32(5)) // 5 points

			// Write points (SW, SE, NE, NW, SW)
			points := [5]struct{ x, y float64 }{
				{tt.expected.MinX, tt.expected.MinY},
				{tt.expected.MaxX, tt.expected.MinY},
				{tt.expected.MaxX, tt.expected.MaxY},
				{tt.expected.MinX, tt.expected.MaxY},
				{tt.expected.MinX, tt.expected.MinY},
			}
			for _, pt := range points {
				binary.Write(buf, binary.LittleEndian, pt.x)
				binary.Write(buf, binary.LittleEndian, pt.y)
			}

			// Decode
			shape, err := decoder.Decode(buf.Bytes())
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			// Check type
			rect, ok := shape.(*Rectangle)
			if !ok {
				t.Fatalf("expected *Rectangle, got %T", shape)
			}

			// Compare
			if rect.MinX != tt.expected.MinX || rect.MinY != tt.expected.MinY ||
				rect.MaxX != tt.expected.MaxX || rect.MaxY != tt.expected.MaxY {
				t.Errorf("expected Rectangle(%v, %v, %v, %v), got Rectangle(%v, %v, %v, %v)",
					tt.expected.MinX, tt.expected.MinY, tt.expected.MaxX, tt.expected.MaxY,
					rect.MinX, rect.MinY, rect.MaxX, rect.MaxY)
			}
		})
	}
}

func TestJTSGeometryDecoder_DecodeLineString(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	// Create WKB linestring data
	buf := new(bytes.Buffer)
	buf.WriteByte(WKBNDR) // Little endian
	binary.Write(buf, binary.LittleEndian, WKBLineString)
	binary.Write(buf, binary.LittleEndian, uint32(3)) // 3 points

	// Write points
	points := []struct{ x, y float64 }{
		{0, 0},
		{10, 10},
		{20, 5},
	}
	for _, pt := range points {
		binary.Write(buf, binary.LittleEndian, pt.x)
		binary.Write(buf, binary.LittleEndian, pt.y)
	}

	// Decode
	shape, err := decoder.Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Should return Rectangle (bounding box)
	rect, ok := shape.(*Rectangle)
	if !ok {
		t.Fatalf("expected *Rectangle, got %T", shape)
	}

	// Bounding box should be (0,0) to (20,10)
	if rect.MinX != 0 || rect.MinY != 0 || rect.MaxX != 20 || rect.MaxY != 10 {
		t.Errorf("expected Rectangle(0, 0, 20, 10), got Rectangle(%v, %v, %v, %v)",
			rect.MinX, rect.MinY, rect.MaxX, rect.MaxY)
	}
}

func TestJTSGeometryDecoder_DecodeFrom(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	point := NewPoint(10.5, 20.5)

	// Create WKB data
	buf := new(bytes.Buffer)
	buf.WriteByte(WKBNDR)
	binary.Write(buf, binary.LittleEndian, WKBPoint)
	binary.Write(buf, binary.LittleEndian, point.X)
	binary.Write(buf, binary.LittleEndian, point.Y)

	// Decode from reader
	shape, err := decoder.DecodeFrom(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	p, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if p.X != point.X || p.Y != point.Y {
		t.Errorf("coordinates mismatch")
	}
}

func TestJTSGeometryDecoder_ErrorCases(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	// Test empty data
	t.Run("empty data", func(t *testing.T) {
		_, err := decoder.Decode([]byte{})
		if err == nil {
			t.Error("expected error for empty data")
		}
	})

	// Test invalid byte order
	t.Run("invalid byte order", func(t *testing.T) {
		data := []byte{0xFF}
		_, err := decoder.Decode(data)
		if err == nil {
			t.Error("expected error for invalid byte order")
		}
	})

	// Test unsupported geometry type
	t.Run("unsupported geometry type", func(t *testing.T) {
		buf := new(bytes.Buffer)
		buf.WriteByte(WKBNDR)
		binary.Write(buf, binary.LittleEndian, uint32(999))
		_, err := decoder.Decode(buf.Bytes())
		if err == nil {
			t.Error("expected error for unsupported geometry type")
		}
	})

	// Test polygon with no rings
	t.Run("polygon no rings", func(t *testing.T) {
		buf := new(bytes.Buffer)
		buf.WriteByte(WKBNDR)
		binary.Write(buf, binary.LittleEndian, WKBPolygon)
		binary.Write(buf, binary.LittleEndian, uint32(0)) // 0 rings
		_, err := decoder.Decode(buf.Bytes())
		if err == nil {
			t.Error("expected error for polygon with no rings")
		}
	})

	// Test polygon with too few points
	t.Run("polygon too few points", func(t *testing.T) {
		buf := new(bytes.Buffer)
		buf.WriteByte(WKBNDR)
		binary.Write(buf, binary.LittleEndian, WKBPolygon)
		binary.Write(buf, binary.LittleEndian, uint32(1)) // 1 ring
		binary.Write(buf, binary.LittleEndian, uint32(3)) // 3 points (too few)
		_, err := decoder.Decode(buf.Bytes())
		if err == nil {
			t.Error("expected error for polygon with too few points")
		}
	})

	// Test linestring with no points
	t.Run("linestring no points", func(t *testing.T) {
		buf := new(bytes.Buffer)
		buf.WriteByte(WKBNDR)
		binary.Write(buf, binary.LittleEndian, WKBLineString)
		binary.Write(buf, binary.LittleEndian, uint32(0)) // 0 points
		_, err := decoder.Decode(buf.Bytes())
		if err == nil {
			t.Error("expected error for linestring with no points")
		}
	})
}

func TestJTSGeometryDecoder_DecodeWithValidation(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	t.Run("valid point", func(t *testing.T) {
		// Create valid WKB point
		buf := new(bytes.Buffer)
		buf.WriteByte(WKBNDR)
		binary.Write(buf, binary.LittleEndian, WKBPoint)
		binary.Write(buf, binary.LittleEndian, 10.5)
		binary.Write(buf, binary.LittleEndian, 20.5)

		shape, err := decoder.DecodeWithValidation(buf.Bytes())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if shape == nil {
			t.Error("expected non-nil shape")
		}
	})

	t.Run("invalid bounding box", func(t *testing.T) {
		// Create point with NaN
		buf := new(bytes.Buffer)
		buf.WriteByte(WKBNDR)
		binary.Write(buf, binary.LittleEndian, WKBPoint)
		binary.Write(buf, binary.LittleEndian, math.NaN())
		binary.Write(buf, binary.LittleEndian, 20.5)

		_, err := decoder.DecodeWithValidation(buf.Bytes())
		if err == nil {
			t.Error("expected error for NaN coordinate")
		}
	})
}

func TestJTSGeometryDecoder_FormatInfo(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	if decoder.GetFormatName() != "WKB" {
		t.Errorf("expected format name 'WKB', got '%s'", decoder.GetFormatName())
	}

	if decoder.GetFormatVersion() != "1.2.0" {
		t.Errorf("expected version '1.2.0', got '%s'", decoder.GetFormatVersion())
	}
}

func TestJTSGeometryDecoder_SetCalculator(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	calc := &CartesianCalculator{}
	decoder.SetCalculator(calc)

	if decoder.GetCalculator() != calc {
		t.Error("failed to set calculator")
	}
}

func TestGeometryDecoderFactory(t *testing.T) {
	factory := NewGeometryDecoderFactory()

	// Create decoder
	decoder := factory.CreateJTSGeometryDecoder()
	if decoder == nil {
		t.Fatal("expected non-nil decoder")
	}

	// Set custom calculator
	calc := &CartesianCalculator{}
	factory.SetDefaultCalculator(calc)

	// Create another decoder
	decoder2 := factory.CreateJTSGeometryDecoder()
	if decoder2 == nil {
		t.Fatal("expected non-nil decoder")
	}
}

func TestJTSGeometryDecoder_IntegrationWithSerializer(t *testing.T) {
	// Test round-trip serialization/deserialization
	serializer := NewJTSGeometrySerializer()
	decoder := NewJTSGeometryDecoder()

	tests := []struct {
		name  string
		shape Shape
	}{
		{"point", NewPoint(10.5, 20.5)},
		{"rectangle", NewRectangle(0, 0, 100, 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := serializer.Serialize(tt.shape)
			if err != nil {
				t.Fatalf("serialize failed: %v", err)
			}

			// Decode
			decoded, err := decoder.Decode(data)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			// Compare bounding boxes
			originalBBox := tt.shape.GetBoundingBox()
			decodedBBox := decoded.GetBoundingBox()

			if originalBBox == nil || decodedBBox == nil {
				t.Fatal("expected non-nil bounding boxes")
			}

			if originalBBox.MinX != decodedBBox.MinX ||
				originalBBox.MinY != decodedBBox.MinY ||
				originalBBox.MaxX != decodedBBox.MaxX ||
				originalBBox.MaxY != decodedBBox.MaxY {
				t.Errorf("bounding box mismatch: expected %v, got %v", originalBBox, decodedBBox)
			}
		})
	}
}

func TestJTSGeometryDecoder_PolygonWithHoles(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	// Create WKB polygon with outer ring and one hole
	buf := new(bytes.Buffer)
	buf.WriteByte(WKBNDR)
	binary.Write(buf, binary.LittleEndian, WKBPolygon)
	binary.Write(buf, binary.LittleEndian, uint32(2)) // 2 rings

	// Outer ring (5 points)
	binary.Write(buf, binary.LittleEndian, uint32(5))
	binary.Write(buf, binary.LittleEndian, 0.0) // SW
	binary.Write(buf, binary.LittleEndian, 0.0)
	binary.Write(buf, binary.LittleEndian, 100.0) // SE
	binary.Write(buf, binary.LittleEndian, 0.0)
	binary.Write(buf, binary.LittleEndian, 100.0) // NE
	binary.Write(buf, binary.LittleEndian, 100.0)
	binary.Write(buf, binary.LittleEndian, 0.0) // NW
	binary.Write(buf, binary.LittleEndian, 100.0)
	binary.Write(buf, binary.LittleEndian, 0.0) // SW (close)
	binary.Write(buf, binary.LittleEndian, 0.0)

	// Inner ring/hole (5 points)
	binary.Write(buf, binary.LittleEndian, uint32(5))
	binary.Write(buf, binary.LittleEndian, 25.0)
	binary.Write(buf, binary.LittleEndian, 25.0)
	binary.Write(buf, binary.LittleEndian, 75.0)
	binary.Write(buf, binary.LittleEndian, 25.0)
	binary.Write(buf, binary.LittleEndian, 75.0)
	binary.Write(buf, binary.LittleEndian, 75.0)
	binary.Write(buf, binary.LittleEndian, 25.0)
	binary.Write(buf, binary.LittleEndian, 75.0)
	binary.Write(buf, binary.LittleEndian, 25.0)
	binary.Write(buf, binary.LittleEndian, 25.0)

	// Decode - should return bounding box of outer ring
	shape, err := decoder.Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	rect, ok := shape.(*Rectangle)
	if !ok {
		t.Fatalf("expected *Rectangle, got %T", shape)
	}

	// Should be bounding box of outer ring
	if rect.MinX != 0 || rect.MinY != 0 || rect.MaxX != 100 || rect.MaxY != 100 {
		t.Errorf("expected Rectangle(0, 0, 100, 100), got Rectangle(%v, %v, %v, %v)",
			rect.MinX, rect.MinY, rect.MaxX, rect.MaxY)
	}
}

func TestJTSGeometryDecoder_SRID(t *testing.T) {
	decoder := NewJTSGeometryDecoder()

	// Create WKB point with SRID flag
	buf := new(bytes.Buffer)
	buf.WriteByte(WKBNDR)
	// Set SRID flag in geometry type
	geomType := WKBPoint | WKBGeometrySRIDFlag
	binary.Write(buf, binary.LittleEndian, geomType)
	binary.Write(buf, binary.LittleEndian, uint32(4326)) // SRID (WGS84)
	binary.Write(buf, binary.LittleEndian, 10.5)
	binary.Write(buf, binary.LittleEndian, 20.5)

	// Decode
	shape, err := decoder.Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	p, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if p.X != 10.5 || p.Y != 20.5 {
		t.Errorf("coordinates mismatch")
	}
}

// BenchmarkPointDecoding benchmarks point decoding
func BenchmarkPointDecoding(b *testing.B) {
	decoder := NewJTSGeometryDecoder()

	// Create WKB point data
	buf := new(bytes.Buffer)
	buf.WriteByte(WKBNDR)
	binary.Write(buf, binary.LittleEndian, WKBPoint)
	binary.Write(buf, binary.LittleEndian, 10.5)
	binary.Write(buf, binary.LittleEndian, 20.5)
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decoder.Decode(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPolygonDecoding benchmarks polygon decoding
func BenchmarkPolygonDecoding(b *testing.B) {
	decoder := NewJTSGeometryDecoder()

	// Create WKB polygon data
	buf := new(bytes.Buffer)
	buf.WriteByte(WKBNDR)
	binary.Write(buf, binary.LittleEndian, WKBPolygon)
	binary.Write(buf, binary.LittleEndian, uint32(1))
	binary.Write(buf, binary.LittleEndian, uint32(5))
	for i := 0; i < 5; i++ {
		binary.Write(buf, binary.LittleEndian, float64(i*10))
		binary.Write(buf, binary.LittleEndian, float64(i*10))
	}
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decoder.Decode(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
