// Package index provides core index functionality for Gocene.
// This file implements the PersistentSnapshotDeletionPolicy for persistent snapshots.
// Source: org.apache.lucene.index.PersistentSnapshotDeletionPolicy (Apache Lucene 10.x)
package index

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// PersistentSnapshotDeletionPolicy extends SnapshotDeletionPolicy to persist
// snapshots to a directory, allowing recovery after a crash or restart.
//
// Each snapshot() or release() operation is immediately persisted to disk.
// This ensures that snapshots survive application restarts.
//
// IMPORTANT: Each IndexWriter must have its own instance of this policy
// to avoid corruption.
//
// This is the Go port of Lucene's
// org.apache.lucene.index.PersistentSnapshotDeletionPolicy.
type PersistentSnapshotDeletionPolicy struct {
	*SnapshotDeletionPolicy

	// dir is the directory where snapshots are persisted
	dir store.Directory

	// infoStream for logging
	infoStream io.Writer

	// lastSaveFile is the filename of the last saved snapshots file
	lastSaveFile string

	// mu protects the lastSaveFile
	mu sync.RWMutex

	// pendingSnapshots holds generations loaded from disk that need to be applied
	pendingSnapshots []int64
}

// snapshotFilePrefix is the prefix for snapshot files
const snapshotFilePrefix = "snapshots-"

// NewPersistentSnapshotDeletionPolicy creates a new PersistentSnapshotDeletionPolicy.
//
// Parameters:
//   - primary: The primary deletion policy to wrap. If nil, KeepAllDeletionPolicy is used.
//   - dir: The directory where snapshots will be persisted.
//
// Returns:
//   - A new PersistentSnapshotDeletionPolicy instance
//   - An error if the snapshots cannot be loaded
//
// Example:
//
//	policy, err := NewPersistentSnapshotDeletionPolicy(nil, dir)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer policy.ReleaseAll()
func NewPersistentSnapshotDeletionPolicy(primary IndexDeletionPolicy, dir store.Directory) (*PersistentSnapshotDeletionPolicy, error) {
	if primary == nil {
		primary = NewKeepAllDeletionPolicy()
	}

	psdp := &PersistentSnapshotDeletionPolicy{
		SnapshotDeletionPolicy: NewSnapshotDeletionPolicy(primary),
		dir:                    dir,
		infoStream:             io.Discard,
	}

	// Load existing snapshots from disk
	if err := psdp.loadSnapshots(); err != nil {
		return nil, fmt.Errorf("cannot load snapshots: %w", err)
	}

	return psdp, nil
}

// SetInfoStream sets the output stream for logging.
func (psdp *PersistentSnapshotDeletionPolicy) SetInfoStream(out io.Writer) {
	psdp.mu.Lock()
	defer psdp.mu.Unlock()
	psdp.infoStream = out
}

// msg prints a message to the info stream.
func (psdp *PersistentSnapshotDeletionPolicy) msg(message string) {
	if psdp.infoStream != nil {
		fmt.Fprintln(psdp.infoStream, message)
	}
}

// msgf prints a formatted message to the info stream.
func (psdp *PersistentSnapshotDeletionPolicy) msgf(format string, args ...interface{}) {
	if psdp.infoStream != nil {
		fmt.Fprintf(psdp.infoStream, format+"\n", args...)
	}
}

// Snapshot snapshots the last commit and persists the snapshot information to
// the directory before returning. Mirrors
// PersistentSnapshotDeletionPolicy.snapshot.
func (psdp *PersistentSnapshotDeletionPolicy) Snapshot() (Commit, error) {
	ic, err := psdp.SnapshotDeletionPolicy.Snapshot()
	if err != nil {
		return nil, err
	}

	// Persist the snapshot. On failure roll the reference back, mirroring the
	// finally block in Java, which releases the commit again so the original
	// exception is the one that propagates.
	if err := psdp.saveSnapshots(); err != nil {
		if relErr := psdp.SnapshotDeletionPolicy.Release(ic); relErr != nil {
			psdp.msgf("Warning: cannot roll back snapshot %d after failed persist: %v",
				ic.GetGeneration(), relErr)
		}
		return nil, fmt.Errorf("cannot persist snapshot: %w", err)
	}

	psdp.msgf("Snapshot %d persisted to disk", ic.GetGeneration())
	return ic, nil
}

// SnapshotGeneration snapshots the commit carrying the given generation and
// persists the snapshot information to the directory.
//
// PORT NOTE: Java has no such method. It restores snapshots read back from
// disk by repopulating refCounts inside loadPriorSnapshots, and lets
// SnapshotDeletionPolicy.onInit re-associate each generation with its commit.
// Gocene keeps the generations aside until OnInit hands it the commit list,
// which is what this method consumes.
func (psdp *PersistentSnapshotDeletionPolicy) SnapshotGeneration(commits []Commit, generation int64) error {
	if err := psdp.pinGeneration(commits, generation); err != nil {
		return err
	}

	// Persist the snapshot
	if err := psdp.saveSnapshots(); err != nil {
		// Try to release the snapshot we just created
		if relErr := psdp.SnapshotDeletionPolicy.ReleaseGen(generation); relErr != nil {
			psdp.msgf("Warning: cannot roll back snapshot %d after failed persist: %v",
				generation, relErr)
		}
		return fmt.Errorf("cannot persist snapshot: %w", err)
	}

	psdp.msgf("Snapshot %d persisted to disk", generation)
	return nil
}

// pinGeneration takes a reference on the commit whose generation matches,
// which is how a snapshot read back from disk is re-established.
func (psdp *PersistentSnapshotDeletionPolicy) pinGeneration(commits []Commit, generation int64) error {
	sdp := psdp.SnapshotDeletionPolicy
	sdp.mu.Lock()
	defer sdp.mu.Unlock()

	for _, commit := range commits {
		if commit.GetGeneration() != generation {
			continue
		}
		sdp.refCounts[generation]++
		sdp.indexCommits[generation] = commit
		return nil
	}
	return fmt.Errorf("commit gen=%d is not present in this index", generation)
}

// Release releases a snapshotted commit and persists the change. Mirrors
// PersistentSnapshotDeletionPolicy.release(IndexCommit).
func (psdp *PersistentSnapshotDeletionPolicy) Release(commit Commit) error {
	if err := psdp.SnapshotDeletionPolicy.Release(commit); err != nil {
		return err
	}

	// Persist the change. On failure re-take the reference, mirroring the
	// finally block in Java, which calls incRef again.
	if err := psdp.saveSnapshots(); err != nil {
		psdp.reIncRef(commit)
		return fmt.Errorf("cannot persist release of snapshot %d: %w", commit.GetGeneration(), err)
	}

	psdp.msgf("Released snapshot %d persisted to disk", commit.GetGeneration())
	return nil
}

// ReleaseGen releases a snapshotted commit by generation and persists the
// change. Mirrors PersistentSnapshotDeletionPolicy.release(long).
func (psdp *PersistentSnapshotDeletionPolicy) ReleaseGen(generation int64) error {
	if err := psdp.SnapshotDeletionPolicy.ReleaseGen(generation); err != nil {
		return err
	}
	if err := psdp.saveSnapshots(); err != nil {
		return fmt.Errorf("cannot persist release of snapshot %d: %w", generation, err)
	}
	psdp.msgf("Released snapshot %d persisted to disk", generation)
	return nil
}

// reIncRef re-takes the reference dropped by a release whose persist failed.
// Mirrors the incRef call in the finally block of
// PersistentSnapshotDeletionPolicy.release.
func (psdp *PersistentSnapshotDeletionPolicy) reIncRef(commit Commit) {
	sdp := psdp.SnapshotDeletionPolicy
	sdp.mu.Lock()
	defer sdp.mu.Unlock()
	sdp.incRef(commit)
}

// ReleaseAll releases every snapshot this policy holds and persists the change.
//
// PORT NOTE: Java has no releaseAll; it is a Gocene convenience built out of
// the released-one-at-a-time primitives.
func (psdp *PersistentSnapshotDeletionPolicy) ReleaseAll() {
	for _, commit := range psdp.SnapshotDeletionPolicy.GetSnapshots() {
		// A generation may hold more than one reference; drop them all.
		gen := commit.GetGeneration()
		for psdp.SnapshotDeletionPolicy.GetIndexCommit(gen) != nil {
			if err := psdp.SnapshotDeletionPolicy.ReleaseGen(gen); err != nil {
				psdp.msgf("Warning: cannot release snapshot %d: %v", gen, err)
				break
			}
		}
	}

	// Persist the change
	if err := psdp.saveSnapshots(); err != nil {
		psdp.msgf("Warning: cannot persist release of all snapshots: %v", err)
	} else {
		psdp.msg("Released all snapshots persisted to disk")
	}
}

// GetLastSaveFile returns the filename of the last saved snapshots file.
func (psdp *PersistentSnapshotDeletionPolicy) GetLastSaveFile() string {
	psdp.mu.RLock()
	defer psdp.mu.RUnlock()
	return psdp.lastSaveFile
}

// saveSnapshots persists the current snapshots to disk.
func (psdp *PersistentSnapshotDeletionPolicy) saveSnapshots() error {
	psdp.mu.Lock()
	defer psdp.mu.Unlock()

	// Get current snapshots
	snapshots := psdp.SnapshotDeletionPolicy.GetSnapshots()

	// Create a new snapshots file
	// Use a timestamp to ensure uniqueness
	timestamp := getCurrentTimestamp()
	filename := fmt.Sprintf("%s%d.txt", snapshotFilePrefix, timestamp)

	// Create the file
	out, err := psdp.dir.CreateOutput(filename, store.IOContextWrite)
	if err != nil {
		return fmt.Errorf("cannot create snapshots file: %w", err)
	}
	defer out.Close()

	// Build content
	var content strings.Builder
	for _, snapshot := range snapshots {
		fmt.Fprintf(&content, "%d\n", snapshot.GetGeneration())
	}

	// Write content
	data := []byte(content.String())
	if err := out.WriteBytes(data, 0, len(data)); err != nil {
		return fmt.Errorf("cannot write snapshots: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("cannot close output: %w", err)
	}

	// Delete the old snapshots file if any
	if psdp.lastSaveFile != "" {
		if err := psdp.dir.DeleteFile(psdp.lastSaveFile); err != nil {
			// Log but don't fail - the new file is already saved
			psdp.msgf("Warning: cannot delete old snapshots file %s: %v", psdp.lastSaveFile, err)
		}
	}

	psdp.lastSaveFile = filename
	return nil
}

// loadSnapshots loads snapshots from disk.
func (psdp *PersistentSnapshotDeletionPolicy) loadSnapshots() error {
	psdp.mu.Lock()
	defer psdp.mu.Unlock()

	// List all files in the directory
	files, err := psdp.dir.ListAll()
	if err != nil {
		return fmt.Errorf("cannot list directory: %w", err)
	}

	// Find the most recent snapshots file
	var latestFile string
	var latestTimestamp int64 = -1

	for _, file := range files {
		if strings.HasPrefix(file, snapshotFilePrefix) {
			// Extract timestamp from filename
			timestampStr := strings.TrimPrefix(file, snapshotFilePrefix)
			timestampStr = strings.TrimSuffix(timestampStr, ".txt")
			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				// Skip files with invalid timestamps
				continue
			}
			if timestamp > latestTimestamp {
				latestTimestamp = timestamp
				latestFile = file
			}
		}
	}

	if latestFile == "" {
		// No snapshots file found - this is OK for a new index
		return nil
	}

	// Read the snapshots file
	in, err := psdp.dir.OpenInput(latestFile, store.IOContextRead)
	if err != nil {
		return fmt.Errorf("cannot open snapshots file: %w", err)
	}
	defer in.Close()

	// Read all data
	data := make([]byte, in.Length())
	if err := in.ReadBytes(data, 0, len(data)); err != nil {
		return fmt.Errorf("cannot read snapshots file: %w", err)
	}
	in.Close()

	// Parse snapshots
	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		gen, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			psdp.msgf("Warning: invalid generation in snapshots file: %s", line)
			continue
		}

		// Store for later application
		psdp.pendingSnapshots = append(psdp.pendingSnapshots, gen)
	}

	psdp.lastSaveFile = latestFile
	psdp.msgf("Loaded %d snapshots from %s", len(psdp.pendingSnapshots), latestFile)

	return nil
}

// getCurrentTimestamp returns the current timestamp in milliseconds.
func getCurrentTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// OnInit is called when IndexWriter is initialized.
// It applies any pending snapshots loaded from disk.
func (psdp *PersistentSnapshotDeletionPolicy) OnInit(commits []Commit) error {
	// Call parent's OnInit
	if err := psdp.SnapshotDeletionPolicy.OnInit(commits); err != nil {
		return err
	}

	// Apply pending snapshots
	if len(psdp.pendingSnapshots) > 0 && len(commits) > 0 {
		psdp.msgf("Applying %d pending snapshots", len(psdp.pendingSnapshots))
		for _, gen := range psdp.pendingSnapshots {
			if err := psdp.pinGeneration(commits, gen); err != nil {
				psdp.msgf("Warning: cannot apply pending snapshot %d: %v", gen, err)
			}
		}
		psdp.pendingSnapshots = nil
	}

	return nil
}

// Clone returns a clone of this policy.
// Note: The clone will share the same directory but will have its own
// snapshot tracking. This should be used with caution.
func (psdp *PersistentSnapshotDeletionPolicy) Clone() IndexDeletionPolicy {
	// Clone the primary policy. SnapshotDeletionPolicy.primary is assigned
	// once at construction and never mutated, so it is read directly here
	// (the two types live in the same package).
	primaryClone := psdp.SnapshotDeletionPolicy.primary.Clone()

	// Create a new instance
	clone := &PersistentSnapshotDeletionPolicy{
		SnapshotDeletionPolicy: NewSnapshotDeletionPolicy(primaryClone),
		dir:                    psdp.dir,
		infoStream:             psdp.infoStream,
		lastSaveFile:           psdp.lastSaveFile,
		pendingSnapshots:       append([]int64(nil), psdp.pendingSnapshots...),
	}

	return clone
}

// String returns a string representation of this policy.
func (psdp *PersistentSnapshotDeletionPolicy) String() string {
	return fmt.Sprintf("PersistentSnapshotDeletionPolicy(primary=%v, snapshotCount=%d, dir=%v)",
		psdp.SnapshotDeletionPolicy.primary,
		psdp.SnapshotDeletionPolicy.GetSnapshotCount(),
		psdp.dir)
}
