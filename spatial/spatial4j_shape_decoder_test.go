// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNewSpatial4jShapeDecoder(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	if decoder == nil {
		t.Fatal("expected non-nil decoder")
	}
	if decoder.GetContext() != ctx {
		t.Error("expected context to match")
	}
}

func TestNewSpatial4jShapeDecoderWithCalculator(t *testing.T) {
	ctx := NewSpatialContext()
	calc := &CartesianCalculator{}
	decoder := NewSpatial4jShapeDecoderWithCalculator(ctx, calc)

	if decoder == nil {
		t.Fatal("expected non-nil decoder")
	}
	if decoder.GetContext() != ctx {
		t.Error("expected context to match")
	}
}

func TestSpatial4jShapeDecoder_DecodeFromWKT_Point(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	tests := []struct {
		name     string
		wkt      string
		expected Point
	}{
		{
			name:     "simple space",
			wkt:      "POINT(10.5 20.5)",
			expected: NewPoint(10.5, 20.5),
		},
		{
			name:     "simple comma",
			wkt:      "POINT(10.5, 20.5)",
			expected: NewPoint(10.5, 20.5),
		},
		{
			name:     "with spaces",
			wkt:      "POINT(  10.5   20.5  )",
			expected: NewPoint(10.5, 20.5),
		},
		{
			name:     "negative",
			wkt:      "POINT(-10.5 -20.5)",
			expected: NewPoint(-10.5, -20.5),
		},
		{
			name:     "origin",
			wkt:      "POINT(0 0)",
			expected: NewPoint(0, 0),
		},
		{
			name:     "uppercase",
			wkt:      "POINT(10.5 20.5)",
			expected: NewPoint(10.5, 20.5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shape, err := decoder.DecodeFromWKT(tt.wkt)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			point, ok := shape.(Point)
			if !ok {
				t.Fatalf("expected Point, got %T", shape)
			}

			if point.X != tt.expected.X || point.Y != tt.expected.Y {
				t.Errorf("expected Point(%v, %v), got Point(%v, %v)",
					tt.expected.X, tt.expected.Y, point.X, point.Y)
			}
		})
	}
}

func TestSpatial4jShapeDecoder_DecodeFromWKT_Envelope(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	tests := []struct {
		name     string
		wkt      string
		expected *Rectangle
	}{
		{
			name:     "simple",
			wkt:      "ENVELOPE(0, 10, 20, 5)",
			expected: NewRectangle(0, 5, 10, 20),
		},
		{
			name:     "with spaces",
			wkt:      "ENVELOPE(  0  ,  10  ,  20  ,  5  )",
			expected: NewRectangle(0, 5, 10, 20),
		},
		{
			name:     "negative",
			wkt:      "ENVELOPE(-100, -10, -20, -50)",
			expected: NewRectangle(-100, -50, -10, -20),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shape, err := decoder.DecodeFromWKT(tt.wkt)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			rect, ok := shape.(*Rectangle)
			if !ok {
				t.Fatalf("expected *Rectangle, got %T", shape)
			}

			if rect.MinX != tt.expected.MinX || rect.MinY != tt.expected.MinY ||
				rect.MaxX != tt.expected.MaxX || rect.MaxY != tt.expected.MaxY {
				t.Errorf("expected Rectangle(%v, %v, %v, %v), got Rectangle(%v, %v, %v, %v)",
					tt.expected.MinX, tt.expected.MinY, tt.expected.MaxX, tt.expected.MaxY,
					rect.MinX, rect.MinY, rect.MaxX, rect.MaxY)
			}
		})
	}
}

func TestSpatial4jShapeDecoder_DecodeFromWKT_Rectangle(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	tests := []struct {
		name     string
		wkt      string
		expected *Rectangle
	}{
		{
			name:     "simple comma",
			wkt:      "RECTANGLE(0, 10, 5, 20)",
			expected: NewRectangle(0, 5, 10, 20),
		},
		{
			name:     "simple space",
			wkt:      "RECTANGLE(0 10 5 20)",
			expected: NewRectangle(0, 5, 10, 20),
		},
		{
			name:     "negative",
			wkt:      "RECTANGLE(-100, -10, -50, -20)",
			expected: NewRectangle(-100, -50, -10, -20),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shape, err := decoder.DecodeFromWKT(tt.wkt)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			rect, ok := shape.(*Rectangle)
			if !ok {
				t.Fatalf("expected *Rectangle, got %T", shape)
			}

			if rect.MinX != tt.expected.MinX || rect.MinY != tt.expected.MinY ||
				rect.MaxX != tt.expected.MaxX || rect.MaxY != tt.expected.MaxY {
				t.Errorf("expected Rectangle(%v, %v, %v, %v), got Rectangle(%v, %v, %v, %v)",
					tt.expected.MinX, tt.expected.MinY, tt.expected.MaxX, tt.expected.MaxY,
					rect.MinX, rect.MinY, rect.MaxX, rect.MaxY)
			}
		})
	}
}

func TestSpatial4jShapeDecoder_DecodeFromWKT_Errors(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	tests := []struct {
		name string
		wkt  string
	}{
		{"empty", ""},
		{"no paren", "POINT 10 20"},
		{"no close", "POINT(10 20"},
		{"invalid type", "UNKNOWN(10 20)"},
		{"invalid coords", "POINT(x y)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decoder.DecodeFromWKT(tt.wkt)
			if err == nil {
				t.Errorf("expected error for WKT: %s", tt.wkt)
			}
		})
	}
}

func TestSpatial4jShapeDecoder_DecodeFromBytes_Point(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	// Create binary point data
	buf := new(bytes.Buffer)
	buf.WriteByte(spatial4jTypePoint)
	binary.Write(buf, binary.LittleEndian, 10.5)
	binary.Write(buf, binary.LittleEndian, 20.5)

	shape, err := decoder.DecodeFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	point, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if point.X != 10.5 || point.Y != 20.5 {
		t.Errorf("expected Point(10.5, 20.5), got Point(%v, %v)", point.X, point.Y)
	}
}

func TestSpatial4jShapeDecoder_DecodeFromBytes_Rectangle(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	// Create binary rectangle data
	buf := new(bytes.Buffer)
	buf.WriteByte(spatial4jTypeRectangle)
	binary.Write(buf, binary.LittleEndian, 0.0)  // minX
	binary.Write(buf, binary.LittleEndian, 5.0)  // minY
	binary.Write(buf, binary.LittleEndian, 10.0) // maxX
	binary.Write(buf, binary.LittleEndian, 20.0) // maxY

	shape, err := decoder.DecodeFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	rect, ok := shape.(*Rectangle)
	if !ok {
		t.Fatalf("expected *Rectangle, got %T", shape)
	}

	if rect.MinX != 0.0 || rect.MinY != 5.0 || rect.MaxX != 10.0 || rect.MaxY != 20.0 {
		t.Errorf("unexpected rectangle: %v", rect)
	}
}

func TestSpatial4jShapeDecoder_DecodeFromBytes_Circle(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	// Create binary circle data
	buf := new(bytes.Buffer)
	buf.WriteByte(spatial4jTypeCircle)
	binary.Write(buf, binary.LittleEndian, 50.0) // centerX
	binary.Write(buf, binary.LittleEndian, 50.0) // centerY
	binary.Write(buf, binary.LittleEndian, 10.0) // radius

	shape, err := decoder.DecodeFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Circle returns bounding box as Rectangle
	rect, ok := shape.(*Rectangle)
	if !ok {
		t.Fatalf("expected *Rectangle, got %T", shape)
	}

	// Bounding box should be center +/- radius
	if rect.MinX != 40.0 || rect.MinY != 40.0 || rect.MaxX != 60.0 || rect.MaxY != 60.0 {
		t.Errorf("expected Rectangle(40, 40, 60, 60), got Rectangle(%v, %v, %v, %v)",
			rect.MinX, rect.MinY, rect.MaxX, rect.MaxY)
	}
}

func TestSpatial4jShapeDecoder_DecodeFromBytes_Errors(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	// Test empty data
	t.Run("empty", func(t *testing.T) {
		_, err := decoder.DecodeFromBytes([]byte{})
		if err == nil {
			t.Error("expected error for empty data")
		}
	})

	// Test invalid type
	t.Run("invalid type", func(t *testing.T) {
		_, err := decoder.DecodeFromBytes([]byte{0xFF})
		if err == nil {
			t.Error("expected error for invalid type")
		}
	})
}

func TestSpatial4jShapeDecoder_EncodeToBytes(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	t.Run("point", func(t *testing.T) {
		point := NewPoint(10.5, 20.5)
		data, err := decoder.EncodeToBytes(point)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}

		// Verify type
		if data[0] != spatial4jTypePoint {
			t.Errorf("expected type %d, got %d", spatial4jTypePoint, data[0])
		}

		// Decode and verify
		shape, err := decoder.DecodeFromBytes(data)
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
	})

	t.Run("rectangle", func(t *testing.T) {
		rect := NewRectangle(0, 5, 10, 20)
		data, err := decoder.EncodeToBytes(rect)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}

		// Verify type
		if data[0] != spatial4jTypeRectangle {
			t.Errorf("expected type %d, got %d", spatial4jTypeRectangle, data[0])
		}

		// Decode and verify
		shape, err := decoder.DecodeFromBytes(data)
		if err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		r, ok := shape.(*Rectangle)
		if !ok {
			t.Fatalf("expected *Rectangle, got %T", shape)
		}

		if r.MinX != 0.0 || r.MinY != 5.0 || r.MaxX != 10.0 || r.MaxY != 20.0 {
			t.Errorf("rectangle mismatch")
		}
	})

	t.Run("nil shape", func(t *testing.T) {
		_, err := decoder.EncodeToBytes(nil)
		if err == nil {
			t.Error("expected error for nil shape")
		}
	})
}

func TestSpatial4jShapeDecoder_EncodeToWKT(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	t.Run("point", func(t *testing.T) {
		point := NewPoint(10.5, 20.5)
		wkt, err := decoder.EncodeToWKT(point)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}

		// Should contain POINT
		if !contains(wkt, "POINT") {
			t.Errorf("expected WKT to contain POINT: %s", wkt)
		}
	})

	t.Run("rectangle", func(t *testing.T) {
		rect := NewRectangle(0, 5, 10, 20)
		wkt, err := decoder.EncodeToWKT(rect)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}

		// Should contain ENVELOPE
		if !contains(wkt, "ENVELOPE") {
			t.Errorf("expected WKT to contain ENVELOPE: %s", wkt)
		}
	})

	t.Run("nil shape", func(t *testing.T) {
		_, err := decoder.EncodeToWKT(nil)
		if err == nil {
			t.Error("expected error for nil shape")
		}
	})
}

func TestSpatial4jShapeDecoder_RoundTrip(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	tests := []struct {
		name  string
		shape Shape
	}{
		{"point", NewPoint(10.5, 20.5)},
		{"rectangle", NewRectangle(0, 5, 100, 200)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode to WKT
			wkt, err := decoder.EncodeToWKT(tt.shape)
			if err != nil {
				t.Fatalf("encode to WKT failed: %v", err)
			}

			// Decode from WKT
			decoded, err := decoder.DecodeFromWKT(wkt)
			if err != nil {
				t.Fatalf("decode from WKT failed: %v", err)
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

func TestSpatial4jShapeDecoder_FormatInfo(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	if decoder.GetFormatName() != "Spatial4j" {
		t.Errorf("expected format name 'Spatial4j', got '%s'", decoder.GetFormatName())
	}

	if decoder.GetFormatVersion() != "0.8" {
		t.Errorf("expected version '0.8', got '%s'", decoder.GetFormatVersion())
	}
}

func TestSpatial4jShapeDecoder_SetContext(t *testing.T) {
	ctx1 := NewSpatialContext()
	ctx2 := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx1)

	if decoder.GetContext() != ctx1 {
		t.Error("expected context to be ctx1")
	}

	decoder.SetContext(ctx2)
	if decoder.GetContext() != ctx2 {
		t.Error("expected context to be ctx2")
	}
}

func TestSpatial4jShapeDecoder_Decode(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	// Test Decode method (uses binary format)
	buf := new(bytes.Buffer)
	buf.WriteByte(spatial4jTypePoint)
	binary.Write(buf, binary.LittleEndian, 10.5)
	binary.Write(buf, binary.LittleEndian, 20.5)

	shape, err := decoder.Decode(buf.Bytes())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	point, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if point.X != 10.5 || point.Y != 20.5 {
		t.Errorf("coordinates mismatch")
	}
}

func TestSpatial4jShapeDecoder_DecodeFrom(t *testing.T) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	// Test DecodeFrom method (uses binary format)
	buf := new(bytes.Buffer)
	buf.WriteByte(spatial4jTypePoint)
	binary.Write(buf, binary.LittleEndian, 10.5)
	binary.Write(buf, binary.LittleEndian, 20.5)

	shape, err := decoder.DecodeFrom(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	point, ok := shape.(Point)
	if !ok {
		t.Fatalf("expected Point, got %T", shape)
	}

	if point.X != 10.5 || point.Y != 20.5 {
		t.Errorf("coordinates mismatch")
	}
}

func TestSpatial4jShapeDecoderFactory(t *testing.T) {
	ctx := NewSpatialContext()
	factory := NewSpatial4jShapeDecoderFactory(ctx)

	decoder := factory.CreateDecoder()
	if decoder == nil {
		t.Fatal("expected non-nil decoder")
	}

	if decoder.GetContext() != ctx {
		t.Error("expected context to match")
	}
}

// BenchmarkWKTDecoding benchmarks WKT parsing
func BenchmarkWKTDecoding(b *testing.B) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)
	wkt := "POINT(10.5 20.5)"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decoder.DecodeFromWKT(wkt)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBinaryDecoding benchmarks binary decoding
func BenchmarkBinaryDecoding(b *testing.B) {
	ctx := NewSpatialContext()
	decoder := NewSpatial4jShapeDecoder(ctx)

	buf := new(bytes.Buffer)
	buf.WriteByte(spatial4jTypePoint)
	binary.Write(buf, binary.LittleEndian, 10.5)
	binary.Write(buf, binary.LittleEndian, 20.5)
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decoder.DecodeFromBytes(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
