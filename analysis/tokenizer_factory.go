package analysis

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TokenizerFactory is the Go port of org.apache.lucene.analysis.TokenizerFactory.
// It is an interface for factories that create Tokenizer instances.
type TokenizerFactory interface {
	// Create creates a Tokenizer using the given AttributeFactory.
	Create(factory util.AttributeFactory) Tokenizer
}

// BaseAnalysisFactory is the Go port of org.apache.lucene.analysis.AbstractAnalysisFactory.
// It provides common functionality for analysis factories, such as argument parsing
// and Lucene version matching.
type BaseAnalysisFactory struct {
	originalArgs       map[string]string
	luceneMatchVersion *util.Version
}

// NewBaseAnalysisFactory creates a new BaseAnalysisFactory and initializes the Lucene match version.
func NewBaseAnalysisFactory(args map[string]string) *BaseAnalysisFactory {
	originalArgs := make(map[string]string, len(args))
	for k, v := range args {
		originalArgs[k] = v
	}

	versionStr := args["luceneMatchVersion"]
	var version *util.Version
	if versionStr == "" {
		version = util.Latest
	} else {
		var err error
		version, err = util.Parse(versionStr)
		if err != nil {
			panic(fmt.Sprintf("IllegalArgumentException: %v", err))
		}
	}

	return &BaseAnalysisFactory{
		originalArgs:       originalArgs,
		luceneMatchVersion: version,
	}
}

// GetOriginalArgs returns the original arguments used to initialize the factory.
func (b *BaseAnalysisFactory) GetOriginalArgs() map[string]string {
	return b.originalArgs
}

// GetLuceneMatchVersion returns the Lucene version this factory is matched to.
func (b *BaseAnalysisFactory) GetLuceneMatchVersion() *util.Version {
	return b.luceneMatchVersion
}

// Require returns the value of the specified argument or panics if it is missing.
func (b *BaseAnalysisFactory) Require(args map[string]string, name string) string {
	s, ok := args[name]
	if !ok {
		panic(fmt.Sprintf("Configuration Error: missing parameter '%s'", name))
	}
	delete(args, name)
	return s
}

// Get returns the value of the specified argument, or a default value if it is missing.
func (b *BaseAnalysisFactory) Get(args map[string]string, name string, defaultVal string) string {
	s, ok := args[name]
	if !ok {
		return defaultVal
	}
	delete(args, name)
	return s
}

// --- SPI Registry ---

var (
	tokenizerRegistry = make(map[string]func(map[string]string) TokenizerFactory)
	registryMu       sync.RWMutex
)

// RegisterTokenizerFactory registers a tokenizer factory creator.
func RegisterTokenizerFactory(name string, creator func(map[string]string) TokenizerFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	tokenizerRegistry[name] = creator
}

// ForName looks up a tokenizer by name from the registry.
// This is the Go port of TokenizerFactory.forName.
func ForName(name string, args map[string]string) (TokenizerFactory, error) {
	registryMu.RLock()
	creator, ok := tokenizerRegistry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tokenizer factory not found: %s", name)
	}
	return creator(args), nil
}

// AvailableTokenizers returns a list of all available tokenizer names.
// This is the Go port of TokenizerFactory.availableTokenizers.
func AvailableTokenizers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(tokenizerRegistry))
	for name := range tokenizerRegistry {
		names = append(names, name)
	}
	return names
}

// CreateDefaultTokenizer is a helper that creates a Tokenizer using the default attribute factory.
// This mirrors the no-arg create() method in Java's TokenizerFactory.
func CreateDefaultTokenizer(f TokenizerFactory) Tokenizer {
	return f.Create(util.DefaultAttributeFactoryInstance)
}
