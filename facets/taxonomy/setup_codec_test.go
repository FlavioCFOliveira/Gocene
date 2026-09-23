// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

// Blank-import the codecs packages so the taxonomy test binary installs the
// production codec as the default, rendering Java's classpath SPI
// registration of Codec.getDefault(). Without this, a NewIndexWriterConfig
// leaves the codec nil and IndexWriter panics with "codec must be non-null".
// The compressing package supplies the stored-fields / term-vectors formats
// that the default codec composes; both are required for a non-nil, fully
// wired default codec. This file declares no test functions; it exists solely
// for the side-effecting registration. Mirrors index/setup_codec_test.go.
import (
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
)
