// Package formulas_test verifies that the Go implementation of each estimated-1RM
// formula and its SQL counterpart produce the same numbers.
//
// This is the test that makes it safe to hold a formula in two languages. If the
// Go path (used for the optimistic preview while logging a set) and the SQL path
// (used for the aggregates behind PR flags and the progress charts) ever disagree,
// a PR badge would be computed one way while the chart displayed the other — a
// bug that is essentially invisible in review and very confusing in use.
//
// The SQL side is exercised through a real database, so it also verifies the
// expression actually evaluates as intended against Postgres.
package formulas_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/jackpatchell/workout-tracker/backend/formulas"
	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// tolerance is generous enough to absorb float8 rounding but far tighter than any
// difference that would matter to a user reading a number off a chart.
const tolerance = 0.01

func sqlURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/workout_tracker?sslmode=disable"
}

// sqlEstimate1RM evaluates the formula in SQL for one input.
func sqlEstimate1RM(ctx context.Context, db *tables.DB, f formulas.Formula, weight float64, reps int, rpe *float64) (float64, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM (SELECT $1::float8 AS weight, $2::float8 AS reps, $3::float8 AS rpe) s`,
		f.SQLFor("s"))

	var out float64
	err := db.Pool.QueryRow(ctx, query, weight, reps, rpe).Scan(&out)
	return out, err
}

func TestFormulaImplementationsAgree(t *testing.T) {
	db, err := tables.Connect(context.Background(), sqlURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()

	// A grid spanning realistic weights, the full RPE range including values
	// outside the 0-10 scale, and rep counts on both sides of the polynomial's
	// useful range.
	weights := []float64{45, 95, 135, 225, 315, 405}
	repsList := []int{1, 3, 5, 8, 10, 12, 20, 30}
	rpes := []*float64{
		nil, // exercises the "blank means 10" rule
		fptr(6), fptr(6.5), fptr(7), fptr(7.5), fptr(8), fptr(8.5),
		fptr(9), fptr(9.5), fptr(10), fptr(0), fptr(10.5), // outside the scale
	}

	for _, f := range []formulas.Formula{formulas.Epley, formulas.RTSPolynomial} {
		f := f
		t.Run(string(f), func(t *testing.T) {
			for _, w := range weights {
				for _, reps := range repsList {
					for _, rpe := range rpes {
						gotSQL, err := sqlEstimate1RM(ctx, db, f, w, reps, rpe)
						if err != nil {
							t.Fatalf("sql w=%v reps=%d rpe=%v: %v", w, reps, rpe, err)
						}

						gotGo := f.Estimate1RM(w, reps, rpe)
						if diff := gotGo - gotSQL; diff > tolerance || diff < -tolerance {
							label := "nil"
							if rpe != nil {
								label = fmt.Sprintf("%g", *rpe)
							}
							t.Errorf("weight=%v reps=%d rpe=%s: Go=%.4f SQL=%.4f (diff %.4f)",
								w, reps, label, gotGo, gotSQL, diff)
						}
					}
				}
			}
		})
	}
}

// TestRTSTotalRepsBridging pins the step that folds RPE into the polynomial, since
// getting it wrong silently produces plausible but incorrect numbers.
func TestRTSTotalRepsBridging(t *testing.T) {
	cases := []struct {
		reps int
		rpe  *float64
		want float64
	}{
		{5, fptr(8), 7},  // 5 + (10-8)
		{5, fptr(10), 5}, // no reserve
		{5, fptr(6), 9},  // four in reserve
		{1, fptr(10), 1},
		{5, nil, 5}, // blank RPE == RPE 10 == no reserve
	}
	for _, tc := range cases {
		if got := formulas.TotalReps(tc.reps, tc.rpe); got != tc.want {
			t.Errorf("TotalReps(%d, %v) = %v, want %v", tc.reps, tc.rpe, got, tc.want)
		}
	}
}

// TestRTSMatchesWorkedExample checks the formula against the worked example from
// the RTS documentation: 200 lb x 5 at RPE 8 is ~83.7% of a 1RM, giving ~239 lb.
func TestRTSMatchesWorkedExample(t *testing.T) {
	got := formulas.RTSPercentage(formulas.TotalReps(5, fptr(8)))
	if math.Abs(got-0.8371) > 0.0001 {
		t.Errorf("5 reps @ RPE 8 = %.4f of 1RM, want 0.8371", got)
	}

	e1rm := formulas.RTSPolynomial.Estimate1RM(200, 5, fptr(8))
	if math.Abs(e1rm-238.9) > 0.5 {
		t.Errorf("200x5 @ RPE 8 = %.1f, want ~238.9", e1rm)
	}
}

// TestRTSEstimateIsPlausibleBounds guards against a whole class of error: an
// estimated max must sit above the working weight and below weight*reps by a
// sensible margin. A formula that gains or loses a factor blows past one of these.
// Note e1RM legitimately exceeds weight*reps for high-rep low-RPE sets, since the
// reserve-reps term pushes total reps well past the reps performed, so the upper
// bound is generous.
func TestRTSEstimateIsPlausibleBounds(t *testing.T) {
	for _, rpe := range []float64{6, 7, 8, 9, 10} {
		for _, reps := range []int{1, 3, 5, 8, 10} {
			for _, w := range []float64{95, 225, 405} {
				got := formulas.RTSPolynomial.Estimate1RM(w, reps, fptr(rpe))
				// The regression is a fit, not an exact inversion: a single rep at
				// RPE 10 evaluates to 99.85% rather than 100%, so the estimate can
				// land a fraction under the load. Allow 1% slack.
				if got < w*0.99 {
					t.Errorf("w=%v reps=%d rpe=%v: e1RM %.1f is below the working weight", w, reps, rpe, got)
				}
				if got > w*float64(reps)*3 {
					t.Errorf("w=%v reps=%d rpe=%v: e1RM %.1f is implausibly high", w, reps, rpe, got)
				}
			}
		}
	}
}

// TestRTSClampsRatherThanGoingNegative covers the polynomial's extrapolation past
// its fitted range, where it would otherwise return a negative percentage and a
// negative estimate.
func TestRTSClampsRatherThanGoingNegative(t *testing.T) {
	for _, tc := range []struct {
		reps int
		rpe  float64
	}{{25, 6}, {30, 6}, {50, 6}, {100, 6}} {
		pct := formulas.RTSPercentage(formulas.TotalReps(tc.reps, fptr(tc.rpe)))
		if pct < formulas.MinPercentage {
			t.Errorf("totalReps=%v: pct %v fell below the floor %v",
				formulas.TotalReps(tc.reps, fptr(tc.rpe)), pct, formulas.MinPercentage)
		}

		got := formulas.RTSPolynomial.Estimate1RM(225, tc.reps, fptr(tc.rpe))
		if got < 225 {
			t.Errorf("%d reps @ RPE %v gave %v, below the working weight", tc.reps, tc.rpe, got)
		}
	}
}

// TestRTSSingleRepAtRPE10EqualsLoad pins the formula's shape. A single rep taken
// to RPE 10 IS a one-rep max, so the estimate must be very close to the load.
//
// The polynomial gives 99.85% for that case rather than a clean 100%, so 405 comes
// out as 404.4 — within 0.2%. That residual is the regression's approximation
// error and is expected; what this test rules out is a missing or extra `reps`
// factor, which would move the answer by whole multiples.
func TestRTSSingleRepAtRPE10EqualsLoad(t *testing.T) {
	for _, w := range []float64{45, 225, 405} {
		got := formulas.RTSPolynomial.Estimate1RM(w, 1, fptr(10))
		if math.Abs(got-w)/w > 0.005 {
			t.Errorf("1 rep at RPE 10: got %.2f, want within 0.5%% of %v", got, w)
		}
	}
}

// TestRTSDistinguishesRPEsThatEpleyCannot is the reason the setting exists: the
// same load at different effort should not estimate the same max. Epley, having no
// RPE term, necessarily does.
func TestRTSDistinguishesRPEsThatEpleyCannot(t *testing.T) {
	const w, reps = 225.0, 5

	easy := formulas.RTSPolynomial.Estimate1RM(w, reps, fptr(6))
	max := formulas.RTSPolynomial.Estimate1RM(w, reps, fptr(10))

	if easy <= max {
		t.Errorf("a lower RPE should estimate a higher 1RM: RPE6=%.1f RPE10=%.1f", easy, max)
	}
	if spread := easy - max; spread < 10 {
		t.Errorf("expected a meaningful spread from RPE, got %.1f lb", spread)
	}

	// Epley is blind to effort, which is precisely the limitation being addressed.
	e1 := formulas.Epley.Estimate1RM(w, reps, fptr(6))
	e2 := formulas.Epley.Estimate1RM(w, reps, fptr(10))
	if e1 != e2 {
		t.Errorf("Epley should ignore RPE: %v vs %v", e1, e2)
	}
}

// TestRPEFormulaIsCloseToEpleyWhenRPEIsMax guards the default path. A user who
// switches to the RPE formula but logs everything at RPE 10 should see numbers
// close to what Epley gave them, not a jump. The polynomial is a regression fit
// rather than an exact inversion of the chart, so a few percent is expected.
func TestRPEFormulaIsCloseToEpleyWhenRPEIsMax(t *testing.T) {
	for _, tc := range []struct {
		w    float64
		reps int
	}{{225, 5}, {405, 3}, {135, 10}, {315, 1}} {
		rts := formulas.RTSPolynomial.Estimate1RM(tc.w, tc.reps, fptr(10))
		epley := formulas.Epley.Estimate1RM(tc.w, tc.reps, nil)

		diffPct := math.Abs(rts-epley) / epley * 100
		if diffPct > 6 {
			t.Errorf("%vx%d @ RPE10: RTS %.1f vs Epley %.1f (%.1f%% apart, want <6%%)",
				tc.w, tc.reps, rts, epley, diffPct)
		}
	}
}

func TestBlankRPEIsTreatedAsTen(t *testing.T) {
	const w, reps = 405.0, 3

	withNil := formulas.RTSPolynomial.Estimate1RM(w, reps, nil)
	explicit := formulas.RTSPolynomial.Estimate1RM(w, reps, fptr(formulas.DefaultAssumedRPE))

	if math.Abs(withNil-explicit) > tolerance {
		t.Errorf("nil RPE (%v) should equal explicit RPE %v (%v)",
			withNil, formulas.DefaultAssumedRPE, explicit)
	}
}

func TestEpleyMatchesHistoricalValues(t *testing.T) {
	// These values are asserted in the backend integration suite and the frontend
	// suite. Epley must not change; existing users' charts are built from it.
	cases := []struct {
		weight float64
		reps   int
		want   float64
	}{
		{405, 3, 445.5},
		{385, 5, 449.1666666667},
		{415, 3, 456.5},
		{120, 10, 160},
	}

	for _, tc := range cases {
		got := formulas.Epley.Estimate1RM(tc.weight, tc.reps, nil)
		if math.Abs(got-tc.want) > tolerance {
			t.Errorf("Epley(%v, %d) = %v, want %v", tc.weight, tc.reps, got, tc.want)
		}
	}
}

func TestFormulaIsRPEAware(t *testing.T) {
	if formulas.Epley.UsesRPE() {
		t.Error("Epley should not consume RPE")
	}
	if !formulas.RTSPolynomial.UsesRPE() {
		t.Error("RTS polynomial should consume RPE")
	}
}

func TestParseFormula(t *testing.T) {
	if f, err := formulas.Parse(""); err != nil || f != formulas.Default {
		t.Errorf("empty should default to %v, got %v (err %v)", formulas.Default, f, err)
	}
	if f, err := formulas.Parse("  RTS  "); err != nil || f != formulas.RTSPolynomial {
		t.Errorf("' RTS ' should parse to rts, got %v (err %v)", f, err)
	}
	if _, err := formulas.Parse("brzyczky"); err == nil {
		t.Error("an unknown formula should be rejected")
	}
}

// TestFormulaNeverNegative guards the pathological inputs a user could reach
// through the API: zero reps, zero weight, and an extreme RPE.
func TestFormulaNeverNegative(t *testing.T) {
	for _, f := range []formulas.Formula{formulas.Epley, formulas.RTSPolynomial} {
		for _, reps := range []int{0, -5, 1, 1000} {
			for _, rpe := range []*float64{nil, fptr(0), fptr(10), fptr(-3)} {
				got := f.Estimate1RM(0, reps, rpe)
				if got < 0 {
					t.Errorf("%s weight=0 reps=%d rpe=%v gave %v", f, reps, rpe, got)
				}
			}
		}
	}
}

// TestPercentageDecreasesMonotonically is a mathematical invariant of the
// polynomial. If it ever failed, the chart's trend line could invert.
func TestPercentageDecreasesMonotonically(t *testing.T) {
	prev := math.Inf(1)
	for tr := 0.0; tr <= 30; tr += 0.5 {
		pct := formulas.RTSPercentage(tr)
		if pct > prev {
			t.Errorf("percentage rose at totalReps=%v: %v > %v", tr, pct, prev)
		}
		prev = pct
	}
}

func fptr(v float64) *float64 { return &v }
