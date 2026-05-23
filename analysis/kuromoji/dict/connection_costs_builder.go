// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

// ConnectionCostsBuilder builds a ConnectionCosts matrix from a matrix.def
// source file.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.ConnectionCostsBuilder from Apache
// Lucene 10.4.0.
type ConnectionCostsBuilder struct{}

// Build reads a matrix.def-format reader and returns a ConnectionCosts matrix.
//
// The format is:
//
//	<forwardSize> <backwardSize>
//	<forwardID> <backwardID> <cost>
//	...
func (ConnectionCostsBuilder) Build(r io.Reader) (*morph.ConnectionCosts, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return nil, fmt.Errorf("connectionCostsBuilder: empty input")
	}
	dims := strings.Fields(scanner.Text())
	if len(dims) < 2 {
		return nil, fmt.Errorf("connectionCostsBuilder: bad header: %s", scanner.Text())
	}
	fwd, err := strconv.Atoi(dims[0])
	if err != nil {
		return nil, err
	}
	bwd, err := strconv.Atoi(dims[1])
	if err != nil {
		return nil, err
	}
	w := morph.NewConnectionCostsWriter(fwd, bwd)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		f, _ := strconv.Atoi(fields[0])
		b, _ := strconv.Atoi(fields[1])
		cost, _ := strconv.Atoi(fields[2])
		w.SetCost(f, b, int16(cost))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return w.Build(), nil
}
