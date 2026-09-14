package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type ClaimTicketCommand struct {
	Actor           Actor
	TicketID        string
	ExpectedVersion uint64
}

type ClaimTicketHandler struct {
	unitOfWork ports.UnitOfWork
	clock      ports.Clock
}

func NewClaimTicketHandler(unitOfWork ports.UnitOfWork, clock ports.Clock) (ClaimTicketHandler, error) {
	if unitOfWork == nil || clock == nil {
		return ClaimTicketHandler{}, ErrInvalidDependency
	}
	return ClaimTicketHandler{
		unitOfWork: unitOfWork,
		clock:      clock,
	}, nil
}

func (h ClaimTicketHandler) Handle(ctx context.Context, command ClaimTicketCommand) (AssignmentResult, error) {
	if err := command.Actor.Valid(); err != nil {
		return AssignmentResult{}, err
	}

	if command.Actor.Role != RoleOperator {
		return AssignmentResult{}, deny(command.Actor, "ticket.claim")
	}

	if err := validateExpectedVersion(command.ExpectedVersion); err != nil {
		return AssignmentResult{}, err
	}

	ticketID, err := domain.ParseTicketID(command.TicketID)
	if err != nil {
		return AssignmentResult{}, err
	}

	var result AssignmentResult
	err = h.unitOfWork.WithinTransaction(
		ctx,
		func(repositories ports.Repositories) error {
			ticket, err := repositories.Tickets.GetByIDForUpdate(
				ctx,
				ticketID,
			)
			if err != nil {
				return err
			}

			snapshot := ticket.Snapshot()
			now := h.clock.Now()
			_, err = repositories.Operators.LockEligibleForUpdate(
				ctx,
				command.Actor.UserID,
				snapshot.CategoryID,
				now,
			)
			if errors.Is(err, ports.ErrNoEligibleOperator) {
				return fmt.Errorf(
					"%w: %s",
					ErrOperatorUnavailable,
					command.Actor.UserID,
				)
			}
			if err != nil {
				return err
			}

			eligible, err := repositories.Operators.IsEligible(
				ctx,
				command.Actor.UserID,
				snapshot.CategoryID,
				now,
			)
			if err != nil {
				return err
			}
			if !eligible {
				return fmt.Errorf(
					"%w: %s",
					ErrOperatorUnavailable,
					command.Actor.UserID,
				)
			}

			if err := ensureUnassignedTicket(ticket); err != nil {
				return err
			}

			if err := checkExpectedVersion(
				ticketID,
				command.ExpectedVersion,
				snapshot.Version,
			); err != nil {
				return err
			}

			if err := ticket.Assign(
				command.Actor.UserID,
				now,
			); err != nil {
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
				command.Actor.UserID,
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
