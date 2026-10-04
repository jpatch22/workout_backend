-- 002_seed_exercises.up.sql
-- Starter catalog so a new user can log a real workout immediately.
-- user_id IS NULL marks these as global (shared, immutable).

INSERT INTO exercises (user_id, name, slug, muscle_group, equipment, is_custom) VALUES
  (NULL, 'Back Squat',          'back-squat',          'legs',     'barbell',   FALSE),
  (NULL, 'Front Squat',         'front-squat',         'legs',     'barbell',   FALSE),
  (NULL, 'Romanian Deadlift',   'romanian-deadlift',   'legs',     'barbell',   FALSE),
  (NULL, 'Conventional Deadlift','conventional-deadlift','back',   'barbell',   FALSE),
  (NULL, 'Deadlift',            'deadlift',            'back',     'barbell',   FALSE),
  (NULL, 'Bench Press',         'bench-press',         'chest',    'barbell',   FALSE),
  (NULL, 'Incline Bench Press', 'incline-bench-press', 'chest',    'barbell',   FALSE),
  (NULL, 'Overhead Press',      'overhead-press',      'shoulders','barbell',   FALSE),
  (NULL, 'Pull-Up',             'pull-up',             'back',     'bodyweight',FALSE),
  (NULL, 'Chin-Up',             'chin-up',             'back',     'bodyweight',FALSE),
  (NULL, 'Barbell Row',         'barbell-row',         'back',     'barbell',   FALSE),
  (NULL, 'Lat Pulldown',        'lat-pulldown',        'back',     'cable',     FALSE),
  (NULL, 'Dumbbell Curl',       'dumbbell-curl',       'biceps',   'dumbbell',  FALSE),
  (NULL, 'Triceps Pushdown',    'triceps-pushdown',    'triceps',  'cable',     FALSE),
  (NULL, 'Leg Press',           'leg-press',           'legs',     'machine',   FALSE),
  (NULL, 'Leg Curl',            'leg-curl',            'legs',     'machine',   FALSE),
  (NULL, 'Leg Extension',       'leg-extension',       'legs',     'machine',   FALSE),
  (NULL, 'Hip Thrust',          'hip-thrust',          'glutes',   'barbell',   FALSE),
  (NULL, 'Bulgarian Split Squat','bulgarian-split-squat','legs',    'dumbbell',  FALSE),
  (NULL, 'Lateral Raise',       'lateral-raise',       'shoulders','dumbbell',  FALSE),
  (NULL, 'Face Pull',           'face-pull',           'shoulders','cable',     FALSE),
  (NULL, 'Plank',               'plank',               'core',     'bodyweight',FALSE)
ON CONFLICT DO NOTHING;