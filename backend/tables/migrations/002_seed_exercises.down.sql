-- 002_seed_exercises.down.sql
DELETE FROM exercises
WHERE user_id IS NULL
  AND slug IN (
    'back-squat','front-squat','romanian-deadlift','conventional-deadlift','deadlift',
    'bench-press','incline-bench-press','overhead-press','pull-up','chin-up','barbell-row',
    'lat-pulldown','dumbbell-curl','triceps-pushdown','leg-press','leg-curl','leg-extension',
    'hip-thrust','bulgarian-split-squat','lateral-raise','face-pull','plank'
  );