package application

import (
	"time"

	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type TicketView struct {
	ID          string
	RequesterID string
	CategoryID  string
	AssigneeID  *string

	Title         string
	Description   string
	Priority      string
	Status        string
	WaitingReason string
	Resolution    string

	Version uint64

	CreatedAt               time.Time
	UpdatedAt               time.Time
	ResolutionStartedAt     time.Time
	FirstResponseDeadlineAt time.Time
	ResolutionDeadlineAt    time.Time
	FirstRespondedAt        *time.Time
	ResolvedAt              *time.Time
	ClosedAt                *time.Time
}

func newTicketView(ticket *domain.Ticket) TicketView {
	snapshot := ticket.Snapshot()

	return TicketView{
		ID:                      snapshot.ID.String(),
		RequesterID:             snapshot.RequesterID.String(),
		CategoryID:              snapshot.CategoryID.String(),
		AssigneeID:              userIDString(snapshot.AssigneeID),
		Title:                   snapshot.Title,
		Description:             snapshot.Description,
		Priority:                string(snapshot.Priority),
		Status:                  string(snapshot.Status),
		WaitingReason:           snapshot.WaitingReason,
		Resolution:              snapshot.Resolution,
		Version:                 snapshot.Version,
		CreatedAt:               snapshot.CreatedAt,
		UpdatedAt:               snapshot.UpdatedAt,
		ResolutionStartedAt:     snapshot.ResolutionStartedAt,
		FirstResponseDeadlineAt: snapshot.FirstResponseDeadlineAt,
		ResolutionDeadlineAt:    snapshot.ResolutionDeadlineAt,
		FirstRespondedAt:        cloneTime(snapshot.FirstRespondedAt),
		ResolvedAt:              cloneTime(snapshot.ResolvedAt),
		ClosedAt:                cloneTime(snapshot.ClosedAt),
	}
}

func userIDString(value *domain.UserID) *string {
	if value == nil {
		return nil
	}

	result := value.String()
	return &result
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	result := *value
	return &result
}
