package application

import (
	"context"

	"github.com/Kosench/supportflow/internal/ticket/application/ports"
	"github.com/Kosench/supportflow/internal/ticket/domain"
)

type GetTicketQuery struct {
	Actor    Actor
	TicketID string
}

type GetTicketHandler struct {
	tickets ports.TicketRepository
}

func NewGetTicketHandler(tickets ports.TicketRepository) (GetTicketHandler, error) {
	if tickets == nil {
		return GetTicketHandler{}, ErrInvalidDependency
	}
	return GetTicketHandler{tickets: tickets}, nil
}

func (h GetTicketHandler) Handle(ctx context.Context, query GetTicketQuery) (TicketView, error) {
	if err := query.Actor.Valid(); err != nil {
		return TicketView{}, err
	}

	ticketID, err := domain.ParseTicketID(query.TicketID)
	if err != nil {
		return TicketView{}, err
	}

	ticket, err := h.tickets.GetByID(ctx, ticketID)
	if err != nil {
		return TicketView{}, err
	}

	if err := authorizeRead(query.Actor, ticket); err != nil {
		return TicketView{}, err
	}

	return newTicketView(ticket), nil
}
