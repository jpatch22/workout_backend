/**
 * Settings screen.
 *
 * The 1RM formula choice is the substantive setting: it changes how personal
 * records and the progress charts are computed, server-side. The live worked
 * example below the picker is deliberate — switching formulas silently changes
 * historical numbers, and a user deserves to see that before they commit to it
 * rather than discovering it on a chart later.
 */
import { useEffect, useState } from 'react';
import { useFormulaCatalog, useSettings, useUpdateSettings } from '../hooks/useSettings';
import { Button, Card, ErrorBanner, Spinner } from '../components/ui';
import { Badge } from '../components/ui';
import { estimated1RM } from '../utils/e1rm';
import { formatWeight } from '../utils/formatting';
import { ApiError } from '../api/client';

// A representative set for the worked example: 225 x 5 is a common working load.
const EXAMPLE = { weight: 225, reps: 5 };

function Switch({
  checked,
  onChange,
  disabled,
  label,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={[
        'relative h-6 w-11 shrink-0 rounded-full transition-colors',
        'focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-emerald-600',
        'disabled:opacity-50',
        checked ? 'bg-emerald-600' : 'bg-slate-700',
      ].join(' ')}
    >
      <span
        className={[
          'absolute top-0.5 h-5 w-5 rounded-full bg-white transition-transform',
          checked ? 'translate-x-5' : 'translate-x-0.5',
        ].join(' ')}
      />
    </button>
  );
}

export default function Settings() {
  const { data: settings, isLoading, error } = useSettings();
  const { data: catalog } = useFormulaCatalog();
  const update = useUpdateSettings();

  const [rpeError, setRpeError] = useState<string>();

  useEffect(() => {
    // Surface validation errors from the last update.
    if (update.isError) {
      setRpeError(update.error instanceof ApiError ? update.error.message : 'Could not save settings');
    } else if (update.isSuccess) {
      setRpeError(undefined);
    }
  }, [update.isError, update.isSuccess, update.error]);

  if (isLoading) return <Spinner />;
  if (error) return <ErrorBanner message={error.message} />;
  if (!settings) return null;

  function setFormula(id: string) {
    update.mutate({ oneRmFormula: id });
  }

  function setBarWeight(value: number) {
    update.mutate({ plateBarWeight: value });
  }

  const formulas = catalog?.formulas ?? [];

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold text-slate-100">Settings</h1>
        <p className="text-sm text-slate-500">
          Preferences are saved to your account as you change them.
        </p>
      </div>

      <ErrorBanner message={rpeError} onDismiss={() => setRpeError(undefined)} />

      {/* --- RPE capture --- */}
      <Card title="RPE (rate of perceived exertion)">
        <div className="flex flex-col gap-4">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <p className="text-sm font-medium text-slate-200">Record RPE on sets</p>
              <p className="mt-0.5 text-sm text-slate-500">
                Adds an RPE field (1–10) when logging a set, so estimated maxes can
                account for how close to failure you actually were.
              </p>
            </div>
            <Switch
              checked={settings.rpeEnabled}
              onChange={(next) => update.mutate({ rpeEnabled: next })}
              disabled={update.isPending}
              label="Record RPE on sets"
            />
          </div>

          <div className="flex items-start justify-between gap-4 border-t border-slate-800 pt-4">
            <div className="min-w-0">
              <p className="flex items-center gap-2 text-sm font-medium text-slate-200">
                Treat a blank RPE as {settings.assumeRpe}
                <Badge>max effort</Badge>
              </p>
              <p className="mt-0.5 text-sm text-slate-500">
                Used whenever you log a set without an RPE. This is the conservative
                choice: it produces the lowest estimated max for that load, so an
                untracked set can never create a false personal record.
              </p>
            </div>
          </div>
        </div>
      </Card>

      {/* --- 1RM formula --- */}
      <Card title="Estimated 1RM formula">
        <fieldset className="flex flex-col gap-3">
          <legend className="sr-only">Estimated 1RM formula</legend>
          <p className="text-sm text-slate-500">
            Determines how personal records and the progress charts are calculated.
            Changing this recalculates your existing history with the same formula.
          </p>

          <div className="mt-1 flex flex-col gap-2">
            {formulas.map((f) => {
              const selected = settings.oneRmFormula === f.id;
              return (
                <label
                  key={f.id}
                  className={[
                    'flex cursor-pointer items-start gap-3 rounded-lg border px-3 py-3 transition-colors',
                    selected
                      ? 'border-emerald-700 bg-emerald-950/20'
                      : 'border-slate-800 hover:border-slate-700',
                  ].join(' ')}
                >
                  <input
                    type="radio"
                    name="oneRmFormula"
                    value={f.id}
                    checked={selected}
                    onChange={() => setFormula(f.id)}
                    disabled={update.isPending}
                    className="mt-1 h-4 w-4 accent-emerald-600"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap items-center gap-2">
                      <span className="text-sm font-medium text-slate-100">{f.name}</span>
                      {f.usesRpe && <Badge tone="info">uses RPE</Badge>}
                      {f.id === catalog?.default && <Badge>default</Badge>}
                    </span>
                    <span className="mt-0.5 block text-sm text-slate-500">{f.description}</span>
                  </span>
                </label>
              );
            })}
          </div>
        </fieldset>

        {/* Worked example, so the effect of the choice is visible up front. */}
        <div className="mt-4 rounded-lg border border-slate-800 bg-slate-950/50 p-3">
          <p className="text-xs font-medium uppercase tracking-wide text-slate-500">
            Worked example · {EXAMPLE.weight} × {EXAMPLE.reps}
          </p>
          <div className="mt-2 grid gap-2 sm:grid-cols-2">
            {formulas.map((f) => {
              // The RPE-aware formula needs an RPE to show a different number;
              // show it at RPE 8 so the comparison is meaningful.
              const rpe = f.usesRpe ? 8 : undefined;
              const value = estimated1RM(EXAMPLE.weight, EXAMPLE.reps, f.id, rpe);
              const active = settings.oneRmFormula === f.id;
              return (
                <div
                  key={f.id}
                  className={[
                    'rounded-md border px-3 py-2',
                    active ? 'border-emerald-800 bg-emerald-950/20' : 'border-slate-800',
                  ].join(' ')}
                >
                  <p className="text-xs text-slate-500">
                    {f.name}
                    {f.usesRpe ? ' @ RPE 8' : ''}
                  </p>
                  <p className="text-lg font-semibold tabular-nums text-slate-100">
                    {formatWeight(value)} lb
                  </p>
                </div>
              );
            })}
          </div>
          <p className="mt-2 text-xs text-slate-500">
            Same load, different formula. The RPE option credits the two reps left in
            reserve, so it reads higher here; at RPE 10 the two converge.
          </p>
        </div>
      </Card>

      {/* --- Lifting defaults --- */}
      <Card title="Lifting defaults">
        <div className="flex flex-col gap-4">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <p className="text-sm font-medium text-slate-200">Bar weight</p>
              <p className="mt-0.5 text-sm text-slate-500">
                Used when working out which plates to load.
              </p>
            </div>
            <div className="flex gap-2">
              {[45, 35, 15, 10].map((w) => (
                <Button
                  key={w}
                  size="sm"
                  variant={settings.plateBarWeight === w ? 'primary' : 'secondary'}
                  onClick={() => setBarWeight(w)}
                  disabled={update.isPending}
                >
                  {w} lb
                </Button>
              ))}
            </div>
          </div>

          <div className="flex flex-wrap items-end justify-between gap-4 border-t border-slate-800 pt-4">
            <div>
              <p className="text-sm font-medium text-slate-200">Units</p>
              <p className="mt-0.5 text-sm text-slate-500">
                Display only — logged values are stored exactly as you enter them.
              </p>
            </div>
            <div className="flex gap-2">
              {(['lb', 'kg'] as const).map((u) => (
                <Button
                  key={u}
                  size="sm"
                  variant={settings.weightUnit === u ? 'primary' : 'secondary'}
                  onClick={() => update.mutate({ weightUnit: u })}
                  disabled={update.isPending}
                >
                  {u}
                </Button>
              ))}
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
}