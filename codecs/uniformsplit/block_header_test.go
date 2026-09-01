package uniformsplit

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestBlockHeader_RoundTrip(t *testing.T) {
	tests := []struct {
		name             string
		linesCount       int32
		baseDocsFP       int64
		basePositionsFP  int64
		basePayloadsFP    int64
		termStatesOffset  int32
		middleLineOffset  int32
	}{
		{
			name:             "typical",
			linesCount:       32,
			baseDocsFP:       1000,
			basePositionsFP:  2000,
			basePayloadsFP:    3000,
			termStatesOffset:  500,
			middleLineOffset:  250,
		},
		{
			name:             "min_lines",
			linesCount:       1,
			baseDocsFP:       0,
			basePositionsFP:  0,
			basePayloadsFP:    0,
			termStatesOffset:  0,
			middleLineOffset:  0,
		},
		{
			name:             "max_lines",
			linesCount:       MaxNumBlockLines,
			baseDocsFP:       1e12,
			basePositionsFP:  2e12,
			basePayloadsFP:    3e12,
			termStatesOffset:  10000,
			middleLineOffset:  5000,
		},
		{
			name:             "large_fps",
			linesCount:       10,
			baseDocsFP:       9223372036854775807,
			basePositionsFP:  9223372036854775807,
			basePayloadsFP:    9223372036854775807,
			termStatesOffset:  1,
			middleLineOffset:  1,
		},
	}

	serializer := &BlockHeaderSerializer{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bh := NewBlockHeader(tc.linesCount, tc.baseDocsFP, tc.basePositionsFP, tc.basePayloadsFP, tc.termStatesOffset, tc.middleLineOffset)

			// Write to memory
			out := store.NewByteBuffersDataOutput()
			if err := serializer.Write(out, bh); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Read back
			in := out.ToDataInput()
			decoded, err := serializer.Read(in, nil)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}

			if decoded.LinesCount() != tc.linesCount {
				t.Errorf("linesCount: expected %d, got %d", tc.linesCount, decoded.LinesCount())
			}
			if decoded.BaseDocsFP() != tc.baseDocsFP {
				t.Errorf("baseDocsFP: expected %d, got %d", tc.baseDocsFP, decoded.BaseDocsFP())
			}
			if decoded.BasePositionsFP() != tc.basePositionsFP {
				t.Errorf("basePositionsFP: expected %d, got %d", tc.basePositionsFP, decoded.BasePositionsFP())
			}
			if decoded.BasePayloadsFP() != tc.basePayloadsFP {
				t.Errorf("basePayloadsFP: expected %d, got %d", tc.basePayloadsFP, decoded.BasePayloadsFP())
			}
			if decoded.TermStatesBaseOffset() != tc.termStatesOffset {
				t.Errorf("termStatesBaseOffset: expected %d, got %d", tc.termStatesOffset, decoded.TermStatesBaseOffset())
			}
			if decoded.MiddleLineOffset() != tc.middleLineOffset {
				t.Errorf("middleLineOffset: expected %d, got %d", tc.middleLineOffset, decoded.MiddleLineOffset())
			}
			if decoded.MiddleLineIndex() != tc.linesCount>>1 {
				t.Errorf("middleLineIndex: expected %d, got %d", tc.linesCount>>1, decoded.MiddleLineIndex())
			}
		})
	}
}

func TestBlockHeader_ReadReuse(t *testing.T) {
	serializer := &BlockHeaderSerializer{}
	bh := NewBlockHeader(10, 100, 200, 300, 50, 25)
	out := store.NewByteBuffersDataOutput()
	serializer.Write(out, bh)

	reuse := &BlockHeader{}
	decoded, err := serializer.Read(out.ToDataInput(), reuse)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if decoded != reuse {
		t.Errorf("expected decoded object to be the same instance as reuse")
	}
}

func TestBlockHeader_InvalidRead(t *testing.T) {
	serializer := &BlockHeaderSerializer{}

	t.Run("linesCount_zero", func(t *testing.T) {
		out := store.NewByteBuffersDataOutput()
		out.WriteVInt(0) // linesCount = 0
		out.WriteVLong(0)
		out.WriteVLong(0)
		out.WriteVLong(0)
		out.WriteVInt(0)
		out.WriteVInt(0)

		_, err := serializer.Read(out.ToDataInput(), nil)
		if err == nil {
			t.Error("expected error for linesCount=0")
		}
	})

	t.Run("linesCount_too_large", func(t *testing.T) {
		out := store.NewByteBuffersDataOutput()
		out.WriteVInt(MaxNumBlockLines + 1)
		out.WriteVLong(0)
		out.WriteVLong(0)
		out.WriteVLong(0)
		out.WriteVInt(0)
		out.WriteVInt(0)

		_, err := serializer.Read(out.ToDataInput(), nil)
		if err == nil {
			t.Error("expected error for linesCount > MaxNumBlockLines")
		}
	})

	t.Run("negative_offset", func(t *testing.T) {
		out := store.NewByteBuffersDataOutput()
		out.WriteVInt(10)
		out.WriteVLong(0)
		out.WriteVLong(0)
		out.WriteVLong(0)
		out.WriteVInt(-1) // termStatesBaseOffset = -1
		out.WriteVInt(0)

		_, err := serializer.Read(out.ToDataInput(), nil)
		if err == nil {
			t.Error("expected error for negative termStatesBaseOffset")
		}
	})
}
