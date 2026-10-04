/**
 * Formatting helpers shared across the UI.
 *
 * The estimated-1RM formula here MUST match the backend
 * (services.Estimated1RM and the SQL in tables.MarkPRIfBeater), otherwise the
 * number shown while logging a set would disagree with the PR it creates.
 */

/** Epley estimated one-rep max: weight * (1 + reps/30). */
export function estimated1RM(weight: number, reps: number): number {
  if (reps <= 0) return weight;
  return weight * (1 + reps / 30);
}

/** Tonnage of one set. */
export function setVolume(weight: number, reps: number): number {
  return weight * reps;
}

/** Drops trailing zeros: 225 stays "225", 137.5 stays "137.5". */
export function formatWeight(weight: number): string {
  return Number.isInteger(weight) ? String(weight) : weight.toFixed(1);
}

/** Thousands-separated, rounded to a whole unit: 12345 -> "12,345". */
export function formatVolume(volume: number): string {
  return Math.round(volume).toLocaleString();
}

/** Parses user weight input, returning null when it isn't a usable number. */
export function parseWeight(input: string): number | null {
  const trimmed = input.trim();
  // Number('') is 0, which would silently log an empty field as a 0 lb set.
  if (trimmed === '') return null;
  const n = Number(trimmed);
  if (!Number.isFinite(n) || n < 0) return null;
  return n;
}

/** Parses reps input, returning null when it isn't a positive whole number. */
export function parseReps(input: string): number | null {
  const trimmed = input.trim();
  if (trimmed === '') return null;
  const n = Number(trimmed);
  if (!Number.isInteger(n) || n <= 0) return null;
  return n;
}

const DATE_FMT = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
});

const DATETIME_FMT = new Intl.DateTimeFormat(undefined, {
  month: 'short',
  day: 'numeric',
  year: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
});

/** "Oct 3, 2026" */
export function formatDate(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '—' : DATE_FMT.format(d);
}

/** "Oct 3, 2026, 6:45 PM" */
export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '—' : DATETIME_FMT.format(d);
}

/** "Today" / "Yesterday" / "3 days ago" / falls back to an absolute date. */
export function formatRelativeDay(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';

  const days = daysBetween(startOfDay(new Date()), startOfDay(d));
  if (days === 0) return 'Today';
  if (days === 1) return 'Yesterday';
  if (days < 7) return `${days} days ago`;
  return formatDate(iso);
}

function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

/** Whole days from a to b, ignoring time of day. */
function daysBetween(a: Date, b: Date): number {
  return Math.round((a.getTime() - b.getTime()) / 86_400_000);
}

/** Renders an RPE for display, or an em dash when unset. */
export function formatRPE(rpe?: number): string {
  return rpe === undefined || rpe === null ? '—' : rpe.toFixed(1);
}

/**
 * Picks the plate pairs (per side) needed to load a target weight given the
 * bar and the available plates. Returns [] for a bare bar, which is how the UI
 * signals "nothing to load".
 *
 * Greedy from the largest plate down is correct whenever the smallest plate is
 * 2.5 lb (every remainder is then reachable). When it isn't — a gym with no
 * 2.5s, say — a greedy fill can strand a remainder, so this tops the bar up
 * with the smallest plate instead of returning a load that adds up short.
 */
export function platePairs(
  targetWeight: number,
  barWeight = 45,
  availablePlates = [45, 35, 25, 10, 5, 2.5],
): number[] {
  const perSide = (targetWeight - barWeight) / 2;
  if (perSide <= 0) return [];

  const smallest = Math.min(...availablePlates);
  const pairs: number[] = [];
  let remaining = Math.round(perSide * 2) / 2; // work in half-pound steps

  for (const plate of availablePlates) {
    while (remaining >= plate) {
      pairs.push(plate);
      remaining = Math.round((remaining - plate) * 2) / 2;
    }
  }

  // Close any gap greedy left behind with the smallest plate we have.
  while (remaining >= smallest) {
    pairs.push(smallest);
    remaining = Math.round((remaining - smallest) * 2) / 2;
  }

  return pairs;
}