package ports

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kosench/supportflow/internal/ticket/domain"
)

var (
	ErrTicketNotFound      = errors.New("ticket not found")
	ErrTicketAlreadyExists = errors.New("ticket already exists")
	ErrVersionConflict     = errors.New("ticket version conflict")
	ErrSLAPolicyNotFound   = errors.New("SLA policy not found")
)

type VersionConflictError struct {
	TicketID domain.TicketID
	Expected uint64
	Actual   uint64
}

func (err VersionConflictError) Error() string {
	return fmt.Sprintf(
		"%s: ticket=%s expected=%d actual=%d",
		ErrVersionConflict,
		err.TicketID,
		err.Expected,
		err.Actual,
	)
}

func (err VersionConflictError) Unwrap() error {
	return ErrVersionConflict
}

type TicketRepository interface {
	Create(ctx context.Context, ticket *domain.Ticket) error
	GetByID(
		ctx context.Context,
		ticketID domain.TicketID,
	) (*domain.Ticket, error)
	Update(
		ctx context.Context,
		ticket *domain.Ticket,
		expectedVersion uint64,
	) error
}

type SLAPolicyRepository interface {
	GetByPriority(
		ctx context.Context,
		priority domain.Priority,
	) (domain.SLATarget, error)
}

type Repositories struct {
	Tickets     TicketRepository
	SLAPolicies SLAPolicyRepository
}

type UnitOfWork interface {
	WithinTransaction(
		ctx context.Context,
		fn func(repositories Repositories) error,
	) error
}
