/** Typed bindings for the /api/settings endpoints. */
import { http } from './client';

export interface Settings {
  oneRmFormula: string;
  rpeEnabled: boolean;
  assumeRpe: number;
  plateBarWeight: number;
  weightUnit: 'lb' | 'kg';
}

/**
 * Partial settings update. Omitted keys are left unchanged, so a single
 * preference can be saved without resending the rest.
 */
export interface SettingsPatch {
  oneRmFormula?: string;
  rpeEnabled?: boolean;
  assumeRpe?: number;
  plateBarWeight?: number;
  weightUnit?: 'lb' | 'kg';
}

export interface FormulaOption {
  id: string;
  name: string;
  description: string;
  usesRpe: boolean;
}

export interface FormulaCatalog {
  formulas: FormulaOption[];
  default: string;
}

export const settingsApi = {
  get: () => http.get<Settings>('/api/settings'),

  update: (patch: SettingsPatch) => http.patch<Settings>('/api/settings', patch),

  /** Formulas the backend supports, so the UI never hardcodes them. */
  formulas: () => http.get<FormulaCatalog>('/api/settings/formulas'),
};