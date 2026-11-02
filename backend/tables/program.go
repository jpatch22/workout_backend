package tables

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type ProgramSet struct {
	ID   string   `json:"id"`
	Reps int      `json:"reps"`
	RPE  *float64 `json:"rpe,omitempty"`
}

type ProgramExercise struct {
	ExerciseID int64        `json:"exerciseId"`
	Sets       []ProgramSet `json:"sets"`
	Notes      string       `json:"notes"`
}

type ProgramSession struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Exercises []ProgramExercise `json:"exercises"`
	Status    string            `json:"status"`
	WorkoutID *int64            `json:"workoutId,omitempty"`
}

type ProgramWeek struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Sessions []ProgramSession `json:"sessions"`
}

type Program struct {
	ID          int64         `json:"id"`
	UserID      int64         `json:"userId"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Weeks       []ProgramWeek `json:"weeks"`
	CreatedAt   time.Time     `json:"createdAt"`
}

type Programs struct{ db *DB }

func NewPrograms(db *DB) *Programs { return &Programs{db: db} }

func (p *Programs) Create(ctx context.Context, program *Program) (*Program, error) {
	weeks, err := json.Marshal(program.Weeks)
	if err != nil {
		return nil, fmt.Errorf("marshal program weeks: %w", err)
	}
	var out Program
	var raw []byte
	err = p.db.Pool.QueryRow(ctx, `
		INSERT INTO programs (user_id, name, description, weeks)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, description, weeks, created_at`,
		program.UserID, program.Name, program.Description, weeks,
	).Scan(&out.ID, &out.UserID, &out.Name, &out.Description, &raw, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create program: %w", err)
	}
	if err := json.Unmarshal(raw, &out.Weeks); err != nil {
		return nil, fmt.Errorf("decode program weeks: %w", err)
	}
	return &out, nil
}

func (p *Programs) ListByUser(ctx context.Context, userID int64) ([]Program, error) {
	rows, err := p.db.Pool.Query(ctx, `
		SELECT id, user_id, name, description, weeks, created_at
		FROM programs WHERE user_id = $1 ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list programs: %w", err)
	}
	defer rows.Close()
	out := make([]Program, 0)
	for rows.Next() {
		var program Program
		var raw []byte
		if err := rows.Scan(&program.ID, &program.UserID, &program.Name, &program.Description, &raw, &program.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan program: %w", err)
		}
		if err := json.Unmarshal(raw, &program.Weeks); err != nil {
			return nil, fmt.Errorf("decode program weeks: %w", err)
		}
		out = append(out, program)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate programs: %w", err)
	}
	return out, nil
}

func (p *Programs) GetOwned(ctx context.Context, userID, id int64) (*Program, error) {
	var out Program
	var raw []byte
	err := p.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, name, description, weeks, created_at
		FROM programs WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&out.ID, &out.UserID, &out.Name, &out.Description, &raw, &out.CreatedAt)
	if err != nil {
		return nil, wrapNoRows(err, fmt.Errorf("get owned program %d: %w", id, err))
	}
	if err := json.Unmarshal(raw, &out.Weeks); err != nil {
		return nil, fmt.Errorf("decode program weeks: %w", err)
	}
	return &out, nil
}

func (p *Programs) UpdateWeeks(ctx context.Context, userID, id int64, weeks []ProgramWeek) error {
	raw, err := json.Marshal(weeks)
	if err != nil {
		return fmt.Errorf("marshal program weeks: %w", err)
	}
	tag, err := p.db.Pool.Exec(ctx, `UPDATE programs SET weeks = $3 WHERE id = $1 AND user_id = $2`, id, userID, raw)
	if err != nil {
		return fmt.Errorf("update program: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
