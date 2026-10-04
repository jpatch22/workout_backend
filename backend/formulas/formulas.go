// Package formulas defines the estimated-1RM formulas the app can use.
//
// # One source of truth
//
// A formula exists in two places: Go (for validation, the optimistic preview,
// and unit tests) and PostgreSQL (for the aggregates that drive PR flags and the
// progress charts). Those two copies drifting apart is the dangerous failure mode
// here: a PR badge computed by one formula while the chart shows another would be
// a genuinely confusing bug.
//
// So the registry below is the only place a formula is defined. Each entry carries
// a Go implementation and, via Formula.SQLFor, the SQL expression for the same
// math — the SQL coefficients are interpolated from the Go constants rather than
// retyped. TestFormulaImplementationsAgree cross-checks the two against a live
// database, so a disagreement fails the build instead of reaching a user.
//
// # Available formulas
//
// Epley (default) ignores RPE:
//
//	weight * (1 + reps/30)
//
// RTSPolynomial is the RPE-aware formula used by most modern 1RM calculators: a
// polynomial regression fitted to Mike Tuchscherer's standard RTS RPE chart.
//
//	totalReps = reps + (10 - rpe)
//	pct       = 1.0261 - 0.0242*totalReps - 0.0004*totalReps^2
//	e1RM      = weight / pct
//
// The RPE enters through totalReps rather than as its own term: two reps in
// reserve (a low RPE) is treated as though you performed two extra reps, which is
// exactly the interpretation the RTS chart encodes.
//
// # Choosing between them
//
// RTSPolynomial is the better choice when sets carry an RPE, because it
// distinguishes 5 reps at RPE 6 from 5 reps at RPE 10 — which Epley cannot, and
// which is worth roughly 39 lb of estimated max on a 225x5. Epley remains the
// default because it needs no RPE data at all, and a set with no RPE recorded is
// treated as RPE 10, which keeps the two formulas close there.
package formulas

import (
	"fmt"
	"strings"
)

// Formula identifies an estimated-1RM method.
type Formula string

const (
	// Epley is the default: weight * (1 + reps/30). Ignores RPE.
	Epley Formula = "epley"
	// RTSPolynomial is the RPE-aware polynomial fitted to the RTS chart.
	RTSPolynomial Formula = "rts"
)

// Default is used when a user has expressed no preference, and whenever a stored
// value is missing or unrecognized.
const Default = Epley

// DefaultAssumedRPE is substituted for a set with no RPE recorded, per the
// requirement that a blank RPE be treated as 10 ("as hard as possible").
//
// This is the conservative direction: at a fixed load, RPE 10 yields the *lowest*
// estimated max, so an untracked set can never inflate a max or trigger a false
// personal record.
const DefaultAssumedRPE = 10.0

// Coefficients of the RTS percentage polynomial, named so the SQL expression and
// any future UI copy reference the same values rather than retyping them.
const (
	rtsCoeffA = 1.0261 // constant
	rtsCoeffB = 0.0242 // linear term
	rtsCoeffC = 0.0004 // quadratic term
)

// MinPercentage floors the polynomial's output.
//
// The regression is fitted to the RTS chart, which spans single reps to the high
// teens. Extrapolated past roughly 24 total reps it crosses zero and would yield
// a negative percentage and a negative estimate (25 reps at RPE 6 evaluates to a
// negative percentage). Rather than return nonsense, clamp here, capping the
// implied estimate at weight/MinPercentage.
const MinPercentage = 0.10

// FormulaInfo describes one formula for the settings UI and for validation.
type FormulaInfo struct {
	ID          Formula `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	UsesRPE     bool    `json:"usesRpe"`
}

// Registry lists every supported formula, in the order the settings UI shows them.
var Registry = []FormulaInfo{
	{
		ID:          Epley,
		Name:        "Epley",
		Description: "weight × (1 + reps / 30). The classic formula; ignores RPE.",
		UsesRPE:     false,
	},
	{
		ID:   RTSPolynomial,
		Name: "RPE (RTS polynomial)",
		Description: "Polynomial regression fitted to the standard RTS RPE chart, so it reads the " +
			"effort of each set. Treats a missing RPE as 10 (a true max).",
		UsesRPE: true,
	},
}

// IsValid reports whether f is a formula the app knows how to compute.
func (f Formula) IsValid() bool {
	for _, info := range Registry {
		if info.ID == f {
			return true
		}
	}
	return false
}

// UsesRPE reports whether the formula consumes the RPE value of a set.
func (f Formula) UsesRPE() bool {
	for _, info := range Registry {
		if info.ID == f {
			return info.UsesRPE
		}
	}
	return false
}

// Info returns the registry entry for f, falling back to the default so the UI
// never has to handle an unknown value.
func (f Formula) Info() FormulaInfo {
	for _, info := range Registry {
		if info.ID == f {
			return info
		}
	}
	return Registry[0]
}

// TotalReps is the bridging step that folds RPE into the polynomial: reps
// performed, plus the reps left in reserve.
//
//	Total Reps = Actual Reps + (10 - RPE)
//
// A nil rpe is treated as DefaultAssumedRPE, so it contributes zero reserve reps
// and the result matches an explicit RPE of 10.
func TotalReps(reps int, rpe *float64) float64 {
	r := DefaultAssumedRPE
	if rpe != nil {
		r = *rpe
	}
	return float64(reps) + (DefaultAssumedRPE - r)
}

// RTSPercentage returns the fraction of a 1RM a set represents, per the RTS
// polynomial regression, clamped to MinPercentage.
//
// Exported because the settings UI shows a live worked example of this number as
// the user moves the RPE slider.
func RTSPercentage(totalReps float64) float64 {
	if totalReps < 0 {
		totalReps = 0
	}
	pct := rtsCoeffA - rtsCoeffB*totalReps - rtsCoeffC*totalReps*totalReps
	if pct < MinPercentage {
		return MinPercentage
	}
	return pct
}

// Estimate1RM computes the formula in Go. A nil rpe is treated as
// DefaultAssumedRPE, matching the documented behavior and the SQL.
func (f Formula) Estimate1RM(weight float64, reps int, rpe *float64) float64 {
	if reps <= 0 {
		return weight
	}

	switch f {
	case RTSPolynomial:
		// The percentage already accounts for the rep count, so the estimate is
		// the load divided by the fraction. Multiplying by reps as well would
		// double-count it and inflate the result several-fold.
		return weight / RTSPercentage(TotalReps(reps, rpe))
	default:
		// Epley.
		return weight * (1 + float64(reps)/30)
	}
}

// SQLFor returns the PostgreSQL expression computing the same value as
// Formula.Estimate1RM for the row referenced by alias `s`, which must expose
// columns named weight, reps, and rpe.
//
// The returned SQL is an expression over `s`, not a standalone SELECT, so it can be
// dropped into any aggregate or comparison. A NULL s.rpe is handled inside the
// expression with the same "blank means 10" rule as the Go path.
func (f Formula) SQLFor(alias string) string {
	w := quoteIdent(alias) + ".weight"
	reps := quoteIdent(alias) + ".reps"
	rpe := quoteIdent(alias) + ".rpe"

	switch f {
	case RTSPolynomial:
		// Total reps, written once as a parenthesized sub-expression.
		total := fmt.Sprintf("(%s::float8 + (10 - COALESCE(%s::float8, %g)))", reps, rpe, DefaultAssumedRPE)

		// pct = A - B*total - C*total^2, floored at MinPercentage.
		pct := fmt.Sprintf(
			"GREATEST(%g, %g - %g*%s - %g*(%s)^2)",
			MinPercentage, rtsCoeffA, rtsCoeffB, total, rtsCoeffC, total,
		)

		return fmt.Sprintf("(%s / (%s))", w, pct)
	default:
		return fmt.Sprintf("(%s * (1 + %s::float8 / 30))", w, reps)
	}
}

// quoteIdent wraps an SQL identifier so it can be safely concatenated.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// Parse validates a formula id from user input, falling back to the default.
func Parse(s string) (Formula, error) {
	f := Formula(strings.ToLower(strings.TrimSpace(s)))
	if f == "" {
		return Default, nil
	}
	if !f.IsValid() {
		return "", fmt.Errorf("unknown 1RM formula %q (supported: %s)", s, supportedList())
	}
	return f, nil
}

func supportedList() string {
	names := make([]string, 0, len(Registry))
	for _, info := range Registry {
		names = append(names, string(info.ID))
	}
	return strings.Join(names, ", ")
}
