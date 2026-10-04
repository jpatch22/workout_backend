/** Typed bindings for the /api/auth endpoints. */
import { http } from './client';
import type { AuthResult, User } from '../types';

export const authApi = {
  register: (input: { email: string; password: string; displayName: string }) =>
    http.post<AuthResult>('/api/auth/register', input),

  login: (input: { email: string; password: string }) =>
    http.post<AuthResult>('/api/auth/login', input),

  me: () => http.get<User>('/api/auth/me'),

  updateProfile: (displayName: string) =>
    http.patch<User>('/api/auth/me', { displayName }),
};