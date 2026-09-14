package application

import (
	"context"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type CancelTicketCommand struct {
	Actor           Actor
	TicketID        string
	Reason          string
	ExpectedVersion uint64
}

type CancelTicketHandler struct {
	unitOfWork ports.UnitOfWork
	clock      ports.Clock
}

func NewCancelTicketHandler(unitOfWork ports.UnitOfWork, clock ports.Clock) (CancelTicketHandler, error) {
	if unitOfWork == nil || clock == nil {
		return CancelTicketHandler{}, ErrInvalidDependency
	}

	return CancelTicketHandler{
		unitOfWork: unitOfWork,
		clock:      clock,
	}, nil
}

func (handler CancelTicketHandler) Handle(ctx context.Context, command CancelTicketCommand) (TicketView, error) {
	if err := command.Actor.Valid(); err != nil {
		return TicketView{}, err
	}

	ticketID, err := domain.ParseTicketID(command.TicketID)
	if err != nil {
		return TicketView{}, err
	}

	if err := validateExpectedVersion(command.ExpectedVersion); err != nil {
		return TicketView{}, err
	}

	var result TicketView
	err = handler.unitOfWork.WithinTransaction(
		ctx,
		func(repositories ports.Repositories) error {
			ticket, loadedVersion, err := loadTicketForUpdate(ctx, repositories.Tickets, ticketID)
			if err != nil {
				return err
			}

			if err := authorizeCancel(command.Actor, ticket); err != nil {
				return err
			}

			if err := checkExpectedVersion(ticketID, command.ExpectedVersion, loadedVersion); err != nil {
				return err
			}

			if err := ticket.Cancel(command.Reason, handler.clock.Now()); err != nil {
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
