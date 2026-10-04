/**
 * TanStack Query hooks — the only place components touch server state.
 *
 * Every mutation invalidates the queries it affects, so a set logged on the
 * Workout screen immediately updates History, Progress, and the Dashboard
 * without any manual refetch plumbing.
 */
import {
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { exerciseApi } from '../api/exercises';
import { progressApi } from '../api/progress';
import { setApi, workoutApi } from '../api/workouts';
import type { SetInput, WorkoutInput } from '../types';

// --- queries ---

export function useDashboard() {
  return useQuery({
    queryKey: ['dashboard'],
    queryFn: progressApi.dashboard,
  });
}

export function useWorkoutList(limit?: number) {
  return useQuery({
    queryKey: ['workouts', limit ?? 'all'],
    queryFn: () => workoutApi.list(limit),
  });
}

export function useWorkout(id: number | null) {
  return useQuery({
    queryKey: ['workout', id],
    queryFn: () => workoutApi.get(id as number),
    enabled: id !== null,
  });
}

export function useExerciseCatalog() {
  return useQuery({
    queryKey: ['exercises'],
    queryFn: exerciseApi.list,
    // The catalog rarely changes; don't refetch it on every window focus.
    staleTime: 5 * 60 * 1000,
  });
}

export function useTrackedExercises() {
  return useQuery({
    queryKey: ['tracked-exercises'],
    queryFn: progressApi.trackedExercises,
  });
}

export function useExerciseProgress(slug: string | null) {
  return useQuery({
    queryKey: ['exercise-progress', slug],
    queryFn: () => progressApi.exercise(slug as string),
    enabled: Boolean(slug),
  });
}

// --- mutations ---

/**
 * Invalidates everything whose numbers depend on the sets table. Volume, PRs,
 * streaks and per-exercise progress are all derived, so a single set write
 * touches all of them.
 */
function useInvalidateTrainingData() {
  const qc = useQueryClient();
  return (workoutId?: number) => {
    void qc.invalidateQueries({ queryKey: ['dashboard'] });
    void qc.invalidateQueries({ queryKey: ['workouts'] });
    void qc.invalidateQueries({ queryKey: ['tracked-exercises'] });
    void qc.invalidateQueries({ queryKey: ['exercise-progress'] });
    if (workoutId !== undefined) {
      void qc.invalidateQueries({ queryKey: ['workout', workoutId] });
    }
  };
}

export function useCreateWorkout() {
  const invalidate = useInvalidateTrainingData();
  return useMutation({
    mutationFn: (input: WorkoutInput) => workoutApi.create(input),
    onSuccess: () => invalidate(),
  });
}

export function useUpdateWorkout(id: number) {
  const invalidate = useInvalidateTrainingData();
  return useMutation({
    mutationFn: (input: WorkoutInput) => workoutApi.update(id, input),
    onSuccess: () => invalidate(id),
  });
}

export function useDeleteWorkout() {
  const invalidate = useInvalidateTrainingData();
  return useMutation({
    mutationFn: (id: number) => workoutApi.remove(id),
    onSuccess: () => invalidate(),
  });
}

export function useAddExercise(workoutId: number) {
  const invalidate = useInvalidateTrainingData();
  return useMutation({
    mutationFn: ({ exerciseId, notes }: { exerciseId: number; notes?: string }) =>
      workoutApi.addExercise(workoutId, exerciseId, notes),
    onSuccess: () => invalidate(workoutId),
  });
}

export function useRemoveExercise(workoutId: number) {
  const invalidate = useInvalidateTrainingData();
  return useMutation({
    mutationFn: (workoutExerciseId: number) => workoutApi.removeExercise(workoutId, workoutExerciseId),
    onSuccess: () => invalidate(workoutId),
  });
}

export function useLogSet(workoutId: number) {
  const invalidate = useInvalidateTrainingData();
  return useMutation({
    mutationFn: ({ workoutExerciseId, input }: { workoutExerciseId: number; input: SetInput }) =>
      setApi.log(workoutExerciseId, input),
    onSuccess: () => invalidate(workoutId),
  });
}

export function useUpdateSet(workoutId: number) {
  const invalidate = useInvalidateTrainingData();
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: SetInput }) => setApi.update(id, input),
    onSuccess: () => invalidate(workoutId),
  });
}

export function useDeleteSet(workoutId: number) {
  const invalidate = useInvalidateTrainingData();
  return useMutation({
    mutationFn: (id: number) => setApi.remove(id),
    onSuccess: () => invalidate(workoutId),
  });
}

export function useCreateExercise() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string; muscleGroup?: string; equipment?: string }) =>
      exerciseApi.create(input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['exercises'] });
    },
  });
}