/**
 * A single logged set, editable in place.
 *
 * Weight and reps commit on blur/Enter so a lifter can type through several
 * sets without a save button, and Enter moves to the next row — the fastest
 * possible path through a session.
 */
import { useEffect, useRef, useState } from 'react';
import type { KeyboardEvent } from 'react';
import type { Set } from '../types';
import { Badge } from './ui';
import {
  estimated1RM,
  formatRPE,
  formatWeight,
  parseReps,
  parseWeight,
} from '../utils/formatting';

interface Props {
  set: Set;
  index: number;
  onSave: (id: number, patch: { weight: number; reps: number }) => void;
  onDelete: (id: number) => void;
  saving?: boolean;
}

export function SetRow({ set, index, onSave, onDelete, saving }: Props) {
  const [weight, setWeight] = useState(String(set.weight));
  const [reps, setReps] = useState(String(set.reps));
  const weightRef = useRef<HTMLInputElement>(null);
  const repsRef = useRef<HTMLInputElement>(null);

  // Re-sync when the server's value changes (e.g. after a refetch).
  useEffect(() => setWeight(String(set.weight)), [set.weight]);
  useEffect(() => setReps(String(set.reps)), [set.reps]);

  function commit() {
    const w = parseWeight(weight);
    const r = parseReps(reps);
    if (w === null || r === null) {
      // Invalid input: snap back to the stored values rather than sending junk.
      setWeight(String(set.weight));
      setReps(String(set.reps));
      return;
    }
    if (w !== set.weight || r !== set.reps) {
      onSave(set.id, { weight: w, reps: r });
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
  const previewEst = w !== null && r !== null ? estimated1RM(w, r) : 0;

  return (
    <div className="grid grid-cols-[1.5rem_1fr_1fr_auto] items-center gap-2 border-t border-slate-800/70 px-3 py-2 first:border-t-0">
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
        <span className="sr-only">RPE {formatRPE(set.rpe)}</span>
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