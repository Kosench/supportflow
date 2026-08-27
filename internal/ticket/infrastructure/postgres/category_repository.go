package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kosench/supportflow/internal/ticket/domain"
	"github.com/jackc/pgx/v5"
)

type CategoryRepository struct {
	db DBTX
}

func NewCategoryRepository(db DBTX) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) IsActive(ctx context.Context, id domain.CategoryID) (bool, error) {
	if id.IsZero() {
		return false, fmt.Errorf("check category: %w", domain.ErrValidation)
	}

	const query = `
        SELECT is_active
        FROM ticket.categories
        WHERE id = $1
        FOR SHARE
    `

	var active bool

	err := r.db.QueryRow(ctx, query, id.String()).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("scan category state: %w", err)
	}

	return active, nil
}
