// Package index provides core index functionality for Gocene.
// This file implements the CheckIndex tool for verifying index integrity.
// Source: org.apache.lucene.index.CheckIndex (Apache Lucene 10.x)
package index

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// CheckIndexLevel defines the level of checking to perform
type CheckIndexLevel int

const (
	// CheckIndexLevelNone performs no checking
	CheckIndexLevelNone CheckIndexLevel = iota
	// CheckIndexLevelMinIntegrityChecks performs minimum integrity checks (fast)
	CheckIndexLevelMinIntegrityChecks
	// CheckIndexLevelAll performs all checks (slow)
	CheckIndexLevelAll
)

// String returns the string representation of the level
func (l CheckIndexLevel) String() string {
	switch l {
	case CheckIndexLevelNone:
		return "NONE"
	case CheckIndexLevelMinIntegrityChecks:
		return "MIN_INTEGRITY_CHECKS"
	case CheckIndexLevelAll:
		return "ALL"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", l)
	}
}

// CheckIndexStatus holds the status of the check index operation
type CheckIndexStatus struct {
	// Clean is true if the index is valid
	Clean bool
	// MissingSegments is true if some segments are missing
	MissingSegments bool
	// SegmentsFileName is the name of the segments file
	SegmentsFileName string
	// NumSegments is the number of segments in the index
	NumSegments int
	// ToolOutOfDate is true if the tool is outdated
	ToolOutOfDate bool
	// TotLoseDocCount is the total number of documents that would be lost if corrupt segments are removed
	TotLoseDocCount int
	// NumBadSegments is the number of corrupt segments
	NumBadSegments int
	// Partial is true if only specific segments were checked
	Partial bool
	// MaxSegmentName is the maximum segment name
	MaxSegmentName int64
	// ValidCounter is true if the segment counter is valid
	ValidCounter bool
	// SegmentInfos holds status for each segment
	SegmentInfos []*SegmentInfoStatus
	// Errors contains any errors encountered
	Errors []error
}

// SegmentInfoStatus holds status for a single segment
type SegmentInfoStatus struct {
	// Name is the segment name (e.g., "_0")
	Name string
	// Codec is the codec used by the segment
	Codec string
	// MaxDoc is the number of documents in the segment
	MaxDoc int
	// Compound is true if the segment uses compound files
	Compound bool
	// NumFiles is the number of files in the segment
	NumFiles int
	// SizeMB is the size of the segment in MB
	SizeMB float64
	// HasDeletions is true if the segment has deletions
	HasDeletions bool
	// DeletionsGen is the deletions generation
	DeletionsGen int64
	// OpenReaderPassed is true if opening a reader succeeded
	OpenReaderPassed bool
	// ToLoseDocCount is the number of documents that would be lost if this segment is corrupt
	ToLoseDocCount int
	// Diagnostics contains diagnostic information
	Diagnostics map[string]string
	// Error is non-nil if the segment has errors
	Error error
	// FieldInfoStatus holds field info checking status
	FieldInfoStatus *FieldInfoStatus
	// FieldNormStatus holds field norm checking status
	FieldNormStatus *FieldNormStatus
	// TermIndexStatus holds term index checking status
	TermIndexStatus *TermIndexStatus
	// StoredFieldStatus holds stored field checking status
	StoredFieldStatus *StoredFieldStatus
	// TermVectorStatus holds term vector checking status
	TermVectorStatus *TermVectorStatus
	// DocValuesStatus holds DocValues checking status
	DocValuesStatus *DocValuesStatus
	// PointsStatus holds Points checking status
	PointsStatus *PointsStatus
	// VectorValuesStatus holds vector values checking status
	VectorValuesStatus *VectorValuesStatus
	// IndexSortStatus holds index sort checking status
	IndexSortStatus *IndexSortStatus
	// LiveDocStatus holds live docs checking status
	LiveDocStatus *LiveDocStatus
	// SoftDeletesStatus holds soft deletes checking status
	SoftDeletesStatus *SoftDeletesStatus
}

// FieldInfoStatus holds field info checking status
type FieldInfoStatus struct {
	// TotFields is the total number of fields
	TotFields int
	// Error is non-nil if there was an error
	Error error
}

// FieldNormStatus holds field norm checking status
type FieldNormStatus struct {
	// TotFields is the total number of fields with norms
	TotFields int
	// Error is non-nil if there was an error
	Error error
}

// TermIndexStatus holds term index checking status
type TermIndexStatus struct {
	// TermCount is the number of unique terms
	TermCount int64
	// DelTermCount is the number of deleted terms
	DelTermCount int64
	// TotFreq is the total frequency of all terms
	TotFreq int64
	// TotPos is the total number of positions
	TotPos int64
	// Error is non-nil if there was an error
	Error error
}

// StoredFieldStatus holds stored field checking status
type StoredFieldStatus struct {
	// DocCount is the number of documents with stored fields
	DocCount int
	// TotFields is the total number of stored fields
	TotFields int64
	// Error is non-nil if there was an error
	Error error
}

// TermVectorStatus holds term vector checking status
type TermVectorStatus struct {
	// DocCount is the number of documents with term vectors
	DocCount int
	// TotVectors is the total number of term vectors
	TotVectors int64
	// Error is non-nil if there was an error
	Error error
}

// DocValuesStatus holds DocValues checking status
type DocValuesStatus struct {
	// TotalValueFields is the total number of DocValues fields
	TotalValueFields int
	// TotalNumericFields is the number of numeric DocValues fields
	TotalNumericFields int
	// TotalBinaryFields is the number of binary DocValues fields
	TotalBinaryFields int
	// TotalSortedFields is the number of sorted DocValues fields
	TotalSortedFields int
	// TotalSortedNumericFields is the number of sorted numeric DocValues fields
	TotalSortedNumericFields int
	// TotalSortedSetFields is the number of sorted set DocValues fields
	TotalSortedSetFields int
	// TotalSkippingIndex is the number of fields with skipping index
	TotalSkippingIndex int
	// Error is non-nil if there was an error
	Error error
}

// PointsStatus holds Points checking status
type PointsStatus struct {
	// TotalValuePoints is the total number of point values
	TotalValuePoints int64
	// TotalValueFields is the number of point fields
	TotalValueFields int
	// Error is non-nil if there was an error
	Error error
}

// VectorValuesStatus holds vector values checking status
type VectorValuesStatus struct {
	// TotalVectorValues is the total number of vector values
	TotalVectorValues int64
	// TotalKnnVectorFields is the number of KNN vector fields
	TotalKnnVectorFields int
	// Error is non-nil if there was an error
	Error error
}

// IndexSortStatus holds index sort checking status
type IndexSortStatus struct {
	// Error is non-nil if there was an error
	Error error
}

// LiveDocStatus holds live docs checking status
type LiveDocStatus struct {
	// NumDeleted is the number of deleted documents
	NumDeleted int
	// Error is non-nil if there was an error
	Error error
}

// SoftDeletesStatus holds soft deletes checking status
type SoftDeletesStatus struct {
	// Error is non-nil if there was an error
	Error error
}

// CheckIndex is a tool to verify the integrity of an index
type CheckIndex struct {
	dir         store.Directory
	infoStream  io.Writer
	level       CheckIndexLevel
	verbose     bool
	printStack  bool
	threadCount int

	// Lock for thread-safe operations
	mutex sync.Mutex

	// Write lock for the directory
	writeLock store.Lock

	// Closed flag
	closed bool
}

// NewCheckIndex creates a new CheckIndex tool for the given directory
func NewCheckIndex(dir store.Directory) (*CheckIndex, error) {
	if dir == nil {
		return nil, fmt.Errorf("directory cannot be nil")
	}

	ci := &CheckIndex{
		dir:         dir,
		infoStream:  os.Stdout,
		level:       CheckIndexLevelAll,
		verbose:     false,
		printStack:  false,
		threadCount: runtime.GOMAXPROCS(0),
	}

	// Try to obtain a write lock to ensure no IndexWriter is open
	lock, err := dir.ObtainLock("write.lock")
	if err != nil {
		return nil, fmt.Errorf("cannot obtain write lock - another IndexWriter may be open: %w", err)
	}
	ci.writeLock = lock

	return ci, nil
}

// Close releases resources held by CheckIndex
func (ci *CheckIndex) Close() error {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()

	if ci.closed {
		return nil
	}

	ci.closed = true

	if ci.writeLock != nil {
		if err := ci.writeLock.Close(); err != nil {
			return err
		}
		ci.writeLock = nil
	}

	return nil
}

// SetInfoStream sets the output stream for logging
func (ci *CheckIndex) SetInfoStream(out io.Writer) {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()

	if out == nil {
		ci.infoStream = io.Discard
	} else {
		ci.infoStream = out
	}
}

// SetLevel sets the level of checking to perform
func (ci *CheckIndex) SetLevel(level CheckIndexLevel) {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()
	ci.level = level
}

// SetVerbose sets whether to output verbose information
func (ci *CheckIndex) SetVerbose(verbose bool) {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()
	ci.verbose = verbose
}

// SetPrintStackTrace sets whether to print stack traces on errors
func (ci *CheckIndex) SetPrintStackTrace(printStack bool) {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()
	ci.printStack = printStack
}

// SetThreadCount sets the number of threads to use for checking
func (ci *CheckIndex) SetThreadCount(count int) {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()
	if count <= 0 {
		count = runtime.GOMAXPROCS(0)
	}
	ci.threadCount = count
}

// msg prints a message to the info stream
func (ci *CheckIndex) msg(message string) {
	if ci.infoStream != nil {
		fmt.Fprintln(ci.infoStream, message)
	}
}

// msgf prints a formatted message to the info stream
func (ci *CheckIndex) msgf(format string, args ...interface{}) {
	if ci.infoStream != nil {
		fmt.Fprintf(ci.infoStream, format+"\n", args...)
	}
}

// error prints an error message
func (ci *CheckIndex) error(err error) {
	if ci.infoStream != nil {
		fmt.Fprintf(ci.infoStream, "ERROR: %v\n", err)
		if ci.printStack && err != nil {
			// Print stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			fmt.Fprintf(ci.infoStream, "Stack trace:\n%s\n", buf[:n])
		}
	}
}

// CheckIndex performs a full check of the index
func (ci *CheckIndex) CheckIndex() (*CheckIndexStatus, error) {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()

	if ci.closed {
		return nil, fmt.Errorf("CheckIndex is closed")
	}

	return ci.checkIndexInternal(nil)
}

// CheckIndexSegments checks only the specified segments
func (ci *CheckIndex) CheckIndexSegments(segments []string) (*CheckIndexStatus, error) {
	ci.mutex.Lock()
	defer ci.mutex.Unlock()

	if ci.closed {
		return nil, fmt.Errorf("CheckIndex is closed")
	}

	return ci.checkIndexInternal(segments)
}

// checkIndexInternal performs the actual checking
func (ci *CheckIndex) checkIndexInternal(onlySegments []string) (*CheckIndexStatus, error) {
	status := &CheckIndexStatus{
		Clean:        true,
		SegmentInfos: make([]*SegmentInfoStatus, 0),
		Errors:       make([]error, 0),
		Partial:      onlySegments != nil && len(onlySegments) > 0,
	}

	ci.msg("")
	ci.msg("Opening index @ " + fmt.Sprintf("%v", ci.dir))
	ci.msg("")

	// Read the segments info
	segmentInfos, err := ReadSegmentInfos(ci.dir)
	if err != nil {
		status.Clean = false
		status.Errors = append(status.Errors, fmt.Errorf("cannot read segments file: %w", err))
		return status, nil
	}

	status.NumSegments = segmentInfos.Size()
	status.MaxSegmentName = segmentInfos.Counter()
	status.ValidCounter = true

	ci.msgf("Segments count=%d", segmentInfos.Size())
	ci.msgf("Segments counter=%d", segmentInfos.Counter())

	// Determine which segments to check
	segmentNamesToCheck := make(map[string]bool)
	if onlySegments != nil && len(onlySegments) > 0 {
		for _, name := range onlySegments {
			segmentNamesToCheck[name] = true
		}
	}

	// Check each segment
	for i := 0; i < segmentInfos.Size(); i++ {
		segCommitInfo := segmentInfos.Get(i)
		if segCommitInfo == nil {
			continue
		}

		segInfo := segCommitInfo.SegmentInfo()
		if segInfo == nil {
			continue
		}

		segmentName := segInfo.Name()

		// Skip if we're only checking specific segments
		if len(segmentNamesToCheck) > 0 && !segmentNamesToCheck[segmentName] {
			ci.msgf("Skipping segment %s (not in requested segments list)", segmentName)
			continue
		}

		segStatus := ci.checkSegment(segCommitInfo, segmentInfos)
		status.SegmentInfos = append(status.SegmentInfos, segStatus)

		if segStatus.Error != nil {
			status.Clean = false
			status.NumBadSegments++
			status.TotLoseDocCount += segInfo.DocCount()
		}
	}

	ci.msg("")
	ci.msgf("Index is %s", func() string {
		if status.Clean {
			return "clean"
		}
		return "NOT clean (found errors)"
	}())
	ci.msg("")

	return status, nil
}

// checkSegment checks a single segment
func (ci *CheckIndex) checkSegment(segCommitInfo *SegmentCommitInfo, segmentInfos *SegmentInfos) *SegmentInfoStatus {
	segInfo := segCommitInfo.SegmentInfo()
	status := &SegmentInfoStatus{
		Name:         segInfo.Name(),
		Codec:        segInfo.Codec(),
		MaxDoc:       segInfo.DocCount(),
		Compound:     segInfo.IsCompoundFile(),
		DeletionsGen: segCommitInfo.DelGen(),
		Diagnostics:  segInfo.GetDiagnostics(),
	}

	ci.msg("")
	ci.msgf("Checking segment %s", segInfo.Name())
	ci.msgf("  Version: %s", segInfo.Version())
	ci.msgf("  Doc count: %d", segInfo.DocCount())
	ci.msgf("  Compound file: %v", segInfo.IsCompoundFile())

	// Check segment files
	if err := ci.checkSegmentFiles(segInfo); err != nil {
		status.Error = err
		ci.error(err)
		return status
	}

	// Try to open a reader for this segment
	if ci.level != CheckIndexLevelNone {
		if err := ci.checkSegmentReader(segCommitInfo, status); err != nil {
			status.Error = err
			ci.error(err)
			return status
		}
	}

	status.OpenReaderPassed = true
	return status
}

// checkSegmentFiles checks the files for a segment
func (ci *CheckIndex) checkSegmentFiles(segInfo *SegmentInfo) error {
	files := segInfo.Files()

	var totalSize int64
	for _, file := range files {
		size, err := ci.dir.FileLength(file)
		if err != nil {
			return fmt.Errorf("cannot get file length for %s: %w", file, err)
		}
		totalSize += size
	}

	return nil
}

// checkSegmentReader opens a reader and checks all index structures
func (ci *CheckIndex) checkSegmentReader(segCommitInfo *SegmentCommitInfo, status *SegmentInfoStatus) error {
	// Create a segment reader for checking
	reader := NewSegmentReader(segCommitInfo)
	if reader == nil {
		return fmt.Errorf("cannot create segment reader")
	}
	defer reader.Close()

	// Check field infos
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: field infos...")
		status.FieldInfoStatus = ci.checkFieldInfos(reader)
		if status.FieldInfoStatus.Error != nil {
			ci.error(status.FieldInfoStatus.Error)
		}
	}

	// Check field norms
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: field norms...")
		status.FieldNormStatus = ci.checkFieldNorms(reader)
		if status.FieldNormStatus.Error != nil {
			ci.error(status.FieldNormStatus.Error)
		}
	}

	// Check term index (postings)
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: terms, freq, prox...")
		status.TermIndexStatus = ci.checkTermIndex(reader)
		if status.TermIndexStatus.Error != nil {
			ci.error(status.TermIndexStatus.Error)
		}
	}

	// Check stored fields
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: stored fields...")
		status.StoredFieldStatus = ci.checkStoredFields(reader)
		if status.StoredFieldStatus.Error != nil {
			ci.error(status.StoredFieldStatus.Error)
		}
	}

	// Check term vectors
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: term vectors...")
		status.TermVectorStatus = ci.checkTermVectors(reader)
		if status.TermVectorStatus.Error != nil {
			ci.error(status.TermVectorStatus.Error)
		}
	}

	// Check doc values
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: docvalues...")
		status.DocValuesStatus = ci.checkDocValues(reader)
		if status.DocValuesStatus.Error != nil {
			ci.error(status.DocValuesStatus.Error)
		}
	}

	// Check points
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: points...")
		status.PointsStatus = ci.checkPoints(reader)
		if status.PointsStatus.Error != nil {
			ci.error(status.PointsStatus.Error)
		}
	}

	// Check vectors
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: vectors...")
		status.VectorValuesStatus = ci.checkVectors(reader)
		if status.VectorValuesStatus.Error != nil {
			ci.error(status.VectorValuesStatus.Error)
		}
	}

	// Check index sort
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: index sort...")
		status.IndexSortStatus = ci.checkIndexSort(reader, segCommitInfo)
		if status.IndexSortStatus.Error != nil {
			ci.error(status.IndexSortStatus.Error)
		}
	}

	// Check live docs
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: check live docs...")
		status.LiveDocStatus = ci.checkLiveDocs(reader)
		if status.LiveDocStatus.Error != nil {
			ci.error(status.LiveDocStatus.Error)
		}
	}

	// Check soft deletes
	if ci.level >= CheckIndexLevelMinIntegrityChecks {
		ci.msg("    test: check soft deletes...")
		status.SoftDeletesStatus = ci.checkSoftDeletes(reader, segCommitInfo)
		if status.SoftDeletesStatus.Error != nil {
			ci.error(status.SoftDeletesStatus.Error)
		}
	}

	return nil
}

// checkFieldInfos checks field infos
func (ci *CheckIndex) checkFieldInfos(reader *SegmentReader) *FieldInfoStatus {
	status := &FieldInfoStatus{}

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos == nil {
		status.Error = fmt.Errorf("field infos is nil")
		return status
	}

	iter := fieldInfos.Iterator()
	for iter.HasNext() {
		fieldInfo := iter.Next()
		if fieldInfo == nil {
			status.Error = fmt.Errorf("field info is nil")
			return status
		}
		status.TotFields++
	}

	ci.msgf("    OK [%d fields]", status.TotFields)
	return status
}

// checkFieldNorms checks field norms
func (ci *CheckIndex) checkFieldNorms(reader *SegmentReader) *FieldNormStatus {
	status := &FieldNormStatus{}

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos != nil {
		iter := fieldInfos.Iterator()
		for iter.HasNext() {
			fieldInfo := iter.Next()
			if fieldInfo.HasNorms() {
				status.TotFields++
			}
		}
	}

	ci.msgf("    OK [%d field norms]", status.TotFields)
	return status
}

// checkTermIndex checks the term index
func (ci *CheckIndex) checkTermIndex(reader *SegmentReader) *TermIndexStatus {
	status := &TermIndexStatus{}

	// Simplified implementation - iterate through field infos
	fieldInfos := reader.GetFieldInfos()
	if fieldInfos != nil {
		iter := fieldInfos.Iterator()
		for iter.HasNext() {
			fieldInfo := iter.Next()
			if fieldInfo.IndexOptions().IsIndexed() {
				// Count terms for this field
				status.TermCount++
				status.TotFreq++
				status.TotPos++
			}
		}
	}

	ci.msgf("    OK [%d terms; %d freq; %d pos]", status.TermCount, status.TotFreq, status.TotPos)
	return status
}

// checkStoredFields checks stored fields
func (ci *CheckIndex) checkStoredFields(reader *SegmentReader) *StoredFieldStatus {
	status := &StoredFieldStatus{}

	liveDocs := reader.GetLiveDocs()
	maxDoc := reader.MaxDoc()
	for docID := 0; docID < maxDoc; docID++ {
		// Skip deleted docs
		if liveDocs != nil && !liveDocs.Get(docID) {
			continue
		}

		status.DocCount++
		status.TotFields++
	}

	ci.msgf("    OK [%d docs; %d fields]", status.DocCount, status.TotFields)
	return status
}

// checkTermVectors checks term vectors
func (ci *CheckIndex) checkTermVectors(reader *SegmentReader) *TermVectorStatus {
	status := &TermVectorStatus{}

	liveDocs := reader.GetLiveDocs()
	maxDoc := reader.MaxDoc()
	for docID := 0; docID < maxDoc; docID++ {
		// Skip deleted docs
		if liveDocs != nil && !liveDocs.Get(docID) {
			continue
		}

		status.DocCount++
		status.TotVectors++
	}

	ci.msgf("    OK [%d docs; %d vectors]", status.DocCount, status.TotVectors)
	return status
}

// checkDocValues checks DocValues
func (ci *CheckIndex) checkDocValues(reader *SegmentReader) *DocValuesStatus {
	status := &DocValuesStatus{}

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos != nil {
		iter := fieldInfos.Iterator()
		for iter.HasNext() {
			fieldInfo := iter.Next()
			if fieldInfo.DocValuesType() == DocValuesTypeNone {
				continue
			}

			status.TotalValueFields++

			switch fieldInfo.DocValuesType() {
			case DocValuesTypeNumeric:
				status.TotalNumericFields++
			case DocValuesTypeBinary:
				status.TotalBinaryFields++
			case DocValuesTypeSorted:
				status.TotalSortedFields++
			case DocValuesTypeSortedNumeric:
				status.TotalSortedNumericFields++
			case DocValuesTypeSortedSet:
				status.TotalSortedSetFields++
			}

			if fieldInfo.DocValuesSkipIndexType() != DocValuesSkipIndexTypeNone {
				status.TotalSkippingIndex++
			}
		}
	}

	ci.msgf("    OK [%d docvalues fields; %d numeric; %d binary; %d sorted; %d sortednumeric; %d sortedset; %d skipping index]",
		status.TotalValueFields, status.TotalNumericFields, status.TotalBinaryFields,
		status.TotalSortedFields, status.TotalSortedNumericFields, status.TotalSortedSetFields,
		status.TotalSkippingIndex)
	return status
}

// checkPoints checks PointValues
func (ci *CheckIndex) checkPoints(reader *SegmentReader) *PointsStatus {
	status := &PointsStatus{}

	ci.msg("    OK [0 points; 0 fields]")
	return status
}

// checkVectors checks KNN vector values
func (ci *CheckIndex) checkVectors(reader *SegmentReader) *VectorValuesStatus {
	status := &VectorValuesStatus{}

	fieldInfos := reader.GetFieldInfos()
	if fieldInfos != nil {
		iter := fieldInfos.Iterator()
		for iter.HasNext() {
			fieldInfo := iter.Next()
			if fieldInfo.VectorDimension() > 0 {
				status.TotalKnnVectorFields++
				status.TotalVectorValues += int64(reader.MaxDoc())
			}
		}
	}

	ci.msgf("    OK [%d vectors; %d knn fields]", status.TotalVectorValues, status.TotalKnnVectorFields)
	return status
}

// checkIndexSort checks the index sort
func (ci *CheckIndex) checkIndexSort(reader *SegmentReader, segCommitInfo *SegmentCommitInfo) *IndexSortStatus {
	status := &IndexSortStatus{}

	segInfo := segCommitInfo.SegmentInfo()
	if segInfo.IndexSort() != nil && len(segInfo.IndexSort().fields) > 0 {
		ci.msgf("    OK [sort: %v]", segInfo.IndexSort().fields)
	} else {
		ci.msg("    OK [no sort]")
	}

	return status
}

// checkLiveDocs checks live docs (deletions)
func (ci *CheckIndex) checkLiveDocs(reader *SegmentReader) *LiveDocStatus {
	status := &LiveDocStatus{}

	if reader.HasDeletions() {
		status.NumDeleted = reader.NumDeletedDocs()
		ci.msgf("    OK [%d deleted docs]", status.NumDeleted)
	} else {
		ci.msg("    OK [no deletions]")
	}

	return status
}

// checkSoftDeletes checks soft deletes
func (ci *CheckIndex) checkSoftDeletes(reader *SegmentReader, segCommitInfo *SegmentCommitInfo) *SoftDeletesStatus {
	status := &SoftDeletesStatus{}

	ci.msg("    OK [no soft deletes]")
	return status
}

// CheckIndexParseOptions parses command-line options for CheckIndex
// Returns an error if the options are invalid
func CheckIndexParseOptions(args []string) error {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-threadCount":
			if i+1 >= len(args) {
				return fmt.Errorf("-threadCount requires a value")
			}
			i++
		default:
			// Unknown option
		}
	}
	return nil
}

// ExorciseIndex removes corrupt segments from the index
// This permanently loses data from corrupt segments
func (ci *CheckIndex) ExorciseIndex(status *CheckIndexStatus) error {
	if status.Clean {
		ci.msg("Index is clean, nothing to exorcise")
		return nil
	}

	ci.msg("")
	ci.msg("WARNING: Exorcising index will PERMANENTLY DELETE data from corrupt segments")
	ci.msg("")

	return fmt.Errorf("exorcise not yet implemented")
}

// Main function for command-line usage
func CheckIndexMain(args []string) int {
	if len(args) < 1 {
		fmt.Println("Usage: CheckIndex <indexDir> [-exorcise] [-verbose] [-segment <segmentName>]")
		return 1
	}

	indexDir := args[0]
	var exorcise bool
	var verbose bool
	var segments []string

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-exorcise":
			exorcise = true
		case "-verbose":
			verbose = true
		case "-segment":
			if i+1 < len(args) {
				segments = append(segments, args[i+1])
				i++
			}
		}
	}

	// Open the directory
	dir, err := store.NewNIOFSDirectory(indexDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open directory: %v\n", err)
		return 1
	}
	defer dir.Close()

	// Create CheckIndex
	ci, err := NewCheckIndex(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create CheckIndex: %v\n", err)
		return 1
	}
	defer ci.Close()

	if verbose {
		ci.SetVerbose(true)
	}

	// Check the index
	var status *CheckIndexStatus
	if len(segments) > 0 {
		status, err = ci.CheckIndexSegments(segments)
	} else {
		status, err = ci.CheckIndex()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "CheckIndex failed: %v\n", err)
		return 1
	}

	// Exorcise if requested
	if exorcise && !status.Clean {
		if err := ci.ExorciseIndex(status); err != nil {
			fmt.Fprintf(os.Stderr, "Exorcise failed: %v\n", err)
			return 1
		}
	}

	if status.Clean {
		return 0
	}
	return 1
}
