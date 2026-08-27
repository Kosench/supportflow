package ports

import (
	"context"

	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type CategoryRepository interface {
	IsActive(
		ctx context.Context,
		categoryID domain.CategoryID,
	) (bool, error)
}
