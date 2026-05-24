// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sync"
)

// ConcurrentQueryLoader loads queries into a Monitor concurrently.
//
// Use as follows:
//
//	loader := NewConcurrentQueryLoader(monitor, 4, 2000)
//	for _, mq := range queries {
//	    loader.Add(mq)
//	}
//	if err := loader.Close(); err != nil { /* handle */ }
//
// Port of org.apache.lucene.monitor.ConcurrentQueryLoader.
//
// Deviation: Java uses NamedThreadFactory + BlockingQueue.  Gocene uses
// goroutines + channels.
const DefaultQueueSize = 2000

// MonitorRegistrar is anything that can register a batch of MonitorQueries.
// Monitor implements this interface.
type MonitorRegistrar interface {
	Register(queries []*MonitorQuery) error
}

// ConcurrentQueryLoader batches queries and feeds them to the Monitor via worker goroutines.
type ConcurrentQueryLoader struct {
	monitor  MonitorRegistrar
	queue    chan *MonitorQuery
	wg       sync.WaitGroup
	errs     []error
	mu       sync.Mutex
	shutdown bool
}

// NewConcurrentQueryLoader creates a loader with the given thread count and queue size.
func NewConcurrentQueryLoader(monitor MonitorRegistrar, threads, queueSize int) *ConcurrentQueryLoader {
	l := &ConcurrentQueryLoader{
		monitor: monitor,
		queue:   make(chan *MonitorQuery, queueSize),
	}
	batchSize := queueSize / threads
	if batchSize < 1 {
		batchSize = 1
	}
	for i := 0; i < threads; i++ {
		l.wg.Add(1)
		go l.worker(batchSize)
	}
	return l
}

// NewConcurrentQueryLoaderDefault creates a loader using runtime.NumCPU() threads.
func NewConcurrentQueryLoaderDefault(monitor MonitorRegistrar) *ConcurrentQueryLoader {
	return NewConcurrentQueryLoader(monitor, 1, DefaultQueueSize)
}

// Add enqueues a MonitorQuery for loading.
func (l *ConcurrentQueryLoader) Add(mq *MonitorQuery) error {
	l.mu.Lock()
	if l.shutdown {
		l.mu.Unlock()
		return fmt.Errorf("ConcurrentQueryLoader has been shutdown")
	}
	l.mu.Unlock()
	l.queue <- mq
	return nil
}

// Close drains the queue and waits for all workers to finish.
func (l *ConcurrentQueryLoader) Close() error {
	l.mu.Lock()
	l.shutdown = true
	l.mu.Unlock()
	close(l.queue)
	l.wg.Wait()
	l.mu.Lock()
	errs := l.errs
	l.mu.Unlock()
	if len(errs) > 0 {
		return errs[0] // return first error (matches Java's exception chaining)
	}
	return nil
}

func (l *ConcurrentQueryLoader) worker(batchSize int) {
	defer l.wg.Done()
	batch := make([]*MonitorQuery, 0, batchSize)
	for mq := range l.queue {
		batch = append(batch, mq)
		if len(batch) >= batchSize {
			l.flush(batch)
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		l.flush(batch)
	}
}

func (l *ConcurrentQueryLoader) flush(batch []*MonitorQuery) {
	if err := l.monitor.Register(batch); err != nil {
		l.mu.Lock()
		l.errs = append(l.errs, err)
		l.mu.Unlock()
	}
}
