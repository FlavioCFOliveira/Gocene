package search

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// mockExecutor is a mock implementation of Executor for testing
type mockExecutor struct {
	executed []func()
}

func (m *mockExecutor) Execute(fn func()) {
	m.executed = append(m.executed, fn)
	fn()
}

func (m *mockExecutor) Shutdown() error {
	return nil
}

// factoryMockIndexReader is a mock implementation of IndexReaderInterface for testing
type factoryMockIndexReader struct {
	numDocs    int
	docCount   int
	maxDoc     int
	closed     atomic.Bool
	refCount   atomic.Int32
	numDeleted int
}

func (m *factoryMockIndexReader) DocCount() int {
	return m.docCount
}

func (m *factoryMockIndexReader) NumDocs() int {
	return m.numDocs
}

func (m *factoryMockIndexReader) MaxDoc() int {
	return m.maxDoc
}

func (m *factoryMockIndexReader) Close() error {
	m.closed.Store(true)
	return nil
}

func (m *factoryMockIndexReader) HasDeletions() bool {
	return m.numDeleted > 0
}

func (m *factoryMockIndexReader) NumDeletedDocs() int {
	return m.numDeleted
}

func (m *factoryMockIndexReader) EnsureOpen() error {
	if m.closed.Load() {
		return errors.New("reader is closed")
	}
	return nil
}

func (m *factoryMockIndexReader) IncRef() error {
	if m.closed.Load() {
		return errors.New("reader is closed")
	}
	m.refCount.Add(1)
	return nil
}

func (m *factoryMockIndexReader) DecRef() error {
	if m.refCount.Add(-1) <= 0 {
		return m.Close()
	}
	return nil
}

func (m *factoryMockIndexReader) TryIncRef() bool {
	if m.closed.Load() {
		return false
	}
	m.refCount.Add(1)
	return true
}

func (m *factoryMockIndexReader) GetRefCount() int32 {
	return m.refCount.Load()
}

func (m *factoryMockIndexReader) GetContext() (index.IndexReaderContext, error) {
	return nil, nil
}

func (m *factoryMockIndexReader) Leaves() ([]*index.LeafReaderContext, error) {
	return nil, nil
}

func (m *factoryMockIndexReader) StoredFields() (index.StoredFields, error) {
	return nil, nil
}

func newFactoryMockIndexReader(numDocs int) *factoryMockIndexReader {
	return &factoryMockIndexReader{
		numDocs:  numDocs,
		docCount: numDocs,
		maxDoc:   numDocs,
	}
}

func TestNewDefaultSearcherFactory(t *testing.T) {
	factory := NewDefaultSearcherFactory()
	if factory == nil {
		t.Fatal("expected factory to not be nil")
	}
	if factory.GetExecutor() != nil {
		t.Error("expected executor to be nil")
	}
}

func TestNewDefaultSearcherFactoryWithExecutor(t *testing.T) {
	executor := &mockExecutor{}
	factory := NewDefaultSearcherFactoryWithExecutor(executor)
	if factory == nil {
		t.Fatal("expected factory to not be nil")
	}
	if factory.GetExecutor() != executor {
		t.Error("expected executor to be set")
	}
}

func TestDefaultSearcherFactory_NewSearcher(t *testing.T) {
	factory := NewDefaultSearcherFactory()
	reader := newFactoryMockIndexReader(100)

	ctx := context.Background()
	searcher, err := factory.NewSearcher(ctx, reader)
	if err != nil {
		t.Fatalf("failed to create searcher: %v", err)
	}
	if searcher == nil {
		t.Fatal("expected searcher to not be nil")
	}
}

func TestDefaultSearcherFactory_NewSearcher_NilReader(t *testing.T) {
	factory := NewDefaultSearcherFactory()
	ctx := context.Background()

	_, err := factory.NewSearcher(ctx, nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestDefaultSearcherFactory_SetExecutor(t *testing.T) {
	factory := NewDefaultSearcherFactory()
	executor := &mockExecutor{}

	factory.SetExecutor(executor)
	if factory.GetExecutor() != executor {
		t.Error("expected executor to be set")
	}
}

func TestNewWarmingSearcherFactory(t *testing.T) {
	warmFunc := func(ctx context.Context, searcher *IndexSearcher) error {
		return nil
	}

	factory := NewWarmingSearcherFactory(warmFunc)
	if factory == nil {
		t.Fatal("expected factory to not be nil")
	}
	if factory.GetWarmFunction() == nil {
		t.Error("expected warm function to be set")
	}
}

func TestWarmingSearcherFactory_NewSearcher(t *testing.T) {
	warmed := false
	warmFunc := func(ctx context.Context, searcher *IndexSearcher) error {
		warmed = true
		return nil
	}

	factory := NewWarmingSearcherFactory(warmFunc)
	reader := newFactoryMockIndexReader(100)

	ctx := context.Background()
	searcher, err := factory.NewSearcher(ctx, reader)
	if err != nil {
		t.Fatalf("failed to create searcher: %v", err)
	}
	if searcher == nil {
		t.Fatal("expected searcher to not be nil")
	}
	if !warmed {
		t.Error("expected warm function to be called")
	}
}

func TestWarmingSearcherFactory_NewSearcher_WarmError(t *testing.T) {
	warmFunc := func(ctx context.Context, searcher *IndexSearcher) error {
		return errors.New("warm error")
	}

	factory := NewWarmingSearcherFactory(warmFunc)
	reader := newFactoryMockIndexReader(100)

	ctx := context.Background()
	_, err := factory.NewSearcher(ctx, reader)
	if err == nil {
		t.Error("expected error when warm function fails")
	}
}

func TestWarmingSearcherFactory_SetWarmFunction(t *testing.T) {
	factory := NewWarmingSearcherFactory(nil)

	warmFunc := func(ctx context.Context, searcher *IndexSearcher) error {
		return nil
	}

	factory.SetWarmFunction(warmFunc)
	if factory.GetWarmFunction() == nil {
		t.Error("expected warm function to be set")
	}
}

func TestNewCachingSearcherFactory(t *testing.T) {
	factory := NewCachingSearcherFactory()
	if factory == nil {
		t.Fatal("expected factory to not be nil")
	}
	if factory.GetCacheSize() != 0 {
		t.Errorf("expected cache size 0, got %d", factory.GetCacheSize())
	}
}

func TestCachingSearcherFactory_NewSearcher_CachesResult(t *testing.T) {
	factory := NewCachingSearcherFactory()
	reader := newFactoryMockIndexReader(100)

	ctx := context.Background()

	// Create first searcher
	searcher1, err := factory.NewSearcher(ctx, reader)
	if err != nil {
		t.Fatalf("failed to create searcher: %v", err)
	}

	// Create second searcher with same reader
	searcher2, err := factory.NewSearcher(ctx, reader)
	if err != nil {
		t.Fatalf("failed to create searcher: %v", err)
	}

	// Should be the same instance
	if searcher1 != searcher2 {
		t.Error("expected cached searcher to be returned")
	}

	if factory.GetCacheSize() != 1 {
		t.Errorf("expected cache size 1, got %d", factory.GetCacheSize())
	}
}

func TestCachingSearcherFactory_ClearCache(t *testing.T) {
	factory := NewCachingSearcherFactory()
	reader := newFactoryMockIndexReader(100)

	ctx := context.Background()
	_, err := factory.NewSearcher(ctx, reader)
	if err != nil {
		t.Fatalf("failed to create searcher: %v", err)
	}

	if factory.GetCacheSize() != 1 {
		t.Errorf("expected cache size 1, got %d", factory.GetCacheSize())
	}

	factory.ClearCache()

	if factory.GetCacheSize() != 0 {
		t.Errorf("expected cache size 0 after clear, got %d", factory.GetCacheSize())
	}
}

func TestCachingSearcherFactory_RemoveFromCache(t *testing.T) {
	factory := NewCachingSearcherFactory()
	reader1 := newFactoryMockIndexReader(100)
	reader2 := newFactoryMockIndexReader(200)

	ctx := context.Background()
	_, err := factory.NewSearcher(ctx, reader1)
	if err != nil {
		t.Fatalf("failed to create searcher: %v", err)
	}
	_, err = factory.NewSearcher(ctx, reader2)
	if err != nil {
		t.Fatalf("failed to create searcher: %v", err)
	}

	if factory.GetCacheSize() != 2 {
		t.Errorf("expected cache size 2, got %d", factory.GetCacheSize())
	}

	factory.RemoveFromCache(reader1)

	if factory.GetCacheSize() != 1 {
		t.Errorf("expected cache size 1 after remove, got %d", factory.GetCacheSize())
	}
}

func TestCachingSearcherFactory_NewSearcher_NilReader(t *testing.T) {
	factory := NewCachingSearcherFactory()
	ctx := context.Background()

	_, err := factory.NewSearcher(ctx, nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestNewWarmingSearcherFactoryWithExecutor(t *testing.T) {
	executor := &mockExecutor{}
	warmFunc := func(ctx context.Context, searcher *IndexSearcher) error {
		return nil
	}

	factory := NewWarmingSearcherFactoryWithExecutor(warmFunc, executor)
	if factory == nil {
		t.Fatal("expected factory to not be nil")
	}
	if factory.GetExecutor() != executor {
		t.Error("expected executor to be set")
	}
	if factory.GetWarmFunction() == nil {
		t.Error("expected warm function to be set")
	}
}
