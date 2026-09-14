package ports

import (
	"context"
	"errors"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/domain"
)

var (
	ErrNoEligibleOperator = errors.New("no eligible operator")
	ErrOperatorNotFound   = errors.New("operator not found")
)

type OperatorCandidate struct {
	UserID               domain.UserID
	ActiveTicketCount    int64
	MaxActiveTicketCount int32
}

type OperatorRepository interface {
	FindBestForUpdate(
		ctx context.Context,
		categoryID domain.CategoryID,
		at time.Time,
	) (OperatorCandidate, error)
	LockEligibleForUpdate(
		ctx context.Context,
		operatorID domain.UserID,
		categoryID domain.CategoryID,
		at time.Time,
	) (OperatorCandidate, error)

	LockActiveForUpdate(
		ctx context.Context,
		operatorID domain.UserID,
	) error

	IsEligible(
		ctx context.Context,
		operatorID domain.UserID,
		categoryID domain.CategoryID,
		at time.Time,
	) (bool, error)

	MarkAssigned(
		ctx context.Context,
		operatorID domain.UserID,
		assignedAt time.Time,
	) error
}
