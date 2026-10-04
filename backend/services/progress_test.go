// Package services_test exercises the pure business logic that needs no
// database: the estimated-1RM formula, default naming, and the password and
// email rules.
package services_test

import (
	"testing"

	"github.com/jackpatchell/workout-tracker/backend/services"
)

func TestEstimated1RM(t *testing.T) {
	// These values are also asserted in the frontend suite and in the SQL PR
	// detection. All three must agree, or a user sees one number while logging
	// and another on the chart.
	cases := []struct {
		weight float64
		reps   int
		want   float64
	}{
		{405, 3, 445.5},
		{385, 5, 449.1666666667},
		{415, 3, 456.5},
		{120, 10, 160},
		{225, 1, 232.5},
	}

	for _, tc := range cases {
		got := services.Estimated1RM(tc.weight, tc.reps)
		if diff := got - tc.want; diff > 0.001 || diff < -0.001 {
			t.Errorf("Estimated1RM(%v, %d) = %v, want %v", tc.weight, tc.reps, got, tc.want)
		}
	}
}

func TestEstimated1RMZeroRepsDoesNotPanic(t *testing.T) {
	// A set can't have zero reps (validation rejects it), but the exported
	// helper should still be total rather than producing NaN.
	if got := services.Estimated1RM(225, 0); got != 225 {
		t.Errorf("Estimated1RM(225, 0) = %v, want 225", got)
	}
}

func TestSetVolume(t *testing.T) {
	if got := services.SetVolume(405, 3); got != 1215 {
		t.Errorf("SetVolume(405, 3) = %v, want 1215", got)
	}
	if got := services.SetVolume(0, 10); got != 0 {
		t.Errorf("SetVolume(0, 10) = %v, want 0", got)
	}
}
