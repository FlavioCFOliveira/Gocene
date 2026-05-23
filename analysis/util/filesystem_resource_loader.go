// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// ResourceLoader opens named resources and returns them as io.ReadCloser.
//
// Go analogue of org.apache.lucene.util.ResourceLoader.
type ResourceLoader interface {
	// OpenResource opens the named resource for reading.
	OpenResource(name string) (io.ReadCloser, error)
}

// FilesystemResourceLoader resolves named resources against a base directory
// on the local filesystem. Resources not found in the base directory are
// delegated to a fallback ResourceLoader.
//
// Go port of org.apache.lucene.analysis.util.FilesystemResourceLoader
// (Apache Lucene 10.4.0).
type FilesystemResourceLoader struct {
	baseDir  string
	delegate ResourceLoader
}

// NewFilesystemResourceLoader creates a loader that resolves resources against
// baseDir. Resources not found there are forwarded to delegate (may be nil to
// raise an error on miss).
func NewFilesystemResourceLoader(baseDir string, delegate ResourceLoader) (*FilesystemResourceLoader, error) {
	info, err := os.Stat(baseDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("FilesystemResourceLoader: " + baseDir + " is not a directory")
	}
	return &FilesystemResourceLoader{baseDir: baseDir, delegate: delegate}, nil
}

// OpenResource opens name relative to the base directory. If the file does
// not exist and a delegate was provided, the delegate is tried.
func (l *FilesystemResourceLoader) OpenResource(name string) (io.ReadCloser, error) {
	path := filepath.Join(l.baseDir, name)
	f, err := os.Open(path)
	if err == nil {
		return f, nil
	}
	if os.IsNotExist(err) && l.delegate != nil {
		return l.delegate.OpenResource(name)
	}
	return nil, err
}

// Ensure FilesystemResourceLoader implements ResourceLoader.
var _ ResourceLoader = (*FilesystemResourceLoader)(nil)
