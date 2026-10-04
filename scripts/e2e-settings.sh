#!/usr/bin/env bash
# Live end-to-end check of the settings feature against a running API on :8080.
#
# The point is to prove the formula setting reaches the database: PR flags and
# chart values must actually change when the user switches formulas. A settings
# screen that saves correctly but does not alter the computation would look
# perfect while being entirely inert.
set -euo pipefail

API="${API:-http://localhost:8080}"
pass() { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
fail() { printf '  \033[31mFAIL\033[0m %s\n' "$1"; exit 1; }
step() { printf '\n\033[1m%s\033[0m\n' "$1"; }

# Reads JSON on stdin and prints the value at a path.
# Usage: ... | jp "['token']"     ... | jp "[0]['totalVolume']"
jp() {
  python3 -c "
import sys, json
d = json.load(sys.stdin)
print(eval('d' + sys.argv[1]))
" "$1"
}

EMAIL="rpe-$(date +%s%N)@example.com"
PASS='Str0ngPass!'

step "Register $EMAIL"
TOKEN=$(curl -s -X POST $API/api/auth/register -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jp "['token']")
AUTH="Authorization: Bearer $TOKEN"
[ -n "$TOKEN" ] && pass "registered" || fail "register"

step "Defaults"
S=$(curl -s $API/api/settings -H "$AUTH")
echo "$S" | grep -q '"oneRmFormula":"epley"' && pass "formula defaults to epley" || fail "default formula"
echo "$S" | grep -q '"rpeEnabled":false' && pass "RPE off by default" || fail "rpe default"
echo "$S" | grep -q '"assumeRpe":10' && pass "blank RPE treated as 10" || fail "assumeRpe default"

step "Formula catalog"
C=$(curl -s $API/api/settings/formulas -H "$AUTH")
echo "$C" | grep -q '"id":"epley"' && pass "epley offered" || fail "epley missing"
echo "$C" | grep -q '"id":"rts"' && pass "rts offered" || fail "rts missing"
echo "$C" | grep -q '"usesRpe":true' && pass "rts advertises RPE use" || fail "usesRpe flag"

EX=$(curl -s $API/api/exercises -H "$AUTH")
BENCH=$(python3 -c "import sys,json;print(next(e['id'] for e in json.load(sys.stdin) if e['slug']=='bench-press'))" <<<"$EX")
pass "bench-press id=$BENCH"

# logSet <weight> <reps> <rpe-or-empty>
logSet() {
  local w=$1 reps=$2 rpe=$3
  local wk we body
  wk=$(curl -s -X POST $API/api/workouts -H "$AUTH" -H 'Content-Type: application/json' \
    -d '{"name":"e2e formula"}' | jp "['id']")
  we=$(curl -s -X POST $API/api/workouts/$wk/exercises -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"exerciseId\":$BENCH}" | jp "['id']")
  body="{\"weight\":$w,\"reps\":$reps"
  [ -n "$rpe" ] && body="$body,\"rpe\":$rpe"
  body="$body}"
  curl -s -X POST $API/api/workout-exercises/$we/sets -H "$AUTH" -H 'Content-Type: application/json' -d "$body"
}

chartValue() {
  curl -s "$API/api/progress/exercise/bench-press" -H "$AUTH" | jp "['points'][0]['estimated1Rm']"
}

step "Epley: 225x5 @ RPE 8"
curl -s -X PATCH $API/api/settings -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"oneRmFormula":"epley","rpeEnabled":true}' >/dev/null
logSet 225 5 8 >/dev/null
E=$(chartValue)
python3 -c "
import sys
want = 225*(1+5/30)
got = float('$E')
assert abs(got-want) < 0.01, f'{got} != {want}'
print(f'  \033[32mPASS\033[0m chart shows Epley value {got:.1f} lb (RPE ignored)')
"

step "Switch to RTS: same load, RPE now counts"
curl -s -X PATCH $API/api/settings -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"oneRmFormula":"rts"}' >/dev/null
R=$(chartValue)
python3 -c "
tr = 5 + (10-8)
want = 225/(1.0261-0.0242*tr-0.0004*tr*tr)
got = float('$R')
assert abs(got-want) < 0.01, f'{got} != {want}'
print(f'  \033[32mPASS\033[0m chart now shows {got:.1f} lb for the same set (RPE-aware)')
"

step "The setting changes PR flags, not just display"
# A second session logging the identical load at an easier RPE.
SECOND=$(logSet 225 5 6)
ISPR=$(echo "$SECOND" | jp "['isPr']")
if [ "$ISPR" = "True" ]; then
  pass "225x5 @ RPE 6 flagged as a PR over the prior RPE 8 set (Epley could not detect this)"
else
  fail "RPE-aware PR detection did not fire (isPr=$ISPR)"
fi

step "Blank RPE is read as 10"
BLANK=$(logSet 200 5 "")
ISPR2=$(echo "$BLANK" | jp "['isPr']")
[ "$ISPR2" = "False" ] && pass "first 200x5 at RPE 10 is a PR" || fail "expected True, got $ISPR2"
REPEAT=$(logSet 200 5 "")
ISPR3=$(echo "$REPEAT" | jp "['isPr']")
[ "$ISPR3" = "False" ] && pass "repeating it is NOT a PR (blank == RPE 10)" || fail "blank RPE inflated a false PR"

step "Validation"
for bad in '{"oneRmFormula":"nonsense"}' '{"assumeRpe":99}' '{"weightUnit":"stone"}'; do
  CODE=$(curl -s -o /dev/null -w '%{http_code}' -X PATCH $API/api/settings -H "$AUTH" \
    -H 'Content-Type: application/json' -d "$bad")
  [ "$CODE" = "400" ] || fail "expected 400 for $bad, got $CODE"
done
pass "invalid settings rejected with 400"

step "Settings are per-user"
OTHER="other-$(date +%s%N)@example.com"
OTOK=$(curl -s -X POST $API/api/auth/register -H 'Content-Type: application/json' \
  -d "{\"email\":\"$OTHER\",\"password\":\"$PASS\"}" | jp "['token']")
DEF=$(curl -s $API/api/settings -H "Authorization: Bearer $OTOK" | jp "['oneRmFormula']")
[ "$DEF" = "epley" ] && pass "another user still defaults to epley" || fail "settings leaked ($DEF)"

step "Unauthenticated access refused"
CODE=$(curl -s -o /dev/null -w '%{http_code}' $API/api/settings)
[ "$CODE" = "401" ] && pass "401 without a token" || fail "got $CODE"

printf '\n\033[1;32mSettings feature verified end to end.\033[0m\n'