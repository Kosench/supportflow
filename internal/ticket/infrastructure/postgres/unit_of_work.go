package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UnitOfWork struct {
	pool *pgxpool.Pool
}

func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{pool: pool}
}

func (unit *UnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(repositories ports.Repositories) error,
) error {
	if fn == nil {
		return fmt.Errorf("transaction callback must not be nil")
	}

	tx, err := unit.pool.BeginTx(
		ctx,
		pgx.TxOptions{
			IsoLevel: pgx.ReadCommitted,
		},
	)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer rollback(tx)

	repositories := ports.Repositories{
		Tickets:     NewTicketRepository(tx),
		SLAPolicies: NewSLAPolicyRepository(tx),
	}

	if err := fn(repositories); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	_ = tx.Rollback(ctx)
}
