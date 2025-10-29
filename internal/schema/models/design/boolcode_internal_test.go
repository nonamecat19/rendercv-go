package design

import (
	"math"
	"strconv"
	"testing"
)

// TestBoolCodeIsBoundedByInt64 pins which of the two boolean errors a number
// carries, at the boundary that decides it.
//
// pydantic-core converts a float through an `i64` before asking whether it is
// 0 or 1, so a value too large to be one takes `bool_type` rather than
// `bool_parsing` even though it is mathematically whole. Measured against
// pydantic on both sides: `9.2e18` is `bool_parsing`, `9.3e18` is `bool_type`,
// and so are `1e308` and an infinity.
//
// Reachable from an ordinary document since the reader began resolving an
// overflowing literal as the float it is rather than as a string.
func TestBoolCodeIsBoundedByInt64(t *testing.T) {
	tests := []struct {
		value float64
		whole bool
	}{
		{value: 2, whole: true},
		{value: -2, whole: true},
		{value: 1e18, whole: true},
		{value: 9.2e18, whole: true},
		{value: 9.3e18, whole: false},
		{value: 1e19, whole: false},
		{value: 1e308, whole: false},
		{value: math.Inf(1), whole: false},
		{value: math.Inf(-1), whole: false},
		{value: math.NaN(), whole: false},
		{value: 1.5, whole: false},
	}

	for _, test := range tests {
		t.Run(strconv.FormatFloat(test.value, 'g', -1, 64), func(t *testing.T) {
			if got := isWholeNumber(test.value); got != test.whole {
				t.Errorf("isWholeNumber(%v) = %v, want %v", test.value, got, test.whole)
			}
		})
	}
}
