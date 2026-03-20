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

func TestJTSGeometrySerializer_New(t *testing.T) {
	serializer := NewJTSGeometrySerializer()
	if serializer == nil {
		t.Fatal("expected non-nil serializer")
	}
	if serializer.GetByteOrder() != binary.LittleEndian {
		t.Errorf("expected LittleEndian byte order, got %v", serializer.GetByteOrder())
	}
}

func TestJTSGeometrySerializer_NewWithByteOrder(t *testing.T) {
	// Test LittleEndian
	le := NewJTSGeometrySerializerWithByteOrder(binary.LittleEndian)
	if le.GetByteOrder() != binary.LittleEndian {
		t.Error("expected LittleEndian byte order")
	}

	// Test BigEndian
	be := NewJTSGeometrySerializerWithByteOrder(binary.BigEndian)
	if be.GetByteOrder() != binary.BigEndian {
		t.Error("expected BigEndian byte order")
	}
}

func TestJTSGeometrySerializer_SerializePoint(t *testing.T) {
	serializer := NewJTSGeometrySerializer()

	tests := []struct {
		name  string
		point Point
	}{
		{"origin", NewPoint(0, 0)},
		{"positive", NewPoint(10.5, 20.5)},
		{"negative", NewPoint(-10.5, -20.5)},
		{"mixed", NewPoint(-10.5, 20.5)},
		{"large", NewPoint(123456789.123, -987654321.456)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := serializer.Serialize(tt.point)
			if err != nil {
				t.Fatalf("serialize failed: %v", err)
			}

			// Expected size: 1 (byte order) + 4 (type) + 8 (X) + 8 (Y) = 21 bytes
			expectedSize := 21
			if len(data) != expectedSize {
				t.Errorf("expected %d bytes, got %d", expectedSize, len(data))
			}

			// Verify byte order marker
			if data[0] != WKBNDR {
				t.Errorf("expected LittleEndian marker %d, got %d", WKBNDR, data[0])
			}

			// Verify geometry type
			geomType := binary.LittleEndian.Uint32(data[1:5])
			if geomType != WKBPoint {
				t.Errorf("expected Point type %d, got %d", WKBPoint, geomType)
			}

			// Verify coordinates
			xBits := binary.LittleEndian.Uint64(data[5:13])
			yBits := binary.LittleEndian.Uint64(data[13:21])

			x := math.Float64frombits(xBits)
			y := math.Float64frombits(yBits)

			if x != tt.point.X {
				t.Errorf("expected X=%v, got %v", tt.point.X, x)
			}
			if y != tt.point.Y {
				t.Errorf("expected Y=%v, got %v", tt.point.Y, y)
			}
		})
	}
}

func TestJTSGeometrySerializer_SerializeRectangle(t *testing.T) {
	serializer := NewJTSGeometrySerializer()

	tests := []struct {
		name      string
		rectangle *Rectangle
	}{
		{
			name:      "simple",
			rectangle: NewRectangle(0, 0, 10, 10),
		},
		{
			name:      "negative",
			rectangle: NewRectangle(-10, -20, -5, -10),
		},
		{
			name:      "mixed",
			rectangle: NewRectangle(-180, -90, 180, 90),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := serializer.Serialize(tt.rectangle)
			if err != nil {
				t.Fatalf("serialize failed: %v", err)
			}

			// Expected size: 1 (byte order) + 4 (type) + 4 (rings) + 4 (points) + 5*16 (coords) = 93 bytes
			expectedSize := 93
			if len(data) != expectedSize {
				t.Errorf("expected %d bytes, got %d", expectedSize, len(data))
			}

			// Verify it's a polygon type
			geomType := binary.LittleEndian.Uint32(data[1:5])
			if geomType != WKBPolygon {
				t.Errorf("expected Polygon type %d, got %d", WKBPolygon, geomType)
			}

			// Verify number of rings
			numRings := binary.LittleEndian.Uint32(data[5:9])
			if numRings != 1 {
				t.Errorf("expected 1 ring, got %d", numRings)
			}

			// Verify number of points (5 for rectangle)
			numPoints := binary.LittleEndian.Uint32(data[9:13])
			if numPoints != 5 {
				t.Errorf("expected 5 points, got %d", numPoints)
			}
		})
	}
}

func TestJTSGeometrySerializer_DeserializePoint(t *testing.T) {
	serializer := NewJTSGeometrySerializer()

	tests := []struct {
		name     string
		original Point
	}{
		{"origin", NewPoint(0, 0)},
		{"simple", NewPoint(10.5, 20.5)},
		{"negative", NewPoint(-10.5, -20.5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := serializer.Serialize(tt.original)
			if err != nil {
				t.Fatalf("serialize failed: %v", err)
			}

			// Deserialize
			shape, err := serializer.Deserialize(data)
			if err != nil {
				t.Fatalf("deserialize failed: %v", err)
			}

			// Check type
			point, ok := shape.(Point)
			if !ok {
				t.Fatalf("expected Point, got %T", shape)
			}

			// Compare coordinates
			if point.X != tt.original.X || point.Y != tt.original.Y {
				t.Errorf("expected Point(%v, %v), got Point(%v, %v)",
					tt.original.X, tt.original.Y, point.X, point.Y)
			}
		})
	}
}

func TestJTSGeometrySerializer_DeserializeRectangle(t *testing.T) {
	serializer := NewJTSGeometrySerializer()

	tests := []struct {
		name      string
		original  *Rectangle
		tolerance float64
	}{
		{
			name:     "simple",
			original: NewRectangle(0, 0, 10, 20),
		},
		{
			name:     "negative",
			original: NewRectangle(-100, -50, -10, -20),
		},
		{
			name:     "coordinates",
			original: NewRectangle(-180, -90, 180, 90),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := serializer.Serialize(tt.original)
			if err != nil {
				t.Fatalf("serialize failed: %v", err)
			}

			// Deserialize (returns Rectangle from Polygon)
			shape, err := serializer.Deserialize(data)
			if err != nil {
				t.Fatalf("deserialize failed: %v", err)
			}

			// Check type
			rect, ok := shape.(*Rectangle)
			if !ok {
				t.Fatalf("expected *Rectangle, got %T", shape)
			}

			// Compare coordinates
			if rect.MinX != tt.original.MinX || rect.MinY != tt.original.MinY ||
				rect.MaxX != tt.original.MaxX || rect.MaxY != tt.original.MaxY {
				t.Errorf("expected Rectangle(%v, %v, %v, %v), got Rectangle(%v, %v, %v, %v)",
					tt.original.MinX, tt.original.MinY, tt.original.MaxX, tt.original.MaxY,
					rect.MinX, rect.MinY, rect.MaxX, rect.MaxY)
			}
		})
	}
}

func TestJTSGeometrySerializer_BigEndian(t *testing.T) {
	serializer := NewJTSGeometrySerializerWithByteOrder(binary.BigEndian)

	point := NewPoint(10.5, 20.5)
	data, err := serializer.Serialize(point)
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	// Verify BigEndian marker
	if data[0] != WKBXDR {
		t.Errorf("expected BigEndian marker %d, got %d", WKBXDR, data[0])
	}

	// Deserialize should work with either byte order
	shape, err := serializer.Deserialize(data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	p, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if p.X != point.X || p.Y != point.Y {
		t.Errorf("coordinates mismatch: expected (%v, %v), got (%v, %v)",
			point.X, point.Y, p.X, p.Y)
	}
}

func TestJTSGeometrySerializer_ErrorCases(t *testing.T) {
	serializer := NewJTSGeometrySerializer()

	// Test nil shape serialization
	t.Run("nil shape", func(t *testing.T) {
		_, err := serializer.Serialize(nil)
		if err == nil {
			t.Error("expected error for nil shape")
		}
	})

	// Test empty data deserialization
	t.Run("empty data", func(t *testing.T) {
		_, err := serializer.Deserialize([]byte{})
		if err == nil {
			t.Error("expected error for empty data")
		}
	})

	// Test invalid byte order
	t.Run("invalid byte order", func(t *testing.T) {
		data := []byte{0xFF} // Invalid byte order
		_, err := serializer.Deserialize(data)
		if err == nil {
			t.Error("expected error for invalid byte order")
		}
	})

	// Test unsupported geometry type
	t.Run("unsupported geometry type", func(t *testing.T) {
		buf := new(bytes.Buffer)
		buf.WriteByte(WKBNDR)                               // Little endian
		binary.Write(buf, binary.LittleEndian, uint32(999)) // Invalid type
		_, err := serializer.Deserialize(buf.Bytes())
		if err == nil {
			t.Error("expected error for unsupported geometry type")
		}
	})
}

func TestJTSGeometrySerializer_SerializeTo(t *testing.T) {
	serializer := NewJTSGeometrySerializer()

	t.Run("point", func(t *testing.T) {
		point := NewPoint(10.5, 20.5)
		var buf bytes.Buffer
		err := serializer.SerializeTo(&buf, point)
		if err != nil {
			t.Fatalf("serializeTo failed: %v", err)
		}

		data := buf.Bytes()
		if len(data) != 21 {
			t.Errorf("expected 21 bytes, got %d", len(data))
		}
	})

	t.Run("rectangle", func(t *testing.T) {
		rect := NewRectangle(0, 0, 10, 10)
		var buf bytes.Buffer
		err := serializer.SerializeTo(&buf, rect)
		if err != nil {
			t.Fatalf("serializeTo failed: %v", err)
		}

		data := buf.Bytes()
		if len(data) != 93 {
			t.Errorf("expected 93 bytes, got %d", len(data))
		}
	})

	t.Run("nil shape", func(t *testing.T) {
		var buf bytes.Buffer
		err := serializer.SerializeTo(&buf, nil)
		if err == nil {
			t.Error("expected error for nil shape")
		}
	})
}

func TestJTSGeometrySerializer_FormatInfo(t *testing.T) {
	serializer := NewJTSGeometrySerializer()

	if serializer.GetFormatName() != "WKB" {
		t.Errorf("expected format name 'WKB', got '%s'", serializer.GetFormatName())
	}

	if serializer.GetFormatVersion() != "1.2.0" {
		t.Errorf("expected version '1.2.0', got '%s'", serializer.GetFormatVersion())
	}
}

func TestJTSGeometrySerializer_SetByteOrder(t *testing.T) {
	serializer := NewJTSGeometrySerializer()

	// Change to BigEndian
	serializer.SetByteOrder(binary.BigEndian)
	if serializer.GetByteOrder() != binary.BigEndian {
		t.Error("failed to set byte order to BigEndian")
	}

	// Change back to LittleEndian
	serializer.SetByteOrder(binary.LittleEndian)
	if serializer.GetByteOrder() != binary.LittleEndian {
		t.Error("failed to set byte order to LittleEndian")
	}
}

// BenchmarkPointSerialization benchmarks point serialization
func BenchmarkPointSerialization(b *testing.B) {
	serializer := NewJTSGeometrySerializer()
	point := NewPoint(10.5, 20.5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.Serialize(point)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPointDeserialization benchmarks point deserialization
func BenchmarkPointDeserialization(b *testing.B) {
	serializer := NewJTSGeometrySerializer()
	point := NewPoint(10.5, 20.5)
	data, _ := serializer.Serialize(point)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.Deserialize(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRectangleSerialization benchmarks rectangle serialization
func BenchmarkRectangleSerialization(b *testing.B) {
	serializer := NewJTSGeometrySerializer()
	rect := NewRectangle(0, 0, 10, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.Serialize(rect)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRectangleDeserialization benchmarks rectangle deserialization
func BenchmarkRectangleDeserialization(b *testing.B) {
	serializer := NewJTSGeometrySerializer()
	rect := NewRectangle(0, 0, 10, 10)
	data, _ := serializer.Serialize(rect)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.Deserialize(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
