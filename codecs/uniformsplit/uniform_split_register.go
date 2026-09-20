// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import "github.com/FlavioCFOliveira/Gocene/codecs"

// init publishes UniformSplitPostingsFormat in the global PostingsFormat
// registry under its Lucene name, "UniformSplit".
//
// Mirrors the ServiceLoader entry
// org.apache.lucene.codecs.uniformsplit.UniformSplitPostingsFormat in
// lucene/codecs/src/resources/META-INF/services/org.apache.lucene.codecs.PostingsFormat
//
// Java's ServiceLoader instantiates the no-argument constructor and raises a
// ServiceConfigurationError if it fails, so a failure here is fatal in the
// same way. The default settings are valid, so this cannot trigger in practice.
func init() {
	format, err := NewUniformSplitPostingsFormat()
	if err != nil {
		panic("codecs/uniformsplit: failed to create UniformSplitPostingsFormat: " + err.Error())
	}
	codecs.RegisterPostingsFormat(format)
}
