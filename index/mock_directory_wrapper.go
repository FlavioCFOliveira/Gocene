package index

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"

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

	maxSize        int64
	maxUsedSize    int64
	randomIOERate  float64
	randomIOERateOpen float64
	randomState    *rand.Rand

	assertNoDeleteOpenFile           bool
	trackDiskUsage                  bool
	useSlowOpenClosers              bool
	allowRandomFileNotFoundException bool
	allowReadingFilesStillOpenForWrite bool

	unSyncedFiles     map[string]struct{}
	createdFiles      map[string]struct{}
	openFilesForWrite map[string]struct{}
	openLocks         sync.Map // map[string]error
	crashed           bool
	throttling        Throttling

	alwaysCorrupt bool
	inputCloneCount atomic.Int32
	openFileHandles map[io.Closer]error
	openFilesDeleted map[string]struct{}
	failures        []Failure
}

func NewMockDirectoryWrapper(r *rand.Rand, delegate store.Directory) *MockDirectoryWrapper {
	return &MockDirectoryWrapper{
		in:                      delegate,
		randomState:             rand.New(rand.NewSource(r.Int63())),
		unSyncedFiles:           make(map[string]struct{}),
		createdFiles:            make(map[string]struct{}),
		openFilesForWrite:       make(map[string]struct{}),
		openFileHandles:        make(map[io.Closer]error),
		openFilesDeleted:        make(map[string]struct{}),
		throttling:              ThrottlingNever,
		allowRandomFileNotFoundException: true,
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

	var size int64
	files, err := m.in.ListAll()
	if err != nil {
		return 0, err
	}
	for _, file := range files {
		if len(file) >= 5 && file[:5] == "extra" {
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

func (m *MockDirectoryWrapper) CorruptUnknownFiles() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	knownFiles := make(map[string]struct{})
	files, err := m.in.ListAll()
	if err != nil {
		return err
	}
	for _, fileName := range files {
		if len(fileName) >= 9 && fileName[:9] == "segments_" {
		}
	}

	toCorrupt := make([]string, 0)
	for _, fileName := range files {
		if _, known := knownFiles[fileName]; !known &&
		   fileName != "write.lock" &&
		   (len(fileName) >= 15 && fileName[:15] == "pending_segments") {
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

func (m *MockDirectoryWrapper) corruptFilesInternal(files []string) error {
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
			zeroes := make([]byte, 256)
			var upto int64
			for upto < length {
				limit := int(length - upto)
				if limit > 256 {
					limit = 256
				}
				out.WriteBytes(zeroes[:limit])
				upto += int64(limit)
			}
			out.Close()
		case 2: // partially truncated
			tempOut, err := m.in.CreateOutput("mdw_corrupt_temp", store.NewIOContext())
			if err != nil {
				return err
			}
			tempName := tempOut.GetName()
			ii, err := m.in.OpenInput(name, store.NewIOContext())
			if err != nil {
				tempOut.Close()
				return err
			}
			ii.Seek(0)
			buf := make([]byte, 4096)
			var copied int64
			half := ii.Length() / 2
			for copied < half {
				n, err := ii.ReadBytes(buf)
				if n > 0 {
					if copied+int64(n) > half {
						tempOut.WriteBytes(buf[:int(half-copied)])
						copied = half
						break
					}
					tempOut.WriteBytes(buf[:n])
					copied += int64(n)
				}
				if err != nil {
					break
				}
			}
			ii.Close()
			tempOut.Close()

			m.in.DeleteFile(name)
			out, err := m.in.CreateOutput(name, store.NewIOContext())
			if err != nil {
				return err
			}
			ii2, err := m.in.OpenInput(tempName, store.NewIOContext())
			if err != nil {
				out.Close()
				return err
			}
			for {
				n, err := ii2.ReadBytes(buf)
				if n > 0 {
					out.WriteBytes(buf[:n])
				}
				if err != nil {
					break
				}
			}
			ii2.Close()
			out.Close()
			m.in.DeleteFile(tempName)
		case 3: // didn't change
		case 4: // flip bit
			tempOut, err := m.in.CreateOutput("mdw_corrupt_bit", store.NewIOContext())
			if err != nil {
				return err
			}
			tempName := tempOut.GetName()
			ii, err := m.in.OpenInput(name, store.NewIOContext())
			if err != nil {
				tempOut.Close()
				return err
			}
			length := ii.Length()
			if length > 0 {
				byteToCorrupt := m.randomState.Int63n(length)
				ii.Seek(byteToCorrupt)
				b := ii.ReadByte()
				bitToFlip := uint(m.randomState.Intn(8))
				b ^= (1 << bitToFlip)

				ii.Seek(0)
				buf := make([]byte, 4096)
				var current int64
				for current < length {
					n, err := ii.ReadBytes(buf)
					if n > 0 {
						for i := 0; i < n; i++ {
							if current+int64(i) == byteToCorrupt {
								buf[i] = b
							}
						}
						tempOut.WriteBytes(buf[:n])
						current += int64(n)
					}
					if err != nil {
						break
					}
				}
			}
			ii.Close()
			tempOut.Close()

			m.in.DeleteFile(name)
			out, err := m.in.CreateOutput(name, store.NewIOContext())
			if err != nil {
				return err
			}
			ii2, err := m.in.OpenInput(tempName, store.NewIOContext())
			if err != nil {
				out.Close()
				return err
			}
			buf2 := make([]byte, 4096)
			for {
				n, err := ii2.ReadBytes(buf2)
				if n > 0 {
					out.WriteBytes(buf2[:n])
				}
				if err != nil {
					break
				}
			}
			ii2.Close()
			out.Close()
			m.in.DeleteFile(tempName)
		case 5: // fully truncated
			m.in.DeleteFile(name)
			out, err := m.in.CreateOutput(name, store.NewIOContext())
			if err != nil {
				return err
			}
			out.Close()
		}
	}
	return nil
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

	io := &mockIndexOutputWrapper{
		dir:  m,
		out:  delegateOutput,
		name: name,
	}
	m.addFileHandle(io, name)
	m.openFilesForWrite[name] = struct{}{}

	return m.maybeThrottle(name, io)
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

	delegateOutput, err := m.in.CreateTempOutput(prefix, suffix, ctx)
	if err != nil {
		return nil, err
	}
	name := delegateOutput.GetName()

	m.unSyncedFiles[name] = struct{}{}
	m.createdFiles[name] = struct{}{}

	io := &mockIndexOutputWrapper{
		dir:  m,
		out:  delegateOutput,
		name: name,
	}
	m.addFileHandle(io, name)
	m.openFilesForWrite[name] = struct{}{}

	return m.maybeThrottle(name, io)
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

	ii := &mockIndexInputWrapper{
		dir:      m,
		name:     name,
		delegate: delegateInput,
	}
	m.addFileHandle(ii, name)
	return ii
}

func (m *MockDirectoryWrapper) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.in.IsOpen() {
		return m.in.Close()
	}

	if len(m.openFileHandles) > 0 {
		return fmt.Errorf("MockDirectoryWrapper: cannot close: there are still %d open files", len(m.openFileHandles))
	}

	err := m.in.Close()
	return err
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

type mockIndexOutputWrapper struct {
	dir  *MockDirectoryWrapper
	out  store.IndexOutput
	name string
}

func (w *mockIndexOutputWrapper) WriteBytes(b []byte) error {
	return w.WriteBytesWithOffset(b, 0, len(b))
}

func (w *mockIndexOutputWrapper) WriteBytesWithOffset(b []byte, offset, length int) error {
	w.dir.mu.Lock()
	defer w.dir.mu.Unlock()

	if w.dir.crashed {
		return fmt.Errorf("MockDirectoryWrapper has crashed; cannot write to %s", w.name)
	}

	if w.dir.maxSize != 0 {
		size, _ := w.dir.SizeInBytes()
		freeSpace := w.dir.maxSize - size
		if freeSpace <= int64(length) {
			return fmt.Errorf("fake disk full at %d bytes when writing %s", size, w.name)
		}
	}

	if w.dir.randomState.Intn(200) == 0 {
		half := length / 2
		w.out.WriteBytes(b[offset : offset+half])
		w.out.WriteBytes(b[offset+half : offset+length])
	} else {
		w.out.WriteBytes(b[offset : offset+length])
	}

	if err := w.dir.maybeThrowDeterministicException(); err != nil {
		return err
	}

	return nil
}

func (w *mockIndexOutputWrapper) CopyBytes(in store.IndexInput, numBytes int64) error {
	w.dir.mu.Lock()
	defer w.dir.mu.Unlock()

	if w.dir.crashed {
		return fmt.Errorf("MockDirectoryWrapper has crashed; cannot write to %s", w.name)
	}
	err := w.out.CopyBytes(in, numBytes)
	if err != nil {
		return err
	}
	return w.dir.maybeThrowDeterministicException()
}

func (w *mockIndexOutputWrapper) GetFilePointer() int64 {
	return w.out.GetFilePointer()
}

func (w *mockIndexOutputWrapper) GetName() string {
	return w.out.GetName()
}

func (w *mockIndexOutputWrapper) Close() error {
	err := w.out.Close()
	w.dir.removeIndexOutput(w, w.name)
	return err
}

type mockIndexInputWrapper struct {
	dir      *MockDirectoryWrapper
	name     string
	delegate store.IndexInput
	closed   bool
}

func (w *mockIndexInputWrapper) Close() error {
	if w.closed {
		return w.delegate.Close()
	}
	w.closed = true
	err := w.delegate.Close()
	w.dir.removeIndexInput(w, w.name)
	w.dir.mu.Lock()
	if err := w.dir.maybeThrowDeterministicException(); err != nil {
		w.dir.mu.Unlock()
		return err
	}
	w.dir.mu.Unlock()
	return err
}

func (w *mockIndexInputWrapper) ReadByte() (byte, error) {
	return w.delegate.ReadByte()
}

func (w *mockIndexInputWrapper) ReadBytes(b []byte) (int, error) {
	return w.delegate.ReadBytes(b)
}

func (w *mockIndexInputWrapper) ReadInt() (int, error) {
	return w.delegate.ReadInt()
}

func (w *mockIndexInputWrapper) ReadLong() (int64, error) {
	return w.delegate.ReadLong()
}

func (w *mockIndexInputWrapper) ReadFloat() (float32, error) {
	return w.delegate.ReadFloat()
}

func (w *mockIndexInputWrapper) ReadVInt() (int, error) {
	return w.delegate.ReadVInt()
}

func (w *mockIndexInputWrapper) ReadVLong() (int64, error) {
	return w.delegate.ReadVLong()
}

func (w *mockIndexInputWrapper) ReadZInt() (int, error) {
	return w.delegate.ReadZInt()
}

func (w *mockIndexInputWrapper) ReadZLong() (int64, error) {
	return w.delegate.ReadZLong()
}

func (w *mockIndexInputWrapper) ReadString() (string, error) {
	return w.delegate.ReadString()
}

func (w *mockIndexInputWrapper) Seek(pos int64) error {
	return w.delegate.Seek(pos)
}

func (w *mockIndexInputWrapper) GetFilePointer() int64 {
	return w.delegate.GetFilePointer()
}

func (w *mockIndexInputWrapper) Length() int64 {
	return w.delegate.Length()
}

func (w *mockIndexInputWrapper) Clone() (store.IndexInput, error) {
	w.dir.mu.Lock()
	w.dir.inputCloneCount.Add(1)
	w.dir.mu.Unlock()

	clone, err := w.delegate.Clone()
	if err != nil {
		return nil, err
	}
	return &mockIndexInputWrapper{
		dir:      w.dir,
		name:     w.name,
		delegate: clone,
	}, nil
}

func (w *mockIndexInputWrapper) Slice(description string, offset, length int64) (store.IndexInput, error) {
	w.dir.mu.Lock()
	w.dir.inputCloneCount.Add(1)
	w.dir.mu.Unlock()

	slice, err := w.delegate.Slice(description, offset, length)
	if err != nil {
		return nil, err
	}
	return &mockIndexInputWrapper{
		dir:      w.dir,
		name:     description,
		delegate: slice,
	}, nil
}

func (w *mockIndexInputWrapper) Prefetch(offset, length int64) error {
	return w.delegate.Prefetch(offset, length)
}

func (w *mockIndexInputWrapper) UpdateIOContext(ctx store.IOContext) error {
	return w.delegate.UpdateIOContext(ctx)
}
