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

package fst

import "errors"

// ErrUnsupportedMerge indicates that the receiving Outputs implementation
// does not support the optional Merge operation. Mirrors
// java.lang.UnsupportedOperationException thrown by Outputs.merge in
// Lucene's reference implementation.
var ErrUnsupportedMerge = errors.New("fst: merge not supported by this Outputs")

// errNotVariableLengthOutput is returned when an Outputs implementation
// needs WriteVInt/WriteVLong on its DataOutput but the concrete value
// does not implement store.VariableLengthOutput.
var errNotVariableLengthOutput = errors.New("fst: DataOutput does not implement VariableLengthOutput")

// errNotVariableLengthInput is the input-side counterpart of
// errNotVariableLengthOutput.
var errNotVariableLengthInput = errors.New("fst: DataInput does not implement VariableLengthInput")
