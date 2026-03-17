// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ResourceLoader provides functionality for loading resources from various sources.
// This is the Go port of resource loading functionality from Lucene's IOUtils.

// ResourceAsStream loads a resource as an io.ReadCloser.
// It attempts to load from the following sources in order:
// 1. File system (if name is an absolute or relative path)
// 2. Embedded resources (if an embed.FS is provided)
// 3. Returns an error if the resource cannot be found
//
// The caller is responsible for closing the returned ReadCloser.
func ResourceAsStream(name string) (io.ReadCloser, error) {
	return ResourceAsStreamWithEmbed(nil, name)
}

// ResourceAsStreamWithEmbed loads a resource as an io.ReadCloser with an embed.FS.
// It attempts to load from the following sources in order:
// 1. File system (if name is an absolute or relative path)
// 2. Embedded resources (if embedFS is not nil)
// 3. Returns an error if the resource cannot be found
//
// The caller is responsible for closing the returned ReadCloser.
func ResourceAsStreamWithEmbed(embedFS *embed.FS, name string) (io.ReadCloser, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}

	// Try file system first
	if _, err := os.Stat(name); err == nil {
		// File exists on filesystem
		return os.Open(name)
	}

	// Try embedded resources if provided
	if embedFS != nil {
		if f, err := embedFS.Open(name); err == nil {
			return f, nil
		}
	}

	return nil, fmt.Errorf("resource not found: %s", name)
}

// ResourceAsBytes loads a resource and returns its contents as a byte slice.
func ResourceAsBytes(name string) ([]byte, error) {
	return ResourceAsBytesWithEmbed(nil, name)
}

// ResourceAsBytesWithEmbed loads a resource with embed.FS and returns its contents as a byte slice.
func ResourceAsBytesWithEmbed(embedFS *embed.FS, name string) ([]byte, error) {
	rc, err := ResourceAsStreamWithEmbed(embedFS, name)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// ResourceAsString loads a resource and returns its contents as a string.
func ResourceAsString(name string) (string, error) {
	return ResourceAsStringWithEmbed(nil, name)
}

// ResourceAsStringWithEmbed loads a resource with embed.FS and returns its contents as a string.
func ResourceAsStringWithEmbed(embedFS *embed.FS, name string) (string, error) {
	bytes, err := ResourceAsBytesWithEmbed(embedFS, name)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ResourceExists checks if a resource exists.
func ResourceExists(name string) bool {
	return ResourceExistsWithEmbed(nil, name)
}

// ResourceExistsWithEmbed checks if a resource exists with embed.FS.
func ResourceExistsWithEmbed(embedFS *embed.FS, name string) bool {
	// Check file system
	if _, err := os.Stat(name); err == nil {
		return true
	}

	// Check embedded resources
	if embedFS != nil {
		if _, err := embedFS.Open(name); err == nil {
			return true
		}
	}

	return false
}

// ListResources lists all resources in a directory.
// If embedFS is nil, only filesystem resources are listed.
func ListResources(dir string, embedFS *embed.FS) ([]string, error) {
	var results []string

	// List from filesystem
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			results = append(results, filepath.Join(dir, entry.Name()))
		}
	}

	// List from embedded resources
	if embedFS != nil {
		if entries, err := embedFS.ReadDir(dir); err == nil {
			for _, entry := range entries {
				results = append(results, filepath.Join(dir, entry.Name()))
			}
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("directory not found: %s", dir)
	}

	return results, nil
}
