// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package word2vec

import (
	"fmt"
	"io"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Word2VecSupportedFormats enumerates the model file formats understood
// by Word2VecSynonymProviderFactory.
type Word2VecSupportedFormats int

const (
	// Word2VecFormatDL4J selects the DeepLearning4J zip format.
	Word2VecFormatDL4J Word2VecSupportedFormats = iota
)

// Word2VecSynonymProviderFactory is a process-wide cache of
// Word2VecSynonymProvider instances, keyed by model file name. Loading a
// model is expensive (HNSW graph construction); this factory ensures
// that every Word2VecSynonymFilterFactory that references the same file
// shares a single provider.
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymProviderFactory
// from Apache Lucene 10.4.0.
//
// Deviation: Java uses a static ConcurrentHashMap with no eviction.
// Gocene preserves the same semantics using a package-level
// sync.Map so that the cache is shared across all instances within a
// process, matching the Java behaviour.
type Word2VecSynonymProviderFactory struct{}

// cache stores Word2VecSynonymProvider instances keyed by model file name.
var cache sync.Map // map[string]*Word2VecSynonymProvider

// GetSynonymProvider returns a cached (or newly built) provider for the
// model at modelFileName, loaded via loader.
func GetSynonymProvider(
	loader util.ResourceLoader,
	modelFileName string,
	format Word2VecSupportedFormats,
) (*Word2VecSynonymProvider, error) {
	if v, ok := cache.Load(modelFileName); ok {
		return v.(*Word2VecSynonymProvider), nil
	}

	rc, err := loader.OpenResource(modelFileName)
	if err != nil {
		return nil, fmt.Errorf("word2vec: open model %q: %w", modelFileName, err)
	}
	defer rc.Close() //nolint:errcheck

	reader, err := getModelReader(format, rc)
	if err != nil {
		return nil, err
	}

	model, err := reader.Read()
	if err != nil {
		return nil, err
	}

	provider, err := NewWord2VecSynonymProvider(model)
	if err != nil {
		return nil, err
	}

	// Store only if no other goroutine won the race.
	actual, _ := cache.LoadOrStore(modelFileName, provider)
	return actual.(*Word2VecSynonymProvider), nil
}

// getModelReader returns a Dl4jModelReader for the given format.
func getModelReader(format Word2VecSupportedFormats, r io.Reader) (*Dl4jModelReader, error) {
	switch format {
	case Word2VecFormatDL4J:
		return NewDl4jModelReader(r)
	default:
		return nil, fmt.Errorf("word2vec: unsupported model format %d", format)
	}
}
