// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.misc.store.TestHardLinkCopyDirectoryWrapper.
package store_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/misc/store"
)

// hardlinksSupported reports whether the filesystem at dir supports hard-links.
// It attempts to create a test link and cleans it up immediately.
func hardlinksSupported(dir string) bool {
	src := filepath.Join(dir, ".hl_probe_src")
	dst := filepath.Join(dir, ".hl_probe_dst")

	f, err := os.Create(src)
	if err != nil {
		return false
	}
	f.Close()
	defer os.Remove(src)
	defer os.Remove(dst)

	err = os.Link(src, dst)
	return err == nil
}

// inodeOf returns the inode number of the file at path.
// Returns 0 and false when the inode cannot be determined.
func inodeOf(path string) (uint64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return sys.Ino, true
}

// TestHardLinkCopyDirectoryWrapper_CopyHardLinks mirrors testCopyHardLinks.
// Verifies that CopyFrom uses a hardlink when the source and destination are on
// the same filesystem, so source and destination share the same inode.
func TestHardLinkCopyDirectoryWrapper_CopyHardLinks(t *testing.T) {
	tempDir := t.TempDir()
	dir1 := filepath.Join(tempDir, "dir_1")
	dir2 := filepath.Join(tempDir, "dir_2")
	if err := os.MkdirAll(dir1, 0o755); err != nil {
		t.Fatalf("MkdirAll dir1: %v", err)
	}
	if err := os.MkdirAll(dir2, 0o755); err != nil {
		t.Fatalf("MkdirAll dir2: %v", err)
	}

	if !hardlinksSupported(tempDir) {
		t.Skip("hardlinks are not supported on this filesystem")
	}

	// Write a file into dir1.
	srcFile := filepath.Join(dir1, "foo.bar")
	if err := os.WriteFile(srcFile, []byte("hey man, nice shot!"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	wrapDir1 := store.NewFSHardlinkCopyDirectoryWrapper(dir1)
	wrapDir2 := store.NewFSHardlinkCopyDirectoryWrapper(dir2)

	if err := wrapDir2.CopyFrom(wrapDir1, "foo.bar", "bar.foo"); err != nil {
		t.Fatalf("CopyFrom: %v", err)
	}

	dstFile := filepath.Join(dir2, "bar.foo")
	if _, err := os.Stat(dstFile); err != nil {
		t.Fatalf("destination file not found: %v", err)
	}

	srcIno, srcOK := inodeOf(srcFile)
	dstIno, dstOK := inodeOf(dstFile)

	if srcOK && dstOK {
		if srcIno != dstIno {
			t.Errorf("expected hardlink (same inode): src inode=%d, dst inode=%d", srcIno, dstIno)
		}
	}

	// Verify the content is intact.
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hey man, nice shot!" {
		t.Errorf("file content mismatch: %q", string(got))
	}
}

// TestHardLinkCopyDirectoryWrapper_FallbackCopy verifies that CopyFrom produces
// a correct file copy even when the source and destination are on different
// mounts (hardlinks would fail). This exercises the byte-copy fallback path.
func TestHardLinkCopyDirectoryWrapper_FallbackCopy(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	srcFile := filepath.Join(dir1, "source.txt")
	if err := os.WriteFile(srcFile, []byte("fallback content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	wrapDir1 := store.NewFSHardlinkCopyDirectoryWrapper(dir1)
	wrapDir2 := store.NewFSHardlinkCopyDirectoryWrapper(dir2)

	if err := wrapDir2.CopyFrom(wrapDir1, "source.txt", "dest.txt"); err != nil {
		t.Fatalf("CopyFrom: %v", err)
	}

	dstFile := filepath.Join(dir2, "dest.txt")
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "fallback content" {
		t.Errorf("file content mismatch: %q", string(got))
	}
}

// TestHardLinkCopyDirectoryWrapper_RenameWithHardLink mirrors testRenameWithHardLink.
// The Java original uses WindowsFS (not available in Go test infrastructure).
// This test exercises the analogous rename-while-hardlink-is-open scenario on
// POSIX: renaming the target replaces it atomically while the hardlink remains
// accessible.
func TestHardLinkCopyDirectoryWrapper_RenameWithHardLink(t *testing.T) {
	dir := t.TempDir()
	linkDir := filepath.Join(dir, "link")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if !hardlinksSupported(dir) {
		t.Skip("hardlinks are not supported on this filesystem")
	}

	// Write the target file.
	targetPath := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(targetPath, []byte{0x00, 0x00, 0x00, 0x01}, 0o644); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}

	wrapDir := store.NewFSHardlinkCopyDirectoryWrapper(dir)
	wrapLinkDir := store.NewFSHardlinkCopyDirectoryWrapper(linkDir)

	// Copy via hardlink: target.txt -> link/link.txt
	if err := wrapLinkDir.CopyFrom(wrapDir, "target.txt", "link.txt"); err != nil {
		t.Fatalf("CopyFrom: %v", err)
	}

	// Open the hardlink.
	linkFile := filepath.Join(linkDir, "link.txt")
	f, err := os.Open(linkFile)
	if err != nil {
		t.Fatalf("Open hardlink: %v", err)
	}

	// Write a new source and rename it over target.txt while the hardlink is open.
	sourcePath := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(sourcePath, []byte{0x00, 0x00, 0x00, 0x02}, 0o644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	if err := os.Rename(sourcePath, targetPath); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// The hardlink (link.txt) still points to the original inode.
	f.Close()

	// target.txt now contains the new content (value 2).
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) < 4 || got[3] != 0x02 {
		t.Errorf("expected target.txt to hold new content after rename, got %v", got)
	}
}
