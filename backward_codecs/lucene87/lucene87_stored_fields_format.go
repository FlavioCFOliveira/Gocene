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
	// BestCompressionMode is the compression mode BEST_COMPRESSION uses.
	//
	// Mirrors {@code static final CompressionMode BEST_COMPRESSION_MODE =
	// new DeflateWithPresetDictCompressionMode();}.
	BestCompressionMode compressing.CompressionMode = NewDeflateWithPresetDictCompressionMode()
	// BestSpeedMode is the compression mode BEST_SPEED uses.
	//
	// Mirrors {@code static final CompressionMode BEST_SPEED_MODE =
	// new LZ4WithPresetDictCompressionMode();}.
	BestSpeedMode compressing.CompressionMode = NewLZ4WithPresetDictCompressionMode()
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
	format, err := f.impl(mode)
	if err != nil {
		return nil, err
	}
	return format.FieldsReader(dir, si, fn, context)
}

// FieldsWriter is not supported for old codecs.
func (f *Lucene87StoredFieldsFormat) FieldsWriter(dir store.Directory, si *spi.SegmentInfo, context store.IOContext) (spi.StoredFieldsWriter, error) {
	return nil, fmt.Errorf("old codecs may only be used for reading")
}

// impl reproduces {@code StoredFieldsFormat impl(Mode mode)}.
//
// Java's switch has no checked exception; the Go constructor validates its
// arguments and returns an error, which cannot fire for these literals but is
// propagated rather than discarded.
func (f *Lucene87StoredFieldsFormat) impl(mode Mode) (spi.StoredFieldsFormat, error) {
	switch mode {
	case BestSpeed:
		return lucene50compressing.NewLucene50CompressingStoredFieldsFormat(
			"Lucene87StoredFieldsFastData", BestSpeedMode, bestSpeedBlockLength, 1024, 10)
	case BestCompression:
		return lucene50compressing.NewLucene50CompressingStoredFieldsFormat(
			"Lucene87StoredFieldsHighData", BestCompressionMode, bestCompressionBlockLength, 4096, 10)
	default:
		return nil, fmt.Errorf("unsupported mode: %v", mode)
	}
}
