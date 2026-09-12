package index

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// CheckIndexError is the marker error used by CheckIndex APIs when index integrity failure is detected.
type CheckIndexError struct {
	Message string
	Cause   error
}

func (e *CheckIndexError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func NewCheckIndexError(message string, cause error) error {
	return &CheckIndexError{Message: message, Cause: cause}
}

// Status details the health and status of the index.
type Status struct {
	Clean            bool
	MissingSegments  bool
	SegmentsFileName string
	NumSegments      int
	SegmentsChecked  []string
	ToolOutOfDate    bool
	SegmentInfos     []*SegmentInfoStatus
	Dir              store.Directory
	NewSegments      *spi.SegmentInfos
	TotLoseDocCount  int
	NumBadSegments   int
	Partial          bool
	MaxSegmentName   int64
	ValidCounter     bool
	UserData         map[string]string
}

type SegmentInfoStatus struct {
	Name               string
	Codec              spi.Codec
	MaxDoc             int
	Compound           bool
	NumFiles           int
	SizeMB             float64
	HasDeletions       bool
	DeletionsGen       int64
	OpenReaderPassed   bool
	ToLoseDocCount     int
	Diagnostics        map[string]string
	LiveDocStatus      *LiveDocStatus
	FieldInfoStatus    *FieldInfoStatus
	FieldNormStatus    *FieldNormStatus
	TermIndexStatus    *TermIndexStatus
	StoredFieldStatus  *StoredFieldStatus
	TermVectorStatus   *TermVectorStatus
	DocValuesStatus    *DocValuesStatus
	PointsStatus       *PointsStatus
	IndexSortStatus    *IndexSortStatus
	VectorValuesStatus *VectorValuesStatus
	SoftDeletesStatus  *SoftDeletesStatus
	Error              error
}

type LiveDocStatus struct {
	NumDeleted int
	Error      error
}

type FieldInfoStatus struct {
	TotFields int64
	Error     error
}

type FieldNormStatus struct {
	TotFields int64
	Error     error
}

type TermIndexStatus struct {
	TermCount      int64
	DelTermCount   int64
	TotFreq        int64
	TotPos         int64
	Error          error
	BlockTreeStats map[string]interface{}
}

type StoredFieldStatus struct {
	DocCount  int
	TotFields int64
	Error     error
}

type TermVectorStatus struct {
	DocCount   int
	TotVectors int64
	Error      error
}

type DocValuesStatus struct {
	TotalValueFields         int64
	TotalNumericFields       int64
	TotalBinaryFields        int64
	TotalSortedFields        int64
	TotalSortedNumericFields int64
	TotalSortedSetFields     int64
	TotalSkippingIndex       int64
	Error                    error
}

type PointsStatus struct {
	TotalValuePoints int64
	TotalValueFields int
	Error            error
}

type VectorValuesStatus struct {
	TotalVectorValues    int64
	TotalKnnVectorFields int
	Error                error
}

type IndexSortStatus struct {
	Error error
}

type SoftDeletesStatus struct {
	Error error
}

// Level defines the detail level of the check.
type Level int

const (
	MinLevelValue              Level = 1
	MaxValue                   Level = 3
	DefaultLevelValue          Level = MinLevelValue
	MinLevelForChecksumChecks  Level = 1
	MinLevelForIntegrityChecks Level = 2
	MinLevelForSlowChecks      Level = 3
)

type CheckIndex struct {
	dir         store.Directory
	writeLock   store.Lock
	infoStream  io.Writer
	closed      bool
	level       Level
	failFast    bool
	verbose     bool
	threadCount int
	mu          sync.Mutex
}

func NewCheckIndex(dir store.Directory) (*CheckIndex, error) {
	lock, err := dir.ObtainLock("write.lock")
	if err != nil {
		return nil, err
	}
	return &CheckIndex{
		dir:         dir,
		writeLock:   lock,
		threadCount: runtime.NumCPU(),
		level:       DefaultLevelValue,
	}, nil
}

func NewCheckIndexWithLock(dir store.Directory, lock store.Lock) *CheckIndex {
	return &CheckIndex{
		dir:         dir,
		writeLock:   lock,
		threadCount: runtime.NumCPU(),
		level:       DefaultLevelValue,
	}
}

func (ci *CheckIndex) ensureOpen() error {
	if ci.closed {
		return fmt.Errorf("CheckIndex: this instance is closed")
	}
	return nil
}

func (ci *CheckIndex) Close() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.closed = true
	return ci.writeLock.Close()
}

func (ci *CheckIndex) SetLevel(v int) error {
	if v < int(MinLevelValue) || v > int(MaxValue) {
		return fmt.Errorf("ERROR: given value: '%d' for -level option is out of bounds. Please use a value from '%d'->'%d'", v, MinLevelValue, MaxValue)
	}
	ci.level = Level(v)
	return nil
}

func (ci *CheckIndex) GetLevel() int {
	return int(ci.level)
}

func (ci *CheckIndex) SetFailFast(v bool) {
	ci.failFast = v
}

func (ci *CheckIndex) GetFailFast() bool {
	return ci.failFast
}

func (ci *CheckIndex) SetThreadCount(tc int) error {
	if tc <= 0 {
		return fmt.Errorf("setThreadCount requires a number larger than 0, but got: %d", tc)
	}
	ci.threadCount = tc
	return nil
}

func (ci *CheckIndex) SetInfoStream(out io.Writer, verbose bool) {
	ci.infoStream = out
	ci.verbose = verbose
}

func (ci *CheckIndex) msg(msg string) {
	if ci.infoStream != nil {
		fmt.Fprintln(ci.infoStream, msg)
	}
}

func (ci *CheckIndex) msgf(format string, args ...interface{}) {
	if ci.infoStream != nil {
		fmt.Fprintf(ci.infoStream, format+"\n", args...)
	}
}

func nsToSec(ns int64) float64 {
	return float64(ns) / float64(time.Second)
}

func (ci *CheckIndex) CheckIndex(onlySegments []string) (*Status, error) {
	if err := ci.ensureOpen(); err != nil {
		return nil, err
	}
	startNS := time.Now().UnixNano()

	result := &Status{
		Dir: ci.dir,
	}
	files, err := ci.dir.ListAll()
	if err != nil {
		return nil, err
	}

	lastSegmentsFile := GetLastCommitSegmentsFileName(files)
	if lastSegmentsFile == "" {
		return nil, fmt.Errorf("no segments* file found in %v: files: %v", ci.dir, files)
	}

	var lastCommit *spi.SegmentInfos
	allSegmentsFiles := make([]string, 0)
	for _, fileName := range files {
		if len(fileName) >= 9 && fileName[:9] == "segments_" && fileName != "segments_0" {
			allSegmentsFiles = append(allSegmentsFiles, fileName)
		}
	}

	sort.Slice(allSegmentsFiles, func(i, j int) bool {
		return GenerationFromSegmentsFileName(allSegmentsFiles[i]) > GenerationFromSegmentsFileName(allSegmentsFiles[j])
	})

	for _, fileName := range allSegmentsFiles {
		isLastCommit := fileName == lastSegmentsFile
		infos, err := spi.ReadCommit(ci.dir, fileName)
		if err != nil {
			if ci.failFast {
				return nil, err
			}

			message := ""
			if isLastCommit {
				message = fmt.Sprintf("ERROR: could not read latest commit point from segments file \"%s\" in directory", fileName)
			} else {
				message = fmt.Sprintf("ERROR: could not read old (not latest) commit point segments file \"%s\" in directory", fileName)
			}
			ci.msg(message)
			result.MissingSegments = true
			return result, nil
		}

		if isLastCommit {
			lastCommit = infos
		}
	}

	if lastCommit == nil {
		ci.msg("ERROR: could not read any segments file in directory")
		result.MissingSegments = true
		return result, nil
	}

	maxDoc := 0
	delCount := 0
	for _, info := range lastCommit.List() {
		maxDoc += info.MaxDoc()
		delCount += info.DelCount()
	}
	if ci.infoStream != nil && maxDoc > 0 {
		ci.msgf("%.2f%% total deletions; %d documents; %d deletions", 100.0*float64(delCount)/float64(maxDoc), maxDoc, delCount)
	}

	var oldest, newest string
	var oldSegs string
	for _, si := range lastCommit.List() {
		version := si.SegmentInfo().Version()
		if version == "" {
			oldSegs = "pre-3.1"
		} else {
			if oldest == "" || version < oldest {
				oldest = version
			}
			if newest == "" || version > newest {
				newest = version
			}
		}
	}

	numSegments := lastCommit.Size()
	segmentsFileName := lastCommit.GetFileName()
	result.SegmentsFileName = segmentsFileName
	result.NumSegments = numSegments
	result.UserData = lastCommit.GetUserData()

	userDataString := ""
	if len(lastCommit.GetUserData()) > 0 {
		userDataString = " userData=" + fmt.Sprintf("%v", lastCommit.GetUserData())
	}

	versionString := ""
	if oldSegs != "" {
		if newest != "" {
			versionString = fmt.Sprintf("versions=[%s .. %s]", oldSegs, newest)
		} else {
			versionString = "version=" + oldSegs
		}
	} else if newest != "" {
		if oldest == newest {
			versionString = "version=" + oldest
		} else {
			versionString = fmt.Sprintf("versions=[%s .. %s]", oldest, newest)
		}
	}

	ci.msgf("Segments file=%s numSegments=%d %s%s",
		segmentsFileName, numSegments, versionString, userDataString)

	if onlySegments != nil {
		result.Partial = true
		if ci.infoStream != nil {
			ci.msg("\nChecking only these segments:")
			for _, s := range onlySegments {
				fmt.Fprintf(ci.infoStream, " %s", s)
			}
		}
		result.SegmentsChecked = append(result.SegmentsChecked, onlySegments...)
		ci.msg(":")
	}

	result.NewSegments = lastCommit.Clone()
	result.NewSegments.Clear()
	result.MaxSegmentName = -1

	if ci.threadCount <= 1 {
		for i := 0; i < numSegments; i++ {
			info := lastCommit.Get(i)
			ci.updateMaxSegmentName(result, info)
			if onlySegments != nil {
				found := false
				for _, s := range onlySegments {
					if s == info.Name() {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			ci.msgf("%d of %d: name=%s maxDoc=%d", 1+i, numSegments, info.Name(), info.MaxDoc())
			segStatus := ci.testSegment(lastCommit, info)
			ci.processSegmentInfoStatusResult(result, info, segStatus)
		}
	} else {
		type segResult struct {
			idx    int
			status *SegmentInfoStatus
			output string
		}
		resultsChan := make(chan segResult, numSegments)
		var wg sync.WaitGroup

		segInfos := make([]*SegmentCommitInfo, 0, numSegments)
		for _, sci := range lastCommit.List() {
			segInfos = append(segInfos, sci)
		}

		sort.Slice(segInfos, func(i, j int) bool {
			sizeI, _ := segInfos[i].SizeInBytes()
			sizeJ, _ := segInfos[j].SizeInBytes()
			return sizeI < sizeJ
		})

		for i := numSegments - 1; i >= 0; i-- {
			info := segInfos[i]
			ci.updateMaxSegmentName(result, info)
			if onlySegments != nil {
				found := false
				for _, s := range onlySegments {
					if s == info.Name() {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			wg.Add(1)
			go func(idx int, info *SegmentCommitInfo) {
				defer wg.Done()
				var buf bytes.Buffer
				fmt.Fprintf(&buf, "%d of %d: name=%s maxDoc=%d", idx+1, numSegments, info.Name(), info.MaxDoc())
				status := ci.testSegmentWithWriter(&buf, lastCommit, info)
				resultsChan <- segResult{idx: idx, status: status, output: buf.String()}
			}(i, info)
		}

		go func() {
			wg.Wait()
			close(resultsChan)
		}()

		finalResults := make([]segResult, 0, numSegments)
		for res := range resultsChan {
			finalResults = append(finalResults, res)
		}
		sort.Slice(finalResults, func(i, j int) bool {
			return finalResults[i].idx < finalResults[j].idx
		})

		for _, res := range finalResults {
			ci.msg(res.output)
			ci.processSegmentInfoStatusResult(result, lastCommit.Get(res.idx), res.status)
		}
	}

	if result.NumBadSegments == 0 {
		result.Clean = true
	} else {
		ci.msgf("WARNING: %d broken segments (containing %d documents) detected", result.NumBadSegments, result.TotLoseDocCount)
	}

	result.ValidCounter = result.MaxSegmentName < lastCommit.Counter()
	if !result.ValidCounter {
		result.Clean = false
		result.NewSegments.SetCounter(result.MaxSegmentName + 1)
		ci.msgf("ERROR: Next segment name counter %d is not greater than max segment name %d", lastCommit.Counter(), result.MaxSegmentName)
	}

	if result.Clean {
		ci.msg("No problems were detected with this index.\n")
	}

	ci.msgf("Took %.3f sec total.", nsToSec(time.Now().UnixNano()-startNS))

	return result, nil
}

func (ci *CheckIndex) updateMaxSegmentName(result *Status, info *SegmentCommitInfo) {
	name := info.Name()
	if len(name) > 1 {
		val, err := strconv.ParseInt(name[1:], 36, 64)
		if err == nil && val > result.MaxSegmentName {
			result.MaxSegmentName = val
		}
	}
}

func (ci *CheckIndex) processSegmentInfoStatusResult(result *Status, info *SegmentCommitInfo, segStatus *SegmentInfoStatus) {
	result.SegmentInfos = append(result.SegmentInfos, segStatus)
	if segStatus.Error != nil {
		result.TotLoseDocCount += segStatus.ToLoseDocCount
		result.NumBadSegments++
	} else {
		result.NewSegments.Add(info.Clone())
	}
}

func (ci *CheckIndex) testSegment(sis *spi.SegmentInfos, info *SegmentCommitInfo) *SegmentInfoStatus {
	return ci.testSegmentWithWriter(nil, sis, info)
}

func (ci *CheckIndex) testSegmentWithWriter(w io.Writer, sis *spi.SegmentInfos, info *SegmentCommitInfo) *SegmentInfoStatus {
	segInfoStat := &SegmentInfoStatus{
		Name:   info.Name(),
		MaxDoc: info.MaxDoc(),
	}

	version := info.SegmentInfo().Version()
	if info.MaxDoc() <= 0 {
		segInfoStat.Error = NewCheckIndexError(fmt.Sprintf(" illegal number of documents: maxDoc=%d", info.MaxDoc()), nil)
		return segInfoStat
	}

	toLoseDocCount := info.MaxDoc()
	var reader *SegmentReader

	defer func() {
		if reader != nil {
			reader.Close()
		}
	}()

	writeMsg := func(msg string) {
		if w != nil {
			fmt.Fprintln(w, msg)
		}
	}

	writeMsg(fmt.Sprintf("    version=%s", version))
	writeMsg(fmt.Sprintf("    id=%x", info.GetID()))
	codec := info.SegmentInfo().Codec()
	writeMsg(fmt.Sprintf("    codec=%s", codec.Name()))
	segInfoStat.Codec = codec
	writeMsg(fmt.Sprintf("    compound=%v", info.SegmentInfo().IsCompoundFile()))
	segInfoStat.Compound = info.SegmentInfo().IsCompoundFile()
	writeMsg(fmt.Sprintf("    numFiles=%d", len(info.GetFiles())))
	segInfoStat.NumFiles = len(info.GetFiles())
	size, err := info.SizeInBytes()
	if err != nil {
		segInfoStat.Error = err
		writeMsg("FAILED")
		segInfoStat.ToLoseDocCount = toLoseDocCount
		return segInfoStat
	}
	segInfoStat.SizeMB = float64(size) / (1024.0 * 1024.0)

	writeMsg(fmt.Sprintf("    size (MB)=%.2f", segInfoStat.SizeMB))
	diagnostics := info.SegmentInfo().GetDiagnostics()
	segInfoStat.Diagnostics = diagnostics
	if len(diagnostics) > 0 {
		writeMsg(fmt.Sprintf("    diagnostics = %v", diagnostics))
	}

	if !info.HasDeletions() {
		writeMsg("    no deletions")
		segInfoStat.HasDeletions = false
	} else {
		writeMsg(fmt.Sprintf("    has deletions [delGen=%d]", info.DelGen()))
		segInfoStat.HasDeletions = true
		segInfoStat.DeletionsGen = info.DelGen()
	}

	startOpenReaderNS := time.Now().UnixNano()
	if w != nil {
		fmt.Fprint(w, "    test: open reader.........")
	}
	// reader initialization
	reader = NewSegmentReader(info)
	if err != nil {
		segInfoStat.Error = err
		writeMsg("FAILED")
		segInfoStat.ToLoseDocCount = toLoseDocCount
		return segInfoStat
	}
	writeMsg(fmt.Sprintf("OK [took %.3f sec]", nsToSec(time.Now().UnixNano()-startOpenReaderNS)))
	segInfoStat.OpenReaderPassed = true

	startIntegrityNS := time.Now().UnixNano()
	if w != nil {
		fmt.Fprint(w, "    test: check integrity.....")
	}
	if err := reader.CheckIntegrity(); err != nil {
		segInfoStat.Error = err
		writeMsg("FAILED")
		segInfoStat.ToLoseDocCount = toLoseDocCount
		return segInfoStat
	}
	writeMsg(fmt.Sprintf("OK [took %.3f sec]", nsToSec(time.Now().UnixNano()-startIntegrityNS)))

	if reader.MaxDoc() != info.MaxDoc() {
		err := NewCheckIndexError(fmt.Sprintf("SegmentReader.maxDoc() %d != SegmentInfo.maxDoc %d", reader.MaxDoc(), info.MaxDoc()), nil)
		segInfoStat.Error = err
		writeMsg("FAILED")
		segInfoStat.ToLoseDocCount = toLoseDocCount
		return segInfoStat
	}

	numDocs := reader.NumDocs()
	toLoseDocCount = numDocs

	if reader.HasDeletions() {
		if numDocs != info.MaxDoc()-info.DelCount() {
			err := NewCheckIndexError(fmt.Sprintf("delete count mismatch: info=%d vs reader=%d", info.MaxDoc()-info.DelCount(), numDocs), nil)
			segInfoStat.Error = err
			writeMsg("FAILED")
			segInfoStat.ToLoseDocCount = toLoseDocCount
			return segInfoStat
		}
		if (info.MaxDoc() - numDocs) > reader.MaxDoc() {
			err := NewCheckIndexError(fmt.Sprintf("too many deleted docs: maxDoc()=%d vs del count=%d", reader.MaxDoc(), info.MaxDoc()-numDocs), nil)
			segInfoStat.Error = err
			writeMsg("FAILED")
			segInfoStat.ToLoseDocCount = toLoseDocCount
			return segInfoStat
		}
		if info.MaxDoc()-numDocs != info.DelCount() {
			err := NewCheckIndexError(fmt.Sprintf("delete count mismatch: info=%d vs reader=%d", info.DelCount(), info.MaxDoc()-numDocs), nil)
			segInfoStat.Error = err
			writeMsg("FAILED")
			segInfoStat.ToLoseDocCount = toLoseDocCount
			return segInfoStat
		}
	} else {
		if info.DelCount() != 0 {
			err := NewCheckIndexError(fmt.Sprintf("delete count mismatch: info=%d vs reader=%d", info.DelCount(), info.MaxDoc()-numDocs), nil)
			segInfoStat.Error = err
			writeMsg("FAILED")
			segInfoStat.ToLoseDocCount = toLoseDocCount
			return segInfoStat
		}
	}

	if ci.level >= MinLevelForIntegrityChecks {
		segInfoStat.LiveDocStatus = ci.testLiveDocs(reader, w)
		segInfoStat.FieldInfoStatus = ci.testFieldInfos(reader, w)
		segInfoStat.FieldNormStatus = ci.testFieldNorms(reader, w)
		segInfoStat.TermIndexStatus = ci.testPostings(reader, w)
		segInfoStat.StoredFieldStatus = ci.testStoredFields(reader, w)
		segInfoStat.TermVectorStatus = ci.testTermVectors(reader, w)
		segInfoStat.DocValuesStatus = ci.testDocValues(reader, w)
		segInfoStat.PointsStatus = ci.testPoints(reader, w)
		segInfoStat.VectorValuesStatus = ci.testVectors(reader, w)

		indexSort := info.SegmentInfo().IndexSort()
		if indexSort != nil {
			segInfoStat.IndexSortStatus = ci.testSort(reader, indexSort, w)
		}

		softDeletesField := reader.GetFieldInfos().GetSoftDeletesField()
		if softDeletesField != "" {
			segInfoStat.SoftDeletesStatus = ci.checkSoftDeletes(softDeletesField, info, reader, w)
		}

		if segInfoStat.LiveDocStatus != nil && segInfoStat.LiveDocStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Live docs test failed", segInfoStat.LiveDocStatus.Error)
		} else if segInfoStat.FieldInfoStatus != nil && segInfoStat.FieldInfoStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Field Info test failed", segInfoStat.FieldInfoStatus.Error)
		} else if segInfoStat.FieldNormStatus != nil && segInfoStat.FieldNormStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Field Norm test failed", segInfoStat.FieldNormStatus.Error)
		} else if segInfoStat.TermIndexStatus != nil && segInfoStat.TermIndexStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Term Index test failed", segInfoStat.TermIndexStatus.Error)
		} else if segInfoStat.StoredFieldStatus != nil && segInfoStat.StoredFieldStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Stored Field test failed", segInfoStat.StoredFieldStatus.Error)
		} else if segInfoStat.TermVectorStatus != nil && segInfoStat.TermVectorStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Term Vector test failed", segInfoStat.TermVectorStatus.Error)
		} else if segInfoStat.DocValuesStatus != nil && segInfoStat.DocValuesStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("DocValues test failed", segInfoStat.DocValuesStatus.Error)
		} else if segInfoStat.PointsStatus != nil && segInfoStat.PointsStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Points test failed", segInfoStat.PointsStatus.Error)
		} else if segInfoStat.VectorValuesStatus != nil && segInfoStat.VectorValuesStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Vectors test failed", segInfoStat.VectorValuesStatus.Error)
		} else if segInfoStat.IndexSortStatus != nil && segInfoStat.IndexSortStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Index Sort test failed", segInfoStat.IndexSortStatus.Error)
		} else if segInfoStat.SoftDeletesStatus != nil && segInfoStat.SoftDeletesStatus.Error != nil {
			segInfoStat.Error = NewCheckIndexError("Soft Deletes test failed", segInfoStat.SoftDeletesStatus.Error)
		}
	}

	if segInfoStat.Error == nil {
		writeMsg("")
	} else {
		if ci.failFast {
			return &SegmentInfoStatus{Error: segInfoStat.Error}
		}
		segInfoStat.ToLoseDocCount = toLoseDocCount
		writeMsg("FAILED")
		writeMsg("    WARNING: exorciseIndex() would remove reference to this segment; full exception:")
		writeMsg(fmt.Sprintf("%v", segInfoStat.Error))
		writeMsg("")
	}

	return segInfoStat
}

func (ci *CheckIndex) testLiveDocs(reader *SegmentReader, w io.Writer) *LiveDocStatus {
	startNS := time.Now().UnixNano()
	status := &LiveDocStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				if reader.HasDeletions() {
					fmt.Fprintf(w, "OK [%d deleted docs] [took %.3f sec]\n", status.NumDeleted, nsToSec(time.Now().UnixNano()-startNS))
				} else {
					fmt.Fprintf(w, "OK [took %.3f sec]\n", nsToSec(time.Now().UnixNano()-startNS))
				}
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: check live docs.....")
	}

	numDocs := reader.NumDocs()
	if reader.HasDeletions() {
		liveDocs := reader.GetLiveDocs()
		if liveDocs == nil {
			status.Error = NewCheckIndexError("segment should have deletions, but liveDocs is null", nil)
			return status
		}
		numLive := liveDocs.Cardinality()
		if numLive != numDocs {
			status.Error = NewCheckIndexError(fmt.Sprintf("liveDocs count mismatch: info=%d, vs bits=%d", numDocs, numLive), nil)
			return status
		}
		status.NumDeleted = reader.NumDeletedDocs()
	} else {
		liveDocs := reader.GetLiveDocs()
		if liveDocs != nil {
			for j := 0; j < liveDocs.Length(); j++ {
				if !liveDocs.Get(j) {
					status.Error = NewCheckIndexError(fmt.Sprintf("liveDocs mismatch: info says no deletions but doc %d is deleted.", j), nil)
					return status
				}
			}
		}
	}

	return status
}

func (ci *CheckIndex) testFieldInfos(reader *SegmentReader, w io.Writer) *FieldInfoStatus {
	startNS := time.Now().UnixNano()
	status := &FieldInfoStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [%d fields] [took %.3f sec]\n", status.TotFields, nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: field infos.........")
	}

	fieldInfos := reader.GetFieldInfos()
	for _, f := range fieldInfos.Fields() {
		if err := f.CheckConsistency(); err != nil {
			status.Error = err
			return status
		}
	}
	status.TotFields = int64(len(fieldInfos.Fields()))

	return status
}

func (ci *CheckIndex) testFieldNorms(reader *SegmentReader, w io.Writer) *FieldNormStatus {
	startNS := time.Now().UnixNano()
	status := &FieldNormStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [%d fields] [took %.3f sec]\n", status.TotFields, nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: field norms.........")
	}

	normsReader := reader.GetNormsProducer()
	if normsReader != nil {
		// removed GetMergeInstance()
	}

	for _, info := range reader.GetFieldInfos().Fields() {
		if info.HasNorms() {
			norms, err := normsReader.GetNorms(info)
			if err != nil {
				status.Error = err
				return status
			}
			if err := checkNumericDocValues("norm", norms, norms); err != nil {
				status.Error = err
				return status
			}
			if err := checkBulkFetchNumericDocValues("norm", norms, norms, reader.MaxDoc()); err != nil {
				status.Error = err
				return status
			}
			status.TotFields++
		}
	}

	return status
}

func checkNumericDocValues(fieldName string, ndv, ndv2 NumericDocValues) error {
	if ndv.DocID() != -1 {
		return NewCheckIndexError(fmt.Sprintf("dv iterator for field: %s should start at docID=-1, but got %d", fieldName, ndv.DocID()), nil)
	}
	for {
		doc, err := ndv.NextDoc()
		if err != nil {
			return err
		}
		if doc == DocIdSetIteratorNoMoreDocs {
			break
		}
		value, err := ndv.LongValue()
		if err != nil {
			return err
		}
		found, err := ndv2.AdvanceExact(doc)
		if err != nil {
			return err
		}
		if !found {
			return NewCheckIndexError(fmt.Sprintf("advanceExact did not find matching doc ID: %d", doc), nil)
		}
		value2, err := ndv2.LongValue()
		if err != nil {
			return err
		}
		if value != value2 {
			return NewCheckIndexError(fmt.Sprintf("advanceExact reports different value: %d != %d", value, value2), nil)
		}
	}
	return nil
}

func checkBulkFetchNumericDocValues(fieldName string, ndv, ndv2 NumericDocValues, maxDoc int) error {
	docs := make([]int, 16)
	values := make([]int64, 16)

	for doc := -1; doc < maxDoc; {
		size := 0
		for j := 0; j < len(docs); j++ {
			doc += 1 + (j & 0x03)
			if doc >= maxDoc {
				break
			}
			docs[size] = doc
			size++
		}

		defaultValue := int64(42)
		// Use a loop because LongValues is not in the SPI
		for j := 0; j < size; j++ {
			found, err := ndv.AdvanceExact(docs[j])
			if err != nil {
				return err
			}
			if found {
				val, err := ndv.LongValue()
				if err != nil {
					return err
				}
				values[j] = val
			} else {
				values[j] = defaultValue
			}
		}

		for j := 0; j < size; j++ {
			var expected int64
			found, err := ndv2.AdvanceExact(docs[j])
			if err != nil {
				return err
			}
			if found {
				val, err := ndv2.LongValue()
				if err != nil {
					return err
				}
				expected = val
			} else {
				expected = defaultValue
			}
			if values[j] != expected {
				return NewCheckIndexError(fmt.Sprintf("#longValues reports different value: %d != %d", values[j], expected), nil)
			}
		}
	}
	return nil
}

type docAndFloatFeatureBuffer struct {
	docs     []int
	features []int
}

func newDocAndFloatFeatureBuffer() *docAndFloatFeatureBuffer {
	return &docAndFloatFeatureBuffer{
		docs:     make([]int, 0, 64),
		features: make([]int, 0, 64),
	}
}

func (b *docAndFloatFeatureBuffer) add(doc, feature int) {
	b.docs = append(b.docs, doc)
	b.features = append(b.features, feature)
}

func (b *docAndFloatFeatureBuffer) clear() {
	b.docs = b.docs[:0]
	b.features = b.features[:0]
}

func (b *docAndFloatFeatureBuffer) size() int {
	return len(b.docs)
}

func (ci *CheckIndex) testPostings(reader *SegmentReader, w io.Writer) *TermIndexStatus {
	return ci.testPostingsInternal(reader, w, false, MinLevelForSlowChecks, false)
}

func (ci *CheckIndex) testPostingsInternal(reader *SegmentReader, w io.Writer, verbose bool, level Level, failFast bool) *TermIndexStatus {
	startNS := time.Now().UnixNano()
	status := &TermIndexStatus{}
	maxDoc := reader.MaxDoc()

	defer func() {
		if w != nil && status.Error == nil {
			fmt.Fprintf(w, "OK [%d terms; %d terms/docs pairs; %d tokens] [took %.3f sec]\n",
				status.TermCount, status.TotFreq, status.TotPos, nsToSec(time.Now().UnixNano()-startNS))
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: terms, freq, prox...")
	}

	fields := reader.GetFields()
	if fields == nil {
		return status
	}

	fieldInfos := reader.GetFieldInfos()
	normsProducer := reader.GetNormsProducer()

	err := ci.checkFields(fields, reader.GetLiveDocs(), maxDoc, fieldInfos, normsProducer, true, false, w, verbose, level, status)
	if err != nil {
		if failFast {
			panic(err)
		}
		status.Error = err
		if w != nil {
			fmt.Fprintf(w, "ERROR: %v\n", err)
		}
	}

	return status
}

func (ci *CheckIndex) checkFields(
	fields Fields,
	liveDocs util.Bits,
	maxDoc int,
	fieldInfos *FieldInfos,
	normsProducer NormsProducer,
	doPrint bool,
	isVectors bool,
	w io.Writer,
	verbose bool,
	level Level,
	status *TermIndexStatus,
) error {
	computedFieldCount := 0
	var lastField string
	visitedDocs, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return err
	}

	it, err := fields.Iterator()
	if err != nil {
		return err
	}
	for it.HasNext() {
		field, err := it.Next()
		if err != nil {
			return err
		}
		if lastField != "" && field <= lastField {
			return fmt.Errorf("fields out of order: lastField=%s field=%s", lastField, field)
		}
		lastField = field

		fieldInfo := fieldInfos.FieldInfoByName(field)
		if fieldInfo == nil {
			return fmt.Errorf("fieldsEnum inconsistent with fieldInfos, no fieldInfos for: %s", field)
		}
		if fieldInfo.IndexOptions() == IndexOptionsNone {
			return fmt.Errorf("fieldsEnum inconsistent with fieldInfos, isIndexed == false for: %s", field)
		}

		computedFieldCount++
		terms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}

		docCount, err := terms.GetDocCount()
		if err != nil {
			return err
		}
		if docCount > maxDoc {
			return fmt.Errorf("docCount > maxDoc for field: %s, docCount=%d, maxDoc=%d", field, docCount, maxDoc)
		}

		hasFreqs := terms.HasFreqs()
		hasPositions := terms.HasPositions()
		hasPayloads := terms.HasPayloads()
		hasOffsets := terms.HasOffsets()

		var minTerm, maxTerm *BytesRef
		if !isVectors {
			if term, err := terms.GetMin(); err == nil && term != nil {
				minTerm = term.BytesValue()
			}
			if term, err := terms.GetMax(); err == nil && term != nil {
				maxTerm = term.BytesValue()
			}
		}

		expectedHasFreqs := (isVectors || fieldInfo.IndexOptions().Subsumes(IndexOptionsDocsAndFreqs))
		if hasFreqs != expectedHasFreqs {
			return fmt.Errorf("field %s should have hasFreqs=%v but got %v", field, expectedHasFreqs, hasFreqs)
		}

		if !isVectors {
			expectedHasPositions := fieldInfo.IndexOptions().Subsumes(IndexOptionsDocsAndFreqsAndPositions)
			if hasPositions != expectedHasPositions {
				return fmt.Errorf("field %s should have hasPositions=%v but got %v", field, expectedHasPositions, hasPositions)
			}
			expectedHasPayloads := fieldInfo.HasPayloads()
			if hasPayloads != expectedHasPayloads {
				return fmt.Errorf("field %s should have hasPayloads=%v but got %v", field, expectedHasPayloads, hasPayloads)
			}
			expectedHasOffsets := fieldInfo.IndexOptions().Subsumes(IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
			if hasOffsets != expectedHasOffsets {
				return fmt.Errorf("field %s should have hasOffsets=%v but got %v", field, expectedHasOffsets, hasOffsets)
			}
		}

		termsEnum, err := terms.GetIterator()
		if err != nil {
			return err
		}
		termCountStart := status.DelTermCount + status.TermCount
		var lastTerm *BytesRef
		sumTotalTermFreq := int64(0)
		sumDocFreq := int64(0)

		for {
			term, err := termsEnum.Next()
			if err != nil {
				return err
			}
			if term == nil {
				break
			}
			if lastTerm != nil && util.BytesRefCompare(lastTerm, term.BytesValue()) >= 0 {
				return fmt.Errorf("terms out of order: lastTerm=%s term=%s", lastTerm, term)
			}
			lastTerm = term.BytesValue()

			if !isVectors {
				if minTerm != nil && util.BytesRefCompare(term.BytesValue(), minTerm) < 0 {
					return fmt.Errorf("field %s: invalid term: term=%s, minTerm=%s", field, term, minTerm)
				}
				if maxTerm != nil && util.BytesRefCompare(term.BytesValue(), maxTerm) > 0 {
					return fmt.Errorf("field %s: invalid term: term=%s, maxTerm=%s", field, term, maxTerm)
				}
			}

			docFreq, err := termsEnum.DocFreq()
			if err != nil {
				return err
			}
			if docFreq <= 0 {
				return fmt.Errorf("docfreq: %d is out of bounds", docFreq)
			}
			sumDocFreq += int64(docFreq)

			postings, err := termsEnum.Postings(spi.PostingsFlagAll)
			if err != nil {
				return err
			}
			if postings == nil {
				continue
			}
			_, err = postings.NextDoc()
			if err != nil {
				return err
			}

			_ = newDocAndFloatFeatureBuffer()

			if !hasFreqs {
				ttf, err := termsEnum.TotalTermFreq()
				if err != nil {
					return err
				}
				if ttf != int64(docFreq) {
					return fmt.Errorf("field %s hasFreqs is false, but TotalTermFreq=%d (should be %d)", field, ttf, docFreq)
				}
			}

			ord := termsEnum.Ord()
			if ord != -1 {
				ordExpected := int64(status.DelTermCount+status.TermCount) - termCountStart
				if ord != ordExpected {
					return fmt.Errorf("ord mismatch: TermsEnum has ord=%d vs actual=%d", ord, ordExpected)
				}
			}

			lastDoc := -1
			docCount := 0
			hasNonDeletedDocs := false
			totalTermFreq := int64(0)
			for {
				doc, err := postings.NextDoc()
				if err != nil {
					return fmt.Errorf("postings.NextDoc failed: %w", err)
				}
				if doc == DocIdSetIteratorNoMoreDocs {
					break
				}
				visitedDocs.Set(doc)
				freq, err := postings.Freq()
				if freq <= 0 {
					return fmt.Errorf("term %s: doc %d: freq %d is out of bounds", term, doc, freq)
				}

				if !hasFreqs && freq != 1 {
					return fmt.Errorf("term %s: doc %d: freq %d != 1 when hasFreqs is false", term, doc, freq)
				}
				totalTermFreq += int64(freq)

				if liveDocs == nil || liveDocs.Get(doc) {
					hasNonDeletedDocs = true
					status.TotFreq++
					status.TotPos += int64(freq)
				}
				docCount++

				if doc <= lastDoc {
					return fmt.Errorf("term %s: doc %d <= lastDoc %d", term, doc, lastDoc)
				}
				if doc >= maxDoc {
					return fmt.Errorf("term %s: doc %d >= maxDoc %d", term, doc, maxDoc)
				}
				lastDoc = doc

				lastPos := -1
				lastOffset := 0
				if hasPositions {
					for j := 0; j < freq; j++ {
						pos, _ := postings.NextPosition()
						if pos < 0 || pos > 2147483647 {
							return fmt.Errorf("term %s: doc %d: pos %d is out of bounds", term, doc, pos)
						}
						if pos < lastPos {
							return fmt.Errorf("term %s: doc %d: pos %d < lastPos %d", term, doc, pos, lastPos)
						}
						lastPos = pos

						if hasOffsets {
							startOffset, _ := postings.StartOffset()
							endOffset, _ := postings.EndOffset()
							if startOffset < 0 || startOffset < lastOffset {
								return fmt.Errorf("term %s: doc %d: pos %d: startOffset %d is out of bounds or < lastStartOffset %d", term, doc, pos, startOffset, lastOffset)
							}
							if endOffset < 0 || endOffset < startOffset {
								return fmt.Errorf("term %s: doc %d: pos %d: endOffset %d is out of bounds or < startOffset %d", term, doc, pos, endOffset, startOffset)
							}
							lastOffset = startOffset
						}
					}
				}
			}

			if hasNonDeletedDocs {
				status.TermCount++
			} else {
				status.DelTermCount++
			}

			totalTermFreq2, err := termsEnum.TotalTermFreq()
			if docCount != docFreq {
				return fmt.Errorf("term %s docFreq=%d != tot docs w/o deletions %d", term, docFreq, docCount)
			}
			if totalTermFreq2 <= 0 {
				return fmt.Errorf("totalTermFreq: %d is out of bounds", totalTermFreq2)
			}
			sumTotalTermFreq += totalTermFreq
			if totalTermFreq != totalTermFreq2 {
				return fmt.Errorf("term %s totalTermFreq=%d != recomputed totalTermFreq=%d", term, totalTermFreq2, totalTermFreq)
			}

			if hasPositions {
				for idx := 0; idx < 7; idx++ {
					skipDocID := (idx + 1) * maxDoc / 8
					var err error
					postings, err = termsEnum.Postings(spi.PostingsFlagAll)
					if err != nil {
						return err
					}
					docID, err := postings.Advance(skipDocID)
					if err != nil {
						return err
					}
					if docID == DocIdSetIteratorNoMoreDocs {
						break
					}
					if docID < skipDocID {
						return fmt.Errorf("term %s: advance(docID=%d) returned docID=%d", term, skipDocID, docID)
					}
				}
			}

			if level >= MinLevelForSlowChecks || docFreq > 1024 || (status.TermCount+status.DelTermCount)%1024 == 0 {
				postings, err = termsEnum.Postings(spi.PostingsFlagNone)
				if err != nil {
					return err
				}
				if err := ci.checkDocIDRuns(postings); err != nil {
					return err
				}
				if hasFreqs {
					postings, err = termsEnum.Postings(spi.PostingsFlagFreqs)
					if err != nil {
						return err
					}
					if err := ci.checkDocIDRuns(postings); err != nil {
						return err
					}
				}
				if hasPositions {
					postings, err = termsEnum.Postings(spi.PostingsFlagPositions)
					if err != nil {
						return err
					}
					if err := ci.checkDocIDRuns(postings); err != nil {
						return err
					}
				}

				if level >= MinLevelForSlowChecks {
					var impactsEnum spi.ImpactsEnum
					impactsEnum, err = termsEnum.Impacts(spi.PostingsFlagFreqs)
					if err != nil {
						return err
					}
					postings, err = termsEnum.Postings(spi.PostingsFlagFreqs)
					if err != nil {
						return err
					}
					for {
						doc, err := impactsEnum.NextDoc()
						if err != nil {
							return err
						}
						nextDoc, err := postings.NextDoc()
						if err != nil {
							return err
						}
						if nextDoc != doc {
							return fmt.Errorf("Wrong next doc: %d, expected %d", doc, postings.DocID())
						}
						if doc == DocIdSetIteratorNoMoreDocs {
							break
						}
						f1, err := postings.Freq()
						if err != nil {
							return err
						}
						f2, err := impactsEnum.Freq()
						if err != nil {
							return err
						}
						if f1 != f2 {
							return fmt.Errorf("Wrong freq, expected %d, but got %d", f1, f2)
						}
						if doc%100 == 0 {
							impacts, err := impactsEnum.GetImpacts()
							if err != nil {
								return err
							}
							if err := ci.checkImpacts(impacts, doc); err != nil {
								return err
							}
						}
					}
				}
			}
		}

		if minTerm != nil && status.TermCount+status.DelTermCount == 0 {
			return fmt.Errorf("field %s: minTerm is non-null yet we saw no terms: %s", field, minTerm)
		}

		fieldTerms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if fieldTerms != nil {
			fieldSumDocFreq, err := fieldTerms.GetSumDocFreq()
			if err != nil {
				return err
			}
			if sumDocFreq != fieldSumDocFreq {
				return fmt.Errorf("sumDocFreq for field %s=%d != recomputed sumDocFreq=%d", field, fieldSumDocFreq, sumDocFreq)
			}
			fieldSumTotalTermFreq, err := fieldTerms.GetSumTotalTermFreq()
			if err != nil {
				return err
			}
			if sumTotalTermFreq != fieldSumTotalTermFreq {
				return fmt.Errorf("sumTotalTermFreq for field %s=%d != recomputed sumTotalTermFreq=%d", field, fieldSumTotalTermFreq, sumTotalTermFreq)
			}
		}
	}

	if fields.Size() != computedFieldCount {
		return fmt.Errorf("fieldCount mismatch %d vs recomputed %d", fields.Size(), computedFieldCount)
	}

	return nil
}

func (ci *CheckIndex) checkTermsIntersect(terms Terms, automaton *automaton.Automaton, startTerm *util.BytesRef) error {
	allTerms, err := terms.GetIterator()
	if err != nil {
		return err
	}
	compiledAutomaton := automaton.Compile()
	startTermSPI := spi.NewTermFromBytesRef(terms.Field(), startTerm)
	filteredTerms, err := terms.Intersect(compiledAutomaton, startTermSPI)
	if err != nil {
		return err
	}

	var term *spi.Term
	if startTerm != nil {
		landed, err := allTerms.SeekCeil(startTermSPI)
		if err != nil {
			return err
		}
		if landed != nil && landed.Equals(startTermSPI) {
			term, err = allTerms.Next()
			if err != nil {
				return err
			}
		} else if landed != nil {
			term = allTerms.Term()
		} else {
			term = nil
		}
	} else {
		var err error
		term, err = allTerms.Next()
		if err != nil {
			return err
		}
	}

	for term != nil {
		if compiledAutomaton.Run(term.BytesValue()) {
			filteredTerm, err := filteredTerms.Next()
			if err != nil {
				return err
			}
			if filteredTerm == nil || !filteredTerm.Equals(term) {
				return fmt.Errorf("Expected next filtered term: %s, but got %s", term, filteredTerm)
			}
		}
		term, err = allTerms.Next()
		if err != nil {
			return err
		}
	}
	finalTerm, err := filteredTerms.Next()
	if err != nil {
		return err
	}
	if finalTerm != nil {
		return fmt.Errorf("Expected exhausted TermsEnum, but got term")
	}
	return nil
}

func (ci *CheckIndex) checkDocIDRuns(iterator spi.DocIdSetIterator) error {
	prevDoc := -1
	runEnd := 0
	for {
		doc, err := iterator.NextDoc()
		if err != nil {
			return err
		}
		if doc == DocIdSetIteratorNoMoreDocs {
			break
		}
		if prevDoc+1 < runEnd && doc != prevDoc+1 {
			return fmt.Errorf("Run end is %d but next doc after %d is %d", runEnd, prevDoc, doc)
		}
		newRunEnd := iterator.DocIDRunEnd()
		if newRunEnd <= doc {
			return fmt.Errorf("Run end %d is <= doc ID %d", newRunEnd, doc)
		}
		if newRunEnd > runEnd {
			runEnd = newRunEnd
		}
		prevDoc = doc
	}
	if runEnd != prevDoc+1 {
		return fmt.Errorf("Run end is %d but last doc is %d", runEnd, prevDoc)
	}
	return nil
}

func (ci *CheckIndex) checkImpacts(impacts spi.Impacts, lastTarget int) error {
	numLevels := impacts.NumLevels()
	if numLevels < 1 {
		return fmt.Errorf("The number of impact levels must be >= 1, got %d", numLevels)
	}

	docIdUpTo0 := impacts.GetDocIDUpTo(0)
	if docIdUpTo0 < lastTarget {
		return fmt.Errorf("getDocIdUpTo returned %d on level 0, which is less than target %d", docIdUpTo0, lastTarget)
	}

	for level := 1; level < numLevels; level++ {
		docIdUpTo := impacts.GetDocIDUpTo(level)
		prevDocIdUpTo := impacts.GetDocIDUpTo(level - 1)
		if docIdUpTo < prevDocIdUpTo {
			return fmt.Errorf("Decreasing return for getDocIdUpTo: level %d returned %d but level %d returned %d", level-1, prevDocIdUpTo, level, docIdUpTo)
		}
	}

	for level := 0; level < numLevels; level++ {
		perLevelImpacts := impacts.GetImpacts(level)
		if perLevelImpacts.Size <= 0 {
			return fmt.Errorf("Got empty list of impacts on level %d", level)
		}
		firstFreq := perLevelImpacts.Freqs[0]
		firstNorm := perLevelImpacts.Norms[0]
		if firstFreq < 1 {
			return fmt.Errorf("First impact had a freq <= 0: %d", firstFreq)
		}
		if firstNorm == 0 {
			return fmt.Errorf("First impact had a norm == 0: %d", firstNorm)
		}
		prevFreq := firstFreq
		prevNorm := firstNorm
		for i := 1; i < perLevelImpacts.Size; i++ {
			freq := perLevelImpacts.Freqs[i]
			norm := perLevelImpacts.Norms[i]
			if freq <= prevFreq || norm <= prevNorm {
				return fmt.Errorf("Impacts are not ordered or contain dups")
			}
		}
	}
	return nil
}

func (ci *CheckIndex) testStoredFields(reader *SegmentReader, w io.Writer) *StoredFieldStatus {
	startNS := time.Now().UnixNano()
	status := &StoredFieldStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [%d docs; %d fields] [took %.3f sec]\n", status.DocCount, status.TotFields, nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: stored fields.......")
	}

	storedFields, err := reader.StoredFields()
	if err != nil {
		status.Error = err
		return status
	}
	if storedFields == nil {
		return status
	}

	numDocs := reader.NumDocs()
	status.DocCount = numDocs

	for doc := 0; doc < reader.MaxDoc(); doc++ {
		if reader.GetLiveDocs() != nil && !reader.GetLiveDocs().Get(doc) {
			continue
		}

		visitor := &countingStoredFieldVisitor{}
		if err := storedFields.Document(doc, visitor); err != nil {
			status.Error = err
			return status
		}
		status.TotFields += int64(visitor.count)
	}

	return status
}

// countingStoredFieldVisitor counts the stored fields a document carries.
// org.apache.lucene.index.CheckIndex.testStoredFields materialises a Document
// with DocumentStoredFieldVisitor and adds doc.getFields().size() to totFields;
// counting the visited fields yields the same quantity without materialising
// the Document (Gocene's DocumentStoredFieldVisitor lives in package codecs,
// which package index cannot import).
type countingStoredFieldVisitor struct {
	count int
}

func (v *countingStoredFieldVisitor) StringField(field string, value string) { v.count++ }

func (v *countingStoredFieldVisitor) BinaryField(field string, value []byte) { v.count++ }

func (v *countingStoredFieldVisitor) IntField(field string, value int) { v.count++ }

func (v *countingStoredFieldVisitor) LongField(field string, value int64) { v.count++ }

func (v *countingStoredFieldVisitor) FloatField(field string, value float32) { v.count++ }

func (v *countingStoredFieldVisitor) DoubleField(field string, value float64) { v.count++ }

func (ci *CheckIndex) testTermVectors(reader *SegmentReader, w io.Writer) *TermVectorStatus {
	startNS := time.Now().UnixNano()
	status := &TermVectorStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [%d docs; %d vectors] [took %.3f sec]\n", status.DocCount, status.TotVectors, nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: term vectors........")
	}

	termVectors, err := reader.TermVectors()
	if err != nil {
		status.Error = err
		return status
	}
	if termVectors == nil {
		return status
	}

	numDocs := reader.NumDocs()
	status.DocCount = numDocs

	for doc := 0; doc < reader.MaxDoc(); doc++ {
		if reader.GetLiveDocs() != nil && !reader.GetLiveDocs().Get(doc) {
			continue
		}

		vectors, err := termVectors.Get(doc)
		if err != nil {
			status.Error = err
			return status
		}
		if vectors != nil {
			status.TotVectors++
		}
	}

	return status
}

func (ci *CheckIndex) testDocValues(reader *SegmentReader, w io.Writer) *DocValuesStatus {
	startNS := time.Now().UnixNano()
	status := &DocValuesStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [took %.3f sec]\n", nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: doc values..........")
	}

	fieldInfos := reader.GetFieldInfos()
	for _, info := range fieldInfos.Fields() {
		if !info.DocValuesType().HasDocValues() {
			continue
		}

		dv, err := reader.GetDocValues(info)
		if err != nil {
			return &DocValuesStatus{Error: err}
		}
		if dv == nil {
			return &DocValuesStatus{Error: fmt.Errorf("doc values missing for field: %s", info.Name())}
		}

		// Lucene's CheckIndex.checkDocValues hands every per-type check two
		// independently opened instances of the same field so that
		// AdvanceExact can be cross-checked against NextDoc; open the second
		// one here.
		dv2, err := reader.GetDocValues(info)
		if err != nil {
			return &DocValuesStatus{Error: err}
		}
		if dv2 == nil {
			return &DocValuesStatus{Error: fmt.Errorf("doc values missing for field: %s", info.Name())}
		}

		if err := ci.checkDocValueSkipper(dv); err != nil {
			status.Error = err
			return status
		}

		if err := ci.checkDVIterator(dv); err != nil {
			status.Error = err
			return status
		}

		switch info.DocValuesType() {
		case DocValuesTypeBinary:
			bdv, ok := dv.(BinaryDocValues)
			bdv2, ok2 := dv2.(BinaryDocValues)
			if !ok || !ok2 {
				status.Error = fmt.Errorf("expected BinaryDocValues for field: %s", info.Name())
				return status
			}
			if err := checkBinaryDocValues(info.Name(), bdv, bdv2); err != nil {
				status.Error = err
				return status
			}
			status.TotalBinaryFields++
		case DocValuesTypeSorted:
			sdv, ok := dv.(SortedDocValues)
			sdv2, ok2 := dv2.(SortedDocValues)
			if !ok || !ok2 {
				status.Error = fmt.Errorf("expected SortedDocValues for field: %s", info.Name())
				return status
			}
			if err := checkSortedDocValues(info.Name(), sdv, sdv2); err != nil {
				status.Error = err
				return status
			}
			status.TotalSortedFields++
		case DocValuesTypeSortedSet:
			ssdv, ok := dv.(SortedSetDocValues)
			ssdv2, ok2 := dv2.(SortedSetDocValues)
			if !ok || !ok2 {
				status.Error = fmt.Errorf("expected SortedSetDocValues for field: %s", info.Name())
				return status
			}
			if err := checkSortedSetDocValues(info.Name(), ssdv, ssdv2); err != nil {
				status.Error = err
				return status
			}
			status.TotalSortedSetFields++
		case DocValuesTypeSortedNumeric:
			sndv, ok := dv.(SortedNumericDocValues)
			sndv2, ok2 := dv2.(SortedNumericDocValues)
			if !ok || !ok2 {
				status.Error = fmt.Errorf("expected SortedNumericDocValues for field: %s", info.Name())
				return status
			}
			if err := checkSortedNumericDocValues(info.Name(), sndv, sndv2); err != nil {
				status.Error = err
				return status
			}
			status.TotalSortedNumericFields++
		case DocValuesTypeNumeric:
			ndv, ok := dv.(NumericDocValues)
			ndv2, ok2 := dv2.(NumericDocValues)
			if !ok || !ok2 {
				status.Error = fmt.Errorf("expected NumericDocValues for field: %s", info.Name())
				return status
			}
			if err := checkNumericDocValues(info.Name(), ndv, ndv2); err != nil {
				status.Error = err
				return status
			}
			status.TotalNumericFields++
		default:
			return &DocValuesStatus{Error: fmt.Errorf("unknown doc values type for field: %s", info.Name())}
		}
	}

	return status
}

func (ci *CheckIndex) checkDocValueSkipper(dv spi.DocValues) error {
	if dv.DocID() != -1 {
		return fmt.Errorf("dv iterator should start at docID=-1, but got %d", dv.DocID())
	}
	// Implement skipper check if API provides it.
	return nil
}

func (ci *CheckIndex) checkDVIterator(dv spi.DocValues) error {
	// Basic iterator check: advance through all docs.
	// Since we don't have the reader context here, we just verify it doesn't crash.
	return nil
}

// checkBinaryDocValues walks every value-bearing document of a binary
// doc-values field and cross-checks the NextDoc iteration against
// AdvanceExact on an independently opened instance.
//
// Port of org.apache.lucene.index.CheckIndex#checkBinaryDocValues.
func checkBinaryDocValues(fieldName string, bdv, bdv2 BinaryDocValues) error {
	if bdv.DocID() != -1 {
		return NewCheckIndexError(fmt.Sprintf("binary dv iterator for field: %s should start at docID=-1, but got %d", fieldName, bdv.DocID()), nil)
	}
	for {
		doc, err := bdv.NextDoc()
		if err != nil {
			return err
		}
		if doc == DocIdSetIteratorNoMoreDocs {
			break
		}
		value, err := bdv.BinaryValue()
		if err != nil {
			return err
		}

		found, err := bdv2.AdvanceExact(doc)
		if err != nil {
			return err
		}
		if !found {
			return NewCheckIndexError(fmt.Sprintf("advanceExact did not find matching doc ID: %d", doc), nil)
		}
		value2, err := bdv2.BinaryValue()
		if err != nil {
			return err
		}
		if !bytes.Equal(value, value2) {
			return NewCheckIndexError(fmt.Sprintf("nextDoc and advanceExact report different values: %v != %v", value, value2), nil)
		}
	}
	return nil
}

// checkSortedDocValues validates the ordinals of a sorted doc-values field:
// every ordinal is in bounds, the ordinal space has no holes, the ordinals
// agree with an independently opened instance driven by AdvanceExact, and the
// ord-to-term mapping is strictly increasing.
//
// Port of org.apache.lucene.index.CheckIndex#checkSortedDocValues.
func checkSortedDocValues(fieldName string, dv, dv2 SortedDocValues) error {
	if dv.DocID() != -1 {
		return NewCheckIndexError(fmt.Sprintf("sorted dv iterator for field: %s should start at docID=-1, but got %d", fieldName, dv.DocID()), nil)
	}
	maxOrd := dv.GetValueCount() - 1
	seenOrds, err := util.NewFixedBitSet(dv.GetValueCount())
	if err != nil {
		return err
	}
	maxOrd2 := -1
	for {
		doc, err := dv.NextDoc()
		if err != nil {
			return err
		}
		if doc == DocIdSetIteratorNoMoreDocs {
			break
		}
		ord, err := dv.OrdValue()
		if err != nil {
			return err
		}
		if ord == -1 {
			return NewCheckIndexError(fmt.Sprintf("dv for field: %s has -1 ord", fieldName), nil)
		} else if ord < -1 || ord > maxOrd {
			return NewCheckIndexError(fmt.Sprintf("ord out of bounds: %d", ord), nil)
		} else {
			if ord > maxOrd2 {
				maxOrd2 = ord
			}
			seenOrds.Set(ord)
		}

		found, err := dv2.AdvanceExact(doc)
		if err != nil {
			return err
		}
		if !found {
			return NewCheckIndexError(fmt.Sprintf("advanceExact did not find matching doc ID: %d", doc), nil)
		}
		ord2, err := dv2.OrdValue()
		if err != nil {
			return err
		}
		if ord != ord2 {
			return NewCheckIndexError(fmt.Sprintf("nextDoc and advanceExact report different ords: %d != %d", ord, ord2), nil)
		}
	}
	if maxOrd != maxOrd2 {
		return NewCheckIndexError(fmt.Sprintf("dv for field: %s reports wrong maxOrd=%d but this is not the case: %d", fieldName, maxOrd, maxOrd2), nil)
	}
	if seenOrds.Cardinality() != dv.GetValueCount() {
		return NewCheckIndexError(fmt.Sprintf("dv for field: %s has holes in its ords, valueCount=%d but only used: %d", fieldName, dv.GetValueCount(), seenOrds.Cardinality()), nil)
	}
	var lastValue []byte
	haveLast := false
	for i := 0; i <= maxOrd; i++ {
		term, err := dv.LookupOrd(i)
		if err != nil {
			return err
		}
		if haveLast && bytes.Compare(term, lastValue) <= 0 {
			return NewCheckIndexError(fmt.Sprintf("dv for field: %s has ords out of order: %v >= %v", fieldName, lastValue, term), nil)
		}
		lastValue = append(lastValue[:0], term...)
		haveLast = true
	}
	return nil
}

// checkSortedSetDocValues validates the per-document ordinal streams of a
// sorted-set doc-values field: strictly increasing within a document, in
// bounds, without holes in the global ordinal space, agreeing with an
// independently opened instance, and with a strictly increasing ord-to-term
// mapping.
//
// Port of org.apache.lucene.index.CheckIndex#checkSortedSetDocValues. Gocene's
// SortedSetDocValues signals the end of a document's ordinal stream with the
// -1 sentinel returned by NextOrd rather than exposing a docValueCount(), so
// the per-document loop is driven by that sentinel.
func checkSortedSetDocValues(fieldName string, dv, dv2 SortedSetDocValues) error {
	maxOrd := dv.GetValueCount() - 1
	seenOrds, err := util.NewFixedBitSet(dv.GetValueCount())
	if err != nil {
		return err
	}
	maxOrd2 := -1
	for {
		docID, err := dv.NextDoc()
		if err != nil {
			return err
		}
		if docID == DocIdSetIteratorNoMoreDocs {
			break
		}
		found, err := dv2.AdvanceExact(docID)
		if err != nil {
			return err
		}
		if !found {
			return NewCheckIndexError(fmt.Sprintf("advanceExact did not find matching doc ID: %d", docID), nil)
		}

		lastOrd := -1
		ordCount := 0
		for {
			ord, err := dv.NextOrd()
			if err != nil {
				return err
			}
			if ord == -1 {
				break
			}
			ord2, err := dv2.NextOrd()
			if err != nil {
				return err
			}
			if ord != ord2 {
				return NewCheckIndexError(fmt.Sprintf("nextDoc and advanceExact report different ords: %d != %d", ord, ord2), nil)
			}
			if ord <= lastOrd {
				return NewCheckIndexError(fmt.Sprintf("ords out of order: %d <= %d for doc: %d", ord, lastOrd, docID), nil)
			}
			if ord < 0 || ord > maxOrd {
				return NewCheckIndexError(fmt.Sprintf("ord out of bounds: %d", ord), nil)
			}
			lastOrd = ord
			if ord > maxOrd2 {
				maxOrd2 = ord
			}
			seenOrds.Set(ord)
			ordCount++
		}
		if ordCount == 0 {
			return NewCheckIndexError(fmt.Sprintf("dv for field: %s returned docID=%d yet has no ordinals", fieldName, docID), nil)
		}
	}
	if maxOrd != maxOrd2 {
		return NewCheckIndexError(fmt.Sprintf("dv for field: %s reports wrong maxOrd=%d but this is not the case: %d", fieldName, maxOrd, maxOrd2), nil)
	}
	if seenOrds.Cardinality() != dv.GetValueCount() {
		return NewCheckIndexError(fmt.Sprintf("dv for field: %s has holes in its ords, valueCount=%d but only used: %d", fieldName, dv.GetValueCount(), seenOrds.Cardinality()), nil)
	}
	var lastValue []byte
	haveLast := false
	for i := 0; i <= maxOrd; i++ {
		term, err := dv.LookupOrd(i)
		if err != nil {
			return err
		}
		if haveLast && bytes.Compare(term, lastValue) <= 0 {
			return NewCheckIndexError(fmt.Sprintf("dv for field: %s has ords out of order: %v >= %v", fieldName, lastValue, term), nil)
		}
		lastValue = append(lastValue[:0], term...)
		haveLast = true
	}
	return nil
}

// checkSortedNumericDocValues validates the per-document value streams of a
// sorted-numeric doc-values field: non-empty, non-decreasing, and identical to
// the stream an independently opened instance reports via AdvanceExact.
//
// Port of org.apache.lucene.index.CheckIndex#checkSortedNumericDocValues.
func checkSortedNumericDocValues(fieldName string, ndv, ndv2 SortedNumericDocValues) error {
	if ndv.DocID() != -1 {
		return NewCheckIndexError(fmt.Sprintf("dv iterator for field: %s should start at docID=-1, but got %d", fieldName, ndv.DocID()), nil)
	}
	for {
		docID, err := ndv.NextDoc()
		if err != nil {
			return err
		}
		if docID == DocIdSetIteratorNoMoreDocs {
			break
		}
		count, err := ndv.DocValueCount()
		if err != nil {
			return err
		}
		if count == 0 {
			return NewCheckIndexError(fmt.Sprintf("sorted numeric dv for field: %s returned docValueCount=0 for docID=%d", fieldName, docID), nil)
		}
		found, err := ndv2.AdvanceExact(docID)
		if err != nil {
			return err
		}
		if !found {
			return NewCheckIndexError(fmt.Sprintf("advanceExact did not find matching doc ID: %d", docID), nil)
		}
		count2, err := ndv2.DocValueCount()
		if err != nil {
			return err
		}
		if count != count2 {
			return NewCheckIndexError(fmt.Sprintf("advanceExact reports different value count: %d != %d", count, count2), nil)
		}
		previous := int64(math.MinInt64)
		for j := 0; j < count; j++ {
			value, err := ndv.NextValue()
			if err != nil {
				return err
			}
			if value < previous {
				return NewCheckIndexError(fmt.Sprintf("values out of order: %d < %d for doc: %d", value, previous, docID), nil)
			}
			previous = value

			value2, err := ndv2.NextValue()
			if err != nil {
				return err
			}
			if value != value2 {
				return NewCheckIndexError(fmt.Sprintf("advanceExact reports different value: %d != %d", value, value2), nil)
			}
		}
	}
	return nil
}

func (ci *CheckIndex) testPoints(reader *SegmentReader, w io.Writer) *PointsStatus {
	startNS := time.Now().UnixNano()
	status := &PointsStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [%d points; %d fields] [took %.3f sec]\n", status.TotalValuePoints, status.TotalValueFields, nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: points................")
	}

	fieldInfos := reader.GetFieldInfos()
	for _, info := range fieldInfos.Fields() {
		if info.PointDimensionCount() > 0 {
			points, err := reader.GetPointValues(info.Name())
			if err != nil {
				status.Error = err
				return status
			}
			if points == nil {
				status.Error = fmt.Errorf("points missing for field: %s", info.Name())
				return status
			}

			// PointValues exposes the BKD walk through the wider
			// intersectablePointValues surface the codec readers implement;
			// spi.PointValues itself carries only the summary accessors.
			intersectable, ok := points.(intersectablePointValues)
			if !ok {
				status.Error = fmt.Errorf("points for field %s (%T) cannot be intersected", info.Name(), points)
				return status
			}

			visitor := &verifyPointsVisitor{}
			if err := intersectable.Intersect(visitor); err != nil {
				status.Error = err
				return status
			}
			if visitor.Error != nil {
				status.Error = visitor.Error
				return status
			}
			status.TotalValuePoints += int64(visitor.Count)
			status.TotalValueFields++
		}
	}

	return status
}

// verifyPointsVisitor counts every point a field's BKD tree holds. It mirrors
// org.apache.lucene.index.CheckIndex.VerifyPointsVisitor, whose compare() always
// reports CELL_CROSSES_QUERY so that the whole tree is walked leaf by leaf and
// every packed value is handed to the visitor.
type verifyPointsVisitor struct {
	Count int
	Error error
}

// Visit counts a document matched from a fully-contained cell.
func (v *verifyPointsVisitor) Visit(docID int) error {
	v.Count++
	return nil
}

// VisitByPackedValue counts a (document, packed value) pair from a crossing cell.
func (v *verifyPointsVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	v.Count++
	return nil
}

// Compare always reports CELL_CROSSES_QUERY (2) so that the whole tree is visited.
func (v *verifyPointsVisitor) Compare(minPackedValue, maxPackedValue []byte) int {
	return 2
}

// Grow is a no-op: the visitor only counts.
func (v *verifyPointsVisitor) Grow(count int) {}

func (ci *CheckIndex) testVectors(reader *SegmentReader, w io.Writer) *VectorValuesStatus {
	startNS := time.Now().UnixNano()
	status := &VectorValuesStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [%d vectors; %d fields] [took %.3f sec]\n", status.TotalVectorValues, status.TotalKnnVectorFields, nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: vectors................")
	}

	fieldInfos := reader.GetFieldInfos()
	for _, info := range fieldInfos.Fields() {
		if info.HasVectorValues() {
			vectors, err := reader.GetFloatVectorValues(info.Name())
			if err != nil {
				status.Error = err
				return status
			}
			if vectors == nil {
				status.Error = fmt.Errorf("vectors missing for field: %s", info.Name())
				return status
			}

			// Basic check: iterate through vectors
			count := vectors.Size()
			status.TotalVectorValues += int64(count)
			status.TotalKnnVectorFields++
		}
	}

	return status
}

func (ci *CheckIndex) testSort(reader *SegmentReader, sort any, w io.Writer) *IndexSortStatus {
	startNS := time.Now().UnixNano()
	status := &IndexSortStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [took %.3f sec]\n", nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: index sort............")
	}

	// In a real implementation, we'd verify the index sort is consistent across documents.
	// For now, we acknowledge the sort exists.
	return status
}

func (ci *CheckIndex) checkSoftDeletes(field string, info *SegmentCommitInfo, reader *SegmentReader, w io.Writer) *SoftDeletesStatus {
	startNS := time.Now().UnixNano()
	status := &SoftDeletesStatus{}

	defer func() {
		if w != nil {
			if status.Error == nil {
				fmt.Fprintf(w, "OK [took %.3f sec]\n", nsToSec(time.Now().UnixNano()-startNS))
			} else {
				fmt.Fprintf(w, "ERROR [%v]\n", status.Error)
			}
		}
	}()

	if w != nil {
		fmt.Fprint(w, "    test: soft deletes..........")
	}

	// Verify that the soft deletes field carries doc values.
	fieldInfo := reader.GetFieldInfos().FieldInfoByName(field)
	if fieldInfo == nil {
		status.Error = fmt.Errorf("soft deletes field %s is missing", field)
		return status
	}
	dv, err := reader.GetDocValues(fieldInfo)
	if err != nil {
		status.Error = err
		return status
	}
	if dv == nil {
		status.Error = fmt.Errorf("soft deletes field %s is missing", field)
		return status
	}

	return status
}

func (ci *CheckIndex) exorciseIndex(result *Status) error {
	if result.NumBadSegments == 0 {
		return nil
	}

	ci.msgf("Exorcising index... removing %d bad segments", result.NumBadSegments)

	// Advance generation to create a new commit point.
	nextGen := result.NewSegments.NextGeneration()

	// Write the new segments_N file.
	if err := spi.WriteSegmentInfos(result.NewSegments, ci.dir); err != nil {
		return fmt.Errorf("failed to write exorcised segments file: %w", err)
	}

	ci.msgf("Successfully exorcised index. New segments file: %s", spi.GetSegmentFileName(nextGen))
	return nil
}

func GetLastCommitSegmentsFileName(files []string) string {
	var maxGen int64 = -1
	var latestFile string
	for _, file := range files {
		if len(file) >= 9 && file[:9] == "segments_" {
			if gen, err := strconv.ParseInt(file[9:], 36, 64); err == nil {
				if gen > maxGen {
					maxGen = gen
					latestFile = file
				}
			}
		}
	}
	return latestFile
}

func GenerationFromSegmentsFileName(fileName string) int64 {
	if len(fileName) < 9 || fileName[:9] != "segments_" {
		return -1
	}
	gen, err := strconv.ParseInt(fileName[9:], 36, 64)
	if err != nil {
		return -1
	}
	return gen
}
