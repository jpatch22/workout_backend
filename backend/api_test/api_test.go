// Package api_test exercises the full stack — router -> service -> tables ->
// Postgres — over real HTTP requests.
//
// These are the tests that prove the layering actually works end to end. They
// require a live database; set TEST_DATABASE_URL (default matches
// docker-compose.yml) to point at a throwaway Postgres.
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackpatchell/workout-tracker/backend/config"
	"github.com/jackpatchell/workout-tracker/backend/server"
)

var testServer http.Handler

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/workout_tracker?sslmode=disable"
	}

	cfg := config.Config{
		Port:        "0",
		Env:         "test",
		DatabaseURL: url,
		JWTSecret:   []byte("test-secret-value-that-is-long-enough-32"),
		JWTTTL:      time.Hour,
		AutoMigrate: true,
	}

	// Retry: on a cold CI database the container may still be starting.
	var handler http.Handler
	var cleanup func()
	var err error
	for attempt := 0; attempt < 15; attempt++ {
		handler, cleanup, err = server.NewRouter(context.Background(), cfg)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: cannot reach database %q: %v\n", url, err)
		os.Exit(0) // a missing database skips rather than fails; CI starts one
	}
	defer cleanup()

	testServer = handler
	os.Exit(m.Run())
}

var emailCounter atomic.Int64

// client is a thin JSON HTTP helper bound to one user's token.
type client struct {
	t     *testing.T
	email string
	token string
}

func newClient(t *testing.T) *client {
	t.Helper()

	email := fmt.Sprintf("test-%d-%d@example.com", time.Now().UnixNano(), emailCounter.Add(1))
	body := map[string]any{
		"email":       email,
		"password":    "Str0ngPass!",
		"displayName": "Test User",
	}

	var res authResult
	code := do(t, http.MethodPost, "/api/auth/register", "", body, &res)
	if code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", code)
	}
	if res.Token == "" {
		t.Fatal("register: expected a token")
	}
	return &client{t: t, email: email, token: res.Token}
}

type authResult struct {
	Token string `json:"token"`
	User  struct {
		ID          int64  `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
	} `json:"user"`
}

type exercise struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	MuscleGroup string `json:"muscleGroup"`
	IsCustom    bool   `json:"isCustom"`
}

type workout struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	PerformedAt time.Time         `json:"performedAt"`
	Exercises   []workoutExercise `json:"exercises"`
}

type workoutExercise struct {
	ID       int64    `json:"id"`
	Exercise exercise `json:"exercise"`
	Sets     []set    `json:"sets"`
}

type set struct {
	ID       int64    `json:"id"`
	Weight   float64  `json:"weight"`
	Reps     int      `json:"reps"`
	RPE      *float64 `json:"rpe"`
	IsWarmup bool     `json:"isWarmup"`
	IsPR     bool     `json:"isPr"`
}

type workoutSummary struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	SetCount        int     `json:"setCount"`
	TotalVolume     float64 `json:"totalVolume"`
	TopEstimated1RM float64 `json:"topEstimated1Rm"`
}

// do issues a request and decodes the JSON response into out (if non-nil).
func do(t *testing.T, method, path, token string, body any, out any) int {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	testServer.ServeHTTP(rec, req)

	if out != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code
}

// --- tests ---

func TestHealth(t *testing.T) {
	var body map[string]string
	if code := do(t, http.MethodGet, "/api/health", "", nil, &body); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestSeededExerciseCatalog(t *testing.T) {
	c := newClient(t)

	var list []exercise
	if code := do(t, http.MethodGet, "/api/exercises", c.token, nil, &list); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(list) < 20 {
		t.Fatalf("expected the seeded catalog (>=20), got %d", len(list))
	}

	var found bool
	for _, ex := range list {
		if ex.Slug == "back-squat" {
			found = true
			if ex.IsCustom {
				t.Error("seeded exercise should not be marked custom")
			}
		}
	}
	if !found {
		t.Fatal("expected back-squat in the catalog")
	}
}

func TestRegisterAndLogin(t *testing.T) {
	email := fmt.Sprintf("login-%d@example.com", time.Now().UnixNano())

	var reg authResult
	if code := do(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"email": email, "password": "Str0ngPass!", "displayName": "Round Trip",
	}, &reg); code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", code)
	}

	var login authResult
	if code := do(t, http.MethodPost, "/api/auth/login", "", map[string]any{
		"email": email, "password": "Str0ngPass!",
	}, &login); code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", code)
	}
	if login.Token == "" {
		t.Fatal("login: expected a token")
	}

	// A wrong password must be rejected.
	if code := do(t, http.MethodPost, "/api/auth/login", "", map[string]any{
		"email": email, "password": "wrong-password",
	}, nil); code != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", code)
	}
}

func TestRegisterRejectsWeakPassword(t *testing.T) {
	var errBody struct {
		Error string `json:"error"`
		Field string `json:"field"`
	}
	code := do(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"email": fmt.Sprintf("weak-%d@example.com", time.Now().UnixNano()), "password": "short",
	}, &errBody)

	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
	if errBody.Field != "password" {
		t.Fatalf("expected field=password, got %q", errBody.Field)
	}
}

func TestDuplicateEmailRejected(t *testing.T) {
	c := newClient(t)

	code := do(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"email": c.email, "password": "Str0ngPass!",
	}, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate email, got %d", code)
	}
}

// TestFullWorkoutFlow is the headline scenario: create a session, add an
// exercise, log sets, and verify the derived aggregates.
func TestFullWorkoutFlow(t *testing.T) {
	c := newClient(t)

	// 1. Create a workout.
	var wk workout
	code := do(t, http.MethodPost, "/api/workouts", c.token, map[string]any{
		"name": "Lower Body", "notes": "felt strong",
	}, &wk)
	if code != http.StatusCreated {
		t.Fatalf("create workout: expected 201, got %d", code)
	}
	if wk.ID == 0 {
		t.Fatal("create workout: expected an id")
	}

	// 2. Add two exercises.
	var catalog []exercise
	do(t, http.MethodGet, "/api/exercises", c.token, nil, &catalog)

	bySlug := map[string]exercise{}
	for _, ex := range catalog {
		bySlug[ex.Slug] = ex
	}

	var weSquat, weCurl workoutExercise
	if code := do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), c.token,
		map[string]any{"exerciseId": bySlug["back-squat"].ID}, &weSquat); code != http.StatusCreated {
		t.Fatalf("add squat: expected 201, got %d", code)
	}
	if code := do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), c.token,
		map[string]any{"exerciseId": bySlug["leg-curl"].ID}, &weCurl); code != http.StatusCreated {
		t.Fatalf("add leg curl: expected 201, got %d", code)
	}

	// 3. Log sets: 405x3, 385x5, then leg curls.
	rpe := 8.5
	type spec struct {
		weID int64
		wt   float64
		reps int
		rpe  *float64
		warm bool
	}
	specs := []spec{
		{weSquat.ID, 135, 5, nil, true}, // warmup, excluded from volume
		{weSquat.ID, 405, 3, &rpe, false},
		{weSquat.ID, 385, 5, nil, false},
		{weCurl.ID, 120, 10, nil, false},
		{weCurl.ID, 120, 10, nil, false},
	}

	var firstSet set
	for i, s := range specs {
		var got set
		code := do(t, http.MethodPost, fmt.Sprintf("/api/workout-exercises/%d/sets", s.weID), c.token,
			map[string]any{"weight": s.wt, "reps": s.reps, "rpe": s.rpe, "isWarmup": s.warm}, &got)
		if code != http.StatusCreated {
			t.Fatalf("log set %d: expected 201, got %d", i, code)
		}
		if i == 1 {
			firstSet = got
		}
	}

	// The first real working set of the very first session must be a PR.
	if !firstSet.IsPR {
		t.Error("expected the first heavy set to be flagged as a PR")
	}

	// 4. Read the workout back with everything inlined.
	var loaded workout
	if code := do(t, http.MethodGet, fmt.Sprintf("/api/workouts/%d", wk.ID), c.token, nil, &loaded); code != http.StatusOK {
		t.Fatalf("get workout: expected 200, got %d", code)
	}
	if len(loaded.Exercises) != 2 {
		t.Fatalf("expected 2 exercises, got %d", len(loaded.Exercises))
	}
	totalSets := 0
	for _, we := range loaded.Exercises {
		totalSets += len(we.Sets)
	}
	if totalSets != 5 {
		t.Fatalf("expected 5 sets, got %d", totalSets)
	}

	// 5. The summary must exclude the warmup from volume and set count.
	var list []workoutSummary
	do(t, http.MethodGet, "/api/workouts", c.token, nil, &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 workout, got %d", len(list))
	}
	// volume = 405*3 + 385*5 + 120*10 + 120*10 = 1215 + 1925 + 1200 + 1200 = 5540
	if list[0].TotalVolume != 5540 {
		t.Errorf("expected volume 5540, got %v", list[0].TotalVolume)
	}
	if list[0].SetCount != 4 {
		t.Errorf("expected 4 non-warmup sets, got %d", list[0].SetCount)
	}
	// Best est 1RM = 385 * (1 + 5/30) = 449.1666...
	// Note this beats 405 * (1 + 3/30) = 445.5: fewer reps at a higher
	// percentage of a 1RM wins, which is exactly why the metric is used
	// instead of raw weight.
	if math.Abs(list[0].TopEstimated1RM-449.1666666667) > 0.001 {
		t.Errorf("expected top est 1RM ~449.1667, got %v", list[0].TopEstimated1RM)
	}

	// 6. Dashboard aggregates must reflect the same data.
	var dash struct {
		Stats struct {
			TotalWorkouts int     `json:"totalWorkouts"`
			TotalVolume   float64 `json:"totalVolume"`
			TotalSets     int     `json:"totalSets"`
			PRsThisMonth  int     `json:"prsThisMonth"`
		} `json:"stats"`
		RecentWorkouts []workoutSummary `json:"recentWorkouts"`
	}
	if code := do(t, http.MethodGet, "/api/progress/dashboard", c.token, nil, &dash); code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d", code)
	}
	if dash.Stats.TotalWorkouts != 1 {
		t.Errorf("expected 1 workout in stats, got %d", dash.Stats.TotalWorkouts)
	}
	if dash.Stats.TotalVolume != 5540 {
		t.Errorf("expected stats volume 5540, got %v", dash.Stats.TotalVolume)
	}
	if dash.Stats.TotalSets != 4 {
		t.Errorf("expected 4 sets in stats, got %d", dash.Stats.TotalSets)
	}
	if dash.Stats.PRsThisMonth < 1 {
		t.Error("expected at least one PR this month")
	}
	if len(dash.RecentWorkouts) != 1 {
		t.Errorf("expected 1 recent workout, got %d", len(dash.RecentWorkouts))
	}

	// 7. Per-exercise progress for the squat.
	var prog struct {
		Slug   string `json:"slug"`
		Points []struct {
			Estimated1RM float64 `json:"estimated1Rm"`
			SetCount     int     `json:"setCount"`
		} `json:"points"`
		CurrentEst1RM float64 `json:"currentEstimated1Rm"`
		BestEst1RM    float64 `json:"bestEstimated1Rm"`
		TotalSessions int     `json:"totalSessions"`
	}
	if code := do(t, http.MethodGet, "/api/progress/exercise/back-squat", c.token, nil, &prog); code != http.StatusOK {
		t.Fatalf("exercise progress: expected 200, got %d", code)
	}
	if len(prog.Points) != 1 {
		t.Fatalf("expected 1 progress point, got %d", len(prog.Points))
	}
	// Warmup excluded: 2 sets in this session.
	if prog.Points[0].SetCount != 2 {
		t.Errorf("expected 2 sets in the squat point, got %d", prog.Points[0].SetCount)
	}
	if math.Abs(prog.BestEst1RM-449.1666666667) > 0.001 {
		t.Errorf("expected best est 1RM ~449.1667, got %v", prog.BestEst1RM)
	}
	if prog.TotalSessions != 1 {
		t.Errorf("expected 1 session, got %d", prog.TotalSessions)
	}

	// 8. Update a set, then delete it.
	var updated set
	if code := do(t, http.MethodPut, fmt.Sprintf("/api/sets/%d", firstSet.ID), c.token,
		map[string]any{"weight": 415, "reps": 3}, &updated); code != http.StatusOK {
		t.Fatalf("update set: expected 200, got %d", code)
	}
	if updated.Weight != 415 {
		t.Errorf("expected weight 415, got %v", updated.Weight)
	}

	if code := do(t, http.MethodDelete, fmt.Sprintf("/api/sets/%d", firstSet.ID), c.token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete set: expected 204, got %d", code)
	}

	// 9. Remove the leg curl, then the whole workout.
	if code := do(t, http.MethodDelete,
		fmt.Sprintf("/api/workouts/%d/exercises/%d", wk.ID, weCurl.ID), c.token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("remove exercise: expected 204, got %d", code)
	}
	if code := do(t, http.MethodDelete, fmt.Sprintf("/api/workouts/%d", wk.ID), c.token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete workout: expected 204, got %d", code)
	}
	if code := do(t, http.MethodGet, fmt.Sprintf("/api/workouts/%d", wk.ID), c.token, nil, nil); code != http.StatusNotFound {
		t.Fatalf("get deleted workout: expected 404, got %d", code)
	}
}

func TestSetValidation(t *testing.T) {
	c := newClient(t)

	var wk workout
	do(t, http.MethodPost, "/api/workouts", c.token, map[string]any{"name": "Validation"}, &wk)

	var catalog []exercise
	do(t, http.MethodGet, "/api/exercises", c.token, nil, &catalog)

	var we workoutExercise
	do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), c.token,
		map[string]any{"exerciseId": catalog[0].ID}, &we)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"zero reps", map[string]any{"weight": 100, "reps": 0}},
		{"negative weight", map[string]any{"weight": -5, "reps": 5}},
		{"rpe too high", map[string]any{"weight": 100, "reps": 5, "rpe": 12.0}},
		{"zero weight without flag", map[string]any{"weight": 0, "reps": 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := do(t, http.MethodPost, fmt.Sprintf("/api/workout-exercises/%d/sets", we.ID), c.token, tc.body, nil); code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", code)
			}
		})
	}

	// Bodyweight at weight 0 is legitimate.
	if code := do(t, http.MethodPost, fmt.Sprintf("/api/workout-exercises/%d/sets", we.ID), c.token,
		map[string]any{"weight": 0, "reps": 10, "isBodyweight": true}, nil); code != http.StatusCreated {
		t.Errorf("bodyweight set: expected 201, got %d", code)
	}
}

func TestCustomExercise(t *testing.T) {
	c := newClient(t)

	var ex exercise
	code := do(t, http.MethodPost, "/api/exercises", c.token, map[string]any{
		"name": "Zercher Squat", "muscleGroup": "legs", "equipment": "barbell",
	}, &ex)
	if code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", code)
	}
	if ex.Slug != "zercher-squat" {
		t.Errorf("expected slug zercher-squat, got %q", ex.Slug)
	}
	if !ex.IsCustom {
		t.Error("expected isCustom true")
	}

	// A second exercise with the same name must get a distinct slug.
	var dup exercise
	do(t, http.MethodPost, "/api/exercises", c.token, map[string]any{"name": "Zercher Squat"}, &dup)
	if dup.Slug != "zercher-squat-2" {
		t.Errorf("expected zercher-squat-2, got %q", dup.Slug)
	}

	if code := do(t, http.MethodDelete, fmt.Sprintf("/api/exercises/%d", ex.ID), c.token, nil, nil); code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", code)
	}
}

func TestDuplicateExerciseInWorkoutIsIdempotent(t *testing.T) {
	c := newClient(t)

	var wk workout
	do(t, http.MethodPost, "/api/workouts", c.token, map[string]any{"name": "Idempotency"}, &wk)

	var catalog []exercise
	do(t, http.MethodGet, "/api/exercises", c.token, nil, &catalog)
	target := catalog[0].ID

	var first workoutExercise
	do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), c.token,
		map[string]any{"exerciseId": target}, &first)

	var second workoutExercise
	do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), c.token,
		map[string]any{"exerciseId": target}, &second)

	if first.ID != second.ID {
		t.Errorf("re-adding should return the same row: %d vs %d", first.ID, second.ID)
	}

	var loaded workout
	do(t, http.MethodGet, fmt.Sprintf("/api/workouts/%d", wk.ID), c.token, nil, &loaded)
	if len(loaded.Exercises) != 1 {
		t.Errorf("expected 1 exercise, got %d", len(loaded.Exercises))
	}
}

// TestUsersCannotTouchEachOthersData is the security-critical case: every
// lookup must be scoped by owner, so a guessed id leaks nothing.
func TestUsersCannotTouchEachOthersData(t *testing.T) {
	alice := newClient(t)
	bob := newClient(t)

	var wk workout
	do(t, http.MethodPost, "/api/workouts", alice.token, map[string]any{"name": "Alice's session"}, &wk)

	var catalog []exercise
	do(t, http.MethodGet, "/api/exercises", alice.token, nil, &catalog)

	var we workoutExercise
	do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), alice.token,
		map[string]any{"exerciseId": catalog[0].ID}, &we)

	var aliceSet set
	do(t, http.MethodPost, fmt.Sprintf("/api/workout-exercises/%d/sets", we.ID), alice.token,
		map[string]any{"weight": 300, "reps": 5}, &aliceSet)

	// Bob must get 404 (not 403) everywhere, and must not be able to mutate.
	if code := do(t, http.MethodGet, fmt.Sprintf("/api/workouts/%d", wk.ID), bob.token, nil, nil); code != http.StatusNotFound {
		t.Errorf("bob get alice's workout: expected 404, got %d", code)
	}
	if code := do(t, http.MethodPut, fmt.Sprintf("/api/workouts/%d", wk.ID), bob.token,
		map[string]any{"name": "hijacked"}, nil); code != http.StatusNotFound {
		t.Errorf("bob update alice's workout: expected 404, got %d", code)
	}
	if code := do(t, http.MethodDelete, fmt.Sprintf("/api/workouts/%d", wk.ID), bob.token, nil, nil); code != http.StatusNotFound {
		t.Errorf("bob delete alice's workout: expected 404, got %d", code)
	}
	if code := do(t, http.MethodPut, fmt.Sprintf("/api/sets/%d", aliceSet.ID), bob.token,
		map[string]any{"weight": 1, "reps": 1}, nil); code != http.StatusNotFound {
		t.Errorf("bob update alice's set: expected 404, got %d", code)
	}
	if code := do(t, http.MethodDelete, fmt.Sprintf("/api/sets/%d", aliceSet.ID), bob.token, nil, nil); code != http.StatusNotFound {
		t.Errorf("bob delete alice's set: expected 404, got %d", code)
	}

	// Bob's own listing must not contain Alice's workout.
	var bobList []workoutSummary
	do(t, http.MethodGet, "/api/workouts", bob.token, nil, &bobList)
	for _, s := range bobList {
		if s.ID == wk.ID {
			t.Error("alice's workout leaked into bob's history")
		}
	}

	// Bob's progress view must not include Alice's PRs.
	var bobDash struct {
		Stats struct {
			TotalWorkouts int `json:"totalWorkouts"`
		} `json:"stats"`
	}
	do(t, http.MethodGet, "/api/progress/dashboard", bob.token, nil, &bobDash)
	if bobDash.Stats.TotalWorkouts != 0 {
		t.Errorf("bob should have 0 workouts, got %d", bobDash.Stats.TotalWorkouts)
	}
}

func TestAuthenticationRequired(t *testing.T) {
	protected := []struct{ method, path string }{
		{http.MethodGet, "/api/exercises"},
		{http.MethodGet, "/api/workouts"},
		{http.MethodPost, "/api/workouts"},
		{http.MethodGet, "/api/progress/dashboard"},
		{http.MethodGet, "/api/progress/exercise/back-squat"},
	}
	for _, ep := range protected {
		if code := do(t, ep.method, ep.path, "", nil, nil); code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", ep.method, ep.path, code)
		}
	}

	// A garbage token is also rejected.
	if code := do(t, http.MethodGet, "/api/exercises", "not-a-real-token", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("expected 401 for a bad token, got %d", code)
	}
}

// TestPersonalRecordsImproveOverTime checks the PR logic across sessions:
// a heavier set later must be flagged, and a lighter one must not.
func TestPersonalRecordsImproveOverTime(t *testing.T) {
	c := newClient(t)

	var catalog []exercise
	do(t, http.MethodGet, "/api/exercises", c.token, nil, &catalog)

	slug := ""
	for _, ex := range catalog {
		if ex.Slug == "bench-press" {
			slug = ex.Slug
		}
	}
	if slug == "" {
		t.Skip("bench-press not seeded")
	}
	var benchID int64
	for _, ex := range catalog {
		if ex.Slug == "bench-press" {
			benchID = ex.ID
		}
	}

	// Session 1: 225x5 then 205x5.
	w1 := logSet(t, c.token, benchID, 225, 5, false)
	if !w1.IsPR {
		t.Error("225x5 should be a PR (nothing before it)")
	}
	lower := logSet(t, c.token, benchID, 205, 5, false)
	if lower.IsPR {
		t.Error("205x5 after 225x5 should not be a PR")
	}

	// Session 2: a genuine improvement must be flagged.
	better := logSet(t, c.token, benchID, 235, 5, false)
	if !better.IsPR {
		t.Error("235x5 should be a new PR")
	}

	// And a warmup should never be a PR, however light.
	warm := logSet(t, c.token, benchID, 135, 10, true)
	if warm.IsPR {
		t.Error("warmup should never be a PR")
	}
}

// logSet creates a workout, adds the exercise, logs one set, and returns it.
func logSet(t *testing.T, token string, exerciseID int64, weight float64, reps int, warmup bool) set {
	t.Helper()

	var wk workout
	if code := do(t, http.MethodPost, "/api/workouts", token, map[string]any{"name": "PR session"}, &wk); code != http.StatusCreated {
		t.Fatalf("create workout: %d", code)
	}
	var we workoutExercise
	do(t, http.MethodPost, fmt.Sprintf("/api/workouts/%d/exercises", wk.ID), token,
		map[string]any{"exerciseId": exerciseID}, &we)

	var s set
	if code := do(t, http.MethodPost, fmt.Sprintf("/api/workout-exercises/%d/sets", we.ID), token,
		map[string]any{"weight": weight, "reps": reps, "isWarmup": warmup}, &s); code != http.StatusCreated {
		t.Fatalf("log set: %d", code)
	}
	return s
}

func TestWorkoutRejectsFutureDate(t *testing.T) {
	c := newClient(t)
	future := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)

	if code := do(t, http.MethodPost, "/api/workouts", c.token, map[string]any{
		"name": "Tomorrow", "performedAt": future,
	}, nil); code != http.StatusBadRequest {
		t.Errorf("expected 400 for a future date, got %d", code)
	}
}

func TestUnknownEndpointReturns404(t *testing.T) {
	if code := do(t, http.MethodGet, "/api/nope", "", nil, nil); code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	if code := do(t, http.MethodDelete, "/api/health", "", nil, nil); code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", code)
	}
}
