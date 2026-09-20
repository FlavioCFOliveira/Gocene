package index

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Throttling controls hard disk throttling simulation.
type Throttling int

const (
	ThrottlingNever Throttling = iota
	ThrottlingSometimes
	ThrottlingAlways
)

// Failure is an object that can trigger deterministic exceptions during directory operations.
type Failure struct {
	Eval func(dir *MockDirectoryWrapper) error
}

// MockDirectoryWrapper is a Directory wrapper that adds methods for unit testing and fault injection.
type MockDirectoryWrapper struct {
	in store.Directory

	mu sync.Mutex

	maxSize           int64
	maxUsedSize       int64
	randomIOERate     float64
	randomIOERateOpen float64
	randomState       *rand.Rand

	assertNoDeleteOpenFile             bool
	trackDiskUsage                     bool
	useSlowOpenClosers                 bool
	allowRandomFileNotFoundException   bool
	allowReadingFilesStillOpenForWrite bool

	unSyncedFiles     map[string]struct{}
	createdFiles      map[string]struct{}
	openFilesForWrite map[string]struct{}
	openLocks         sync.Map // map[string]error
	crashed           bool
	throttling        Throttling

	// isOpen mirrors org.apache.lucene.tests.store.BaseDirectoryWrapper.isOpen:
	// it tracks whether Close has already run on the wrapper itself, so that a
	// double close is forwarded to the delegate instead of being masked.
	isOpen bool

	alwaysCorrupt    bool
	inputCloneCount  atomic.Int32
	openFileHandles  map[io.Closer]error
	openFilesDeleted map[string]struct{}
	failures         []Failure
}

func NewMockDirectoryWrapper(r *rand.Rand, delegate store.Directory) *MockDirectoryWrapper {
	return &MockDirectoryWrapper{
		in:                               delegate,
		randomState:                      rand.New(rand.NewSource(r.Int63())),
		unSyncedFiles:                    make(map[string]struct{}),
		createdFiles:                     make(map[string]struct{}),
		openFilesForWrite:                make(map[string]struct{}),
		openFileHandles:                  make(map[io.Closer]error),
		openFilesDeleted:                 make(map[string]struct{}),
		throttling:                       ThrottlingNever,
		allowRandomFileNotFoundException: true,
		isOpen:                           true,
	}
}

func (m *MockDirectoryWrapper) GetInputCloneCount() int32 {
	return m.inputCloneCount.Load()
}

func (m *MockDirectoryWrapper) SetTrackDiskUsage(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trackDiskUsage = v
}

func (m *MockDirectoryWrapper) SetAllowRandomFileNotFoundException(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowRandomFileNotFoundException = v
}

func (m *MockDirectoryWrapper) SetAllowReadingFilesStillOpenForWrite(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowReadingFilesStillOpenForWrite = v
}

func (m *MockDirectoryWrapper) SetThrottling(t Throttling) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.throttling = t
}

func (m *MockDirectoryWrapper) SetUseSlowOpenClosers(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.useSlowOpenClosers = v
}

func (m *MockDirectoryWrapper) Sync(names []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maybeYield()
	if err := m.maybeThrowDeterministicException(); err != nil {
		return err
	}
	if m.crashed {
		return fmt.Errorf("cannot sync after crash")
	}

	for _, name := range names {
		if err := m.maybeThrowIOException(name); err != nil {
			return err
		}
		delete(m.unSyncedFiles, name)
	}
	return nil
}

func (m *MockDirectoryWrapper) Rename(source, dest string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maybeYield()
	if err := m.maybeThrowDeterministicException(); err != nil {
		return err
	}
	if m.crashed {
		return fmt.Errorf("cannot rename after crash")
	}

	if m.assertNoDeleteOpenFile && m.isOpenFile(source) {
		return fmt.Errorf("MockDirectoryWrapper: source file %q is still open: cannot rename", source)
	}
	if m.assertNoDeleteOpenFile && m.isOpenFile(dest) {
		return fmt.Errorf("MockDirectoryWrapper: dest file %q is still open: cannot rename", dest)
	}

	err := m.in.Rename(source, dest)
	if err == nil {
		if _, ok := m.unSyncedFiles[source]; ok {
			delete(m.unSyncedFiles, source)
			m.unSyncedFiles[dest] = struct{}{}
		}
		delete(m.openFilesDeleted, source)
		delete(m.createdFiles, source)
		m.createdFiles[dest] = struct{}{}
	}
	return err
}

func (m *MockDirectoryWrapper) SyncMetaData() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maybeYield()
	if err := m.maybeThrowDeterministicException(); err != nil {
		return err
	}
	if m.crashed {
		return fmt.Errorf("cannot sync metadata after crash")
	}
	return nil
}

func (m *MockDirectoryWrapper) SizeInBytes() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sizeInBytes()
}

// sizeInBytes is the body of SizeInBytes with the wrapper lock already held.
// Lucene declares every MockDirectoryWrapper method synchronized and Java
// monitors are reentrant, so sizeInBytes() can be called from inside another
// synchronized method; a Go sync.Mutex is not reentrant, so the lock-free core
// is factored out here and called by the holders of m.mu.
func (m *MockDirectoryWrapper) sizeInBytes() (int64, error) {
	var size int64
	files, err := m.in.ListAll()
	if err != nil {
		return 0, err
	}
	for _, file := range files {
		if strings.HasPrefix(file, "extra") {
			continue
		}
		l, err := m.in.FileLength(file)
		if err != nil {
			return 0, err
		}
		size += l
	}
	return size, nil
}

// CorruptUnknownFiles corrupts every file in the directory that no commit
// point references, mirroring
// org.apache.lucene.tests.store.MockDirectoryWrapper.corruptUnknownFiles().
func (m *MockDirectoryWrapper) CorruptUnknownFiles() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	files, err := m.in.ListAll()
	if err != nil {
		return err
	}

	// Gather every file referenced by a commit point.
	knownFiles := make(map[string]struct{})
	for _, fileName := range files {
		if !strings.HasPrefix(fileName, SegmentsPrefix) {
			continue
		}
		// Read through the delegate: Lucene reads through "this", but a Go
		// sync.Mutex is not reentrant and m.mu is already held here.
		infos, err := spi.ReadCommit(m.in, fileName)
		if err != nil {
			return err
		}
		for _, f := range infos.Files(true) {
			knownFiles[f] = struct{}{}
		}
	}

	toCorrupt := make([]string, 0, len(files))
	for _, fileName := range files {
		if _, known := knownFiles[fileName]; known {
			continue
		}
		if strings.HasSuffix(fileName, "write.lock") {
			continue
		}
		if CodecFilePattern.MatchString(fileName) || strings.HasPrefix(fileName, PendingSegmentsPrefix) {
			toCorrupt = append(toCorrupt, fileName)
		}
	}
	return m.corruptFilesInternal(toCorrupt)
}

func (m *MockDirectoryWrapper) CorruptFiles(files []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.corruptFilesInternal(files)
}

// corruptFilesInternal applies one of six kinds of damage to each of the given
// files, mirroring org.apache.lucene.tests.store.MockDirectoryWrapper._corruptFiles.
// The caller must hold m.mu.
func (m *MockDirectoryWrapper) corruptFilesInternal(files []string) error {
	// Must make a copy because the incoming collection changes as temp files
	// are created and files are deleted below. Sort so that the damage is
	// reproducible regardless of the iteration order of the caller's set.
	filesToCorrupt := make([]string, len(files))
	copy(filesToCorrupt, files)
	sort.Strings(filesToCorrupt)

	for _, name := range filesToCorrupt {
		damage := m.randomState.Intn(6)
		if m.alwaysCorrupt && damage == 3 {
			damage = 4
		}

		switch damage {
		case 0: // deleted
			if err := m.in.DeleteFile(name); err != nil {
				return err
			}

		case 1: // zeroed
			if err := m.zeroFile(name); err != nil {
				return err
			}

		case 2: // partially truncated
			tempName, err := m.copyHalfToTemp(name)
			if err != nil {
				return err
			}
			if err := m.copyWholeFileBack(name, tempName); err != nil {
				return err
			}
			if err := m.in.DeleteFile(tempName); err != nil {
				return err
			}

		case 3: // the file survived intact

		case 4: // one bit flipped
			tempName, err := m.copyWithFlippedBitToTemp(name)
			if err != nil {
				return err
			}
			if err := m.copyWholeFileBack(name, tempName); err != nil {
				return err
			}
			if err := m.in.DeleteFile(tempName); err != nil {
				return err
			}

		case 5: // fully truncated
			if err := m.truncateFile(name); err != nil {
				return err
			}
		}
	}
	return nil
}

// zeroFile rewrites name with the same number of zero bytes.
func (m *MockDirectoryWrapper) zeroFile(name string) (err error) {
	length, err := m.in.FileLength(name)
	if err != nil {
		return err
	}
	if err := m.in.DeleteFile(name); err != nil {
		return err
	}

	out, err := m.in.CreateOutput(name, store.NewIOContext())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	zeroes := make([]byte, 256)
	var upto int64
	for upto < length {
		limit := int(length - upto)
		if limit > len(zeroes) {
			limit = len(zeroes)
		}
		if err := out.WriteBytes(zeroes, 0, limit); err != nil {
			return err
		}
		upto += int64(limit)
	}
	return nil
}

// truncateFile replaces name with a zero-length file.
func (m *MockDirectoryWrapper) truncateFile(name string) (err error) {
	if err := m.in.DeleteFile(name); err != nil {
		return err
	}
	out, err := m.in.CreateOutput(name, store.NewIOContext())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	// Mirrors Lucene's "just fake access to prevent compiler warning".
	_ = out.GetFilePointer()
	return nil
}

// copyHalfToTemp copies the first half of name into a fresh temporary file and
// returns that file's name.
func (m *MockDirectoryWrapper) copyHalfToTemp(name string) (tempName string, err error) {
	tempOut, err := m.delegateCreateTempOutput("name", "mdw_corrupt", store.NewIOContext())
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := tempOut.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	tempName = tempOut.GetName()

	ii, err := m.in.OpenInput(name, store.NewIOContext())
	if err != nil {
		return tempName, err
	}
	defer func() {
		if cerr := ii.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return tempName, tempOut.CopyBytes(ii, ii.Length()/2)
}

// copyWithFlippedBitToTemp copies name into a fresh temporary file with exactly
// one randomly chosen bit flipped, and returns that file's name.
func (m *MockDirectoryWrapper) copyWithFlippedBitToTemp(name string) (tempName string, err error) {
	tempOut, err := m.delegateCreateTempOutput("name", "mdw_corrupt", store.NewIOContext())
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := tempOut.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	tempName = tempOut.GetName()

	ii, err := m.in.OpenInput(name, store.NewIOContext())
	if err != nil {
		return tempName, err
	}
	defer func() {
		if cerr := ii.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	length := ii.Length()
	if length == 0 {
		// The file survived intact.
		return tempName, nil
	}

	// Copy the first part unchanged.
	byteToCorrupt := int64(m.randomState.Float64() * float64(length))
	if byteToCorrupt > 0 {
		if err := tempOut.CopyBytes(ii, byteToCorrupt); err != nil {
			return tempName, err
		}
	}

	// Randomly flip one bit of this byte.
	b, err := ii.ReadByte()
	if err != nil {
		return tempName, err
	}
	bitToFlip := uint(m.randomState.Intn(8))
	b ^= 1 << bitToFlip
	if err := tempOut.WriteByte(b); err != nil {
		return tempName, err
	}

	// Copy the last part unchanged.
	bytesLeft := length - byteToCorrupt - 1
	if bytesLeft > 0 {
		if err := tempOut.CopyBytes(ii, bytesLeft); err != nil {
			return tempName, err
		}
	}
	return tempName, nil
}

// copyWholeFileBack deletes name and rewrites it with the full contents of
// tempName.
func (m *MockDirectoryWrapper) copyWholeFileBack(name, tempName string) (err error) {
	if err := m.in.DeleteFile(name); err != nil {
		return err
	}

	out, err := m.in.CreateOutput(name, store.NewIOContext())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	ii, err := m.in.OpenInput(tempName, store.NewIOContext())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := ii.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return out.CopyBytes(ii, ii.Length())
}

// delegateCreateTempOutput creates a temporary file on the wrapped directory.
func (m *MockDirectoryWrapper) delegateCreateTempOutput(prefix, suffix string, ctx store.IOContext) (store.IndexOutput, error) {
	creator, ok := m.in.(tempOutputCreator)
	if !ok {
		return nil, fmt.Errorf("MockDirectoryWrapper: delegate directory %T does not support CreateTempOutput", m.in)
	}
	return creator.CreateTempOutput(prefix, suffix, ctx)
}

func (m *MockDirectoryWrapper) Crash() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.openFilesForWrite = make(map[string]struct{})
	m.openFilesDeleted = make(map[string]struct{})

	for h := range m.openFileHandles {
		h.Close()
	}

	if err := m.corruptFilesInternal(m.getUnsyncedFiles()); err != nil {
		return err
	}
	m.crashed = true
	m.unSyncedFiles = make(map[string]struct{})
	return nil
}

func (m *MockDirectoryWrapper) ClearCrash() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.crashed = false
	m.openLocks.Range(func(k, v interface{}) bool {
		m.openLocks.Delete(k)
		return true
	})
}

func (m *MockDirectoryWrapper) SetMaxSizeInBytes(s int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxSize = s
}

func (m *MockDirectoryWrapper) GetMaxSizeInBytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxSize
}

func (m *MockDirectoryWrapper) GetMaxUsedSizeInBytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxUsedSize
}

func (m *MockDirectoryWrapper) ResetMaxUsedSizeInBytes() error {
	size, err := m.SizeInBytes()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.maxUsedSize = size
	m.mu.Unlock()
	return nil
}

func (m *MockDirectoryWrapper) SetAssertNoDeleteOpenFile(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assertNoDeleteOpenFile = v
}

func (m *MockDirectoryWrapper) GetAssertNoDeleteOpenFile() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.assertNoDeleteOpenFile
}

func (m *MockDirectoryWrapper) SetRandomIOExceptionRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.randomIOERate = rate
}

func (m *MockDirectoryWrapper) GetRandomIOExceptionRate() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.randomIOERate
}

func (m *MockDirectoryWrapper) SetRandomIOExceptionRateOnOpen(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.randomIOERateOpen = rate
}

func (m *MockDirectoryWrapper) GetRandomIOExceptionRateOnOpen() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.randomIOERateOpen
}

func (m *MockDirectoryWrapper) maybeThrowIOException(name string) error {
	if m.randomState.Float64() < m.randomIOERate {
		return fmt.Errorf("a random IOException (%s)", name)
	}
	return nil
}

func (m *MockDirectoryWrapper) maybeThrowIOExceptionOnOpen(name string) error {
	if m.randomState.Float64() < m.randomIOERateOpen {
		if !m.allowRandomFileNotFoundException || m.randomState.Intn(2) == 0 {
			return fmt.Errorf("a random IOException (%s)", name)
		}
		return store.ErrFileNotFound
	}
	return nil
}

func (m *MockDirectoryWrapper) GetFileHandleCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.openFileHandles)
}

func (m *MockDirectoryWrapper) DeleteFile(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maybeYield()
	if err := m.maybeThrowDeterministicException(); err != nil {
		return err
	}
	if m.crashed {
		return fmt.Errorf("cannot delete after crash")
	}

	if m.isOpenFile(name) {
		m.openFilesDeleted[name] = struct{}{}
		if m.assertNoDeleteOpenFile {
			return fmt.Errorf("MockDirectoryWrapper: file %q is still open: cannot delete", name)
		}
	} else {
		delete(m.openFilesDeleted, name)
	}

	delete(m.unSyncedFiles, name)
	err := m.in.DeleteFile(name)
	delete(m.createdFiles, name)
	return err
}

func (m *MockDirectoryWrapper) removeIndexOutput(out io.Closer, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.openFilesForWrite, name)
	m.removeOpenFile(out, name)
}

func (m *MockDirectoryWrapper) removeIndexInput(in io.Closer, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeOpenFile(in, name)
}

func (m *MockDirectoryWrapper) removeOpenFile(c io.Closer, name string) {
	delete(m.openFileHandles, c)
}

func (m *MockDirectoryWrapper) maybeYield() {
}

func (m *MockDirectoryWrapper) GetOpenDeletedFiles() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]string, 0, len(m.openFilesDeleted))
	for k := range m.openFilesDeleted {
		res = append(res, k)
	}
	return res
}

func (m *MockDirectoryWrapper) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.maybeThrowDeterministicException(); err != nil {
		return nil, err
	}
	if err := m.maybeThrowIOExceptionOnOpen(name); err != nil {
		return nil, err
	}
	m.maybeYield()

	if m.crashed {
		return nil, fmt.Errorf("cannot createOutput after crash")
	}

	if _, ok := m.createdFiles[name]; ok {
		return nil, store.ErrFileAlreadyExists
	}

	if m.assertNoDeleteOpenFile && m.isOpenFile(name) {
		return nil, fmt.Errorf("MockDirectoryWrapper: file %q is still open: cannot overwrite", name)
	}

	m.unSyncedFiles[name] = struct{}{}
	m.createdFiles[name] = struct{}{}

	delegateOutput, err := m.in.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}

	out := newMockIndexOutputWrapper(m, delegateOutput, name)
	m.addFileHandle(out, name)
	m.openFilesForWrite[name] = struct{}{}

	return m.maybeThrottle(name, out), nil
}

func (m *MockDirectoryWrapper) CreateTempOutput(prefix, suffix string, ctx store.IOContext) (store.IndexOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.maybeThrowDeterministicException(); err != nil {
		return nil, err
	}
	if err := m.maybeThrowIOExceptionOnOpen("temp: prefix=" + prefix + " suffix=" + suffix); err != nil {
		return nil, err
	}
	m.maybeYield()

	if m.crashed {
		return nil, fmt.Errorf("cannot createTempOutput after crash")
	}

	delegateOutput, err := m.delegateCreateTempOutput(prefix, suffix, ctx)
	if err != nil {
		return nil, err
	}
	name := delegateOutput.GetName()

	m.unSyncedFiles[name] = struct{}{}
	m.createdFiles[name] = struct{}{}

	out := newMockIndexOutputWrapper(m, delegateOutput, name)
	m.addFileHandle(out, name)
	m.openFilesForWrite[name] = struct{}{}

	return m.maybeThrottle(name, out), nil
}

func (m *MockDirectoryWrapper) maybeThrottle(name string, output store.IndexOutput) store.IndexOutput {
	if m.throttling == ThrottlingAlways || (m.throttling == ThrottlingSometimes && m.randomState.Intn(200) == 0) {
		return output
	}
	return output
}

func (m *MockDirectoryWrapper) addFileHandle(c io.Closer, name string) {
	m.openFileHandles[c] = fmt.Errorf("unclosed IndexHandle: %s", name)
}

func (m *MockDirectoryWrapper) OpenInput(name string, ctx store.IOContext) (store.IndexInput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.maybeThrowDeterministicException(); err != nil {
		return nil, err
	}
	if err := m.maybeThrowIOExceptionOnOpen(name); err != nil {
		return nil, err
	}
	m.maybeYield()

	if !m.in.FileExists(name) {
		if m.randomState.Intn(2) == 0 {
			return nil, store.ErrFileNotFound
		}
		return nil, fmt.Errorf("no such file: %s", name)
	}

	if !m.allowReadingFilesStillOpenForWrite {
		if _, ok := m.openFilesForWrite[name]; ok {
			return nil, fmt.Errorf("MockDirectoryWrapper: file %q is still open for writing", name)
		}
	}

	delegateInput, err := m.in.OpenInput(name, ctx)
	if err != nil {
		return nil, err
	}

	ii := newMockIndexInputWrapper(m, name, delegateInput, nil)
	m.addFileHandle(ii, name)
	return ii, nil
}

// IsOpen reports whether this wrapper has not been closed yet.
// Mirrors org.apache.lucene.tests.store.BaseDirectoryWrapper.isOpen().
func (m *MockDirectoryWrapper) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isOpen
}

func (m *MockDirectoryWrapper) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isOpen {
		// Already closed: close the wrapped directory again rather than
		// masking a double-close bug (MockDirectoryWrapper.close()).
		return m.in.Close()
	}
	m.isOpen = false

	if len(m.openFileHandles) > 0 {
		return fmt.Errorf("MockDirectoryWrapper: cannot close: there are still %d open files", len(m.openFileHandles))
	}

	return m.in.Close()
}

func (m *MockDirectoryWrapper) ListAll() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maybeYield()
	return m.in.ListAll()
}

func (m *MockDirectoryWrapper) FileExists(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.in.FileExists(name)
}

func (m *MockDirectoryWrapper) FileLength(name string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maybeYield()
	return m.in.FileLength(name)
}

func (m *MockDirectoryWrapper) ObtainLock(name string) (store.Lock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maybeYield()
	return m.in.ObtainLock(name)
}

func (m *MockDirectoryWrapper) GetDirectory() store.Directory {
	return m
}

func (m *MockDirectoryWrapper) isOpenFile(name string) bool {
	for handle := range m.openFileHandles {
		if wrapper, ok := handle.(*mockIndexInputWrapper); ok && wrapper.name == name {
			return true
		}
		if wrapper, ok := handle.(*mockIndexOutputWrapper); ok && wrapper.name == name {
			return true
		}
	}
	return false
}

func (m *MockDirectoryWrapper) getUnsyncedFiles() []string {
	res := make([]string, 0, len(m.unSyncedFiles))
	for k := range m.unSyncedFiles {
		res = append(res, k)
	}
	return res
}

func (m *MockDirectoryWrapper) FailOn(f Failure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures = append(m.failures, f)
}

func (m *MockDirectoryWrapper) maybeThrowDeterministicException() error {
	for _, f := range m.failures {
		if err := f.Eval(m); err != nil {
			return err
		}
	}
	return nil
}

// --- Wrappers ---

// mockIndexOutputWrapper is an IndexOutput that fails on a simulated full
// disk, tracks the maximum disk space actually used, and may raise random
// I/O errors.
//
// Port of org.apache.lucene.tests.store.MockIndexOutputWrapper.
type mockIndexOutputWrapper struct {
	*store.FilterIndexOutput

	dir  *MockDirectoryWrapper
	out  store.IndexOutput
	name string

	first  bool
	closed bool

	singleByte []byte
}

// newMockIndexOutputWrapper wraps out for the given directory.
func newMockIndexOutputWrapper(dir *MockDirectoryWrapper, out store.IndexOutput, name string) *mockIndexOutputWrapper {
	return &mockIndexOutputWrapper{
		FilterIndexOutput: store.NewFilterIndexOutput("MockIndexOutputWrapper("+out.GetName()+")", out.GetName(), out),
		dir:               dir,
		out:               out,
		name:              name,
		first:             true,
		singleByte:        make([]byte, 1),
	}
}

// ensureOpen reports an error once this output has been closed.
func (w *mockIndexOutputWrapper) ensureOpen() error {
	if w.closed {
		return store.NewAlreadyClosedException("Already closed: "+w.name, nil)
	}
	return nil
}

// checkCrashed reports an error if the directory crashed since this output was
// opened, in which case nothing may be written. The caller must hold w.dir.mu.
func (w *mockIndexOutputWrapper) checkCrashed() error {
	if w.dir.crashed {
		return fmt.Errorf("MockDirectoryWrapper has crashed; cannot write to %s", w.name)
	}
	return nil
}

// checkDiskFull enforces the simulated disk-size limit and tracks the maximum
// used size. Exactly one of b and in carries the pending payload. The caller
// must hold w.dir.mu.
func (w *mockIndexOutputWrapper) checkDiskFull(b []byte, offset int, in store.DataInput, length int64) error {
	if w.dir.maxSize == 0 {
		return nil
	}

	size, err := w.dir.sizeInBytes()
	if err != nil {
		return err
	}
	freeSpace := w.dir.maxSize - size
	var realUsage int64

	// Enforce disk full: compute the real disk free. This greatly slows the
	// test down but makes it accurate.
	if freeSpace <= length {
		realUsage, err = w.dir.sizeInBytes()
		if err != nil {
			return err
		}
		freeSpace = w.dir.maxSize - realUsage
	}

	if freeSpace > length {
		return nil
	}

	if freeSpace > 0 {
		realUsage += freeSpace
		if b != nil {
			if err := w.out.WriteBytes(b, offset, int(freeSpace)); err != nil {
				return err
			}
		} else {
			if err := w.out.CopyBytes(in, freeSpace); err != nil {
				return err
			}
		}
	}
	if realUsage > w.dir.maxUsedSize {
		w.dir.maxUsedSize = realUsage
	}

	size, err = w.dir.sizeInBytes()
	if err != nil {
		return err
	}
	message := fmt.Sprintf("fake disk full at %d bytes when writing %s (file length=%d", size, w.name, w.out.GetFilePointer())
	if freeSpace > 0 {
		message += fmt.Sprintf("; wrote %d of %d bytes", freeSpace, length)
	}
	message += ")"
	return errors.New(message)
}

// WriteByte writes a single byte through WriteBytes so that the disk-full and
// fault-injection checks apply to it as well.
func (w *mockIndexOutputWrapper) WriteByte(b byte) error {
	w.singleByte[0] = b
	return w.WriteBytes(w.singleByte, 0, 1)
}

// WriteBytes writes length bytes of b starting at offset.
func (w *mockIndexOutputWrapper) WriteBytes(b []byte, offset, length int) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.dir.mu.Lock()
	defer w.dir.mu.Unlock()

	if err := w.checkCrashed(); err != nil {
		return err
	}
	if err := w.checkDiskFull(b, offset, nil, int64(length)); err != nil {
		return err
	}

	if w.dir.randomState.Intn(200) == 0 {
		half := length / 2
		if err := w.out.WriteBytes(b, offset, half); err != nil {
			return err
		}
		runtime.Gosched()
		if err := w.out.WriteBytes(b, offset+half, length-half); err != nil {
			return err
		}
	} else {
		if err := w.out.WriteBytes(b, offset, length); err != nil {
			return err
		}
	}

	if err := w.dir.maybeThrowDeterministicException(); err != nil {
		return err
	}

	if w.first {
		// Maybe raise a random error; only on the first write to a new file.
		w.first = false
		if err := w.dir.maybeThrowIOException(w.name); err != nil {
			return err
		}
	}
	return nil
}

// WriteBytesN writes the first n bytes of b.
func (w *mockIndexOutputWrapper) WriteBytesN(b []byte, n int) error {
	return w.WriteBytes(b, 0, n)
}

// CopyBytes copies numBytes from input into this output.
func (w *mockIndexOutputWrapper) CopyBytes(input store.DataInput, numBytes int64) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}

	w.dir.mu.Lock()
	defer w.dir.mu.Unlock()

	if err := w.checkCrashed(); err != nil {
		return err
	}
	if err := w.checkDiskFull(nil, 0, input, numBytes); err != nil {
		return err
	}

	if err := w.out.CopyBytes(input, numBytes); err != nil {
		return err
	}
	return w.dir.maybeThrowDeterministicException()
}

// GetFilePointer returns the delegate's current position.
func (w *mockIndexOutputWrapper) GetFilePointer() int64 {
	return w.out.GetFilePointer()
}

// GetName returns the delegate's file name.
func (w *mockIndexOutputWrapper) GetName() string {
	return w.out.GetName()
}

// Close closes the delegate, unregisters the handle and, when disk-usage
// tracking is on, refreshes the directory's maximum used size.
func (w *mockIndexOutputWrapper) Close() error {
	if w.closed {
		// Do not mask double-close bugs.
		return w.out.Close()
	}
	w.closed = true

	err := w.out.Close()

	w.dir.removeIndexOutput(w, w.name)

	w.dir.mu.Lock()
	defer w.dir.mu.Unlock()

	if derr := w.dir.maybeThrowDeterministicException(); derr != nil && err == nil {
		err = derr
	}
	if w.dir.trackDiskUsage {
		// Compute the actual disk usage and track the maximum in the directory.
		size, serr := w.dir.sizeInBytes()
		if serr != nil {
			if err == nil {
				err = serr
			}
		} else if size > w.dir.maxUsedSize {
			w.dir.maxUsedSize = size
		}
	}
	return err
}

// prefetchableIndexInput is the optional IndexInput capability Lucene declares
// as IndexInput.prefetch, whose base implementation is a no-op. Gocene's
// spi.IndexInput does not carry it, so a delegate that supports it is reached
// through this assertion.
type prefetchableIndexInput interface {
	Prefetch(offset int64, length int64) error
}

// ioContextUpdatableIndexInput is the optional IndexInput capability Lucene
// declares as IndexInput.updateIOContext, whose base implementation is a no-op.
type ioContextUpdatableIndexInput interface {
	UpdateIOContext(ctx store.IOContext) error
}

// mockIndexInputWrapper is an IndexInput that keeps track of when it has been
// closed and of every clone and slice taken from it.
//
// Port of org.apache.lucene.tests.store.MockIndexInputWrapper.
type mockIndexInputWrapper struct {
	*store.FilterIndexInput

	dir      *MockDirectoryWrapper
	name     string
	delegate store.IndexInput
	closed   bool

	// parent is the wrapper this one was cloned or sliced from, or nil when
	// this wrapper was opened directly from the directory.
	parent *mockIndexInputWrapper
}

// newMockIndexInputWrapper wraps delegate for the given directory. parent is
// the wrapper this one was cloned or sliced from, or nil.
func newMockIndexInputWrapper(dir *MockDirectoryWrapper, name string, delegate store.IndexInput, parent *mockIndexInputWrapper) *mockIndexInputWrapper {
	return &mockIndexInputWrapper{
		FilterIndexInput: store.NewFilterIndexInput("MockIndexInputWrapper(name="+name+")", delegate),
		dir:              dir,
		name:             name,
		delegate:         delegate,
		parent:           parent,
	}
}

// ensureOpen reports an error when this input, or the input it was cloned
// from, has already been closed.
func (w *mockIndexInputWrapper) ensureOpen() error {
	if w.closed {
		return errors.New("Abusing closed IndexInput!")
	}
	if w.parent != nil && w.parent.closed {
		return errors.New("Abusing clone of a closed IndexInput!")
	}
	return nil
}

// Close closes the delegate and, for a non-clone, unregisters the handle.
func (w *mockIndexInputWrapper) Close() error {
	if w.closed {
		// Do not mask double-close bugs.
		return w.delegate.Close()
	}
	w.closed = true

	err := w.delegate.Close()

	// Pending resolution on LUCENE-686 the clones are deliberately not tracked.
	if w.parent == nil {
		w.dir.removeIndexInput(w, w.name)
	}

	w.dir.mu.Lock()
	if derr := w.dir.maybeThrowDeterministicException(); derr != nil && err == nil {
		err = derr
	}
	w.dir.mu.Unlock()

	return err
}

// Clone returns an independent view over the same file.
//
// spi.IndexInput.Clone returns no error, so the "abusing a closed IndexInput"
// check Lucene performs here is deferred to the next call that can report one.
func (w *mockIndexInputWrapper) Clone() store.IndexInput {
	w.dir.inputCloneCount.Add(1)

	parent := w.parent
	if parent == nil {
		parent = w
	}
	return newMockIndexInputWrapper(w.dir, w.name, w.delegate.Clone(), parent)
}

// Slice returns an independent view over a region of the same file.
func (w *mockIndexInputWrapper) Slice(description string, offset, length int64) (store.IndexInput, error) {
	if err := w.ensureOpen(); err != nil {
		return nil, err
	}
	w.dir.inputCloneCount.Add(1)

	slice, err := w.delegate.Slice(description, offset, length)
	if err != nil {
		return nil, err
	}

	parent := w.parent
	if parent == nil {
		parent = w
	}
	return newMockIndexInputWrapper(w.dir, description, slice, parent), nil
}

// SetPosition seeks the delegate to pos. This is Gocene's spelling of Lucene's
// IndexInput.seek.
func (w *mockIndexInputWrapper) SetPosition(pos int64) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}
	return w.delegate.SetPosition(pos)
}

// Prefetch forwards the hint to the delegate when it supports one; Lucene's
// IndexInput.prefetch is a no-op by default.
func (w *mockIndexInputWrapper) Prefetch(offset, length int64) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}
	if p, ok := w.delegate.(prefetchableIndexInput); ok {
		return p.Prefetch(offset, length)
	}
	return nil
}

// UpdateIOContext forwards the new context to the delegate when it supports
// one; Lucene's IndexInput.updateIOContext is a no-op by default.
func (w *mockIndexInputWrapper) UpdateIOContext(ctx store.IOContext) error {
	if err := w.ensureOpen(); err != nil {
		return err
	}
	if u, ok := w.delegate.(ioContextUpdatableIndexInput); ok {
		return u.UpdateIOContext(ctx)
	}
	return nil
}

// Compile-time assertions that the wrappers satisfy the store contracts.
var (
	_ store.IndexOutput = (*mockIndexOutputWrapper)(nil)
	_ store.IndexInput  = (*mockIndexInputWrapper)(nil)
)
