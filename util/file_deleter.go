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
	"io/fs"
	"strings"
)

// FileDeleterMsgType classifies messages broadcast by FileDeleter.
// Mirrors org.apache.lucene.util.FileDeleter.MsgType.
type FileDeleterMsgType int

const (
	// FileDeleterMsgRef is emitted on incRef/decRef transitions.
	FileDeleterMsgRef FileDeleterMsgType = iota
	// FileDeleterMsgFile is emitted around physical delete operations.
	FileDeleterMsgFile
)

// FileDeleterMessenger receives verbose-debug messages from a
// FileDeleter. Equivalent to Java's BiConsumer<MsgType, String>.
type FileDeleterMessenger func(typ FileDeleterMsgType, msg string)

// FileDeleterDirectory is the slice of the Directory interface that
// FileDeleter actually exercises. Keeping this small avoids dragging
// the full store package into util and lets tests substitute a
// minimal fake.
type FileDeleterDirectory interface {
	// DeleteFile removes the named entry from the directory. May
	// return fs.ErrNotExist (or anything that wraps it) when the file
	// is already gone; FileDeleter swallows that on Windows hosts to
	// tolerate the "pending delete" state.
	DeleteFile(name string) error
}

// FileDeleterRefCount tracks the reference count for a single file.
type FileDeleterRefCount struct {
	// FileName is retained for messenger output; mirrors the Java
	// counterpart, which stores it solely for assert messages.
	FileName string

	// InitDone records whether the first IncRef has happened. Used to
	// suppress the count>0-pre-increment invariant on the first
	// reference.
	InitDone bool

	// Count is the current reference count.
	Count int
}

// IncRef bumps Count and returns the new value. The first call sets
// InitDone and skips the count>0 invariant; subsequent calls
// require Count to have been positive before the increment.
func (r *FileDeleterRefCount) IncRef() int {
	if !r.InitDone {
		r.InitDone = true
	}
	r.Count++
	return r.Count
}

// DecRef decrements Count and returns the new value. Callers are
// responsible for ensuring Count was strictly positive before the
// call; the function does not panic on under-zero counts so the
// caller can attach context.
func (r *FileDeleterRefCount) DecRef() int {
	r.Count--
	return r.Count
}

// FileDeleter tracks reference counts for a set of index files and
// deletes them when their counts reach zero. It is the Go port of
// org.apache.lucene.util.FileDeleter.
//
// FileDeleter is NOT safe for concurrent use; callers must serialise
// access. This matches the Java contract.
type FileDeleter struct {
	directory FileDeleterDirectory
	messenger FileDeleterMessenger
	refCounts map[string]*FileDeleterRefCount

	// SegmentsPrefix is the filename prefix used to identify
	// segment-meta files that must be deleted before any others
	// (Lucene's two-pass delete invariant). Defaults to "segments_".
	// Callers porting from Lucene set this to IndexFileNames.SEGMENTS.
	SegmentsPrefix string

	// TolerateMissingOnDelete controls whether NoSuchFile/NotExist
	// errors are swallowed during delete. Lucene swallows them only
	// on Windows (because the OS reports "pending delete" entries in
	// directory listings); the Go port exposes this knob explicitly
	// so unit tests can pin behavior regardless of host OS.
	TolerateMissingOnDelete bool
}

// NewFileDeleter constructs a FileDeleter that delegates physical
// deletes to directory. The messenger may be nil. The
// TolerateMissingOnDelete flag defaults to true on Windows and false
// elsewhere, mirroring the Java behaviour gated by Constants.WINDOWS.
func NewFileDeleter(directory FileDeleterDirectory, messenger FileDeleterMessenger) *FileDeleter {
	return &FileDeleter{
		directory:               directory,
		messenger:               messenger,
		refCounts:               make(map[string]*FileDeleterRefCount),
		SegmentsPrefix:          "segments_",
		TolerateMissingOnDelete: IsWindows,
	}
}

func (d *FileDeleter) emit(typ FileDeleterMsgType, format string, args ...any) {
	if d.messenger == nil {
		return
	}
	d.messenger(typ, fmt.Sprintf(format, args...))
}

func (d *FileDeleter) getRefCountInternal(fileName string) *FileDeleterRefCount {
	rc, ok := d.refCounts[fileName]
	if !ok {
		rc = &FileDeleterRefCount{FileName: fileName}
		d.refCounts[fileName] = rc
	}
	return rc
}

// IncRefAll increments the reference count for every file in the slice.
func (d *FileDeleter) IncRefAll(fileNames []string) {
	for _, f := range fileNames {
		d.IncRef(f)
	}
}

// IncRef increments the reference count for a single file.
func (d *FileDeleter) IncRef(fileName string) {
	rc := d.getRefCountInternal(fileName)
	d.emit(FileDeleterMsgRef, "IncRef %q: pre-incr count is %d", fileName, rc.Count)
	rc.IncRef()
}

// DecRefAll decrements counts for all provided files and deletes
// every file whose count reached zero. Errors are collected and
// joined (errors.Join), then the first one is returned. Deletion
// continues across errors so a single failure does not strand the
// remainder of the batch.
func (d *FileDeleter) DecRefAll(fileNames []string) error {
	toDelete := make([]string, 0, len(fileNames))
	var errs []error
	for _, name := range fileNames {
		drop, err := d.decRef(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if drop {
			toDelete = append(toDelete, name)
		}
	}
	if err := d.delete(toDelete); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// decRef returns (shouldDelete, error).
func (d *FileDeleter) decRef(fileName string) (bool, error) {
	rc := d.getRefCountInternal(fileName)
	d.emit(FileDeleterMsgRef, "DecRef %q: pre-decr count is %d", fileName, rc.Count)
	if rc.Count <= 0 {
		return false, fmt.Errorf("DecRef of %q: count is %d", fileName, rc.Count)
	}
	if rc.DecRef() == 0 {
		delete(d.refCounts, fileName)
		return true, nil
	}
	return false, nil
}

// InitRefCount records a file with a zero reference count if it is
// not already tracked. Mirrors initRefCount.
func (d *FileDeleter) InitRefCount(fileName string) {
	if _, ok := d.refCounts[fileName]; !ok {
		d.refCounts[fileName] = &FileDeleterRefCount{FileName: fileName}
	}
}

// GetRefCount returns the current ref count for the file, or 0 if it
// is not tracked.
func (d *FileDeleter) GetRefCount(fileName string) int {
	rc, ok := d.refCounts[fileName]
	if !ok {
		return 0
	}
	return rc.Count
}

// AllFiles returns every tracked filename. The returned slice is a
// fresh copy; callers may modify it freely.
func (d *FileDeleter) AllFiles() []string {
	out := make([]string, 0, len(d.refCounts))
	for name := range d.refCounts {
		out = append(out, name)
	}
	return out
}

// Exists reports whether the file is tracked AND has a positive
// reference count. Mirrors Java's exists semantics.
func (d *FileDeleter) Exists(fileName string) bool {
	rc, ok := d.refCounts[fileName]
	return ok && rc.Count > 0
}

// UnrefedFiles returns every tracked file whose ref count is zero.
// Emits a FILE-typed message for each entry, mirroring Lucene's
// "removing unreferenced file" log.
func (d *FileDeleter) UnrefedFiles() []string {
	var out []string
	for name, rc := range d.refCounts {
		if rc.Count == 0 {
			d.emit(FileDeleterMsgFile, "removing unreferenced file %q", name)
			out = append(out, name)
		}
	}
	return out
}

// DeleteFilesIfNoRef deletes every file in files that is not tracked
// or has a zero ref count.
func (d *FileDeleter) DeleteFilesIfNoRef(files []string) error {
	toDelete := make([]string, 0, len(files))
	for _, name := range files {
		if !d.Exists(name) {
			d.emit(FileDeleterMsgFile, "will delete new file %q", name)
			toDelete = append(toDelete, name)
		}
	}
	return d.delete(toDelete)
}

// ForceDelete removes the tracker entry and unconditionally deletes
// the underlying file.
func (d *FileDeleter) ForceDelete(fileName string) error {
	delete(d.refCounts, fileName)
	return d.deleteOne(fileName)
}

// DeleteFileIfNoRef deletes a single file if no live ref exists for it.
func (d *FileDeleter) DeleteFileIfNoRef(fileName string) error {
	if !d.Exists(fileName) {
		d.emit(FileDeleterMsgFile, "will delete new file %q", fileName)
		return d.deleteOne(fileName)
	}
	return nil
}

// delete performs a two-pass deletion: segments_-prefixed files first,
// then everything else. This preserves Lucene's crash-safety
// invariant: removing the commit pointers before the files they
// reference means a crash never leaves a corrupt half-deleted commit.
func (d *FileDeleter) delete(toDelete []string) error {
	if len(toDelete) == 0 {
		return nil
	}
	d.emit(FileDeleterMsgFile, "now delete %d files: %v", len(toDelete), toDelete)

	var errs []error
	for _, name := range toDelete {
		if strings.HasPrefix(name, d.SegmentsPrefix) {
			if err := d.deleteOne(name); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for _, name := range toDelete {
		if !strings.HasPrefix(name, d.SegmentsPrefix) {
			if err := d.deleteOne(name); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// deleteOne dispatches a single delete to the directory and, when
// TolerateMissingOnDelete is true, swallows NotExist-like errors.
func (d *FileDeleter) deleteOne(fileName string) error {
	err := d.directory.DeleteFile(fileName)
	if err == nil {
		return nil
	}
	if d.TolerateMissingOnDelete && errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
