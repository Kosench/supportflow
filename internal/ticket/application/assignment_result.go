package application

import "github.com/Kosench/supportflow/internal/ticket/domain"

type AssignmentResult struct {
	Ticket     TicketView
	Assigned   bool
	Changed    bool
	OperatorID *string
}

func newAssignmentResult(
	ticket *domain.Ticket,
	changed bool,
) AssignmentResult {
	snapshot := ticket.Snapshot()

	return AssignmentResult{
		Ticket:     newTicketView(ticket),
		Assigned:   snapshot.AssigneeID != nil,
		Changed:    changed,
		OperatorID: userIDString(snapshot.AssigneeID),
	}
}

func ensureUnassignedTicket(ticket *domain.Ticket) error {
	snapshot := ticket.Snapshot()

	if snapshot.AssigneeID != nil {
		return ErrTicketNotAssignable
	}

	if snapshot.Status != domain.StatusNew &&
		snapshot.Status != domain.StatusReopened {
		return ErrTicketNotAssignable
	}

	return nil
}
