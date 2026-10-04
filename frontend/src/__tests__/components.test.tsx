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
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ExerciseBlock } from '../components/ExerciseBlock';
import { SetRow } from '../components/SetRow';
import { StatCard, Badge } from '../components/ui';
import {
  estimated1RM,
  formatVolume,
  formatWeight,
  formatRPE,
  parseReps,
  parseWeight,
  platePairs,
} from '../utils/formatting';
import type { Set, WorkoutExercise } from '../types';

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

describe('estimated1RM', () => {
  // These exact values are asserted in the Go integration suite. If the two
  // implementations ever drift, a user would see one number while logging and a
  // different one on the chart, and PRs would fire inconsistently.
  it('matches the backend Epley formula', () => {
    expect(estimated1RM(405, 3)).toBeCloseTo(445.5, 6);
    expect(estimated1RM(385, 5)).toBeCloseTo(449.1666666667, 6);
    expect(estimated1RM(415, 3)).toBeCloseTo(456.5, 6);
    expect(estimated1RM(120, 10)).toBeCloseTo(160, 6);
  });

  it('returns the weight itself for zero reps rather than dividing by zero', () => {
    expect(estimated1RM(225, 0)).toBe(225);
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
    render(<SetRow set={set({ isPr: true })} index={0} onSave={vi.fn()} onDelete={vi.fn()} />);
    expect(screen.getByLabelText('Set 1 weight')).toHaveValue('405');
    expect(screen.getByLabelText('Set 1 reps')).toHaveValue('3');
    expect(screen.getByText('PR')).toBeTruthy();
  });

  it('previews the estimated 1RM as you type', async () => {
    const user = userEvent.setup();
    render(<SetRow set={set()} index={0} onSave={vi.fn()} onDelete={vi.fn()} />);

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
    render(<SetRow set={set()} index={0} onSave={onSave} onDelete={vi.fn()} />);

    const weight = screen.getByLabelText('Set 1 weight');
    await user.clear(weight);
    await user.type(weight, '425');
    await user.tab();

    expect(onSave).toHaveBeenCalledWith(1, { weight: 425, reps: 3 });
  });

  it('does not save when the value is unchanged', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(<SetRow set={set()} index={0} onSave={onSave} onDelete={vi.fn()} />);

    await user.click(screen.getByLabelText('Set 1 weight'));
    await user.tab();

    expect(onSave).not.toHaveBeenCalled();
  });

  it('snaps back instead of saving invalid input', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(<SetRow set={set()} index={0} onSave={onSave} onDelete={vi.fn()} />);

    const weight = screen.getByLabelText('Set 1 weight');
    await user.clear(weight);
    await user.type(weight, '-50');
    await user.tab();

    expect(onSave).not.toHaveBeenCalled();
    expect(weight).toHaveValue('405');
  });

  it('marks a warmup and disables its inputs', () => {
    render(
      <SetRow set={set({ isWarmup: true, weight: 135, reps: 5 })} index={0} onSave={vi.fn()} onDelete={vi.fn()} />,
    );
    expect(screen.getByText('warmup')).toBeTruthy();
    expect(screen.getByLabelText('Set 1 weight')).toBeDisabled();
  });

  it('deletes on click', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(<SetRow set={set()} index={0} onSave={vi.fn()} onDelete={onDelete} />);

    await user.click(screen.getByLabelText('Delete set 1'));
    expect(onDelete).toHaveBeenCalledWith(1);
  });
});

describe('ExerciseBlock', () => {
  const noop = () => {};

  it('summarizes volume and excludes warmups from the set count', () => {
    render(
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
    render(
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
    render(
      <ExerciseBlock
        workoutExercise={workoutExercise([set({ id: 1, weight: 315, reps: 5 })])}
        onLogSet={onLogSet}
        onUpdateSet={noop}
        onDeleteSet={noop}
        onRemove={noop}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Add set' }));
    expect(onLogSet).toHaveBeenCalledWith(10, { weight: 315, reps: 5, isWarmup: false });
  });

  it('refuses to log invalid input', async () => {
    const user = userEvent.setup();
    const onLogSet = vi.fn();
    render(
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
    const { container } = render(
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
    render(
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
    render(
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