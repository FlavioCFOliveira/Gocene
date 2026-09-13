// Package lucene87 implements org.apache.lucene.backward_codecs.lucene87.
package lucene87

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/backward_codecs/compressing"
	lucene50compressing "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene50/compressing"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Mode mirrors org.apache.lucene.backward_codecs.lucene87.Lucene87StoredFieldsFormat.Mode.
type Mode int

const (
	// BestSpeed trade compression ratio for retrieval speed.
	BestSpeed Mode = iota
	// BestCompression trade retrieval speed for compression ratio.
	BestCompression
)

func (m Mode) String() string {
	switch m {
	case BestSpeed:
		return "BEST_SPEED"
	case BestCompression:
		return "BEST_COMPRESSION"
	default:
		return fmt.Sprintf("Mode(%d)", m)
	}
}

// parseMode converts a string representation of Mode to its corresponding constant.
func parseMode(s string) (Mode, error) {
	switch s {
	case "BEST_SPEED":
		return BestSpeed, nil
	case "BEST_COMPRESSION":
		return BestCompression, nil
	default:
		return 0, fmt.Errorf("invalid mode: %s", s)
	}
}

// ModeKey mirrors org.apache.lucene.backward_codecs.lucene87.Lucene87StoredFieldsFormat.MODE_KEY.
const ModeKey = "Lucene87StoredFieldsFormat.mode"

const (
	bestCompressionBlockLength = 10 * 48 * 1024
	bestSpeedBlockLength       = 10 * 8 * 1024
)

var (
	// BestCompressionMode mirrors org.apache.lucene.backward_codecs.lucene87.Lucene87StoredFieldsFormat.BEST_COMPRESSION_MODE.
	BestCompressionMode = compressing.NewCompressionMode("DeflateWithPresetDictCompressionMode")
	// BestSpeedMode mirrors org.apache.lucene.backward_codecs.lucene87.Lucene87StoredFieldsFormat.BEST_SPEED_MODE.
	BestSpeedMode = compressing.NewCompressionMode("LZ4WithPresetDictCompressionMode")
)

// Lucene87StoredFieldsFormat mirrors org.apache.lucene.backward_codecs.lucene87.Lucene87StoredFieldsFormat.
type Lucene87StoredFieldsFormat struct {
	mode Mode
}

// NewLucene87StoredFieldsFormat returns a Lucene87StoredFieldsFormat with default options (BestSpeed).
func NewLucene87StoredFieldsFormat() *Lucene87StoredFieldsFormat {
	return NewLucene87StoredFieldsFormatWithMode(BestSpeed)
}

// NewLucene87StoredFieldsFormatWithMode returns a Lucene87StoredFieldsFormat with the specified mode.
func NewLucene87StoredFieldsFormatWithMode(mode Mode) *Lucene87StoredFieldsFormat {
	return &Lucene87StoredFieldsFormat{
		mode: mode,
	}
}

// Name returns the codec name.
func (f *Lucene87StoredFieldsFormat) Name() string {
	return "Lucene87StoredFieldsFormat"
}

// FieldsReader opens a reader over the stored fields.
func (f *Lucene87StoredFieldsFormat) FieldsReader(dir store.Directory, si *spi.SegmentInfo, fn *spi.FieldInfos, context store.IOContext) (spi.StoredFieldsReader, error) {
	value := si.GetAttribute(ModeKey)
	if value == "" {
		return nil, fmt.Errorf("missing value for %s for segment: %s", ModeKey, si.Name)
	}
	mode, err := parseMode(value)
	if err != nil {
		return nil, err
	}
	return f.impl(mode).FieldsReader(dir, si, fn, context)
}

// FieldsWriter is not supported for old codecs.
func (f *Lucene87StoredFieldsFormat) FieldsWriter(dir store.Directory, si *spi.SegmentInfo, context store.IOContext) (spi.StoredFieldsWriter, error) {
	return nil, fmt.Errorf("old codecs may only be used for reading")
}

func (f *Lucene87StoredFieldsFormat) impl(mode Mode) spi.StoredFieldsFormat {
	switch mode {
	case BestSpeed:
		// Java: return new Lucene50CompressingStoredFieldsFormat("Lucene87StoredFieldsFastData", BEST_SPEED_MODE, BEST_SPEED_BLOCK_LENGTH, 1024, 10);
		// Note: Gocene stub currently only takes formatName.
		return lucene50compressing.NewLucene50CompressingStoredFieldsFormat("Lucene87StoredFieldsFastData")
	case BestCompression:
		// Java: return new Lucene50CompressingStoredFieldsFormat("Lucene87StoredFieldsHighData", BEST_COMPRESSION_MODE, BEST_COMPRESSION_BLOCK_LENGTH, 4096, 10);
		// Note: Gocene stub currently only takes formatName.
		return lucene50compressing.NewLucene50CompressingStoredFieldsFormat("Lucene87StoredFieldsHighData")
	default:
		panic("unsupported mode")
	}
}
