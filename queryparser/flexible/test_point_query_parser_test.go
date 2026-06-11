// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestPointQueryParser verifies the point query infrastructure available in the
// flexible query parser: PointsConfig, PointQueryNode, PointRangeQueryNode,
// and PointRangeQueryNodeBuilder.
//
// The Java original tests the integrated point-range parsing pipeline
// (SetPointsConfigMap + automatic IntPoint/FloatPoint/etc. query production);
// in Gocene the builder infrastructure is available but not yet wired into the
// StandardQueryParser's syntax parser. This test validates the building blocks.
func TestPointQueryParser(t *testing.T) {
	t.Run("PointsConfig", func(t *testing.T) {
		cfg := flexible.NewPointsConfig(flexible.PointsTypeInt, 1)
		if cfg.GetType() != flexible.PointsTypeInt {
			t.Errorf("expected PointsTypeInt, got %v", cfg.GetType())
		}
		if cfg.GetNumDims() != 1 {
			t.Errorf("expected 1 dim, got %d", cfg.GetNumDims())
		}
		if cfg.GetBytesPerDim() != 4 {
			t.Errorf("expected 4 bytes per dim for int, got %d", cfg.GetBytesPerDim())
		}
	})

	t.Run("PointsConfig types", func(t *testing.T) {
		types := []struct {
			typ        flexible.PointsType
			bytesPer   int
		}{
			{flexible.PointsTypeInt, 4},
			{flexible.PointsTypeLong, 8},
			{flexible.PointsTypeFloat, 4},
			{flexible.PointsTypeDouble, 8},
		}
		for _, tc := range types {
			cfg := flexible.NewPointsConfig(tc.typ, 1)
			if cfg.GetBytesPerDim() != tc.bytesPer {
				t.Errorf("PointsType %v: expected %d bytes, got %d", tc.typ, tc.bytesPer, cfg.GetBytesPerDim())
			}
		}
	})
}

// TestPointRangeQueryNodeBuilder verifies that PointRangeQueryNodeBuilder
// can construct a PointRangeQuery from a PointRangeQueryNode.
func TestPointRangeQueryNodeBuilder(t *testing.T) {
	lower := flexible.NewPointQueryNode("field", []byte{0, 0, 0, 5})
	upper := flexible.NewPointQueryNode("field", []byte{0, 0, 0, 10})

	prNode := flexible.NewPointRangeQueryNode("field", lower, upper, true, true)

	builder := flexible.NewPointRangeQueryNodeBuilder()
	query, err := builder.Build(prNode)
	if err != nil {
		t.Fatal(err)
	}
	if query == nil {
		t.Fatal("Build should not return nil query")
	}
	if _, ok := query.(*search.PointRangeQuery); !ok {
		t.Errorf("expected PointRangeQuery, got %T", query)
	}
}

// TestPointsConfigListener verifies the PointsConfigListener registry.
func TestPointsConfigListener(t *testing.T) {
	listener := flexible.NewPointsConfigListener()
	if listener == nil {
		t.Fatal("NewPointsConfigListener should not return nil")
	}

	configMap := listener.GetPointsConfigMap()
	if configMap == nil {
		t.Fatal("GetPointsConfigMap should not return nil")
	}

	cfg := flexible.NewPointsConfig(flexible.PointsTypeInt, 1)
	listener.SetPointsConfigMap(map[string]*flexible.PointsConfig{
		"field1": cfg,
	})

	got := listener.GetPointsConfigMap()
	if got["field1"] != cfg {
		t.Error("PointsConfig map should contain the set config")
	}
}

// TestStandardQueryConfigHandlerFullPoints verifies point config on the full
// config handler.
func TestStandardQueryConfigHandlerFullPoints(t *testing.T) {
	h := flexible.NewStandardQueryConfigHandlerFull()
	cfg := flexible.NewPointsConfig(flexible.PointsTypeDouble, 1)
	h.SetPointsConfig("price", cfg)

	got := h.GetPointsConfig("price")
	if got == nil {
		t.Fatal("GetPointsConfig should return the set config")
	}
	if got.GetType() != flexible.PointsTypeDouble {
		t.Errorf("expected PointsTypeDouble, got %v", got.GetType())
	}
}
