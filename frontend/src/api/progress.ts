/** Typed bindings for the derived /api/progress endpoints. */
import { http } from './client';
import type { Dashboard, DashboardStats, ExerciseProgress, TrackedExercise } from '../types';

export const progressApi = {
  dashboard: () => http.get<Dashboard>('/api/progress/dashboard'),

  overview: () => http.get<DashboardStats>('/api/progress'),

  trackedExercises: () => http.get<TrackedExercise[]>('/api/progress/exercises'),

  /** Full time series for one exercise, addressed by slug. */
  exercise: (slug: string, since?: string) =>
    http.get<ExerciseProgress>(`/api/progress/exercise/${encodeURIComponent(slug)}${since ? `?since=${since}` : ''}`),
};