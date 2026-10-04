/** Typed bindings for /api/workouts, /api/workout-exercises, and /api/sets. */
import { http } from './client';
import type { Set, SetInput, Workout, WorkoutExercise, WorkoutInput, WorkoutSummary } from '../types';

export const workoutApi = {
  list: (limit?: number) =>
    http.get<WorkoutSummary[]>(`/api/workouts${limit ? `?limit=${limit}` : ''}`),

  get: (id: number) => http.get<Workout>(`/api/workouts/${id}`),

  create: (input: WorkoutInput = {}) => http.post<Workout>('/api/workouts', input),

  update: (id: number, input: WorkoutInput) => http.put<Workout>(`/api/workouts/${id}`, input),

  remove: (id: number) => http.delete<void>(`/api/workouts/${id}`),

  addExercise: (workoutId: number, exerciseId: number, notes?: string) =>
    http.post<WorkoutExercise>(`/api/workouts/${workoutId}/exercises`, { exerciseId, notes }),

  removeExercise: (workoutId: number, workoutExerciseId: number) =>
    http.delete<void>(`/api/workouts/${workoutId}/exercises/${workoutExerciseId}`),
};

export const setApi = {
  /** Log a set against a workout_exercises row. */
  log: (workoutExerciseId: number, input: SetInput) =>
    http.post<Set>(`/api/workout-exercises/${workoutExerciseId}/sets`, input),

  update: (id: number, input: SetInput) => http.put<Set>(`/api/sets/${id}`, input),

  remove: (id: number) => http.delete<void>(`/api/sets/${id}`),
};