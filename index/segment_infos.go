// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfos manages a collection of SegmentCommitInfo representing a point-in-time
// view of the index segments.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentInfos.
//
// SegmentInfos maintains the list of segments in the index and handles generation-based
// file naming for the segments file (e.g., segments_1, segments_2, etc.).
type SegmentInfos struct {
	// segments is the list of SegmentCommitInfo in order of document IDs
	segments SegmentCommitInfoList

	// generation is the current generation number for the segments file
	// This is used to create the segments file name (segments_generation)
	generation int64

	// lastGeneration is the last committed generation
	// Used when opening an existing index
	lastGeneration int64

	// version counts how many times the index has been changed
	version int64

	// luceneVersion is the Lucene version that created this SegmentInfos
	luceneVersion string

	// indexCreatedVersionMajor is the major version of Lucene that created this index
	indexCreatedVersionMajor int32

	// counter is used to generate new segment names
	counter int64

	// userData holds optional user-supplied commit data
	userData map[string]string

	// inMemoryParentField records the parentField from IndexWriterConfig at the
	// time of the last Commit.  Not serialised; used by AddIndexes validation.
	inMemoryParentField string

	// inMemoryIndexSort records the indexSort from IndexWriterConfig at the time
	// of the last Commit.  Not serialised; used by AddIndexes validation.
	inMemoryIndexSort *Sort

	// mu protects mutable fields
	mu sync.RWMutex
}

// Default index version
const defaultIndexVersion = "10.0.0"

// NewSegmentInfos creates a new empty SegmentInfos.
func NewSegmentInfos() *SegmentInfos {
	return &SegmentInfos{
		segments:                 make(SegmentCommitInfoList, 0),
		generation:               1,
		lastGeneration:           0,
		version:                  0,
		luceneVersion:            defaultIndexVersion,
		indexCreatedVersionMajor: 10,
		counter:                  0,
		userData:                 make(map[string]string),
	}
}

// Size returns the number of segments in this SegmentInfos.
func (si *SegmentInfos) Size() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return len(si.segments)
}

// Get returns the SegmentCommitInfo at the given index.
// Returns nil if index is out of bounds.
func (si *SegmentInfos) Get(index int) *SegmentCommitInfo {
	si.mu.RLock()
	defer si.mu.RUnlock()
	if index < 0 || index >= len(si.segments) {
		return nil
	}
	return si.segments[index]
}

// Add adds a new SegmentCommitInfo to the end of the list.
func (si *SegmentInfos) Add(sci *SegmentCommitInfo) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.segments = append(si.segments, sci)
}

// Insert inserts a SegmentCommitInfo at the specified index.
func (si *SegmentInfos) Insert(index int, sci *SegmentCommitInfo) {
	si.mu.Lock()
	defer si.mu.Unlock()

	if index < 0 || index > len(si.segments) {
		return
	}

	// Insert at position
	si.segments = append(si.segments, nil)
	copy(si.segments[index+1:], si.segments[index:])
	si.segments[index] = sci
}

// Remove removes the SegmentCommitInfo at the given index.
// Returns the removed SegmentCommitInfo or nil if index is out of bounds.
func (si *SegmentInfos) Remove(index int) *SegmentCommitInfo {
	si.mu.Lock()
	defer si.mu.Unlock()

	if index < 0 || index >= len(si.segments) {
		return nil
	}

	removed := si.segments[index]
	si.segments = append(si.segments[:index], si.segments[index+1:]...)
	return removed
}

// Clear removes all segments from this SegmentInfos.
func (si *SegmentInfos) Clear() {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.segments = si.segments[:0]
}

// List returns a copy of the SegmentCommitInfo list.
func (si *SegmentInfos) List() SegmentCommitInfoList {
	si.mu.RLock()
	defer si.mu.RUnlock()

	list := make(SegmentCommitInfoList, len(si.segments))
	copy(list, si.segments)
	return list
}

// Generation returns the current generation number.
// This is used for the segments file name.
func (si *SegmentInfos) Generation() int64 {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.generation
}

// SetGeneration sets the generation number.
func (si *SegmentInfos) SetGeneration(gen int64) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.generation = gen
}

// LastGeneration returns the last committed generation.
// This is the generation of the last successfully written segments file.
func (si *SegmentInfos) LastGeneration() int64 {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.lastGeneration
}

// SetLastGeneration sets the last committed generation.
func (si *SegmentInfos) SetLastGeneration(gen int64) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.lastGeneration = gen
}

// NextGeneration advances the generation and returns the new value.
func (si *SegmentInfos) NextGeneration() int64 {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.generation++
	return si.generation
}

// GetNextSegmentName generates a new unique segment name.
// Segment names follow the pattern "_N" where N is an incrementing counter.
func (si *SegmentInfos) GetNextSegmentName() string {
	si.mu.Lock()
	defer si.mu.Unlock()

	name := fmt.Sprintf("_%d", si.counter)
	si.counter++
	return name
}

// GetSegmentFileName returns the segments file name for a given generation.
// The format is "segments_N" where N is the generation encoded in base-36
// (lowercase), matching Lucene's Long.toString(gen, Character.MAX_RADIX).
func GetSegmentFileName(generation int64) string {
	if generation < 0 {
		return ""
	}
	return "segments_" + strconv.FormatInt(generation, 36)
}

// GetFileName returns the segments file name for the current generation.
func (si *SegmentInfos) GetFileName() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return GetSegmentFileName(si.generation)
}

// GetLastFileName returns the segments file name for the last generation.
func (si *SegmentInfos) GetLastFileName() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return GetSegmentFileName(si.lastGeneration)
}

// Version returns the index version.
func (si *SegmentInfos) Version() int64 {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.version
}

// SetVersion sets the index version.
func (si *SegmentInfos) SetVersion(version int64) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.version = version
}

// LuceneVersion returns the Lucene version.
func (si *SegmentInfos) LuceneVersion() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.luceneVersion
}

// SetLuceneVersion sets the Lucene version.
func (si *SegmentInfos) SetLuceneVersion(v string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.luceneVersion = v
}

// IndexCreatedVersionMajor returns the major version of Lucene that created this index.
func (si *SegmentInfos) IndexCreatedVersionMajor() int32 {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.indexCreatedVersionMajor
}

// SetIndexCreatedVersionMajor sets the major version of Lucene that created this index.
func (si *SegmentInfos) SetIndexCreatedVersionMajor(v int32) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.indexCreatedVersionMajor = v
}

// Counter returns the current segment name counter.
func (si *SegmentInfos) Counter() int64 {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.counter
}

// SetCounter sets the segment name counter.
func (si *SegmentInfos) SetCounter(counter int64) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.counter = counter
}

// TotalDocCount returns the total number of documents across all segments.
func (si *SegmentInfos) TotalDocCount() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.segments.TotalDocCount()
}

// TotalNumDocs returns the total number of live documents across all segments.
func (si *SegmentInfos) TotalNumDocs() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.segments.TotalNumDocs()
}

// TotalDelCount returns the total number of deleted documents across all segments.
func (si *SegmentInfos) TotalDelCount() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.segments.TotalDelCount()
}

// GetUserData returns the user-supplied commit data.
func (si *SegmentInfos) GetUserData() map[string]string {
	si.mu.RLock()
	defer si.mu.RUnlock()

	data := make(map[string]string, len(si.userData))
	for k, v := range si.userData {
		data[k] = v
	}
	return data
}

// SetUserData sets the user-supplied commit data.
func (si *SegmentInfos) SetUserData(data map[string]string) {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.userData = make(map[string]string, len(data))
	for k, v := range data {
		si.userData[k] = v
	}
}

// SetUserDataValue sets a single user data value.
func (si *SegmentInfos) SetUserDataValue(key, value string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.userData[key] = value
}

// GetUserDataValue returns a single user data value.
func (si *SegmentInfos) GetUserDataValue(key string) string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.userData[key]
}

// GetInMemoryParentField returns the parentField recorded at commit time.
func (si *SegmentInfos) GetInMemoryParentField() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.inMemoryParentField
}

// SetInMemoryParentField records the parentField for in-memory validation.
func (si *SegmentInfos) SetInMemoryParentField(f string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.inMemoryParentField = f
}

// GetInMemoryIndexSort returns the indexSort recorded at commit time.
func (si *SegmentInfos) GetInMemoryIndexSort() *Sort {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.inMemoryIndexSort
}

// SetInMemoryIndexSort records the indexSort for in-memory validation.
func (si *SegmentInfos) SetInMemoryIndexSort(s *Sort) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.inMemoryIndexSort = s
}

// Clone creates a deep copy of this SegmentInfos.
// The segments list is copied but the SegmentCommitInfo references are shared.
func (si *SegmentInfos) Clone() *SegmentInfos {
	si.mu.RLock()
	defer si.mu.RUnlock()

	clone := &SegmentInfos{
		segments:                 make(SegmentCommitInfoList, len(si.segments)),
		generation:               si.generation,
		lastGeneration:           si.lastGeneration,
		version:                  si.version,
		luceneVersion:            si.luceneVersion,
		indexCreatedVersionMajor: si.indexCreatedVersionMajor,
		counter:                  si.counter,
		userData:                 make(map[string]string, len(si.userData)),
	}

	copy(clone.segments, si.segments)
	for k, v := range si.userData {
		clone.userData[k] = v
	}

	return clone
}

// Contains returns true if the given SegmentCommitInfo is in the list.
func (si *SegmentInfos) Contains(sci *SegmentCommitInfo) bool {
	si.mu.RLock()
	defer si.mu.RUnlock()

	for _, s := range si.segments {
		if s == sci {
			return true
		}
	}
	return false
}

// IndexOf returns the index of the given SegmentCommitInfo or -1 if not found.
func (si *SegmentInfos) IndexOf(sci *SegmentCommitInfo) int {
	si.mu.RLock()
	defer si.mu.RUnlock()

	for i, s := range si.segments {
		if s == sci {
			return i
		}
	}
	return -1
}

// RemoveByName removes all segments with the given name.
// Returns the number of segments removed.
func (si *SegmentInfos) RemoveByName(name string) int {
	si.mu.Lock()
	defer si.mu.Unlock()

	count := 0
	newList := make(SegmentCommitInfoList, 0, len(si.segments))
	for _, s := range si.segments {
		if s.Name() != name {
			newList = append(newList, s)
		} else {
			count++
		}
	}
	si.segments = newList
	return count
}

// SortByName sorts the segments by their name.
func (si *SegmentInfos) SortByName() {
	si.mu.Lock()
	defer si.mu.Unlock()

	sort.Slice(si.segments, func(i, j int) bool {
		return si.segments[i].Name() < si.segments[j].Name()
	})
}

// String returns a string representation of SegmentInfos.
func (si *SegmentInfos) String() string {
	si.mu.RLock()
	defer si.mu.RUnlock()

	return fmt.Sprintf("SegmentInfos(segments=%d, generation=%d, version=%d, docs=%d)",
		len(si.segments), si.generation, si.version, si.segments.TotalNumDocs())
}

// Iterator returns a function that iterates over segments.
// Usage: for sci := range si.Iterator() { ... }
func (si *SegmentInfos) Iterator() func(yield func(*SegmentCommitInfo) bool) {
	return func(yield func(*SegmentCommitInfo) bool) {
		si.mu.RLock()
		defer si.mu.RUnlock()
		for _, sci := range si.segments {
			if !yield(sci) {
				return
			}
		}
	}
}

// GetMaxSegmentName returns the maximum segment name (highest generation).
// Returns empty string if there are no segments.
func (si *SegmentInfos) GetMaxSegmentName() string {
	si.mu.RLock()
	defer si.mu.RUnlock()

	maxName := ""
	for _, sci := range si.segments {
		name := sci.Name()
		if name > maxName {
			maxName = name
		}
	}
	return maxName
}

// UpdateCounterFromSegments updates the counter based on existing segment names.
// This ensures new segment names don't conflict with existing ones.
func (si *SegmentInfos) UpdateCounterFromSegments() {
	si.mu.Lock()
	defer si.mu.Unlock()

	maxGen := int64(0)
	for _, sci := range si.segments {
		gen := sci.GetGeneration()
		if gen > maxGen {
			maxGen = gen
		}
	}
	si.counter = maxGen + 1
}

// WriteSegmentInfos writes the SegmentInfos to a directory.
func WriteSegmentInfos(si *SegmentInfos, directory store.Directory) error {
	si.mu.RLock()
	defer si.mu.RUnlock()

	fileName := si.GetFileName()
	out, err := directory.CreateOutput(fileName, store.IOContextWrite)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write header
	if err := store.WriteInt32(out, 0x3d767); err != nil {
		return err
	}
	if err := store.WriteInt64(out, si.generation); err != nil {
		return err
	}
	if err := store.WriteInt64(out, si.version); err != nil {
		return err
	}
	if err := store.WriteInt32(out, si.indexCreatedVersionMajor); err != nil {
		return err
	}
	if err := store.WriteString(out, si.luceneVersion); err != nil {
		return err
	}
	if err := store.WriteInt64(out, si.counter); err != nil {
		return err
	}

	// Write parentField (empty string if none).
	if err := store.WriteString(out, si.inMemoryParentField); err != nil {
		return err
	}

	// Write indexSort: numFields int32, then per field: name string, sortType int32, descending int32.
	if si.inMemoryIndexSort == nil || len(si.inMemoryIndexSort.fields) == 0 {
		if err := store.WriteInt32(out, 0); err != nil {
			return err
		}
	} else {
		if err := store.WriteInt32(out, int32(len(si.inMemoryIndexSort.fields))); err != nil {
			return err
		}
		for _, sf := range si.inMemoryIndexSort.fields {
			if err := store.WriteString(out, sf.field); err != nil {
				return err
			}
			if err := store.WriteInt32(out, int32(sf.sortType)); err != nil {
				return err
			}
			desc := int32(0)
			if sf.descending {
				desc = 1
			}
			if err := store.WriteInt32(out, desc); err != nil {
				return err
			}
		}
	}

	if err := store.WriteInt32(out, int32(len(si.segments))); err != nil {
		return err
	}

	// Write segments
	for _, sci := range si.segments {
		if err := store.WriteString(out, sci.Name()); err != nil {
			return err
		}
		if err := store.WriteInt32(out, int32(sci.segmentInfo.docCount)); err != nil {
			return err
		}
		if err := store.WriteInt32(out, int32(sci.delCount)); err != nil {
			return err
		}
		// Write ID
		id := sci.segmentInfo.GetID()
		if err := out.WriteBytes(id); err != nil {
			return err
		}
		// Write in-memory FieldInfos so readers can enumerate fields without
		// codec infrastructure.  Format: numFields int32, then per field:
		//   name string, number int32, indexOptions int32, docValuesType int32,
		//   flags int32 (bit 0=stored, 1=tokenized, 2=storeTermVectors, 3=omitNorms)
		fi := sci.GetInMemoryFieldInfos()
		if fi == nil {
			if err := store.WriteInt32(out, 0); err != nil {
				return err
			}
		} else {
			if err := store.WriteInt32(out, int32(fi.Size())); err != nil {
				return err
			}
			iter := fi.Iterator()
			for {
				info := iter.Next()
				if info == nil {
					break
				}
				if err := store.WriteString(out, info.Name()); err != nil {
					return err
				}
				if err := store.WriteInt32(out, int32(info.Number())); err != nil {
					return err
				}
				if err := store.WriteInt32(out, int32(info.IndexOptions())); err != nil {
					return err
				}
				if err := store.WriteInt32(out, int32(info.DocValuesType())); err != nil {
					return err
				}
				flags := int32(0)
				if info.IsStored() {
					flags |= 1
				}
				if info.IsTokenized() {
					flags |= 2
				}
				if info.HasTermVectors() {
					flags |= 4
				}
				if info.OmitNorms() {
					flags |= 8
				}
				if err := store.WriteInt32(out, flags); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// ReadSegmentInfos reads the SegmentInfos from a directory.
// This is used when opening an existing index.
func ReadSegmentInfos(directory store.Directory) (*SegmentInfos, error) {
	// List all files in the directory
	files, err := directory.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listing directory: %w", err)
	}

	// Find the most recent segments file
	var maxGen int64 = -1
	var latestFile string
	for _, file := range files {
		if len(file) > 9 && file[:9] == "segments_" {
			// Parse generation number (base-36, matching Lucene Long.parseLong).
			if gen, err := strconv.ParseInt(file[9:], 36, 64); err == nil {
				if gen > maxGen {
					maxGen = gen
					latestFile = file
				}
			}
		}
	}

	if maxGen < 0 {
		return nil, NewIndexNotFoundException("no segments* file found in directory", nil)
	}

	in, err := directory.OpenInput(latestFile, store.IOContextRead)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	// Read header
	magic, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}
	if magic != 0x3d767 {
		return nil, fmt.Errorf("invalid segments file magic: %x", magic)
	}

	gen, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	version, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	createdMajor, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}

	luceneVersion, err := store.ReadString(in)
	if err != nil {
		return nil, err
	}

	counter, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	// Read parentField.
	parentField, err := store.ReadString(in)
	if err != nil {
		// Older format without parentField — tolerate gracefully.
		si := NewSegmentInfos()
		si.generation = gen
		si.lastGeneration = gen
		si.version = version
		si.indexCreatedVersionMajor = createdMajor
		si.luceneVersion = luceneVersion
		si.counter = counter
		return si, nil
	}

	// Read indexSort.
	numSortFields, err := store.ReadInt32(in)
	if err != nil {
		si := NewSegmentInfos()
		si.generation = gen
		si.lastGeneration = gen
		si.version = version
		si.indexCreatedVersionMajor = createdMajor
		si.luceneVersion = luceneVersion
		si.counter = counter
		si.inMemoryParentField = parentField
		return si, nil
	}
	var indexSort *Sort
	if numSortFields > 0 {
		fields := make([]SortField, 0, numSortFields)
		for j := int32(0); j < numSortFields; j++ {
			fname, err := store.ReadString(in)
			if err != nil {
				return nil, fmt.Errorf("reading sort field name: %w", err)
			}
			stRaw, err := store.ReadInt32(in)
			if err != nil {
				return nil, fmt.Errorf("reading sort type: %w", err)
			}
			descRaw, err := store.ReadInt32(in)
			if err != nil {
				return nil, fmt.Errorf("reading sort descending: %w", err)
			}
			sf := SortField{field: fname, sortType: SortType(stRaw), descending: descRaw != 0}
			fields = append(fields, sf)
		}
		indexSort = &Sort{fields: fields}
	}

	numSegments, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}

	si := NewSegmentInfos()
	si.generation = gen
	si.lastGeneration = gen
	si.version = version
	si.indexCreatedVersionMajor = createdMajor
	si.luceneVersion = luceneVersion
	si.counter = counter
	si.inMemoryParentField = parentField
	si.inMemoryIndexSort = indexSort

	// Read segments
	for i := 0; i < int(numSegments); i++ {
		name, err := store.ReadString(in)
		if err != nil {
			return nil, err
		}
		docCount, err := store.ReadInt32(in)
		if err != nil {
			return nil, err
		}
		delCount, err := store.ReadInt32(in)
		if err != nil {
			return nil, err
		}
		// Read ID
		id, err := in.ReadBytesN(16)
		if err != nil {
			return nil, err
		}

		segmentInfo := NewSegmentInfo(name, int(docCount), directory)
		segmentInfo.SetID(id)
		sci := NewSegmentCommitInfo(segmentInfo, int(delCount), -1)

		// Read in-memory FieldInfos (written by current Gocene writer).
		numFields, err := store.ReadInt32(in)
		if err != nil {
			// Older format without FieldInfos extension — tolerate gracefully.
			si.Add(sci)
			continue
		}
		if numFields > 0 {
			fis := NewFieldInfos()
			for f := int32(0); f < numFields; f++ {
				fname, err := store.ReadString(in)
				if err != nil {
					return nil, fmt.Errorf("reading field name: %w", err)
				}
				fnum, err := store.ReadInt32(in)
				if err != nil {
					return nil, fmt.Errorf("reading field number: %w", err)
				}
				ioRaw, err := store.ReadInt32(in)
				if err != nil {
					return nil, fmt.Errorf("reading index options: %w", err)
				}
				dvRaw, err := store.ReadInt32(in)
				if err != nil {
					return nil, fmt.Errorf("reading doc values type: %w", err)
				}
				flags, err := store.ReadInt32(in)
				if err != nil {
					return nil, fmt.Errorf("reading flags: %w", err)
				}
				opts := FieldInfoOptions{
					IndexOptions:             IndexOptions(ioRaw),
					DocValuesType:            DocValuesType(dvRaw),
					DocValuesSkipIndexType:   DocValuesSkipIndexTypeNone,
					DocValuesGen:             -1,
					Stored:                   flags&1 != 0,
					Tokenized:                flags&2 != 0,
					StoreTermVectors:         flags&4 != 0,
					OmitNorms:                flags&8 != 0,
					VectorEncoding:           VectorEncodingFloat32,
					VectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
				}
				fi := NewFieldInfo(fname, int(fnum), opts)
				_ = fis.Add(fi)
			}
			sci.SetInMemoryFieldInfos(fis)
		}
		si.Add(sci)
	}

	return si, nil
}
