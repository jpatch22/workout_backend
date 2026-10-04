/**
 * Types mirroring the Go API's JSON shapes.
 *
 * These are hand-written rather than generated so a mismatch shows up as a
 * TypeScript error at the call site. Keep them in sync with:
 *   backend/services/*.go and backend/tables/*.go (json tags)
 */

export interface User {
  id: number;
  email: string;
  displayName: string;
}

export interface AuthResult {
  token: string;
  user: User;
}

export interface Exercise {
  id: number;
  userId?: number;
  name: string;
  slug: string;
  muscleGroup?: string;
  equipment?: string;
  isCustom: boolean;
}

export interface Set {
  id: number;
  workoutExerciseId: number;
  weight: number;
  reps: number;
  rpe?: number;
  isWarmup: boolean;
  isBodyweight: boolean;
  isPr: boolean;
  loggedAt: string;
}

export interface WorkoutExercise {
  id: number;
  workoutId: number;
  exerciseId: number;
  exercise: Exercise;
  position: number;
  notes?: string;
  sets: Set[];
}

export interface Workout {
  id: number;
  userId: number;
  name: string;
  performedAt: string;
  notes?: string;
  createdAt: string;
  updatedAt: string;
  exercises: WorkoutExercise[];
}

export interface WorkoutSummary {
  id: number;
  name: string;
  performedAt: string;
  exerciseCount: number;
  setCount: number;
  totalVolume: number;
  topEstimated1Rm: number;
}

export interface ProgressPoint {
  date: string;
  workoutId?: number;
  bestWeight: number;
  bestReps: number;
  estimated1Rm: number;
  totalVolume: number;
  setCount: number;
}

export interface ExerciseProgress {
  exerciseId: number;
  slug: string;
  name: string;
  muscleGroup?: string;
  points: ProgressPoint[];
  currentEstimated1Rm: number;
  previousEstimated1Rm: number;
  bestEstimated1Rm: number;
  bestEstimated1RmDate?: string;
  personalRecords: number;
  totalSessions: number;
  currentBestWeight: number;
  trendPercent: number;
}

export interface TrackedExercise {
  exerciseId: number;
  name: string;
  slug: string;
  muscleGroup?: string;
  lastTrained: string;
  sessionCount: number;
  bestEstimated1Rm: number;
}

export interface DashboardStats {
  totalWorkouts: number;
  totalVolume: number;
  totalSets: number;
  workoutsThisWeek: number;
  volumeThisWeek: number;
  prsThisMonth: number;
  streakDays: number;
}

export interface Dashboard {
  stats: DashboardStats;
  recentWorkouts: WorkoutSummary[];
  trackedExercises: TrackedExercise[];
}

/** Payload for creating or updating a set. */
export interface SetInput {
  weight: number;
  reps: number;
  rpe?: number;
  isWarmup?: boolean;
  isBodyweight?: boolean;
}

/** Payload for creating or updating a workout. */
export interface WorkoutInput {
  name?: string;
  performedAt?: string;
  notes?: string;
}