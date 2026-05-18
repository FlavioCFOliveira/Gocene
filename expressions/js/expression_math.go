// Package js implements org.apache.lucene.expressions.js: the bundled
// JavaScript-syntax expression compiler.
package js

import "math"

// ExpressionMath exposes the bundled mathematical helpers available inside
// every compiled expression. Mirrors org.apache.lucene.expressions.js.ExpressionMath.

// Pow returns base raised to the supplied exponent.
func Pow(base, exp float64) float64 { return math.Pow(base, exp) }

// Log10 returns the base-10 logarithm.
func Log10(x float64) float64 { return math.Log10(x) }

// Log2 returns the base-2 logarithm.
func Log2(x float64) float64 { return math.Log2(x) }

// Sqrt returns the square root.
func Sqrt(x float64) float64 { return math.Sqrt(x) }

// Exp returns e raised to x.
func Exp(x float64) float64 { return math.Exp(x) }

// Sin / Cos / Tan are the trigonometric helpers.
func Sin(x float64) float64 { return math.Sin(x) }
func Cos(x float64) float64 { return math.Cos(x) }
func Tan(x float64) float64 { return math.Tan(x) }

// Abs returns |x|.
func Abs(x float64) float64 { return math.Abs(x) }

// Ceil / Floor / Round mirror their JavaScript equivalents.
func Ceil(x float64) float64  { return math.Ceil(x) }
func Floor(x float64) float64 { return math.Floor(x) }
func Round(x float64) float64 { return math.Round(x) }

// Max / Min are the two-argument variants used inside expressions.
func Max(a, b float64) float64 { return math.Max(a, b) }
func Min(a, b float64) float64 { return math.Min(a, b) }
