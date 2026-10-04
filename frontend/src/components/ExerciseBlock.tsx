/**
 * One exercise within a workout: its logged sets plus a quick-add row.
 *
 * The quick-add row carries the previous set's weight and reps forward, so
 * repeated sets at the same load are one tap.
 */
import { useState } from 'react';
import type { WorkoutExercise } from '../types';
import { SetRow } from './SetRow';
import { useSettingsContext } from '../hooks/useSettings';
import { estimated1RM } from '../utils/e1rm';
import { Badge, Button, ErrorBanner } from './ui';
import { formatVolume, formatWeight } from '../utils/formatting';

interface Props {
  workoutExercise: WorkoutExercise;
  onLogSet: (
    workoutExerciseId: number,
    input: { weight: number; reps: number; isWarmup: boolean; rpe?: number | null },
  ) => void;
  onUpdateSet: (
    workoutExerciseId: number,
    id: number,
    patch: { weight: number; reps: number; rpe?: number | null },
  ) => void;
  onDeleteSet: (workoutExerciseId: number, id: number) => void;
  onRemove: (workoutExerciseId: number) => void;
  savingSetId?: number;
  logging?: boolean;
}

export function ExerciseBlock({
  workoutExercise,
  onLogSet,
  onUpdateSet,
  onDeleteSet,
  onRemove,
  savingSetId,
  logging,
}: Props) {
  const { formula, usesRPE, rpeEnabled } = useSettingsContext();
  const showRPE = rpeEnabled && usesRPE;

  const working = workoutExercise.sets.filter((s) => !s.isWarmup);

  // Seed the quick-add from the last working set, which is what you repeat most.
  const last = working[working.length - 1];
  const [weight, setWeight] = useState('');
  const [reps, setReps] = useState('');
  const [rpe, setRpe] = useState('');
  const [isWarmup, setIsWarmup] = useState(false);
  const [error, setError] = useState<string>();

  function quickAdd() {
    const w = weight.trim() === '' ? last?.weight ?? 45 : Number(weight);
    const r = reps.trim() === '' ? last?.reps ?? 8 : Number(reps);

    if (!Number.isFinite(w) || w < 0 || !Number.isInteger(r) || r <= 0) {
      setError('Enter a valid weight and reps');
      return;
    }
    // An empty RPE field is sent as null, which the backend reads as the
    // configured default (10) rather than as zero.
    let parsedRPE: number | null = null;
    if (showRPE && rpe.trim() !== '') {
      const parsed = Number(rpe);
      if (!Number.isFinite(parsed) || parsed < 0 || parsed > 10) {
        setError('RPE must be between 0 and 10');
        return;
      }
      parsedRPE = parsed;
    }

    setError(undefined);
    onLogSet(workoutExercise.id, { weight: w, reps: r, isWarmup, rpe: parsedRPE });
  }

  const volume = working.reduce((sum, s) => sum + s.weight * s.reps, 0);
  // Must use the same formula as the backend, or this summary would disagree
  // with the chart and the PR badge for the very same sets.
  const topEst = working.reduce(
    (best, s) => Math.max(best, estimated1RM(s.weight, s.reps, formula, s.rpe)),
    0,
  );

  return (
    <section className="rounded-xl border border-slate-800 bg-slate-900/60">
      <header className="flex items-start justify-between gap-3 px-3 py-2.5">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-slate-100">
            {workoutExercise.exercise.name}
          </h3>
          <p className="mt-0.5 text-xs text-slate-500">
            {working.length} working set{working.length === 1 ? '' : 's'}
            {workoutExercise.sets.length > working.length &&
              ` · ${workoutExercise.sets.length - working.length} warmup`}
            {volume > 0 && ` · ${formatVolume(volume)} lb`}
            {topEst > 0 && ` · e1RM ${formatWeight(topEst)}`}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {workoutExercise.exercise.muscleGroup && (
            <Badge>{workoutExercise.exercise.muscleGroup}</Badge>
          )}
          <button
            type="button"
            onClick={() => onRemove(workoutExercise.id)}
            className="rounded-md px-1.5 py-1 text-xs text-slate-600 transition-colors hover:bg-red-950/50 hover:text-red-400"
            aria-label={`Remove ${workoutExercise.exercise.name}`}
          >
            Remove
          </button>
        </div>
      </header>

      <div
        className={[
          'grid items-center gap-2 border-y border-slate-800/70 px-3 py-1.5',
          showRPE ? 'grid-cols-[1.5rem_1fr_1fr_3.5rem_auto]' : 'grid-cols-[1.5rem_1fr_1fr_auto]',
        ].join(' ')}
      >
        <span />
        <span className="text-xs font-medium uppercase tracking-wide text-slate-500">Weight</span>
        <span className="text-xs font-medium uppercase tracking-wide text-slate-500">Reps</span>
        {showRPE && (
          <span
            className="text-xs font-medium uppercase tracking-wide text-slate-500"
            title="Rate of perceived exertion, 1-10"
          >
            RPE
          </span>
        )}
        <span />
      </div>

      {workoutExercise.sets.map((s, i) => (
        <SetRow
          key={s.id}
          set={s}
          index={i}
          saving={savingSetId === s.id}
          onSave={(id, patch) => onUpdateSet(workoutExercise.id, id, patch)}
          onDelete={(id) => onDeleteSet(workoutExercise.id, id)}
        />
      ))}

      <div className="flex flex-col gap-2 border-t border-slate-800/70 p-3">
        <ErrorBanner message={error} onDismiss={() => setError(undefined)} />
        <div className={['grid gap-2', showRPE ? 'grid-cols-[1fr_1fr_3.5rem]' : 'grid-cols-2'].join(' ')}>
          <input
            aria-label="New set weight"
            inputMode="decimal"
            placeholder={last ? formatWeight(last.weight) : 'Weight'}
            className="rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm tabular-nums text-slate-100 placeholder:text-slate-600 focus:border-emerald-500 focus:outline-none"
            value={weight}
            onChange={(e) => setWeight(e.target.value)}
          />
          <input
            aria-label="New set reps"
            inputMode="numeric"
            placeholder={last ? String(last.reps) : 'Reps'}
            className="rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm tabular-nums text-slate-100 placeholder:text-slate-600 focus:border-emerald-500 focus:outline-none"
            value={reps}
            onChange={(e) => setReps(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') quickAdd();
            }}
          />
          {showRPE && (
            <input
              aria-label="New set RPE"
              inputMode="decimal"
              placeholder="RPE"
              title="1-10. Leave blank to assume a max effort."
              className="rounded-md border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm tabular-nums text-slate-100 placeholder:text-slate-600 focus:border-emerald-500 focus:outline-none"
              value={rpe}
              onChange={(e) => setRpe(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') quickAdd();
              }}
            />
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" onClick={quickAdd} disabled={logging}>
            {logging ? 'Logging…' : 'Add set'}
          </Button>
          <label className="flex cursor-pointer items-center gap-1.5 text-xs text-slate-400">
            <input
              type="checkbox"
              checked={isWarmup}
              onChange={(e) => setIsWarmup(e.target.checked)}
              className="h-3.5 w-3.5 rounded border-slate-600 bg-slate-900 accent-emerald-600"
            />
            Warmup
          </label>
        </div>
      </div>
    </section>
  );
}