// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// IOUtils provides utility methods for I/O operations.
// This is the Go port of Lucene's org.apache.lucene.util.IOUtils.
//
// IOUtils provides helper methods for:
//   - Closing resources safely
//   - Deleting files with exception handling
//   - FSync operations for durability
//   - Applying functions to multiple resources
type IOUtils struct{}

// Close closes the given Closeable, ignoring any errors.
// Use this when you don't care about errors during close.
func Close(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}

// CloseWhileHandlingException closes the given Closeable and returns any error.
// The error is wrapped with context about what was being closed.
func CloseWhileHandlingException(c io.Closer, name string) error {
	if c == nil {
		return nil
	}
	if err := c.Close(); err != nil {
		return fmt.Errorf("error closing %s: %w", name, err)
	}
	return nil
}

// CloseWhileSuppressingExceptions closes all given Closeables, suppressing exceptions.
// If a primary error is provided, any close errors are added to it.
// If a close throws a panic/recoverable error, it takes precedence over the primary error.
// Returns the primary error (possibly with suppressed exceptions) or any new error/panic that occurred.
func CloseWhileSuppressingExceptions(primaryErr error, closeables ...io.Closer) error {
	var suppressed []error
	var panicErr error

	for _, c := range closeables {
		if c == nil {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicErr = fmt.Errorf("panic during close: %v", r)
				}
			}()
			if err := c.Close(); err != nil {
				suppressed = append(suppressed, err)
			}
		}()
	}

	// If there was a panic, it takes precedence
	if panicErr != nil {
		// If we have a primary error, add it as suppressed to the panic error
		if primaryErr != nil {
			return fmt.Errorf("%w [suppressed: %v]", panicErr, primaryErr)
		}
		return panicErr
	}

	// If there's no primary error but we have suppressed errors, return them
	if primaryErr == nil && len(suppressed) > 0 {
		return fmt.Errorf("errors during close: %v", suppressed)
	}

	// Add suppressed errors to primary error
	if primaryErr != nil && len(suppressed) > 0 {
		return fmt.Errorf("%w [suppressed: %v]", primaryErr, suppressed)
	}

	return primaryErr
}

// CloseAll closes all given Closeables, collecting all errors.
// Returns a single error containing all individual errors, or nil if all closed successfully.
func CloseAll(closeables ...io.Closer) error {
	var errs []error
	for _, c := range closeables {
		if c != nil {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing resources: %v", errs)
	}
	return nil
}

// CloseAllWhileHandlingException closes all given Closeables, ignoring errors.
// This is useful when you need to close multiple resources and don't want
// one close error to prevent closing the others.
func CloseAllWhileHandlingException(closeables ...io.Closer) {
	for _, c := range closeables {
		Close(c)
	}
}

// DeleteFilesIgnoringExceptions deletes the given files, ignoring any errors.
// This is useful for cleanup operations where you don't care if deletion fails.
func DeleteFilesIgnoringExceptions(files ...string) {
	for _, file := range files {
		_ = os.Remove(file)
	}
}

// DeleteFilesIfExist deletes the given files if they exist.
// Unlike DeleteFiles, this does not return an error for non-existent files.
func DeleteFilesIfExist(files ...string) error {
	var errs []error
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("failed to delete %s: %w", file, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors deleting files: %v", errs)
	}
	return nil
}

// DeleteFiles deletes the given files, collecting all errors.
// Returns a single error containing all individual errors, or nil if all deletions succeeded.
func DeleteFiles(files ...string) error {
	var errs []error
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("failed to delete %s: %w", file, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors deleting files: %v", errs)
	}
	return nil
}

// FSync syncs the given file to disk, ensuring all data is written.
// This is important for durability guarantees.
func FSync(file *os.File) error {
	if file == nil {
		return nil
	}
	return file.Sync()
}

// FSyncDirectory syncs the given directory to disk.
// On some systems, this ensures that directory entries are persisted.
func FSyncDirectory(dir string) error {
	// Open the directory
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("failed to open directory %s for fsync: %w", dir, err)
	}
	defer f.Close()

	// Sync the directory
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to fsync directory %s: %w", dir, err)
	}
	return nil
}

// ApplyToAll applies the given function to all items.
// All items are always processed; the first non-nil error is returned.
// This matches Lucene's IOUtils.applyToAll semantics.
func ApplyToAll[T any](items []T, fn func(T) error) error {
	var firstErr error
	for _, item := range items {
		if err := fn(item); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// CloseChan closes the given channel safely.
// This is useful for closing channels that may be accessed concurrently.
func CloseChan[T any](ch chan T) {
	select {
	case <-ch:
		// Already closed
	default:
		close(ch)
	}
}

// SafeClose closes a resource with a recover to catch panics.
// This is useful when closing resources that might panic.
func SafeClose(c io.Closer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while closing: %v", r)
		}
	}()
	if c != nil {
		err = c.Close()
	}
	return
}

// ResourcePool manages a pool of closable resources.
// This is useful for managing resources that need to be closed together.
type ResourcePool struct {
	resources []io.Closer
	mu        sync.Mutex
}

// NewResourcePool creates a new ResourcePool.
func NewResourcePool() *ResourcePool {
	return &ResourcePool{
		resources: make([]io.Closer, 0),
	}
}

// Add adds a resource to the pool.
func (p *ResourcePool) Add(c io.Closer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resources = append(p.resources, c)
}

// CloseAll closes all resources in the pool.
// Returns an error if any close fails, but attempts to close all resources.
func (p *ResourcePool) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for _, c := range p.resources {
		if c != nil {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	p.resources = p.resources[:0] // Clear the slice

	if len(errs) > 0 {
		return fmt.Errorf("errors closing resources: %v", errs)
	}
	return nil
}

// CloseAllWhileHandlingException closes all resources, ignoring errors.
func (p *ResourcePool) CloseAllWhileHandlingException() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, c := range p.resources {
		Close(c)
	}
	p.resources = p.resources[:0] // Clear the slice
}

// Len returns the number of resources in the pool.
func (p *ResourcePool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.resources)
}

// EnsureOpen panics if the given closer is nil.
// This is useful for defensive programming.
func EnsureOpen(c io.Closer, name string) {
	if c == nil {
		panic(fmt.Sprintf("%s is closed", name))
	}
}

// CheckClosed returns true if the given error indicates a closed resource.
func CheckClosed(err error) bool {
	if err == nil {
		return false
	}
	// Check for common closed error messages
	errStr := err.Error()
	return contains(errStr, "closed") || contains(errStr, "EOF")
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetDecodingReader wraps the given stream in a reader that decodes it with
// charSet and fails, instead of substituting, when the bytes do not match the
// expected charset.
//
// This is the Go port of Lucene's IOUtils.getDecodingReader(InputStream,
// Charset) (lucene/core/src/java/org/apache/lucene/util/IOUtils.java). Java
// characters are rendered as UTF-8 bytes: the returned reader yields the
// decoded text as UTF-8. java.nio.charset.Charset is rendered as
// golang.org/x/text/encoding.Encoding.
//
// Java's decoder is configured with CodingErrorAction.REPORT for malformed
// input and unmappable characters, so reading throws a
// CharacterCodingException. Gocene reproduces that as follows:
//   - for unicode.UTF8 the stream is checked with encoding.UTF8Validator,
//     which fails on exactly the byte sequences Java's UTF-8 decoder rejects;
//   - for every other encoding, x/text decoders substitute U+FFFD for
//     malformed or unmappable input, so the decoded text is rejected at the
//     first U+FFFD. Residual difference: a U+FFFD that is legitimately encoded
//     in a non-UTF-8 stream (for example in UTF-16) is also rejected, whereas
//     Java decodes it.
//
// The error returned on a decoding failure wraps the underlying cause and
// renders Java's MalformedInputException/UnmappableCharacterException.
//
// The returned reader is buffered (Java returns a BufferedReader). Closing it
// closes stream when stream implements io.Closer, as closing Java's
// BufferedReader closes the wrapped InputStream.
func GetDecodingReader(stream io.Reader, charSet encoding.Encoding) io.ReadCloser {
	var t transform.Transformer
	if charSet == unicode.UTF8 {
		t = encoding.UTF8Validator
	} else {
		t = transform.Chain(charSet.NewDecoder(), reportReplacement{})
	}
	return &decodingReader{
		Reader: bufio.NewReader(&decodingErrorReader{r: transform.NewReader(stream, t)}),
		stream: stream,
	}
}

// decodingReader is the buffered reader returned by GetDecodingReader.
type decodingReader struct {
	*bufio.Reader
	stream io.Reader
}

// Close closes the wrapped stream when it implements io.Closer.
func (d *decodingReader) Close() error {
	if c, ok := d.stream.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// decodingErrorReader labels decoding failures as Java's
// CharacterCodingException while keeping the cause reachable via errors.Is.
type decodingErrorReader struct {
	r io.Reader
}

func (d *decodingErrorReader) Read(p []byte) (int, error) {
	n, err := d.r.Read(p)
	if err != nil && isDecodingError(err) {
		err = fmt.Errorf("MalformedInputException: %w", err)
	}
	return n, err
}

func isDecodingError(err error) bool {
	return errors.Is(err, encoding.ErrInvalidUTF8) || errors.Is(err, errUnmappableOrMalformed)
}

// errUnmappableOrMalformed is reported when a non-UTF-8 decoder substituted
// U+FFFD for input it could not decode.
var errUnmappableOrMalformed = errors.New("encoding: malformed input or unmappable character")

// reportReplacement copies UTF-8 text and fails at the first U+FFFD, which
// x/text decoders emit in place of malformed or unmappable input.
type reportReplacement struct{ transform.NopResetter }

func (reportReplacement) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for nSrc < len(src) {
		r, size := rune(src[nSrc]), 1
		if r >= utf8.RuneSelf {
			if !atEOF && !utf8.FullRune(src[nSrc:]) {
				return nDst, nSrc, transform.ErrShortSrc
			}
			r, size = utf8.DecodeRune(src[nSrc:])
			if r == utf8.RuneError {
				return nDst, nSrc, errUnmappableOrMalformed
			}
		}
		if nDst+size > len(dst) {
			return nDst, nSrc, transform.ErrShortDst
		}
		copy(dst[nDst:], src[nSrc:nSrc+size])
		nDst += size
		nSrc += size
	}
	return nDst, nSrc, nil
}
