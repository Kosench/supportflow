package postgres

import (
	"fmt"
	"time"

	"github.com/Kosench/supportflow/internal/ticket/domain"
	"github.com/jackc/pgx/v5/pgtype"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type ticketRow struct {
	ID          pgtype.UUID
	RequesterID pgtype.UUID
	CategoryID  pgtype.UUID
	AssigneeID  pgtype.UUID

	Title         string
	Description   string
	Priority      string
	Status        string
	WaitingReason string
	Resolution    string

	Version int64

	CreatedAt               time.Time
	UpdatedAt               time.Time
	ResolutionStartedAt     time.Time
	FirstResponseDeadlineAt time.Time
	ResolutionDeadlineAt    time.Time
	FirstRespondedAt        pgtype.Timestamptz
	ResolvedAt              pgtype.Timestamptz
	ClosedAt                pgtype.Timestamptz
}

func scanTicket(scanner rowScanner) (*domain.Ticket, error) {
	var row ticketRow

	err := scanner.Scan(
		&row.ID,
		&row.RequesterID,
		&row.CategoryID,
		&row.AssigneeID,
		&row.Title,
		&row.Description,
		&row.Priority,
		&row.Status,
		&row.WaitingReason,
		&row.Resolution,
		&row.Version,
		&row.CreatedAt,
		&row.UpdatedAt,
		&row.ResolutionStartedAt,
		&row.FirstResponseDeadlineAt,
		&row.ResolutionDeadlineAt,
		&row.FirstRespondedAt,
		&row.ResolvedAt,
		&row.ClosedAt,
	)
	if err != nil {
		return nil, err
	}

	return row.toDomain()
}

func (row ticketRow) toDomain() (*domain.Ticket, error) {
	ticketID, err := domain.ParseTicketID(row.ID.String())
	if err != nil {
		return nil, fmt.Errorf("map ticket ID: %w", err)
	}

	requesterID, err := domain.ParseUserID(row.RequesterID.String())
	if err != nil {
		return nil, fmt.Errorf("map requester ID: %w", err)
	}

	categoryID, err := domain.ParseCategoryID(row.CategoryID.String())
	if err != nil {
		return nil, fmt.Errorf("map category ID: %w", err)
	}

	assigneeID, err := optionalUserID(row.AssigneeID)
	if err != nil {
		return nil, err
	}

	priority, err := domain.ParsePriority(row.Priority)
	if err != nil {
		return nil, fmt.Errorf("map priority: %w", err)
	}

	status, err := domain.ParseStatus(row.Status)
	if err != nil {
		return nil, fmt.Errorf("map status: %w", err)
	}

	if row.Version <= 0 {
		return nil, fmt.Errorf(
			"map ticket version: %w",
			domain.ErrValidation,
		)
	}

	return domain.RestoreTicket(domain.TicketSnapshot{
		ID:                      ticketID,
		RequesterID:             requesterID,
		CategoryID:              categoryID,
		AssigneeID:              assigneeID,
		Title:                   row.Title,
		Description:             row.Description,
		Priority:                priority,
		Status:                  status,
		WaitingReason:           row.WaitingReason,
		Resolution:              row.Resolution,
		Version:                 uint64(row.Version),
		CreatedAt:               row.CreatedAt,
		UpdatedAt:               row.UpdatedAt,
		ResolutionStartedAt:     row.ResolutionStartedAt,
		FirstResponseDeadlineAt: row.FirstResponseDeadlineAt,
		ResolutionDeadlineAt:    row.ResolutionDeadlineAt,
		FirstRespondedAt:        optionalTimestamp(row.FirstRespondedAt),
		ResolvedAt:              optionalTimestamp(row.ResolvedAt),
		ClosedAt:                optionalTimestamp(row.ClosedAt),
	})
}

func optionalUserID(value pgtype.UUID) (*domain.UserID, error) {
	if !value.Valid {
		return nil, nil
	}

	id, err := domain.ParseUserID(value.String())
	if err != nil {
		return nil, fmt.Errorf("map assignee ID: %w", err)
	}

	return &id, nil
}

func optionalTimestamp(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	result := value.Time.UTC()
	return &result
}

func nullableUserID(value *domain.UserID) any {
	if value == nil {
		return nil
	}

	return value.String()
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}

	return value.UTC()
}
