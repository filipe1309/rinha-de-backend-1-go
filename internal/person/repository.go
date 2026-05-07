package person

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the interface for person persistence.
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Person, error)
	Search(ctx context.Context, term string) ([]Person, error)
	Count(ctx context.Context) (int, error)
	InsertBatch(ctx context.Context, persons []*Person) error
}

// PgRepository implements Repository using PostgreSQL.
type PgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository creates a new PostgreSQL repository.
func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

// GetByID retrieves a person by UUID from PostgreSQL.
func (r *PgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Person, error) {
	row := r.pool.QueryRow(ctx,
		"SELECT id, apelido, nome, nascimento, stack FROM pessoas WHERE id = $1", id)

	var p Person
	var nascimento time.Time
	var stack *string
	err := row.Scan(&p.ID, &p.Apelido, &p.Nome, &nascimento, &stack)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying person by id: %w", err)
	}
	p.Nascimento = nascimento.Format("2006-01-02")
	p.Stack = StackFromText(stack)
	return &p, nil
}

const searchPeopleQuery = "SELECT id, apelido, nome, nascimento, stack FROM pessoas WHERE search_field ILIKE '%' || $1 || '%' LIMIT 50"

// Search finds people whose search_field contains the given term (case-insensitive).
func (r *PgRepository) Search(ctx context.Context, term string) ([]Person, error) {
	rows, err := r.pool.Query(ctx, searchPeopleQuery, strings.ToLower(term))
	if err != nil {
		return nil, fmt.Errorf("searching people: %w", err)
	}
	defer rows.Close()

	var people []Person
	for rows.Next() {
		var p Person
		var nascimento time.Time
		var stack *string
		if err := rows.Scan(&p.ID, &p.Apelido, &p.Nome, &nascimento, &stack); err != nil {
			return nil, fmt.Errorf("scanning person row: %w", err)
		}
		p.Nascimento = nascimento.Format("2006-01-02")
		p.Stack = StackFromText(stack)
		people = append(people, p)
	}

	if people == nil {
		people = []Person{}
	}
	return people, nil
}

// Count returns the total number of people in the database.
func (r *PgRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM pessoas").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting people: %w", err)
	}
	return count, nil
}

// InsertBatch inserts multiple people using PostgreSQL COPY protocol.
func (r *PgRepository) InsertBatch(ctx context.Context, persons []*Person) error {
	if len(persons) == 0 {
		return nil
	}

	rows := make([][]interface{}, 0, len(persons))
	for _, p := range persons {
		rows = append(rows, []interface{}{
			p.ID,
			p.Apelido,
			p.Nome,
			p.Nascimento,
			p.StackAsText(),
			p.BuildSearchField(),
		})
	}

	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"pessoas"},
		[]string{"id", "apelido", "nome", "nascimento", "stack", "search_field"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return r.fallbackInsert(ctx, persons)
	}
	return nil
}

// fallbackInsert inserts each person individually with ON CONFLICT DO NOTHING.
func (r *PgRepository) fallbackInsert(ctx context.Context, persons []*Person) error {
	for _, p := range persons {
		_, err := r.pool.Exec(ctx,
			`INSERT INTO pessoas (id, apelido, nome, nascimento, stack, search_field)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT DO NOTHING`,
			p.ID, p.Apelido, p.Nome, p.Nascimento, p.StackAsText(), p.BuildSearchField())
		if err != nil {
			return fmt.Errorf("fallback insert for %s: %w", p.Apelido, err)
		}
	}
	return nil
}
