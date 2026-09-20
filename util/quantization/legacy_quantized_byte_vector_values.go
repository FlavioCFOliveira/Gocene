// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package quantization

// LegacyQuantizedByteVectorValues is the Go port of
// org.apache.lucene.util.quantization.LegacyQuantizedByteVectorValues
// (Lucene 10.5.0): a [BaseQuantizedByteVectorValues] that additionally
// retrieves the score correction offset for scalar quantization scores.
//
// The Java abstract class carries two default bodies, which implementers
// that do not override them reproduce: getScalarQuantizer() throws
// UnsupportedOperationException (a panic, since the method declares no
// IOException), and copy() returns this. The covariant copy() override is
// rendered as CopyLegacyQuantizedByteVectorValues.
type LegacyQuantizedByteVectorValues interface {
	BaseQuantizedByteVectorValues

	// GetScalarQuantizer returns the quantizer that produced the stored
	// vectors. Mirrors getScalarQuantizer().
	GetScalarQuantizer() *ScalarQuantizer

	// GetScoreCorrectionConstant returns the score correction constant of
	// the vector at the given ordinal. Mirrors the abstract
	// getScoreCorrectionConstant(int).
	GetScoreCorrectionConstant(ord int) (float32, error)

	// CopyLegacyQuantizedByteVectorValues is the covariant copy() override.
	CopyLegacyQuantizedByteVectorValues() (LegacyQuantizedByteVectorValues, error)
}
