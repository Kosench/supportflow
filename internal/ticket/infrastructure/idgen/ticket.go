package idgen

import "github.com/Kosench/supportflow/internal/ticket/domain"

type Ticket struct{}

func (Ticket) NewTicketID() (domain.TicketID, error) {
	return domain.NewTicketID()
}
