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

package util

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
		if err := in.ReadBytes(buf, 0, want); err != nil {
			return err
		}
		if err := out.WriteBytes(buf, 0, want); err != nil {
			return err
		}
		copied += int64(want)
	}
	return nil
}
