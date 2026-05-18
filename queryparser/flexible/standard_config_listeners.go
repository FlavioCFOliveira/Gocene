// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "time"

// FieldBoostMapFCListener sets per-field boost values in the FieldConfig.
// This is the Go equivalent of Lucene's FieldBoostMapFCListener.
type FieldBoostMapFCListener struct {
	fieldBoostMap map[string]float32
}

// NewFieldBoostMapFCListener creates a new FieldBoostMapFCListener with
// the provided per-field boost map.
func NewFieldBoostMapFCListener(boostMap map[string]float32) *FieldBoostMapFCListener {
	if boostMap == nil {
		boostMap = make(map[string]float32)
	}
	return &FieldBoostMapFCListener{fieldBoostMap: boostMap}
}

// SetFieldBoostMap replaces the field-to-boost mapping.
func (l *FieldBoostMapFCListener) SetFieldBoostMap(m map[string]float32) {
	l.fieldBoostMap = m
}

// GetFieldBoostMap returns the current field-to-boost mapping.
func (l *FieldBoostMapFCListener) GetFieldBoostMap() map[string]float32 {
	return l.fieldBoostMap
}

// BuildFieldConfig stamps a boost value onto the FieldConfig for the field, if one exists.
func (l *FieldBoostMapFCListener) BuildFieldConfig(fieldConfig *FieldConfig) {
	if fieldConfig == nil {
		return
	}
	if boost, ok := l.fieldBoostMap[fieldConfig.GetField()]; ok {
		fieldConfig.Set(FieldConfigBoostKey, boost)
	}
}

// Ensure compile-time interface satisfaction.
var _ FieldConfigListener = (*FieldBoostMapFCListener)(nil)

// FieldConfigBoostKey is the ConfigurationKey for per-field boost values.
var FieldConfigBoostKey = NewConfigurationKey("field.boost")

// FieldDateResolutionFCListener sets the date resolution in the FieldConfig.
// This is the Go equivalent of Lucene's FieldDateResolutionFCListener.
type FieldDateResolutionFCListener struct {
	dateResolution         time.Duration
	fieldDateResolutionMap map[string]time.Duration
}

// NewFieldDateResolutionFCListener creates a new FieldDateResolutionFCListener
// with the given default date resolution.
func NewFieldDateResolutionFCListener(dateResolution time.Duration) *FieldDateResolutionFCListener {
	return &FieldDateResolutionFCListener{
		dateResolution:         dateResolution,
		fieldDateResolutionMap: make(map[string]time.Duration),
	}
}

// SetDateResolution sets the default date resolution.
func (l *FieldDateResolutionFCListener) SetDateResolution(resolution time.Duration) {
	l.dateResolution = resolution
}

// GetDateResolution returns the default date resolution.
func (l *FieldDateResolutionFCListener) GetDateResolution() time.Duration {
	return l.dateResolution
}

// SetFieldDateResolution sets a per-field date resolution.
func (l *FieldDateResolutionFCListener) SetFieldDateResolution(field string, resolution time.Duration) {
	l.fieldDateResolutionMap[field] = resolution
}

// BuildFieldConfig stamps the appropriate date resolution onto the FieldConfig.
func (l *FieldDateResolutionFCListener) BuildFieldConfig(fieldConfig *FieldConfig) {
	if fieldConfig == nil {
		return
	}
	fieldName := fieldConfig.GetField()
	if res, ok := l.fieldDateResolutionMap[fieldName]; ok {
		fieldConfig.Set(FieldConfigDateResolutionKey, res)
	} else if l.dateResolution > 0 {
		fieldConfig.Set(FieldConfigDateResolutionKey, l.dateResolution)
	}
}

// Ensure compile-time interface satisfaction.
var _ FieldConfigListener = (*FieldDateResolutionFCListener)(nil)

// FieldConfigDateResolutionKey is the ConfigurationKey for date resolution.
var FieldConfigDateResolutionKey = NewConfigurationKey("field.dateResolution")

// PointsConfigListener sets PointsConfig values in the FieldConfig.
// This is the Go equivalent of Lucene's PointsConfigListener.
type PointsConfigListener struct {
	pointsConfigMap map[string]*PointsConfig
}

// NewPointsConfigListener creates a new PointsConfigListener with the given
// field-to-PointsConfig map.
func NewPointsConfigListener(pointsConfigMap map[string]*PointsConfig) *PointsConfigListener {
	if pointsConfigMap == nil {
		pointsConfigMap = make(map[string]*PointsConfig)
	}
	return &PointsConfigListener{pointsConfigMap: pointsConfigMap}
}

// SetPointsConfigMap replaces the field-to-PointsConfig mapping.
func (l *PointsConfigListener) SetPointsConfigMap(m map[string]*PointsConfig) {
	l.pointsConfigMap = m
}

// GetPointsConfigMap returns the current field-to-PointsConfig mapping.
func (l *PointsConfigListener) GetPointsConfigMap() map[string]*PointsConfig {
	return l.pointsConfigMap
}

// BuildFieldConfig stamps a PointsConfig onto the FieldConfig for the field, if one exists.
func (l *PointsConfigListener) BuildFieldConfig(fieldConfig *FieldConfig) {
	if fieldConfig == nil {
		return
	}
	if pc, ok := l.pointsConfigMap[fieldConfig.GetField()]; ok {
		fieldConfig.Set(FieldConfigPointsConfigKey, pc)
	}
}

// Ensure compile-time interface satisfaction.
var _ FieldConfigListener = (*PointsConfigListener)(nil)

// FieldConfigPointsConfigKey is the ConfigurationKey for PointsConfig.
var FieldConfigPointsConfigKey = NewConfigurationKey("field.pointsConfig")
