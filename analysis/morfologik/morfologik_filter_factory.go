// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/morfologik/src/java/org/apache/lucene/analysis/morfologik/MorfologikFilterFactory.java

package morfologik

import (
	"errors"
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DictionaryLoader can load a [Dictionary] from a pair of resource streams
// identified by a base name (e.g. "mylang.dict"). Implementations are
// responsible for resolving the companion metadata file
// (conventionally "<name>.info" or "<name>.meta") from the same loader.
//
// This interface replaces the Java-side Dictionary.read(InputStream, InputStream)
// call, which cannot be ported directly because Go has no morfologik FSA
// binary reader.
type DictionaryLoader interface {
	// LoadDictionary loads the dictionary identified by resourceName from
	// loader, returning a ready-to-use [Dictionary].
	LoadDictionary(loader util.ResourceLoader, resourceName string) (Dictionary, error)
}

// defaultDictionaryLoader is the nil singleton used when no custom loader is
// provided. It returns an error on every call, mirroring the Java side where a
// missing dictionary binary causes a ClassNotFoundException at runtime.
type defaultDictionaryLoader struct{}

func (defaultDictionaryLoader) LoadDictionary(_ util.ResourceLoader, name string) (Dictionary, error) {
	return nil, fmt.Errorf("no DictionaryLoader registered; cannot load %q (no Go morfologik binary available)", name)
}

// MorfologikFilterFactory is a [analysis.TokenFilterFactory] that creates
// [MorfologikFilter] instances.
//
// An optional resource name of the dictionary (".dict") can be provided via
// the "dictionary" configuration key. When omitted the factory's default
// dictionary — set by [MorfologikFilterFactory.SetDefaultDictionary] — is
// used. If neither is available, [Inform] returns an error.
//
// This is the Go port of
// org.apache.lucene.analysis.morfologik.MorfologikFilterFactory
// (Apache Lucene 10.4.0).
//
// # Deviation from Java
//
// The Java factory loads a binary Morfologik FSA dictionary via
// Dictionary.read(). Go has no morfologik FSA reader, so the binary loading
// is delegated to the optional [DictionaryLoader] interface. Callers that
// need real dictionary support must inject a DictionaryLoader via
// [MorfologikFilterFactory.SetDictionaryLoader]. The default loader always
// returns an error.
//
// An alternative to using a DictionaryLoader is to call
// [MorfologikFilterFactory.SetDefaultDictionary] with a pre-built [Dictionary]
// (e.g. for the Polish built-in), which is used when no "dictionary" attribute
// is supplied.
type MorfologikFilterFactory struct {
	// NAME is the SPI name for this factory.

	// resourceName is the value of the "dictionary" configuration key.
	resourceName string

	// dictionary is the resolved Dictionary, populated by Inform.
	dictionary Dictionary

	// dictLoader is the loader used when resourceName != "".
	dictLoader DictionaryLoader

	// args holds any unused configuration keys detected during construction.
	unusedArgs []string
}

// NAME is the SPI name for MorfologikFilterFactory.
const MorfologikFilterFactoryName = "morfologik"

// DictionaryAttribute is the configuration key for the dictionary resource name.
const DictionaryAttribute = "dictionary"

// NewMorfologikFilterFactory constructs a factory from a string-keyed
// configuration map, mirroring the Java constructor. Only the
// [DictionaryAttribute] key is consumed; any remaining keys are recorded and
// cause the constructor to return an error, matching the Java behaviour.
func NewMorfologikFilterFactory(args map[string]string) (*MorfologikFilterFactory, error) {
	// Reject the legacy "dictionary-resource" attribute.
	const legacyAttr = "dictionary-resource"
	if v, ok := args[legacyAttr]; ok && v != "" {
		return nil, fmt.Errorf(
			"the %q attribute is no longer supported; use %q instead (see LUCENE-6833)",
			legacyAttr, DictionaryAttribute,
		)
	}
	delete(args, legacyAttr)

	f := &MorfologikFilterFactory{
		dictLoader: defaultDictionaryLoader{},
	}
	f.resourceName = args[DictionaryAttribute]
	delete(args, DictionaryAttribute)

	if len(args) > 0 {
		unknown := make([]string, 0, len(args))
		for k := range args {
			unknown = append(unknown, k)
		}
		return nil, fmt.Errorf("unknown parameters: %s", strings.Join(unknown, ", "))
	}

	return f, nil
}

// SetDictionaryLoader replaces the [DictionaryLoader] used when a resource
// name is configured. Call this before [Inform].
func (f *MorfologikFilterFactory) SetDictionaryLoader(loader DictionaryLoader) {
	f.dictLoader = loader
}

// SetDefaultDictionary sets the dictionary to use when no "dictionary"
// resource name was provided. Call this before [Inform].
func (f *MorfologikFilterFactory) SetDefaultDictionary(d Dictionary) {
	// Only applies when no explicit resourceName was given.
	if f.resourceName == "" {
		f.dictionary = d
	}
}

// Inform implements [util.ResourceLoaderAware]. It loads the dictionary
// identified by the "dictionary" configuration attribute, or uses the default
// dictionary set by [MorfologikFilterFactory.SetDefaultDictionary].
func (f *MorfologikFilterFactory) Inform(loader util.ResourceLoader) error {
	if f.resourceName == "" {
		// Use default dictionary if set.
		if f.dictionary == nil {
			return errors.New("MorfologikFilterFactory: no dictionary configured and no default dictionary set")
		}
		return nil
	}
	d, err := f.dictLoader.LoadDictionary(loader, f.resourceName)
	if err != nil {
		return fmt.Errorf("MorfologikFilterFactory: %w", err)
	}
	f.dictionary = d
	return nil
}

// Create wraps ts with a [MorfologikFilter] using the loaded dictionary.
// [Inform] must have been called successfully before Create.
func (f *MorfologikFilterFactory) Create(ts analysis.TokenStream) analysis.TokenFilter {
	if f.dictionary == nil {
		panic("MorfologikFilterFactory was not fully initialized: call Inform before Create")
	}
	return NewMorfologikFilter(ts, f.dictionary.NewStemmer())
}

// Ensure MorfologikFilterFactory implements analysis.TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*MorfologikFilterFactory)(nil)

// Ensure MorfologikFilterFactory implements util.ResourceLoaderAware.
var _ util.ResourceLoaderAware = (*MorfologikFilterFactory)(nil)
