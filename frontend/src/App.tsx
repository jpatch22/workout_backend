/**
 * Route table.
 *
 * Protected routes sit behind <RequireAuth>, which waits for the session to
 * restore before deciding — otherwise a page reload would flash the login
 * screen before the stored token resolves.
 */
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import type { ReactNode } from 'react';
import { useAuth } from './hooks/useAuth';
import { AppLayout } from './components/AppLayout';
import { Spinner } from './components/ui';
import Dashboard from './pages/Dashboard';
import History from './pages/History';
import Login from './pages/Login';
import Progress from './pages/Progress';
import Settings from './pages/Settings';
import WorkoutPage from './pages/Workout';

function RequireAuth({ children }: { children: ReactNode }) {
  const { status } = useAuth();
  const location = useLocation();

  if (status === 'loading') return <Spinner label="Restoring session…" />;
  if (status === 'unauthenticated') {
    // Remember where they were headed so login can send them back.
    return <Navigate to="/login" state={{ from: location }} replace />;
  }
  return <>{children}</>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />

      <Route
        element={
          <RequireAuth>
            <AppLayout />
          </RequireAuth>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="workout" element={<WorkoutPage />} />
        <Route path="workout/:id" element={<WorkoutPage />} />
        <Route path="history" element={<History />} />
        <Route path="progress" element={<Progress />} />
        <Route path="progress/:slug" element={<Progress />} />
        <Route path="settings" element={<Settings />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}