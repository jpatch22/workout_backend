/**
 * A single logged set, editable in place.
 *
 * Weight and reps commit on blur/Enter so a lifter can type through several sets
 * without a save button, and Enter moves to the next row — the fastest possible
 * path through a session.
 *
 * The estimated-1RM preview uses the user's selected formula, read from
 * SettingsContext. It must agree with the backend's copy of that formula, since
 * the backend is what decides whether this set becomes a PR.
 */
import { useEffect, useRef, useState } from 'react';
import type { KeyboardEvent } from 'react';
import type { Set } from '../types';
import { useSettingsContext } from '../hooks/useSettings';
import { estimated1RM } from '../utils/e1rm';
import { Badge } from './ui';
import { formatWeight, parseReps, parseWeight } from '../utils/formatting';

interface Props {
  set: Set;
  index: number;
  onSave: (id: number, patch: { weight: number; reps: number; rpe?: number | null }) => void;
  onDelete: (id: number) => void;
  saving?: boolean;
}

export function SetRow({ set, index, onSave, onDelete, saving }: Props) {
  const { formula, usesRPE, rpeEnabled } = useSettingsContext();

  const [weight, setWeight] = useState(String(set.weight));
  const [reps, setReps] = useState(String(set.reps));
  const [rpe, setRpe] = useState(set.rpe === undefined ? '' : String(set.rpe));
  const weightRef = useRef<HTMLInputElement>(null);
  const repsRef = useRef<HTMLInputElement>(null);

  // Re-sync when the server's value changes (e.g. after a refetch).
  useEffect(() => setWeight(String(set.weight)), [set.weight]);
  useEffect(() => setReps(String(set.reps)), [set.reps]);
  useEffect(() => setRpe(set.rpe === undefined ? '' : String(set.rpe)), [set.rpe]);

  // The RPE column only appears when the user turned on capture AND picked a
  // formula that reads RPE; otherwise it would be a field nobody reads.
  const showRPE = rpeEnabled && usesRPE;

  function commit() {
    const w = parseWeight(weight);
    const r = parseReps(reps);
    if (w === null || r === null) {
      // Invalid input: snap back to the stored values rather than sending junk.
      setWeight(String(set.weight));
      setReps(String(set.reps));
      return;
    }

    const patch: { weight: number; reps: number; rpe?: number | null } = { weight: w, reps: r };
    const storedRPE = set.rpe === undefined ? null : set.rpe;
    let rpeChanged = false;

    if (showRPE) {
      const trimmed = rpe.trim();
      if (trimmed === '') {
        // Explicitly clear the RPE rather than leaving a stale value behind.
        patch.rpe = null;
        rpeChanged = storedRPE !== null;
      } else {
        const parsed = Number(trimmed);
        if (Number.isFinite(parsed) && parsed >= 0 && parsed <= 10) {
          patch.rpe = parsed;
          rpeChanged = parsed !== storedRPE;
        } else {
          // Out-of-range RPE: revert the field and leave the stored value alone.
          setRpe(set.rpe === undefined ? '' : String(set.rpe));
        }
      }
    }

    if (w !== set.weight || r !== set.reps || rpeChanged) {
      onSave(set.id, patch);
    }
  }

  function handleWeightKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      e.preventDefault();
      commit();
      repsRef.current?.focus();
      repsRef.current?.select();
    }
  }

  function handleRepsKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      e.preventDefault();
      commit();
    }
  }

  const w = parseWeight(weight);
  const r = parseReps(reps);
  const trimmedRPE = rpe.trim();
  const previewRPE = trimmedRPE === '' ? undefined : Number(trimmedRPE);
  const previewEst = w !== null && r !== null ? estimated1RM(w, r, formula, previewRPE) : 0;

  const gridCols = showRPE
    ? 'grid-cols-[1.5rem_1fr_1fr_3.5rem_auto]'
    : 'grid-cols-[1.5rem_1fr_1fr_auto]';

  return (
    <div
      className={[
        'grid items-center gap-2 border-t border-slate-800/70 px-3 py-2 first:border-t-0',
        gridCols,
      ].join(' ')}
    >
      <span className="text-xs tabular-nums text-slate-500">{index + 1}</span>

      <input
        ref={weightRef}
        aria-label={`Set ${index + 1} weight`}
        inputMode="decimal"
        className="w-full rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm tabular-nums text-slate-100 focus:border-emerald-500 focus:outline-none"
        value={weight}
        disabled={set.isWarmup}
        onChange={(e) => setWeight(e.target.value)}
        onBlur={commit}
        onKeyDown={handleWeightKey}
      />

      <input
        ref={repsRef}
        aria-label={`Set ${index + 1} reps`}
        inputMode="numeric"
        className="w-full rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm tabular-nums text-slate-100 focus:border-emerald-500 focus:outline-none"
        value={reps}
        disabled={set.isWarmup}
        onChange={(e) => setReps(e.target.value)}
        onBlur={commit}
        onKeyDown={handleRepsKey}
      />

      {showRPE && (
        <input
          aria-label={`Set ${index + 1} RPE`}
          inputMode="decimal"
          placeholder="—"
          title="RPE (1–10). Leave blank to assume a max effort."
          className="w-full rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm tabular-nums text-slate-100 placeholder:text-slate-600 focus:border-emerald-500 focus:outline-none"
          value={rpe}
          disabled={set.isWarmup}
          onChange={(e) => setRpe(e.target.value)}
          onBlur={commit}
        />
      )}

      <div className="flex items-center gap-2">
        {set.isWarmup ? (
          <Badge tone="warmup">warmup</Badge>
        ) : (
          <>
            {set.isPr && <Badge tone="pr">PR</Badge>}
            <span className="hidden w-16 text-right text-xs tabular-nums text-slate-500 sm:inline">
              e1RM {formatWeight(previewEst)}
            </span>
            {saving && <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-400" />}
          </>
        )}
        <button
          type="button"
          onClick={() => onDelete(set.id)}
          className="rounded-md px-1.5 py-1 text-xs text-slate-600 transition-colors hover:bg-red-950/50 hover:text-red-400"
          aria-label={`Delete set ${index + 1}`}
        >
          ×
        </button>
      </div>
    </div>
  );
}