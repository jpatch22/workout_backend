/**
 * Progress view: pick an exercise, then see how it has moved over time.
 *
 * The exercise lives in the URL (/progress/:slug) so a given chart is a
 * shareable, bookmarkable link and the browser back button works.
 */
import { useState, lazy, Suspense } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useExerciseProgress, useTrackedExercises } from '../hooks/useWorkouts';
import { Badge, Card, EmptyState, ErrorBanner, Spinner, StatCard } from '../components/ui';
import { formatDate, formatRelativeDay, formatVolume, formatWeight } from '../utils/formatting';

/*
 * Recharts is ~400 kB of the bundle and only the Progress page needs it, so it
 * is loaded on demand — the Dashboard, Workout, and History routes never pay
 * for it.
 */
const ProgressChart = lazy(() =>
  import('../components/ProgressChart').then((m) => ({ default: m.ProgressChart })),
);
const ProgressLegend = lazy(() =>
  import('../components/ProgressChart').then((m) => ({ default: m.ProgressLegend })),
);

export default function Progress() {
  const { slug } = useParams();
  const navigate = useNavigate();
  const { data: tracked, isLoading: loadingList } = useTrackedExercises();
  const {
    data: progress,
    isLoading: loadingChart,
    error: chartError,
  } = useExerciseProgress(slug ?? null);
  const [metric, setMetric] = useState<'estimated1Rm' | 'totalVolume'>('estimated1Rm');

  if (loadingList) return <Spinner />;

  if (!tracked || tracked.length === 0) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-semibold text-slate-100">Progress</h1>
        <EmptyState
          title="Nothing to chart yet"
          message="Log a few sessions and your strength curve, volume trend, and PR history will show up here."
          action={
            <Link to="/workout">
              <Badge tone="info">Start a workout</Badge>
            </Link>
          }
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-2xl font-semibold text-slate-100">Progress</h1>
        <p className="text-sm text-slate-500">
          {tracked.length} exercise{tracked.length === 1 ? '' : 's'} tracked
        </p>
      </div>

      <div className="grid gap-5 lg:grid-cols-[16rem_1fr] lg:items-start">
        <nav className="flex flex-col gap-1 lg:sticky lg:top-6">
          {tracked.map((ex) => {
            const active = ex.slug === slug;
            return (
              <button
                key={ex.exerciseId}
                type="button"
                onClick={() => navigate(`/progress/${ex.slug}`)}
                className={[
                  'flex items-center justify-between gap-2 rounded-lg px-3 py-2 text-left text-sm transition-colors',
                  active
                    ? 'bg-slate-800 text-emerald-400'
                    : 'text-slate-300 hover:bg-slate-800/60',
                ].join(' ')}
              >
                <span className="truncate">{ex.name}</span>
                <span className="shrink-0 text-xs tabular-nums text-slate-500">
                  {formatWeight(ex.bestEstimated1Rm)}
                </span>
              </button>
            );
          })}
        </nav>

        <div className="flex flex-col gap-4">
          {!slug ? (
            <EmptyState
              title="Select an exercise"
              message="Choose an exercise on the left to see its strength curve and volume trend."
            />
          ) : loadingChart ? (
            <Spinner />
          ) : chartError ? (
            <ErrorBanner message={chartError.message} />
          ) : progress ? (
            <>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <StatCard
                  label="Current e1RM"
                  value={formatWeight(progress.currentEstimated1Rm)}
                  accent
                />
                <StatCard label="Best e1RM" value={formatWeight(progress.bestEstimated1Rm)} />
                <StatCard label="Heaviest" value={`${formatWeight(progress.currentBestWeight)} lb`} />
                <StatCard label="PRs" value={progress.personalRecords} />
              </div>

              <Card
                title={progress.name}
                action={
                  <div className="flex gap-1">
                    {(['estimated1Rm', 'totalVolume'] as const).map((m) => (
                      <button
                        key={m}
                        type="button"
                        onClick={() => setMetric(m)}
                        className={[
                          'rounded-md px-2.5 py-1 text-xs font-medium transition-colors',
                          metric === m
                            ? 'bg-slate-800 text-emerald-400'
                            : 'text-slate-500 hover:text-slate-300',
                        ].join(' ')}
                      >
                        {m === 'estimated1Rm' ? 'Strength' : 'Volume'}
                      </button>
                    ))}
                  </div>
                }
              >
                <Suspense fallback={<Spinner label="Loading chart…" />}>
                  <ProgressChart progress={progress} metric={metric} />
                </Suspense>
                <div className="mt-2">
                  <ProgressLegend progress={progress} />
                </div>
              </Card>

              <Card title="Session history">
                <ul className="flex flex-col divide-y divide-slate-800">
                  {[...progress.points].reverse().map((p) => (
                    <li
                      key={`${p.date}-${p.workoutId}`}
                      className="flex items-center justify-between gap-3 py-2.5 text-sm"
                    >
                      <div>
                        <p className="text-slate-200">{formatDate(p.date)}</p>
                        <p className="text-xs text-slate-500">
                          {formatRelativeDay(p.date)} · {p.setCount} set{p.setCount === 1 ? '' : 's'}
                        </p>
                      </div>
                      <div className="flex items-center gap-4 text-right">
                        <span className="tabular-nums text-slate-300">
                          {formatWeight(p.bestWeight)} × {p.bestReps}
                        </span>
                        <span className="w-20 tabular-nums text-emerald-400">
                          {formatWeight(p.estimated1Rm)}
                        </span>
                        <span className="w-20 tabular-nums text-slate-500">
                          {formatVolume(p.totalVolume)} lb
                        </span>
                      </div>
                    </li>
                  ))}
                </ul>
              </Card>
            </>
          ) : null}
        </div>
      </div>
    </div>
  );
}