// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfos manages a collection of SegmentCommitInfo representing a
// point-in-time view of the index segments.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentInfos.
// Lifted from package index to package spi by rmp #4706 so that the canonical
// SegmentInfosFormat interface and the segments_N reader/writer can sit on the
// SPI surface without a back-edge into package index.
//
// SegmentInfos maintains the list of segments in the index and handles
// generation-based file naming for the segments file (e.g., segments_1,
// segments_2, etc.).
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
	inMemoryIndexSort *schema.Sort

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

// GetCounter returns the current segment name counter.
func (si *SegmentInfos) GetCounter() int64 {
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
func (si *SegmentInfos) GetInMemoryIndexSort() *schema.Sort {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.inMemoryIndexSort
}

// SetInMemoryIndexSort records the indexSort for in-memory validation.
func (si *SegmentInfos) SetInMemoryIndexSort(s *schema.Sort) {
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

	// Deep-clone each SegmentCommitInfo so mutations through the clone (e.g.
	// AdvanceDelGen) do not race or bleed into the original. This mirrors
	// Lucene's SegmentInfos.clone(), which adds info.clone() per element rather
	// than sharing the SegmentCommitInfo references.
	for i, sci := range si.segments {
		clone.segments[i] = sci.Clone()
	}
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

// Replace replaces all segments in this instance with the segments from other,
// while preserving generation, version, counter, luceneVersion and
// indexCreatedVersionMajor so that future commits remain write-once. It also
// copies lastGeneration and userData from other, matching Lucene's
// SegmentInfos.replace.
func (si *SegmentInfos) Replace(other *SegmentInfos) {
	if other == nil {
		return
	}
	si.mu.Lock()
	defer si.mu.Unlock()

	other.mu.RLock()
	infos := make(SegmentCommitInfoList, len(other.segments))
	for i, sci := range other.segments {
		infos[i] = sci.Clone()
	}
	lastGen := other.lastGeneration
	userData := make(map[string]string, len(other.userData))
	for k, v := range other.userData {
		userData[k] = v
	}
	other.mu.RUnlock()

	si.segments = infos
	si.lastGeneration = lastGen
	si.userData = userData
}

// CreateBackupSegmentInfos returns a deep copy of every SegmentCommitInfo in
// this SegmentInfos. This is the rollback snapshot used by IndexWriter.
func (si *SegmentInfos) CreateBackupSegmentInfos() SegmentCommitInfoList {
	si.mu.RLock()
	defer si.mu.RUnlock()

	list := make(SegmentCommitInfoList, len(si.segments))
	for i, sci := range si.segments {
		list[i] = sci.Clone()
	}
	return list
}

// RollbackSegmentInfos restores the segment list from the supplied backup.
// The backup is typically produced by CreateBackupSegmentInfos. Metadata such
// as generation and counter are left untouched so the writer can continue to
// use the same SegmentInfos instance after a rollback.
func (si *SegmentInfos) RollbackSegmentInfos(infos SegmentCommitInfoList) {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.segments = make(SegmentCommitInfoList, len(infos))
	for i, sci := range infos {
		si.segments[i] = sci.Clone()
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

// goceneExtVersion is the version marker stored in userData to signal that
// in-memory Gocene extensions are present.
const goceneExtVersion = "1"

// segmentInfoReader is an optional, process-wide hook that reads a per-segment
// .si file and returns the fully-populated SegmentInfo (docCount, version,
// compound flag, file set, etc.). It mirrors what
// org.apache.lucene.index.SegmentInfos.parseSegmentInfos does when it calls
// codec.segmentInfoFormat().read(...) for every segment record.
//
// The hook is registered by package codecs at init (it owns the concrete
// Lucene99SegmentInfoFormat reader) via RegisterSegmentInfoReader, keeping
// package spi free of a back-edge into package codecs. When no hook is
// registered (codec-less structural tests), ReadSegmentInfos falls back to the
// docCount carried in userData (see the _gocene_dc_ legacy compatibility path).
var (
	segmentInfoReaderMu sync.RWMutex
	segmentInfoReader   func(dir store.Directory, segmentName string, segmentID []byte) (*schema.SegmentInfo, error)
)

// RegisterSegmentInfoReader installs the process-wide .si reader hook used by
// ReadSegmentInfos to populate each segment's authoritative metadata (docCount
// in particular) from the on-disk .si file rather than from segments_N
// userData. Passing nil clears the hook. Registering is idempotent and safe to
// call from an init function.
func RegisterSegmentInfoReader(fn func(dir store.Directory, segmentName string, segmentID []byte) (*schema.SegmentInfo, error)) {
	segmentInfoReaderMu.Lock()
	segmentInfoReader = fn
	segmentInfoReaderMu.Unlock()
}

// lookupSegmentInfoReader returns the registered .si reader hook, or nil.
func lookupSegmentInfoReader() func(dir store.Directory, segmentName string, segmentID []byte) (*schema.SegmentInfo, error) {
	segmentInfoReaderMu.RLock()
	defer segmentInfoReaderMu.RUnlock()
	return segmentInfoReader
}

// escapeUserDataToken percent-encodes the delimiter characters used by the
// userData FieldInfos encoding ('\n', '|', '=', ';', '%') so that arbitrary
// attribute keys/values can be embedded without breaking the framing.
func escapeUserDataToken(s string) string {
	repl := strings.NewReplacer(
		"%", "%25",
		"\n", "%0A",
		"|", "%7C",
		"=", "%3D",
		";", "%3B",
	)
	return repl.Replace(s)
}

// unescapeUserDataToken reverses escapeUserDataToken.
func unescapeUserDataToken(s string) string {
	repl := strings.NewReplacer(
		"%0A", "\n",
		"%7C", "|",
		"%3D", "=",
		"%3B", ";",
		"%25", "%",
	)
	return repl.Replace(s)
}

// encodeFieldInfoAttributes serialises a FieldInfo attribute map as a single
// token of the form "k1=v1;k2=v2" with each key and value escaped via
// escapeUserDataToken.  An empty/nil map yields the empty string.
func encodeFieldInfoAttributes(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	// Deterministic ordering keeps the encoded segments_N stable across runs.
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(escapeUserDataToken(k))
		b.WriteByte('=')
		b.WriteString(escapeUserDataToken(attrs[k]))
	}
	return b.String()
}

// decodeFieldInfoAttributes reverses encodeFieldInfoAttributes.  Returns nil
// for the empty string.
func decodeFieldInfoAttributes(encoded string) map[string]string {
	if encoded == "" {
		return nil
	}
	out := make(map[string]string)
	for _, pair := range strings.Split(encoded, ";") {
		if pair == "" {
			continue
		}
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		k := unescapeUserDataToken(pair[:eq])
		v := unescapeUserDataToken(pair[eq+1:])
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// encodeFieldInfosForUserData serialises a FieldInfos as a compact string
// suitable for storage in the userData map.  Format per FieldInfo (pipe-separated
// tokens, newline-separated entries):
//
//	"<name>|<num>|<indexOpts>|<dvType>|<flags>|<vecDim>|<vecEnc>|<vecSim>|<ptDim>|<ptIdxDim>|<ptBytes>|<attrs>"
//
// where flags is a bitmask: 1=stored, 2=tokenized, 4=termVectors, 8=omitNorms.
//
// Tokens 6-11 (the vector and point triplets) and token 12 (the codec
// attribute map) were appended after the original 5-token form so that KNN
// vector and BKD point fields — and the per-field codec attributes that the
// PerField*Format readers need (e.g. PerFieldKnnVectorsFormat.format /
// .suffix) — survive a reopen that resolves FieldInfos from this
// Gocene-private userData encoding rather than from the on-disk .fnm.
// decodeFieldInfosFromUserData accepts the legacy 5-token and intermediate
// 11-token entries (treating the missing attributes as zero / empty) for
// forward compatibility with indices written before this change.
func encodeFieldInfosForUserData(fi *schema.FieldInfos) string {
	if fi == nil || fi.Size() == 0 {
		return ""
	}
	var b strings.Builder
	iter := fi.Iterator()
	first := true
	for {
		info := iter.Next()
		if info == nil {
			break
		}
		if !first {
			b.WriteByte('\n')
		}
		first = false
		flags := 0
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
		fmt.Fprintf(&b, "%s|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|%s",
			info.Name(),
			info.Number(),
			int(info.IndexOptions()),
			int(info.DocValuesType()),
			flags,
			info.VectorDimension(),
			int(info.VectorEncoding()),
			int(info.VectorSimilarityFunction()),
			info.PointDimensionCount(),
			info.PointIndexDimensionCount(),
			info.PointNumBytes(),
			encodeFieldInfoAttributes(info.GetAttributes()),
		)
	}
	return b.String()
}

// decodeFieldInfosFromUserData reconstructs a FieldInfos from the compact
// string format written by encodeFieldInfosForUserData.  Returns nil on empty input.
func decodeFieldInfosFromUserData(encoded string) (*schema.FieldInfos, error) {
	if encoded == "" {
		return nil, nil
	}
	fis := schema.NewFieldInfos()
	for _, line := range strings.Split(encoded, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		// 5 tokens  = legacy form (pre vector/point);
		// 11 tokens = vector + point triplets, no attribute map;
		// 12 tokens = current form (adds the codec attribute map).
		if len(parts) != 5 && len(parts) != 11 && len(parts) != 12 {
			return nil, fmt.Errorf("malformed FieldInfo entry: %q", line)
		}
		fnum, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid field number %q: %w", parts[1], err)
		}
		ioRaw, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid indexOptions %q: %w", parts[2], err)
		}
		dvRaw, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid dvType %q: %w", parts[3], err)
		}
		flags, err := strconv.Atoi(parts[4])
		if err != nil {
			return nil, fmt.Errorf("invalid flags %q: %w", parts[4], err)
		}
		opts := schema.FieldInfoOptions{
			IndexOptions:             schema.IndexOptions(ioRaw),
			DocValuesType:            schema.DocValuesType(dvRaw),
			DocValuesSkipIndexType:   schema.DocValuesSkipIndexTypeNone,
			DocValuesGen:             -1,
			Stored:                   flags&1 != 0,
			Tokenized:                flags&2 != 0,
			StoreTermVectors:         flags&4 != 0,
			OmitNorms:                flags&8 != 0,
			VectorEncoding:           schema.VectorEncodingFloat32,
			VectorSimilarityFunction: schema.VectorSimilarityFunctionEuclidean,
		}
		// Vector and point triplets are present in the 11- and 12-token forms.
		// Legacy 5-token entries leave VectorDimension / point dimensions at
		// zero (no vector / no point field), which matches their original
		// information content.
		if len(parts) >= 11 {
			vecDim, err := strconv.Atoi(parts[5])
			if err != nil {
				return nil, fmt.Errorf("invalid vectorDimension %q: %w", parts[5], err)
			}
			vecEnc, err := strconv.Atoi(parts[6])
			if err != nil {
				return nil, fmt.Errorf("invalid vectorEncoding %q: %w", parts[6], err)
			}
			vecSim, err := strconv.Atoi(parts[7])
			if err != nil {
				return nil, fmt.Errorf("invalid vectorSimilarityFunction %q: %w", parts[7], err)
			}
			ptDim, err := strconv.Atoi(parts[8])
			if err != nil {
				return nil, fmt.Errorf("invalid pointDimensionCount %q: %w", parts[8], err)
			}
			ptIdxDim, err := strconv.Atoi(parts[9])
			if err != nil {
				return nil, fmt.Errorf("invalid pointIndexDimensionCount %q: %w", parts[9], err)
			}
			ptBytes, err := strconv.Atoi(parts[10])
			if err != nil {
				return nil, fmt.Errorf("invalid pointNumBytes %q: %w", parts[10], err)
			}
			opts.VectorDimension = vecDim
			// Only adopt the encoded vector encoding/similarity for actual
			// vector fields (dimension > 0); for non-vector fields keep the
			// Float32/Euclidean defaults set above so the cosmetic getters do
			// not report a spurious BYTE encoding.
			if vecDim > 0 {
				opts.VectorEncoding = schema.VectorEncoding(vecEnc)
				opts.VectorSimilarityFunction = schema.VectorSimilarityFunction(vecSim)
			}
			opts.PointDimensionCount = ptDim
			opts.PointIndexDimensionCount = ptIdxDim
			opts.PointNumBytes = ptBytes
		}
		fi := schema.NewFieldInfo(parts[0], fnum, opts)
		// The codec attribute map (token 12) carries the per-field codec
		// metadata that PerField*Format readers require to resolve their
		// delegate (e.g. PerFieldKnnVectorsFormat.format / .suffix). Restore
		// it via PutCodecAttribute, which is the frozen-safe setter codec
		// metadata uses on both the write and read paths.
		if len(parts) >= 12 {
			for k, v := range decodeFieldInfoAttributes(parts[11]) {
				fi.PutCodecAttribute(k, v)
			}
		}
		_ = fis.Add(fi)
	}
	return fis, nil
}

// WriteSegmentInfos writes SegmentInfos to a directory using the real Lucene
// 10.4.0 segments_N format (codec magic 0x3FD76C17, codec name "segments",
// version 10).
//
// In-memory Gocene extensions (FieldInfos, deleted ordinals, parentField,
// indexSort) that have no Lucene wire counterpart are packed into the userData
// map under "_gocene_*" keys so that the file remains byte-format compatible
// with the Lucene 10.4.0 reader.
func WriteSegmentInfos(si *SegmentInfos, directory store.Directory) error {
	si.mu.RLock()
	defer si.mu.RUnlock()

	fileName := GetSegmentFileName(si.generation)
	rawOut, err := directory.CreateOutput(fileName, store.IOContextWrite)
	if err != nil {
		return err
	}
	out := store.NewChecksumIndexOutput(rawOut)
	defer out.Close()

	// Random 16-byte ID for the index header.
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return fmt.Errorf("generating header ID: %w", err)
	}

	suffix := strconv.FormatInt(si.generation, 36)
	if err := WriteIndexHeader(out, "segments", 10, id, suffix); err != nil {
		return err
	}

	// Lucene version (major.minor.bugfix as VInts).
	var major, minor, bugfix int32
	fmt.Sscanf(si.luceneVersion, "%d.%d.%d", &major, &minor, &bugfix)
	if err := store.WriteVInt(out, major); err != nil {
		return err
	}
	if err := store.WriteVInt(out, minor); err != nil {
		return err
	}
	if err := store.WriteVInt(out, bugfix); err != nil {
		return err
	}

	// Index created major version.
	if err := store.WriteVInt(out, si.indexCreatedVersionMajor); err != nil {
		return err
	}

	// Index version counter.
	if err := store.WriteInt64(out, si.version); err != nil {
		return err
	}

	// Segment name counter.
	if err := store.WriteVLong(out, si.counter); err != nil {
		return err
	}

	// Number of segments.
	if err := store.WriteInt32(out, int32(len(si.segments))); err != nil {
		return err
	}

	// Min segment version (written only when numSegments > 0). Mirrors
	// SegmentInfos.write: the minimum version across all segments, NOT the
	// SegmentInfos luceneVersion. For a single-version index these coincide,
	// but a multi-segment index merged across versions records the lowest.
	if len(si.segments) > 0 {
		minMajor, minMinor, minBugfix := minSegmentVersion(si.segments)
		if err := store.WriteVInt(out, minMajor); err != nil {
			return err
		}
		if err := store.WriteVInt(out, minMinor); err != nil {
			return err
		}
		if err := store.WriteVInt(out, minBugfix); err != nil {
			return err
		}
	}

	// Per-segment data.
	for _, sci := range si.segments {
		if err := writeSegmentCommitInfoLucene104(out, sci); err != nil {
			return err
		}
	}

	// Build userData: start with existing user data, then overlay Gocene extensions.
	userData := make(map[string]string, len(si.userData))
	for k, v := range si.userData {
		userData[k] = v
	}

	// Per-segment docCount and FieldInfos are NO LONGER round-tripped through
	// segments_N userData (rmp #4785): the real .si file carries the
	// authoritative docCount (read back via the registered .si reader hook) and
	// the real .fnm carries the authoritative FieldInfos (read back in
	// openSegmentReader, from inside the .cfs for a compound segment). The
	// former _gocene_dc_ and _gocene_fi_ keys are therefore not written.
	//
	// Deleted ordinals are NO LONGER round-tripped through segments_N userData
	// (rmp #4789): the merge/ForceMerge/AddIndexes path now writes a real
	// Lucene90 .liv file and bumps the segment's delGen/delCount (see
	// IndexWriter.persistMergedDeletions), and the reopen path
	// (loadLiveDocsFromDisk) reads the .liv back when delGen >= 0. The former
	// _gocene_del_ keys are therefore not written.
	//
	// The parentField is NO LONGER round-tripped through segments_N userData
	// (rmp #4789): the authoritative on-disk home is the per-segment .fnm parent
	// bit (Lucene94FieldInfosFormat), which AddIndexes consults via
	// IndexWriter.sourceParentFieldFromDisk. The former _gocene_parent key is
	// therefore not written.
	//
	// The index sort is NO LONGER round-tripped through segments_N userData
	// (rmp #4789): it is serialised into each per-segment .si numSortFields
	// block (writeSegmentInfoSort, byte-faithful to Lucene90SegmentInfoFormat)
	// and read back from there by ReadSegmentInfos (which derives the
	// index-level sort from the first segment's .si).
	//
	// No _gocene_* keys are written: segments_N userData is now pure
	// user-supplied commit data (rmp #4789).

	if err := store.WriteMapOfStrings(out, userData); err != nil {
		return err
	}

	return WriteFooter(out)
}

// writeSegmentCommitInfoLucene104 writes a single SegmentCommitInfo using the
// Lucene 10.4.0 per-segment layout inside segments_N.
func writeSegmentCommitInfoLucene104(out store.IndexOutput, sci *SegmentCommitInfo) error {
	if err := store.WriteString(out, sci.Name()); err != nil {
		return err
	}

	// 16-byte segment ID; write zeros if absent.
	id := sci.segmentInfo.GetID()
	if len(id) != 16 {
		id = make([]byte, 16)
	}
	if err := out.WriteBytes(id); err != nil {
		return err
	}

	// Codec name (empty string for Gocene-only segments without a real codec).
	codec := sci.segmentInfo.Codec()
	if err := store.WriteString(out, codec); err != nil {
		return err
	}


	// Deletion generation (-1 if no deletions file).
	if err := store.WriteInt64(out, sci.DelGen()); err != nil {
		return err
	}

	// Deletion count.
	if err := store.WriteInt32(out, int32(sci.DelCount())); err != nil {
		return err
	}

	// FieldInfos generation (-1 if none).
	if err := store.WriteInt64(out, sci.FieldInfosGen()); err != nil {
		return err
	}

	// DocValues generation (-1 if none).
	if err := store.WriteInt64(out, sci.DocValuesGen()); err != nil {
		return err
	}

	// Soft delete count.
	if err := store.WriteInt32(out, int32(sci.SoftDelCount())); err != nil {
		return err
	}

	// SegmentCommitInfo ID: 0 = absent, 1 = present + 16 bytes.
	sciID := sci.GetID()
	if len(sciID) == 16 {
		if err := out.WriteByte(1); err != nil {
			return err
		}
		if err := out.WriteBytes(sciID); err != nil {
			return err
		}
	} else {
		if err := out.WriteByte(0); err != nil {
			return err
		}
	}

	// FieldInfos files set.
	if err := store.WriteSetOfStrings(out, sci.FieldInfosFiles()); err != nil {
		return err
	}

	// DocValues updates files: Lucene writes the count and keys as BE int32
	// (CodecUtil.writeBEInt), not VInt.  We mirror that here for wire compatibility.
	return writeDVUpdateFilesLucene(out, sci.DocValuesUpdatesFiles())
}

// readDVUpdateFilesLucene reads the docValuesUpdatesFiles map in Lucene wire
// format: count as BE int32, then per entry: key as BE int32 + value as
// ReadSetOfStrings.  This differs from store.ReadMapOfIntToSetOfStrings which
// uses VInt for both count and key.
func readDVUpdateFilesLucene(in store.IndexInput) (map[int]map[string]struct{}, error) {
	countRaw, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}
	count := int(countRaw)
	if count == 0 {
		return nil, nil
	}
	m := make(map[int]map[string]struct{}, count)
	for i := 0; i < count; i++ {
		keyRaw, err := store.ReadInt32(in)
		if err != nil {
			return nil, err
		}
		val, err := store.ReadSetOfStrings(in)
		if err != nil {
			return nil, err
		}
		m[int(keyRaw)] = val
	}
	return m, nil
}

// minSegmentVersion returns the lowest Lucene version (major, minor, bugfix)
// across the supplied segments, mirroring the pre-pass in
// org.apache.lucene.index.SegmentInfos.write. Version ordering follows
// Lucene's Version.onOrAfter, which compares the encoded triple
// major<<18 | minor<<8 | bugfix. An empty input yields (0,0,0); callers only
// invoke this when at least one segment is present.
func minSegmentVersion(segments SegmentCommitInfoList) (int32, int32, int32) {
	var haveMin bool
	var minEncoded int64
	var minMajor, minMinor, minBugfix int32
	for _, sci := range segments {
		var maj, min, bug int32
		fmt.Sscanf(sci.segmentInfo.Version(), "%d.%d.%d", &maj, &min, &bug)
		encoded := int64(maj)<<18 | int64(min)<<8 | int64(bug)
		if !haveMin || encoded < minEncoded {
			haveMin = true
			minEncoded = encoded
			minMajor, minMinor, minBugfix = maj, min, bug
		}
	}
	return minMajor, minMinor, minBugfix
}

// writeDVUpdateFilesLucene writes the docValuesUpdatesFiles map in Lucene wire
// format: count as BE int32, then per entry: key as BE int32 + value as
// WriteSetOfStrings.  This mirrors CodecUtil.writeBEInt used by Lucene Java.
func writeDVUpdateFilesLucene(out store.IndexOutput, m map[int]map[string]struct{}) error {
	if err := store.WriteInt32(out, int32(len(m))); err != nil {
		return err
	}
	for k, v := range m {
		if err := store.WriteInt32(out, int32(k)); err != nil {
			return err
		}
		if err := store.WriteSetOfStrings(out, v); err != nil {
			return err
		}
	}
	return nil
}

// ReadSegmentInfos reads the SegmentInfos from a directory, locating the most
// recent segments_N file.
//
// Two formats are accepted:
//   - Lucene 10.4.0 format (codecMagic 0x3FD76C17): written by the current
//     WriteSegmentInfos and by real Apache Lucene 10.4.0.  In-memory Gocene
//     extensions are restored from userData if the "_gocene_fiv" key is present.
//   - Legacy Gocene stub format (magic 0x3d767): written by older versions of
//     WriteSegmentInfos; retained for backward compatibility only.
func ReadSegmentInfos(directory store.Directory) (*SegmentInfos, error) {
	files, err := directory.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listing directory: %w", err)
	}

	var maxGen int64 = -1
	var latestFile string
	for _, file := range files {
		if len(file) > 9 && file[:9] == "segments_" {
			if gen, err2 := strconv.ParseInt(file[9:], 36, 64); err2 == nil {
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

	rawIn, err := directory.OpenInput(latestFile, store.IOContextRead)
	if err != nil {
		return nil, err
	}

	// Peek at the first 4 bytes to determine the format without consuming them
	// from a non-seekable stream.  Both formats write an int32 as their first
	// 4 bytes, so we can inspect rawIn directly via ReadInt32 and then branch.
	magic, err := store.ReadInt32(rawIn)
	if err != nil {
		_ = rawIn.Close()
		return nil, fmt.Errorf("reading segments magic: %w", err)
	}

	switch magic {
	case CodecMagic: // 0x3FD76C17 — Lucene 10.4.0 / current Gocene format
		// readSegmentInfosLucene104 closes rawIn itself (it re-opens the file).
		return readSegmentInfosLucene104(rawIn, directory, maxGen)
	case 0x3d767: // legacy Gocene stub format
		defer rawIn.Close()
		return readSegmentInfosLegacy(rawIn, directory, maxGen)
	default:
		_ = rawIn.Close()
		return nil, fmt.Errorf("invalid segments file magic: 0x%x", uint32(magic))
	}
}

// ReadSegmentInfosFromHandle reads a Lucene 10.4.0-format segments_N file
// given a partially-consumed handle (the caller already peeked the magic
// word) and the resolved generation. The supplied rawIn is closed by this
// function; a fresh handle is opened internally to cover all bytes from
// offset 0 for checksum verification.
//
// Exported for the package index ListCommits path, which enumerates prior
// commits by directory name and therefore needs to feed in a specific
// generation rather than the latest one.
func ReadSegmentInfosFromHandle(rawIn store.IndexInput, directory store.Directory, generation int64) (*SegmentInfos, error) {
	return readSegmentInfosLucene104(rawIn, directory, generation)
}

// readSegmentInfosLucene104 reads a segments_N file in Lucene 10.4.0 format.
// rawIn must be closed by the caller; this function opens its own fresh handle
// so the ChecksumIndexInput covers all bytes from offset 0 (including the magic
// word that was already peeked by the dispatch function).
func readSegmentInfosLucene104(rawIn store.IndexInput, directory store.Directory, maxGen int64) (*SegmentInfos, error) {
	// Close the partially-consumed handle from the caller; we re-open below.
	_ = rawIn.Close()

	in2, err := directory.OpenInput(GetSegmentFileName(maxGen), store.IOContextRead)
	if err != nil {
		return nil, err
	}
	checksumIn := store.NewChecksumIndexInput(in2)
	defer checksumIn.Close()

	suffix := strconv.FormatInt(maxGen, 36)
	if _, err := CheckIndexHeader(checksumIn, "segments", 10, 10, nil, suffix); err != nil {
		return nil, fmt.Errorf("segments_N header: %w", err)
	}

	// Lucene version.
	major, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}
	minor, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}
	bugfix, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}

	// Index created major.
	createdMajor, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}

	// Index version.
	version, err := store.ReadInt64(checksumIn)
	if err != nil {
		return nil, err
	}

	// Segment counter.
	counter, err := store.ReadVLong(checksumIn)
	if err != nil {
		return nil, err
	}

	// Number of segments.
	numSegments, err := store.ReadInt32(checksumIn)
	if err != nil {
		return nil, err
	}
	if numSegments < 0 {
		return nil, fmt.Errorf("invalid segment count: %d", numSegments)
	}

	// Skip min-segment version triplet (present only when numSegments > 0).
	if numSegments > 0 {
		if _, err := store.ReadVInt(checksumIn); err != nil {
			return nil, err
		}
		if _, err := store.ReadVInt(checksumIn); err != nil {
			return nil, err
		}
		if _, err := store.ReadVInt(checksumIn); err != nil {
			return nil, err
		}
	}

	si := NewSegmentInfos()
	si.generation = maxGen
	si.lastGeneration = maxGen
	si.version = version
	si.indexCreatedVersionMajor = createdMajor
	si.luceneVersion = fmt.Sprintf("%d.%d.%d", major, minor, bugfix)
	si.counter = counter

	for i := int32(0); i < numSegments; i++ {
		sci, err := readSegmentCommitInfoLucene104(checksumIn, directory)
		if err != nil {
			return nil, fmt.Errorf("reading segment %d: %w", i, err)
		}
		si.segments = append(si.segments, sci)
		// Derive the index-level sort from the per-segment .si numSortFields
		// block (rmp #4789): every segment shares the same index sort, so the
		// first segment that carries one is authoritative. This replaces the
		// segments_N _gocene_sort_* userData round-trip for AddIndexes
		// sort-compat validation.
		if si.inMemoryIndexSort == nil {
			if sort := sci.segmentInfo.IndexSort(); sort != nil && len(sort.Fields()) > 0 {
				si.inMemoryIndexSort = sort
			}
		}
	}

	userData, err := store.ReadMapOfStrings(checksumIn)
	if err != nil {
		return nil, err
	}

	if _, err := CheckFooter(checksumIn); err != nil {
		return nil, fmt.Errorf("segments_N footer: %w", err)
	}

	// Restore in-memory Gocene extensions from userData when present.
	if userData["_gocene_fiv"] == goceneExtVersion {
		if err := restoreGoceneExtensions(si, userData); err != nil {
			return nil, err
		}
		// Remove Gocene-private keys before exposing userData to callers.
		cleanedUserData := make(map[string]string)
		for k, v := range userData {
			if !strings.HasPrefix(k, "_gocene_") {
				cleanedUserData[k] = v
			}
		}
		si.userData = cleanedUserData
	} else {
		si.userData = userData
	}

	return si, nil
}

// readSegmentCommitInfoLucene104 reads a single per-segment entry from a
// segments_N body in Lucene 10.4.0 format.
func readSegmentCommitInfoLucene104(in store.IndexInput, directory store.Directory) (*SegmentCommitInfo, error) {
	name, err := store.ReadString(in)
	if err != nil {
		return nil, err
	}

	id, err := in.ReadBytesN(16)
	if err != nil {
		return nil, err
	}

	codec, err := store.ReadString(in)
	if err != nil {
		return nil, err
	}

	// Min version: hasMinVersion byte then VInt major/minor/bugfix.
	delGen, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	delCount, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}

	fieldInfosGen, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	docValuesGen, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	softDelCount, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}

	hasSciID, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	var sciID []byte
	if hasSciID == 1 {
		sciID, err = in.ReadBytesN(16)
		if err != nil {
			return nil, err
		}
	}

	fieldInfosFiles, err := store.ReadSetOfStrings(in)
	if err != nil {
		return nil, err
	}

	// dvUpdateFiles: Lucene writes the count as a BE int32 (CodecUtil.writeBEInt),
	// not as a VInt.  The individual keys are also BE int32.  We cannot use
	// store.ReadMapOfIntToSetOfStrings here because that helper uses ReadVInt.
	docValuesUpdatesFiles, err := readDVUpdateFilesLucene(in)
	if err != nil {
		return nil, err
	}

	segInfo := schema.NewSegmentInfo(name, 0, directory)
	segInfo.SetID(id)
	segInfo.SetCodec(codec)
	// Reconstruct the expected file list so CheckIndex can detect missing files.
	segInfo.SetFiles([]string{name + ".si"})

	// Prefer the on-disk .si as the authoritative source of per-segment
	// metadata (docCount, version, compound flag, file set), mirroring
	// org.apache.lucene.index.SegmentInfos.parseSegmentInfos which calls
	// codec.segmentInfoFormat().read for every segment. When a reader hook is
	// registered (real codec linked) and the .si exists, its values replace the
	// placeholders above. The legacy _gocene_dc_ userData key remains as a
	// fallback for codec-less indices that never wrote a real .si.
	if read := lookupSegmentInfoReader(); read != nil {
		if full, err := read(directory, name, id); err == nil && full != nil {
			// The .si file does not carry the codec name (that lives in the
			// segments_N record we just read) nor the segment id, so carry both
			// forward from the placeholder onto the authoritative SegmentInfo.
			full.SetCodec(codec)
			full.SetID(id)
			segInfo = full
		}
	}

	sci := NewSegmentCommitInfo(segInfo, int(delCount), delGen)
	sci.SetFieldInfosGen(fieldInfosGen)
	sci.SetDocValuesGen(docValuesGen)
	sci.SetSoftDelCount(int(softDelCount))
	sci.SetID(sciID)
	sci.SetFieldInfosFiles(fieldInfosFiles)
	sci.SetDocValuesUpdatesFiles(docValuesUpdatesFiles)

	return sci, nil
}

// restoreGoceneExtensions unpacks in-memory Gocene state from the userData map
// into si and its segments.  The userData map is read-only here; the caller
// strips the _gocene_* keys before storing userData on si.
func restoreGoceneExtensions(si *SegmentInfos, userData map[string]string) error {
	// Parent field: NO LONGER restored from userData (rmp #4789). The
	// authoritative source is the per-segment .fnm parent bit, consulted on
	// demand by AddIndexes (sourceParentFieldFromDisk). inMemoryParentField is
	// set by the writer at commit time from IndexWriterConfig and is not
	// persisted; after a cold reopen it remains "" (empty string), which is
	// correct because sourceParentFieldFromDisk will scan the .fnm files.

	// Index sort is NO LONGER restored from userData (rmp #4789): it is read
	// from each per-segment .si numSortFields block and the index-level sort is
	// derived from the first segment in ReadSegmentInfos, before this function
	// runs. Overwriting it here would clobber the authoritative .si value.

	// Per-segment docCount and FieldInfos are restored from disk, not from
	// userData (rmp #4785): docCount from the .si reader hook in
	// readSegmentCommitInfoLucene104, FieldInfos from the .fnm in
	// openSegmentReader. The _gocene_dc_/_gocene_fi_ keys are no longer consumed.
	//
	// Backward compatibility: when no .si reader hook is registered (legacy
	// codec-less index opened without the index package wiring) the fallback
	// below restores docCount and FieldInfos from the legacy keys so such
	// indices keep opening.
	hookRegistered := lookupSegmentInfoReader() != nil
	for _, sci := range si.segments {
		name := sci.Name()
		if !hookRegistered {
			if dcStr := userData["_gocene_dc_"+name]; dcStr != "" {
				dc, err := strconv.Atoi(dcStr)
				if err != nil {
					return fmt.Errorf("invalid _gocene_dc_%s: %w", name, err)
				}
				sci.segmentInfo.SetDocCount(dc)
			}
			if fiEnc := userData["_gocene_fi_"+name]; fiEnc != "" {
				fis, err := decodeFieldInfosFromUserData(fiEnc)
				if err != nil {
					return fmt.Errorf("decoding FieldInfos for segment %s: %w", name, err)
				}
				if fis != nil {
					sci.SetInMemoryFieldInfos(fis)
				}
			}
		}
		// Deleted ordinals are NO LONGER restored from userData (rmp #4789):
		// the byte-faithful Lucene90 .liv file is the authoritative on-disk
		// source, read back by loadLiveDocsFromDisk during the reader reopen.
		_ = name
	}

	return nil
}

// readSegmentInfosLegacy reads a segments_N body in the old Gocene stub format
// (magic 0x3d767) that was written by earlier versions of WriteSegmentInfos.
// The magic word has already been consumed from rawIn by the caller.
func readSegmentInfosLegacy(rawIn store.IndexInput, directory store.Directory, maxGen int64) (*SegmentInfos, error) {
	in := rawIn // alias for clarity; caller holds the defer Close

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

	// parentField — absent in the oldest sub-versions; tolerate gracefully.
	parentField, err := store.ReadString(in)
	if err != nil {
		si := NewSegmentInfos()
		si.generation = gen
		si.lastGeneration = gen
		si.version = version
		si.indexCreatedVersionMajor = createdMajor
		si.luceneVersion = luceneVersion
		si.counter = counter
		return si, nil
	}

	// indexSort.
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
	var indexSort *schema.Sort
	if numSortFields > 0 {
		fields := make([]schema.SortField, 0, numSortFields)
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
			fields = append(fields, schema.NewSortFieldFull(fname, schema.SortType(stRaw), descRaw != 0))
		}
		indexSort = schema.NewSortFromFields(fields)
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
		softDelCount, err := store.ReadInt32(in)
		if err != nil {
			return nil, err
		}
		id, err := in.ReadBytesN(16)
		if err != nil {
			return nil, err
		}

		segmentInfo := schema.NewSegmentInfo(name, int(docCount), directory)
		segmentInfo.SetID(id)
		segmentInfo.SetFiles([]string{name + ".si"})
		sci := NewSegmentCommitInfo(segmentInfo, int(delCount), -1)
		if softDelCount > 0 {
			sci.SetSoftDelCount(int(softDelCount))
		}

		numFields, err := store.ReadInt32(in)
		if err != nil {
			si.Add(sci)
			continue
		}
		if numFields > 0 {
			fis := schema.NewFieldInfos()
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
				opts := schema.FieldInfoOptions{
					IndexOptions:             schema.IndexOptions(ioRaw),
					DocValuesType:            schema.DocValuesType(dvRaw),
					DocValuesSkipIndexType:   schema.DocValuesSkipIndexTypeNone,
					DocValuesGen:             -1,
					Stored:                   flags&1 != 0,
					Tokenized:                flags&2 != 0,
					StoreTermVectors:         flags&4 != 0,
					OmitNorms:                flags&8 != 0,
					VectorEncoding:           schema.VectorEncodingFloat32,
					VectorSimilarityFunction: schema.VectorSimilarityFunctionEuclidean,
				}
				fi := schema.NewFieldInfo(fname, int(fnum), opts)
				_ = fis.Add(fi)
			}
			sci.SetInMemoryFieldInfos(fis)
		}
		numDel, err := store.ReadInt32(in)
		if err != nil {
			si.Add(sci)
			continue
		}
		if numDel > 0 {
			ords := make([]int, numDel)
			for j := int32(0); j < numDel; j++ {
				ord, err := store.ReadInt32(in)
				if err != nil {
					return nil, fmt.Errorf("reading deleted ordinal: %w", err)
				}
				ords[j] = int(ord)
			}
			sci.SetDeletedOrdinals(ords)
		}
		si.Add(sci)
	}

	return si, nil
}
