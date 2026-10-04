/**
 * Settings hooks.
 *
 * The active formula is read by many components, so it lives in a context rather
 * than requiring each one to issue its own query. Changing a setting invalidates
 * every derived value in the app — PR flags, chart series, dashboard numbers —
 * because they are all computed from the selected formula server-side.
 */
import { createContext, useCallback, useContext, useMemo } from 'react';
import type { ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { settingsApi } from '../api/settings';
import type { FormulaCatalog, Settings, SettingsPatch } from '../api/settings';
import { EPLEY, formulaUsesRPE } from '../utils/e1rm';

/** Invalidates everything whose numbers depend on the formula. */
function useInvalidateDerived() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: ['settings'] });
    void qc.invalidateQueries({ queryKey: ['dashboard'] });
    void qc.invalidateQueries({ queryKey: ['workouts'] });
    void qc.invalidateQueries({ queryKey: ['tracked-exercises'] });
    void qc.invalidateQueries({ queryKey: ['exercise-progress'] });
  };
}

export function useSettings() {
  return useQuery({
    queryKey: ['settings'],
    queryFn: settingsApi.get,
    // Settings rarely change and are read on nearly every screen; avoid a
    // refetch storm while the user logs a set.
    staleTime: 5 * 60 * 1000,
  });
}

export function useFormulaCatalog() {
  return useQuery({
    queryKey: ['settings', 'formulas'],
    queryFn: settingsApi.formulas,
    staleTime: Infinity,
  });
}

export function useUpdateSettings() {
  const invalidate = useInvalidateDerived();
  return useMutation({
    mutationFn: (patch: SettingsPatch) => settingsApi.update(patch),
    onSuccess: invalidate,
  });
}

/**
 * SettingsContext exposes the resolved settings to components that need them
 * without a hook call per consumer (the set row's preview in particular).
 */
interface SettingsContextValue {
  settings: Settings | undefined;
  /** The active formula id; Epley until settings load. */
  formula: string;
  /** Whether the active formula reads RPE. */
  usesRPE: boolean;
  /** Whether the user turned on RPE capture. */
  rpeEnabled: boolean;
}

const SettingsContext = createContext<SettingsContextValue | null>(null);

export function SettingsProvider({ children }: { children: ReactNode }) {
  const { data } = useSettings();

  const value = useMemo<SettingsContextValue>(() => {
    const formula = data?.oneRmFormula ?? EPLEY;
    return {
      settings: data,
      formula,
      // RPE only matters if the user both enabled capture and picked a formula
      // that consumes it.
      usesRPE: formulaUsesRPE(formula),
      rpeEnabled: data?.rpeEnabled ?? false,
    };
  }, [data]);

  return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>;
}

export function useSettingsContext(): SettingsContextValue {
  const ctx = useContext(SettingsContext);
  if (!ctx) {
    // Falling back rather than throwing keeps components usable in isolation,
    // e.g. in a unit test that renders SetRow without the whole tree.
    return { settings: undefined, formula: EPLEY, usesRPE: false, rpeEnabled: false };
  }
  return ctx;
}

export type { FormulaCatalog, Settings, SettingsPatch };

// Re-exported so callers can build a patch object without importing the API layer.
export const useSettingsUpdate = useUpdateSettings;

export function useToggleRPE() {
  const { settings } = useSettingsContext();
  const update = useUpdateSettings();
  const current = settings?.rpeEnabled ?? false;
  const toggle = useCallback(() => update.mutate({ rpeEnabled: !current }), [current, update]);
  return { enabled: current, toggle, isPending: update.isPending };
}