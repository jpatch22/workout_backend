/** Sign-in and registration, sharing one screen with a mode toggle. */
import { useState } from 'react';
import type { FormEvent } from 'react';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import { ApiError } from '../api/client';
import { useAuth } from '../hooks/useAuth';
import { Button, ErrorBanner, Input } from '../components/ui';

type Mode = 'login' | 'register';

export default function Login() {
  const { login, register, status } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const [mode, setMode] = useState<Mode>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [error, setError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);

  const from = (location.state as { from?: { pathname: string } } | null)?.from?.pathname ?? '/';

  if (status === 'authenticated') return <Navigate to={from} replace />;

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(undefined);
    setSubmitting(true);
    try {
      if (mode === 'login') {
        await login(email, password);
      } else {
        await register(email, password, displayName);
      }
      navigate(from, { replace: true });
    } catch (err) {
      // Validation errors name the offending field; show it inline-ish.
      if (err instanceof ApiError) {
        setError(err.field ? `${err.field} ${err.message}` : err.message);
      } else {
        setError('Something went wrong. Please try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="grid min-h-screen place-items-center bg-slate-950 px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-2">
          <span className="grid h-11 w-11 place-items-center rounded-xl bg-emerald-600 text-lg font-bold">
            W
          </span>
          <h1 className="text-xl font-semibold text-slate-100">Workout Tracker</h1>
          <p className="text-sm text-slate-500">
            {mode === 'login' ? 'Sign in to log your training' : 'Create an account to get started'}
          </p>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4 rounded-xl border border-slate-800 bg-slate-900/60 p-6">
          <ErrorBanner message={error} onDismiss={() => setError(undefined)} />

          {mode === 'register' && (
            <Input
              label="Name"
              name="displayName"
              autoComplete="name"
              placeholder="Alex"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          )}

          <Input
            label="Email"
            name="email"
            type="email"
            required
            autoComplete="email"
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />

          <Input
            label="Password"
            name="password"
            type="password"
            required
            autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
            placeholder="••••••••"
            hint={mode === 'register' ? 'At least 8 characters, with a letter and a digit or symbol' : undefined}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />

          <Button type="submit" size="lg" fullWidth disabled={submitting}>
            {submitting ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}
          </Button>

          <p className="text-center text-sm text-slate-500">
            {mode === 'login' ? "Don't have an account? " : 'Already have an account? '}
            <button
              type="button"
              className="font-medium text-emerald-400 hover:text-emerald-300"
              onClick={() => {
                setMode(mode === 'login' ? 'register' : 'login');
                setError(undefined);
              }}
            >
              {mode === 'login' ? 'Sign up' : 'Sign in'}
            </button>
          </p>
        </form>
      </div>
    </div>
  );
}