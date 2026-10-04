/**
 * DOM tests for the presentational components.
 *
 * These mount real React trees in jsdom and assert on rendered output and user
 * interaction. They cover the parts of the UI where a mistake is invisible in a
 * typecheck: derived-number formatting, the e1RM formula agreeing with the
 * backend, and the set-row edit interaction.
 *
 * Server data is stubbed at the api/ layer, so these stay fast and hermetic.
 */
import { describe, expect, it, vi } from 'vitest';
import type { ReactElement } from 'react';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SettingsProvider } from '../hooks/useSettings';
import type { Settings } from '../api/settings';
import { ExerciseBlock } from '../components/ExerciseBlock';
import { SetRow } from '../components/SetRow';
import { StatCard, Badge } from '../components/ui';
import { formatVolume, formatWeight, formatRPE, parseReps, parseWeight, platePairs } from '../utils/formatting';
import {
  EPLEY,
  RTS,
  MIN_PERCENTAGE,
  estimated1RM,
  formulaUsesRPE,
  rtsPercentage,
  totalReps,
} from '../utils/e1rm';
import type { Set, WorkoutExercise } from '../types';

/**
 * Renders a component inside the providers it now depends on.
 *
 * SetRow and ExerciseBlock read the active 1RM formula from SettingsContext, so
 * a bare render would fall back to defaults and never exercise the RPE path.
 * `settings` overrides what the provider reports.
 */
function renderWithSettings(
  ui: ReactElement,
  settings?: Partial<Settings>,
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  // Seed the settings query so the provider resolves immediately with the
  // requested values instead of the loading fallback.
  if (settings) {
    queryClient.setQueryData(['settings'], {
      oneRmFormula: 'epley',
      rpeEnabled: false,
      assumeRpe: 10,
      plateBarWeight: 45,
      weightUnit: 'lb',
      ...settings,
    });
  } else {
    queryClient.setQueryData(['settings'], undefined);
  }

  return render(
    <QueryClientProvider client={queryClient}>
      <SettingsProvider>{ui}</SettingsProvider>
    </QueryClientProvider>,
  );
}

const set = (over: Partial<Set> = {}): Set => ({
  id: 1,
  workoutExerciseId: 10,
  weight: 405,
  reps: 3,
  isWarmup: false,
  isBodyweight: false,
  isPr: false,
  loggedAt: new Date().toISOString(),
  ...over,
});

const workoutExercise = (sets: Set[]): WorkoutExercise => ({
  id: 10,
  workoutId: 1,
  exerciseId: 5,
  position: 0,
  exercise: {
    id: 5,
    name: 'Back Squat',
    slug: 'back-squat',
    muscleGroup: 'legs',
    isCustom: false,
  },
  sets,
});

// --- estimated 1RM, mirroring backend/formulas/formulas.go ---
//
// These values are also asserted on the Go side (TestEpleyMatchesHistoricalValues,
// TestRTSMatchesWorkedExample) and cross-checked Go-vs-SQL over a grid. If the
// TypeScript copy drifts, the per-set preview while logging would disagree with
// the PR badge and the chart that the backend computes.

describe('estimated1RM (Epley)', () => {
  it('matches the values the backend asserts', () => {
    expect(estimated1RM(405, 3, EPLEY)).toBeCloseTo(445.5, 6);
    expect(estimated1RM(385, 5, EPLEY)).toBeCloseTo(449.1666666667, 6);
    expect(estimated1RM(415, 3, EPLEY)).toBeCloseTo(456.5, 6);
    expect(estimated1RM(120, 10, EPLEY)).toBeCloseTo(160, 6);
  });

  it('returns the weight itself for zero reps rather than dividing by zero', () => {
    expect(estimated1RM(225, 0, EPLEY)).toBe(225);
  });

  it('ignores RPE entirely', () => {
    expect(estimated1RM(225, 5, EPLEY, 6)).toBe(estimated1RM(225, 5, EPLEY, 10));
  });
});

describe('estimated1RM (RTS polynomial)', () => {
  it('folds RPE into total reps as reps plus reps in reserve', () => {
    expect(totalReps(5, 8)).toBe(7);
    expect(totalReps(5, 10)).toBe(5);
    expect(totalReps(5, 6)).toBe(9);
    // A blank RPE is read as 10, contributing no reserve.
    expect(totalReps(5, undefined)).toBe(5);
  });

  it('reproduces the documented worked example: 200x5 @ RPE 8 -> ~239 lb', () => {
    expect(rtsPercentage(totalReps(5, 8))).toBeCloseTo(0.8371, 4);
    expect(estimated1RM(200, 5, RTS, 8)).toBeCloseTo(238.9, 1);
  });

  it('estimates a lower RPE as a higher max', () => {
    const easy = estimated1RM(225, 5, RTS, 6);
    const max = estimated1RM(225, 5, RTS, 10);
    expect(easy).toBeGreaterThan(max);
  });

  it('treats a blank RPE as 10', () => {
    expect(estimated1RM(225, 5, RTS, undefined)).toBeCloseTo(estimated1RM(225, 5, RTS, 10), 6);
  });

  it('clamps rather than going negative past its fitted range', () => {
    // The polynomial crosses zero around 24 total reps; without a floor this
    // would return a negative estimate.
    const wild = estimated1RM(225, 40, RTS, 6);
    expect(wild).toBeGreaterThan(0);
    expect(rtsPercentage(totalReps(40, 6))).toBe(MIN_PERCENTAGE);
  });

  it('reports which formulas read RPE', () => {
    expect(formulaUsesRPE(RTS)).toBe(true);
    expect(formulaUsesRPE(EPLEY)).toBe(false);
  });
});

describe('formatting', () => {
  it('drops trailing zeros on weights', () => {
    expect(formatWeight(225)).toBe('225');
    expect(formatWeight(137.5)).toBe('137.5');
  });

  it('rounds and separates volume', () => {
    expect(formatVolume(3140)).toBe('3,140');
    expect(formatVolume(449.166)).toBe('449');
  });

  it('renders a missing RPE as an em dash', () => {
    expect(formatRPE(undefined)).toBe('—');
    expect(formatRPE(8.5)).toBe('8.5');
  });

  it('rejects unusable weight and reps input', () => {
    expect(parseWeight('225')).toBe(225);
    expect(parseWeight('-5')).toBeNull();
    expect(parseWeight('')).toBeNull();
    expect(parseReps('5')).toBe(5);
    expect(parseReps('0')).toBeNull();
    expect(parseReps('2.5')).toBeNull();
  });
});

describe('platePairs', () => {
  it('returns nothing for a bare bar', () => {
    expect(platePairs(45)).toEqual([]);
    expect(platePairs(30)).toEqual([]);
  });

  it('builds per-side pairs for a loaded bar', () => {
    // 225 lb = 45 bar + 2 sides x (45 + 45)
    expect(platePairs(225)).toEqual([45, 45]);
  });

  it('sums the pairs back to the target weight', () => {
    // The pairs are per side; doubled plus the bar must equal the target.
    for (const target of [95, 135, 185, 225, 275, 315, 405]) {
      const loaded = platePairs(target).reduce((a, b) => a + b, 0) * 2 + 45;
      expect(loaded).toBe(target);
    }
  });

  it('tops up a remainder when 2.5 lb plates are unavailable', () => {
    // Without 2.5s, 275 lb would otherwise come up short on the greedy fill.
    const pairs = platePairs(275, 45, [45, 35, 25, 10, 5]);
    const loaded = pairs.reduce((a, b) => a + b, 0) * 2 + 45;
    expect(loaded).toBeGreaterThanOrEqual(275);
  });
});

describe('StatCard', () => {
  it('renders the label and value', () => {
    render(<StatCard label="Total volume" value="3,140" sub="lb · all time" accent />);
    expect(screen.getByText('Total volume')).toBeTruthy();
    expect(screen.getByText('3,140')).toBeTruthy();
    expect(screen.getByText('lb · all time')).toBeTruthy();
  });
});

describe('Badge', () => {
  it('renders its children', () => {
    render(<Badge tone="pr">PR</Badge>);
    expect(screen.getByText('PR')).toBeTruthy();
  });
});

describe('SetRow', () => {
  it('shows the stored weight and reps and flags a PR', () => {
    renderWithSettings(<SetRow set={set({ isPr: true })} index={0} onSave={vi.fn()} onDelete={vi.fn()} />);
    expect(screen.getByLabelText('Set 1 weight')).toHaveValue('405');
    expect(screen.getByLabelText('Set 1 reps')).toHaveValue('3');
    expect(screen.getByText('PR')).toBeTruthy();
  });

  it('previews the estimated 1RM as you type', async () => {
    const user = userEvent.setup();
    renderWithSettings(<SetRow set={set()} index={0} onSave={vi.fn()} onDelete={vi.fn()} />);

    // 385 x 5 previews 449.17, matching the backend.
    await user.clear(screen.getByLabelText('Set 1 weight'));
    await user.type(screen.getByLabelText('Set 1 weight'), '385');
    await user.clear(screen.getByLabelText('Set 1 reps'));
    await user.type(screen.getByLabelText('Set 1 reps'), '5');

    expect(screen.getByText(/e1RM 449/)).toBeTruthy();
  });

  it('saves on blur when the value changed', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderWithSettings(<SetRow set={set()} index={0} onSave={onSave} onDelete={vi.fn()} />);

    const weight = screen.getByLabelText('Set 1 weight');
    await user.clear(weight);
    await user.type(weight, '425');
    await user.tab();

    expect(onSave).toHaveBeenCalledWith(1, { weight: 425, reps: 3 });
  });

  it('does not save when the value is unchanged', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderWithSettings(<SetRow set={set()} index={0} onSave={onSave} onDelete={vi.fn()} />);

    await user.click(screen.getByLabelText('Set 1 weight'));
    await user.tab();

    expect(onSave).not.toHaveBeenCalled();
  });

  it('snaps back instead of saving invalid input', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderWithSettings(<SetRow set={set()} index={0} onSave={onSave} onDelete={vi.fn()} />);

    const weight = screen.getByLabelText('Set 1 weight');
    await user.clear(weight);
    await user.type(weight, '-50');
    await user.tab();

    expect(onSave).not.toHaveBeenCalled();
    expect(weight).toHaveValue('405');
  });

  it('marks a warmup and disables its inputs', () => {
    renderWithSettings(
      <SetRow set={set({ isWarmup: true, weight: 135, reps: 5 })} index={0} onSave={vi.fn()} onDelete={vi.fn()} />,
    );
    expect(screen.getByText('warmup')).toBeTruthy();
    expect(screen.getByLabelText('Set 1 weight')).toBeDisabled();
  });

  it('deletes on click', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    renderWithSettings(<SetRow set={set()} index={0} onSave={vi.fn()} onDelete={onDelete} />);

    await user.click(screen.getByLabelText('Delete set 1'));
    expect(onDelete).toHaveBeenCalledWith(1);
  });
});

describe('ExerciseBlock', () => {
  const noop = () => {};

  it('summarizes volume and excludes warmups from the set count', () => {
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([
          set({ id: 1, weight: 135, reps: 5, isWarmup: true }),
          set({ id: 2, weight: 405, reps: 3 }),
          set({ id: 3, weight: 385, reps: 5 }),
        ])}
        onLogSet={noop}
        onUpdateSet={noop}
        onDeleteSet={noop}
        onRemove={noop}
      />,
    );

    // 405*3 + 385*5 = 3140, warmup excluded.
    expect(screen.getByText(/3,140 lb/)).toBeTruthy();
    expect(screen.getByText(/2 working sets · 1 warmup/)).toBeTruthy();
  });

  it('seeds the quick-add row from the previous set', () => {
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([set({ id: 1, weight: 315, reps: 5 })])}
        onLogSet={noop}
        onUpdateSet={noop}
        onDeleteSet={noop}
        onRemove={noop}
      />,
    );

    expect(screen.getByLabelText('New set weight')).toHaveAttribute('placeholder', '315');
    expect(screen.getByLabelText('New set reps')).toHaveAttribute('placeholder', '5');
  });

  it('logs a repeated set from the seeded values', async () => {
    const user = userEvent.setup();
    const onLogSet = vi.fn();
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([set({ id: 1, weight: 315, reps: 5 })])}
        onLogSet={onLogSet}
        onUpdateSet={noop}
        onDeleteSet={noop}
        onRemove={noop}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Add set' }));
    // rpe: null is sent explicitly so the backend reads the field as unset rather
    // than leaving a previously recorded value in place.
    expect(onLogSet).toHaveBeenCalledWith(10, {
      weight: 315,
      reps: 5,
      isWarmup: false,
      rpe: null,
    });
  });

  it('refuses to log invalid input', async () => {
    const user = userEvent.setup();
    const onLogSet = vi.fn();
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([set({ id: 1 })])}
        onLogSet={onLogSet}
        onUpdateSet={noop}
        onDeleteSet={noop}
        onRemove={noop}
      />,
    );

    await user.type(screen.getByLabelText('New set reps'), '0');
    await user.click(screen.getByRole('button', { name: 'Add set' }));

    expect(onLogSet).not.toHaveBeenCalled();
    expect(screen.getByRole('alert').textContent).toMatch(/valid weight and reps/);
  });

  it('shows a PR badge only on the set that earned it', () => {
    const { container } = renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([
          set({ id: 1, weight: 405, reps: 3, isPr: true }),
          set({ id: 2, weight: 385, reps: 5 }),
        ])}
        onLogSet={noop}
        onUpdateSet={noop}
        onDeleteSet={noop}
        onRemove={noop}
      />,
    );

    const badges = within(container).queryAllByText('PR');
    expect(badges).toHaveLength(1);
  });

  it('renders an empty state when no sets are logged yet', () => {
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([])}
        onLogSet={noop}
        onUpdateSet={noop}
        onDeleteSet={noop}
        onRemove={noop}
      />,
    );
    expect(screen.getByText(/0 working sets/)).toBeTruthy();
  });

  it('removes the exercise from the workout', async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([set({ id: 1 })])}
        onLogSet={noop}
        onUpdateSet={noop}
        onDeleteSet={noop}
        onRemove={onRemove}
      />,
    );

    await user.click(screen.getByLabelText('Remove Back Squat'));
    expect(onRemove).toHaveBeenCalledWith(10);
  });
});

// --- RPE capture, gated on both the setting and the chosen formula ---

describe('SetRow with RPE enabled', () => {
  it('shows the RPE field when capture is on and the formula reads RPE', () => {
    renderWithSettings(
      <SetRow set={set()} index={0} onSave={vi.fn()} onDelete={vi.fn()} />,
      { rpeEnabled: true, oneRmFormula: 'rts' },
    );
    expect(screen.getByLabelText('Set 1 RPE')).toBeTruthy();
  });

  it('hides the RPE field when the formula ignores RPE', () => {
    renderWithSettings(
      <SetRow set={set()} index={0} onSave={vi.fn()} onDelete={vi.fn()} />,
      { rpeEnabled: true, oneRmFormula: 'epley' },
    );
    expect(screen.queryByLabelText('Set 1 RPE')).toBeNull();
  });

  it('hides the RPE field when capture is off, even with an RPE formula', () => {
    renderWithSettings(
      <SetRow set={set()} index={0} onSave={vi.fn()} onDelete={vi.fn()} />,
      { rpeEnabled: false, oneRmFormula: 'rts' },
    );
    expect(screen.queryByLabelText('Set 1 RPE')).toBeNull();
  });

  it('saves an entered RPE alongside weight and reps', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderWithSettings(
      <SetRow set={set({ weight: 225, reps: 5 })} index={0} onSave={onSave} onDelete={vi.fn()} />,
      { rpeEnabled: true, oneRmFormula: 'rts' },
    );

    await user.type(screen.getByLabelText('Set 1 RPE'), '8');
    await user.tab();

    expect(onSave).toHaveBeenCalledWith(1, { weight: 225, reps: 5, rpe: 8 });
  });

  it('sends null when the RPE field is cleared', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderWithSettings(
      <SetRow set={set({ weight: 225, reps: 5, rpe: 8 })} index={0} onSave={onSave} onDelete={vi.fn()} />,
      { rpeEnabled: true, oneRmFormula: 'rts' },
    );

    const field = screen.getByLabelText('Set 1 RPE');
    expect(field).toHaveValue('8');

    await user.clear(field);
    await user.tab();

    expect(onSave).toHaveBeenCalledWith(1, { weight: 225, reps: 5, rpe: null });
  });

  it('rejects an out-of-range RPE instead of saving it', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderWithSettings(
      <SetRow set={set({ weight: 225, reps: 5 })} index={0} onSave={onSave} onDelete={vi.fn()} />,
      { rpeEnabled: true, oneRmFormula: 'rts' },
    );

    const field = screen.getByLabelText('Set 1 RPE');
    await user.type(field, '15');
    await user.tab();

    // The field snaps back and nothing is written.
    expect(onSave).not.toHaveBeenCalled();
    expect(field).toHaveValue('');
  });

  it('previews a higher estimate for an easier RPE', async () => {
    const user = userEvent.setup();
    renderWithSettings(
      <SetRow set={set({ weight: 225, reps: 5 })} index={0} onSave={vi.fn()} onDelete={vi.fn()} />,
      { rpeEnabled: true, oneRmFormula: 'rts' },
    );

    const field = screen.getByLabelText('Set 1 RPE');
    await user.type(field, '10');
    const hard = screen.getByText(/e1RM/).textContent;

    await user.clear(field);
    await user.type(field, '6');
    const easy = screen.getByText(/e1RM/).textContent;

    // The RPE-6 preview must read higher than the RPE-10 preview.
    const n = (s: string | null) => Number((s ?? '').replace(/[^0-9.]/g, ''));
    expect(n(easy)).toBeGreaterThan(n(hard));
  });
});

describe('ExerciseBlock with RPE enabled', () => {
  it('logs a set with the entered RPE', async () => {
    const user = userEvent.setup();
    const onLogSet = vi.fn();
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([set({ id: 1, weight: 225, reps: 5 })])}
        onLogSet={onLogSet}
        onUpdateSet={vi.fn()}
        onDeleteSet={vi.fn()}
        onRemove={vi.fn()}
      />,
      { rpeEnabled: true, oneRmFormula: 'rts' },
    );

    await user.type(screen.getByLabelText('New set RPE'), '7');
    await user.click(screen.getByRole('button', { name: 'Add set' }));

    expect(onLogSet).toHaveBeenCalledWith(10, {
      weight: 225,
      reps: 5,
      isWarmup: false,
      rpe: 7,
    });
  });

  it('sends rpe null when the field is left blank', async () => {
    const user = userEvent.setup();
    const onLogSet = vi.fn();
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([set({ id: 1, weight: 225, reps: 5 })])}
        onLogSet={onLogSet}
        onUpdateSet={vi.fn()}
        onDeleteSet={vi.fn()}
        onRemove={vi.fn()}
      />,
      { rpeEnabled: true, oneRmFormula: 'rts' },
    );

    await user.click(screen.getByRole('button', { name: 'Add set' }));
    expect(onLogSet).toHaveBeenCalledWith(10, {
      weight: 225,
      reps: 5,
      isWarmup: false,
      rpe: null,
    });
  });

  it('rejects an out-of-range RPE before logging', async () => {
    const user = userEvent.setup();
    const onLogSet = vi.fn();
    renderWithSettings(
      <ExerciseBlock
        workoutExercise={workoutExercise([set({ id: 1 })])}
        onLogSet={onLogSet}
        onUpdateSet={vi.fn()}
        onDeleteSet={vi.fn()}
        onRemove={vi.fn()}
      />,
      { rpeEnabled: true, oneRmFormula: 'rts' },
    );

    await user.type(screen.getByLabelText('New set RPE'), '12');
    await user.click(screen.getByRole('button', { name: 'Add set' }));

    expect(onLogSet).not.toHaveBeenCalled();
    expect(screen.getByRole('alert').textContent).toMatch(/RPE must be between 0 and 10/);
  });
});
