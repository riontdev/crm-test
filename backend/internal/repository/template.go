package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Template struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Category  string    `db:"category"`
	Content   string    `db:"content"`
	Language  string    `db:"language"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

const templateColumns = `id, name, category, content, language, created_at, updated_at`

func scanTemplate(row pgx.Row) (*Template, error) {
	var t Template
	if err := row.Scan(&t.ID, &t.Name, &t.Category, &t.Content, &t.Language, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

type TemplateRepository struct {
	pool *pgxpool.Pool
}

func NewTemplateRepository(pool *pgxpool.Pool) *TemplateRepository {
	return &TemplateRepository{pool: pool}
}

// List returns templates ordered by updated_at DESC, optionally filtered by
// case-insensitive search over name/content and by exact category.
func (r *TemplateRepository) List(ctx context.Context, search, category string) ([]*Template, error) {
	where := []string{}
	args := []interface{}{}

	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+search+"%")
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR content ILIKE $%d)", len(args), len(args)))
	}
	if category = strings.TrimSpace(category); category != "" {
		args = append(args, category)
		where = append(where, fmt.Sprintf("category = $%d", len(args)))
	}

	query := `SELECT ` + templateColumns + ` FROM templates`
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	results := []*Template{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template: %w", err)
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate templates: %w", err)
	}
	return results, nil
}

// GetByID returns a single template by its UUID.
func (r *TemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*Template, error) {
	t, err := scanTemplate(r.pool.QueryRow(ctx,
		`SELECT `+templateColumns+` FROM templates WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	return t, nil
}

// Create inserts a new template and returns the full stored row.
func (r *TemplateRepository) Create(ctx context.Context, name, category, content, language string) (*Template, error) {
	t, err := scanTemplate(r.pool.QueryRow(ctx,
		`INSERT INTO templates (name, category, content, language)
		 VALUES ($1, $2, $3, $4)
		 RETURNING `+templateColumns,
		name, category, content, language))
	if err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}
	return t, nil
}

// Update applies partial changes to a template and returns the updated row.
// Returns pgx.ErrNoRows if the template does not exist.
func (r *TemplateRepository) Update(ctx context.Context, id uuid.UUID, name, category, content, language *string) (*Template, error) {
	setClauses := []string{}
	args := []interface{}{}

	if name != nil {
		args = append(args, *name)
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", len(args)))
	}
	if category != nil {
		args = append(args, *category)
		setClauses = append(setClauses, fmt.Sprintf("category = $%d", len(args)))
	}
	if content != nil {
		args = append(args, *content)
		setClauses = append(setClauses, fmt.Sprintf("content = $%d", len(args)))
	}
	if language != nil {
		args = append(args, *language)
		setClauses = append(setClauses, fmt.Sprintf("language = $%d", len(args)))
	}

	query := `UPDATE templates SET updated_at = now()`
	for _, clause := range setClauses {
		query += `, ` + clause
	}
	args = append(args, id)
	query += fmt.Sprintf(` WHERE id = $%d RETURNING %s`, len(args), templateColumns)

	t, err := scanTemplate(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, fmt.Errorf("failed to update template: %w", err)
	}
	return t, nil
}

// Delete removes a template. Returns pgx.ErrNoRows if it does not exist.
func (r *TemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
