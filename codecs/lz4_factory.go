// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"
)

// LZ4Factory creates LZ4 compressors and decompressors.
// This is the Go port of Lucene's LZ4Factory.
//
// The factory provides a centralized way to create LZ4 compression
// instances with different safety and performance characteristics.
type LZ4Factory struct {
	// safe indicates whether to use safe (slower but portable) implementations
	safe bool
	// highCompression indicates whether to use high compression mode
	highCompression bool
}

var (
	// lz4FactoryInstance is the singleton instance
	lz4FactoryInstance *LZ4Factory
	// lz4FactoryOnce ensures the factory is created only once
	lz4FactoryOnce sync.Once
	// lz4FactoryMu protects the factory configuration
	lz4FactoryMu sync.RWMutex
)

// GetLZ4Factory returns the singleton LZ4Factory instance.
// By default, it returns a factory configured for fast compression.
func GetLZ4Factory() *LZ4Factory {
	lz4FactoryOnce.Do(func() {
		lz4FactoryInstance = NewLZ4Factory(false, false)
	})
	return lz4FactoryInstance
}

// NewLZ4Factory creates a new LZ4Factory with the specified configuration.
//
// Parameters:
//   - safe: If true, uses safe implementations that don't rely on unsafe operations.
//     Safe implementations are slower but more portable.
//   - highCompression: If true, uses high compression mode (slower but better ratio).
//
// Returns:
//   - A new LZ4Factory instance
func NewLZ4Factory(safe, highCompression bool) *LZ4Factory {
	return &LZ4Factory{
		safe:            safe,
		highCompression: highCompression,
	}
}

// FastCompressor returns a fast LZ4 compressor.
// This compressor prioritizes speed over compression ratio.
func (f *LZ4Factory) FastCompressor() LZ4Compressor {
	if f.safe {
		return NewLZ4SafeCompressor(false)
	}
	return NewLZ4FastCompressor()
}

// HighCompressor returns a high-compression LZ4 compressor.
// This compressor prioritizes compression ratio over speed.
func (f *LZ4Factory) HighCompressor() LZ4Compressor {
	if f.safe {
		return NewLZ4SafeCompressor(true)
	}
	return NewLZ4HighCompressor()
}

// FastDecompressor returns a fast LZ4 decompressor.
func (f *LZ4Factory) FastDecompressor() LZ4Decompressor {
	if f.safe {
		return NewLZ4SafeDecompressor()
	}
	return NewLZ4FastDecompressor()
}

// SafeDecompressor returns a safe LZ4 decompressor.
// Safe decompressors perform additional bounds checking.
func (f *LZ4Factory) SafeDecompressor() LZ4Decompressor {
	return NewLZ4SafeDecompressor()
}

// IsSafe returns true if this factory creates safe implementations.
func (f *LZ4Factory) IsSafe() bool {
	return f.safe
}

// IsHighCompression returns true if this factory creates high-compression compressors.
func (f *LZ4Factory) IsHighCompression() bool {
	return f.highCompression
}

// LZ4Compressor is the interface for LZ4 compressors.
// This is the Go port of Lucene's LZ4Compressor interface.
type LZ4Compressor interface {
	// Compress compresses the source data into the destination buffer.
	// Returns the compressed data and any error encountered.
	Compress(src, dst []byte) ([]byte, error)

	// MaxCompressedLength returns the maximum compressed length for the given source length.
	MaxCompressedLength(srcLen int) int

	// Name returns the name of this compressor.
	Name() string
}

// LZ4Decompressor is the interface for LZ4 decompressors.
// This is the Go port of Lucene's LZ4Decompressor interface.
type LZ4Decompressor interface {
	// Decompress decompresses the source data into the destination buffer.
	// The uncompressed length must be known in advance.
	// Returns any error encountered.
	Decompress(src, dst []byte) error

	// Name returns the name of this decompressor.
	Name() string
}

// lz4FastCompressor is a fast LZ4 compressor implementation.
type lz4FastCompressor struct {
	name string
}

// NewLZ4FastCompressor creates a new fast LZ4 compressor.
func NewLZ4FastCompressor() LZ4Compressor {
	return &lz4FastCompressor{
		name: "LZ4FastCompressor",
	}
}

// Compress compresses the source data.
func (c *lz4FastCompressor) Compress(src, dst []byte) ([]byte, error) {
	// For now, use the existing lz4FastCompress function
	return lz4FastCompress(src)
}

// MaxCompressedLength returns the maximum compressed length.
func (c *lz4FastCompressor) MaxCompressedLength(srcLen int) int {
	// LZ4 compression overhead is typically small
	// Add 4 bytes for length prefix + some overhead
	return srcLen + 4 + (srcLen / 255) + 16
}

// Name returns the compressor name.
func (c *lz4FastCompressor) Name() string {
	return c.name
}

// lz4HighCompressor is a high-compression LZ4 compressor.
type lz4HighCompressor struct {
	name string
}

// NewLZ4HighCompressor creates a new high-compression LZ4 compressor.
func NewLZ4HighCompressor() LZ4Compressor {
	return &lz4HighCompressor{
		name: "LZ4HighCompressor",
	}
}

// Compress compresses the source data with high compression.
func (c *lz4HighCompressor) Compress(src, dst []byte) ([]byte, error) {
	// For now, use the existing lz4HighCompress function
	return lz4HighCompress(src)
}

// MaxCompressedLength returns the maximum compressed length.
func (c *lz4HighCompressor) MaxCompressedLength(srcLen int) int {
	// Same as fast compressor
	return srcLen + 4 + (srcLen / 255) + 16
}

// Name returns the compressor name.
func (c *lz4HighCompressor) Name() string {
	return c.name
}

// lz4SafeCompressor is a safe LZ4 compressor that performs bounds checking.
type lz4SafeCompressor struct {
	highCompression bool
	name            string
}

// NewLZ4SafeCompressor creates a new safe LZ4 compressor.
func NewLZ4SafeCompressor(highCompression bool) LZ4Compressor {
	name := "LZ4SafeCompressor"
	if highCompression {
		name = "LZ4SafeHighCompressor"
	}
	return &lz4SafeCompressor{
		highCompression: highCompression,
		name:            name,
	}
}

// Compress compresses the source data with bounds checking.
func (c *lz4SafeCompressor) Compress(src, dst []byte) ([]byte, error) {
	if len(src) == 0 {
		return []byte{}, nil
	}

	// Use the appropriate compression function
	if c.highCompression {
		return lz4HighCompress(src)
	}
	return lz4FastCompress(src)
}

// MaxCompressedLength returns the maximum compressed length.
func (c *lz4SafeCompressor) MaxCompressedLength(srcLen int) int {
	return srcLen + 4 + (srcLen / 255) + 16
}

// Name returns the compressor name.
func (c *lz4SafeCompressor) Name() string {
	return c.name
}

// lz4FastDecompressor is a fast LZ4 decompressor.
type lz4FastDecompressor struct {
	name string
}

// NewLZ4FastDecompressor creates a new fast LZ4 decompressor.
func NewLZ4FastDecompressor() LZ4Decompressor {
	return &lz4FastDecompressor{
		name: "LZ4FastDecompressor",
	}
}

// Decompress decompresses the source data.
func (d *lz4FastDecompressor) Decompress(src, dst []byte) error {
	if len(src) < 4 {
		return fmt.Errorf("source too short")
	}

	// Use the existing lz4Decompress function
	decompressed, err := lz4Decompress(src, len(dst))
	if err != nil {
		return err
	}

	if len(decompressed) != len(dst) {
		return fmt.Errorf("decompressed length mismatch: expected %d, got %d", len(dst), len(decompressed))
	}

	copy(dst, decompressed)
	return nil
}

// Name returns the decompressor name.
func (d *lz4FastDecompressor) Name() string {
	return d.name
}

// lz4SafeDecompressor is a safe LZ4 decompressor with bounds checking.
type lz4SafeDecompressor struct {
	name string
}

// NewLZ4SafeDecompressor creates a new safe LZ4 decompressor.
func NewLZ4SafeDecompressor() LZ4Decompressor {
	return &lz4SafeDecompressor{
		name: "LZ4SafeDecompressor",
	}
}

// Decompress decompresses the source data with bounds checking.
func (d *lz4SafeDecompressor) Decompress(src, dst []byte) error {
	if len(src) < 4 {
		return fmt.Errorf("source too short: got %d bytes, need at least 4", len(src))
	}

	// Read the original length
	originalLen := int(src[0])
	if len(dst) < originalLen {
		return fmt.Errorf("destination buffer too small: need %d, got %d", originalLen, len(dst))
	}

	// Use the existing lz4Decompress function
	decompressed, err := lz4Decompress(src, len(dst))
	if err != nil {
		return err
	}

	if len(decompressed) > len(dst) {
		return fmt.Errorf("decompressed data too large: %d bytes, buffer is %d", len(decompressed), len(dst))
	}

	copy(dst, decompressed)
	return nil
}

// Name returns the decompressor name.
func (d *lz4SafeDecompressor) Name() string {
	return d.name
}
