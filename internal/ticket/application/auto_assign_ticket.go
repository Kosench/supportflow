package application

import (
	"context"
	"errors"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type AutoAssignTicketCommand struct {
	TicketID string
}

type AutoAssignTicketHandler struct {
	unitOfWork ports.UnitOfWork
	clock      ports.Clock
}

func NewAutoAssignTicketHandler(unitOfWork ports.UnitOfWork, clock ports.Clock) (AutoAssignTicketHandler, error) {
	if unitOfWork == nil || clock == nil {
		return AutoAssignTicketHandler{}, ErrInvalidDependency
	}

	return AutoAssignTicketHandler{
		unitOfWork: unitOfWork,
		clock:      clock,
	}, nil
}

func (h AutoAssignTicketHandler) Handle(ctx context.Context, command AutoAssignTicketCommand) (AssignmentResult, error) {
	ticketID, err := domain.ParseTicketID(command.TicketID)
	if err != nil {
		return AssignmentResult{}, err
	}

	var result AssignmentResult
	err = h.unitOfWork.WithinTransaction(
		ctx,
		func(repositories ports.Repositories) error {
			ticket, err := repositories.Tickets.GetByIDForUpdate(ctx, ticketID)
			if err != nil {
				return err
			}

			snapshot := ticket.Snapshot()
			if snapshot.AssigneeID != nil {
				result = newAssignmentResult(ticket, false)
				return nil
			}

			if err := ensureUnassignedTicket(ticket); err != nil {
				return err
			}

			now := h.clock.Now()
			candidate, err := repositories.Operators.FindBestForUpdate(
				ctx,
				snapshot.CategoryID,
				now,
			)
			if errors.Is(err, ports.ErrNoEligibleOperator) {
				result = newAssignmentResult(ticket, false)
				return nil
			}
			if err != nil {
				return err
			}

			eligible, err := repositories.Operators.IsEligible(
				ctx,
				candidate.UserID,
				snapshot.CategoryID,
				now,
			)
			if err != nil {
				return err
			}
			if !eligible {
				result = newAssignmentResult(ticket, false)
				return nil
			}

			if err := ticket.Assign(candidate.UserID, now); err != nil {
				return err
			}

			if err := repositories.Tickets.Update(
				ctx,
				ticket,
				snapshot.Version,
			); err != nil {
				return err
			}

			if err := repositories.Operators.MarkAssigned(
				ctx,
				candidate.UserID,
				now,
			); err != nil {
				return err
			}

			result = newAssignmentResult(ticket, true)
			return nil
		},
	)
	if err != nil {
		return AssignmentResult{}, err
	}

	return result, nil
}
