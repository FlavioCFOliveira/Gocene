package search

import (
	"context"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SearcherFactory is a factory pattern for creating IndexSearcher instances.
// It allows customization of how IndexSearcher instances are created and configured.
// Implementations can provide custom warming, caching, or other initialization logic.
type SearcherFactory interface {
	// NewSearcher creates a new IndexSearcher from the provided IndexReader.
	// The factory is responsible for any custom initialization of the searcher.
	NewSearcher(ctx context.Context, reader index.IndexReaderInterface) (*IndexSearcher, error)
}

// DefaultSearcherFactory is the default implementation of SearcherFactory.
// It creates IndexSearcher instances with standard configuration.
type DefaultSearcherFactory struct {
	// executor is optional and can be used for multithreaded search
	executor Executor
}

// Executor is an interface for executing search tasks concurrently.
// This is typically a thread pool or similar mechanism.
type Executor interface {
	// Execute runs the given function in a separate goroutine/worker
	Execute(fn func())
	// Shutdown gracefully shuts down the executor
	Shutdown() error
}

// NewDefaultSearcherFactory creates a new DefaultSearcherFactory.
// This factory creates IndexSearcher instances with standard configuration.
func NewDefaultSearcherFactory() *DefaultSearcherFactory {
	return &DefaultSearcherFactory{}
}

// NewDefaultSearcherFactoryWithExecutor creates a new DefaultSearcherFactory
// with a custom executor for multithreaded search.
func NewDefaultSearcherFactoryWithExecutor(executor Executor) *DefaultSearcherFactory {
	return &DefaultSearcherFactory{
		executor: executor,
	}
}

// NewSearcher creates a new IndexSearcher from the provided IndexReader.
// Returns an error if the reader is nil.
func (f *DefaultSearcherFactory) NewSearcher(ctx context.Context, reader index.IndexReaderInterface) (*IndexSearcher, error) {
	if reader == nil {
		return nil, fmt.Errorf("index reader cannot be nil")
	}

	// Create the IndexSearcher
	searcher := NewIndexSearcher(reader)

	return searcher, nil
}

// SetExecutor sets the executor for multithreaded search.
// This can be used to change the executor after factory creation.
func (f *DefaultSearcherFactory) SetExecutor(executor Executor) {
	f.executor = executor
}

// GetExecutor returns the current executor, or nil if not set.
func (f *DefaultSearcherFactory) GetExecutor() Executor {
	return f.executor
}

// WarmFunction is a function type for warming searchers.
// Warming is the process of pre-populating caches or performing
// other initialization work before the searcher is used for queries.
type WarmFunction func(ctx context.Context, searcher *IndexSearcher) error

// WarmingSearcherFactory is a SearcherFactory that supports warming.
// It executes a warm function on newly created searchers before they are returned.
type WarmingSearcherFactory struct {
	*DefaultSearcherFactory
	warmFunction WarmFunction
}

// NewWarmingSearcherFactory creates a new WarmingSearcherFactory.
// The warm function is called on each new searcher before it is returned.
func NewWarmingSearcherFactory(warmFunction WarmFunction) *WarmingSearcherFactory {
	return &WarmingSearcherFactory{
		DefaultSearcherFactory: NewDefaultSearcherFactory(),
		warmFunction:           warmFunction,
	}
}

// NewWarmingSearcherFactoryWithExecutor creates a new WarmingSearcherFactory
// with a custom executor for multithreaded search.
func NewWarmingSearcherFactoryWithExecutor(warmFunction WarmFunction, executor Executor) *WarmingSearcherFactory {
	return &WarmingSearcherFactory{
		DefaultSearcherFactory: NewDefaultSearcherFactoryWithExecutor(executor),
		warmFunction:           warmFunction,
	}
}

// NewSearcher creates a new IndexSearcher and warms it if a warm function is configured.
func (f *WarmingSearcherFactory) NewSearcher(ctx context.Context, reader index.IndexReaderInterface) (*IndexSearcher, error) {
	// Create the searcher using the default factory
	searcher, err := f.DefaultSearcherFactory.NewSearcher(ctx, reader)
	if err != nil {
		return nil, err
	}

	// Warm the searcher if a warm function is configured
	if f.warmFunction != nil {
		if err := f.warmFunction(ctx, searcher); err != nil {
			return nil, fmt.Errorf("warming searcher failed: %w", err)
		}
	}

	return searcher, nil
}

// SetWarmFunction sets the warm function for this factory.
// The warm function will be called on all newly created searchers.
func (f *WarmingSearcherFactory) SetWarmFunction(warmFunction WarmFunction) {
	f.warmFunction = warmFunction
}

// GetWarmFunction returns the current warm function, or nil if not set.
func (f *WarmingSearcherFactory) GetWarmFunction() WarmFunction {
	return f.warmFunction
}

// CachingSearcherFactory is a SearcherFactory that caches IndexSearcher instances
// by IndexReader to avoid recreating them for the same reader.
// Note: This is primarily useful when readers are reused.
type CachingSearcherFactory struct {
	*DefaultSearcherFactory
	cache map[index.IndexReaderInterface]*IndexSearcher
	mu    sync.RWMutex
}

// NewCachingSearcherFactory creates a new CachingSearcherFactory.
func NewCachingSearcherFactory() *CachingSearcherFactory {
	return &CachingSearcherFactory{
		DefaultSearcherFactory: NewDefaultSearcherFactory(),
		cache:                  make(map[index.IndexReaderInterface]*IndexSearcher),
	}
}

// NewSearcher creates or retrieves a cached IndexSearcher for the given reader.
func (f *CachingSearcherFactory) NewSearcher(ctx context.Context, reader index.IndexReaderInterface) (*IndexSearcher, error) {
	if reader == nil {
		return nil, fmt.Errorf("index reader cannot be nil")
	}

	// Check cache first
	f.mu.RLock()
	if searcher, ok := f.cache[reader]; ok {
		f.mu.RUnlock()
		return searcher, nil
	}
	f.mu.RUnlock()

	// Create new searcher
	searcher, err := f.DefaultSearcherFactory.NewSearcher(ctx, reader)
	if err != nil {
		return nil, err
	}

	// Cache the searcher
	f.mu.Lock()
	f.cache[reader] = searcher
	f.mu.Unlock()

	return searcher, nil
}

// ClearCache removes all cached searchers.
func (f *CachingSearcherFactory) ClearCache() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cache = make(map[index.IndexReaderInterface]*IndexSearcher)
}

// RemoveFromCache removes a specific reader's searcher from the cache.
func (f *CachingSearcherFactory) RemoveFromCache(reader index.IndexReaderInterface) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.cache, reader)
}

// GetCacheSize returns the number of cached searchers.
func (f *CachingSearcherFactory) GetCacheSize() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.cache)
}
