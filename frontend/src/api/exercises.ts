/** Typed bindings for the /api/exercises catalog endpoints. */
import { http } from './client';
import type { Exercise } from '../types';

export const exerciseApi = {
  list: () => http.get<Exercise[]>('/api/exercises'),

  get: (id: number) => http.get<Exercise>(`/api/exercises/${id}`),

  create: (input: { name: string; muscleGroup?: string; equipment?: string }) =>
    http.post<Exercise>('/api/exercises', input),

  remove: (id: number) => http.delete<void>(`/api/exercises/${id}`),
};