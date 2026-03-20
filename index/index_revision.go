package index

import (
	"fmt"
	"time"
)

// IndexRevision represents a point-in-time snapshot of the index for replication.
// It contains metadata about the index state at a specific generation.
type IndexRevision struct {
	// Generation is the commit generation number
	Generation int64

	// Version is the index version number
	Version int64

	// Timestamp is when this revision was created
	Timestamp time.Time

	// Files is the list of files in this revision
	Files []string

	// SegmentInfos contains segment information
	SegmentInfos *SegmentInfos

	// Metadata contains additional revision metadata
	Metadata map[string]string
}

// NewIndexRevision creates a new IndexRevision with the given parameters.
func NewIndexRevision(generation, version int64, files []string) *IndexRevision {
	return &IndexRevision{
		Generation: generation,
		Version:    version,
		Timestamp:  time.Now(),
		Files:      files,
		Metadata:   make(map[string]string),
	}
}

// Clone creates a deep copy of the IndexRevision.
func (r *IndexRevision) Clone() *IndexRevision {
	if r == nil {
		return nil
	}

	cloned := &IndexRevision{
		Generation: r.Generation,
		Version:    r.Version,
		Timestamp:  r.Timestamp,
		Metadata:   make(map[string]string),
	}

	// Deep copy files slice
	if r.Files != nil {
		cloned.Files = make([]string, len(r.Files))
		copy(cloned.Files, r.Files)
	}

	// Deep copy metadata map
	for k, v := range r.Metadata {
		cloned.Metadata[k] = v
	}

	return cloned
}

// Equals compares two IndexRevisions for equality.
func (r *IndexRevision) Equals(other *IndexRevision) bool {
	if r == nil || other == nil {
		return r == other
	}

	if r.Generation != other.Generation {
		return false
	}

	if r.Version != other.Version {
		return false
	}

	if len(r.Files) != len(other.Files) {
		return false
	}

	for i, file := range r.Files {
		if file != other.Files[i] {
			return false
		}
	}

	return true
}

// GetFileCount returns the number of files in this revision.
func (r *IndexRevision) GetFileCount() int {
	if r == nil {
		return 0
	}
	return len(r.Files)
}

// HasFile returns true if the revision contains the given file.
func (r *IndexRevision) HasFile(filename string) bool {
	if r == nil {
		return false
	}
	for _, file := range r.Files {
		if file == filename {
			return true
		}
	}
	return false
}

// AddFile adds a file to the revision.
func (r *IndexRevision) AddFile(filename string) {
	if r == nil {
		return
	}
	r.Files = append(r.Files, filename)
}

// RemoveFile removes a file from the revision.
func (r *IndexRevision) RemoveFile(filename string) bool {
	if r == nil {
		return false
	}
	for i, file := range r.Files {
		if file == filename {
			r.Files = append(r.Files[:i], r.Files[i+1:]...)
			return true
		}
	}
	return false
}

// SetMetadata sets a metadata key-value pair.
func (r *IndexRevision) SetMetadata(key, value string) {
	if r == nil {
		return
	}
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata[key] = value
}

// GetMetadata returns a metadata value by key.
func (r *IndexRevision) GetMetadata(key string) string {
	if r == nil || r.Metadata == nil {
		return ""
	}
	return r.Metadata[key]
}

// IsNewerThan returns true if this revision is newer than the other.
func (r *IndexRevision) IsNewerThan(other *IndexRevision) bool {
	if r == nil {
		return false
	}
	if other == nil {
		return true
	}
	return r.Generation > other.Generation
}

// GetAge returns the age of this revision.
func (r *IndexRevision) GetAge() time.Duration {
	if r == nil {
		return 0
	}
	return time.Since(r.Timestamp)
}

// String returns a string representation of the IndexRevision.
func (r *IndexRevision) String() string {
	if r == nil {
		return "IndexRevision(nil)"
	}
	return fmt.Sprintf("IndexRevision{gen=%d, ver=%d, files=%d}",
		r.Generation, r.Version, len(r.Files))
}
