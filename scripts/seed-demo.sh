#!/usr/bin/env bash
# Seeds a demo account with several weeks of training history so the Dashboard,
# History, and Progress screens have real data to show.
#
# Idempotent per run: it creates a fresh account each time (unique email), so
# re-running gives you a clean slate rather than duplicating data.
set -euo pipefail

API="${API:-http://localhost:8080}"
EMAIL="${DEMO_EMAIL:-demo@workouttracker.local}"
PASS="${DEMO_PASSWORD:-Str0ngPass!}"

# Reads JSON on stdin and prints the value at a dotted/indexed path.
# Usage: ... | jp "['token']"     ... | jp "[0]['totalVolume']"
jp() {
  python3 -c "
import sys, json
d = json.load(sys.stdin)
print(eval('d' + sys.argv[1]))
" "$1"
}
step() { printf '\033[1m%s\033[0m\n' "$1"; }

step "Signing up $EMAIL"

# Register, but fall back to logging in if this account already exists — the
# script should be re-runnable without manual cleanup between attempts.
REG=$(curl -s -X POST $API/api/auth/register -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"displayName\":\"Alex\"}")
TOKEN=$(printf '%s' "$REG" | jp "['token']" 2>/dev/null || true)

if [ -z "$TOKEN" ] || [ "$TOKEN" = "None" ]; then
  step "Account exists, signing in instead"
  TOKEN=$(curl -s -X POST $API/api/auth/login -H 'Content-Type: application/json' \
    -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jp "['token']")
fi

AUTH="Authorization: Bearer $TOKEN"
[ -n "$TOKEN" ] && [ "$TOKEN" != "None" ] || { printf 'auth failed: %s\n' "$REG"; exit 1; }

# Pull ids for the movements we program. Count first so an unseeded catalog
# fails loudly here rather than producing a confusing 404 later.
EX=$(curl -s $API/api/exercises -H "$AUTH")
COUNT=$(python3 -c 'import sys,json;print(len(json.load(sys.stdin)))' <<<"$EX")
if [ "$COUNT" -lt 20 ]; then
  echo "catalog has only $COUNT exercises - migrations probably did not seed"
  exit 1
fi
printf 'catalog: %s exercises\n' "$COUNT"
id_for() { python3 -c "import sys,json;print(next(e['id'] for e in json.load(sys.stdin) if e['slug']=='$1'))" <<<"$EX"; }

SQUAT=$(id_for back-squat)
BENCH=$(id_for bench-press)
DEAD=$(id_for deadlift)
ROW=$(id_for barbell-row)
OHP=$(id_for overhead-press)
CURL=$(id_for leg-curl)

# session <days-ago> <name> <exerciseVar:weight:reps,...>
session() {
  local days=$1 name=$2 spec=$3
  local when
  when=$(python3 -c "import datetime;print((datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=$days)).isoformat().replace('+00:00','Z'))")

  local wk
  wk=$(curl -s -X POST $API/api/workouts -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"name\":\"$name\",\"performedAt\":\"$when\"}" | jp "['id']")

  for item in $spec; do
    local ex_var="${item%%:*}" rest="${item#*:}"
    local weight="${rest%%:*}" reps="${rest##*:}"
    local ex_id="${!ex_var}"

    local we
    we=$(curl -s -X POST $API/api/workouts/$wk/exercises -H "$AUTH" -H 'Content-Type: application/json' \
      -d "{\"exerciseId\":$ex_id}" | jp "['id']")

    # one warmup plus the working set
    curl -s -o /dev/null -X POST $API/api/workout-exercises/$we/sets -H "$AUTH" \
      -H 'Content-Type: application/json' \
      -d "{\"weight\":$(python3 -c "print(round($weight*0.5))"),\"reps\":5,\"isWarmup\":true}"
    curl -s -o /dev/null -X POST $API/api/workout-exercises/$we/sets -H "$AUTH" \
      -H 'Content-Type: application/json' -d "{\"weight\":$weight,\"reps\":$reps,\"rpe\":8}"
  done
  printf '  %-28s %s\n' "$name" "$(python3 -c "import datetime;print((datetime.date.today()-datetime.timedelta(days=$days)).isoformat())")"
}

step "Seeding 4 weeks of sessions"
session 27 "Lower Body"  "SQUAT:315:5  CURL:100:12"
session 25 "Upper Body"  "BENCH:185:5  ROW:135:8"
session 24 "Lower Body"  "SQUAT:325:5  CURL:105:12"
session 22 "Upper Body"  "BENCH:195:5  ROW:140:8"
session 21 "Lower Body"  "SQUAT:335:5  CURL:110:10"
session 19 "Pull Day"    "DEAD:275:5  ROW:145:8"
session 18 "Upper Body"  "BENCH:205:5  OHP:115:6"
session 16 "Lower Body"  "SQUAT:345:5  CURL:110:12"
session 15 "Pull Day"    "DEAD:285:5  ROW:150:8"
session 14 "Upper Body"  "BENCH:215:5  OHP:120:6"
session 12 "Lower Body"  "SQUAT:355:3  CURL:115:10"
session 11 "Pull Day"    "DEAD:295:5  ROW:155:8"
session 9  "Upper Body"  "BENCH:225:5  OHP:125:5"
session 8  "Lower Body"  "SQUAT:365:3  CURL:120:12"
session 6  "Pull Day"    "DEAD:315:3  ROW:160:8"
session 5  "Upper Body"  "BENCH:235:3  OHP:130:5"
session 3  "Lower Body"  "SQUAT:375:3  CURL:125:10"
session 1  "Upper Body"  "BENCH:245:3  OHP:135:5"

step "Dashboard"
curl -s $API/api/progress/dashboard -H "$AUTH" | python3 -c "
import sys,json
d=json.load(sys.stdin); s=d['stats']
print(f\"  workouts       {s['totalWorkouts']}\")
print(f\"  total volume   {s['totalVolume']:,.0f} lb\")
print(f\"  working sets   {s['totalSets']}\")
print(f\"  PRs this month {s['prsThisMonth']}\")
print(f\"  streak         {s['streakDays']} days\")
print(f\"  tracked moves  {len(d['trackedExercises'])}\")
"

cat <<EOF

Seeded. Sign in at http://localhost:5173/login

  email:    $EMAIL
  password: $PASS

Progress pages worth visiting:
  /progress/back-squat
  /progress/bench-press
  /progress/deadlift
EOF