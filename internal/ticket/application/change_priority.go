package application

import (
	"context"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type ChangePriorityCommand struct {
	Actor           Actor
	TicketID        string
	Priority        string
	Reason          string
	ExpectedVersion uint64
}
type ChangePriorityHandler struct {
	unitOfWork ports.UnitOfWork
	clock      ports.Clock
}

func NewChangePriorityHandler(
	unitOfWork ports.UnitOfWork,
	clock ports.Clock,
) (ChangePriorityHandler, error) {
	if unitOfWork == nil || clock == nil {
		return ChangePriorityHandler{}, ErrInvalidDependency
	}
	return ChangePriorityHandler{
		unitOfWork: unitOfWork,
		clock:      clock,
	}, nil
}

func (h ChangePriorityHandler) Handle(
	ctx context.Context,
	command ChangePriorityCommand,
) (TicketView, error) {
	if err := command.Actor.Valid(); err != nil {
		return TicketView{}, err
	}

	ticketID, err := domain.ParseTicketID(command.TicketID)
	if err != nil {
		return TicketView{}, err
	}

	priority, err := domain.ParsePriority(command.Priority)
	if err != nil {
		return TicketView{}, err
	}

	if err := validateExpectedVersion(command.ExpectedVersion); err != nil {
		return TicketView{}, err
	}

	var result TicketView
	err = h.unitOfWork.WithinTransaction(
		ctx,
		func(repositories ports.Repositories) error {
			ticket, loadedVersion, err := loadTicketForUpdate(ctx, repositories.Tickets, ticketID)
			if err != nil {
				return err
			}

			if err := authorizePriorityChange(command.Actor, ticket); err != nil {
				return err
			}

			if err := checkExpectedVersion(ticketID, command.ExpectedVersion, loadedVersion); err != nil {
				return err
			}

			sla, err := repositories.SLAPolicies.GetByPriority(ctx, priority)
			if err != nil {
				return err
			}

			if err := ticket.ChangePriority(priority, command.Reason, sla, h.clock.Now()); err != nil {
				return err
			}

			if err := repositories.Tickets.Update(ctx, ticket, loadedVersion); err != nil {
				return err
			}

			result = newTicketView(ticket)
			return nil
		},
	)
	if err != nil {
		return TicketView{}, err
	}

	return result, nil
}
