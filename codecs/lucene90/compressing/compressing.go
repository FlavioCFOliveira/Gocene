// Package compressing hosts the Sprint 48 ports for
// org.apache.lucene.codecs.lucene90.compressing.
package compressing

import (
	"fmt"

	codecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// The Sprint 48 lucene90-compressing port surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// (chunk-based LZ4/Deflate compression layout with chunk-index slicing)
// land progressively in follow-up deep-port sprints.

// FieldsIndexWriter mirrors
// org.apache.lucene.codecs.lucene90.compressing.FieldsIndexWriter.
type FieldsIndexWriter struct{}

// NewFieldsIndexWriter builds a FieldsIndexWriter.
func NewFieldsIndexWriter() *FieldsIndexWriter { return &FieldsIndexWriter{} }

// Lucene90CompressingStoredFieldsFormat mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsFormat.
//
// The on-disk chunk layout (LZ4/Deflate compressed sub-blocks with a
// monotonic per-block index) is being ported progressively; the type is
// already configurable so callers such as Lucene90StoredFieldsFormat can
// resolve the BEST_SPEED / BEST_COMPRESSION preset parameters.
type Lucene90CompressingStoredFieldsFormat struct {
	formatName      string
	compressionMode compressing.CompressionMode
	chunkSize       int
	maxDocsPerChunk int
	blockShift      int
}

// NewLucene90CompressingStoredFieldsFormat builds a
// Lucene90CompressingStoredFieldsFormat with zero-valued tuning
// parameters. The constructor is preserved for backwards compatibility
// with the original Sprint 48 stub; prefer
// NewLucene90CompressingStoredFieldsFormatWithOptions when the preset
// values are known.
func NewLucene90CompressingStoredFieldsFormat() *Lucene90CompressingStoredFieldsFormat {
	return &Lucene90CompressingStoredFieldsFormat{}
}

// NewLucene90CompressingStoredFieldsFormatWithOptions builds a
// Lucene90CompressingStoredFieldsFormat configured for a specific mode.
//
// It is the Go counterpart of the 5-arg Java constructor
// Lucene90CompressingStoredFieldsFormat(String, CompressionMode, int,
// int, int).
func NewLucene90CompressingStoredFieldsFormatWithOptions(
	formatName string,
	compressionMode compressing.CompressionMode,
	chunkSize, maxDocsPerChunk, blockShift int,
) *Lucene90CompressingStoredFieldsFormat {
	return &Lucene90CompressingStoredFieldsFormat{
		formatName:      formatName,
		compressionMode: compressionMode,
		chunkSize:       chunkSize,
		maxDocsPerChunk: maxDocsPerChunk,
		blockShift:      blockShift,
	}
}

// FormatName returns the on-disk format tag (one of
// "Lucene90StoredFieldsFastData" / "Lucene90StoredFieldsHighData").
func (f *Lucene90CompressingStoredFieldsFormat) FormatName() string { return f.formatName }

// CompressionMode returns the configured CompressionMode singleton.
func (f *Lucene90CompressingStoredFieldsFormat) CompressionMode() compressing.CompressionMode {
	return f.compressionMode
}

// ChunkSize returns the target uncompressed chunk size in bytes.
func (f *Lucene90CompressingStoredFieldsFormat) ChunkSize() int { return f.chunkSize }

// MaxDocsPerChunk returns the cap on documents per stored-fields chunk.
func (f *Lucene90CompressingStoredFieldsFormat) MaxDocsPerChunk() int { return f.maxDocsPerChunk }

// BlockShift returns the per-fields-index block shift (one block per
// 1 << blockShift chunks).
func (f *Lucene90CompressingStoredFieldsFormat) BlockShift() int { return f.blockShift }

// Name implements [codecs.StoredFieldsFormat].
func (f *Lucene90CompressingStoredFieldsFormat) Name() string {
	return "Lucene90CompressingStoredFieldsFormat"
}

// FieldsReader is not yet implemented; the .fdt/.fdx/.fdm decoder lands
// in a follow-up deep-port sprint. It is declared so callers can rely on
// the [codecs.StoredFieldsFormat] interface today.
func (f *Lucene90CompressingStoredFieldsFormat) FieldsReader(
	_ store.Directory,
	_ *index.SegmentInfo,
	_ *index.FieldInfos,
	_ store.IOContext,
) (codecs.StoredFieldsReader, error) {
	return nil, fmt.Errorf(
		"lucene90/compressing: FieldsReader for %s not implemented yet",
		f.formatName,
	)
}

// FieldsWriter is not yet implemented; see FieldsReader.
func (f *Lucene90CompressingStoredFieldsFormat) FieldsWriter(
	_ store.Directory,
	_ *index.SegmentInfo,
	_ store.IOContext,
) (codecs.StoredFieldsWriter, error) {
	return nil, fmt.Errorf(
		"lucene90/compressing: FieldsWriter for %s not implemented yet",
		f.formatName,
	)
}

// Compile-time guarantee that the format satisfies the public
// [codecs.StoredFieldsFormat] interface.
var _ codecs.StoredFieldsFormat = (*Lucene90CompressingStoredFieldsFormat)(nil)

// Lucene90CompressingStoredFieldsReader mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsReader.
type Lucene90CompressingStoredFieldsReader struct{}

// NewLucene90CompressingStoredFieldsReader builds a
// Lucene90CompressingStoredFieldsReader.
func NewLucene90CompressingStoredFieldsReader() *Lucene90CompressingStoredFieldsReader {
	return &Lucene90CompressingStoredFieldsReader{}
}

// Lucene90CompressingStoredFieldsWriter mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsWriter.
type Lucene90CompressingStoredFieldsWriter struct{}

// NewLucene90CompressingStoredFieldsWriter builds a
// Lucene90CompressingStoredFieldsWriter.
func NewLucene90CompressingStoredFieldsWriter() *Lucene90CompressingStoredFieldsWriter {
	return &Lucene90CompressingStoredFieldsWriter{}
}

// Lucene90CompressingTermVectorsFormat mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsFormat.
type Lucene90CompressingTermVectorsFormat struct{}

// NewLucene90CompressingTermVectorsFormat builds a
// Lucene90CompressingTermVectorsFormat.
func NewLucene90CompressingTermVectorsFormat() *Lucene90CompressingTermVectorsFormat {
	return &Lucene90CompressingTermVectorsFormat{}
}

// Lucene90CompressingTermVectorsReader mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsReader.
type Lucene90CompressingTermVectorsReader struct{}

// NewLucene90CompressingTermVectorsReader builds a
// Lucene90CompressingTermVectorsReader.
func NewLucene90CompressingTermVectorsReader() *Lucene90CompressingTermVectorsReader {
	return &Lucene90CompressingTermVectorsReader{}
}

// Lucene90CompressingTermVectorsWriter mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsWriter.
type Lucene90CompressingTermVectorsWriter struct{}

// NewLucene90CompressingTermVectorsWriter builds a
// Lucene90CompressingTermVectorsWriter.
func NewLucene90CompressingTermVectorsWriter() *Lucene90CompressingTermVectorsWriter {
	return &Lucene90CompressingTermVectorsWriter{}
}
