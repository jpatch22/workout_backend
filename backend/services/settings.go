package services

import (
	"context"

	"github.com/jackpatchell/workout-tracker/backend/formulas"
	"github.com/jackpatchell/workout-tracker/backend/tables"
)

// Settings is the user-facing shape of the preferences document.
//
// Every field is concrete and always populated: the service fills defaults when
// reading, so a client never has to distinguish "unset" from "default", and the
// frontend can bind to these values without defensive checks.
type Settings struct {
	// OneRMFormula selects the estimated-1RM method used for PR flags and the
	// progress charts.
	OneRMFormula formulas.Formula `json:"oneRmFormula"`
	// RPEEnabled turns RPE capture on. When false the UI omits the RPE field and
	// every set is treated as having no RPE, which the formula reads as 10.
	RPEEnabled bool `json:"rpeEnabled"`
	// AssumeRPE is what to use for a set with no RPE recorded. Always 10 in
	// practice; exposed so the choice is explicit in the API and can change
	// without a contract break.
	AssumeRPE float64 `json:"assumeRpe"`
	// PlateBarWeight is the bar used when working out which plates to load.
	PlateBarWeight float64 `json:"plateBarWeight"`
	// WeightUnit is "lb" or "kg". Display only for now — values are stored in
	// whatever unit was logged.
	WeightUnit string `json:"weightUnit"`
}

// Preference keys as they appear in the JSONB document.
const (
	prefKeyOneRMFormula   = "oneRmFormula"
	prefKeyRPEEnabled     = "rpeEnabled"
	prefKeyAssumeRPE      = "assumeRpe"
	prefKeyPlateBarWeight = "plateBarWeight"
	prefKeyWeightUnit     = "weightUnit"
)

// defaultPlateBarWeight is the standard 45 lb / 20 kg bar. The unit field is
// display-only, so this is stated in pounds.
const defaultPlateBarWeight = 45.0

// SettingsService reads and writes user preferences and resolves the formula that
// the rest of the application should use.
type SettingsService struct {
	settings *tables.Settings
}

// NewSettingsService builds the settings service.
func NewSettingsService(s *tables.Settings) *SettingsService {
	return &SettingsService{settings: s}
}

// Get returns a user's settings, filling in defaults for anything absent or
// unrecognized.
func (s *SettingsService) Get(ctx context.Context, userID int64) (*Settings, error) {
	prefs, err := s.settings.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return settingsFromPreferences(prefs), nil
}

// Formula returns just the user's 1RM formula, for the many call sites that only
// need that one value.
func (s *SettingsService) Formula(ctx context.Context, userID int64) (formulas.Formula, error) {
	prefs, err := s.settings.Get(ctx, userID)
	if err != nil {
		return formulas.Default, err
	}
	return settingsFromPreferences(prefs).OneRMFormula, nil
}

// Update validates and persists a partial settings document. Only the keys
// present in the request are changed, so a client can PATCH one preference without
// resending the rest.
func (s *SettingsService) Update(ctx context.Context, userID int64, patch SettingsPatch) (*Settings, error) {
	prefs, err := s.settings.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	if patch.OneRMFormula != nil {
		f, err := formulas.Parse(*patch.OneRMFormula)
		if err != nil {
			return nil, ValidationError{Field: "oneRmFormula", Message: err.Error()}
		}
		prefs[prefKeyOneRMFormula] = string(f)
	}

	if patch.RPEEnabled != nil {
		prefs[prefKeyRPEEnabled] = *patch.RPEEnabled
	}

	if patch.AssumeRPE != nil {
		if *patch.AssumeRPE < 0 || *patch.AssumeRPE > 10 {
			return nil, ValidationError{Field: "assumeRpe", Message: "must be between 0 and 10"}
		}
		prefs[prefKeyAssumeRPE] = *patch.AssumeRPE
	}

	if patch.PlateBarWeight != nil {
		if *patch.PlateBarWeight < 0 || *patch.PlateBarWeight > 100 {
			return nil, ValidationError{Field: "plateBarWeight", Message: "must be between 0 and 100"}
		}
		prefs[prefKeyPlateBarWeight] = *patch.PlateBarWeight
	}

	if patch.WeightUnit != nil {
		unit := *patch.WeightUnit
		if unit != "lb" && unit != "kg" {
			return nil, ValidationError{Field: "weightUnit", Message: `must be "lb" or "kg"`}
		}
		prefs[prefKeyWeightUnit] = unit
	}

	if err := s.settings.Update(ctx, userID, prefs); err != nil {
		return nil, err
	}

	return settingsFromPreferences(prefs), nil
}

// SettingsPatch is the write shape. Pointers distinguish "leave unchanged" from
// "set to zero/false", which matters for boolean and numeric settings.
type SettingsPatch struct {
	OneRMFormula   *string  `json:"oneRmFormula"`
	RPEEnabled     *bool    `json:"rpeEnabled"`
	AssumeRPE      *float64 `json:"assumeRpe"`
	PlateBarWeight *float64 `json:"plateBarWeight"`
	WeightUnit     *string  `json:"weightUnit"`
}

// FormulaCatalog is returned alongside settings so the UI can render the options
// without hardcoding names or descriptions.
type FormulaCatalog struct {
	Formulas []FormulaOption `json:"formulas"`
	Default  string          `json:"default"`
}

// FormulaOption describes one selectable formula.
type FormulaOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UsesRPE     bool   `json:"usesRpe"`
}

// Catalog returns every supported formula, for the settings screen.
func (s *SettingsService) Catalog() FormulaCatalog {
	out := FormulaCatalog{
		Formulas: make([]FormulaOption, 0, len(formulas.Registry)),
		Default:  string(formulas.Default),
	}
	for _, info := range formulas.Registry {
		out.Formulas = append(out.Formulas, FormulaOption{
			ID:          string(info.ID),
			Name:        info.Name,
			Description: info.Description,
			UsesRPE:     info.UsesRPE,
		})
	}
	return out
}

// settingsFromPreferences maps the stored document onto the concrete struct,
// substituting defaults for anything missing or invalid. Doing it here rather
// than at each call site means a corrupt document degrades to defaults instead of
// erroring, which is the right failure mode for a preference.
func settingsFromPreferences(prefs tables.Preferences) *Settings {
	out := &Settings{
		OneRMFormula:   formulas.Default,
		RPEEnabled:     false,
		AssumeRPE:      formulas.DefaultAssumedRPE,
		PlateBarWeight: defaultPlateBarWeight,
		WeightUnit:     "lb",
	}

	if v, ok := prefs[prefKeyOneRMFormula].(string); ok {
		if f, err := formulas.Parse(v); err == nil {
			out.OneRMFormula = f
		}
	}

	if v, ok := prefs[prefKeyRPEEnabled].(bool); ok {
		out.RPEEnabled = v
	}

	if v, ok := prefs[prefKeyAssumeRPE].(float64); ok && v >= 0 && v <= 10 {
		out.AssumeRPE = v
	}

	if v, ok := prefs[prefKeyPlateBarWeight].(float64); ok && v >= 0 && v <= 100 {
		out.PlateBarWeight = v
	}

	if v, ok := prefs[prefKeyWeightUnit].(string); ok && (v == "lb" || v == "kg") {
		out.WeightUnit = v
	}

	return out
}
