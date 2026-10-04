/** The logging screen: pick exercises, log sets, edit or finish the session. */
import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ApiError } from '../api/client';
import { useAuth } from '../hooks/useAuth';
import {
  useAddExercise,
  useCreateExercise,
  useCreateWorkout,
  useDeleteSet,
  useDeleteWorkout,
  useExerciseCatalog,
  useLogSet,
  useRemoveExercise,
  useUpdateSet,
  useWorkout,
} from '../hooks/useWorkouts';
import { ExerciseBlock } from '../components/ExerciseBlock';
import { ExerciseSelector } from '../components/ExerciseSelector';
import { Button, Card, EmptyState, ErrorBanner, Spinner } from '../components/ui';
import { formatDateTime, formatVolume } from '../utils/formatting';
import type { Exercise } from '../types';

export default function WorkoutPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { user } = useAuth();

  const workoutId = id ? Number(id) : null;
  const { data: workout, isLoading, error } = useWorkout(workoutId);
  const { data: catalog } = useExerciseCatalog();

  // When no session is chosen yet, offer to start one.
  const [starting, setStarting] = useState(false);
  const createWorkout = useCreateWorkout();

  const addExercise = useAddExercise(workoutId ?? 0);
  const removeExercise = useRemoveExercise(workoutId ?? 0);
  const logSet = useLogSet(workoutId ?? 0);
  const updateSet = useUpdateSet(workoutId ?? 0);
  const deleteSet = useDeleteSet(workoutId ?? 0);
  const deleteWorkout = useDeleteWorkout();
  const createExercise = useCreateExercise();

  const [actionError, setActionError] = useState<string>();

  function report(err: unknown) {
    if (err instanceof ApiError) setActionError(err.message);
    else setActionError('Something went wrong. Please try again.');
  }

  async function startSession() {
    setStarting(true);
    setActionError(undefined);
    try {
      const wk = await createWorkout.mutateAsync({ name: '' });
      navigate(`/workout/${wk.id}`, { replace: true });
    } catch (err) {
      report(err);
    } finally {
      setStarting(false);
    }
  }

  async function handlePick(ex: Exercise) {
    if (!workoutId) return;
    setActionError(undefined);
    try {
      await addExercise.mutateAsync({ exerciseId: ex.id });
    } catch (err) {
      report(err);
    }
  }

  async function handleCreateExercise(name: string) {
    setActionError(undefined);
    try {
      const created = await createExercise.mutateAsync({ name });
      if (workoutId) await addExercise.mutateAsync({ exerciseId: created.id });
    } catch (err) {
      report(err);
    }
  }

  async function handleDeleteSession() {
    if (!workoutId || !confirm('Delete this workout and all its sets?')) return;
    setActionError(undefined);
    try {
      await deleteWorkout.mutateAsync(workoutId);
      navigate('/history', { replace: true });
    } catch (err) {
      report(err);
    }
  }

  // No session selected yet: a launcher, not a blank screen.
  if (!workoutId) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-semibold text-slate-100">Workout</h1>
        <EmptyState
          title="No session open"
          message="Start a new session to log your training. It'll be timestamped now and you can adjust the date later."
          action={<Button size="lg" onClick={startSession} disabled={starting}>
            {starting ? 'Starting…' : 'Start workout'}
          </Button>}
        />
      </div>
    );
  }

  if (isLoading) return <Spinner />;
  if (error) return <ErrorBanner message={error.message} />;
  if (!workout) return null;

  const workingSets = workout.exercises.flatMap((we) => we.sets.filter((s) => !s.isWarmup));
  const totalVolume = workingSets.reduce((sum, s) => sum + s.weight * s.reps, 0);
  const prCount = workingSets.filter((s) => s.isPr).length;

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-slate-100">{workout.name}</h1>
          <p className="text-sm text-slate-500">
            {formatDateTime(workout.performedAt)}
            {user && ` · ${user.displayName}`}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" onClick={() => navigate('/history')}>
            Done
          </Button>
          <Button variant="danger" onClick={handleDeleteSession}>
            Delete
          </Button>
        </div>
      </div>

      <ErrorBanner message={actionError} onDismiss={() => setActionError(undefined)} />

      <div className="flex flex-wrap gap-4 text-sm text-slate-400">
        <span>{workout.exercises.length} exercises</span>
        <span>{workingSets.length} working sets</span>
        <span>{formatVolume(totalVolume)} lb volume</span>
        {prCount > 0 && <span className="text-amber-400">{prCount} PR{prCount === 1 ? '' : 's'}</span>}
      </div>

      <div className="grid gap-5 lg:grid-cols-[1fr_20rem] lg:items-start">
        <div className="flex flex-col gap-4">
          {workout.exercises.length === 0 ? (
            <EmptyState
              title="No exercises yet"
              message="Use the panel to add the movements you're training today."
            />
          ) : (
            workout.exercises.map((we) => (
              <ExerciseBlock
                key={we.id}
                workoutExercise={we}
                logging={logSet.isPending}
                onLogSet={(workoutExerciseId, input) =>
                  logSet.mutate({ workoutExerciseId, input }, { onError: report })
                }
                onUpdateSet={(_workoutExerciseId, setId, patch) =>
                  updateSet.mutate({ id: setId, input: patch }, { onError: report })
                }
                onDeleteSet={(_workoutExerciseId, setId) =>
                  deleteSet.mutate(setId, { onError: report })
                }
                onRemove={(workoutExerciseId) =>
                  removeExercise.mutate(workoutExerciseId, { onError: report })
                }
              />
            ))
          )}
        </div>

        <div className="lg:sticky lg:top-6">
          <Card title="Add exercise">
            <ExerciseSelector
              catalog={catalog ?? []}
              existing={workout.exercises.map((we) => we.exerciseId)}
              onPick={handlePick}
              onCreate={handleCreateExercise}
              creating={createExercise.isPending}
              disabled={addExercise.isPending}
            />
          </Card>
        </div>
      </div>
    </div>
  );
}