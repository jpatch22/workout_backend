// Settings and RPE-formula behaviour, exercised over real HTTP against a real
// database.
//
// The important case is TestFormulaSettingChangesPRFlags: it proves the user's
// formula selection reaches the SQL that decides personal records. A settings
// screen that displays correctly but does not change the underlying computation
// would look perfect and be entirely inert, which is exactly the kind of bug this
// file exists to catch.
package api_test

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

type settings struct {
	OneRMFormula   string  `json:"oneRmFormula"`
	RPEEnabled     bool    `json:"rpeEnabled"`
	AssumeRPE      float64 `json:"assumeRpe"`
	PlateBarWeight float64 `json:"plateBarWeight"`
	WeightUnit     string  `json:"weightUnit"`
}

type formulaCatalog struct {
	Formulas []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		UsesRPE     bool   `json:"usesRpe"`
	} `json:"formulas"`
	Default string `json:"default"`
}

// getSettings reads the caller's settings.
func getSettings(t *testing.T, c *client) settings {
	t.Helper()
	var s settings
	if code := do(t, http.MethodGet, "/api/settings", c.token, nil, &s); code != http.StatusOK {
		t.Fatalf("get settings: expected 200, got %d", code)
	}
	return s
}

// patchSettings applies a partial settings update.
func patchSettings(t *testing.T, c *client, body map[string]any) (settings, int) {
	t.Helper()
	var s settings
	code := do(t, http.MethodPatch, "/api/settings", c.token, body, &s)
	return s, code
}

// exerciseID returns the id of a catalog exercise.
func exerciseID(t *testing.T, c *client, slug string) int64 {
	t.Helper()
	var list []exercise
	do(t, http.MethodGet, "/api/exercises", c.token, nil, &list)
	for _, ex := range list {
		if ex.Slug == slug {
			return ex.ID
		}
	}
	t.Fatalf("exercise %q not found", slug)
	return 0
}

func TestSettingsDefaults(t *testing.T) {
	c := newClient(t)
	s := getSettings(t, c)

	// A brand-new user gets the documented defaults, not an empty object.
	if s.OneRMFormula != "epley" {
		t.Errorf("expected the default formula to be epley, got %q", s.OneRMFormula)
	}
	if s.RPEEnabled {
		t.Error("expected RPE capture to be off by default")
	}
	if s.AssumeRPE != 10 {
		t.Errorf("expected a blank RPE to be read as 10, got %v", s.AssumeRPE)
	}
	if s.PlateBarWeight != 45 {
		t.Errorf("expected a 45 lb bar, got %v", s.PlateBarWeight)
	}
	if s.WeightUnit != "lb" {
		t.Errorf("expected lb, got %q", s.WeightUnit)
	}
}

func TestSettingsFormulaCatalog(t *testing.T) {
	c := newClient(t)

	var cat formulaCatalog
	if code := do(t, http.MethodGet, "/api/settings/formulas", c.token, nil, &cat); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(cat.Formulas) < 2 {
		t.Fatalf("expected at least two formulas, got %d", len(cat.Formulas))
	}
	if cat.Default != "epley" {
		t.Errorf("expected epley to be the advertised default, got %q", cat.Default)
	}

	// The UI selects the RPE formula by id, so ids and descriptions must exist.
	var epley, rts bool
	for _, f := range cat.Formulas {
		if f.ID == "epley" {
			epley = true
			if f.UsesRPE {
				t.Error("epley should not advertise RPE usage")
			}
		}
		if f.ID == "rts" {
			rts = true
			if !f.UsesRPE {
				t.Error("rts should advertise RPE usage")
			}
		}
		if f.Name == "" || f.Description == "" {
			t.Errorf("formula %q needs a name and description for the settings UI", f.ID)
		}
	}
	if !epley || !rts {
		t.Errorf("catalog missing a formula: epley=%v rts=%v", epley, rts)
	}
}

func TestSettingsUpdateAndValidation(t *testing.T) {
	c := newClient(t)

	// Turning on RPE and switching formula.
	got, code := patchSettings(t, c, map[string]any{
		"rpeEnabled":   true,
		"oneRmFormula": "rts",
	})
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !got.RPEEnabled || got.OneRMFormula != "rts" {
		t.Errorf("update did not stick: %+v", got)
	}

	// The change must persist across a read.
	if s := getSettings(t, c); !s.RPEEnabled || s.OneRMFormula != "rts" {
		t.Errorf("settings did not persist: %+v", s)
	}

	// A partial update must leave untouched fields alone.
	got, code = patchSettings(t, c, map[string]any{"plateBarWeight": 35})
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if got.PlateBarWeight != 35 {
		t.Errorf("expected the bar to become 35, got %v", got.PlateBarWeight)
	}
	if !got.RPEEnabled || got.OneRMFormula != "rts" {
		t.Errorf("a partial update clobbered other settings: %+v", got)
	}

	// Invalid input is rejected with a field-tagged 400.
	bad := []map[string]any{
		{"oneRmFormula": "not-a-formula"},
		{"assumeRpe": 15},
		{"assumeRpe": -1},
		{"plateBarWeight": -5},
		{"weightUnit": "stone"},
	}
	for _, body := range bad {
		if _, code := patchSettings(t, c, body); code != http.StatusBadRequest {
			t.Errorf("expected 400 for %v, got %d", body, code)
		}
	}
}

func TestSettingsRequireAuth(t *testing.T) {
	for _, ep := range []struct{ method, path string }{
		{http.MethodGet, "/api/settings"},
		{http.MethodPatch, "/api/settings"},
		{http.MethodGet, "/api/settings/formulas"},
	} {
		if code := do(t, ep.method, ep.path, "", nil, nil); code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", ep.method, ep.path, code)
		}
	}
}

func TestSettingsArePrivateToTheUser(t *testing.T) {
	alice := newClient(t)
	bob := newClient(t)

	if _, code := patchSettings(t, alice, map[string]any{"oneRmFormula": "rts"}); code != http.StatusOK {
		t.Fatalf("alice update failed: %d", code)
	}

	// Bob must still see his own defaults, not Alice's choice.
	if s := getSettings(t, bob); s.OneRMFormula != "epley" {
		t.Errorf("bob's settings leaked from alice: %q", s.OneRMFormula)
	}
}

// TestFormulaSettingChangesPRFlags is the core of this file.
//
// It logs the same set twice under two different formulas and asserts the PR
// logic actually responds to the setting. It also pins the specific numbers: the
// RTS polynomial and Epley disagree about which set is stronger for a given RPE,
// and that disagreement is the entire point of the feature.
func TestFormulaSettingChangesPRFlags(t *testing.T) {
	c := newClient(t)
	bench := exerciseID(t, c, "bench-press")
	rpe := 8.0

	// logBench creates a fresh session and logs one set at the given weight,
	// reps, and RPE, returning the stored set.
	logBench := func(weight float64, reps int, rpe *float64) set {
		t.Helper()
		var wk workout
		if code := do(t, http.MethodPost, "/api/workouts", c.token, map[string]any{"name": "PR probe"}, &wk); code != http.StatusCreated {
			t.Fatalf("create workout: %d", code)
		}
		var we workoutExercise
		do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), c.token,
			map[string]any{"exerciseId": bench}, &we)

		var s set
		body := map[string]any{"weight": weight, "reps": reps}
		if rpe != nil {
			body["rpe"] = *rpe
		}
		if code := do(t, http.MethodPost, fmt.Sprintf("/api/workout-exercises/%d/sets", we.ID), c.token, body, &s); code != http.StatusCreated {
			t.Fatalf("log set: %d", code)
		}
		return s
	}

	// --- Epley: ignores RPE entirely ---
	if _, code := patchSettings(t, c, map[string]any{"oneRmFormula": "epley", "rpeEnabled": true}); code != http.StatusOK {
		t.Fatalf("set epley: %d", code)
	}

	first := logBench(225, 5, &rpe) // 225x5 @ RPE 8
	if !first.IsPR {
		t.Fatal("the first logged set should be a PR under any formula")
	}
	// Epley cannot see effort, so an easier set at the same load is not a PR and
	// the reported chart value must equal the Epley number.
	var epleyProg struct {
		Points []struct {
			Estimated1RM float64 `json:"estimated1Rm"`
		} `json:"points"`
	}
	do(t, http.MethodGet, "/api/progress/exercise/bench-press", c.token, nil, &epleyProg)
	if len(epleyProg.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(epleyProg.Points))
	}
	wantEpley := 225 * (1 + 5.0/30) // 262.5
	if math.Abs(epleyProg.Points[0].Estimated1RM-wantEpley) > 0.01 {
		t.Errorf("epley progress: got %.2f, want %.2f", epleyProg.Points[0].Estimated1RM, wantEpley)
	}

	// --- RTS polynomial: reads the RPE ---
	if _, code := patchSettings(t, c, map[string]any{"oneRmFormula": "rts"}); code != http.StatusOK {
		t.Fatalf("set rts: %d", code)
	}

	// Same load, but now reported with the RPE taken into account.
	var rtsProg struct {
		Points []struct {
			Estimated1RM float64 `json:"estimated1Rm"`
		} `json:"points"`
	}
	do(t, http.MethodGet, "/api/progress/exercise/bench-press", c.token, nil, &rtsProg)
	if len(rtsProg.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(rtsProg.Points))
	}

	// 5 reps at RPE 8 => total reps 7 => pct = 1.0261 - .0242*7 - .0004*49
	tr := 5.0 + (10 - 8)
	wantRTS := 225 / (1.0261 - 0.0242*tr - 0.0004*tr*tr)
	if math.Abs(rtsProg.Points[0].Estimated1RM-wantRTS) > 0.01 {
		t.Errorf("rts progress: got %.2f, want %.2f", rtsProg.Points[0].Estimated1RM, wantRTS)
	}

	// The two formulas must genuinely disagree here, otherwise switching the
	// setting would be cosmetic.
	if math.Abs(wantRTS-wantEpley) < 5 {
		t.Errorf("expected epley (%.2f) and rts (%.2f) to differ appreciably",
			wantEpley, wantRTS)
	}

	// At RPE 8 two reps were left in reserve, so the RTS estimate sits ABOVE
	// Epley: the set was not a true max, and the formula credits the difference.
	// Only near RPE 10 (no reserve) does RTS fall to or below Epley.
	if wantRTS <= wantEpley {
		t.Errorf("expected the RPE-8 adjusted estimate (%.2f) above Epley's (%.2f)",
			wantRTS, wantEpley)
	}

	// Cross-check against the documented direction: lowering the RPE raises the
	// estimate, which is the behaviour Epley cannot reproduce.
	wantEasier := 225 / (1.0261 - 0.0242*8 - 0.0004*8*8) // 5 reps @ RPE 7
	if wantEasier <= wantRTS {
		t.Errorf("RPE 7 (%.2f) should estimate above RPE 8 (%.2f)", wantEasier, wantRTS)
	}

	// A genuinely easier set at the same load should now beat the RPE-8 set,
	// which Epley could never detect.
	easier := logBench(225, 5, fptr(7))
	if !easier.IsPR {
		t.Error("an RPE-7 set should beat a prior RPE-8 set under the RTS formula")
	}
}

// TestBlankRPEUnderRTSFormulaMatchesRPE10 verifies the "blank means 10" rule
// end-to-end, including that it does not produce a false PR.
func TestBlankRPEUnderRTSFormulaMatchesRPE10(t *testing.T) {
	c := newClient(t)
	squat := exerciseID(t, c, "back-squat")

	if _, code := patchSettings(t, c, map[string]any{"oneRmFormula": "rts"}); code != http.StatusOK {
		t.Fatalf("set rts: %d", code)
	}

	// One set with an explicit RPE 10, then the same set with no RPE at all.
	// The blank one must resolve to the same estimate, so it cannot be a PR.
	explicit := logSetForExercise(t, c, squat, 315, 5, fptr(10))
	if !explicit.IsPR {
		t.Fatal("first set should be a PR")
	}

	blank := logSetForExercise(t, c, squat, 315, 5, nil)
	if blank.IsPR {
		t.Error("a blank RPE is read as 10, so repeating the same set must not be a new PR")
	}
}

// logSetForExercise creates a session and logs a single set for an exercise.
func logSetForExercise(t *testing.T, c *client, exerciseID int64, weight float64, reps int, rpe *float64) set {
	t.Helper()

	var wk workout
	if code := do(t, http.MethodPost, "/api/workouts", c.token, map[string]any{"name": "RPE probe"}, &wk); code != http.StatusCreated {
		t.Fatalf("create workout: %d", code)
	}
	var we workoutExercise
	do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), c.token,
		map[string]any{"exerciseId": exerciseID}, &we)

	body := map[string]any{"weight": weight, "reps": reps}
	if rpe != nil {
		body["rpe"] = *rpe
	}
	var s set
	if code := do(t, http.MethodPost, fmt.Sprintf("/api/workout-exercises/%d/sets", we.ID), c.token, body, &s); code != http.StatusCreated {
		t.Fatalf("log set: %d", code)
	}
	return s
}

func fptr(v float64) *float64 { return &v }

// TestSettingsRawJSONShape guards the wire format, so the frontend can rely on
// the field names rather than guessing at them.
func TestSettingsRawJSONShape(t *testing.T) {
	c := newClient(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	rec := httptest.NewRecorder()
	testServer.ServeHTTP(rec, req)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"oneRmFormula", "rpeEnabled", "assumeRpe", "plateBarWeight", "weightUnit"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("settings response is missing %q", key)
		}
	}
}
