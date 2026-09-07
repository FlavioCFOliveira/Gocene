// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// StandardDirectoryReader is the default implementation of DirectoryReader.
// This is the Go port of Lucene's org.apache.lucene.index.StandardDirectoryReader.
type StandardDirectoryReader struct {
	*DirectoryReader

	// writer is the IndexWriter that opened this reader (if any).
	writer *IndexWriter

	// segmentInfos contains information about all segments in this reader.
	segmentInfos *SegmentInfos

	// applyAllDeletes controls whether all buffered deletes are applied.
	applyAllDeletes bool

	// writeAllDeletes controls whether all buffered deletes are written to disk.
	writeAllDeletes bool
}

// Open opens a StandardDirectoryReader for the given directory.
// This is the Go equivalent of Lucene's StandardDirectoryReader.open(Directory, IndexCommit, Comparator, ExecutorService).
func OpenStandardDirectoryReader(directory store.Directory, commit *IndexCommit) (*StandardDirectoryReader, error) {
	return OpenStandardDirectoryReaderWithMinVersion(directory, 0, commit)
}

// OpenStandardDirectoryReaderWithMinVersion opens a StandardDirectoryReader for the given directory, ensuring it meets the minimum supported major version.
func OpenStandardDirectoryReaderWithMinVersion(directory store.Directory, minSupportedMajorVersion int, commit *IndexCommit) (*StandardDirectoryReader, error) {
	if minSupportedMajorVersion < 0 {
		return nil, fmt.Errorf("minSupportedMajorVersion must be positive but was: %d", minSupportedMajorVersion)
	}

	// In Lucene, this uses SegmentInfos.FindSegmentsFile.
	// In Gocene, we use ReadCommit or ReadLatestCommit.
	var segmentFileName string
	if commit != nil {
		segmentFileName = commit.GetSegmentsFileName()
	} else {
		files, err := directory.ListAll()
		if err != nil {
			return nil, fmt.Errorf("failed to list directory: %w", err)
		}
		segmentFileName = spi.GetLastCommitSegmentsFileName(files)
	}

	if segmentFileName == "" {
		return nil, fmt.Errorf("no segments file found in index")
	}

	sis, err := spi.ReadCommit(directory, segmentFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read segment infos: %w", err)
	}

	readers, err := createSegmentReaders(sis, nil)
	if err != nil {
		// Close already opened readers on error
		for _, r := range readers {
			if r != nil {
				_ = r.Close()
			}
		}
		return nil, err
	}

	compReader, err := newCompositeReaderFromSegments(readers)
	if err != nil {
		for _, r := range readers {
			if r != nil {
				_ = r.Close()
			}
		}
		return nil, err
	}

	return &StandardDirectoryReader{
		DirectoryReader: &DirectoryReader{
			CompositeReader: compReader,
			directory:       directory,
			segmentInfos:    sis,
			readers:         readers,
		},
		writer:          nil,
		segmentInfos:    sis,
		applyAllDeletes: false,
		writeAllDeletes: false,
	}, nil
}

// OpenNRT opens a StandardDirectoryReader used by near-real-time search.
// This is the Go port of Lucene's StandardDirectoryReader.open(IndexWriter, IOFunction, SegmentInfos, boolean, boolean).
func OpenNRT(
	writer *IndexWriter,
	readerFunction func(*SegmentCommitInfo) (*SegmentReader, error),
	infos *SegmentInfos,
	applyAllDeletes bool,
	writeAllDeletes bool,
) (*StandardDirectoryReader, error) {
	numSegments := infos.Size()
	readers := make([]*SegmentReader, 0, numSegments)
	dir := writer.GetDirectory()

	segmentInfos := infos.Clone()
	infosUpto := 0

	for i := 0; i < numSegments; i++ {
		info := infos.Get(i)
		reader, err := readerFunction(info)
		if err != nil {
			// Close readers on error
			for _, r := range readers {
				_ = r.DecRef()
			}
			return nil, err
		}

		if reader.NumDocs() > 0 || writer.GetConfig().MergePolicy.KeepFullyDeletedSegment(func() IndexReaderInterface { return reader }) {
			readers = append(readers, reader)
			infosUpto++
		} else {
			_ = reader.DecRef()
			segmentInfos.Remove(infosUpto)
		}
	}

	// In Lucene, writer.incRefDeleter(segmentInfos) is called here.
	// We assume the IndexWriter handles this internally or it's implemented in IndexFileDeleter.

	compReader, err := newCompositeReaderFromSegments(readers)
	if err != nil {
		for _, r := range readers {
			_ = r.DecRef()
		}
		return nil, err
	}

	return &StandardDirectoryReader{
		DirectoryReader: &DirectoryReader{
			CompositeReader: compReader,
			directory:       dir,
			segmentInfos:    segmentInfos,
			readers:         readers,
			nrtGen:          writer.GetNRTGeneration(),
			writer:          writer,
		},
		writer:          writer,
		segmentInfos:    segmentInfos,
		applyAllDeletes: applyAllDeletes,
		writeAllDeletes: writeAllDeletes,
	}, nil
}

// OpenWithInfos opens a StandardDirectoryReader for the given directory and SegmentInfos.
func OpenWithInfos(directory store.Directory, infos *SegmentInfos, oldReaders []*SegmentReader) (*StandardDirectoryReader, error) {
	newReaders, err := createSegmentReaders(infos, oldReaders)
	if err != nil {
		return nil, err
	}

	compReader, err := newCompositeReaderFromSegments(newReaders)
	if err != nil {
		for _, r := range newReaders {
			if r != nil {
				_ = r.Close()
			}
		}
		return nil, err
	}

	return &StandardDirectoryReader{
		DirectoryReader: &DirectoryReader{
			CompositeReader: compReader,
			directory:       directory,
			segmentInfos:    infos,
			readers:         newReaders,
		},
		writer:          nil,
		segmentInfos:    infos,
		applyAllDeletes: false,
		writeAllDeletes: false,
	}, nil
}

// createSegmentReaders creates segment readers, preferring to reuse existing ones.
func createSegmentReaders(sis *SegmentInfos, oldReaders []*SegmentReader) ([]*SegmentReader, error) {
	previousSegmentReaders := mapPreviousReaders(oldReaders)
	readers := make([]*SegmentReader, sis.Size())

	// Go implementation: process segments in parallel using goroutines.
	var wg sync.WaitGroup
	errs := make(chan error, sis.Size())

	for i := 0; i < sis.Size(); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			commitInfo := sis.Get(idx)
			oldReader := getOldSegmentReader(oldReaders, previousSegmentReaders[commitInfo.SegmentInfo().Name()], commitInfo)
			reader, err := createOrReuseSegmentReader(commitInfo, oldReader, sis.GetIndexCreatedVersionMajor())
			if err != nil {
				errs <- err
				return
			}
			readers[idx] = reader
		}(i)
	}

	wg.Wait()
	close(errs)

	if err := <-errs; err != nil {
		// Close all opened readers on error
		for _, r := range readers {
			if r != nil {
				_ = r.DecRef()
			}
		}
		return nil, err
	}

	return readers, nil
}

func mapPreviousReaders(oldReaders []*SegmentReader) map[string]int {
	if oldReaders == nil {
		return nil
	}
	m := make(map[string]int, len(oldReaders))
	for i, sr := range oldReaders {
		m[sr.SegmentInfo().Name()] = i
	}
	return m
}

func getOldSegmentReader(oldReaders []*SegmentReader, oldReaderIndex int, commitInfo *SegmentCommitInfo) *SegmentReader {
	if oldReaders == nil || oldReaderIndex < 0 || oldReaderIndex >= len(oldReaders) {
		return nil
	}
	oldReader := oldReaders[oldReaderIndex]

	// Detect illegal index removal and replacement.
	if oldReader != nil && !util.EqualSlices(commitInfo.SegmentInfo().GetID(), oldReader.SegmentInfo().GetID()) {
		panic(fmt.Sprintf("same segment %s has invalid doc count change; likely you are re-opening a reader after illegally removing index files yourself", commitInfo.SegmentInfo().Name()))
	}
	return oldReader
}

func createOrReuseSegmentReader(commitInfo *SegmentCommitInfo, oldReader *SegmentReader, indexCreatedVersionMajor int) (*SegmentReader, error) {
	var newReader *SegmentReader

	// Condition for creating a brand new reader.
	if oldReader == nil || commitInfo.SegmentInfo().IsCompoundFile() != oldReader.SegmentInfo().IsCompoundFile() {
		newReader = NewSegmentReader(commitInfo, indexCreatedVersionMajor, store.IOContextDefault)
	} else {
		if oldReader.IsNRT() {
			// NRT reader: must load liveDocs/DV updates from disk.
			var liveDocs util.Bits
			if commitInfo.HasDeletions() {
				liveDocs, _ = commitInfo.SegmentInfo().Codec().LiveDocsFormat().ReadLiveDocs(commitInfo.SegmentInfo().Directory(), commitInfo, store.IOContextReadOnce)
			}
			newReader = NewSegmentReaderNRT(commitInfo, oldReader, liveDocs, liveDocs, commitInfo.SegmentInfo().MaxDoc()-commitInfo.DelCount(), false)
		} else {
			if oldReader.SegmentInfo().DelGen() == commitInfo.DelGen() && oldReader.SegmentInfo().FieldInfosGen() == commitInfo.FieldInfosGen() {
				// No change; reuse the reader.
				_ = oldReader.IncRef()
				newReader = oldReader
			} else {
				if oldReader.SegmentInfo().DelGen() == commitInfo.DelGen() {
					// Only DV updates.
					newReader = NewSegmentReaderDVUpdate(commitInfo, oldReader, oldReader.GetLiveDocs(), oldReader.GetHardLiveDocs(), oldReader.NumDocs(), false)
				} else {
					// Both DV and liveDocs changed.
					var liveDocs util.Bits
					if commitInfo.HasDeletions() {
						liveDocs, _ = commitInfo.SegmentInfo().Codec().LiveDocsFormat().ReadLiveDocs(commitInfo.SegmentInfo().Directory(), commitInfo, store.IOContextReadOnce)
					}
					newReader = NewSegmentReaderNRT(commitInfo, oldReader, liveDocs, liveDocs, commitInfo.SegmentInfo().MaxDoc()-commitInfo.DelCount(), false)
				}
			}
		}
	}
	return newReader, nil
}

// doOpenIfChanged implements the reopen logic from Lucene.
func (r *StandardDirectoryReader) doOpenIfChanged(commit *IndexCommit, executor interface{}) (*StandardDirectoryReader, error) {
	if err := r.EnsureOpen(); err != nil {
		return nil, err
	}

	if r.writer != nil {
		return r.doOpenFromWriter(commit, executor)
	}
	return r.doOpenNoWriter(commit, executor)
}

func (r *StandardDirectoryReader) doOpenFromWriter(commit *IndexCommit, executor interface{}) (*StandardDirectoryReader, error) {
	if commit != nil {
		return r.doOpenFromCommit(commit, executor)
	}

	if r.writer.NRTIsCurrent(r.segmentInfos) {
		return nil, nil
	}

	reader, err := r.writer.GetReader(r.applyAllDeletes, r.writeAllDeletes)
	if err != nil {
		return nil, err
	}

	if reader.GetVersion() == r.segmentInfos.GetVersion() {
		_ = reader.DecRef()
		return nil, nil
	}

	// Convert the DirectoryReader returned by GetReader to StandardDirectoryReader.
	// Since GetReader in Gocene already returns *StandardDirectoryReader (based on IndexWriter.go), we can cast.
	if std, ok := reader.(*StandardDirectoryReader); ok {
		return std, nil
	}
	return nil, fmt.Errorf("GetReader did not return a StandardDirectoryReader")
}

func (r *StandardDirectoryReader) doOpenNoWriter(commit *IndexCommit, executor interface{}) (*StandardDirectoryReader, error) {
	if commit == nil {
		if r.IsCurrentInternal() {
			return nil, nil
		}
	} else {
		if r.directory != commit.GetDirectory() {
			return nil, fmt.Errorf("the specified commit does not match the specified Directory")
		}
		if r.segmentInfos != nil && commit.GetSegmentsFileName() == r.segmentInfos.GetFileName() {
			return nil, nil
		}
	}

	return r.doOpenFromCommit(commit, executor)
}

func (r *StandardDirectoryReader) doOpenFromCommit(commit *IndexCommit, executor interface{}) (*StandardDirectoryReader, error) {
	sis := commit.GetSegmentInfos()
	return OpenWithInfos(r.directory, sis, r.GetSequentialSubReaders())
}

func (r *StandardDirectoryReader) IsCurrentInternal() bool {
	sis, err := spi.ReadLatestCommit(r.directory)
	if err != nil {
		return true
	}
	return sis.GetVersion() == r.segmentInfos.GetVersion()
}

// GetVersion returns the version of the segment infos.
func (r *StandardDirectoryReader) GetVersion() int64 {
	return r.segmentInfos.GetVersion()
}

// GetSegmentInfos returns the SegmentInfos for this reader.
func (r *StandardDirectoryReader) GetSegmentInfos() *SegmentInfos {
	return r.segmentInfos
}

// IsCurrent returns true if the reader is still up to date.
func (r *StandardDirectoryReader) IsCurrent() (bool, error) {
	if err := r.EnsureOpen(); err != nil {
		return false, err
	}
	if r.writer == nil {
		return r.IsCurrentInternal(), nil
	}
	return r.writer.NRTIsCurrent(r.segmentInfos), nil
}

func (r *StandardDirectoryReader) doClose() error {
	if r.writer != nil {
		// In Lucene, this calls writer.decRefDeleter(segmentInfos).
		// We assume this is handled by the IndexWriter's internal reference counting.
	}

	var lastErr error
	for _, reader := range r.readers {
		if err := reader.DecRef(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// GetIndexCommit returns the IndexCommit for this reader.
func (r *StandardDirectoryReader) GetIndexCommit() *IndexCommit {
	if r.segmentInfos == nil {
		return nil
	}
	commit := NewIndexCommit(r.segmentInfos)
	commit.SetDirectory(r.directory)
	commit.SetReader(r.DirectoryReader)
	return commit
}

// ReaderCommit is a specialized IndexCommit that refers back to a StandardDirectoryReader.
type ReaderCommit struct {
	*IndexCommit
	reader *StandardDirectoryReader
}

func NewReaderCommit(reader *StandardDirectoryReader, infos *SegmentInfos, dir store.Directory) *ReaderCommit {
	commit := NewIndexCommit(infos)
	commit.SetDirectory(dir)
	return &ReaderCommit{
		IndexCommit: commit,
		reader:      reader,
	}
}

func (rc *ReaderCommit) GetReader() *StandardDirectoryReader {
	return rc.reader
}

func (rc *ReaderCommit) Delete() {
	panic("ReaderCommit does not support deletions")
}

func (r *StandardDirectoryReader) String() string {
	var sb strings.Builder
	sb.WriteString("StandardDirectoryReader(")
	if r.segmentInfos != nil {
		sb.WriteString(r.segmentInfos.GetFileName())
		sb.WriteByte(':')
		sb.WriteString(strconv.FormatInt(r.segmentInfos.GetVersion(), 10))
	}
	if r.writer != nil {
		sb.WriteString(":nrt")
	}
	for _, reader := range r.readers {
		sb.WriteByte(' ')
		sb.WriteString(fmt.Sprintf("%v", reader))
	}
	sb.WriteByte(')')
	return sb.String()
}
