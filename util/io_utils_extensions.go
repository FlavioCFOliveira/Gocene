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

package util

import (
	"errors"
	"fmt"
	"os"
)

// This file closes API gaps in IOUtils relative to Lucene 10.4.0's
// public surface (UseOrSuppress, FSyncAt). The pre-existing
// io_utils.go covers the close-*, deleteFiles*, applyToAll and FSync
// surfaces.

// UseOrSuppress merges two errors in the style of Lucene's
// IOUtils.useOrSuppress: when both errors are non-nil, primary is kept
// as the main error and other is recorded as a suppressed sibling
// (joined via errors.Join so both remain inspectable with errors.Is).
// When only one is non-nil it is returned as-is; when both are nil the
// result is nil. The Go equivalent uses errors.Join so that callers
// can use errors.Is on either argument.
func UseOrSuppress(primary, other error) error {
	if primary == nil {
		return other
	}
	if other == nil {
		return primary
	}
	return errors.Join(primary, other)
}

// FSyncAt fsyncs a regular file (isDir == false) or a directory
// (isDir == true), mirroring Lucene's IOUtils.fsync(Path, boolean) one-
// shot signature. The directory-fsync variant returns nil silently on
// platforms where directory syncing is not supported.
func FSyncAt(path string, isDir bool) error {
	if isDir {
		return FSyncDirectory(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("fsync %s: %w", path, err)
	}
	defer f.Close()
	return f.Sync()
}
