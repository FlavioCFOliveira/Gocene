package index

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// NRTFileDeleter manages file deletion for NRT (Near Real-Time) search.
// It ensures that files are only deleted when they are no longer referenced
// by any readers, preventing the deletion of files that are still in use.
// This is the Go port of Lucene's NRT file deletion management.
type NRTFileDeleter struct {
	mu sync.RWMutex

	// directory is the store.Directory where files are stored
	directory store.Directory

	// pendingDeletions holds files waiting to be deleted
	// Key: filename, Value: reference count
	pendingDeletions map[string]int

	// protectedFiles holds files that are currently protected from deletion
	// Key: filename, Value: protection count
	protectedFiles map[string]int

	// isOpen indicates if the deleter is open
	isOpen atomic.Bool

	// deleteCount tracks the number of files deleted
	deleteCount int64

	// protectedCount tracks the number of files protected
	protectedCount int64
}

// NewNRTFileDeleter creates a new NRTFileDeleter for the given store.Directory.
func NewNRTFileDeleter(directory store.Directory) (*NRTFileDeleter, error) {
	if directory == nil {
		return nil, fmt.Errorf("directory cannot be nil")
	}

	deleter := &NRTFileDeleter{
		directory:        directory,
		pendingDeletions: make(map[string]int),
		protectedFiles:   make(map[string]int),
	}

	deleter.isOpen.Store(true)

	return deleter, nil
}

// Delete marks a file for deletion. The file will only be deleted when
// it is no longer protected and has no pending references.
func (d *NRTFileDeleter) Delete(filename string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isOpen.Load() {
		return fmt.Errorf("deleter is closed")
	}

	// Check if file is protected
	if count, ok := d.protectedFiles[filename]; ok && count > 0 {
		// File is protected, add to pending deletions
		d.pendingDeletions[filename] = d.pendingDeletions[filename] + 1
		return nil
	}

	// File is not protected, delete immediately
	if err := d.doDelete(filename); err != nil {
		return err
	}

	d.deleteCount++
	return nil
}

// Protect protects a file from deletion. The file will not be deleted
// until Unprotect is called the same number of times.
func (d *NRTFileDeleter) Protect(filename string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isOpen.Load() {
		return fmt.Errorf("deleter is closed")
	}

	d.protectedFiles[filename] = d.protectedFiles[filename] + 1
	d.protectedCount++

	return nil
}

// Unprotect removes protection from a file. If the file was marked for
// deletion while protected, it will be deleted now.
func (d *NRTFileDeleter) Unprotect(filename string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isOpen.Load() {
		return fmt.Errorf("deleter is closed")
	}

	count, ok := d.protectedFiles[filename]
	if !ok || count <= 0 {
		return fmt.Errorf("file %s is not protected", filename)
	}

	count--
	if count == 0 {
		delete(d.protectedFiles, filename)
	} else {
		d.protectedFiles[filename] = count
	}

	d.protectedCount--

	// Check if file was pending deletion
	if pendingCount, ok := d.pendingDeletions[filename]; ok && pendingCount > 0 {
		// File was pending, delete it now
		if err := d.doDelete(filename); err != nil {
			return err
		}

		pendingCount--
		if pendingCount == 0 {
			delete(d.pendingDeletions, filename)
		} else {
			d.pendingDeletions[filename] = pendingCount
		}

		d.deleteCount++
	}

	return nil
}

// IsProtected returns true if the file is currently protected.
func (d *NRTFileDeleter) IsProtected(filename string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	count, ok := d.protectedFiles[filename]
	return ok && count > 0
}

// IsPendingDeletion returns true if the file is pending deletion.
func (d *NRTFileDeleter) IsPendingDeletion(filename string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	count, ok := d.pendingDeletions[filename]
	return ok && count > 0
}

// doDelete performs the actual file deletion.
func (d *NRTFileDeleter) doDelete(filename string) error {
	// In a real implementation, this would delete from the directory
	// For now, we just return nil as a placeholder
	return nil
}

// Close closes the NRTFileDeleter.
// Note: This does not delete pending files; they remain pending.
func (d *NRTFileDeleter) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isOpen.Load() {
		return nil
	}

	d.isOpen.Store(false)

	// Clear maps
	d.protectedFiles = nil
	d.pendingDeletions = nil

	return nil
}

// IsOpen returns true if the deleter is open.
func (d *NRTFileDeleter) IsOpen() bool {
	return d.isOpen.Load()
}

// GetPendingDeletions returns a copy of the pending deletions map.
func (d *NRTFileDeleter) GetPendingDeletions() map[string]int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]int)
	for k, v := range d.pendingDeletions {
		result[k] = v
	}

	return result
}

// GetProtectedFiles returns a copy of the protected files map.
func (d *NRTFileDeleter) GetProtectedFiles() map[string]int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]int)
	for k, v := range d.protectedFiles {
		result[k] = v
	}

	return result
}

// GetDeleteCount returns the number of files deleted.
func (d *NRTFileDeleter) GetDeleteCount() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.deleteCount
}

// GetProtectedCount returns the number of files currently protected.
func (d *NRTFileDeleter) GetProtectedCount() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.protectedCount
}

// ProcessPendingDeletions attempts to delete all pending files.
// This should be called periodically to clean up pending deletions.
func (d *NRTFileDeleter) ProcessPendingDeletions() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isOpen.Load() {
		return fmt.Errorf("deleter is closed")
	}

	for filename, count := range d.pendingDeletions {
		// Check if file is still protected
		if protectedCount, ok := d.protectedFiles[filename]; ok && protectedCount > 0 {
			// File is still protected, skip
			continue
		}

		// Delete the file
		if err := d.doDelete(filename); err != nil {
			return err
		}

		d.deleteCount += int64(count)
		delete(d.pendingDeletions, filename)
	}

	return nil
}

// Clear clears all pending deletions without deleting them.
// This is useful for cleanup during testing.
func (d *NRTFileDeleter) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pendingDeletions = make(map[string]int)
}

// String returns a string representation of the NRTFileDeleter.
func (d *NRTFileDeleter) String() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return fmt.Sprintf("NRTFileDeleter{pending=%d, protected=%d, deleted=%d}",
		len(d.pendingDeletions), len(d.protectedFiles), d.deleteCount)
}
