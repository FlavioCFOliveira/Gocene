// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// DeleteFilesIgnoringExceptions deletes all given files from dir, suppressing
// every failure. Mirrors
// org.apache.lucene.util.IOUtils.deleteFilesIgnoringExceptions(Directory,
// String...) of Apache Lucene 10.5.0, which calls dir.deleteFile(name) for
// each name and catches and ignores every Throwable.
//
// The overload lives in store rather than util, where the rest of IOUtils is
// ported, because it names Directory and util cannot import store or spi
// (spi imports util).
func DeleteFilesIgnoringExceptions(dir Directory, files ...string) {
	for _, name := range files {
		// Java: try { dir.deleteFile(name); } catch (Throwable ignored) { }
		if err := dir.DeleteFile(name); err != nil {
			continue
		}
	}
}
