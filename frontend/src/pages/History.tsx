/** Chronological list of past sessions with their derived totals. */
import { Link } from 'react-router-dom';
import { useDeleteWorkout, useWorkoutList } from '../hooks/useWorkouts';
import { Badge, Button, EmptyState, ErrorBanner, Spinner } from '../components/ui';
import { formatDate, formatVolume, formatWeight } from '../utils/formatting';

export default function History() {
  const { data: workouts, isLoading, error } = useWorkoutList();
  const deleteWorkout = useDeleteWorkout();

  if (isLoading) return <Spinner />;
  if (error) return <ErrorBanner message={error.message} />;

  if (!workouts || workouts.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-semibold text-slate-100">History</h1>
        <EmptyState
          title="No sessions logged"
          message="Workouts you log will appear here with volume, set counts, and best estimated 1RM per session."
          action={
            <Link to="/workout">
              <Button>Start a workout</Button>
            </Link>
          }
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-2xl font-semibold text-slate-100">History</h1>
        <p className="text-sm text-slate-500">
          {workouts.length} session{workouts.length === 1 ? '' : 's'}
        </p>
      </div>

      <div className="flex flex-col gap-3">
        {workouts.map((w) => (
          <div
            key={w.id}
            className="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-slate-800 bg-slate-900/60 px-4 py-3 transition-colors hover:border-slate-700"
          >
            <Link to={`/workout/${w.id}`} className="min-w-0 flex-1">
              <p className="truncate font-medium text-slate-100">{w.name}</p>
              <p className="mt-0.5 text-xs text-slate-500">
                {formatDate(w.performedAt)} · {w.exerciseCount} exercise
                {w.exerciseCount === 1 ? '' : 's'} · {w.setCount} sets
              </p>
            </Link>

            <div className="flex items-center gap-4 text-sm">
              <div className="text-right">
                <p className="tabular-nums text-slate-200">{formatVolume(w.totalVolume)} lb</p>
                <p className="text-xs text-slate-500">volume</p>
              </div>
              {w.topEstimated1Rm > 0 && (
                <div className="text-right">
                  <p className="tabular-nums text-slate-200">{formatWeight(w.topEstimated1Rm)}</p>
                  <p className="text-xs text-slate-500">
                    <Badge>est. 1RM</Badge>
                  </p>
                </div>
              )}
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  if (confirm(`Delete "${w.name}"? This cannot be undone.`)) {
                    deleteWorkout.mutate(w.id);
                  }
                }}
              >
                Delete
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}