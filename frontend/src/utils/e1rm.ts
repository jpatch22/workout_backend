/**
 * Estimated-1RM math on the client.
 *
 * These MUST match backend/formulas/formulas.go — the backend computes PR flags
 * and the chart values from its copy, so a disagreement here would show a
 * different number in the per-set preview than the one that eventually creates
 * the PR. TestEstimated1RMMatchesBackend in the test suite pins the shared
 * values; TestFormulaImplementationsAgree covers Go vs SQL on the backend.
 */

/** The Epley formula, which ignores RPE. */
export const EPLEY = 'epley';
/** The RPE-aware RTS polynomial. */
export const RTS = 'rts';

/** A set with no RPE is read as this, which is the conservative direction. */
export const DEFAULT_ASSUMED_RPE = 10;

/** Coeffs of the RTS percentage polynomial. */
const RTS_A = 1.0261;
const RTS_B = 0.0242;
const RTS_C = 0.0004;

/** Floor on the polynomial's output, matching the backend. */
export const MIN_PERCENTAGE = 0.1;

/**
 * Total reps folds RPE into the polynomial: reps performed plus reps left in
 * reserve. An unset RPE contributes zero reserve.
 */
export function totalReps(reps: number, rpe?: number | null): number {
  const r = rpe ?? DEFAULT_ASSUMED_RPE;
  return reps + (DEFAULT_ASSUMED_RPE - r);
}

/**
 * Fraction of a 1RM a set represents, per the RTS polynomial, floored at
 * MIN_PERCENTAGE.
 */
export function rtsPercentage(totalRepsValue: number): number {
  const tr = Math.max(0, totalRepsValue);
  const pct = RTS_A - RTS_B * tr - RTS_C * tr * tr;
  return pct < MIN_PERCENTAGE ? MIN_PERCENTAGE : pct;
}

/**
 * Estimated 1RM for a set, using the given formula.
 *
 * @param formula 'epley' or 'rts'; anything else falls back to Epley.
 */
export function estimated1RM(
  weight: number,
  reps: number,
  formula: string = EPLEY,
  rpe?: number | null,
): number {
  if (reps <= 0) return weight;

  if (formula === RTS) {
    return weight / rtsPercentage(totalReps(reps, rpe));
  }
  // Epley: weight * (1 + reps/30).
  return weight * (1 + reps / 30);
}

/** Whether a formula consumes the RPE of a set. */
export function formulaUsesRPE(formula: string): boolean {
  return formula === RTS;
}