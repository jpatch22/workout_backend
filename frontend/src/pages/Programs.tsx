/** First pass at ordered programs: browser-saved plans and run progress. */
import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import {
  useCompleteProgramSession,
  useCreateProgram,
  useExerciseCatalog,
  usePrograms,
  useRestartProgram,
  useSkipProgramSession,
  useStartProgramSession,
} from '../hooks/useWorkouts';
import { Badge, Button, Card, EmptyState, ErrorBanner, Input, Spinner } from '../components/ui';
import type { Exercise, ProgramExercise, ProgramInput, ProgramSessionStatus } from '../types';

interface DraftSession {
  id: string;
  name: string;
  exercises: ProgramExercise[];
}
interface DraftWeek {
  id: string;
  name: string;
  sessions: DraftSession[];
}

interface LegacyProgramSession {
  id?: string;
  name?: string;
  exercises?: ProgramExercise[];
  exerciseIds?: number[];
  status?: ProgramSessionStatus;
  workoutId?: number;
}

interface LegacyProgram {
  id?: string;
  name?: string;
  description?: string;
  sessions?: LegacyProgramSession[];
  weeks?: Array<{ id?: string; name?: string; sessions: LegacyProgramSession[] }>;
}

function migrateLegacyProgram(program: LegacyProgram, index: number): ProgramInput {
  const weeks = program.weeks ?? [{ id: undefined, name: 'Week 1', sessions: program.sessions ?? [] }];
  return {
    name: program.name?.trim() || `Imported program ${index + 1}`,
    description: program.description?.slice(0, 240) ?? '',
    weeks: weeks.map((week, weekIndex) => ({
      id: week.id || crypto.randomUUID(),
      name: week.name?.trim() || `Week ${weekIndex + 1}`,
      sessions: week.sessions.map((session, sessionIndex) => ({
        id: session.id || crypto.randomUUID(),
        name: session.name?.trim() || `Session ${sessionIndex + 1}`,
        status: session.status,
        workoutId: session.workoutId,
        exercises: session.exercises ?? (session.exerciseIds ?? []).map((exerciseId) => ({
          exerciseId,
          sets: Array.from({ length: 3 }, () => ({ id: crypto.randomUUID(), reps: 8 })),
          notes: '',
        })),
      })),
    })),
  };
}

function sessionLabel(status: ProgramSessionStatus) {
  if (status === 'completed') return 'Completed';
  if (status === 'skipped') return 'Skipped';
  return 'Up next';
}

export default function Programs() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const programsQuery = usePrograms();
  const { data: catalog, isLoading: catalogLoading, error: catalogError } = useExerciseCatalog();
  const createProgramMutation = useCreateProgram();
  const startSessionMutation = useStartProgramSession();
  const completeSessionMutation = useCompleteProgramSession();
  const skipSessionMutation = useSkipProgramSession();
  const restartProgramMutation = useRestartProgram();
  const programs = programsQuery.data ?? [];
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [programOpen, setProgramOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [programName, setProgramName] = useState('');
  const [description, setDescription] = useState('');
  const [draftWeeks, setDraftWeeks] = useState<DraftWeek[]>([]);
  const [error, setError] = useState<string>();
  const legacyMigrationStarted = useRef(false);

  useEffect(() => {
    if (!user || !programsQuery.isSuccess || legacyMigrationStarted.current) return;
    legacyMigrationStarted.current = true;
    const legacyKey = `workout-tracker.programs.${user.id}`;
    const progressKey = `workout-tracker.program-migration.${user.id}`;
    const raw = localStorage.getItem(legacyKey);
    if (!raw) return;

    void (async () => {
      try {
        const legacy = JSON.parse(raw) as LegacyProgram[];
        const imported = new Set(JSON.parse(localStorage.getItem(progressKey) ?? '[]') as string[]);
        for (const [index, program] of legacy.entries()) {
          const oldId = program.id ?? `legacy-${index}`;
          if (imported.has(oldId)) continue;
          await createProgramMutation.mutateAsync(migrateLegacyProgram(program, index));
          imported.add(oldId);
          localStorage.setItem(progressKey, JSON.stringify([...imported]));
        }
        localStorage.removeItem(legacyKey);
        localStorage.removeItem(progressKey);
        await programsQuery.refetch();
      } catch (err) {
        setError(err instanceof Error ? `Could not move your browser-saved programs to your account: ${err.message}` : 'Could not move your browser-saved programs to your account.');
      }
    })();
  }, [user, programsQuery.isSuccess, programsQuery.refetch, createProgramMutation.mutateAsync]);


  const selected = programs.find((program) => program.id === selectedId) ?? null;
  const orderedSessions = selected?.weeks.flatMap((week) => week.sessions) ?? [];
  const nextSession = orderedSessions.find((session) => session.status === 'upcoming');
  const inProgressSession = orderedSessions.find((session) => session.workoutId && session.status === 'upcoming');
  const sequenceFinished = orderedSessions.length > 0 && orderedSessions.every((session) => session.status !== 'upcoming');
  const exerciseById = useMemo(() => new Map((catalog ?? []).map((exercise) => [exercise.id, exercise])), [catalog]);

  function addDraftWeek() {
    setDraftWeeks((current) => [...current, { id: crypto.randomUUID(), name: `Week ${current.length + 1}`, sessions: [] }]);
  }

  function addDraftSession(weekId: string) {
    setDraftWeeks((current) => current.map((week) => week.id !== weekId ? week : ({
      ...week,
      sessions: [...week.sessions, {
        id: crypto.randomUUID(),
        name: `Day ${String.fromCharCode(65 + week.sessions.length)}`,
        exercises: [],
      }],
    })));
  }

  async function createProgram() {
    const name = programName.trim();
    if (!name || draftWeeks.length === 0 || draftWeeks.some((week) => week.sessions.length === 0 || week.sessions.some((session) => session.exercises.length === 0))) {
      setError('Add a program name, at least one session to each week, and an exercise to each session.');
      return;
    }
    try {
      const program = await createProgramMutation.mutateAsync({ name, description: description.trim(), weeks: draftWeeks });
      setSelectedId(program.id);
      setProgramOpen(true);
      setProgramName('');
      setDescription('');
      setDraftWeeks([]);
      setCreating(false);
      setError(undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not save the program.');
    }
  }

  function toggleExercise(sessionId: string, exerciseId: number) {
    setDraftWeeks((current) => current.map((week) => ({
      ...week,
      sessions: week.sessions.map((session) => session.id !== sessionId ? session : {
        ...session,
        exercises: session.exercises.some((exercise) => exercise.exerciseId === exerciseId)
          ? session.exercises.filter((exercise) => exercise.exerciseId !== exerciseId)
          : [...session.exercises, {
            exerciseId,
            sets: Array.from({ length: 3 }, () => ({ id: crypto.randomUUID(), reps: 8 })),
            notes: '',
          }],
      }),
    })));
  }

  function updateDraftExercise(sessionId: string, exerciseId: number, update: (exercise: ProgramExercise) => ProgramExercise) {
    setDraftWeeks((current) => current.map((week) => ({
      ...week,
      sessions: week.sessions.map((session) => session.id !== sessionId ? session : ({
        ...session,
        exercises: session.exercises.map((exercise) => exercise.exerciseId === exerciseId ? update(exercise) : exercise),
      })),
    })));
  }

  async function startNext() {
    if (!selected || !nextSession || startSessionMutation.isPending) return;
    if (inProgressSession?.workoutId) {
      navigate(`/workout/${inProgressSession.workoutId}`);
      return;
    }
    setError(undefined);
    try {
      const workout = await startSessionMutation.mutateAsync({ programId: selected.id, sessionId: nextSession.id });
      navigate(`/workout/${workout.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not start this session.');
    }
  }

  if (programsQuery.isLoading) return <Spinner />;

  const requestError = programsQuery.error ?? catalogError ?? createProgramMutation.error ??
    startSessionMutation.error ?? completeSessionMutation.error ?? skipSessionMutation.error ?? restartProgramMutation.error;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-slate-100">Programs</h1>
          <p className="mt-1 text-sm text-slate-500">Build a sequence, then complete it at your own pace.</p>
        </div>
        {!creating && <Button onClick={() => { setCreating(true); setSelectedId(null); setProgramOpen(false); setError(undefined); setDraftWeeks([{ id: crypto.randomUUID(), name: 'Week 1', sessions: [] }]); }}>Create program</Button>}
      </div>

      <ErrorBanner message={error ?? requestError?.message} onDismiss={() => setError(undefined)} />

      {creating ? (
        <div className="flex flex-col gap-5">
          <Card title="New program">
            <div className="flex flex-col gap-4">
              <Input label="Program name" placeholder="e.g. Beginner Strength" value={programName} onChange={(event) => setProgramName(event.target.value)} maxLength={100} />
              <Input label="Description (optional)" placeholder="A short note about the plan" value={description} onChange={(event) => setDescription(event.target.value)} maxLength={240} />
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-sm font-semibold text-slate-200">Program weeks</h2>
                  <p className="text-xs text-slate-500">Sessions advance in order. Weeks organize the plan; they have no calendar dates.</p>
                </div>
                <Button variant="secondary" size="sm" onClick={addDraftWeek}>Add week</Button>
              </div>
            </div>
          </Card>

          {catalogLoading ? <Spinner /> : draftWeeks.map((week) => (
            <Card key={week.id} title={week.name} action={
              <button type="button" className="text-xs text-slate-500 hover:text-red-400" onClick={() => setDraftWeeks((current) => current.filter((item) => item.id !== week.id))}>Remove week</button>
            }>
              <div className="flex flex-col gap-4">
                <Input label="Week name" value={week.name} onChange={(event) => setDraftWeeks((current) => current.map((item) => item.id === week.id ? { ...item, name: event.target.value } : item))} maxLength={80} />
                {week.sessions.map((session, index) => (
                  <section key={session.id} className="flex flex-col gap-3 rounded-lg border border-slate-800 p-3">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-medium text-slate-200">Session {index + 1}</h3>
                      <button type="button" className="text-xs text-slate-500 hover:text-red-400" onClick={() => setDraftWeeks((current) => current.map((item) => item.id !== week.id ? item : ({ ...item, sessions: item.sessions.filter((row) => row.id !== session.id) })))}>Remove session</button>
                    </div>
                    <Input label="Session name" value={session.name} onChange={(event) => setDraftWeeks((current) => current.map((item) => item.id !== week.id ? item : ({ ...item, sessions: item.sessions.map((row) => row.id === session.id ? { ...row, name: event.target.value } : row) })))} maxLength={80} />
                    <p className="text-xs font-medium uppercase tracking-wide text-slate-500">Choose exercises</p>
                    <div className="grid gap-2 sm:grid-cols-2">
                      {(catalog ?? []).map((exercise: Exercise) => (
                        <label key={exercise.id} className="flex cursor-pointer items-center gap-2 rounded-lg border border-slate-800 px-3 py-2 text-sm text-slate-300 hover:border-slate-700">
                          <input type="checkbox" checked={session.exercises.some((item) => item.exerciseId === exercise.id)} onChange={() => toggleExercise(session.id, exercise.id)} className="accent-emerald-600" />
                          <span>{exercise.name}</span>
                        </label>
                      ))}
                    </div>
                    {session.exercises.map((programExercise) => {
                      const exercise = exerciseById.get(programExercise.exerciseId);
                      return <div key={programExercise.exerciseId} className="flex flex-col gap-3 rounded-lg border border-slate-800 bg-slate-950/40 p-3">
                        <h4 className="text-sm font-medium text-slate-200">{exercise?.name ?? 'Exercise'} · target sets</h4>
                        <div className="flex flex-col gap-2">
                          {programExercise.sets.map((set, setIndex) => <div key={set.id} className="grid grid-cols-[auto_1fr_1fr_auto] items-end gap-2">
                            <span className="pb-2 text-xs text-slate-500">Set {setIndex + 1}</span>
                            <Input aria-label={`${exercise?.name ?? 'Exercise'} set ${setIndex + 1} reps`} label={setIndex === 0 ? 'Reps' : undefined} type="number" min={1} max={100} value={set.reps} onChange={(event) => {
                              const reps = Number(event.target.value);
                              updateDraftExercise(session.id, programExercise.exerciseId, (item) => ({ ...item, sets: item.sets.map((row) => row.id === set.id ? { ...row, reps: Number.isFinite(reps) ? Math.min(100, Math.max(1, Math.round(reps))) : 1 } : row) }));
                            }} />
                            <Input aria-label={`${exercise?.name ?? 'Exercise'} set ${setIndex + 1} target RPE`} label={setIndex === 0 ? 'Target RPE' : undefined} type="number" min={1} max={10} step={0.5} placeholder="Optional" value={set.rpe ?? ''} onChange={(event) => {
                              const value = event.target.value;
                              updateDraftExercise(session.id, programExercise.exerciseId, (item) => ({ ...item, sets: item.sets.map((row) => row.id === set.id ? { ...row, rpe: value === '' ? undefined : Math.min(10, Math.max(1, Number(value))) } : row) }));
                            }} />
                            <button type="button" aria-label={`Remove set ${setIndex + 1}`} disabled={programExercise.sets.length === 1} onClick={() => updateDraftExercise(session.id, programExercise.exerciseId, (item) => ({ ...item, sets: item.sets.filter((row) => row.id !== set.id) }))} className="pb-2 text-sm text-slate-500 hover:text-red-400 disabled:opacity-40">Remove</button>
                          </div>)}
                        </div>
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <Button size="sm" variant="secondary" onClick={() => updateDraftExercise(session.id, programExercise.exerciseId, (item) => ({ ...item, sets: [...item.sets, { id: crypto.randomUUID(), reps: item.sets.at(-1)?.reps ?? 8, rpe: item.sets.at(-1)?.rpe }] }))}>Add set</Button>
                          <Input aria-label={`${exercise?.name ?? 'Exercise'} notes`} placeholder="Exercise notes (optional)" value={programExercise.notes} onChange={(event) => updateDraftExercise(session.id, programExercise.exerciseId, (item) => ({ ...item, notes: event.target.value }))} maxLength={500} className="min-w-64 flex-1" />
                        </div>
                      </div>;
                    })}
                    {session.exercises.length === 0 && <p className="text-xs text-amber-400">Choose at least one exercise for this session.</p>}
                  </section>
                ))}
                <div>
                  <Button variant="secondary" size="sm" onClick={() => addDraftSession(week.id)}>Add session to {week.name}</Button>
                  {week.sessions.length === 0 && <p className="mt-2 text-xs text-amber-400">Add at least one session to this week.</p>}
                </div>
              </div>
            </Card>
          ))}

          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => { setCreating(false); setDraftWeeks([]); setSelectedId(null); setProgramOpen(false); setError(undefined); }}>Cancel</Button>
            <Button onClick={() => void createProgram()} disabled={createProgramMutation.isPending}>
              {createProgramMutation.isPending ? 'Saving…' : 'Save program'}
            </Button>
          </div>
        </div>
      ) : programs.length === 0 ? (
        <EmptyState title="No programs yet" message="Create a program by grouping sessions into weeks. The schedule stays flexible, without calendar dates." action={<Button onClick={() => { setCreating(true); setDraftWeeks([{ id: crypto.randomUUID(), name: 'Week 1', sessions: [] }]); }}>Create your first program</Button>} />
      ) : programOpen && selected ? (
        <div className="flex flex-col gap-4">
          <Button variant="ghost" className="self-start px-0" onClick={() => setProgramOpen(false)}>← All programs</Button>
          <Card title={selected.name} action={<Badge tone="info">Flexible sequence</Badge>}>
            <div className="flex flex-col gap-4">
              {selected.description && <p className="text-sm text-slate-400">{selected.description}</p>}
              <p className="text-xs text-slate-500">{selected.weeks.length} weeks · {orderedSessions.length} sessions in a flexible sequence</p>
              <div className="flex flex-wrap gap-2 text-xs text-slate-500">
                <span>{orderedSessions.filter((session) => session.status === 'completed').length} completed</span>
                <span>·</span>
                <span>{orderedSessions.filter((session) => session.status === 'skipped').length} skipped</span>
                <span>·</span>
                <span>{orderedSessions.filter((session) => session.status === 'upcoming').length} remaining</span>
              </div>
              <div className="flex flex-wrap gap-2">
                {nextSession ? <Button onClick={() => void startNext()} disabled={startSessionMutation.isPending}>
                  {startSessionMutation.isPending ? 'Starting…' : inProgressSession ? 'Resume current workout' : `Start ${nextSession.name}`}
                </Button> : <Badge tone="warmup">Program sequence complete</Badge>}
                {nextSession && !inProgressSession && <Button variant="secondary" disabled={skipSessionMutation.isPending} onClick={() => skipSessionMutation.mutate({ programId: selected.id, sessionId: nextSession.id })}>Skip this session</Button>}
                {inProgressSession && <Button variant="secondary" disabled={completeSessionMutation.isPending} onClick={() => completeSessionMutation.mutate({ programId: selected.id, sessionId: inProgressSession.id })}>Mark complete</Button>}
                {sequenceFinished && <Button variant="secondary" disabled={restartProgramMutation.isPending} onClick={() => restartProgramMutation.mutate(selected.id)}>Restart program</Button>}
              </div>
              {inProgressSession && <p className="text-xs text-slate-500">Mark complete after you finish logging this workout. You can also resume it above.</p>}
            </div>
          </Card>

          {selected.weeks.map((week, weekIndex) => <Card key={week.id} title={week.name} action={<span className="text-xs text-slate-500">Week {weekIndex + 1}</span>}>
            <ol className="flex flex-col divide-y divide-slate-800">
              {week.sessions.map((session, index) => (
                <li key={session.id} className="flex flex-wrap items-center justify-between gap-3 py-3 first:pt-0 last:pb-0">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-xs tabular-nums text-slate-500">{String(index + 1).padStart(2, '0')}</span>
                      <span className="font-medium text-slate-200">{session.name}</span>
                      <Badge tone={session.status === 'completed' ? 'info' : session.status === 'skipped' ? 'warmup' : 'neutral'}>{sessionLabel(session.status)}</Badge>
                    </div>
                    <div className="mt-1 flex flex-col gap-1 pl-8 text-xs text-slate-500">
                      {session.exercises.map((item) => <p key={item.exerciseId}>
                        <span className="text-slate-400">{exerciseById.get(item.exerciseId)?.name ?? 'Exercise'}:</span>{' '}
                        {item.sets.map((set) => `${set.reps}${set.rpe === undefined ? '' : ` @${set.rpe}`}`).join(' / ')} reps
                        {item.notes.trim() && <span> · {item.notes}</span>}
                      </p>)}
                    </div>
                  </div>
                  {session.status === 'upcoming' && session.workoutId && <Button size="sm" variant="ghost" onClick={() => navigate(`/workout/${session.workoutId}`)}>Open workout</Button>}
                </li>
              ))}
            </ol>
            {week.sessions.length === 0 && <p className="text-sm text-slate-500">No sessions in this week.</p>}
          </Card>)}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
            {programs.map((program) => {
              const sessions = program.weeks.flatMap((week) => week.sessions);
              const done = sessions.filter((session) => session.status === 'completed').length;
              const skipped = sessions.filter((session) => session.status === 'skipped').length;
              return <Card key={program.id} title={program.name}>
                <div className="flex flex-col gap-3">
                  {program.description && <p className="text-sm text-slate-400">{program.description}</p>}
                  <p className="text-xs text-slate-500">{program.weeks.length} weeks · {sessions.length} sessions</p>
                  <p className="text-xs text-slate-500">{done} completed · {skipped} skipped · {sessions.length - done - skipped} remaining</p>
                  <Button variant="secondary" className="self-start" onClick={() => { setSelectedId(program.id); setProgramOpen(true); setError(undefined); }}>Open program</Button>
                </div>
              </Card>;
            })}
        </div>
      )}
    </div>
  );
}
