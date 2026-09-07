// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// DirectoryReader is a CompositeReader that reads from a Directory.

//
// This is the Go port of Lucene's org.apache.lucene.index.DirectoryReader.
//
// DirectoryReader is the main implementation of CompositeReader for reading
// indexes stored in a Directory. It manages a collection of SegmentReaders
// (each wrapping a LeafReader) for all segments in the index.
type DirectoryReader struct {
	*CompositeReader

	// directory is the source directory
	directory store.Directory

	// segmentInfos contains information about all segments
	segmentInfos *SegmentInfos

	// readers holds readers for each segment
	readers []*SegmentReader

	// lastCommittedInfos holds the last committed segment infos
	lastCommittedInfos *SegmentInfos

	// readerContext is the context for this reader
	readerContext IndexReaderContext

	// nrtGen records the IndexWriter NRT generation at the moment this
	// reader was created from a writer.  Zero means commit-pinned reader.
	// Used by OpenIfChangedFromWriter for efficient change detection.
	nrtGen int64

	// writer holds the live IndexWriter this reader was opened from when it is
	// an NRT reader.  It allows IsCurrent to consult the writer's pending state
	// instead of comparing against the on-disk commit generation, which is
	// always stale for an in-memory NRT snapshot.
	writer *IndexWriter
}

// GetOnlyLeafReader returns the single leaf reader contained in reader.
// It panics if reader has zero or more than one leaf. This mirrors Lucene's
// LuceneTestCase.getOnlyLeafReader helper used by many index tests.
func GetOnlyLeafReader(reader IndexReaderInterface) IndexReaderInterface {
	leaves, err := reader.Leaves()
	if err != nil {
		panic(fmt.Sprintf("GetOnlyLeafReader: %v", err))
	}
	if len(leaves) != 1 {
		panic(fmt.Sprintf("GetOnlyLeafReader: expected exactly one leaf, got %d", len(leaves)))
	}
	return leaves[0].Reader()
}

// Open opens a DirectoryReader for the given directory.
func OpenDirectoryReader(directory store.Directory) (*DirectoryReader, error) {
	// Read segment infos; an empty (freshly created) directory has no segments
	// file yet — treat it as an empty index rather than an error.
	segmentInfos, err := ReadSegmentInfos(directory)
	if err != nil {
		segmentInfos = NewSegmentInfos()
	}

	return OpenDirectoryReaderWithInfos(directory, segmentInfos)
}

// OpenDirectoryReaderAtCommit opens a DirectoryReader for the given commit
// point. This is the Go equivalent of Lucene's
// DirectoryReader.open(IndexCommit).
func OpenDirectoryReaderAtCommit(commit *IndexCommit) (*DirectoryReader, error) {
	if commit == nil {
		return nil, fmt.Errorf("commit must not be nil")
	}
	dir := commit.GetDirectory()
	if dir == nil {
		return nil, fmt.Errorf("commit has no directory")
	}
	return OpenDirectoryReaderWithInfos(dir, commit.GetSegmentInfos())
}

// newCompositeReaderFromSegments builds a CompositeReader from a slice of SegmentReaders.
// An empty slice produces an empty composite reader (valid for an empty index).
func newCompositeReaderFromSegments(readers []*SegmentReader) (*CompositeReader, error) {
	if len(readers) == 0 {
		return &CompositeReader{
			IndexReader: NewIndexReader(),
			subReaders:  []IndexReaderInterface{},
			starts:      []int{0},
		}, nil
	}
	subReaders := make([]IndexReaderInterface, len(readers))
	for i, r := range readers {
		subReaders[i] = r
	}
	return NewCompositeReaderWithSubReaders(subReaders)
}

// openSegmentReader creates a SegmentReader for one SegmentCommitInfo, loading
// FieldInfos from disk (if not already in memory) and constructing
// SegmentCoreReaders from the codec registered under the segment's codec name.
//
// If no codec is registered under that name (e.g., no bridge import in the
// test binary) the reader falls back to the codec-less path so structural
// tests that do not need stored-field access continue to work.
func openSegmentReader(directory store.Directory, sci *SegmentCommitInfo) (*SegmentReader, error) {
	// Resolve the codec for this segment. Prefer the segment's own codec name;
	// fall back to the registered default when the name is absent (freshly-
	// created in-memory segments have no on-disk codec name yet).
	codecName := sci.SegmentInfo().Codec()
	var codec Codec
	if codecName != "" {
		codec = LookupCodecByName(codecName)
		if codec == nil {
			// Codec name was stamped but not registered: fall back to default.
			codec = GetDefaultCodec()
		}
	}
	// When no codec name is stamped on the segment (in-memory or codec-less
	// write path), do not resolve a default codec. Attempting to open codec
	// files for a segment that was written without them would produce
	// "file not found" errors. The caller (IndexSearcher.Doc, term searches)
	// will receive nil coreReaders and take the in-memory fallback.

	// Load the full SegmentInfo from the .si file to obtain metadata not present
	// in the segments_N entry (isCompoundFile, file set, docCount).  Fall back
	// to the segments_N-constructed SegmentInfo when the .si is absent
	// (in-memory segments that were never flushed to disk).
	segInfo := sci.SegmentInfo()
	if codec != nil {
		if sif := codec.SegmentInfoFormat(); sif != nil {
			if fullSegInfo, err := sif.Read(directory, segInfo.Name(), segInfo.GetID(), store.IOContextRead); err == nil {
				segInfo = fullSegInfo
			}
		}
	}

	// Resolve FieldInfos: prefer in-memory (carried by a freshly-written
	// in-memory segment), otherwise read the authoritative .fnm from disk via
	// the codec's FieldInfosFormat (rmp #4785).
	fi := sci.GetInMemoryFieldInfos()
	if fi == nil && codec != nil {
		fi = readFieldInfosFromDisk(directory, codec, segInfo)
	}
	if fi == nil {
		fi = NewFieldInfos()
	}
	// Keep the base FieldInfos (the one the segment's data files were written
	// against) so the core readers are wired to the original fields/generations.
	// Updated FieldInfos are overlaid below for the exposed SegmentReader view.
	baseFi := fi

	// When the segment has an updated FieldInfos generation (e.g. after a
	// doc-values update), read the newer .fnm so that GetNumericDocValues
	// and other doc-values accessors resolve the correct fields. The codec
	// producers (FieldsProducer, StoredFieldsReader, etc.) were written
	// against the base FieldInfos and stay wired to the core created below;
	// the SegmentReader's exposed fieldInfos may be newer. Mirrors Java
	// SegmentReader.initFieldInfos().
	var updatedFi *FieldInfos
	if fi != nil && sci.HasFieldInfosGen() {
		updatedFi = readFieldInfosWithGen(directory, codec, segInfo, sci.FieldInfosGen())
	}
	if updatedFi != nil {
		fi = updatedFi
	}

	// Construct SegmentCoreReaders when a codec is available. This wires the
	// StoredFieldsReader, FieldsProducer (postings), and TermVectorsReader so
	// that IndexSearcher.Doc and term-based searches work end-to-end.
	if codec != nil {
		core, err := NewSegmentCoreReaders(directory, segInfo, baseFi, codec, store.IOContextRead)
		if err == nil {
			sr := NewSegmentReaderWithCore(sci, core, fi, codec)
			sr.directory = directory

			// When any field carries a doc-values generation > -1 the base
			// producer loaded by NewSegmentCoreReaders reads the original files
			// (_0.dvd) but misses the updated ones (_0_1_Lucene90_0.dvd).
			// Overlay a SegmentDocValuesProducer that fans out to the correct
			// per-generation producer for every field.  Mirrors Java
			// SegmentReader's initFieldInfos() + SegmentCoreReaders wiring.
			if fi != nil && fi.HasDocValues() {
				var hasGen bool
				it := fi.Iterator()
				for it.HasNext() {
					if it.Next().DocValuesGen() > -1 {
						hasGen = true
						break
					}
				}
				if hasGen {
					coreInfos := readFieldInfosFromDisk(directory, codec, segInfo)
					if coreInfos == nil {
						coreInfos = NewFieldInfos()
					}
					factory := func(si *SegmentCommitInfo, dir store.Directory, gen int64, infos *FieldInfos) (DocValuesProducer, error) {
						// The base (gen == -1) producer is already open inside the core readers,
						// possibly against the compound-file directory. Reuse it rather than
						// reopening against |directory|, which would miss files living inside
						// the .cfs. Per-generation update files are always written to (and
						// read from) the raw top-level directory.
						if gen == -1 {
							if base, ok := core.docValuesProducer.(DocValuesProducer); ok && base != nil {
								return base, nil
							}
							return nil, fmt.Errorf("base doc-values producer missing for segment %s", si.Name())
						}
						suffix := strconv.FormatInt(gen, 36)
						state := &SegmentReadState{
							Directory:     directory,
							SegmentInfo:   si.SegmentInfo(),
							FieldInfos:    infos,
							SegmentSuffix: suffix,
						}
						format := codec.DocValuesFormat()
						if format == nil {
							return nil, fmt.Errorf("no DocValuesFormat for codec %q", codec.Name())
						}
						return format.FieldsProducer(state)
					}
					sdv, err := NewSegmentDocValues(factory)
					if err != nil {
						_ = core.DecRef()
						return nil, fmt.Errorf("openSegmentReader: SegmentDocValues init failed: %w", err)
					}
					dvp, err := NewSegmentDocValuesProducer(sci, directory, coreInfos, fi, sdv)
					if err != nil {
						_ = core.DecRef()
						return nil, fmt.Errorf("openSegmentReader: SegmentDocValuesProducer init failed: %w", err)
					}
					// Do not close the base producer: the overlay keeps it alive and the
					// core's own DecRef path will release it when the SegmentReader is
					// discarded.
					core.SetDocValuesProducer(dvp)
				}
			}

			loadLiveDocsFromDisk(directory, sci)
			return sr, nil
		}
		// Wiring the core readers failed. Distinguish a genuine failure from the
		// benign metadata-only segment case (rmp #4):
		//
		//   - If the segment owns per-format data files (a compound .cfs, or any
		//     non-metadata file), the codec readers MUST open; a failure here means
		//     a corrupt or truncated segment, so surface an explicit error instead
		//     of silently returning a data-less reader that would make DocValues
		//     read-back (and every other accessor) appear empty.
		//
		//   - If the segment carries only .si/.fnm it is a metadata-only segment:
		//     an AddIndexes-imported placeholder (backlog #2707) or a not-yet-
		//     data-merged ForceMerge result (the real merge that writes the merged
		//     postings/stored-fields/doc-values is tracked separately). For those we
		//     fall through to the FieldInfos-only reader, preserving the structural
		//     reopen until the merge write-path lands.
		if segmentHasDataFiles(segInfo) {
			return nil, fmt.Errorf("openSegmentReader: segment %q is codec-backed with data files but its core readers could not be opened: %w", segInfo.Name(), err)
		}
	}

	// Codec-less / data-less fallback: expose the .si docCount and the .fnm
	// FieldInfos without core readers.
	sr := &SegmentReader{
		LeafReader:        NewLeafReader(segInfo),
		segmentCommitInfo: sci,
		fieldInfos:        fi,
		directory:         directory,
	}
	loadLiveDocsFromDisk(directory, sci)
	return sr, nil
}

// segmentHasDataFiles reports whether segInfo owns any per-format document-data
// file (as opposed to only the .si segment-info and .fnm field-infos metadata).
// A compound segment always carries data (its .cfs). For a non-compound segment
// any file whose extension is not .si, .fnm or .cfe is a data file (postings,
// stored fields, doc values, points, vectors, norms, term vectors). Used by
// openSegmentReader to decide whether a core-readers wiring failure is a genuine
// corruption (data present but unreadable) or a benign metadata-only segment.
func segmentHasDataFiles(segInfo *SegmentInfo) bool {
	if segInfo.IsCompoundFile() {
		return true
	}
	for _, f := range segInfo.Files() {
		switch {
		case strings.HasSuffix(f, ".si"), strings.HasSuffix(f, ".fnm"), strings.HasSuffix(f, ".cfe"):
			// Metadata only: not document data.
		default:
			return true
		}
	}
	return false
}

// readFieldInfosFromDisk reads the authoritative .fnm FieldInfos for a segment
// from disk via the codec's FieldInfosFormat (rmp #4785). For a compound
// segment the .fnm lives inside the .cfs, so the read is routed through the
// compound directory; reading it from the top-level directory would find no
// .fnm and yield an empty FieldInfos, hiding every indexed field from the
// reopened reader. Returns nil when no codec FieldInfosFormat is available or
// the read fails (caller substitutes an empty FieldInfos).
func readFieldInfosFromDisk(directory store.Directory, codec Codec, segInfo *SegmentInfo) *FieldInfos {
	if codec == nil {
		return nil
	}
	fif := codec.FieldInfosFormat()
	if fif == nil {
		return nil
	}
	fnmDir := directory
	var cfsReader store.Directory
	if segInfo.IsCompoundFile() {
		if cf := codec.CompoundFormat(); cf != nil {
			if r, err := cf.GetCompoundReader(directory, segInfo); err == nil {
				fnmDir = r
				cfsReader = r
			}
		}
	}
	if cfsReader != nil {
		defer func() {
			if closer, ok := cfsReader.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		}()
	}
	fi, err := fif.Read(fnmDir, segInfo, "", store.IOContextRead)
	if err != nil {
		return nil
	}
	return fi
}

// readFieldInfosWithGen reads the .fnm FieldInfos for a segment at the given
// field-infos generation. When a doc-values update bumps the fieldInfosGen,
// Lucene writes a new _N_G.fnm file (where G is the generation in base-36)
// outside the segment's compound file; the base _N.fnm lives inside the .cfs.
// This helper therefore reads from the top-level directory, not from inside
// the compound file. It mirrors Java SegmentReader.initFieldInfos(). Returns
// nil when the read fails (caller falls back to the base FieldInfos).
func readFieldInfosWithGen(directory store.Directory, codec Codec, segInfo *SegmentInfo, gen int64) *FieldInfos {
	if codec == nil {
		return nil
	}
	fif := codec.FieldInfosFormat()
	if fif == nil {
		return nil
	}
	suffix := strconv.FormatInt(gen, 36)
	fi, err := fif.Read(directory, segInfo, suffix, store.IOContextRead)
	if err != nil {
		return nil
	}
	return fi
}

// loadLiveDocsFromDisk reads the segment's .liv file (when the segment has a
// non-default delGen) and records the deleted ordinals on the SegmentCommitInfo
// so SegmentReader.GetLiveDocs and SegmentCommitInfo.NumDocs reflect the
// on-disk deletions. This replaces the legacy _gocene_del userData round-trip
// (rmp #4785) by making the byte-faithful Lucene90 .liv file authoritative.
//
// Best-effort: a missing or unreadable .liv leaves the existing (possibly
// empty) deleted-ordinal state untouched, so segments with no deletions and
// codec-less in-memory segments are unaffected.
func loadLiveDocsFromDisk(directory store.Directory, sci *SegmentCommitInfo) {
	if sci == nil {
		return
	}
	// Skip when the segment already carries deleted ordinals (e.g. freshly
	// written in-memory state that has not yet round-tripped through disk) or
	// when there is no deletion generation to read.
	if len(sci.GetDeletedOrdinals()) > 0 {
		return
	}
	if sci.DelGen() < 0 || sci.DelCount() == 0 {
		return
	}
	segInfo := sci.SegmentInfo()
	maxDoc := segInfo.DocCount()
	if maxDoc <= 0 {
		return
	}
	bits, err := readLiveDocs(directory, segInfo.Name(), segInfo.GetID(), sci.DelGen(), maxDoc)
	if err != nil || bits == nil {
		return
	}
	ords := make([]int, 0, sci.DelCount())
	for doc := 0; doc < maxDoc; doc++ {
		if !bits.Get(doc) {
			ords = append(ords, doc)
		}
	}
	if len(ords) > 0 {
		sci.SetDeletedOrdinals(ords)
	}
}

// OpenDirectoryReaderWithInfos opens a DirectoryReader with existing SegmentInfos.
func OpenDirectoryReaderWithInfos(directory store.Directory, segmentInfos *SegmentInfos) (*DirectoryReader, error) {
	size := segmentInfos.Size()
	readers := make([]*SegmentReader, size)
	errors := make([]error, size)
	var wg sync.WaitGroup

	for i := 0; i < size; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sr, err := openSegmentReader(directory, segmentInfos.Get(idx))
			if err != nil {
				errors[idx] = err
				return
			}
			readers[idx] = sr
		}(i)
	}
	wg.Wait()

	// Check for errors and close any successfully opened readers on failure.
	for i, err := range errors {
		if err != nil {
			for _, opened := range readers {
				if opened != nil {
					opened.Close() //nolint:errcheck
				}
			}
			return nil, err
		}
	}

	compReader, err := newCompositeReaderFromSegments(readers)
	if err != nil {
		for _, opened := range readers {
			if opened != nil {
				opened.Close() //nolint:errcheck
			}
		}
		return nil, err
	}

	return &DirectoryReader{
		CompositeReader: compReader,
		directory:       directory,
		segmentInfos:    segmentInfos,
		readers:         readers,
		nrtGen:          0, // commit-pinned reader
	}, nil
}

// OpenDirectoryReaderFromCommit opens a DirectoryReader from a specific commit.
func OpenDirectoryReaderFromCommit(directory store.Directory, commit *IndexCommit) (*DirectoryReader, error) {
	if commit == nil {
		return nil, fmt.Errorf("commit cannot be nil")
	}

	segmentInfos := commit.GetSegmentInfos()
	if segmentInfos == nil {
		return nil, fmt.Errorf("commit has no segment infos")
	}

	return OpenDirectoryReaderWithInfos(directory, segmentInfos)
}

// OpenDirectoryReaderFromWriter opens a near-real-time DirectoryReader
// directly from a live IndexWriter — the Go analogue of Lucene's
// DirectoryReader.open(IndexWriter). The returned reader reflects every
// document added to the writer so far, including documents still
// buffered and not yet durably committed.
//
// See IndexWriter.GetReader for the NRT contract and the (single) way
// this port currently differs from Lucene's pooled in-memory NRT path.
func OpenDirectoryReaderFromWriter(writer *IndexWriter) (*DirectoryReader, error) {
	if writer == nil {
		return nil, fmt.Errorf("OpenDirectoryReaderFromWriter: writer must not be nil")
	}
	return writer.GetReader()
}

// OpenDirectoryReaderFromWriterWithOptions opens a near-real-time
// DirectoryReader from a live IndexWriter, matching the Lucene overload
// DirectoryReader.open(IndexWriter, applyAllDeletes, writeAllDeletes).
// Gocene's NRT reader always applies all buffered deletes to the returned
// snapshot, so the flags are accepted for API compatibility but do not change
// the reader's behavior.
func OpenDirectoryReaderFromWriterWithOptions(writer *IndexWriter, applyAllDeletes, writeAllDeletes bool) (*DirectoryReader, error) {
	_ = applyAllDeletes
	_ = writeAllDeletes
	return OpenDirectoryReaderFromWriter(writer)
}

// OpenIfChangedFromWriter reopens old against the current state of a live
// IndexWriter — the Go analogue of DirectoryReader.openIfChanged(reader,
// writer). It returns a fresh NRT reader when the writer holds changes
// not yet reflected by old, or (nil, nil) when old is already current
// (matching Lucene's null return). The caller retains ownership of old in
// both cases.
func OpenIfChangedFromWriter(old *DirectoryReader, writer *IndexWriter) (*DirectoryReader, error) {
	if writer == nil {
		return nil, fmt.Errorf("OpenIfChangedFromWriter: writer must not be nil")
	}
	if old != nil && old.nrtGen > 0 {
		// NRT reader path: compare the reader snapshot gen against the
		// writer current counter.  When both match and no uncommitted
		// changes exist, nothing happened since this reader was opened.
		if old.nrtGen == writer.GetNRTGeneration() && !writer.hasUncommittedChanges() {
			return nil, nil
		}
	}
	// Fall back: if writer has no uncommitted changes and committed gen
	// matches old, nothing changed.
	if old != nil && !writer.hasUncommittedChanges() {
		if cur, err := ReadSegmentInfos(writer.directory); err == nil &&
			old.segmentInfos != nil && cur.Generation() == old.segmentInfos.Generation() {
			return nil, nil
		}
	}
	return writer.GetReader()
}

// Reopen reopens the index to see if any changes have been made.
func (r *DirectoryReader) Reopen() (*DirectoryReader, error) {
	newReader, err := r.doOpenIfChanged(nil, nil)
	if err != nil {
		return nil, err
	}
	if newReader == nil {
		return r, nil
	}
	return newReader, nil
}

// IsCurrent returns true if the reader is still up to date with the index.
func (r *DirectoryReader) IsCurrent() (bool, error) {
	if r.directory == nil {
		// No backing directory — treat as always current (nothing can change it).
		return true, nil
	}

	// NRT reader: ask the live writer whether any uncommitted changes have
	// happened since this snapshot was opened.
	if r.writer != nil {
		if r.nrtGen > 0 {
			return r.nrtGen == r.writer.GetNRTGeneration() && !r.writer.hasUncommittedChanges(), nil
		}
		// Commit-pinned reader owned by a writer: treat it as current only when
		// the writer has no uncommitted changes and the disk generation matches.
		if r.writer.hasUncommittedChanges() {
			return false, nil
		}
	}

	segmentInfos, err := ReadSegmentInfos(r.directory)
	if err != nil {
		// No segments file means the directory is still empty — still current.
		return true, nil
	}
	return segmentInfos.Generation() == r.segmentInfos.Generation(), nil
}

// ReopenFromCommit reopens the index from a specific commit.
func (r *DirectoryReader) ReopenFromCommit(commit *IndexCommit) (*DirectoryReader, error) {
	return OpenDirectoryReaderFromCommit(r.directory, commit)
}

// GetDirectory returns the directory being read.
func (r *DirectoryReader) GetDirectory() store.Directory {
	return r.directory
}

// GetSegmentInfos returns the SegmentInfos for this reader.
func (r *DirectoryReader) GetSegmentInfos() *SegmentInfos {
	return r.segmentInfos
}

// GetIndexCommit returns the IndexCommit that this reader is reading from.
// The returned commit carries a reference back to this reader so that
// IndexWriter can detect stale or closed NRT reader commits.
func (r *DirectoryReader) GetIndexCommit() *IndexCommit {
	if r.segmentInfos == nil {
		return nil
	}
	commit := NewIndexCommit(r.segmentInfos)
	commit.SetDirectory(r.directory)
	// Preserve the reader reference for the NRT reopen path. The commit is
	// obtained from a live reader, so this is safe; if the reader is later
	// closed the IndexWriter constructor will detect it via EnsureOpen.
	commit.SetReader(r)
	return commit
}

// GetSegmentReaders returns the SegmentReaders.
func (r *DirectoryReader) GetSegmentReaders() []*SegmentReader {
	return r.readers
}

// NumDocs returns the total number of live documents across all segments.
func (r *DirectoryReader) NumDocs() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.NumDocs()
	}
	return total
}

// MaxDoc returns the maximum document ID across all segments.
func (r *DirectoryReader) MaxDoc() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.MaxDoc()
	}
	return total
}

// DocCount returns the total document count across all segments.
func (r *DirectoryReader) DocCount() int {
	return r.NumDocs()
}

// GetFieldInfos returns the merged FieldInfos across all segments.
//
// Mirrors the contract of FieldInfos.getMergedFieldInfos in Lucene: it unions
// every FieldInfo from every segment, returning a non-nil *FieldInfos even for
// an empty index. Callers that need per-segment granularity should iterate
// Leaves() and call GetFieldInfos() on each LeafReader.
func (r *DirectoryReader) GetFieldInfos() *FieldInfos {
	merged := NewFieldInfos()
	for _, sr := range r.readers {
		fi := sr.GetFieldInfos()
		if fi == nil {
			continue
		}
		it := fi.Iterator()
		for {
			info := it.Next()
			if info == nil {
				break
			}
			// Ignore errors: a duplicate field number from a different segment is
			// legal when merging (same field may appear in multiple segments).
			_ = merged.Add(info)
		}
	}
	return merged
}

// Close closes the DirectoryReader and all segment readers.
func (r *DirectoryReader) Close() error {
	if err := r.EnsureOpen(); err != nil {
		return err
	}

	var lastErr error
	for _, reader := range r.readers {
		if err := reader.Close(); err != nil {
			lastErr = err
		}
	}
	r.readers = nil

	// Close the embedded IndexReader so its cache-helper closed listeners are
	// notified and the closed flag is set. This mirrors Lucene's
	// DirectoryReader.doClose() super-call to IndexReader.close().
	if err := r.IndexReader.Close(); err != nil {
		if lastErr == nil {
			lastErr = err
		}
	}
	return lastErr
}

// GetTermVectors returns term vectors for the given document across all segments.
// Note: This requires mapping the document ID to the correct segment.
// For a DirectoryReader, document IDs are sequential across segments.
func (r *DirectoryReader) GetTermVectors(docID int) (Fields, error) {
	// Find the correct segment for this document ID
	remainingDocID := docID
	for _, reader := range r.readers {
		maxDoc := reader.MaxDoc()
		if remainingDocID < maxDoc {
			return reader.GetTermVectors(remainingDocID)
		}
		remainingDocID -= maxDoc
	}
	return nil, fmt.Errorf("document ID %d out of range", docID)
}

// Terms returns the Terms for a field, merging across all segments.
func (r *DirectoryReader) Terms(field string) (Terms, error) {
	return compositeTermsForField(r.readers, field)
}

// compositeTermsForField returns a Terms view of field merged across every
// segment that contains it: a single sub is returned directly, two or more are
// aggregated through a MultiTerms whose ReaderSlices carry each segment's
// composite docID base so postings read back through the merged doc space.
// Returns (nil, nil) when no segment has the field.
//
// Previously DirectoryReader.Terms returned only the first segment's Terms,
// which silently hid every term unique to a later segment — breaking multi-term
// query rewrites (PrefixQuery/WildcardQuery/RangeQuery/...) over a multi-segment
// index (rmp #18 / #123).
func compositeTermsForField(readers []*SegmentReader, field string) (Terms, error) {
	var subs []Terms
	var slices []ReaderSlice
	docBase := 0
	for i, sr := range readers {
		if sr == nil {
			continue
		}
		maxDoc := sr.MaxDoc()
		terms, err := sr.Terms(field)
		if err != nil {
			return nil, err
		}
		if terms != nil {
			subs = append(subs, terms)
			slices = append(slices, ReaderSlice{Start: docBase, Length: maxDoc, ReaderIndex: i})
		}
		docBase += maxDoc
	}
	switch len(subs) {
	case 0:
		return nil, nil
	case 1:
		return subs[0], nil
	default:
		return NewMultiTermsForField(field, subs, slices)
	}
}

// GetLiveDocs returns a bitset of live documents.
// Returns nil if there are no deletions.
func (r *DirectoryReader) GetLiveDocs() util.Bits {
	if !r.HasDeletions() {
		return nil
	}

	subs := make([]util.Bits, 0, len(r.readers))
	starts := make([]int, 0, len(r.readers)+1)
	docBase := 0

	for _, reader := range r.readers {
		subs = append(subs, reader.GetLiveDocs())
		starts = append(starts, docBase)
		docBase += reader.MaxDoc()
	}
	starts = append(starts, docBase)

	return NewMultiBits(subs, starts)
}

// GetSequentialSubReaders returns the sequential sub-readers.
func (r *DirectoryReader) GetSequentialSubReaders() []*SegmentReader {
	return r.readers
}

// GetReaderCount returns the number of segment readers.
func (r *DirectoryReader) GetReaderCount() int {
	return len(r.readers)
}

// GetLastCommit returns the last committed segment infos.
func (r *DirectoryReader) GetLastCommit() *SegmentInfos {
	return r.lastCommittedInfos
}

// HasDeletions returns true if any segment has deleted documents.
func (r *DirectoryReader) HasDeletions() bool {
	for _, reader := range r.readers {
		if reader.HasDeletions() {
			return true
		}
	}
	return false
}

// NumDeletedDocs returns the total number of deleted documents across all segments.
func (r *DirectoryReader) NumDeletedDocs() int {
	total := 0
	for _, reader := range r.readers {
		total += reader.NumDeletedDocs()
	}
	return total
}

// EnsureOpen throws an error if the reader is closed.
func (r *DirectoryReader) EnsureOpen() error {
	if r.closed.Load() {
		return NewAlreadyClosedException("this IndexReader is closed", nil)
	}
	return nil
}

// IncRef increments the reference count on all sub-readers.
func (r *DirectoryReader) IncRef() error {
	if err := r.EnsureOpen(); err != nil {
		return err
	}
	for _, reader := range r.readers {
		if err := reader.IncRef(); err != nil {
			return err
		}
	}
	return nil
}

// DecRef decrements the reference count on all sub-readers.
func (r *DirectoryReader) DecRef() error {
	var lastErr error
	for _, reader := range r.readers {
		if err := reader.DecRef(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// TryIncRef tries to increment the reference count.
func (r *DirectoryReader) TryIncRef() bool {
	if r.closed.Load() {
		return false
	}
	for _, reader := range r.readers {
		if !reader.TryIncRef() {
			// Rollback
			// Note: This is simplified; a full implementation would track which refs were incremented
			return false
		}
	}
	return true
}

// GetRefCount returns the minimum reference count across all sub-readers.
func (r *DirectoryReader) GetRefCount() int32 {
	if len(r.readers) == 0 {
		return 0
	}
	minRefCount := r.readers[0].GetRefCount()
	for _, reader := range r.readers[1:] {
		rc := reader.GetRefCount()
		if rc < minRefCount {
			minRefCount = rc
		}
	}
	return minRefCount
}

// StoredFields returns a StoredFields instance for accessing stored fields.
func (r *DirectoryReader) StoredFields() (StoredFields, error) {
	// For a DirectoryReader, this would need to aggregate across segments
	// Return a wrapper that delegates to the appropriate segment
	return &directoryStoredFields{reader: r}, nil
}

// TermVectors returns a TermVectors instance for accessing term vectors.
func (r *DirectoryReader) TermVectors() (TermVectors, error) {
	// For a DirectoryReader, this would need to aggregate across segments
	// Return a wrapper that delegates to the appropriate segment
	return &directoryTermVectors{reader: r}, nil
}

// GetContext returns the reader context for this directory reader.
func (r *DirectoryReader) GetContext() (IndexReaderContext, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}
	if r.readerContext == nil {
		ctx, err := buildDirectoryReaderContext(r, nil)
		if err != nil {
			return nil, err
		}
		r.readerContext = ctx
	}
	return r.readerContext, nil
}

// Leaves returns all leaf reader contexts from all segments.
func (r *DirectoryReader) Leaves() ([]*LeafReaderContext, error) {
	ctx, err := r.GetContext()
	if err != nil {
		return nil, err
	}
	return ctx.(*CompositeReaderContext).Leaves()
}

// buildDirectoryReaderContext builds the context hierarchy for a DirectoryReader.
func buildDirectoryReaderContext(reader *DirectoryReader, parent IndexReaderContext) (*CompositeReaderContext, error) {
	children := make([]IndexReaderContext, len(reader.readers))
	leaves := make([]*LeafReaderContext, 0)
	docBase := 0

	for i, subReader := range reader.readers {
		leafCtx := NewLeafReaderContext(subReader, parent, i, docBase)
		children[i] = leafCtx
		leaves = append(leaves, leafCtx)
		docBase += subReader.MaxDoc()
	}

	return NewCompositeReaderContextWithChildren(reader, parent, children, leaves), nil
}

// directoryStoredFields wraps a DirectoryReader to provide StoredFields access.
type directoryStoredFields struct {
	reader *DirectoryReader
}

// Prefetch prefetches stored fields for the given document IDs.
func (dsf *directoryStoredFields) Prefetch(docIDs []int) error {
	// No-op for now
	return nil
}

// Document retrieves the stored fields for a document using the visitor pattern.
func (dsf *directoryStoredFields) Document(docID int, visitor StoredFieldVisitor) error {
	remainingDocID := docID
	for _, subReader := range dsf.reader.readers {
		maxDoc := subReader.MaxDoc()
		if remainingDocID < maxDoc {
			sf, err := subReader.StoredFields()
			if err != nil {
				return err
			}
			if sf == nil {
				return nil
			}
			return sf.Document(remainingDocID, visitor)
		}
		remainingDocID -= maxDoc
	}
	return fmt.Errorf("document ID %d out of range", docID)
}

// directoryTermVectors wraps a DirectoryReader to provide TermVectors access.
type directoryTermVectors struct {
	reader *DirectoryReader
}

// Prefetch prefetches term vectors for the given document IDs.
func (dtv *directoryTermVectors) Prefetch(docIDs []int) error {
	// No-op for now
	return nil
}

// Get retrieves the term vectors for a document.
func (dtv *directoryTermVectors) Get(docID int) (Fields, error) {
	remainingDocID := docID
	for _, subReader := range dtv.reader.readers {
		maxDoc := subReader.MaxDoc()
		if remainingDocID < maxDoc {
			tv, err := subReader.TermVectors()
			if err != nil {
				return nil, err
			}
			if tv == nil {
				return nil, nil
			}
			return tv.Get(remainingDocID)
		}
		remainingDocID -= maxDoc
	}
	return nil, fmt.Errorf("document ID %d out of range", docID)
}

// GetField retrieves the term vector for a specific field in a document.
func (dtv *directoryTermVectors) GetField(docID int, field string) (Terms, error) {
	remainingDocID := docID
	for _, subReader := range dtv.reader.readers {
		maxDoc := subReader.MaxDoc()
		if remainingDocID < maxDoc {
			tv, err := subReader.TermVectors()
			if err != nil {
				return nil, err
			}
			if tv == nil {
				return nil, nil
			}
			return tv.GetField(remainingDocID, field)
		}
		remainingDocID -= maxDoc
	}
	return nil, fmt.Errorf("document ID %d out of range", docID)
}

// ListCommits returns all commits present in the given directory, sorted in
// ascending generation order.  This is the Go equivalent of Lucene's
// DirectoryReader.listCommits().
func ListCommits(dir store.Directory) (IndexCommitList, error) {
	// Read the latest commit. Mirrors Lucene's listCommits which calls
	// SegmentInfos.readLatestCommit and propagates IndexNotFoundException
	// when the directory has no index.
	latest, err := ReadSegmentInfos(dir)
	if err != nil {
		return nil, err
	}

	currentGen := latest.Generation()

	// Start with the latest commit.
	commits := make(IndexCommitList, 0, 4)
	latestCommit := NewIndexCommit(latest)
	latestCommit.SetDirectory(dir)
	commits = append(commits, latestCommit)

	// Scan the directory for older segments_N files.
	files, err := dir.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listCommits: list directory: %w", err)
	}
	for _, name := range files {
		gen := parseSegmentsFileGeneration(name)
		if gen <= 0 || gen >= currentGen {
			continue // not a segments file, or already captured as latest
		}
		sis, readErr := readSegmentInfosFileByGen(dir, name, gen)
		if readErr != nil {
			// File may have been deleted between listing and reading; skip it.
			continue
		}
		c := NewIndexCommit(sis)
		c.SetDirectory(dir)
		commits = append(commits, c)
	}

	// Sort ascending by generation so callers can walk history oldest-first.
	sortCommitsByGeneration(commits)

	return commits, nil
}

// parseSegmentsFileGeneration extracts the generation from a segments_N
// file name. Returns 0 if the name does not match the "segments_<N>" pattern.
// The generation is encoded in base 36, matching Lucene's segment file naming.
func parseSegmentsFileGeneration(name string) int64 {
	const prefix = "segments_"
	if !strings.HasPrefix(name, prefix) {
		return 0
	}
	gen, err := strconv.ParseInt(name[len(prefix):], 36, 64)
	if err != nil {
		return 0
	}
	return gen
}

// readSegmentInfosFileByGen reads a named segments_N file from dir with the
// given generation. Used by ListCommits to enumerate prior commits.
func readSegmentInfosFileByGen(dir store.Directory, name string, gen int64) (*SegmentInfos, error) {
	if gen <= 0 {
		return nil, fmt.Errorf("not a segments file: %q", name)
	}
	// Open a throwaway handle; readSegmentInfosLucene104 closes it and re-opens
	// the canonical file by generation internally.
	rawIn, err := dir.OpenInput(name, store.IOContextRead)
	if err != nil {
		return nil, err
	}
	// Peek at the magic to validate the format before delegating.
	magic, err := store.ReadInt32(rawIn)
	if err != nil {
		_ = rawIn.Close()
		return nil, fmt.Errorf("readSegmentInfosFile %q: read magic: %w", name, err)
	}
	if magic != codecMagic {
		_ = rawIn.Close()
		// Legacy / unknown format — skip gracefully.
		return nil, fmt.Errorf("readSegmentInfosFile %q: unknown magic 0x%x", name, uint32(magic))
	}
	// rawIn is handed off to spi.ReadSegmentInfosFromHandle which closes it
	// and re-opens the file at offset 0 for checksum verification.
	return spi.ReadSegmentInfosFromHandle(rawIn, dir, gen)
}

// sortCommitsByGeneration sorts commits ascending by generation using insertion
// sort (list is typically very small: 1–10 commits).
func sortCommitsByGeneration(commits IndexCommitList) {
	for i := 1; i < len(commits); i++ {
		for j := i; j > 0 && commits[j].GetGeneration() < commits[j-1].GetGeneration(); j-- {
			commits[j], commits[j-1] = commits[j-1], commits[j]
		}
	}
}

// IndexExists reports whether an index likely exists at the specified directory.
// Note that if a corrupt index exists, or if an index in the process of committing
// is not yet complete, this logic may return true.
//
// Mirrors org.apache.lucene.index.DirectoryReader.indexExists.
func IndexExists(directory store.Directory) (bool, error) {
	files, err := directory.ListAll()
	if err != nil {
		return false, err
	}

	prefix := "segments_"
	for _, file := range files {
		if strings.HasPrefix(file, prefix) {
			return true, nil
		}
	}
	return false, nil
}

// Ensure DirectoryReader implements IndexReaderInterface
var _ IndexReaderInterface = (*DirectoryReader)(nil)

// SegmentReader additional methods

// StoredFields returns a StoredFields instance for accessing stored fields.
func (r *SegmentReader) StoredFields() (StoredFields, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("segment reader not initialized")
	}
	sfReader := r.coreReaders.GetStoredFieldsReader()
	if sfReader == nil {
		return NewEmptyStoredFields(), nil
	}
	liveDocs := r.GetLiveDocs()
	return NewStoredFields(sfReader, liveDocs), nil
}

// TermVectors returns a TermVectors instance for accessing term vectors.
func (r *SegmentReader) TermVectors() (TermVectors, error) {
	if r.coreReaders == nil {
		return nil, fmt.Errorf("segment reader not initialized")
	}
	tvReader := r.coreReaders.GetTermVectorsReader()
	if tvReader == nil {
		return NewEmptyTermVectors(), nil
	}
	liveDocs := r.GetLiveDocs()
	return NewTermVectors(tvReader, liveDocs), nil
}

// boolBits is a util.Bits backed by a []bool slice.
type boolBits []bool

func (b boolBits) Get(index int) bool { return b[index] }
func (b boolBits) Length() int        { return len(b) }

// Ensure SegmentReader implements IndexReaderInterface
var _ IndexReaderInterface = (*SegmentReader)(nil)

// doOpenIfChanged implements the logic to reopen the index if it has changed.
func (r *DirectoryReader) doOpenIfChanged(commit *IndexCommit, executor interface{}) (*DirectoryReader, error) {
	r.EnsureOpen()
	if r.writer != nil {
		return r.doOpenFromWriter(commit, executor)
	}
	return r.doOpenNoWriter(commit, executor)
}

func (r *DirectoryReader) doOpenFromWriter(commit *IndexCommit, executor interface{}) (*DirectoryReader, error) {
	if commit != nil {
		return r.doOpenFromCommit(commit, executor)
	}
	if r.writer.NrtIsCurrent(r.segmentInfos) {
		return nil, nil
	}
	reader, err := OpenDirectoryReaderFromWriter(r.writer)
	if err != nil {
		return nil, err
	}
	if reader.GetVersion() == r.segmentInfos.Generation() {
		reader.Close()
		return nil, nil
	}
	return reader, nil
}

func (r *DirectoryReader) doOpenNoWriter(commit *IndexCommit, executor interface{}) (*DirectoryReader, error) {
	if commit == nil {
		if current, err := r.IsCurrent(); err == nil && current {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
	} else {
		if commit.GetDirectory() != r.directory {
			return nil, fmt.Errorf("the specified commit does not match the specified Directory")
		}
		if r.segmentInfos != nil && commit.GetSegmentsFileName() == r.segmentInfos.GetSegmentsFileName() {
			return nil, nil
		}
	}
	return r.doOpenFromCommit(commit, executor)
}

func (r *DirectoryReader) doOpenFromCommit(commit *IndexCommit, executor interface{}) (*DirectoryReader, error) {
	return openDirectoryReaderWithSharing(r.directory, commit.GetSegmentInfos(), r.readers)
}

func openDirectoryReaderWithSharing(directory store.Directory, segmentInfos *SegmentInfos, oldReaders []*SegmentReader) (*DirectoryReader, error) {
	readers := make([]*SegmentReader, 0, segmentInfos.Size())

	previousSegmentReaders := make(map[string]int)
	for i, r := range oldReaders {
		if r != nil {
			previousSegmentReaders[r.GetSegmentName()] = i
		}
	}

	for i := 0; i < segmentInfos.Size(); i++ {
		sci := segmentInfos.Get(i)
		oldReaderIdx, ok := previousSegmentReaders[sci.SegmentInfo().Name()]
		var oldReader *SegmentReader
		if ok {
			oldReader = oldReaders[oldReaderIdx]
			if oldReader.GetSegmentInfo().GetID() != sci.SegmentInfo().GetID() {
				return nil, fmt.Errorf("same segment %s has invalid doc count change; likely you are re-opening a reader after illegally removing index files yourself", sci.SegmentInfo().Name())
			}
		}

		var newReader *SegmentReader
		if oldReader == nil || sci.SegmentInfo().IsCompoundFile() != oldReader.GetSegmentInfo().IsCompoundFile() {
			var err error
			newReader, err = openSegmentReader(directory, sci)
			if err != nil {
				for _, opened := range readers {
					opened.Close()
				}
				return nil, err
			}
		} else {
			if oldReader.isNRT {
				var liveDocs util.Bits
				if sci.HasDeletions() {
					codec := LookupCodecByName(sci.SegmentInfo().Codec())
					var err error
					liveDocs, err = codec.LiveDocsFormat().ReadLiveDocs(directory, sci, store.IOContextReadOnce)
					if err != nil {
						return nil, err
					}
				}
				var err error
				newReader, err = NewSegmentReaderFrom(sci, oldReader, liveDocs, liveDocs, sci.SegmentInfo().MaxDoc()-sci.GetDelCount(), false)
				if err != nil {
					return nil, err
				}
			} else {
				if oldReader.GetSegmentInfo().GetDelGen() == sci.GetDelGen() &&
					oldReader.GetSegmentInfo().GetFieldInfosGen() == sci.GetFieldInfosGen() {
					oldReader.IncRef()
					newReader = oldReader
				} else if oldReader.GetSegmentInfo().GetDelGen() == sci.GetDelGen() {
					var err error
					newReader, err = NewSegmentReaderFrom(sci, oldReader, oldReader.GetLiveDocs(), oldReader.GetHardLiveDocs(), oldReader.NumDocs(), false)
					if err != nil {
						return nil, err
					}
				} else {
					var liveDocs util.Bits
					if sci.HasDeletions() {
						codec := LookupCodecByName(sci.SegmentInfo().Codec())
						var err error
						liveDocs, err = codec.LiveDocsFormat().ReadLiveDocs(directory, sci, store.IOContextReadOnce)
						if err != nil {
							return nil, err
						}
					}
					var err error
					newReader, err = NewSegmentReaderFrom(sci, oldReader, liveDocs, liveDocs, sci.SegmentInfo().MaxDoc()-sci.GetDelCount(), false)
					if err != nil {
						return nil, err
					}
				}
			}
			readers = append(readers, newReader)
		}

		compReader, err := newCompositeReaderFromSegments(readers)
		if err != nil {
			for _, opened := range readers {
				opened.Close()
			}
			return nil, err
		}

		return &DirectoryReader{
			CompositeReader: compReader,
			directory:       directory,
			segmentInfos:    segmentInfos,
			readers:         readers,
			nrtGen:          0,
		}, nil
	}
}
