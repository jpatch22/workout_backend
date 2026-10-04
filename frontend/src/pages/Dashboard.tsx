/** Landing screen: headline stats, recent sessions, and exercises in rotation. */
import { Link, useNavigate } from 'react-router-dom';
import { useDashboard } from '../hooks/useWorkouts';
import { useCreateWorkout } from '../hooks/useWorkouts';
import { Badge, Button, Card, EmptyState, ErrorBanner, Spinner, StatCard } from '../components/ui';
import { formatRelativeDay, formatVolume, formatWeight } from '../utils/formatting';

export default function Dashboard() {
  const navigate = useNavigate();
  const { data, isLoading, error } = useDashboard();
  const createWorkout = useCreateWorkout();

  async function startWorkout() {
    const wk = await createWorkout.mutateAsync({});
    navigate(`/workout/${wk.id}`);
  }

  if (isLoading) return <Spinner />;
  if (error) return <ErrorBanner message={error.message} />;
  if (!data) return null;

  const { stats, recentWorkouts, trackedExercises } = data;
  const hasHistory = stats.totalWorkouts > 0;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-slate-100">Dashboard</h1>
          <p className="text-sm text-slate-500">
            {hasHistory
              ? `${stats.workoutsThisWeek} session${stats.workoutsThisWeek === 1 ? '' : 's'} this week`
              : 'Log your first session to start tracking'}
          </p>
        </div>
        <Button onClick={startWorkout} disabled={createWorkout.isPending}>
          {createWorkout.isPending ? 'Starting…' : 'Start workout'}
        </Button>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard label="Total volume" value={formatVolume(stats.totalVolume)} sub="lb · all time" accent />
        <StatCard label="Workouts" value={stats.totalWorkouts} />
        <StatCard label="PRs this month" value={stats.prsThisMonth} accent={stats.prsThisMonth > 0} />
        <StatCard label="Streak" value={`${stats.streakDays}d`} sub="consecutive days" />
      </div>

      {!hasHistory ? (
        <EmptyState
          title="No workouts yet"
          message="Start a session, add the exercises you're doing, and log your sets. Volume, estimated 1RM and PRs are all derived automatically."
          action={<Button onClick={startWorkout}>Start your first workout</Button>}
        />
      ) : (
        <div className="grid gap-6 lg:grid-cols-2">
          <Card title="Recent sessions">
            <ul className="flex flex-col divide-y divide-slate-800">
              {recentWorkouts.map((w) => (
                <li key={w.id}>
                  <Link
                    to={`/workout/${w.id}`}
                    className="flex items-center justify-between gap-3 py-2.5 transition-colors hover:text-emerald-400"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-slate-200">{w.name}</p>
                      <p className="text-xs text-slate-500">
                        {formatRelativeDay(w.performedAt)} · {w.exerciseCount} exercise
                        {w.exerciseCount === 1 ? '' : 's'} · {w.setCount} sets
                      </p>
                    </div>
                    <div className="text-right">
                      <p className="text-sm tabular-nums text-slate-300">{formatVolume(w.totalVolume)} lb</p>
                      {w.topEstimated1Rm > 0 && (
                        <p className="text-xs tabular-nums text-slate-500">
                          e1RM {formatWeight(w.topEstimated1Rm)}
                        </p>
                      )}
                    </div>
                  </Link>
                </li>
              ))}
            </ul>
          </Card>

          <Card title="Exercises in rotation">
            {trackedExercises.length === 0 ? (
              <p className="py-6 text-center text-sm text-slate-500">
                Once you log sets, your exercises appear here.
              </p>
            ) : (
              <ul className="flex flex-col divide-y divide-slate-800">
                {trackedExercises.map((ex) => (
                  <li key={ex.exerciseId}>
                    <Link
                      to={`/progress/${ex.slug}`}
                      className="flex items-center justify-between gap-3 py-2.5 transition-colors hover:text-emerald-400"
                    >
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium text-slate-200">{ex.name}</p>
                        <p className="text-xs text-slate-500">
                          {ex.sessionCount} session{ex.sessionCount === 1 ? '' : 's'} · last{' '}
                          {formatRelativeDay(ex.lastTrained).toLowerCase()}
                        </p>
                      </div>
                      <div className="flex items-center gap-2">
                        {ex.muscleGroup && <Badge>{ex.muscleGroup}</Badge>}
                        <span className="text-sm tabular-nums text-slate-300">
                          {formatWeight(ex.bestEstimated1Rm)}
                        </span>
                      </div>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}