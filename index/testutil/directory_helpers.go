// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// RamCopyOf copies all index files from dir into a fresh in-memory directory,
// mirroring org.apache.lucene.tests.util.TestUtil.ramCopyOf.
//
// Only files that belong to the index commit state are copied: files whose
// names start with the segments prefix, plus files that match the codec file
// pattern (segment data files). Other files (e.g. lock files or transient
// pending_segments) are skipped, matching the upstream behavior.
func RamCopyOf(dir store.Directory) (store.Directory, error) {
	ram := store.NewByteBuffersDirectory()
	files, err := dir.ListAll()
	if err != nil {
		_ = ram.Close()
		return nil, fmt.Errorf("RamCopyOf: list source directory: %w", err)
	}
	for _, file := range files {
		if !shouldCopyFile(file) {
			continue
		}
		if err := copyFile(dir, ram, file); err != nil {
			_ = ram.Close()
			return nil, fmt.Errorf("RamCopyOf: copy %q: %w", file, err)
		}
	}
	return ram, nil
}

// shouldCopyFile decides whether a file is part of the committed index state
// and should be carried over by RamCopyOf.
func shouldCopyFile(name string) bool {
	if strings.HasPrefix(name, index.SegmentsPrefix) {
		return true
	}
	return index.CodecFilePattern.MatchString(name)
}

// copyFile copies a single file from src to dst by streaming bytes through a
// bounded buffer. The file must not already exist in dst.
func copyFile(src, dst store.Directory, name string) error {
	in, err := src.OpenInput(name, store.IOContextDefault)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := dst.CreateOutput(name, store.IOContextDefault)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	length := in.Length()
	const bufSize = 8192
	buf := make([]byte, bufSize)
	var copied int64
	for copied < length {
		want := bufSize
		if remaining := length - copied; int64(want) > remaining {
			want = int(remaining)
		}
		if err := in.ReadBytes(buf[:want]); err != nil {
			return err
		}
		if err := out.WriteBytes(buf[:want]); err != nil {
			return err
		}
		copied += int64(want)
	}
	return nil
}

// WrapDirectory wraps the supplied directory in a MockDirectoryWrapper so that
// tests can enable failure injection, open-file tracking and disk-usage
// assertions. It is the Gocene equivalent of the wrappers returned by Lucene's
// LuceneTestCase.wrapDirectory.
func WrapDirectory(dir store.Directory) *store.MockDirectoryWrapper {
	return store.NewMockDirectoryWrapper(dir)
}

// GetOnlyLeafReader returns the single segment reader of a DirectoryReader.
// It panics if the reader does not have exactly one leaf. This mirrors the
// LuceneTestCase helper used by many index tests.
func GetOnlyLeafReader(reader *index.DirectoryReader) *index.SegmentReader {
	leaves := reader.GetSegmentReaders()
	if len(leaves) != 1 {
		panic(fmt.Sprintf("GetOnlyLeafReader: expected exactly one leaf, got %d", len(leaves)))
	}
	return leaves[0]
}
