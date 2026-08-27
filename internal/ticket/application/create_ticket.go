package application

import (
	"context"
	"fmt"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type CreateTicketCommand struct {
	Actor       Actor
	CategoryID  string
	Title       string
	Description string
	Priority    string
}

type CreateTicketHandler struct {
	unitOfWork ports.UnitOfWork
	clock      ports.Clock
	ids        ports.TicketIDGenerator
}

func NewCreateTicketHandler(
	unitOfWork ports.UnitOfWork,
	clock ports.Clock,
	ids ports.TicketIDGenerator,
) (CreateTicketHandler, error) {
	if unitOfWork == nil || clock == nil || ids == nil {
		return CreateTicketHandler{}, ErrInvalidDependency
	}

	return CreateTicketHandler{
		unitOfWork: unitOfWork,
		clock:      clock,
		ids:        ids,
	}, nil
}

func (h CreateTicketHandler) Handle(ctx context.Context, command CreateTicketCommand) (TicketView, error) {
	if err := command.Actor.Valid(); err != nil {
		return TicketView{}, err
	}

	if command.Actor.Role != RoleUser {
		return TicketView{}, deny(command.Actor, "ticket.create")
	}

	categoryID, err := domain.ParseCategoryID(command.CategoryID)
	if err != nil {
		return TicketView{}, err
	}

	priority, err := domain.ParsePriority(command.Priority)
	if err != nil {
		return TicketView{}, err
	}

	var result TicketView

	err = h.unitOfWork.WithinTransaction(
		ctx,
		func(repositories ports.Repositories) error {
			active, err := repositories.Categories.IsActive(ctx, categoryID)
			if err != nil {
				return fmt.Errorf("check category: %w", err)
			}
			if !active {
				return fmt.Errorf("%w: %s", ErrCategoryUnavailable, categoryID)
			}

			sla, err := repositories.SLAPolicies.GetByPriority(ctx, priority)
			if err != nil {
				return err
			}

			ticketID, err := h.ids.NewTicketID()
			if err != nil {
				return fmt.Errorf("generate ticket ID: %w", err)
			}

			ticket, err := domain.NewTicket(
				domain.NewTicketParams{
					ID:          ticketID,
					RequesterID: command.Actor.UserID,
					CategoryID:  categoryID,
					Title:       command.Title,
					Description: command.Description,
					Priority:    priority,
					SLA:         sla,
				},
				h.clock.Now(),
			)
			if err != nil {
				return err
			}

			if err := repositories.Tickets.Create(ctx, ticket); err != nil {
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
