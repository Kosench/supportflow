package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
	"github.com/jackc/pgx/v5"
)

type OperatorRepository struct {
	db DBTX
}

func NewOperatorRepository(db DBTX) *OperatorRepository {
	return &OperatorRepository{db: db}
}

func (repository *OperatorRepository) FindBestForUpdate(
	ctx context.Context,
	categoryID domain.CategoryID,
	at time.Time,
) (ports.OperatorCandidate, error) {
	if categoryID.IsZero() || at.IsZero() {
		return ports.OperatorCandidate{}, fmt.Errorf(
			"find assignment candidate: %w",
			domain.ErrValidation,
		)
	}

	const query = `
        SELECT
            profile.user_id,
            load.active_ticket_count,
            profile.max_active_tickets
        FROM ticket.operator_profiles AS profile
        JOIN ticket.users AS account
          ON account.id = profile.user_id
        JOIN ticket.operator_skills AS skill
          ON skill.operator_id = profile.user_id
         AND skill.category_id = $1
        JOIN ticket.operator_schedules AS schedule
          ON schedule.operator_id = profile.user_id
         AND schedule.weekday = EXTRACT(
                DOW FROM ($2::timestamptz AT TIME ZONE profile.timezone)
             )::smallint
        CROSS JOIN LATERAL (
            SELECT COUNT(*)::bigint AS active_ticket_count
            FROM ticket.tickets AS active_ticket
            WHERE active_ticket.assignee_id = profile.user_id
              AND active_ticket.status IN (
                  'ASSIGNED',
                  'IN_PROGRESS',
                  'WAITING_CUSTOMER',
                  'REOPENED'
              )
        ) AS load
        WHERE account.is_active = TRUE
          AND account.role = 'OPERATOR'
          AND profile.is_available = TRUE
          AND ($2::timestamptz AT TIME ZONE profile.timezone)::time
                >= schedule.starts_at
          AND ($2::timestamptz AT TIME ZONE profile.timezone)::time
                < schedule.ends_at
          AND load.active_ticket_count < profile.max_active_tickets
        ORDER BY
            load.active_ticket_count::numeric
                / profile.max_active_tickets::numeric ASC,
            profile.last_assigned_at ASC NULLS FIRST,
            profile.user_id ASC
        FOR UPDATE OF profile, account SKIP LOCKED
        LIMIT 1
    `

	return scanOperatorCandidate(
		repository.db.QueryRow(
			ctx,
			query,
			categoryID.String(),
			at.UTC(),
		),
	)
}

func (repository *OperatorRepository) LockEligibleForUpdate(
	ctx context.Context,
	operatorID domain.UserID,
	categoryID domain.CategoryID,
	at time.Time,
) (ports.OperatorCandidate, error) {
	if operatorID.IsZero() || categoryID.IsZero() || at.IsZero() {
		return ports.OperatorCandidate{}, fmt.Errorf(
			"lock eligible operator: %w",
			domain.ErrValidation,
		)
	}

	const query = `
        SELECT
            profile.user_id,
            load.active_ticket_count,
            profile.max_active_tickets
        FROM ticket.operator_profiles AS profile
        JOIN ticket.users AS account
          ON account.id = profile.user_id
        JOIN ticket.operator_skills AS skill
          ON skill.operator_id = profile.user_id
         AND skill.category_id = $2
        JOIN ticket.operator_schedules AS schedule
          ON schedule.operator_id = profile.user_id
         AND schedule.weekday = EXTRACT(
                DOW FROM ($3::timestamptz AT TIME ZONE profile.timezone)
             )::smallint
        CROSS JOIN LATERAL (
            SELECT COUNT(*)::bigint AS active_ticket_count
            FROM ticket.tickets AS active_ticket
            WHERE active_ticket.assignee_id = profile.user_id
              AND active_ticket.status IN (
                  'ASSIGNED',
                  'IN_PROGRESS',
                  'WAITING_CUSTOMER',
                  'REOPENED'
              )
        ) AS load
        WHERE profile.user_id = $1
          AND account.is_active = TRUE
          AND account.role = 'OPERATOR'
          AND profile.is_available = TRUE
          AND ($3::timestamptz AT TIME ZONE profile.timezone)::time
                >= schedule.starts_at
          AND ($3::timestamptz AT TIME ZONE profile.timezone)::time
                < schedule.ends_at
          AND load.active_ticket_count < profile.max_active_tickets
        FOR UPDATE OF profile, account
    `

	return scanOperatorCandidate(
		repository.db.QueryRow(
			ctx,
			query,
			operatorID.String(),
			categoryID.String(),
			at.UTC(),
		),
	)
}

func (repository *OperatorRepository) LockActiveForUpdate(
	ctx context.Context,
	operatorID domain.UserID,
) error {
	if operatorID.IsZero() {
		return fmt.Errorf(
			"lock active operator: %w",
			domain.ErrValidation,
		)
	}

	const query = `
        SELECT profile.user_id
        FROM ticket.operator_profiles AS profile
        JOIN ticket.users AS account
          ON account.id = profile.user_id
        WHERE profile.user_id = $1
          AND account.is_active = TRUE
          AND account.role = 'OPERATOR'
        FOR UPDATE OF profile, account
    `

	var storedID string
	err := repository.db.QueryRow(
		ctx,
		query,
		operatorID.String(),
	).Scan(&storedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf(
			"%w: %s",
			ports.ErrOperatorNotFound,
			operatorID,
		)
	}
	if err != nil {
		return fmt.Errorf("scan active operator: %w", err)
	}

	return nil
}

func (repository *OperatorRepository) IsEligible(
	ctx context.Context,
	operatorID domain.UserID,
	categoryID domain.CategoryID,
	at time.Time,
) (bool, error) {
	if operatorID.IsZero() || categoryID.IsZero() || at.IsZero() {
		return false, fmt.Errorf(
			"recheck operator eligibility: %w",
			domain.ErrValidation,
		)
	}

	const query = `
        SELECT EXISTS (
            SELECT 1
            FROM ticket.operator_profiles AS profile
            JOIN ticket.users AS account
              ON account.id = profile.user_id
            JOIN ticket.operator_skills AS skill
              ON skill.operator_id = profile.user_id
             AND skill.category_id = $2
            JOIN ticket.operator_schedules AS schedule
              ON schedule.operator_id = profile.user_id
             AND schedule.weekday = EXTRACT(
                    DOW FROM (
                        $3::timestamptz AT TIME ZONE profile.timezone
                    )
                 )::smallint
            WHERE profile.user_id = $1
              AND account.is_active = TRUE
              AND account.role = 'OPERATOR'
              AND profile.is_available = TRUE
              AND ($3::timestamptz AT TIME ZONE profile.timezone)::time
                    >= schedule.starts_at
              AND ($3::timestamptz AT TIME ZONE profile.timezone)::time
                    < schedule.ends_at
              AND (
                  SELECT COUNT(*)
                  FROM ticket.tickets AS active_ticket
                  WHERE active_ticket.assignee_id = profile.user_id
                    AND active_ticket.status IN (
                        'ASSIGNED',
                        'IN_PROGRESS',
                        'WAITING_CUSTOMER',
                        'REOPENED'
                    )
              ) < profile.max_active_tickets
        )
    `

	var eligible bool
	err := repository.db.QueryRow(
		ctx,
		query,
		operatorID.String(),
		categoryID.String(),
		at.UTC(),
	).Scan(&eligible)
	if err != nil {
		return false, fmt.Errorf(
			"scan operator eligibility: %w",
			err,
		)
	}

	return eligible, nil
}

func (repository *OperatorRepository) MarkAssigned(
	ctx context.Context,
	operatorID domain.UserID,
	assignedAt time.Time,
) error {
	if operatorID.IsZero() || assignedAt.IsZero() {
		return fmt.Errorf(
			"mark operator assigned: %w",
			domain.ErrValidation,
		)
	}

	const query = `
        UPDATE ticket.operator_profiles
        SET last_assigned_at = $2,
            updated_at = $2
        WHERE user_id = $1
    `

	tag, err := repository.db.Exec(
		ctx,
		query,
		operatorID.String(),
		assignedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("update operator assignment time: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return fmt.Errorf(
			"%w: %s",
			ports.ErrOperatorNotFound,
			operatorID,
		)
	}

	return nil
}

func scanOperatorCandidate(
	row rowScanner,
) (ports.OperatorCandidate, error) {
	var idText string
	var activeTicketCount int64
	var maxActiveTicketCount int32

	err := row.Scan(
		&idText,
		&activeTicketCount,
		&maxActiveTicketCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.OperatorCandidate{}, ports.ErrNoEligibleOperator
	}
	if err != nil {
		return ports.OperatorCandidate{}, fmt.Errorf(
			"scan assignment candidate: %w",
			err,
		)
	}

	operatorID, err := domain.ParseUserID(idText)
	if err != nil {
		return ports.OperatorCandidate{}, fmt.Errorf(
			"map operator ID: %w",
			err,
		)
	}

	if activeTicketCount < 0 ||
		maxActiveTicketCount <= 0 ||
		activeTicketCount >= int64(maxActiveTicketCount) {
		return ports.OperatorCandidate{}, fmt.Errorf(
			"invalid assignment candidate load: %d/%d",
			activeTicketCount,
			maxActiveTicketCount,
		)
	}

	return ports.OperatorCandidate{
		UserID:               operatorID,
		ActiveTicketCount:    activeTicketCount,
		MaxActiveTicketCount: maxActiveTicketCount,
	}, nil
}
