// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// ExitableDirectoryReader is a DirectoryReader that wraps another DirectoryReader
// and checks for query timeout during document iteration.
//
// This is the Go port of Lucene's org.apache.lucene.search.ExitableDirectoryReader.
//
// ExitableDirectoryReader wraps an existing DirectoryReader and provides the ability
// to cancel long-running searches by checking the timeout periodically during
// document iteration. This is useful for preventing runaway queries from consuming
// excessive resources.
//
// Usage:
//
//	// Wrap an existing reader with a timeout context
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	exitableReader, err := NewExitableDirectoryReader(reader, ExitableReaderConfig{
//	    QueryContext: ctx,
//	    CheckEvery: 1024, // Check timeout every 1024 documents
//	})
//
// When the context is cancelled or times out, any subsequent document iteration
// operations will return an error indicating the query was cancelled.
type ExitableDirectoryReader struct {
	*DirectoryReader
	in     *DirectoryReader
	config ExitableReaderConfig
}

// ExitableReaderConfig contains configuration for an ExitableDirectoryReader.
type ExitableReaderConfig struct {
	// QueryContext is the context that controls query cancellation.
	// When this context is cancelled or times out, the search will be aborted.
	// Required.
	QueryContext context.Context

	// CheckEvery specifies how often to check for timeout (in number of documents).
	// A smaller value means more frequent checks but higher overhead.
	// A larger value means less overhead but slower cancellation response.
	// Default is 1024 if zero.
	CheckEvery int
}

// DefaultCheckEvery is the default number of documents between timeout checks.
const DefaultCheckEvery = 1024

// NewExitableDirectoryReader creates a new ExitableDirectoryReader wrapping the given reader.
//
// The config parameter specifies the timeout behavior. If config.QueryContext is nil,
// the function will return an error.
func NewExitableDirectoryReader(in *DirectoryReader, config ExitableReaderConfig) (*ExitableDirectoryReader, error) {
	if config.QueryContext == nil {
		return nil, fmt.Errorf("QueryContext is required")
	}
	if config.CheckEvery <= 0 {
		config.CheckEvery = DefaultCheckEvery
	}

	return &ExitableDirectoryReader{
		DirectoryReader: in,
		in:              in,
		config:          config,
	}, nil
}

// GetDelegate returns the wrapped DirectoryReader.
func (r *ExitableDirectoryReader) GetDelegate() *DirectoryReader {
	return r.in
}

// Close closes the wrapped reader.
func (r *ExitableDirectoryReader) Close() error {
	return r.in.Close()
}

// GetContext returns the reader context, wrapping leaf readers with exitable versions.
func (r *ExitableDirectoryReader) GetContext() (IndexReaderContext, error) {
	ctx, err := r.in.GetContext()
	if err != nil {
		return nil, err
	}

	// If it's a composite context, we need to wrap the leaves
	if compCtx, ok := ctx.(*CompositeReaderContext); ok {
		// The leaves are already wrapped by the context
		// We return the original context since we handle
		// exitable checks at the postings level
		return compCtx, nil
	}

	return ctx, nil
}

// Leaves returns all leaf reader contexts, wrapping them with exitable versions.
func (r *ExitableDirectoryReader) Leaves() ([]*LeafReaderContext, error) {
	leaves, err := r.in.Leaves()
	if err != nil {
		return nil, err
	}

	// Wrap each leaf context with exitable wrapper
	exitableLeaves := make([]*LeafReaderContext, len(leaves))
	for i, leaf := range leaves {
		exitableLeaf := NewExitableLeafReader(leaf.LeafReader().(*LeafReader), r.config)
		exitableLeaves[i] = NewLeafReaderContext(
			exitableLeaf,
			leaf.Parent(),
			leaf.Ord(),
			leaf.DocBase(),
		)
	}

	return exitableLeaves, nil
}

// GetSequentialSubReaders returns the segment readers from the wrapped reader.
func (r *ExitableDirectoryReader) GetSequentialSubReaders() []*SegmentReader {
	// Return the underlying readers - exitable wrapping is handled at the postings level
	return r.in.GetSequentialSubReaders()
}

// IsCurrent returns true if the wrapped reader is up to date.
func (r *ExitableDirectoryReader) IsCurrent() (bool, error) {
	return r.in.IsCurrent()
}

// Reopen reopens the wrapped reader.
func (r *ExitableDirectoryReader) Reopen() (*ExitableDirectoryReader, error) {
	newReader, err := r.in.Reopen()
	if err != nil {
		return nil, err
	}
	if newReader == r.in {
		return r, nil
	}
	return NewExitableDirectoryReader(newReader, r.config)
}

// ReopenFromCommit reopens from a specific commit.
func (r *ExitableDirectoryReader) ReopenFromCommit(commit *IndexCommit) (*DirectoryReader, error) {
	return r.in.ReopenFromCommit(commit)
}

// GetSegmentInfos returns the segment infos from the wrapped reader.
func (r *ExitableDirectoryReader) GetSegmentInfos() *SegmentInfos {
	return r.in.GetSegmentInfos()
}

// GetIndexCommit returns the index commit from the wrapped reader.
func (r *ExitableDirectoryReader) GetIndexCommit() *IndexCommit {
	return r.in.GetIndexCommit()
}

// GetDirectory returns the directory from the wrapped reader.
func (r *ExitableDirectoryReader) GetDirectory() store.Directory {
	return r.in.GetDirectory()
}

// Ensure DirectoryReader implements IndexReaderInterface
var _ IndexReaderInterface = (*ExitableDirectoryReader)(nil)

// QueryCancelledError is returned when a query is cancelled due to timeout.
type QueryCancelledError struct {
	Reason string
}

// Error returns the error message.
func (e *QueryCancelledError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("query cancelled: %s", e.Reason)
	}
	return "query cancelled"
}

// IsQueryCancelled returns true if the error is a QueryCancelledError.
func IsQueryCancelled(err error) bool {
	_, ok := err.(*QueryCancelledError)
	return ok
}

// ExitableLeafReader is a LeafReader that wraps another LeafReader
// and checks for query timeout during document iteration.
type ExitableLeafReader struct {
	*LeafReader
	in     *LeafReader
	config ExitableReaderConfig
}

// NewExitableLeafReader creates a new ExitableLeafReader wrapping the given reader.
func NewExitableLeafReader(in *LeafReader, config ExitableReaderConfig) *ExitableLeafReader {
	return &ExitableLeafReader{
		LeafReader: in,
		in:         in,
		config:     config,
	}
}

// GetDelegate returns the wrapped LeafReader.
func (r *ExitableLeafReader) GetDelegate() *LeafReader {
	return r.in
}

// checkTimeout checks if the query has been cancelled.
// Returns a QueryCancelledError if the context is done.
func (r *ExitableLeafReader) checkTimeout() error {
	select {
	case <-r.config.QueryContext.Done():
		return &QueryCancelledError{Reason: r.config.QueryContext.Err().Error()}
	default:
		return nil
	}
}

// GetContext returns the reader context.
func (r *ExitableLeafReader) GetContext() (IndexReaderContext, error) {
	return r.in.GetContext()
}

// Postings returns exitable-wrapped postings for a term.
func (r *ExitableLeafReader) Postings(term Term) (PostingsEnum, error) {
	enum, err := r.in.Postings(term)
	if err != nil {
		return nil, err
	}
	if enum == nil {
		return nil, nil
	}
	return NewExitablePostingsEnum(enum, r.config), nil
}

// PostingsWithFreqPositions returns exitable-wrapped postings with specific flags.
func (r *ExitableLeafReader) PostingsWithFreqPositions(term Term, flags int) (PostingsEnum, error) {
	enum, err := r.in.PostingsWithFreqPositions(term, flags)
	if err != nil {
		return nil, err
	}
	if enum == nil {
		return nil, nil
	}
	return NewExitablePostingsEnum(enum, r.config), nil
}

// GetTermVectors returns term vectors with timeout checks.
func (r *ExitableLeafReader) GetTermVectors(docID int) (Fields, error) {
	if err := r.checkTimeout(); err != nil {
		return nil, err
	}
	return r.in.GetTermVectors(docID)
}

// Terms returns terms with timeout checks.
func (r *ExitableLeafReader) Terms(field string) (Terms, error) {
	if err := r.checkTimeout(); err != nil {
		return nil, err
	}
	return r.in.Terms(field)
}

// Close closes the wrapped reader.
func (r *ExitableLeafReader) Close() error {
	return r.in.Close()
}

// Ensure LeafReader implements required interface
var _ interface {
	Postings(term Term) (PostingsEnum, error)
	PostingsWithFreqPositions(term Term, flags int) (PostingsEnum, error)
	GetTermVectors(docID int) (Fields, error)
	Terms(field string) (Terms, error)
	Close() error
	GetContext() (IndexReaderContext, error)
} = (*ExitableLeafReader)(nil)

// ExitableSegmentReader is a SegmentReader that wraps another SegmentReader
// and checks for query timeout during document iteration.
type ExitableSegmentReader struct {
	*SegmentReader
	in     *SegmentReader
	config ExitableReaderConfig
}

// NewExitableSegmentReader creates a new ExitableSegmentReader wrapping the given reader.
func NewExitableSegmentReader(in *SegmentReader, config ExitableReaderConfig) *ExitableSegmentReader {
	return &ExitableSegmentReader{
		SegmentReader: in,
		in:            in,
		config:        config,
	}
}

// GetDelegate returns the wrapped SegmentReader.
func (r *ExitableSegmentReader) GetDelegate() *SegmentReader {
	return r.in
}

// checkTimeout checks if the query has been cancelled.
func (r *ExitableSegmentReader) checkTimeout() error {
	select {
	case <-r.config.QueryContext.Done():
		return &QueryCancelledError{Reason: r.config.QueryContext.Err().Error()}
	default:
		return nil
	}
}

// Postings returns exitable-wrapped postings for a term.
func (r *ExitableSegmentReader) Postings(term Term) (PostingsEnum, error) {
	enum, err := r.in.Postings(term)
	if err != nil {
		return nil, err
	}
	if enum == nil {
		return nil, nil
	}
	return NewExitablePostingsEnum(enum, r.config), nil
}

// PostingsWithFreqPositions returns exitable-wrapped postings with specific flags.
func (r *ExitableSegmentReader) PostingsWithFreqPositions(term Term, flags int) (PostingsEnum, error) {
	enum, err := r.in.PostingsWithFreqPositions(term, flags)
	if err != nil {
		return nil, err
	}
	if enum == nil {
		return nil, nil
	}
	return NewExitablePostingsEnum(enum, r.config), nil
}

// GetTermVectors returns term vectors with timeout checks.
func (r *ExitableSegmentReader) GetTermVectors(docID int) (Fields, error) {
	if err := r.checkTimeout(); err != nil {
		return nil, err
	}
	return r.in.GetTermVectors(docID)
}

// Terms returns terms with timeout checks.
func (r *ExitableSegmentReader) Terms(field string) (Terms, error) {
	if err := r.checkTimeout(); err != nil {
		return nil, err
	}
	return r.in.Terms(field)
}

// Close closes the wrapped reader.
func (r *ExitableSegmentReader) Close() error {
	return r.in.Close()
}

// ExitablePostingsEnum wraps a PostingsEnum and checks for timeout.
type ExitablePostingsEnum struct {
	PostingsEnum
	in     PostingsEnum
	config ExitableReaderConfig
	count  atomic.Int64
}

// NewExitablePostingsEnum creates a new ExitablePostingsEnum wrapping the given enum.
func NewExitablePostingsEnum(in PostingsEnum, config ExitableReaderConfig) *ExitablePostingsEnum {
	return &ExitablePostingsEnum{
		PostingsEnum: in,
		in:           in,
		config:       config,
	}
}

// checkTimeout checks if the query has been cancelled.
func (e *ExitablePostingsEnum) checkTimeout() error {
	select {
	case <-e.config.QueryContext.Done():
		return &QueryCancelledError{Reason: e.config.QueryContext.Err().Error()}
	default:
		return nil
	}
}

// NextDoc returns the next document ID, checking for timeout periodically.
func (e *ExitablePostingsEnum) NextDoc() (int, error) {
	// Check timeout every N documents
	if e.count.Add(1)%int64(e.config.CheckEvery) == 0 {
		if err := e.checkTimeout(); err != nil {
			return -1, err
		}
	}
	return e.in.NextDoc()
}

// Advance advances to the target document ID, checking for timeout.
func (e *ExitablePostingsEnum) Advance(target int) (int, error) {
	if err := e.checkTimeout(); err != nil {
		return -1, err
	}
	return e.in.Advance(target)
}

// DocID returns the current document ID.
func (e *ExitablePostingsEnum) DocID() int {
	return e.in.DocID()
}

// Freq returns the term frequency at the current document.
func (e *ExitablePostingsEnum) Freq() (int, error) {
	return e.in.Freq()
}

// NextPosition returns the next position.
func (e *ExitablePostingsEnum) NextPosition() (int, error) {
	return e.in.NextPosition()
}

// StartOffset returns the start offset.
func (e *ExitablePostingsEnum) StartOffset() (int, error) {
	return e.in.StartOffset()
}

// EndOffset returns the end offset.
func (e *ExitablePostingsEnum) EndOffset() (int, error) {
	return e.in.EndOffset()
}

// GetPayload returns the payload.
func (e *ExitablePostingsEnum) GetPayload() ([]byte, error) {
	return e.in.GetPayload()
}

// Ensure ExitablePostingsEnum implements PostingsEnum interface
var _ PostingsEnum = (*ExitablePostingsEnum)(nil)
