package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SLAPolicyRepository struct {
	db DBTX
}

func NewSLAPolicyRepository(db DBTX) *SLAPolicyRepository {
	return &SLAPolicyRepository{db: db}
}

func (repository *SLAPolicyRepository) GetByPriority(
	ctx context.Context,
	priority domain.Priority,
) (domain.SLATarget, error) {
	if !priority.Valid() {
		return domain.SLATarget{}, fmt.Errorf(
			"get SLA policy: %w",
			domain.ErrValidation,
		)
	}

	const query = `
        SELECT
            first_response_interval,
            resolution_interval
        FROM ticket.sla_policies
        WHERE priority = $1
    `

	var firstResponse pgtype.Interval
	var resolution pgtype.Interval

	err := repository.db.QueryRow(
		ctx,
		query,
		string(priority),
	).Scan(&firstResponse, &resolution)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLATarget{}, fmt.Errorf(
			"%w: %s",
			ports.ErrSLAPolicyNotFound,
			priority,
		)
	}
	if err != nil {
		return domain.SLATarget{}, fmt.Errorf("scan SLA policy: %w", err)
	}

	firstResponseDuration, err := intervalDuration(firstResponse)
	if err != nil {
		return domain.SLATarget{}, fmt.Errorf(
			"map first response interval: %w",
			err,
		)
	}

	resolutionDuration, err := intervalDuration(resolution)
	if err != nil {
		return domain.SLATarget{}, fmt.Errorf(
			"map resolution interval: %w",
			err,
		)
	}

	target := domain.SLATarget{
		FirstResponse: firstResponseDuration,
		Resolution:    resolutionDuration,
	}

	if err := target.Validate(); err != nil {
		return domain.SLATarget{}, fmt.Errorf(
			"validate stored SLA policy: %w",
			err,
		)
	}

	return target, nil
}

func intervalDuration(value pgtype.Interval) (time.Duration, error) {
	if !value.Valid {
		return 0, fmt.Errorf("interval is NULL")
	}

	if value.Months != 0 {
		return 0, fmt.Errorf(
			"calendar months are not supported in SLA interval",
		)
	}

	return time.Duration(value.Days)*24*time.Hour +
		time.Duration(value.Microseconds)*time.Microsecond, nil
}
