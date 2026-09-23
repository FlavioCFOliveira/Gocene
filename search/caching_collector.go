// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
// with this license.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// CachingCollector caches all docs, and optionally also scores, coming from a search,
// and is then able to replay them to another collector.
type CachingCollector interface {
	Collector
	IsCached() bool
	Replay(other Collector) error
}

type cachedScorable struct {
	BaseScorable
	score float32
}

func (s *cachedScorable) Score() (float32, error) {
	return s.score, nil
}

type cachingCollectorBase struct {
	in       Collector
	cached   bool
	contexts []*index.LeafReaderContext
	docs     [][]int
}

func (c *cachingCollectorBase) SetWeight(weight Weight) {
	c.in.SetWeight(weight)
}

type noScoreCachingCollector struct {
	cachingCollectorBase
	maxDocsToCache int
}

func (c *noScoreCachingCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	in, err := c.in.GetLeafCollector(context)
	if err != nil {
		return nil, err
	}
	if c.maxDocsToCache >= 0 {
		if c.contexts != nil {
			c.contexts = append(c.contexts, context)
		}
		return &noScoreCachingLeafCollector{
			in:             in,
			maxDocsToCache: c.maxDocsToCache,
			collector:      c,
			docs:           make([]int, 128),
			docCount:       0,
		}, nil
	}
	return in, nil
}

func (c *noScoreCachingCollector) ScoreMode() ScoreMode {
	return c.in.ScoreMode()
}

func (c *noScoreCachingCollector) IsCached() bool {
	return c.cached
}

func (c *noScoreCachingCollector) Replay(other Collector) error {
	if !c.cached {
		return fmt.Errorf("cannot replay: cache was cleared because too much RAM was required")
	}
	for i, context := range c.contexts {
		collector, err := other.GetLeafCollector(context)
		if err != nil {
			return err
		}
		docs := c.docs[i]
		for _, doc := range docs {
			if err := collector.Collect(doc); err != nil {
				return err
			}
		}
		if err := collector.Finish(); err != nil {
			return err
		}
	}
	return nil
}

func (c *noScoreCachingCollector) invalidate() {
	c.maxDocsToCache = -1
	c.contexts = nil
	c.docs = nil
}

type scoreCachingCollector struct {
	noScoreCachingCollector
	scores [][]float32
}

func (c *scoreCachingCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	in, err := c.in.GetLeafCollector(context)
	if err != nil {
		return nil, err
	}
	if c.maxDocsToCache >= 0 {
		if c.contexts != nil {
			c.contexts = append(c.contexts, context)
		}
		return &scoreCachingLeafCollector{
			noScoreCachingLeafCollector: noScoreCachingLeafCollector{
				in:             in,
				maxDocsToCache: c.maxDocsToCache,
				collector:      &c.noScoreCachingCollector,
				docs:           make([]int, 128),
				docCount:       0,
			},
			scoreCollector: c,
			scores:         make([]float32, 128),
		}, nil
	}
	return in, nil
}

func (c *scoreCachingCollector) ScoreMode() ScoreMode {
	return COMPLETE
}

func (c *scoreCachingCollector) Replay(other Collector) error {
	if !c.cached {
		return fmt.Errorf("cannot replay: cache was cleared because too much RAM was required")
	}
	for i, context := range c.contexts {
		collector, err := other.GetLeafCollector(context)
		if err != nil {
			return err
		}
		docs := c.docs[i]
		scores := c.scores[i]
		scorable := &cachedScorable{score: 0}
		if err := collector.SetScorer(scorable); err != nil {
			return err
		}
		for j, doc := range docs {
			scorable.score = scores[j]
			if err := collector.Collect(doc); err != nil {
				return err
			}
		}
		if err := collector.Finish(); err != nil {
			return err
		}
	}
	return nil
}

type noScoreCachingLeafCollector struct {
	in             LeafCollector
	maxDocsToCache int
	collector      *noScoreCachingCollector
	docs           []int
	docCount       int
}

func (l *noScoreCachingLeafCollector) SetScorer(scorer Scorable) error {
	return l.in.SetScorer(scorer)
}

func (l *noScoreCachingLeafCollector) Collect(doc int) error {
	if l.docs != nil {
		if l.docCount >= len(l.docs) {
			if l.docCount >= l.maxDocsToCache {
				l.collector.invalidate()
			} else {
				newLen := l.docCount + 1
				newLen = (newLen + (newLen >> 2))
				if newLen > l.maxDocsToCache {
					newLen = l.maxDocsToCache
				}
				newDocs := make([]int, newLen)
				copy(newDocs, l.docs)
				l.docs = newDocs
			}
		}
		if l.docs != nil {
			l.docs[l.docCount] = doc
			l.docCount++
		}
	}
	return l.in.Collect(doc)
}

func (l *noScoreCachingLeafCollector) CollectRange(min, max int) error {
	return l.in.CollectRange(min, max)
}

func (l *noScoreCachingLeafCollector) CollectStream(stream DocIdStream) error {
	return l.in.CollectStream(stream)
}

func (l *noScoreCachingLeafCollector) CompetitiveIterator() (DocIdSetIterator, error) {
	return l.in.CompetitiveIterator()
}

func (l *noScoreCachingLeafCollector) Finish() error {
	if l.docs != nil {
		docs := make([]int, l.docCount)
		copy(docs, l.docs[:l.docCount])
		l.collector.maxDocsToCache -= len(docs)
		l.collector.docs = append(l.collector.docs, docs)
	} else {
		l.collector.invalidate()
	}
	return l.in.Finish()
}

type scoreCachingLeafCollector struct {
	noScoreCachingLeafCollector
	// scoreCollector is the same object as the embedded base's collector
	// field, held at its concrete type. Java's ScoreCachingLeafCollector
	// stores only the NoScoreCachingCollector reference inherited from its
	// superclass and recovers the subclass in postCollect with the downcast
	// ((ScoreCachingCollector) collector). Go has no downcast from an
	// embedded base, so the concrete reference is kept alongside; the
	// constructor is the only writer, exactly as the Java cast is guaranteed
	// by ScoreCachingCollector.wrap being its sole caller.
	scoreCollector *scoreCachingCollector
	scorer         Scorable
	scores         []float32
}

func (l *scoreCachingLeafCollector) SetScorer(scorer Scorable) error {
	l.scorer = scorer
	return l.in.SetScorer(scorer)
}

func (l *scoreCachingLeafCollector) Collect(doc int) error {
	if l.docs != nil {
		if l.docCount >= len(l.docs) {
			if l.docCount >= l.maxDocsToCache {
				l.collector.invalidate()
			} else {
				newLen := l.docCount + 1
				newLen = (newLen + (newLen >> 2))
				if newLen > l.maxDocsToCache {
					newLen = l.maxDocsToCache
				}
				newDocs := make([]int, newLen)
				copy(newDocs, l.docs)
				l.docs = newDocs

				if l.scores != nil {
					newScores := make([]float32, newLen)
					copy(newScores, l.scores)
					l.scores = newScores
				}
			}
		}
		if l.docs != nil {
			if l.scores != nil && l.scorer != nil {
				sc0, err := l.scorer.Score()
				if err != nil {
					return err
				}
				l.scores[l.docCount] = sc0
			}
			l.docs[l.docCount] = doc
			l.docCount++
		}
	}
	return l.in.Collect(doc)
}

func (l *scoreCachingLeafCollector) Finish() error {
	if l.docs != nil {
		docs := make([]int, l.docCount)
		copy(docs, l.docs[:l.docCount])

		var scores []float32
		if l.scores != nil {
			scores = make([]float32, l.docCount)
			copy(scores, l.scores[:l.docCount])
		}

		l.collector.maxDocsToCache -= len(docs)
		l.collector.docs = append(l.collector.docs, docs)

		// Mirrors ScoreCachingLeafCollector.postCollect:
		// ((ScoreCachingCollector) collector).scores.add(cachedScores()).
		l.scoreCollector.scores = append(l.scoreCollector.scores, scores)
	} else {
		l.collector.invalidate()
	}
	return l.in.Finish()
}

// CreateCachingCollector creates a CachingCollector which does not wrap
// another collector. The cached documents and scores can later be replayed.
//
// Mirrors the static CachingCollector.create(boolean cacheScores, double
// maxRAMMB) of Apache Lucene 10.5.0: the wrapped collector is an anonymous
// SimpleCollector whose collect(int) does nothing and whose scoreMode() is
// COMPLETE.
func CreateCachingCollector(cacheScores bool, maxRAMMB float64) CachingCollector {
	other := &noOpCompleteCollector{}
	other.Outer = other
	return CreateCachingCollectorWithOther(other, cacheScores, maxRAMMB)
}

// noOpCompleteCollector is the anonymous SimpleCollector of
// CachingCollector.create(boolean, double).
type noOpCompleteCollector struct {
	BaseSimpleCollector
	BaseLeafCollector
}

func (c *noOpCompleteCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	return c, nil
}

// Collect renders collect(int), which does nothing.
func (c *noOpCompleteCollector) Collect(doc int) error { return nil }

// ScoreMode renders scoreMode(): COMPLETE.
func (c *noOpCompleteCollector) ScoreMode() ScoreMode { return COMPLETE }

func (c *noOpCompleteCollector) CollectRange(min, max int) error {
	return DefaultCollectRange(c, min, max)
}

func (c *noOpCompleteCollector) CollectStream(stream DocIdStream) error {
	return DefaultCollectStream(c, stream)
}

// CreateCachingCollectorWithOther creates a CachingCollector that wraps the
// given collector and caches documents and scores up to the specified RAM
// threshold.
//
// Mirrors the static CachingCollector.create(Collector other, boolean
// cacheScores, double maxRAMMB) of Apache Lucene 10.5.0.
func CreateCachingCollectorWithOther(other Collector, cacheScores bool, maxRAMMB float64) CachingCollector {
	bytesPerDoc := 4
	if cacheScores {
		bytesPerDoc += 4
	}
	maxDocsToCache := int((maxRAMMB * 1024 * 1024) / float64(bytesPerDoc))
	return CreateCachingCollectorWithOtherInt(other, cacheScores, maxDocsToCache)
}

// CreateCachingCollectorWithOtherInt creates a new CachingCollector that
// wraps the given collector and caches documents and scores up to the
// specified max docs threshold.
//
// Mirrors the static CachingCollector.create(Collector other, boolean
// cacheScores, int maxDocsToCache) of Apache Lucene 10.5.0; the int overload
// of create(Collector, boolean, double) carries the type suffix.
func CreateCachingCollectorWithOtherInt(other Collector, cacheScores bool, maxDocsToCache int) CachingCollector {
	if cacheScores {
		return &scoreCachingCollector{
			noScoreCachingCollector: noScoreCachingCollector{
				cachingCollectorBase: cachingCollectorBase{
					in:     other,
					cached: true,
				},
				maxDocsToCache: maxDocsToCache,
			},
		}
	}
	return &noScoreCachingCollector{
		cachingCollectorBase: cachingCollectorBase{
			in:     other,
			cached: true,
		},
		maxDocsToCache: maxDocsToCache,
	}
}
