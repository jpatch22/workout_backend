# Workout Tracker

A weight-training log: track sessions, sets, and progression. Go REST API + PostgreSQL, React single-page frontend.

```
React + TypeScript (Vite, Tailwind, TanStack Query, Recharts)
        │  REST + JWT
        ▼
Go API (routers → services → tables)
        │
        ▼
PostgreSQL
```

---

## Architecture

The rule that shapes the whole codebase:

```
HTTP request → routers → services → tables → PostgreSQL
```

- **routers/** — HTTP only. Decode input, call one service method, encode the result. No business logic, no SQL.
- **services/** — all business logic and validation. Owns the error vocabulary (`ValidationError`, `ErrNotFound`, `ErrUnauthorized`).
- **tables/** — models and **every SQL statement in the project**. Nothing above this layer writes a query.

The payoff is that a new feature touches one service file and one router function. Because nothing is denormalized, the derived metrics (volume, estimated 1RM, PRs, streaks) are computed at query time, so changing a formula applies to all history instantly with no backfill.

```
backend/
├── cmd/server/main.go     bootstrap: config → db → services → routers → HTTP
├── server/server.go       the only file that knows all layers exist
├── config/                env-based config, fails fast on bad prod config
├── routers/               auth, workouts, exercises, progress
├── services/              business logic + validation
├── tables/                models + all SQL + embedded migrations
├── middleware/            auth, logging, panic recovery
└── api_test/              end-to-end tests over real HTTP

frontend/src/
├── api/                   typed API bindings (components never call fetch)
├── types/                 TS mirrors of the API's JSON shapes
├── hooks/                 TanStack Query hooks (only place server state lives)
├── pages/                 whole screens
├── components/            reusable UI + domain widgets
└── utils/                 formatting, incl. the shared estimated-1RM formula
```

---

## Running locally

Requires Go 1.24+, Node 20+, and Docker.

**1. Start Postgres**

```bash
docker compose up -d
```

**2. Run the API** (migrations apply automatically on boot)

```bash
cd backend
cp .env.example .env       # defaults already point at the compose database
go run ./cmd/server
```

The API listens on `:8080`; check it with `curl localhost:8080/api/health`.

**3. Run the frontend**

```bash
cd frontend
cp .env.local.example .env.local
npm install
npm run dev
```

Open http://localhost:5173 and create an account. A catalog of ~22 exercises is seeded, so you can log a real workout immediately.

---

## Tests

```bash
# Backend: full stack against a live Postgres
docker compose up -d
cd backend && go test ./... -v

# Frontend: DOM tests, typecheck, and production build
cd frontend && npm test && npm run build
```

The backend suite drives real HTTP requests through the real router, service, and SQL layers, then asserts on the derived numbers (volume, estimated 1RM, PR flags) — including that one user can never read or modify another user's data. If no database is reachable it skips rather than failing; in CI the database service is started first.

The frontend suite mounts real components in jsdom and asserts on rendered output and typing behavior, most importantly that the TypeScript `estimated1RM` returns the same values as the Go implementation and the PR-detection SQL. That agreement is the kind of thing a typecheck cannot catch and a bug in it would be invisible until someone compared the chart to the log.

There is also an end-to-end script that drives a running server with curl:

```bash
./scripts/e2e.sh          # API=http://localhost:8080 by default
```

If Docker's registry is unreachable, `backend/tools/pgtest` boots a real PostgreSQL from a downloaded binary instead, so `go test` still runs against actual SQL rather than skipping:

```bash
go run -tags tools ./backend/tools/pgtest   # leave running in one terminal
cd backend && go test ./...
```

---

## API

All routes except `/api/auth/register` and `/api/auth/login` require `Authorization: Bearer <token>`.

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/auth/register` | Create an account, returns a token |
| POST | `/api/auth/login` | Sign in |
| GET | `/api/auth/me` | Current profile |
| PATCH | `/api/auth/me` | Update display name |
| GET | `/api/exercises` | Catalog (global + your custom) |
| POST | `/api/exercises` | Create a custom exercise |
| DELETE | `/api/exercises/:id` | Delete a custom exercise |
| GET | `/api/workouts` | Session summaries, newest first |
| POST | `/api/workouts` | Start a session |
| GET | `/api/workouts/:id` | One session with exercises and sets |
| PUT | `/api/workouts/:id` | Rename / redate / annotate |
| DELETE | `/api/workouts/:id` | Delete a session |
| POST | `/api/workouts/:id/exercises` | Add an exercise to a session |
| DELETE | `/api/workouts/:id/exercises/:weId` | Remove it |
| POST | `/api/workout-exercises/:weId/sets` | Log a set |
| PUT | `/api/sets/:id` | Edit a set |
| DELETE | `/api/sets/:id` | Delete a set |
| GET | `/api/progress/dashboard` | Stats + recent sessions + exercises |
| GET | `/api/progress` | Headline aggregates |
| GET | `/api/progress/exercises` | Exercises with training history |
| GET | `/api/progress/exercise/:slug` | Full time series for one exercise |

Errors always come back in one shape:

```json
{ "error": "validation_error", "message": "must be at least 1", "field": "reps" }
```

`field` is present for validation errors so the UI can point at the offending input.

### Data model

```
users ──< workouts ──< workout_exercises ──< sets
                    └──< workout_exercises >── exercises
```

`sets` is the atomic unit of all training data. `exercises` holds the shared catalog (`user_id IS NULL`) plus any user-created movements. Warmup sets are excluded from volume, estimated 1RM, streaks, and PR comparisons.

**Estimated 1RM** uses Epley: `weight × (1 + reps / 30)`. It is defined once in `services.Estimated1RM`, mirrored in the PR-detection SQL, and again in `frontend/src/utils/formatting.ts` for the optimistic per-set preview — all three must stay in agreement.

A **PR** is flagged when a set's estimated 1RM beats every prior non-warmup set for that exercise across all of that user's history.

---

## Migrations

SQL lives in `backend/tables/migrations/` and is embedded into the binary, so a deploy needs no separate migration step. `migrations/` at the repo root is a symlink to that directory for convenience.

Add a migration as a new numbered pair:

```bash
# 003_add_something.up.sql
# 003_add_something.down.sql
```

They are applied in order on startup and are idempotent, so restarting the server is safe.

---

## Deployment

**API (Render)** — build `go build -o server ./backend/cmd/server`, start `./server`. Set `APP_ENV=production`, a strong `JWT_SECRET`, `DATABASE_URL`, `ALLOWED_ORIGINS`, and `PORT`. On Supabase, use the pooled connection string and add `?sslmode=require`.

**Frontend (GitHub Pages)** — `npm run build`, publish `frontend/dist`. Set `VITE_API_URL` to the deployed API. `base: './'` in the Vite config makes the bundle work from a project subpath. Because the app is client-routed, add a Pages "404 fallback" pointing at `index.html` so deep links resolve.

**CORS** — `ALLOWED_ORIGINS` must list the deployed frontend origin. Only listed origins get an `Access-Control-Allow-Origin` header.

---

## Adding a feature

The boundaries are the point. To add, say, a "bench press 1RM" endpoint:

1. `tables/progress.go` — the query that derives it.
2. `services/progress.go` — the method that wraps it.
3. `routers/progress.go` — the route that calls it.

No other layer changes. An AI-assisted session works best when each of those instructions names the file and the layer.