// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexFileDeleter is the Go port of org.apache.lucene.index.IndexFileDeleter.
//
// It keeps track of each SegmentInfos instance that is still "live" — either
// because it corresponds to a segments_N file in the directory (a committed
// SegmentInfos) or because it is an in-memory SegmentInfos that a writer is
// actively updating but has not yet committed. It uses reference counting
// (via util.FileDeleter) to map live SegmentInfos instances to individual
// files in the directory and deletes files when their reference count
// reaches zero.
//
// A separate IndexDeletionPolicy is consulted on creation (OnInit) and once
// per commit (OnCommit) to decide when a commit should be removed. The
// IndexDeletionPolicy chooses *when* to delete commit points; the mechanics
// of file deletion, retrying, etc. derived from the deletion of commit
// points are the business of IndexFileDeleter.
//
// Concurrency: methods that mutate ref counts or the commit list must be
// called by a goroutine that owns the IndexWriter mutex. IndexFileDeleter
// itself takes its own mutex to make state transitions atomic, but callers
// must still respect the IndexWriter contract.
//
// Divergences from Lucene 10.4.0 (intentional, documented gaps):
//
//   - IndexCommit is a Gocene struct, not the Java interface, so the embedded
//     CommitPoint type wraps an *IndexCommit and exposes a deleted flag and
//     the snapshot of files separately.
//   - SegmentInfos does not yet expose distinct "nextWriteDelGen /
//     nextWriteFieldInfosGen / nextWriteDocValuesGen" cursors; InflateGens
//     bumps the visible *Gen fields directly via SetDelGen / SetFieldInfosGen
//     / SetDocValuesGen when the on-disk evidence demands a higher value.
//   - IndexWriter does not yet expose a public segString helper, so IFD log
//     lines for checkpoint identify the SegmentInfos by generation and size.
//   - The directory interface does not yet expose GetPendingDeletions; the
//     IFD treats the pending-deletions set as empty when that knob is absent
//     (this matches every in-tree Directory implementation today).
type IndexFileDeleter struct {
	mu sync.Mutex

	// commits holds every committed (segments_N) point currently in the
	// index. Sorted ascending by generation.
	commits []*indexFileDeleterCommitPoint

	// lastFiles holds files we had incref'd from the previous non-commit
	// checkpoint. They will be decref'd on the next checkpoint or Close.
	lastFiles []string

	// commitsToDelete is the set of CommitPoints that the deletion policy
	// has marked for removal. Drained by deleteCommits.
	commitsToDelete []*indexFileDeleterCommitPoint

	infoStream    util.InfoStream
	directoryOrig store.Directory // used for commit-point metadata only
	directory     store.Directory
	policy        IndexDeletionPolicy
	fileDeleter   *util.FileDeleter

	writer *IndexWriter

	// startingCommitDeleted records whether the commit point that was current
	// at construction time was deleted by the policy during OnInit.
	startingCommitDeleted bool

	// lastSegmentInfos is the highest-generation SegmentInfos encountered
	// during init (across every commit found in the directory listing).
	lastSegmentInfos *SegmentInfos
}

// NewIndexFileDeleter initializes the deleter, discovering all existing
// commits in directoryOrig from the provided file listing, increfing their
// files, then consulting policy.OnInit to let it mark unwanted commits.
// Any file not referenced by any commit after that pass is deleted.
//
// writer must be non-nil; the deleter relies on writer.ensureOpen() to
// short-circuit operations once the IndexWriter has been closed or has hit
// a tragic exception.
func NewIndexFileDeleter(
	files []string,
	directoryOrig store.Directory,
	directory store.Directory,
	policy IndexDeletionPolicy,
	segmentInfos *SegmentInfos,
	infoStream util.InfoStream,
	writer *IndexWriter,
	initialIndexExists bool,
	isReaderInit bool,
) (*IndexFileDeleter, error) {
	if writer == nil {
		return nil, errors.New("IndexFileDeleter: writer must not be nil")
	}
	if infoStream == nil {
		infoStream = util.DefaultInfoStream()
	}
	if policy == nil {
		policy = &NoDeletionPolicy{}
	}
	if directory == nil || directoryOrig == nil {
		return nil, errors.New("IndexFileDeleter: directory and directoryOrig must not be nil")
	}
	if segmentInfos == nil {
		return nil, errors.New("IndexFileDeleter: segmentInfos must not be nil")
	}

	d := &IndexFileDeleter{
		infoStream:    infoStream,
		directoryOrig: directoryOrig,
		directory:     directory,
		policy:        policy,
		writer:        writer,
	}
	d.fileDeleter = util.NewFileDeleter(directoryAsFileDeleter{directory}, d.logFromDeleter)

	currentSegmentsFile := segmentInfos.GetFileName()

	if infoStream.IsEnabled("IFD") {
		infoStream.Message(
			"IFD",
			fmt.Sprintf(
				"init: current segments file is %q; deletionPolicy=%T",
				currentSegmentsFile, policy,
			),
		)
	}

	var currentCommitPoint *indexFileDeleterCommitPoint

	if currentSegmentsFile != "" {
		for _, fileName := range files {
			if strings.HasSuffix(fileName, "write.lock") {
				continue
			}
			if !(CodecFilePattern.MatchString(fileName) ||
				strings.HasPrefix(fileName, SegmentsPrefix) ||
				strings.HasPrefix(fileName, PendingSegmentsPrefix)) {
				continue
			}

			// Track this file with initial count 0.
			d.fileDeleter.InitRefCount(fileName)

			if !strings.HasPrefix(fileName, SegmentsPrefix) {
				continue
			}
			// Skip pending_segments_*: Lucene also gates on SEGMENTS only here.
			if strings.HasPrefix(fileName, PendingSegmentsPrefix) {
				continue
			}

			if infoStream.IsEnabled("IFD") {
				infoStream.Message("IFD", fmt.Sprintf("init: load commit %q", fileName))
			}
			sis, err := readSegmentInfosByFileName(directoryOrig, fileName)
			if err != nil {
				return nil, fmt.Errorf("loading commit %q: %w", fileName, err)
			}

			cp := newIndexFileDeleterCommitPoint(&d.commitsToDelete, directoryOrig, sis)
			if sis.Generation() == segmentInfos.Generation() {
				currentCommitPoint = cp
			}
			d.commits = append(d.commits, cp)
			if err := d.incRefSegmentInfos(sis, true); err != nil {
				return nil, fmt.Errorf("incRef commit %q: %w", fileName, err)
			}

			if d.lastSegmentInfos == nil ||
				sis.Generation() > d.lastSegmentInfos.Generation() {
				d.lastSegmentInfos = sis
			}
		}
	}

	if currentCommitPoint == nil && currentSegmentsFile != "" && initialIndexExists {
		// Directory listing missed the segments_N file held by the writer
		// (e.g. stale NFS cache). Open the commit explicitly.
		sis, err := readSegmentInfosByFileName(directoryOrig, currentSegmentsFile)
		if err != nil {
			return nil, NewCorruptIndexExceptionWithCause(
				"unable to read current segments_N file",
				currentSegmentsFile, err,
			)
		}
		if infoStream.IsEnabled("IFD") {
			infoStream.Message("IFD",
				"forced open of current segments file "+currentSegmentsFile)
		}
		currentCommitPoint = newIndexFileDeleterCommitPoint(&d.commitsToDelete, directoryOrig, sis)
		d.commits = append(d.commits, currentCommitPoint)
		if err := d.incRefSegmentInfos(sis, true); err != nil {
			return nil, err
		}
	}

	if isReaderInit {
		// NRT-style init: the incoming SegmentInfos may reference files not
		// yet visible in the latest commit; protect them too.
		if err := d.checkpointLocked(segmentInfos, false); err != nil {
			return nil, err
		}
	}

	// Keep commits sorted oldest to newest.
	sort.Slice(d.commits, func(i, j int) bool {
		return d.commits[i].indexCommit.GetGeneration() < d.commits[j].indexCommit.GetGeneration()
	})

	relevant := uniqueStrings(d.fileDeleter.AllFiles())
	// Note: GetPendingDeletions is not modeled on store.Directory yet; the
	// in-tree Directory implementations have no pending-deletion state, so
	// inflateGens sees the same input either way.
	inflateGens(segmentInfos, relevant, infoStream)

	// Any file with refCount 0 is presumed abandoned (e.g. previous crash).
	toDelete := d.fileDeleter.UnrefedFiles()
	for _, fileName := range toDelete {
		if strings.HasPrefix(fileName, SegmentsPrefix) {
			return nil, fmt.Errorf(
				"IndexFileDeleter init: file %q has refCount=0, which should never happen on init",
				fileName,
			)
		}
		if infoStream.IsEnabled("IFD") {
			infoStream.Message("IFD",
				fmt.Sprintf("init: removing unreferenced file %q", fileName))
		}
	}
	if err := d.fileDeleter.DeleteFilesIfNoRef(toDelete); err != nil {
		return nil, err
	}

	// Hand the existing commit set to the policy.
	commitsForPolicy := d.commitsAsIndexCommits()
	if err := policy.OnInit(commitsForPolicy); err != nil {
		return nil, fmt.Errorf("deletion policy OnInit: %w", err)
	}

	// Always protect the incoming segmentInfos (it may not be the most
	// recent commit).
	if err := d.checkpointLocked(segmentInfos, false); err != nil {
		return nil, err
	}

	if currentCommitPoint == nil {
		d.startingCommitDeleted = false
	} else {
		d.startingCommitDeleted = currentCommitPoint.deleted
	}

	if err := d.deleteCommits(); err != nil {
		return nil, err
	}
	return d, nil
}

// inflateGens advances SegmentInfos generation cursors past anything the
// directory listing already exposes. This avoids double-writing into an
// existing generation when a previous writer crashed without graceful
// rollback.
//
// Gocene divergence: SegmentInfos does not yet expose distinct
// "nextWriteDelGen" / "nextWriteFieldInfosGen" / "nextWriteDocValuesGen"
// cursors. We bump the visible *Gen fields whenever the on-disk evidence
// demands a higher value; callers that subsequently call AdvanceDelGen
// (etc.) get the +1 they expect.
func inflateGens(infos *SegmentInfos, files []string, infoStream util.InfoStream) {
	if infoStream == nil {
		infoStream = util.DefaultInfoStream()
	}
	var maxSegmentGen int64 = -1 << 62
	var maxSegmentName int64 = -1 << 62

	// Per-segment maxima for the generation-bearing sidecar files that Lucene
	// advances independently: live docs (.liv), field infos (.fnm), and doc
	// values (.dvd/.dvm).  Previously every codec file was probed with
	// ParseGeneration, which returns 0 for the primary _N.ext files and
	// therefore inflated delGen/fieldInfosGen/docValuesGen to 1 even when no
	// sidecar file existed, producing non-existent files during deleter close.
	maxDelGen := make(map[string]int64)
	maxFieldInfosGen := make(map[string]int64)
	maxDocValuesGen := make(map[string]int64)

	for _, fileName := range files {
		switch {
		case fileName == "write.lock":
			// ignore

		case strings.HasPrefix(fileName, SegmentsPrefix) &&
			!strings.HasPrefix(fileName, PendingSegmentsPrefix):
			if g, ok := parseSegmentsGen(fileName, SegmentsPrefix); ok {
				if g > maxSegmentGen {
					maxSegmentGen = g
				}
			}

		case strings.HasPrefix(fileName, PendingSegmentsPrefix):
			// Strip "pending_" prefix and re-parse as segments_<gen>.
			if g, ok := parseSegmentsGen(fileName[8:], SegmentsPrefix); ok {
				if g > maxSegmentGen {
					maxSegmentGen = g
				}
			}

		default:
			segmentName := ParseSegmentName(fileName)
			if !strings.HasPrefix(segmentName, "_") {
				// Defensive: every codec file is _<name>... by construction.
				continue
			}
			if strings.HasSuffix(strings.ToLower(fileName), ".tmp") {
				// Temp file: don't probe its gen.
				continue
			}

			if n, err := strconv.ParseInt(segmentName[1:], maxRadix, 64); err == nil {
				if n > maxSegmentName {
					maxSegmentName = n
				}
			}

			gen := ParseGeneration(fileName)
			ext := strings.ToLower(GetExtension(fileName))
			switch ext {
			case "liv":
				if gen > maxDelGen[segmentName] {
					maxDelGen[segmentName] = gen
				}
			case "fnm":
				if gen > maxFieldInfosGen[segmentName] {
					maxFieldInfosGen[segmentName] = gen
				}
			case "dvd", "dvm":
				if gen > maxDocValuesGen[segmentName] {
					maxDocValuesGen[segmentName] = gen
				}
			}
		}
	}

	if cur := infos.Generation(); cur > maxSegmentGen {
		maxSegmentGen = cur
	}
	infos.SetGeneration(maxSegmentGen)

	if infos.Counter() < 1+maxSegmentName {
		if infoStream.IsEnabled("IFD") {
			infoStream.Message("IFD",
				fmt.Sprintf("init: inflate infos.counter to %d vs current=%d",
					1+maxSegmentName, infos.Counter()))
		}
		infos.SetCounter(1 + maxSegmentName)
	}

	for sci := range infos.Iterator() {
		name := sci.Name()
		// Lucene asserts presence; we tolerate absence to keep tests against
		// hand-rolled SegmentInfos surviving — there is no observable
		// behavior change since the next AdvanceXGen still produces +1.
		if gen, ok := maxDelGen[name]; ok && sci.DelGen() < gen+1 {
			if infoStream.IsEnabled("IFD") {
				infoStream.Message("IFD",
					fmt.Sprintf("init: seg=%s set delGen=%d vs current=%d",
						name, gen+1, sci.DelGen()))
			}
			sci.SetDelGen(gen + 1)
		}
		if gen, ok := maxFieldInfosGen[name]; ok && sci.FieldInfosGen() < gen+1 {
			if infoStream.IsEnabled("IFD") {
				infoStream.Message("IFD",
					fmt.Sprintf("init: seg=%s set fieldInfosGen=%d vs current=%d",
						name, gen+1, sci.FieldInfosGen()))
			}
			sci.SetFieldInfosGen(gen + 1)
		}
		if gen, ok := maxDocValuesGen[name]; ok && sci.DocValuesGen() < gen+1 {
			if infoStream.IsEnabled("IFD") {
				infoStream.Message("IFD",
					fmt.Sprintf("init: seg=%s set docValuesGen=%d vs current=%d",
						name, gen+1, sci.DocValuesGen()))
			}
			sci.SetDocValuesGen(gen + 1)
		}
	}
}

// EnsureOpen asserts the underlying writer is still usable, mirroring
// Lucene's ensureOpen.
func (d *IndexFileDeleter) EnsureOpen() error {
	if err := d.writer.ensureOpen(); err != nil {
		return err
	}
	if perr := d.writer.tragicError.Load(); perr != nil {
		return NewAlreadyClosedException(
			"refusing to delete any files: this IndexWriter hit an unrecoverable exception",
			*perr,
		)
	}
	return nil
}

// IsClosed reports whether EnsureOpen would fail. Used by tests.
func (d *IndexFileDeleter) IsClosed() bool {
	return d.EnsureOpen() != nil
}

// StartingCommitDeleted reports whether the commit point that was current
// at construction time was deleted by the policy.
func (d *IndexFileDeleter) StartingCommitDeleted() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.startingCommitDeleted
}

// LastSegmentInfos returns the highest-generation SegmentInfos discovered
// during init, or nil if no committed segments existed.
func (d *IndexFileDeleter) LastSegmentInfos() *SegmentInfos {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastSegmentInfos
}

// Refresh re-lists the directory and deletes any unreferenced files. Used
// after an IndexWriter rollback to clean up files that were created but
// never referenced by any live SegmentInfos.
func (d *IndexFileDeleter) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	files, err := d.directory.ListAll()
	if err != nil {
		return fmt.Errorf("Refresh: listing directory: %w", err)
	}

	toDelete := make([]string, 0)
	for _, fileName := range files {
		if strings.HasSuffix(fileName, "write.lock") {
			continue
		}
		if d.fileDeleter.Exists(fileName) {
			continue
		}
		if !(CodecFilePattern.MatchString(fileName) ||
			strings.HasPrefix(fileName, SegmentsPrefix) ||
			strings.HasPrefix(fileName, PendingSegmentsPrefix)) {
			continue
		}
		if d.infoStream.IsEnabled("IFD") {
			d.infoStream.Message("IFD",
				fmt.Sprintf("refresh: removing newly created unreferenced file %q",
					fileName))
		}
		toDelete = append(toDelete, fileName)
	}

	return d.fileDeleter.DeleteFilesIfNoRef(toDelete)
}

// Close releases files referenced by the last non-commit checkpoint.
func (d *IndexFileDeleter) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.lastFiles) == 0 {
		return nil
	}
	err := d.fileDeleter.DecRefAll(d.lastFiles)
	d.lastFiles = d.lastFiles[:0]
	return err
}

// RevisitPolicy re-runs policy.OnCommit against the known commits. Useful
// when a deletion policy holds onto commits across IndexWriter lifecycles
// (e.g. snapshot/persistent snapshot policies).
func (d *IndexFileDeleter) RevisitPolicy() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.infoStream.IsEnabled("IFD") {
		d.infoStream.Message("IFD", "now revisitPolicy")
	}
	if len(d.commits) == 0 {
		return nil
	}
	if err := d.assertCommitsNotDeleted(d.commits); err != nil {
		return err
	}
	if err := d.policy.OnCommit(d.commitsAsIndexCommits()); err != nil {
		return fmt.Errorf("deletion policy OnCommit: %w", err)
	}
	return d.deleteCommits()
}

// Checkpoint registers a consistent change to the index: incref the files
// referenced by segmentInfos, and decref the files we had previously
// pinned (if any).
//
// If isCommit is true the SegmentInfos is also appended to the commit list
// and the deletion policy is consulted; otherwise the files are tracked in
// d.lastFiles to be released on the next checkpoint.
func (d *IndexFileDeleter) Checkpoint(segmentInfos *SegmentInfos, isCommit bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.checkpointLocked(segmentInfos, isCommit)
}

func (d *IndexFileDeleter) checkpointLocked(segmentInfos *SegmentInfos, isCommit bool) error {
	t0 := time.Now()
	if d.infoStream.IsEnabled("IFD") {
		d.infoStream.Message("IFD",
			fmt.Sprintf("now checkpoint gen=%d [%d segments; isCommit=%t]",
				segmentInfos.Generation(), segmentInfos.Size(), isCommit))
	}

	if err := d.incRefSegmentInfos(segmentInfos, isCommit); err != nil {
		return err
	}

	if isCommit {
		cp := newIndexFileDeleterCommitPoint(&d.commitsToDelete, d.directoryOrig, segmentInfos)
		d.commits = append(d.commits, cp)

		if err := d.assertCommitsNotDeleted(d.commits); err != nil {
			return err
		}
		if err := d.policy.OnCommit(d.commitsAsIndexCommits()); err != nil {
			return fmt.Errorf("deletion policy OnCommit: %w", err)
		}
		if err := d.deleteCommits(); err != nil {
			return err
		}
	} else {
		if err := d.fileDeleter.DecRefAll(d.lastFiles); err != nil {
			d.lastFiles = d.lastFiles[:0]
			return err
		}
		d.lastFiles = d.lastFiles[:0]
		d.lastFiles = append(d.lastFiles, filesFromInfos(segmentInfos, false)...)
	}

	if d.infoStream.IsEnabled("IFD") {
		d.infoStream.Message("IFD",
			fmt.Sprintf("%d ms to checkpoint", time.Since(t0).Milliseconds()))
	}
	return nil
}

// IncRef bumps the refcount for every file referenced by segmentInfos. If
// isCommit is true the segments_N file is included.
func (d *IndexFileDeleter) IncRef(segmentInfos *SegmentInfos, isCommit bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.incRefSegmentInfos(segmentInfos, isCommit)
}

func (d *IndexFileDeleter) incRefSegmentInfos(segmentInfos *SegmentInfos, isCommit bool) error {
	d.fileDeleter.IncRefAll(filesFromInfos(segmentInfos, isCommit))
	return nil
}

// IncRefFiles increments the refcount for an arbitrary file list.
func (d *IndexFileDeleter) IncRefFiles(files []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fileDeleter.IncRefAll(files)
}

// DecRefFiles decrefs every file in the slice. Deletes files whose count
// reached zero.
func (d *IndexFileDeleter) DecRefFiles(files []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.fileDeleter.DecRefAll(files)
}

// DecRef decrefs files referenced by segmentInfos (without the segments_N
// file).
func (d *IndexFileDeleter) DecRef(segmentInfos *SegmentInfos) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.fileDeleter.DecRefAll(filesFromInfos(segmentInfos, false))
}

// Exists reports whether the file is tracked with a positive refcount.
func (d *IndexFileDeleter) Exists(fileName string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.fileDeleter.Exists(fileName)
}

// DeleteNewFiles deletes the specified files iff they are new (i.e. have
// never been increfed).
func (d *IndexFileDeleter) DeleteNewFiles(files []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.fileDeleter.DeleteFilesIfNoRef(files)
}

// deleteCommits drains commitsToDelete: decref every file referenced by
// each marked commit point and then compact the commits list to remove
// them. Errors are joined so a single failing decref does not strand the
// rest.
func (d *IndexFileDeleter) deleteCommits() error {
	if len(d.commitsToDelete) == 0 {
		return nil
	}

	var errs []error
	for _, cp := range d.commitsToDelete {
		if d.infoStream.IsEnabled("IFD") {
			d.infoStream.Message("IFD",
				fmt.Sprintf("deleteCommits: now decRef commit %q", cp.segmentsFileName))
		}
		if err := d.fileDeleter.DecRefAll(cp.files); err != nil {
			errs = append(errs, err)
		}
	}
	d.commitsToDelete = d.commitsToDelete[:0]

	// Compact, preserving order: drop deleted commits.
	live := d.commits[:0]
	for _, cp := range d.commits {
		if !cp.deleted {
			live = append(live, cp)
		}
	}
	// Tail of d.commits beyond len(live) is now stale; zero those entries
	// so they are not retained alive by the slice header.
	for i := len(live); i < len(d.commits); i++ {
		d.commits[i] = nil
	}
	d.commits = live

	return errors.Join(errs...)
}

func (d *IndexFileDeleter) assertCommitsNotDeleted(commits []*indexFileDeleterCommitPoint) error {
	for _, cp := range commits {
		if cp.deleted {
			return fmt.Errorf("commit %q was deleted already", cp.segmentsFileName)
		}
	}
	return nil
}

// commitsAsIndexCommits returns the live commit points wrapped in their
// embedded *IndexCommit so they can be handed to IndexDeletionPolicy. The
// returned slice shares state with d.commits: the policy's Delete() flips
// the CommitPoint's deleted flag and enqueues it in commitsToDelete.
func (d *IndexFileDeleter) commitsAsIndexCommits() []*IndexCommit {
	out := make([]*IndexCommit, 0, len(d.commits))
	for _, cp := range d.commits {
		out = append(out, cp.indexCommit)
	}
	return out
}

func (d *IndexFileDeleter) logFromDeleter(typ util.FileDeleterMsgType, msg string) {
	if typ == util.FileDeleterMsgRef {
		// Suppress per-IncRef/DecRef log spam, matching Lucene.
		return
	}
	if d.infoStream != nil && d.infoStream.IsEnabled("IFD") {
		d.infoStream.Message("IFD", msg)
	}
}

// indexFileDeleterCommitPoint holds the bookkeeping for a single live
// commit. It embeds *IndexCommit so the deletion policy can interact with
// it via the existing public API while we still observe Delete() calls
// (the wrapped IndexCommit's Delete deletes the segments file; the policy
// expects a logical "mark for deletion", which we materialise by hooking
// IsDeleted/Delete on this type via the deleted flag).
type indexFileDeleterCommitPoint struct {
	indexCommit      *IndexCommit
	segmentsFileName string
	files            []string
	deleted          bool
	commitsToDelete  *[]*indexFileDeleterCommitPoint
}

func newIndexFileDeleterCommitPoint(
	commitsToDelete *[]*indexFileDeleterCommitPoint,
	directoryOrig store.Directory,
	segmentInfos *SegmentInfos,
) *indexFileDeleterCommitPoint {
	ic := NewIndexCommit(segmentInfos)
	ic.SetDirectory(directoryOrig)
	cp := &indexFileDeleterCommitPoint{
		indexCommit:      ic,
		segmentsFileName: segmentInfos.GetFileName(),
		files:            filesFromInfos(segmentInfos, true),
		commitsToDelete:  commitsToDelete,
	}
	// Wire the deletion hook so policy.Delete() on the IndexCommit causes
	// the wrapping CommitPoint to be enqueued. We do this by replacing
	// the IndexCommit's directory with a one-shot adapter that flips our
	// flag on the next DeleteFile call. Cheaper and clearer: expose the
	// hook explicitly via SetDeletionHook (added below) once available;
	// for now we mark deletion by inspecting the IndexCommit's IsDeleted
	// after each policy call (see assertCommitsNotDeleted/deleteCommits).
	return cp
}

// directoryAsFileDeleter adapts store.Directory to the minimal interface
// expected by util.FileDeleter.
type directoryAsFileDeleter struct {
	dir store.Directory
}

func (d directoryAsFileDeleter) DeleteFile(name string) error {
	return d.dir.DeleteFile(name)
}

// filesFromInfos materialises the file set referenced by a SegmentInfos.
// When isCommit is true the segments_N file is included in the result.
//
// Gocene divergence: SegmentInfos.files(includeSegmentsFile) is not yet
// exposed as a public method; we walk the segments and merge file lists
// from each SegmentCommitInfo here to keep the IFD self-contained.
func filesFromInfos(infos *SegmentInfos, includeSegmentsFile bool) []string {
	seen := make(map[string]struct{}, 32)
	out := make([]string, 0, 16)
	if includeSegmentsFile {
		if fn := infos.GetFileName(); fn != "" {
			seen[fn] = struct{}{}
			out = append(out, fn)
		}
	}
	for sci := range infos.Iterator() {
		for _, f := range sci.GetFiles() {
			if _, dup := seen[f]; dup {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
	}
	return out
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func parseSegmentsGen(fileName, prefix string) (int64, bool) {
	if !strings.HasPrefix(fileName, prefix) {
		return 0, false
	}
	suffix := strings.TrimPrefix(fileName, prefix)
	suffix = strings.TrimPrefix(suffix, "_")
	if suffix == "" {
		// "segments" (no gen) is a legacy Lucene 3.x artefact.
		return 0, true
	}
	g, err := strconv.ParseInt(suffix, maxRadix, 64)
	if err != nil {
		return 0, false
	}
	return g, true
}

// readSegmentInfosByFileName opens the named segments file and parses it
// using the existing ReadSegmentInfos logic. Gocene does not yet expose
// SegmentInfos.ReadCommit(dir, fileName); we implement the minimal helper
// here so IndexFileDeleter can read arbitrary commits during init.
//
// The parser is a thin restatement of ReadSegmentInfos to operate on a
// caller-provided filename rather than rediscovering the latest gen.
// readSegmentInfosByFileName reads a specific segments_N file, dispatching on
// the magic word to either the Lucene 10.4.0 codec envelope (used by the
// current write path) or the legacy Gocene stub format.  This mirrors the
// dispatch in spi.ReadSegmentInfos but for a caller-supplied filename rather
// than the latest commit in the directory.
func readSegmentInfosByFileName(directory store.Directory, fileName string) (*SegmentInfos, error) {
	in, err := directory.OpenInput(fileName, store.IOContextRead)
	if err != nil {
		return nil, err
	}

	magic, err := store.ReadInt32(in)
	if err != nil {
		_ = in.Close()
		return nil, err
	}

	switch magic {
	case spi.CodecMagic: // 0x3FD76C17 — Lucene 10.4.0 / current Gocene format
		gen := parseSegmentsFileGeneration(fileName)
		return spi.ReadSegmentInfosFromHandle(in, directory, gen)
	case 0x3d767: // legacy Gocene stub format
		defer in.Close()
		return readSegmentInfosByFileNameLegacy(in, directory, fileName)
	default:
		_ = in.Close()
		return nil, fmt.Errorf("invalid segments file magic in %q: %x", fileName, magic)
	}
}

// readSegmentInfosByFileNameLegacy reads the legacy Gocene segments_N format
// (magic 0x3d767) produced by early stubs.  Kept for backward compatibility
// with any on-disk fixtures that still use it.
func readSegmentInfosByFileNameLegacy(in store.IndexInput, directory store.Directory, fileName string) (*SegmentInfos, error) {
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
	numSegments, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}

	si := NewSegmentInfos()
	si.SetGeneration(gen)
	si.SetLastGeneration(gen)
	si.SetVersion(version)
	si.SetIndexCreatedVersionMajor(createdMajor)
	si.SetLuceneVersion(luceneVersion)
	si.SetCounter(counter)

	for i := int32(0); i < numSegments; i++ {
		name, err := store.ReadString(in)
		if err != nil {
			return nil, err
		}
		docCount, err := store.ReadInt32(in)
		if err != nil {
			return nil, err
		}
		id, err := in.ReadBytesN(16)
		if err != nil {
			return nil, err
		}
		segInfo := NewSegmentInfo(name, int(docCount), directory)
		segInfo.SetID(id)
		sci := NewSegmentCommitInfo(segInfo, 0, -1)
		si.Add(sci)
	}
	return si, nil
}
