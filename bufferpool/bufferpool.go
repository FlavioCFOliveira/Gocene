// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package bufferpool provides reusable byte buffer pools for efficient memory management.
// This package helps reduce GC pressure by pooling frequently allocated buffers.
//
// The buffer pool is particularly useful for:
//   - I/O operations that require temporary buffers
//   - Data encoding/decoding operations
//   - Network packet handling
//   - File copying and transformation
//
// Example usage:
//
//	pool := bufferpool.New(32 * 1024) // 32KB buffers
//	buf := pool.Get()
//	defer pool.Put(buf)
//	// Use buffer...
package bufferpool

import (
	"sync"
)

// Pool is a pool of reusable byte buffers.
// It uses sync.Pool internally for efficient memory management.
type Pool struct {
	pool sync.Pool
	size int
}

// New creates a new buffer pool with buffers of the specified size.
//
// Parameters:
//   - bufferSize: the size of each buffer in bytes (e.g., 8192 for 8KB)
//
// Returns:
//   - a new Pool that can be used to get and put byte buffers
//
// Example:
//
//	pool := bufferpool.New(32 * 1024) // Create pool with 32KB buffers
func New(bufferSize int) *Pool {
	if bufferSize <= 0 {
		bufferSize = 8192 // Default 8KB
	}
	return &Pool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, bufferSize)
			},
		},
		size: bufferSize,
	}
}

// Get retrieves a buffer from the pool.
// If the pool is empty, a new buffer is allocated.
//
// Returns:
//   - a byte slice of the configured size
//
// Note: The returned buffer may contain garbage data.
// Always reset the slice before use if needed.
func (p *Pool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a buffer to the pool.
// The buffer should not be used after calling Put.
//
// Parameters:
//   - buf: the buffer to return to the pool
//
// Note: Buffers with capacity different from the pool size
// are accepted but may be garbage collected instead of reused.
func (p *Pool) Put(buf []byte) {
	if buf == nil {
		return
	}
	// Reset length but keep capacity
	p.pool.Put(buf[:cap(buf)])
}

// Size returns the configured buffer size for this pool.
func (p *Pool) Size() int {
	return p.size
}

// SizedPool is a collection of buffer pools for different sizes.
// It automatically selects the appropriate pool based on requested size.
type SizedPool struct {
	pools map[int]*Pool
	mu    sync.RWMutex
}

// NewSizedPool creates a new sized buffer pool.
//
// Parameters:
//   - sizes: the buffer sizes to create pools for (e.g., 1024, 4096, 16384)
//
// Returns:
//   - a new SizedPool that manages multiple size pools
func NewSizedPool(sizes ...int) *SizedPool {
	sp := &SizedPool{
		pools: make(map[int]*Pool, len(sizes)),
	}
	for _, size := range sizes {
		if size > 0 {
			sp.pools[size] = New(size)
		}
	}
	return sp
}

// Get retrieves a buffer of at least the requested size.
// It returns the smallest available buffer that fits the request.
//
// Parameters:
//   - minSize: the minimum buffer size required
//
// Returns:
//   - a byte slice with capacity >= minSize
func (sp *SizedPool) Get(minSize int) []byte {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	// Find the smallest pool that can accommodate minSize
	bestSize := -1
	for size := range sp.pools {
		if size >= minSize && (bestSize == -1 || size < bestSize) {
			bestSize = size
		}
	}

	if bestSize != -1 {
		return sp.pools[bestSize].Get()
	}

	// No suitable pool found, allocate new buffer
	return make([]byte, minSize)
}

// Put returns a buffer to the appropriate pool.
//
// Parameters:
//   - buf: the buffer to return
func (sp *SizedPool) Put(buf []byte) {
	if buf == nil {
		return
	}

	sp.mu.RLock()
	defer sp.mu.RUnlock()

	// Try to find a pool for this buffer size
	capSize := cap(buf)
	if pool, ok := sp.pools[capSize]; ok {
		pool.Put(buf)
		return
	}

	// No matching pool, let GC handle it
}

// Common buffer sizes for convenience
const (
	Size1KB   = 1024
	Size4KB   = 4096
	Size8KB   = 8192
	Size16KB  = 16384
	Size32KB  = 32768
	Size64KB  = 65536
	Size128KB = 131072
)

// CommonPools provides pre-configured sized pools for common use cases.
// This variable is lazily initialized on first access.
var CommonPools = NewSizedPool(Size1KB, Size4KB, Size8KB, Size16KB, Size32KB, Size64KB)
