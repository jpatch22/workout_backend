/** Typed bindings for user-owned programs and their run progress. */
import { http } from './client';
import type { Program, ProgramInput, Workout } from '../types';

export const programApi = {
  list: () => http.get<Program[]>('/api/programs'),
  create: (input: ProgramInput) => http.post<Program>('/api/programs', input),
  startSession: (programId: number, sessionId: string) =>
    http.post<Workout>(`/api/programs/${programId}/sessions/${encodeURIComponent(sessionId)}/start`),
  completeSession: (programId: number, sessionId: string) =>
    http.post<Program>(`/api/programs/${programId}/sessions/${encodeURIComponent(sessionId)}/complete`),
  skipSession: (programId: number, sessionId: string) =>
    http.post<Program>(`/api/programs/${programId}/sessions/${encodeURIComponent(sessionId)}/skip`),
  restart: (programId: number) => http.post<Program>(`/api/programs/${programId}/restart`),
};
