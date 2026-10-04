/** Exercise picker: searchable catalog plus inline creation of a new movement. */
import { useMemo, useState } from 'react';
import type { Exercise } from '../types';
import { Badge, Button, Input } from './ui';

interface Props {
  catalog: Exercise[];
  /** Ids already in this workout, so they can be shown as already added. */
  existing: number[];
  onPick: (exercise: Exercise) => void;
  onCreate: (name: string) => Promise<void>;
  creating?: boolean;
  disabled?: boolean;
}

export function ExerciseSelector({ catalog, existing, onPick, onCreate, creating, disabled }: Props) {
  const [query, setQuery] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return catalog;
    return catalog.filter((e) => e.name.toLowerCase().includes(q));
  }, [catalog, query]);

  // Offer "create X" only when the query isn't an exact catalog hit.
  const exactMatch = catalog.some((e) => e.name.toLowerCase() === query.trim().toLowerCase());
  const canCreate = query.trim().length > 1 && !exactMatch;

  async function handleCreate() {
    const name = newName.trim() || query.trim();
    if (!name) return;
    await onCreate(name);
    setNewName('');
    setQuery('');
    setShowCreate(false);
  }

  return (
    <div className="rounded-xl border border-slate-800 bg-slate-900/60 p-4">
      <Input
        label="Add exercise"
        placeholder="Search or type a new exercise…"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        disabled={disabled}
      />

      {canCreate && !showCreate && (
        <button
          type="button"
          onClick={() => {
            setNewName(query.trim());
            setShowCreate(true);
          }}
          className="mt-2 w-full rounded-lg border border-dashed border-slate-700 px-3 py-2 text-left text-sm text-slate-400 transition-colors hover:border-emerald-600 hover:text-emerald-400"
        >
          Create “{query.trim()}” as a new exercise
        </button>
      )}

      {showCreate && (
        <div className="mt-3 flex flex-col gap-2 rounded-lg border border-emerald-800/50 bg-emerald-950/20 p-3">
          <Input
            name="newExercise"
            placeholder="Exercise name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
          />
          <div className="flex gap-2">
            <Button size="sm" onClick={handleCreate} disabled={creating || !newName.trim()}>
              {creating ? 'Creating…' : 'Create'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setShowCreate(false)}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      <ul className="mt-3 max-h-72 flex-col gap-1 overflow-y-auto">
        {filtered.map((ex) => {
          const added = existing.includes(ex.id);
          return (
            <li key={ex.id}>
              <button
                type="button"
                onClick={() => onPick(ex)}
                disabled={disabled || added}
                className={[
                  'flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left transition-colors',
                  added
                    ? 'cursor-default text-slate-600'
                    : 'text-slate-200 hover:bg-slate-800/70',
                ].join(' ')}
              >
                <span className="flex items-center gap-2">
                  <span className="text-sm">{ex.name}</span>
                  {ex.isCustom && <Badge tone="info">custom</Badge>}
                  {added && <Badge tone="warmup">added</Badge>}
                </span>
                {ex.muscleGroup && <Badge>{ex.muscleGroup}</Badge>}
              </button>
            </li>
          );
        })}
        {filtered.length === 0 && !canCreate && (
          <li className="py-4 text-center text-sm text-slate-500">No exercises match “{query}”.</li>
        )}
      </ul>
    </div>
  );
}