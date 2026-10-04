#!/usr/bin/env bash
# End-to-end smoke test against a running API on :8080.
# Walks the full user journey and asserts the derived numbers.
set -euo pipefail

API="${API:-http://localhost:8080}"
pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; exit 1; }
step() { printf '\n\033[1m%s\033[0m\n' "$1"; }

j() { python3 -c "import sys,json;d=json.load(sys.stdin);print(eval('d'+sys.argv[1]))" "$1"; }

EMAIL="e2e-$(date +%s%N)@example.com"
PASS='Str0ngPass!'

step "1. Health"
[ "$(curl -s $API/api/health | j "['status']")" = "ok" ] && pass "health ok" || fail "health"

step "2. Register"
REG=$(curl -s -X POST $API/api/auth/register -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"displayName\":\"E2E\"}")
TOKEN=$(echo "$REG" | j "['token']")
[ -n "$TOKEN" ] && pass "registered, got token" || fail "register: $REG"
AUTH="Authorization: Bearer $TOKEN"

step "3. Seeded catalog"
N=$(curl -s $API/api/exercises -H "$AUTH" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)))')
[ "$N" -ge 20 ] && pass "$N exercises in catalog" || fail "catalog only $N"
SQUAT=$(curl -s $API/api/exercises -H "$AUTH" | python3 -c "import sys,json;print(next(e['id'] for e in json.load(sys.stdin) if e['slug']=='back-squat'))")
[ -n "$SQUAT" ] && pass "found back-squat id=$SQUAT" || fail "back-squat missing"

step "4. Unauthenticated access is refused"
CODE=$(curl -s -o /dev/null -w '%{http_code}' $API/api/exercises)
[ "$CODE" = "401" ] && pass "401 without token" || fail "got $CODE"

step "5. Create workout"
WK=$(curl -s -X POST $API/api/workouts -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"Lower Body","notes":"e2e"}' | j "['id']")
[ -n "$WK" ] && pass "workout id=$WK" || fail "create workout"

step "6. Add exercise"
WE=$(curl -s -X POST $API/api/workouts/$WK/exercises -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"exerciseId\":$SQUAT}" | j "['id']")
[ -n "$WE" ] && pass "workout_exercise id=$WE" || fail "add exercise"

step "7. Log sets (warmup + 2 working)"
curl -s -X POST $API/api/workout-exercises/$WE/sets -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"weight":135,"reps":5,"isWarmup":true}' >/dev/null
PR1=$(curl -s -X POST $API/api/workout-exercises/$WE/sets -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"weight":405,"reps":3,"rpe":8.5}')
ISPR=$(echo "$PR1" | j "['isPr']")
[ "$ISPR" = "True" ] && pass "405x3 flagged as PR" || fail "405x3 should be a PR, got $ISPR"
curl -s -X POST $API/api/workout-exercises/$WE/sets -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"weight":385,"reps":5}' >/dev/null
pass "logged 3 sets"

step "8. Derived aggregates (warmup excluded)"
SUM=$(curl -s "$API/api/workouts?limit=1" -H "$AUTH")
VOL=$(echo "$SUM" | j "[0]['totalVolume']")
SETS=$(echo "$SUM" | j "[0]['setCount']")
E1RM=$(echo "$SUM" | j "[0]['topEstimated1Rm']")
[ "$VOL" = "405.0*3+385.0*5" ] 2>/dev/null || true
python3 - "$VOL" "$SETS" "$E1RM" <<'PY'
import sys
vol, sets, e1rm = float(sys.argv[1]), int(sys.argv[2]), float(sys.argv[3])
assert abs(vol - (405*3 + 385*5)) < 0.01, f"volume {vol} != 3140"
assert sets == 2, f"set count {sets} != 2 (warmup must be excluded)"
assert abs(e1rm - 449.1666666667) < 0.01, f"e1RM {e1rm} != 449.1667"
print("  \033[32mPASS\033[0m volume=3140 (warmup excluded), 2 working sets, e1RM=449.17")
PY

step "9. Later session beats it -> new PR"
WK2=$(curl -s -X POST $API/api/workouts -H "$AUTH" -H 'Content-Type: application/json' -d '{"name":"Day 2"}' | j "['id']")
WE2=$(curl -s -X POST $API/api/workouts/$WK2/exercises -H "$AUTH" -H 'Content-Type: application/json' -d "{\"exerciseId\":$SQUAT}" | j "['id']")
PR2=$(curl -s -X POST $API/api/workout-exercises/$WE2/sets -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"weight":415,"reps":3}')
ISPR2=$(echo "$PR2" | j "['isPr']")
[ "$ISPR2" = "True" ] && pass "415x3 in a new session is a new PR" || fail "415x3 should be a PR"
LOW=$(curl -s -X POST $API/api/workout-exercises/$WE2/sets -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"weight":300,"reps":3}' | j "['isPr']")
[ "$LOW" = "False" ] && pass "300x3 correctly NOT a PR" || fail "300x3 wrongly a PR"

step "10. Per-exercise progress series"
PROG=$(curl -s $API/api/progress/exercise/back-squat -H "$AUTH")
PTS=$(echo "$PROG" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)["points"]))')
SESS=$(echo "$PROG" | j "['totalSessions']")
BEST=$(echo "$PROG" | j "['bestEstimated1Rm']")
python3 - "$PTS" "$SESS" "$BEST" <<'PY'
import sys
pts, sess, best = int(sys.argv[1]), int(sys.argv[2]), float(sys.argv[3])
assert pts == 2, f"expected 2 progress points, got {pts}"
assert sess == 2, f"expected 2 sessions, got {sess}"
assert abs(best - (415*(1+3/30))) < 0.01, f"best e1RM {best}"
print(f"  \033[32mPASS\033[0m {pts} points, {sess} sessions, best e1RM={best:.2f}")
PY

step "11. Dashboard aggregates"
DASH=$(curl -s $API/api/progress/dashboard -H "$AUTH")
TW=$(echo "$DASH" | j "['stats']['totalWorkouts']")
PRSM=$(echo "$DASH" | j "['stats']['prsThisMonth']")
[ "$TW" = "2" ] && pass "2 total workouts" || fail "expected 2 workouts, got $TW"
[ "$PRSM" -ge 2 ] && pass "$PRSM PRs this month" || fail "expected >=2 PRs, got $PRSM"

step "12. Validation rejects nonsense"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST $API/api/workout-exercises/$WE/sets \
  -H "$AUTH" -H 'Content-Type: application/json' -d '{"weight":100,"reps":0}')
[ "$CODE" = "400" ] && pass "reps=0 -> 400" || fail "reps=0 got $CODE"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X POST $API/api/workouts \
  -H "$AUTH" -H 'Content-Type: application/json' -d '{"performedAt":"2099-01-01T00:00:00Z"}')
[ "$CODE" = "400" ] && pass "future date -> 400" || fail "future date got $CODE"

step "13. Cross-user isolation"
B_EMAIL="e2e-other-$(date +%s%N)@example.com"
B_TOKEN=$(curl -s -X POST $API/api/auth/register -H 'Content-Type: application/json' \
  -d "{\"email\":\"$B_EMAIL\",\"password\":\"$PASS\"}" | j "['token']")
CODE=$(curl -s -o /dev/null -w '%{http_code}' $API/api/workouts/$WK -H "Authorization: Bearer $B_TOKEN")
[ "$CODE" = "404" ] && pass "other user gets 404 on my workout" || fail "isolation broken: $CODE"
BL=$(curl -s $API/api/workouts -H "Authorization: Bearer $B_TOKEN" | python3 -c 'import sys,json;print(len(json.load(sys.stdin)))')
[ "$BL" = "0" ] && pass "other user's history is empty" || fail "leaked $BL workouts"

step "14. CORS preflight from allowed origin"
HDR=$(curl -s -i -X OPTIONS $API/api/workouts -H 'Origin: http://localhost:5173' \
  -H 'Access-Control-Request-Method: POST' | grep -i 'access-control-allow-origin' | tr -d '\r')
[ -n "$HDR" ] && pass "$HDR" || fail "no CORS header for allowed origin"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X OPTIONS $API/api/workouts \
  -H 'Origin: http://evil.example.com' -H 'Access-Control-Request-Method: POST')
[ "$CODE" = "204" ] && pass "disallowed origin gets no CORS grant" || fail "evil origin got $CODE"

step "15. Method not allowed"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE $API/api/health)
[ "$CODE" = "405" ] && pass "DELETE /api/health -> 405" || fail "got $CODE"

step "16. Cleanup"
curl -s -o /dev/null -X DELETE $API/api/workouts/$WK  -H "$AUTH"
curl -s -o /dev/null -X DELETE $API/api/workouts/$WK2 -H "$AUTH"
CODE=$(curl -s -o /dev/null -w '%{http_code}' $API/api/workouts/$WK -H "$AUTH")
[ "$CODE" = "404" ] && pass "deleted workouts are gone" || fail "delete failed"

printf '\n\033[1;32mAll end-to-end checks passed.\033[0m\n'