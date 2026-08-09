package domain

import "time"

type EventType string

const (
	EventTicketCreated         EventType = "ticket.created"
	EventTicketAssigned        EventType = "ticket.assigned"
	EventTicketUnassigned      EventType = "ticket.unassigned"
	EventTicketStatusChanged   EventType = "ticket.status_changed"
	EventTicketPriorityChanged EventType = "ticket.priority_changed"
	EventTicketFirstResponded  EventType = "ticket.first_responded"
	EventTicketResolved        EventType = "ticket.resolved"
	EventTicketClosed          EventType = "ticket.closed"
	EventTicketReopened        EventType = "ticket.reopened"
)

type DomainEvent struct {
	Type             EventType
	AggregateID      TicketID
	AggregateVersion uint64
	OccurredAt       time.Time
	Data             any
}

type TicketCreatedData struct {
	RequesterID             UserID
	CategoryID              CategoryID
	Priority                Priority
	Status                  Status
	Title                   string
	FirstResponseDeadlineAt time.Time
	ResolutionDeadlineAt    time.Time
}

type TicketAssignedData struct {
	AssigneeID         UserID
	PreviousAssigneeID *UserID
}

type TicketUnassignedData struct {
	PreviousAssigneeID UserID
}

type TicketStatusChangedData struct {
	From   Status
	To     Status
	Reason string
}

type TicketPriorityChangedData struct {
	From                    Priority
	To                      Priority
	Reason                  string
	FirstResponseDeadlineAt time.Time
	ResolutionDeadlineAt    time.Time
}

type TicketFirstRespondedData struct {
	RespondedAt time.Time
}

type TicketReopenedData struct {
	Reason               string
	ResolutionDeadlineAt time.Time
}
