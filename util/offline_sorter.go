// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
// Lucene's OfflineSorter is heavily tied to Lucene's Directory /
// IndexInput / IndexOutput abstractions, plus the JDK
// ExecutorService and CodecUtil checksum framing. None of these are
// available yet in Gocene, so the Go port keeps the *core algorithm*
// — chunked in-memory sort followed by multi-way merge via a min-heap
// priority queue — but substitutes:
//
//   - TempDirectory (a small abstraction over file creation/listing/
//     removal) for Lucene's Directory. The default implementation
//     wraps an os.TempDir() subtree;
//   - bufio.Reader / bufio.Writer for IndexInput / IndexOutput;
//   - the existing util.PriorityQueue for the merge heap.
//
// The on-disk format is the same length-prefixed binary layout used by
// Lucene's ByteSequencesReader/Writer: a 2-byte big-endian length
// followed by exactly that many bytes per entry. CodecUtil's checksum
// footer is omitted; consumers that need it can wrap the temp files.
//
// Concurrency: a single-goroutine sort + merge pipeline is used by
// default to keep the Sprint-1 surface minimal. The configurable
// MaxPartitionsInRAM field is preserved for future parallelisation
// but not currently consumed.
// -----------------------------------------------------------------------------

package util

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"
)

// OfflineSorter constants mirroring the Java public statics.
const (
	// OfflineSorterMB is one mebibyte in bytes.
	OfflineSorterMB int64 = 1 << 20
	// OfflineSorterGB is one gibibyte in bytes.
	OfflineSorterGB int64 = OfflineSorterMB << 10
	// OfflineSorterMinBufferSizeMB is the minimum recommended buffer
	// size (in MB) before performance degrades sharply.
	OfflineSorterMinBufferSizeMB int64 = 32
	// OfflineSorterAbsoluteMinSortBufferSize is the smallest buffer
	// the sorter will accept. Matches Java's 0.5 MB minimum.
	OfflineSorterAbsoluteMinSortBufferSize int64 = OfflineSorterMB / 2
	// OfflineSorterMaxTempFiles is the default fan-in for intermediate
	// merges; mirrors MAX_TEMPFILES in the Java source.
	OfflineSorterMaxTempFiles = 10

	// offlineSorterMaxValueLen is the maximum byte length per entry
	// (mirrors java.lang.Short.MAX_VALUE = 32767 in the Java source).
	offlineSorterMaxValueLen = 32767
)

// ErrOfflineSorterBufferTooSmall is returned when the configured RAM
// buffer is below OfflineSorterAbsoluteMinSortBufferSize.
var ErrOfflineSorterBufferTooSmall = errors.New("at least 0.5MB RAM buffer is needed")

// TempDirectory abstracts file creation for OfflineSorter so callers
// may swap an in-memory implementation (for tests) or a custom on-disk
// layout. The default implementation [NewOSTempDirectory] uses the
// process temp directory.
type TempDirectory interface {
	// CreateTempFile returns a path to a fresh, exclusive file under
	// this directory. The returned name is guaranteed to be unique
	// within this TempDirectory for the lifetime of the process.
	CreateTempFile(prefix, suffix string) (string, error)

	// Open returns an io.ReadCloser over the named file.
	Open(name string) (io.ReadCloser, error)

	// Create returns an io.WriteCloser to write the named file.
	Create(name string) (io.WriteCloser, error)

	// Remove deletes the named file. Missing files are not an error.
	Remove(name string) error

	// List returns the absolute paths of every temp file created
	// through this directory that has not yet been removed.
	List() []string
}

// osTempDir is the default TempDirectory backed by os.TempDir().
type osTempDir struct {
	root    string
	tracked atomic.Pointer[[]string]
}

// NewOSTempDirectory returns a TempDirectory rooted at a fresh
// subdirectory of os.TempDir() with the given prefix.
func NewOSTempDirectory(prefix string) (TempDirectory, error) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}
	d := &osTempDir{root: dir}
	d.tracked.Store(&[]string{})
	return d, nil
}

// CreateTempFile creates a new file under the directory root.
func (d *osTempDir) CreateTempFile(prefix, suffix string) (string, error) {
	f, err := os.CreateTemp(d.root, prefix+"-*"+suffix)
	if err != nil {
		return "", err
	}
	_ = f.Close()
	name := f.Name()
	for {
		cur := d.tracked.Load()
		next := append([]string{}, *cur...)
		next = append(next, name)
		if d.tracked.CompareAndSwap(cur, &next) {
			break
		}
	}
	return name, nil
}

// Open opens the file by absolute path.
func (d *osTempDir) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

// Create writes a new file by absolute path.
func (d *osTempDir) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}

// Remove deletes name and untracks it.
func (d *osTempDir) Remove(name string) error {
	for {
		cur := d.tracked.Load()
		next := make([]string, 0, len(*cur))
		for _, n := range *cur {
			if n != name {
				next = append(next, n)
			}
		}
		if d.tracked.CompareAndSwap(cur, &next) {
			break
		}
	}
	err := os.Remove(name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// List returns currently tracked names.
func (d *osTempDir) List() []string {
	cur := d.tracked.Load()
	out := make([]string, len(*cur))
	copy(out, *cur)
	return out
}

// Root returns the underlying filesystem root. Useful for tests.
func (d *osTempDir) Root() string { return d.root }

// SortInfo carries diagnostic counters for an OfflineSorter run,
// mirroring org.apache.lucene.util.OfflineSorter.SortInfo.
type SortInfo struct {
	TempMergeFiles int
	MergeRounds    int
	LineCount      int64
	MergeTimeMS    atomic.Int64
	SortTimeMS     atomic.Int64
	TotalTimeMS    int64
	ReadTimeMS     int64
	BufferSize     int64
}

// String returns the same textual report Lucene's SortInfo.toString()
// produces, with the same precision and field order.
func (s *SortInfo) String() string {
	return fmt.Sprintf(
		"time=%.2f sec. total (%.2f reading, %.2f sorting, %.2f merging), lines=%d, temp files=%d, merges=%d, soft ram limit=%.2f MB",
		float64(s.TotalTimeMS)/1000.0,
		float64(s.ReadTimeMS)/1000.0,
		float64(s.SortTimeMS.Load())/1000.0,
		float64(s.MergeTimeMS.Load())/1000.0,
		s.LineCount,
		s.TempMergeFiles,
		s.MergeRounds,
		float64(s.BufferSize)/float64(OfflineSorterMB),
	)
}

// BytesComparator is a strict-less comparator over byte slices.
// Returning negative means a<b, zero means a==b, positive means a>b.
// Matches Java's Comparator<BytesRef>#compare.
type BytesComparator func(a, b []byte) int

// DefaultBytesComparator is the Lucene default: lexicographic
// unsigned-byte ordering, equivalent to bytes.Compare.
func DefaultBytesComparator(a, b []byte) int { return bytes.Compare(a, b) }

// OfflineSorter is the Go port of org.apache.lucene.util.OfflineSorter.
// It sorts arbitrarily large input streams of length-prefixed byte
// entries by spilling sorted partitions to disk and then multi-way
// merging them.
type OfflineSorter struct {
	dir                TempDirectory
	tempFileNamePrefix string
	comparator         BytesComparator
	ramBufferSize      int64
	maxTempFiles       int
	valueLength        int // -1 for variable length
	maxPartitionsInRAM int
	Info               *SortInfo
}

// OfflineSorterOption configures an OfflineSorter at construction.
type OfflineSorterOption func(*OfflineSorter) error

// WithComparator sets a custom comparator. Default: DefaultBytesComparator.
func WithComparator(c BytesComparator) OfflineSorterOption {
	return func(o *OfflineSorter) error {
		if c == nil {
			return errors.New("comparator must not be nil")
		}
		o.comparator = c
		return nil
	}
}

// WithBufferSize sets the in-memory partition size in bytes. Values
// below OfflineSorterAbsoluteMinSortBufferSize are rejected.
func WithBufferSize(bytes int64) OfflineSorterOption {
	return func(o *OfflineSorter) error {
		if bytes < OfflineSorterAbsoluteMinSortBufferSize {
			return fmt.Errorf("%w: got %d", ErrOfflineSorterBufferTooSmall, bytes)
		}
		o.ramBufferSize = bytes
		return nil
	}
}

// WithMaxTempFiles sets the merge fan-in. Must be >= 2.
func WithMaxTempFiles(n int) OfflineSorterOption {
	return func(o *OfflineSorter) error {
		if n < 2 {
			return fmt.Errorf("maxTempFiles must be >= 2, got %d", n)
		}
		o.maxTempFiles = n
		return nil
	}
}

// WithFixedValueLength configures the sorter for fixed-width entries.
// Set to -1 (default) for variable width.
func WithFixedValueLength(n int) OfflineSorterOption {
	return func(o *OfflineSorter) error {
		if n != -1 && (n <= 0 || n > offlineSorterMaxValueLen) {
			return fmt.Errorf("valueLength must be 1..%d, got %d", offlineSorterMaxValueLen, n)
		}
		o.valueLength = n
		return nil
	}
}

// NewOfflineSorter constructs an OfflineSorter for the given temp
// directory and file-name prefix. Options override the defaults
// (DefaultBytesComparator, 32 MB buffer, MAX_TEMPFILES fan-in,
// variable value length).
func NewOfflineSorter(dir TempDirectory, tempFileNamePrefix string, opts ...OfflineSorterOption) (*OfflineSorter, error) {
	if dir == nil {
		return nil, errors.New("temp directory must not be nil")
	}
	s := &OfflineSorter{
		dir:                dir,
		tempFileNamePrefix: tempFileNamePrefix,
		comparator:         DefaultBytesComparator,
		ramBufferSize:      OfflineSorterMinBufferSizeMB * OfflineSorterMB,
		maxTempFiles:       OfflineSorterMaxTempFiles,
		valueLength:        -1,
		maxPartitionsInRAM: 1,
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Sort consumes the length-prefixed byte stream at inputFileName and
// returns the path to a freshly-written, sorted output file. Any
// intermediate files are deleted on success; on failure the partial
// outputs are best-effort removed.
func (s *OfflineSorter) Sort(inputFileName string) (string, error) {
	s.Info = &SortInfo{BufferSize: s.ramBufferSize}
	start := time.Now()

	created := map[string]struct{}{}
	cleanupOnError := func() {
		for n := range created {
			_ = s.dir.Remove(n)
		}
	}

	rc, err := s.dir.Open(inputFileName)
	if err != nil {
		return "", fmt.Errorf("open input: %w", err)
	}
	defer rc.Close()
	// On-disk format is always length-prefixed; valueLength only
	// influences partition-size capping in [readPartition].
	reader := newByteSequencesReader(rc, -1)

	var partitions []string
	for {
		buf, exhausted, readErr := s.readPartition(reader)
		if readErr != nil {
			cleanupOnError()
			return "", readErr
		}
		if len(buf) == 0 {
			if exhausted {
				break
			}
			continue
		}
		s.Info.LineCount += int64(len(buf))
		s.Info.TempMergeFiles++

		sortStart := time.Now()
		sort.SliceStable(buf, func(i, j int) bool {
			return s.comparator(buf[i], buf[j]) < 0
		})
		s.Info.SortTimeMS.Add(time.Since(sortStart).Milliseconds())

		partFile, err := s.writePartition(buf)
		if err != nil {
			cleanupOnError()
			return "", err
		}
		created[partFile] = struct{}{}
		partitions = append(partitions, partFile)

		// Cascade intermediate merges to bound the working set.
		for len(partitions) >= s.maxTempFiles {
			merged, err := s.mergeRound(partitions[len(partitions)-s.maxTempFiles:])
			if err != nil {
				cleanupOnError()
				return "", err
			}
			for _, p := range partitions[len(partitions)-s.maxTempFiles:] {
				delete(created, p)
				_ = s.dir.Remove(p)
			}
			partitions = partitions[:len(partitions)-s.maxTempFiles]
			partitions = append(partitions, merged)
			created[merged] = struct{}{}
		}

		if exhausted {
			break
		}
	}

	if len(partitions) == 0 {
		out, err := s.createOutput()
		if err != nil {
			cleanupOnError()
			return "", err
		}
		// Empty output is a zero-length file.
		s.Info.TotalTimeMS = time.Since(start).Milliseconds()
		return out, nil
	}

	for len(partitions) > 1 {
		fanIn := len(partitions)
		if fanIn > s.maxTempFiles {
			fanIn = s.maxTempFiles
		}
		merged, err := s.mergeRound(partitions[len(partitions)-fanIn:])
		if err != nil {
			cleanupOnError()
			return "", err
		}
		for _, p := range partitions[len(partitions)-fanIn:] {
			delete(created, p)
			_ = s.dir.Remove(p)
		}
		partitions = partitions[:len(partitions)-fanIn]
		partitions = append(partitions, merged)
		created[merged] = struct{}{}
	}

	s.Info.TotalTimeMS = time.Since(start).Milliseconds()
	return partitions[0], nil
}

// readPartition pulls items from the reader until the configured RAM
// budget is exceeded. Returns the items, whether the input is now
// exhausted, and any read error.
func (s *OfflineSorter) readPartition(r *byteSequencesReader) ([][]byte, bool, error) {
	start := time.Now()
	defer func() { s.Info.ReadTimeMS += time.Since(start).Milliseconds() }()

	var (
		buf   [][]byte
		used  int64
		exh   bool
		limit = s.ramBufferSize
	)
	if s.valueLength != -1 {
		// Fixed length: bound by entries-per-buffer.
		entries := int(limit / int64(s.valueLength))
		for i := 0; i < entries; i++ {
			b, err := r.next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					exh = true
					break
				}
				return nil, false, err
			}
			buf = append(buf, b)
		}
	} else {
		for {
			b, err := r.next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					exh = true
					break
				}
				return nil, false, err
			}
			buf = append(buf, b)
			// Approximate per-entry overhead: 24 (slice header) + len(b).
			used += int64(24 + len(b))
			if used > limit {
				break
			}
		}
	}
	return buf, exh, nil
}

// writePartition serialises buf to a fresh temp file in sorted order.
func (s *OfflineSorter) writePartition(buf [][]byte) (string, error) {
	name, err := s.dir.CreateTempFile(s.tempFileNamePrefix, ".tmp")
	if err != nil {
		return "", err
	}
	wc, err := s.dir.Create(name)
	if err != nil {
		return "", err
	}
	w := newByteSequencesWriter(wc)
	for _, b := range buf {
		if err := w.write(b); err != nil {
			_ = wc.Close()
			_ = s.dir.Remove(name)
			return "", err
		}
	}
	if err := w.flush(); err != nil {
		_ = wc.Close()
		_ = s.dir.Remove(name)
		return "", err
	}
	if err := wc.Close(); err != nil {
		_ = s.dir.Remove(name)
		return "", err
	}
	return name, nil
}

// createOutput allocates a fresh output file for the empty-input path.
func (s *OfflineSorter) createOutput() (string, error) {
	name, err := s.dir.CreateTempFile(s.tempFileNamePrefix, ".sort")
	if err != nil {
		return "", err
	}
	wc, err := s.dir.Create(name)
	if err != nil {
		_ = s.dir.Remove(name)
		return "", err
	}
	if err := wc.Close(); err != nil {
		_ = s.dir.Remove(name)
		return "", err
	}
	return name, nil
}

// mergeRound k-way-merges the given sorted partition files into a
// single output, returning its path.
func (s *OfflineSorter) mergeRound(partitions []string) (string, error) {
	start := time.Now()
	defer func() { s.Info.MergeTimeMS.Add(time.Since(start).Milliseconds()) }()
	s.Info.MergeRounds++

	type stream struct {
		r     *byteSequencesReader
		c     io.Closer
		cur   []byte
		index int
	}
	streams := make([]*stream, 0, len(partitions))
	for i, p := range partitions {
		rc, err := s.dir.Open(p)
		if err != nil {
			for _, st := range streams {
				_ = st.c.Close()
			}
			return "", fmt.Errorf("open partition %q: %w", p, err)
		}
		streams = append(streams, &stream{
			r:     newByteSequencesReader(rc, -1),
			c:     rc,
			index: i,
		})
	}
	defer func() {
		for _, st := range streams {
			_ = st.c.Close()
		}
	}()

	cmp := s.comparator
	pq, err := NewPriorityQueue(len(streams), func(a, b *stream) bool {
		c := cmp(a.cur, b.cur)
		if c != 0 {
			return c < 0
		}
		return a.index < b.index
	})
	if err != nil {
		return "", err
	}
	for _, st := range streams {
		b, err := st.r.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				continue
			}
			return "", err
		}
		st.cur = b
		pq.Add(st)
	}

	out, err := s.dir.CreateTempFile(s.tempFileNamePrefix, ".tmp")
	if err != nil {
		return "", err
	}
	wc, err := s.dir.Create(out)
	if err != nil {
		_ = s.dir.Remove(out)
		return "", err
	}
	defer wc.Close()
	w := newByteSequencesWriter(wc)

	for pq.Size() > 0 {
		top := pq.Top()
		if err := w.write(top.cur); err != nil {
			_ = s.dir.Remove(out)
			return "", err
		}
		nxt, err := top.r.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				pq.Pop()
				continue
			}
			_ = s.dir.Remove(out)
			return "", err
		}
		top.cur = nxt
		pq.UpdateTop()
	}
	if err := w.flush(); err != nil {
		_ = s.dir.Remove(out)
		return "", err
	}
	return out, nil
}

// byteSequencesReader reads length-prefixed byte entries from an
// io.Reader. The framing is 2-byte big-endian length followed by
// exactly that many bytes. Matches Lucene's ByteSequencesReader.
type byteSequencesReader struct {
	r       *bufio.Reader
	hdr     [2]byte
	scratch []byte
	fixed   int
}

func newByteSequencesReader(r io.Reader, valueLength int) *byteSequencesReader {
	return &byteSequencesReader{r: bufio.NewReader(r), fixed: valueLength}
}

func (b *byteSequencesReader) next() ([]byte, error) {
	var n int
	if b.fixed != -1 {
		n = b.fixed
	} else {
		if _, err := io.ReadFull(b.r, b.hdr[:]); err != nil {
			return nil, err
		}
		n = int(binary.BigEndian.Uint16(b.hdr[:]))
	}
	if cap(b.scratch) < n {
		b.scratch = make([]byte, n)
	} else {
		b.scratch = b.scratch[:n]
	}
	if _, err := io.ReadFull(b.r, b.scratch); err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, b.scratch)
	return out, nil
}

// byteSequencesWriter writes the same format consumed by
// byteSequencesReader.
type byteSequencesWriter struct {
	w   *bufio.Writer
	hdr [2]byte
}

func newByteSequencesWriter(w io.Writer) *byteSequencesWriter {
	return &byteSequencesWriter{w: bufio.NewWriter(w)}
}

// Write encodes b with a 2-byte big-endian length prefix.
func (b *byteSequencesWriter) write(p []byte) error {
	if len(p) > offlineSorterMaxValueLen {
		return fmt.Errorf("entry length %d exceeds offline sorter max %d", len(p), offlineSorterMaxValueLen)
	}
	binary.BigEndian.PutUint16(b.hdr[:], uint16(len(p)))
	if _, err := b.w.Write(b.hdr[:]); err != nil {
		return err
	}
	if _, err := b.w.Write(p); err != nil {
		return err
	}
	return nil
}

func (b *byteSequencesWriter) flush() error { return b.w.Flush() }

// WriteEntries is a convenience helper that creates a temp file in
// dir, writes the given byte slices in length-prefixed format, and
// returns its path. Useful in tests and small one-shot pipelines.
func WriteEntries(dir TempDirectory, prefix string, entries [][]byte) (string, error) {
	name, err := dir.CreateTempFile(prefix, ".tmp")
	if err != nil {
		return "", err
	}
	wc, err := dir.Create(name)
	if err != nil {
		_ = dir.Remove(name)
		return "", err
	}
	w := newByteSequencesWriter(wc)
	for _, e := range entries {
		if err := w.write(e); err != nil {
			_ = wc.Close()
			_ = dir.Remove(name)
			return "", err
		}
	}
	if err := w.flush(); err != nil {
		_ = wc.Close()
		_ = dir.Remove(name)
		return "", err
	}
	if err := wc.Close(); err != nil {
		_ = dir.Remove(name)
		return "", err
	}
	return name, nil
}

// ReadEntries is the inverse of [WriteEntries]: it reads every entry
// in the given file into memory.
func ReadEntries(dir TempDirectory, name string, valueLength int) ([][]byte, error) {
	rc, err := dir.Open(name)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	r := newByteSequencesReader(rc, valueLength)
	var out [][]byte
	for {
		b, err := r.next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return nil, err
		}
		out = append(out, b)
	}
}

// CleanupOSTempRoot removes the on-disk root of an osTempDir created
// by NewOSTempDirectory. Intended for callers that want explicit
// teardown (typically defer cleanupOSTempRoot(dir)).
func CleanupOSTempRoot(d TempDirectory) error {
	od, ok := d.(*osTempDir)
	if !ok {
		return nil
	}
	return os.RemoveAll(od.root)
}

// pathInDir returns dir/name. Kept as a helper so callers can pre-
// compute absolute paths without importing path/filepath when they
// only need this one helper.
func pathInDir(dir, name string) string { return filepath.Join(dir, name) }

var _ = pathInDir // exported helper, no-op suppression in case unused
