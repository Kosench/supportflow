package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const ticketColumns = `
    id,
    requester_id,
    category_id,
    assignee_id,
    title,
    description,
    priority,
    status,
    waiting_reason,
    resolution,
    version,
    created_at,
    updated_at,
    resolution_started_at,
    first_response_deadline_at,
    resolution_deadline_at,
    first_responded_at,
    resolved_at,
    closed_at
`

type DBTX interface {
	Exec(
		ctx context.Context,
		sql string,
		arguments ...any,
	) (pgconn.CommandTag, error)
	QueryRow(
		ctx context.Context,
		sql string,
		args ...any,
	) pgx.Row
}

type TicketRepository struct {
	db DBTX
}

const maxDatabaseVersion = uint64(1<<63 - 1)

func NewTicketRepository(db DBTX) *TicketRepository {
	return &TicketRepository{db: db}
}

func (repository *TicketRepository) Create(
	ctx context.Context,
	ticket *domain.Ticket,
) error {
	if ticket == nil {
		return fmt.Errorf("create ticket: ticket must not be nil")
	}

	snapshot := ticket.Snapshot()
	if snapshot.Version > maxDatabaseVersion {
		return fmt.Errorf(
			"create ticket: %w: version exceeds BIGINT",
			domain.ErrValidation,
		)
	}

	const query = `
        INSERT INTO ticket.tickets (
            id,
            requester_id,
            category_id,
            assignee_id,
            title,
            description,
            priority,
            status,
            waiting_reason,
            resolution,
            version,
            created_at,
            updated_at,
            resolution_started_at,
            first_response_deadline_at,
            resolution_deadline_at,
            first_responded_at,
            resolved_at,
            closed_at
        )
        VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
            $11, $12, $13, $14, $15, $16, $17, $18, $19
        )
    `

	tag, err := repository.db.Exec(
		ctx,
		query,
		snapshot.ID.String(),
		snapshot.RequesterID.String(),
		snapshot.CategoryID.String(),
		nullableUserID(snapshot.AssigneeID),
		snapshot.Title,
		snapshot.Description,
		string(snapshot.Priority),
		string(snapshot.Status),
		snapshot.WaitingReason,
		snapshot.Resolution,
		int64(snapshot.Version),
		snapshot.CreatedAt,
		snapshot.UpdatedAt,
		snapshot.ResolutionStartedAt,
		snapshot.FirstResponseDeadlineAt,
		snapshot.ResolutionDeadlineAt,
		nullableTime(snapshot.FirstRespondedAt),
		nullableTime(snapshot.ResolvedAt),
		nullableTime(snapshot.ClosedAt),
	)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) &&
			postgresError.Code == "23505" {
			return fmt.Errorf(
				"%w: %s",
				ports.ErrTicketAlreadyExists,
				snapshot.ID,
			)
		}

		return fmt.Errorf("insert ticket: %w", err)
	}

	if tag.RowsAffected() != 1 {
		return fmt.Errorf(
			"insert ticket: expected 1 affected row, got %d",
			tag.RowsAffected(),
		)
	}

	return nil
}

func (repository *TicketRepository) GetByID(
	ctx context.Context,
	ticketID domain.TicketID,
) (*domain.Ticket, error) {
	if ticketID.IsZero() {
		return nil, fmt.Errorf(
			"get ticket: %w",
			domain.ErrValidation,
		)
	}

	query := `SELECT ` + ticketColumns + `
        FROM ticket.tickets
        WHERE id = $1
    `

	ticket, err := scanTicket(
		repository.db.QueryRow(ctx, query, ticketID.String()),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf(
			"%w: %s",
			ports.ErrTicketNotFound,
			ticketID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("scan ticket: %w", err)
	}

	return ticket, nil
}

func (repository *TicketRepository) Update(
	ctx context.Context,
	ticket *domain.Ticket,
	expectedVersion uint64,
) error {
	if ticket == nil {
		return fmt.Errorf("update ticket: ticket must not be nil")
	}

	snapshot := ticket.Snapshot()
	if expectedVersion == 0 {
		return fmt.Errorf(
			"update ticket: %w: expected version must be positive",
			domain.ErrValidation,
		)
	}

	if snapshot.Version <= expectedVersion {
		return fmt.Errorf(
			"update ticket: %w: new version must exceed expected version",
			domain.ErrValidation,
		)
	}

	if snapshot.Version > maxDatabaseVersion ||
		expectedVersion > maxDatabaseVersion {
		return fmt.Errorf(
			"update ticket: %w: version exceeds BIGINT",
			domain.ErrValidation,
		)
	}

	const query = `
        UPDATE ticket.tickets
        SET
            category_id = $2,
            assignee_id = $3,
            title = $4,
            description = $5,
            priority = $6,
            status = $7,
            waiting_reason = $8,
            resolution = $9,
            version = $10,
            updated_at = $11,
            resolution_started_at = $12,
            first_response_deadline_at = $13,
            resolution_deadline_at = $14,
            first_responded_at = $15,
            resolved_at = $16,
            closed_at = $17
        WHERE id = $1
          AND version = $18
        RETURNING version
    `

	var storedVersion int64
	err := repository.db.QueryRow(
		ctx,
		query,
		snapshot.ID.String(),
		snapshot.CategoryID.String(),
		nullableUserID(snapshot.AssigneeID),
		snapshot.Title,
		snapshot.Description,
		string(snapshot.Priority),
		string(snapshot.Status),
		snapshot.WaitingReason,
		snapshot.Resolution,
		int64(snapshot.Version),
		snapshot.UpdatedAt,
		snapshot.ResolutionStartedAt,
		snapshot.FirstResponseDeadlineAt,
		snapshot.ResolutionDeadlineAt,
		nullableTime(snapshot.FirstRespondedAt),
		nullableTime(snapshot.ResolvedAt),
		nullableTime(snapshot.ClosedAt),
		int64(expectedVersion),
	).Scan(&storedVersion)
	if err == nil {
		if storedVersion != int64(snapshot.Version) {
			return fmt.Errorf(
				"update ticket: unexpected returned version %d",
				storedVersion,
			)
		}

		return nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("update ticket: %w", err)
	}

	actualVersion, versionErr := repository.currentVersion(
		ctx,
		snapshot.ID,
	)
	if errors.Is(versionErr, ports.ErrTicketNotFound) {
		return versionErr
	}
	if versionErr != nil {
		return fmt.Errorf(
			"read ticket version after failed update: %w",
			versionErr,
		)
	}

	return ports.VersionConflictError{
		TicketID: snapshot.ID,
		Expected: expectedVersion,
		Actual:   actualVersion,
	}
}

func (repository *TicketRepository) currentVersion(
	ctx context.Context,
	ticketID domain.TicketID,
) (uint64, error) {
	const query = `
        SELECT version
        FROM ticket.tickets
        WHERE id = $1
    `

	var version int64
	err := repository.db.QueryRow(
		ctx,
		query,
		ticketID.String(),
	).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf(
			"%w: %s",
			ports.ErrTicketNotFound,
			ticketID,
		)
	}
	if err != nil {
		return 0, err
	}

	if version <= 0 {
		return 0, fmt.Errorf(
			"invalid stored ticket version: %d",
			version,
		)
	}

	return uint64(version), nil
}
