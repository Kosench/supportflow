package assignment

import (
	"context"

	"github.com/Kosench/supportflow/internal/ticket/application"
	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type ForceAssignTicketCommand struct {
	Actor            application.Actor
	TicketID         string
	TargetOperatorID string
	ExpectedVersion  uint64
}

type ForceAssignTicketHandler struct {
	unitOfWork ports.UnitOfWork
	clock      ports.Clock
}

func NewForceAssignTicketHandler(
	unitOfWork ports.UnitOfWork,
	clock ports.Clock,
) (ForceAssignTicketHandler, error) {
	if unitOfWork == nil || clock == nil {
		return ForceAssignTicketHandler{}, application.ErrInvalidDependency
	}

	return ForceAssignTicketHandler{
		unitOfWork: unitOfWork,
		clock:      clock,
	}, nil
}

func (handler ForceAssignTicketHandler) Handle(
	ctx context.Context,
	command ForceAssignTicketCommand,
) (application.AssignmentResult, error) {
	if err := command.Actor.Valid(); err != nil {
		return application.AssignmentResult{}, err
	}

	if command.Actor.Role != application.RoleManager &&
		command.Actor.Role != application.RoleAdmin {
		return application.AssignmentResult{}, application.deny(
			command.Actor,
			"ticket.force_assign",
		)
	}

	if err := application.validateExpectedVersion(
		command.ExpectedVersion,
	); err != nil {
		return application.AssignmentResult{}, err
	}

	ticketID, err := domain.ParseTicketID(command.TicketID)
	if err != nil {
		return application.AssignmentResult{}, err
	}

	operatorID, err := domain.ParseUserID(command.TargetOperatorID)
	if err != nil {
		return application.AssignmentResult{}, err
	}

	var result application.AssignmentResult
	err = handler.unitOfWork.WithinTransaction(
		ctx,
		func(repositories ports.Repositories) error {
			ticket, err := repositories.Tickets.GetByIDForUpdate(
				ctx,
				ticketID,
			)
			if err != nil {
				return err
			}

			loadedVersion := ticket.Snapshot().Version
			if err := application.checkExpectedVersion(
				ticketID,
				command.ExpectedVersion,
				loadedVersion,
			); err != nil {
				return err
			}

			if err := repositories.Operators.LockActiveForUpdate(
				ctx,
				operatorID,
			); err != nil {
				return err
			}

			now := handler.clock.Now()
			if err := ticket.Assign(operatorID, now); err != nil {
				return err
			}

			if err := repositories.Tickets.Update(
				ctx,
				ticket,
				loadedVersion,
			); err != nil {
				return err
			}

			if err := repositories.Operators.MarkAssigned(
				ctx,
				operatorID,
				now,
			); err != nil {
				return err
			}

			result = application.newAssignmentResult(ticket, true)
			return nil
		},
	)
	if err != nil {
		return application.AssignmentResult{}, err
	}

	return result, nil
}
