/**
 * Estimated 1RM over time for a single exercise.
 *
 * Deliberately built on Recharts' primitives rather than a wrapper component:
 * the domain is specific enough (two series, a PR scatter, custom tooltip) that
 * a generic chart component would need more configuration than this code.
 */
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import type { ExerciseProgress } from '../types';
import { formatDate, formatWeight } from '../utils/formatting';

interface Props {
  progress: ExerciseProgress;
  /** 'estimated1Rm' tracks strength; 'totalVolume' tracks work done. */
  metric: 'estimated1Rm' | 'totalVolume';
}

const SERIES = {
  estimated1Rm: { key: 'estimated1Rm' as const, label: 'Estimated 1RM' },
  totalVolume: { key: 'totalVolume' as const, label: 'Volume' },
};

export function ProgressChart({ progress, metric }: Props) {
  const series = SERIES[metric];

  const data = progress.points.map((p) => ({ ...p, label: formatDate(p.date) }));

  if (data.length === 0) {
    return (
      <p className="py-12 text-center text-sm text-slate-500">
        No sets logged for this exercise yet.
      </p>
    );
  }

  return (
    <div className="h-64 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 8, right: 12, bottom: 4, left: -12 }}>
          <CartesianGrid stroke="#1e293b" strokeDasharray="3 3" />
          <XAxis
            dataKey="label"
            stroke="#475569"
            tick={{ fill: '#64748b', fontSize: 11 }}
            tickLine={false}
            minTickGap={24}
          />
          <YAxis
            stroke="#475569"
            tick={{ fill: '#64748b', fontSize: 11 }}
            tickLine={false}
            width={56}
            domain={['auto', 'auto']}
          />
          <Tooltip content={<ChartTooltip />} cursor={{ stroke: '#334155' }} />
          <Line
            type="monotone"
            dataKey={series.key}
            name={series.label}
            stroke="#10b981"
            strokeWidth={2}
            dot={{ r: 3, fill: '#10b981', strokeWidth: 0 }}
            activeDot={{ r: 5 }}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}

interface TooltipProps {
  active?: boolean;
  payload?: Array<{
    payload: { date: string; bestWeight: number; bestReps: number; setCount: number };
  }>;
}

function ChartTooltip({ active, payload }: TooltipProps) {
  if (!active || !payload?.length) return null;
  const p = payload[0].payload;

  return (
    <div className="rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-xs shadow-lg">
      <p className="mb-1 font-medium text-slate-200">{formatDate(p.date)}</p>
      <p className="tabular-nums text-slate-400">
        {formatWeight(p.bestWeight)} × {p.bestReps}
      </p>
      <p className="text-slate-500">{p.setCount} sets</p>
    </div>
  );
}

export function ProgressLegend({ progress }: { progress: ExerciseProgress }) {
  const trend = progress.trendPercent;
  const direction = trend > 0.5 ? 'up' : trend < -0.5 ? 'down' : 'flat';
  const tone = {
    up: 'text-emerald-400',
    down: 'text-red-400',
    flat: 'text-slate-400',
  }[direction];

  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-500">
      <span>
        Trend from last session:{' '}
        <span className={`font-medium ${tone}`}>
          {trend > 0 ? '+' : ''}
          {trend.toFixed(1)}%
        </span>
      </span>
      <span>Best e1RM {formatWeight(progress.bestEstimated1Rm)}</span>
      <span>{progress.totalSessions} sessions</span>
      <span>{progress.personalRecords} PRs</span>
    </div>
  );
}